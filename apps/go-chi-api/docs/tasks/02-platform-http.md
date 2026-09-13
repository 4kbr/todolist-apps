# Task 02 — Platform HTTP

**Phase:** 1 — Fondasi
**Bergantung pada:** Task 00 — Foundation
**Status:** belum dikerjakan

## Tujuan

Setelah task ini selesai, tersedia pondasi HTTP yang dipakai semua modul
domain nanti: error RFC 9457, decode/validate request, keyset pagination,
middleware standar (request id, log, recover, timeout, CORS, batas ukuran
body), dan pola transaksi (`UnitOfWork`) yang dibawa lewat `context`. Semua
bagian ini bisa diuji tanpa database — task ini wajib menghasilkan unit test
untuk problem mapping, decode/validate, dan cursor encode/decode. Belum ada
modul `identity` atau `task` yang memakainya — itu menyusul task 03+.

## Rujukan

- ADR-003 (interface dideklarasikan konsumen) — pola yang sama dipakai di
  `middleware.Auth(TokenVerifier)`, walau `TokenVerifier` sungguhan baru ada
  task 04
- ADR-010 (RFC 9457 `application/problem+json`)
- `ARCHITECTURE.md` bagian "Error", "Transaksi", dan "Aliran satu request"
- `AGENTS.md` bagian "Error selalu dibungkus dengan konteks" dan "Transaksi
  dibuka usecase, bukan repository"

## File yang dibuat atau disentuh

| Path | Kenapa |
| --- | --- |
| `internal/platform/problem/problem.go` | struct `Problem`, `Write`, registry pemetaan error → status |
| `internal/platform/problem/problem_test.go` | unit test pemetaan error → body/status |
| `internal/platform/httpx/decode.go` | `Decode[T]` JSON + batas ukuran body |
| `internal/platform/httpx/validate.go` | `Validate` (go-playground/validator v10) → `Problem.Errors` |
| `internal/platform/httpx/respond.go` | `JSON(w, status, v)`, `NoContent(w)` |
| `internal/platform/httpx/pagination.go` | parser `limit`/`cursor` keyset + encoder |
| `internal/platform/httpx/decode_test.go` | unit test decode + validate |
| `internal/platform/httpx/pagination_test.go` | unit test cursor round-trip |
| `internal/platform/middleware/requestid.go` | pakai atau hasilkan request id |
| `internal/platform/middleware/logger.go` | access log terstruktur |
| `internal/platform/middleware/recover.go` | panic → 500 problem+json |
| `internal/platform/middleware/timeout.go` | pembatas durasi per request |
| `internal/platform/middleware/cors.go` | CORS dari config |
| `internal/platform/middleware/maxbody.go` | batas ukuran body |
| `internal/platform/middleware/auth.go` | interface `TokenVerifier` + `Auth` middleware (dipakai mulai task 04) |
| `internal/platform/postgres/tx.go` | `UnitOfWork`, transaksi di context, `Querier` |
| `cmd/api/main.go` | pasang urutan middleware (menyunting file dari task 00) |

## Langkah

### 1. `internal/platform/problem`

```go
package problem

type Problem struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   map[string]string `json:"errors,omitempty"`
}

// mapping menghubungkan sentinel error domain ke Problem template.
// Diisi lewat Register, biasanya dari init() paket ini sendiri untuk error
// generik, dan dari platform/problem tidak pernah oleh domain langsung —
// domain tidak tahu HTTP.
type mapping struct {
	status int
	title  string
}

var registry = map[error]mapping{}

func Register(err error, status int, title string) {
	registry[err] = mapping{status: status, title: title}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	for sentinel, m := range registry {
		if errors.Is(err, sentinel) {
			writeProblem(w, r, m.status, m.title, err.Error())
			return
		}
	}
	// Error tidak dikenal: 500, detail generik ke klien, error asli ke log.
	slog.Error("unhandled error", "error", err, "path", r.URL.Path)
	writeProblem(w, r, http.StatusInternalServerError,
		"Internal Server Error", "an unexpected error occurred")
}

func WriteValidation(w http.ResponseWriter, r *http.Request, fieldErrors map[string]string) {
	writeProblemErrors(w, r, http.StatusBadRequest, "Validation Failed", fieldErrors)
}
```

