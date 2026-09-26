# Task 08 — Hardening

**Phase:** 5 — Rilis
**Bergantung pada:** task 07
**Status:** belum dikerjakan

## Tujuan

Setelah task ini selesai, API tidak lagi cuma "fungsional" tapi juga tidak
gampang disalahgunakan atau ditembak turun oleh satu klien nakal: rate limit
di endpoint auth, timeout tuntas dari HTTP sampai ke query database, batas
ukuran body, header keamanan standar, CORS ketat, audit log yang tidak
membocorkan rahasia, dan pemisahan liveness/readiness yang benar. Tidak ada
endpoint domain baru di task ini — semuanya middleware dan konfigurasi di
atas yang sudah ada.

## Rujukan

- ADR-005, ADR-006, ADR-010 (rate limit dan audit log bersinggungan langsung
  dengan alur auth di sini)
- `ARCHITECTURE.md` bagian "Autentikasi", "Error"
- `AGENTS.md` root — aturan Guard soal apa yang boleh masuk log (rujuk dulu
  sebelum menulis audit logging di langkah 5)
- `AGENTS.md` app ini — "Aturan kode" (error wrapping, tidak ada state
  global)
- Task 00 — `http.Server` timeout dasar sudah ada, task ini memperkuat +
  menambah lapisan lain
- Task 02 — `platform/problem`, `platform/middleware`

## File yang dibuat atau disentuh

| Path | Isi |
| --- | --- |
| `internal/platform/middleware/ratelimit.go` | rate limiter in-memory per IP/email |
| `internal/platform/middleware/timeout.go` | timeout per request |
| `internal/platform/middleware/security_headers.go` | header keamanan |
| `internal/platform/middleware/cors.go` | konfigurasi CORS |
| `internal/platform/middleware/bodylimit.go` | batas ukuran body |
| `internal/platform/audit/audit.go` | logger audit event (login gagal, reuse, logout) |
| `internal/platform/config/config.go` | tambah field config (CORS origin, rate limit, dsb — kalau belum ada dari task 00) |
| `cmd/api/main.go` | pasang seluruh middleware di urutan yang benar, `/healthz` vs `/readyz` |
| `internal/platform/problem/problem_test.go` | test bahwa 500 tidak membocorkan detail internal |
| `internal/modules/identity/internal/application/login.go`, `refresh.go`, `logout.go` | panggil audit logger di titik yang relevan (hanya kalau belum dipasang task 04 — cek dulu) |

## Langkah

### 1. Rate limit di `/auth/login` dan `/auth/register`

Dua lapis:
- **Per IP** — mencegah satu klien membanjiri endpoint apa pun.
- **Per email, khusus login** — mencegah credential stuffing terhadap satu
  akun spesifik dari banyak IP.

```go
package middleware

type limiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

func (l *limiter) Allow(key string) bool {
	// token bucket sederhana: N request per window, refill linear
}

func RateLimit(byKey func(*http.Request) string, limit int, window time.Duration) func(http.Handler) http.Handler
```

Terapkan dua instance middleware ini di route `/auth/login`
(`byKey` = IP, lalu `byKey` = email dari body — perlu baca body sebelum
decode handler, atau taruh limiter kedua di dalam usecase login sebelum
verifikasi password) dan satu instance di `/auth/register` (`byKey` = IP
saja, karena belum ada email yang valid untuk dikunci sebelum akun dibuat).

**Kenapa in-memory cukup untuk sekarang, dan batasannya:** aplikasi ini
berjalan satu instance (ADR-001, modular monolith, satu proses) — state rate
limit yang hidup di memori proses itu valid selama cuma ada satu proses yang
menerima trafik. Batasannya eksplisit dan harus ditulis sebagai komentar di
kode: begitu aplikasi di-scale ke lebih dari satu instance (load balancer di
depan beberapa replica), tiap instance punya bucket-nya sendiri-sendiri —
penyerang bisa melewati limit dengan sekadar mengenai instance yang berbeda
tiap request. Solusi saat itu terjadi adalah rate limit di store bersama
(Redis, atau di API gateway/reverse proxy di depan semua instance) — bukan
sesuatu yang perlu dibangun sekarang, tapi harus disebutkan di komentar kode
supaya tidak terlihat seperti keputusan yang terlupa.