`Write` dipakai handler untuk error domain (mis. `domain.ErrListNotFound`).
`WriteValidation` dipakai khusus hasil `httpx.Validate` karena bentuknya beda
(kumpulan error per field, bukan satu error tunggal).

Aturan tegas: **error yang tidak dikenal di `registry` selalu jadi 500**,
`detail` di body selalu string generik ("an unexpected error occurred"),
tidak pernah `err.Error()` mentah — pesan error Go sering memuat detail
internal (nama tabel, path file, tipe driver) yang tidak boleh sampai ke
klien. Error aslinya wajib masuk log lewat `slog.Error` sebelum response
dikirim, supaya tetap bisa didiagnosis dari sisi server.

Contoh body 400 validasi:

```json
{
  "type": "about:blank",
  "title": "Validation Failed",
  "status": 400,
  "errors": {
    "email": "must be a valid email address",
    "password": "must be at least 8 characters"
  }
}
```

Contoh body 404:

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "list not found"
}
```

`Content-Type` response selalu `application/problem+json`, diset di
`writeProblem`/`writeProblemErrors`, bukan diulang tiap handler.

`Register` dipanggil dari `main.go` atau `init()` di paket masing-masing
modul untuk sentinel error domain mereka (mis. `identity` mendaftarkan
`domain.ErrUserNotFound` → 404 di task 04). Task ini hanya menyediakan
mekanismenya; belum ada domain nyata untuk didaftarkan selain contoh di test.

Tulis `problem_test.go` dengan sentinel error palsu (`var errFake = errors.New(...)`),
daftarkan lewat `Register`, panggil `Write` ke `httptest.NewRecorder()`, cek
status code dan body JSON. Tambah satu test untuk error yang **tidak**
didaftarkan — pastikan hasilnya 500 dan detail generik, bukan pesan error asli.

### 2. `internal/platform/httpx` — decode & validate

```go
package httpx

const maxBodyBytes = 1 << 20 // 1 MiB

func Decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("decode request body: %w", err)
	}
	return v, nil
}
```

`DisallowUnknownFields` menolak field yang tidak dikenal alih-alih diam-diam
mengabaikannya — klien yang salah eja nama field dapat error jelas saat itu
juga, bukan bug tersembunyi ("kenapa `emial` tidak pernah tersimpan?").
`http.MaxBytesReader` mencegah body raksasa menghabiskan memori sebelum
sempat divalidasi.

```go
var validate = validator.New(validator.WithRequiredStructEnabled())

func Validate(v any) map[string]string {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}
	fieldErrors := map[string]string{}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, fe := range verrs {
			fieldErrors[jsonFieldName(v, fe.StructField())] = humanizeTag(fe)
		}
	}
	return fieldErrors
}
```

Pakai `github.com/go-playground/validator/v10` (`go get` di task ini).
`Validate` mengembalikan `nil` kalau valid, atau `map[string]string` siap
pakai untuk `problem.WriteValidation`. `jsonFieldName` memetakan nama field Go
ke nama tag `json:"..."` supaya pesan error merujuk nama field yang klien
kirim, bukan nama field Go internal.

Pola pemakaian di handler (ilustrasi, bukan kode task ini):

```go
req, err := httpx.Decode[RegisterRequest](w, r)
if err != nil {
	problem.Write(w, r, err)
	return
}
if fieldErrors := httpx.Validate(req); fieldErrors != nil {
	problem.WriteValidation(w, r, fieldErrors)
	return
}
```

### 3. `internal/platform/httpx` — respond

```go
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
```

### 4. `internal/platform/httpx` — keyset pagination

```go
const (
	defaultLimit = 20
	maxLimit     = 100
)

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// EncodeCursor mengubah posisi baris terakhir jadi token buram base64
// yang aman dikirim balik ke klien lewat query string.
func EncodeCursor(c Cursor) string {
	raw, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(raw)
}