Response saat limit terlampaui: **429**, tetap `application/problem+json`
lewat `problem.Write` — jangan menulis body ad-hoc di middleware rate limit,
panggil helper yang sama dengan error path lain. Tambahkan error sentinel
baru di `platform/problem` (misal `problem.ErrTooManyRequests`) kalau belum
ada, dengan header `Retry-After` diisi dari sisa window.

### 2. Timeout menyeluruh

Tiga lapis, dan task ini menjelaskan kenapa ketiganya perlu, bukan cuma satu:

1. **`http.Server`** (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`,
   `ReadHeaderTimeout`) — sudah ada dari task 00, verifikasi masih terpasang
   dan nilainya masih masuk akal setelah endpoint bertambah banyak (upload
   tidak ada di scope PRD, jadi angka task 00 tetap wajar).
2. **Middleware timeout per request** — `context.WithTimeout` yang dibungkus
   jadi middleware Chi, dipasang setelah auth (supaya waktu verifikasi token
   tidak ikut kepotong) dan sebelum handler:

```go
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

3. **Timeout di level query database** — pgx menerima `context.Context` di
   setiap pemanggilan query (`pool.Query(ctx, ...)`). **Ini lapisan yang
   sering dilewatkan, dan task ini menegaskannya secara eksplisit:** kalau
   `context` dari `http.Server`/middleware timeout tidak benar-benar dibawa
   sampai ke pemanggilan pgx (misal karena suatu tempat di kode memakai
   `context.Background()` alih-alih context yang diteruskan pemanggil),
   maka timeout HTTP saja **tidak menyelamatkan apa-apa** — begitu response
   HTTP sudah dikirim ke klien (timeout), query yang terlanjur dikirim ke
   Postgres tetap berjalan sampai selesai di sisi server database,
   menghabiskan koneksi dari pool dan resource Postgres untuk pekerjaan yang
   hasilnya sudah tidak akan pernah dibaca siapa pun. Cek ulang seluruh
   repository di modul `identity` dan `task`: setiap method wajib menerima
   `ctx context.Context` sebagai parameter pertama dan meneruskannya apa
   adanya ke pemanggilan `Querier`, tidak pernah menggantinya dengan
   `context.Background()` di tengah jalan.

Tambahkan juga statement timeout di level koneksi Postgres sebagai jaring
pengaman terakhir (`SET statement_timeout` lewat `pgxpool.Config.AfterConnect`
atau parameter `default_query_exec_mode`/`statement_timeout` di
`DATABASE_URL`), untuk kasus context yang entah bagaimana tidak terbawa —
jaring pengaman, bukan pengganti disiplin meneruskan context.

### 3. Batas ukuran body dan panjang field

```go
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
```

Pasang global (misal 1 MiB — tidak ada endpoint upload file di scope PRD,
jadi payload JSON terbesar yang wajar adalah body todo dengan notes panjang).
`http.MaxBytesReader` membuat decoder JSON gagal dengan error yang bisa
ditangkap `httpx` decode helper (task 02) dan dipetakan ke 413/400 lewat
`problem.Write` — pastikan `httpx.Decode` sudah membedakan error
"body terlalu besar" dari error JSON malformed biasa kalau pesannya perlu
beda; kalau tidak, 400 generik untuk keduanya tetap dapat diterima.

Batas panjang field per-field (judul list 100 karakter, judul todo 200
karakter) sudah ditegakkan domain di task 06 — task ini tidak mengulanginya,
cukup pastikan validasi itu benar-benar dipanggil dari handler sebelum masuk
usecase (validasi ganda: bentuk di HTTP, aturan bisnis di domain) supaya
error validasi field balik sebagai 400 dengan pesan per-field, bukan 500 dari
domain error yang tidak dipetakan.

### 4. Security header

```go
func SecurityHeaders(appEnv string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			if appEnv != "local" {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

HSTS dikecualikan untuk `APP_ENV=local` karena dev lokal biasanya jalan lewat
`http://localhost` tanpa TLS — mengirim HSTS di sana membuat browser memaksa
HTTPS untuk `localhost` di kunjungan berikutnya, yang gagal total kalau dev
server memang tidak punya sertifikat. `APP_ENV` sudah ada di `Config` sejak
task 00 (`AppEnv string`) — pakai nilai itu, jangan menambah variabel
lingkungan baru untuk keputusan ini.

### 5. CORS ketat

Pakai `github.com/go-chi/cors` atau tulis manual — origin **dibaca dari
config** (`CORS_ALLOWED_ORIGINS`, sudah ada dari task 00), tidak pernah
wildcard `*` di production. `AllowCredentials: true` **hanya** untuk request
ke `/auth/refresh` (satu-satunya endpoint yang mengandalkan cookie) — bukan
untuk seluruh API. Kalau library CORS yang dipakai tidak bisa mengatur
`AllowCredentials` per-route, pasang dua middleware CORS terpisah: satu untuk
grup route `/auth/refresh` dengan `AllowCredentials: true`, satu untuk sisanya
tanpa itu.

### 6. Audit logging

Event yang **wajib** dicatat: login gagal (email + alasan generik, bukan
"password salah" vs "email tidak ada" — dua-duanya harus tercatat dengan
pesan yang sama supaya log sendiri tidak membocorkan mana email yang
terdaftar), refresh token reuse terdeteksi (ini insiden serius — sertakan
`session_id` dan `user_id`, bukan token apa pun), dan logout.

```go
package audit

type Logger struct {
	log *slog.Logger
}

func (a *Logger) LoginFailed(ctx context.Context, email string, ip string) {
	a.log.WarnContext(ctx, "audit.login_failed", "email", email, "ip", ip)
}

func (a *Logger) RefreshReuseDetected(ctx context.Context, userID, sessionID uuid.UUID) {
	a.log.ErrorContext(ctx, "audit.refresh_reuse_detected", "user_id", userID, "session_id", sessionID)
}

func (a *Logger) Logout(ctx context.Context, userID uuid.UUID) {
	a.log.InfoContext(ctx, "audit.logout", "user_id", userID)
}
```

**TIDAK BOLEH mencatat password, token mentah, atau hash token** — ini
kriteria selesai eksplisit, bukan saran. Rujuk aturan Guard di `AGENTS.md`
root sebelum menulis satu baris log pun di sini: field apa pun yang bisa
dipakai langsung untuk otentikasi (password plaintext, access token JWT,
refresh token mentah, `refresh_token_hash` dari kolom database) haram masuk
argumen `slog`, termasuk secara tidak sengaja lewat `"%+v"` pada struct yang
memuat field itu (`%+v` pada struct request login akan mencetak field
`Password` apa adanya — jangan log struct request mentah, log field yang
sudah dipilih eksplisit satu per satu). Tulis test kecil (bisa unit, bukan
harus integration) yang memanggil tiap fungsi audit dengan input yang
menyertakan token/password contoh dan assert output log tidak memuat
substring itu.

Wiring: dipanggil dari usecase `identity` (`Login`, `RefreshToken`,
`Logout` — task 04) lewat interface kecil yang dideklarasikan di
`application` modul `identity` sendiri (bukan import `platform/audit`
langsung dari domain — usecase menerima `AuditLogger` sebagai dependency,
sama seperti `Clock` dan repository). Kalau task 04 sudah memasang audit
logging dengan pola berbeda, sesuaikan dokumentasi di task ini mengikuti
yang sudah ada — jangan menulis ulang yang sudah benar.

### 7. `/healthz` vs `/readyz`

```go
r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
	httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
})

r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		httpx.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
		return
	}
	httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
})
```

**Kenapa dibedakan, dan kenapa itu penting:** liveness (`/healthz`) menjawab
"apakah proses ini masih hidup dan bisa merespons HTTP sama sekali" — dia
sengaja **tidak** menyentuh database, karena kalau `/healthz` ikut memeriksa
Postgres dan Postgres sedang lambat/down, orchestrator (k8s, atau apa pun di
depan container) akan membaca itu sebagai "proses ini mati" dan
me-restart-nya berulang — padahal proses aplikasinya sendiri sehat, cuma
dependency-nya yang bermasalah, dan me-restart proses tidak menyembuhkan
Postgres. Readiness (`/readyz`) menjawab pertanyaan berbeda: "apakah proses
ini siap menerima trafik nyata sekarang" — ini boleh dan harus memeriksa
pool database, karena kalau database tidak terjangkau, mengirim trafik ke
instance ini cuma menghasilkan 500 untuk semua request. Orchestrator memakai
liveness untuk keputusan restart dan readiness untuk keputusan routing —
mencampur keduanya membuat orchestrator mengambil keputusan yang salah untuk
salah satu dari dua situasi itu.

### 8. Verifikasi `problem.Write` tidak membocorkan detail internal pada 500

Tinjau ulang implementasi `platform/problem` dari task 02: untuk error yang
tidak dikenali (bukan salah satu error domain yang dipetakan eksplisit),
`detail` yang dikirim ke klien harus pesan generik tetap
(`"an internal error occurred"` atau sejenis), sementara `err.Error()` yang
sesungguhnya hanya masuk ke log server. Tambahkan test yang membuktikan ini:

```go
func TestProblemWrite_InternalErrorNotLeaked(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	dbErr := errors.New("pq: password authentication failed for user \"admin\" at 10.0.0.5")
	problem.Write(rec, req, dbErr)

	body := rec.Body.String()
	if strings.Contains(body, "10.0.0.5") || strings.Contains(body, "admin") {
		t.Fatalf("response leaked internal error detail: %s", body)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
}
```

## Kriteria selesai

```bash
cd apps/go-chi-api
go build ./...
go vet ./...
make lint
make test
make test-integration
```

- Semua di atas hijau.
- `TestProblemWrite_InternalErrorNotLeaked` (atau nama setara) ada dan lulus.
- Test audit logging membuktikan token/password tidak muncul di output log —
  ada dan lulus.
- `curl -i -X POST localhost:8080/v1/auth/login -d '...'` berulang lebih
  dari batas rate limit dalam window yang sama mengembalikan 429 dengan
  `Content-Type: application/problem+json` dan header `Retry-After`.
- `curl -i localhost:8080/healthz` tetap 200 walau Postgres dimatikan
  (`make dockerdev-down` lalu tes lagi); `curl -i localhost:8080/readyz`
  menjadi 503 pada kondisi yang sama.
- `curl -i localhost:8080/healthz` menunjukkan header
  `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`; header `Strict-Transport-Security` **tidak
  ada** saat `APP_ENV=local` dan **ada** saat `APP_ENV` diset selain itu.
- Body request lebih besar dari batas yang ditetapkan mengembalikan
  4xx (bukan hang, bukan 500) — cek manual dengan `curl` mengirim body besar.

## Jebakan

- Memasang middleware timeout per-request **sebelum** middleware auth
  membuat waktu verifikasi token ikut termakan budget timeout request —
  pasang setelah auth.
- Rate limit per-email untuk login butuh membaca body sebelum handler decode
  resmi jalan — kalau dibaca dua kali tanpa `io.TeeReader`/buffer ulang,
  decode kedua akan mendapat body kosong. Pertimbangkan menaruh cek rate
  limit per-email di dalam usecase `Login` (setelah decode, sebelum verifikasi
  password) alih-alih di middleware HTTP, supaya tidak perlu membaca body dua
  kali sama sekali — pilih pendekatan ini kalau membaca-ulang body terasa
  rapuh.
- Statement timeout di level Postgres yang di-set terlalu pendek akan
  membatalkan query yang sebenarnya wajar lambat (laporan/agregasi berat) —
  tidak ada endpoint seperti itu di scope PRD sekarang, tapi catat di
  komentar kode kalau nanti ada, angka ini perlu ditinjau ulang per endpoint.
- `context.Background()` yang menyelinap masuk di satu pemanggilan pgx saja
  (misal di kode yang ditulis buru-buru saat task 07) meniadakan seluruh
  poin langkah 2 tanpa terlihat di test unit — hanya kelihatan saat load
  test atau saat klien benar-benar disconnect di tengah query lambat. Grep
  `context.Background()` dan `context.TODO()` di seluruh `internal/modules`
  sebagai bagian dari review task ini; satu-satunya tempat wajar
  memanggilnya adalah di `cmd/api/main.go` saat start up.
- Header `Access-Control-Allow-Credentials: true` yang terpasang global
  (bukan cuma di `/auth/refresh`) bersama `Access-Control-Allow-Origin: *`
  adalah kombinasi yang ditolak browser modern, tapi kalau origin-nya
  spesifik (bukan wildcard) kombinasi itu tetap "jalan" walau melebar dari
  yang seharusnya — jangan mengandalkan browser menolaknya, batasi sendiri
  di kode per-route.