func DecodeCursor(s string) (Cursor, error) {
	var c Cursor
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, fmt.Errorf("decode cursor: %w", err)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return c, nil
}

type PageParams struct {
	Limit  int
	Cursor *Cursor
}

// ParsePageParams membaca "limit" dan "cursor" dari query string.
// limit di luar rentang dijepit ke default/max, bukan menghasilkan error —
// klien yang minta limit=99999 cukup dikoreksi, bukan ditolak.
func ParsePageParams(r *http.Request) (PageParams, error) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return PageParams{}, fmt.Errorf("invalid limit: %w", err)
		}
		limit = n
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	var cursor *Cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, err := DecodeCursor(raw)
		if err != nil {
			return PageParams{}, err
		}
		cursor = &c
	}

	return PageParams{Limit: limit, Cursor: &cursor}, nil
}
```

(Catatan implementasi: `PageParams.Cursor` cukup `*Cursor`, bukan `**Cursor` —
perbaiki tipe balik `cursor` jadi `PageParams{Limit: limit, Cursor: cursor}`
saat menulis kode; contoh di atas fokus ke alur logika.)

Keyset dipilih daripada `OFFSET` karena dua alasan konkret, bukan sekadar
"lebih modern":

1. **Performa menurun seiring kedalaman.** `OFFSET 10000` tetap membuat
   Postgres memindai dan membuang 10000 baris pertama sebelum mengembalikan
   halaman ke-501. Keyset (`WHERE (created_at, id) < (?, ?) ORDER BY
   created_at DESC, id DESC LIMIT ?`) langsung lompat ke titik itu lewat
   index, biaya query tidak naik walau makin dalam halamannya.
2. **Konsistensi saat data berubah di tengah paginasi.** Kalau baris baru
   masuk di antara dua request `OFFSET`, seluruh baris bergeser satu posisi —
   klien bisa melewatkan baris atau melihat baris yang sama dua kali di
   halaman berbeda. Keyset mengikat posisi ke nilai kolom sungguhan
   (`created_at`, `id`), jadi tidak terpengaruh baris baru yang masuk di
   ujung lain urutan.

Harga yang dibayar: klien tidak bisa lompat langsung ke "halaman 5" — hanya
bisa "lanjut dari sini". Untuk feed/list yang terus tumbuh (todo per list),
itu trade-off yang wajar.

Tulis `pagination_test.go`: `EncodeCursor` lalu `DecodeCursor` harus
menghasilkan `Cursor` yang sama persis (bandingkan field, bukan `reflect.DeepEqual`
pada `time.Time` mentah — pakai `.Equal()` untuk waktu). Tambah test
`ParsePageParams` untuk: tanpa query (dapat default), `limit` melebihi
`maxLimit` (dijepit), `cursor` tidak valid (error).

### 5. `internal/platform/middleware`

Urutan pemasangan di `main.go`, dari terluar ke terdalam:

```go
r.Use(middleware.RequestID)
r.Use(middleware.Logger(log))
r.Use(middleware.Recover(log))
r.Use(middleware.Timeout(10 * time.Second))
r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
r.Use(middleware.MaxBodyBytes(1 << 20))
```

Kenapa urutan ini, bukan sembarang:

- **`RequestID` paling luar.** Semua middleware dan handler setelahnya butuh
  request id sudah ada di context supaya log mereka bisa dikorelasikan. Kalau
  dipasang belakangan, log dari middleware yang lebih luar tidak punya id.
- **`Recover` di dalam `Logger`, bukan di luarnya.** Kalau `Recover` di luar
  `Logger`, panic membuat `Logger` tidak sempat mencatat baris response sama
  sekali — request yang panic jadi tidak tercatat di access log. Dengan
  `Recover` di dalam, `Logger` tetap melihat response akhir (500) yang
  dikirim `Recover` dan mencatatnya sebagai baris log normal.

```go
package middleware

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

```go
// statusRecorder membungkus http.ResponseWriter supaya status code yang
// dikirim handler bisa dibaca lagi setelah ServeHTTP selesai — http.ResponseWriter
// standar tidak punya cara membaca balik apa yang sudah ditulis lewat WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}
```

```go
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"error", rec,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
					)
					problem.Write(w, r, fmt.Errorf("internal panic: %v", rec))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
```

`Timeout` cukup pembungkus `http.TimeoutHandler` standard library dengan body
problem+json untuk kasus timeout. `CORS` baca daftar origin dari
`cfg.CORSAllowedOrigins` ([]string) — **jangan** `AllowedOrigins: []string{"*"}`,
karena kombinasi wildcard origin dengan `Access-Control-Allow-Credentials:
true` (dibutuhkan untuk cookie refresh token di ADR-006) ditolak browser
modern dan secara keamanan memang tidak masuk akal untuk endpoint yang
menerima cookie. `MaxBodyBytes` membungkus `http.MaxBytesReader` di level
middleware sebagai lapisan pertahanan tambahan di luar apa yang sudah
dilakukan `httpx.Decode` per-handler.

`internal/platform/middleware/auth.go` — kontraknya ditulis sekarang karena
router butuh tahu bentuknya, implementasinya (`identity.Module`) baru ada
task 04:

```go
package middleware

type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, raw string) (uuid.UUID, error)
}

func Auth(v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if raw == "" {
				problem.Write(w, r, ErrMissingToken)
				return
			}
			userID, err := v.VerifyAccessToken(r.Context(), raw)
			if err != nil {
				problem.Write(w, r, fmt.Errorf("verify access token: %w", err))
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

Ini `TokenVerifier` yang sama persis dengan yang disebut ADR-003 dan
`ARCHITECTURE.md` — dideklarasikan di sisi pemakai (`middleware`), bukan
di-import dari `identity`. Task ini **tidak** memasang `Auth` ke router
manapun karena belum ada `TokenVerifier` sungguhan; itu terjadi saat modul
`identity` disuntikkan di `main.go` (task 04+).

### 6. `internal/platform/postgres` — Unit of Work

Ini bagian yang paling gampang salah, jadi ikuti pola berikut persis:

```go
package postgres

// Querier adalah irisan metode pgx yang dipakai kode hasil generate sqlc —
// dipenuhi baik *pgxpool.Pool maupun pgx.Tx, sehingga repository tidak perlu
// tahu ia sedang jalan di dalam transaksi atau tidak.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type unitOfWork struct {
	pool *pgxpool.Pool
}

func NewUnitOfWork(pool *pgxpool.Pool) UnitOfWork {
	return &unitOfWork{pool: pool}
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op kalau sudah Commit

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// FromContext mengembalikan Querier aktif: pgx.Tx kalau context sedang di
// dalam UnitOfWork.Do, atau pool langsung kalau tidak. Repository memanggil
// ini di setiap metode, tidak pernah menyimpan Querier sebagai field struct.
func FromContext(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
```

Pola pemakaian dari repository (ilustrasi, bukan kode task ini — repository
sungguhan baru ditulis task 05/07):

```go
type SessionRepository struct {
	pool *pgxpool.Pool
}

func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	q := postgres.FromContext(ctx, r.pool)
	queries := generated.New(q)
	return queries.RevokeSession(ctx, id)
}
```

Dan usecase yang membuka transaksi (ilustrasi, mencerminkan contoh di
`ARCHITECTURE.md` bagian "Transaksi"):

```go
err := uow.Do(ctx, func(ctx context.Context) error {
	if err := sessionRepo.Revoke(ctx, old.ID); err != nil {
		return fmt.Errorf("revoke old session: %w", err)
	}
	return sessionRepo.Create(ctx, next)
})
```

Karena `Revoke` dan `Create` sama-sama memanggil `postgres.FromContext(ctx,
pool)`, dan `ctx` yang mereka terima sudah dibungkus `txCtx` oleh `Do`,
keduanya otomatis mendapat `pgx.Tx` yang sama tanpa `SessionRepository` tahu
apa pun tentang transaksi — dia hanya tahu "ambil Querier dari context".
Kalau `Revoke` dipanggil **di luar** `uow.Do`, `FromContext` mengembalikan
`pool` biasa dan jalan sebagai auto-commit statement tunggal, tanpa
`SessionRepository` perlu kode berbeda untuk dua kasus itu.

Repository **tidak pernah** memanggil `pool.Begin` sendiri — itu yang
ditegaskan `AGENTS.md`: transaksi dibuka usecase, bukan repository.

### 7. Sunting `cmd/api/main.go`

Tambahkan pemasangan middleware sesuai urutan langkah 5, dan siapkan
`uow := postgres.NewUnitOfWork(pool)` untuk disuntikkan ke modul nanti (belum
ada modul untuk disuntikkan hari ini, cukup buat variabelnya dan biarkan
`_ = uow` sementara kalau linter komplain unused — atau tunda deklarasi ini
ke task yang benar-benar memakainya kalau `unused` jadi masalah lint; pilih
salah satu dan catat di komentar kode kenapa).

## Kriteria selesai

- [ ] `go test ./internal/platform/... -short` hijau, mencakup test problem
      mapping, decode/validate, dan cursor round-trip
- [ ] `make lint` bersih untuk seluruh `internal/platform/`
- [ ] Test manual: request dengan body JSON berisi field tidak dikenal ke
      handler yang pakai `httpx.Decode` menghasilkan error (butuh handler uji
      coba sementara atau test langsung ke fungsi `Decode`)
- [ ] Test manual: `EncodeCursor` lalu `DecodeCursor` mengembalikan nilai
      `created_at` dan `id` yang identik
- [ ] `curl -s localhost:8080/healthz` masih menghasilkan `{"status":"ok"}`
      setelah middleware dipasang, dan header response memuat `X-Request-ID`
- [ ] Panic sengaja di satu handler percobaan menghasilkan response
      `application/problem+json` status 500, bukan koneksi putus atau
      stack trace mentah ke klien, dan tetap muncul satu baris di access log

## Jebakan

- `errors.Is` di `problem.registry` butuh key map berupa nilai `error` yang
  sama persis (pointer identity untuk `errors.New`) — jangan pakai
  `err.Error()` string sebagai key, itu akan cocok ke sentinel yang salah
  kalau ada dua error dengan pesan yang kebetulan sama.
- `DisallowUnknownFields` tidak bekerja kalau `Decode` dipanggil dua kali ke
  `Body` yang sama (mis. logging middleware yang ikut membaca body) — body
  `http.Request` hanya bisa dibaca sekali kecuali sengaja di-buffer ulang.
  Task ini tidak butuh baca body dua kali; kalau task lain nanti butuh,
  itu perlu penanganan terpisah (`io.TeeReader` atau baca-lalu-`bytes.NewReader`).
- `statusRecorder.WriteHeader` harus benar-benar dipanggil sebelum body
  ditulis — kalau handler langsung `w.Write(...)` tanpa `WriteHeader`
  eksplisit, Go otomatis mengirim 200 duluan, dan `statusRecorder` mencatat
  200 secara implisit lewat default `status: http.StatusOK` yang sudah
  diset di awal `Logger` — pastikan default itu ada, kalau tidak log akan
  mencatat status 0.
- `tx.Rollback(ctx)` yang dipanggil lewat `defer` setelah `tx.Commit(ctx)`
  sukses **bukan** error — pgx mendokumentasikan itu sebagai no-op yang aman.
  Jangan menambah pengecekan `if err != nil` di sekitar `defer tx.Rollback`
  untuk "membersihkan" ini; itu behavior yang disengaja.
- Urutan middleware yang tertukar (`Recover` di luar `Logger`) adalah bug
  senyap: semuanya tetap terlihat jalan normal sampai ada panic sungguhan di
  production, baru ketahuan access log request itu hilang.
- `CORS` dengan `AllowCredentials: true` tidak boleh punya
  `AllowedOrigins: []string{"*"}` — sebagian besar library CORS Go akan
  panic atau diam-diam menolak kombinasi ini; baca dokumentasi library yang
  dipilih (mis. `github.com/go-chi/cors`) sebelum menuliskan konfigurasinya.
