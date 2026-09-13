# Task 07 — Adapter Modul Task (Postgres + HTTP)

**Phase:** 4 — Task
**Bergantung pada:** task 06
**Status:** belum dikerjakan

## Tujuan

Setelah task ini selesai, modul `task` bisa dipanggil lewat HTTP sungguhan
dan datanya benar-benar disimpan di Postgres: semua endpoint list dan todo
dari `docs/openapi/openapi.yaml` (task 03) jalan, di-scope per user sampai ke
lapisan SQL, dan diuji lewat integration test yang menembus database asli.
`internal/modules/task/module.go` jadi satu-satunya pintu keluar modul, dan
`cmd/api/main.go` memasangnya di belakang middleware auth.

## Rujukan

- ADR-003, ADR-004, ADR-007, ADR-008, ADR-011, ADR-013
- `ARCHITECTURE.md` bagian "Aliran satu request", "Transaksi", "Database",
  "Error"
- `AGENTS.md` bagian "SQL dan migrasi", "Testing", "Sebelum bilang selesai"
- `docs/openapi/openapi.yaml` (task 03) — kontrak endpoint, jangan menyimpang
- Task 02 — `platform/httpx` (decode, validate, respond, parse query
  pagination), `platform/problem`, tx manager (`Querier` dari context)
- Task 05 — skema `lists`/`todos` final, termasuk index
- Task 06 — domain dan usecase yang disambungkan di sini

## File yang dibuat atau disentuh

| Path | Isi |
| --- | --- |
| `internal/modules/task/internal/adapter/postgres/queries/lists.sql` | query `ListRepository` |
| `internal/modules/task/internal/adapter/postgres/queries/todos.sql` | query `TodoRepository` |
| `internal/modules/task/internal/adapter/postgres/*.go` (hasil sqlc) | digenerate, jangan ditulis manual |
| `internal/modules/task/internal/adapter/postgres/list_repository.go` | implementasi `domain.ListRepository` |
| `internal/modules/task/internal/adapter/postgres/todo_repository.go` | implementasi `domain.TodoRepository` |
| `internal/modules/task/internal/adapter/postgres/errors.go` | pemetaan `*pgconn.PgError` → error domain |
| `internal/modules/task/internal/adapter/http/lists_handler.go` | handler endpoint list |
| `internal/modules/task/internal/adapter/http/todos_handler.go` | handler endpoint todo |
| `internal/modules/task/internal/adapter/http/dto.go` | request/response struct |
| `internal/modules/task/internal/adapter/http/routes.go` | daftar route Chi |
| `internal/modules/task/module.go` | `Module`, `New(Deps)`, `Routes()` |
| `sqlc.yaml` | tambah entry modul `task` |
| `cmd/api/main.go` | wiring modul `task` di belakang auth |
| `internal/modules/task/internal/adapter/postgres/*_integration_test.go` | integration test repository |
| `internal/modules/task/internal/adapter/http/*_integration_test.go` | integration test HTTP end-to-end |

## Langkah

### 1. `sqlc.yaml` — tambah entry

Ikuti pola entry `identity` yang sudah ada dari task sebelumnya (ADR-008: satu
entry per modul, output tetap di dalam `internal/modules/task/internal/adapter/postgres/`).

### 2. Query list — sederhana

`queries/lists.sql` cukup CRUD lurus dengan `WHERE user_id = $1` di semua
tempat. Contoh:

```sql
-- name: CreateList :one
INSERT INTO lists (id, user_id, name, position, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING *;

-- name: GetListByID :one
SELECT * FROM lists WHERE id = $1 AND user_id = $2;

-- name: ListListsByUser :many
SELECT * FROM lists WHERE user_id = $1 ORDER BY position ASC, created_at ASC;

-- name: DeleteList :execrows
DELETE FROM lists WHERE id = $1 AND user_id = $2;
```

`DeleteList :execrows` supaya repository bisa mengecek jumlah baris terhapus
(0 baris = not found, tanpa query tambahan).

### 3. Query todo berfilter — bagian tersulit

Ini bagian task yang paling gampang salah. `ListTodos` (usecase task 06)
punya filter opsional (`status`, `listID`, `dueBefore`, `dueAfter`), sort
dinamis (tiga pilihan), dan keyset pagination. Tiga sumbu variasi itu tidak
bisa digabung jadi satu query statis sqlc kalau ditulis naif.

**Predikat opsional** — pola `sqlc.narg` supaya satu query menangani filter
ada/tidaknya tanpa cabang SQL:

```sql
-- name: QueryTodos :many
SELECT * FROM todos
WHERE user_id = $1
  AND (sqlc.narg('list_id')::uuid IS NULL OR list_id = sqlc.narg('list_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('due_before')::timestamptz IS NULL OR due_at < sqlc.narg('due_before'))
  AND (sqlc.narg('due_after')::timestamptz IS NULL OR due_at > sqlc.narg('due_after'))
  AND (
    sqlc.narg('cursor_created_at')::timestamptz IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT $2;
```

Predikat keyset `(created_at, id) < ($cursor_created_at, $cursor_id)` memakai
perbandingan tuple Postgres — ini yang membuat keyset pagination benar
walau ada baris dengan `created_at` yang sama persis (tie-break lewat `id`).
Tanpa tie-break kolom kedua, dua baris dengan timestamp identik bisa
terlewat atau terulang di halaman berikutnya.

**Kenapa sort dinamis TIDAK BOLEH dirakit lewat string concat:** menyusun
`ORDER BY " + sortColumn` dari input klien adalah SQL injection kalau
`sortColumn` bocor sedikit saja dari whitelist — bahkan tanpa niat jahat,
satu bug validasi yang terlewat membuka celah. Solusi yang dipakai di sini:
**query terpisah per urutan** (whitelist urutan di level Go, satu query sqlc
per opsi sort), karena hanya ada tiga pilihan sort (`due_at_asc`,
`priority_desc`, `created_desc`) dan itu jumlah yang wajar ditulis eksplisit.
Tulis tiga versi `QueryTodos` dengan nama beda
(`QueryTodosByCreatedDesc`, `QueryTodosByDueAtAsc`,
`QueryTodosByPriorityDesc`), masing-masing identik kecuali klausa `ORDER BY`
dan predikat keyset yang menyesuaikan kolom sort itu (keyset harus ikut kolom
sort, bukan selalu `created_at` — kalau sort `due_at_asc`, kursor jadi
`(due_at, id) > (cursor_due_at, cursor_id)`).

Alternatif yang disebut tapi tidak dipakai di sini: `ORDER BY CASE
WHEN $sort = 'due_at_asc' THEN due_at END ASC, CASE WHEN $sort = ... END
DESC` — bekerja tapi bikin planner Postgres kesulitan memakai index yang
tepat (index hanya berguna untuk `ORDER BY` kolom langsung, bukan hasil
`CASE`). Untuk skala project ini keduanya aman dari sisi keamanan; yang
dipilih dipilih karena performanya lebih dapat diprediksi.

Repository Go (`todo_repository.go`) memilih salah satu dari tiga query itu
berdasarkan `filter.Sort`, dengan `default` fallback ke `created_desc` kalau
nilai sort tidak dikenali — jangan biarkan nilai sort yang tidak valid lolos
ke SQL apa pun.

**Setiap query wajib punya predikat `user_id = $1`.** Ini bukan optimasi,
ini pertahanan lapis kedua: kalau ada bug di usecase yang lupa memvalidasi
kepemilikan sebelum memanggil repository, query tetap tidak akan pernah
mengembalikan atau mengubah baris milik user lain. Saat review sendiri
sebelum lanjut ke langkah berikut, grep semua file `.sql` di modul ini dan
pastikan tidak ada satu pun `SELECT`/`UPDATE`/`DELETE` tanpa `user_id` di
klausa `WHERE`.

### 4. Repository Go — ambil `Querier` dari context

Ikuti pola tx manager dari task 02: repository tidak membuka transaksi
sendiri, dan mengambil `Querier` (interface hasil sqlc, bisa `*pgxpool.Pool`
atau `pgx.Tx`) dari context supaya otomatis ikut transaksi kalau usecase
sedang membuka satu.

```go
type listRepository struct {
	getQuerier func(ctx context.Context) Querier
}

func (r *listRepository) Create(ctx context.Context, l *domain.List) error {
	q := r.getQuerier(ctx)
	row, err := q.CreateList(ctx, CreateListParams{...})
	if err != nil {
		return mapPgError(err)
	}
	*l = toDomainList(row)
	return nil
}
```

### 5. Pemetaan error Postgres → domain

```go
func mapPgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrListNotFound // atau ErrTodoNotFound, tergantung konteks pemanggil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			if pgErr.ConstraintName == "lists_user_id_name_key" {
				return domain.ErrListNameTaken
			}
		}
	}
	return fmt.Errorf("query: %w", err)
}
```

`pgx.ErrNoRows` tidak membedakan "list tidak ada" dari "todo tidak ada" —
tiap pemanggil di repository membungkusnya sendiri jadi error domain yang
tepat sesuai method mana yang dipanggil (`GetListByID` → `ErrListNotFound`,
`GetTodoByID` → `ErrTodoNotFound`), jangan taruh keputusan itu di
`mapPgError` yang generik. Nama constraint (`lists_user_id_name_key`) harus
dicocokkan dengan nama sungguhan yang dihasilkan migrasi task 05 — cek nama
constraint asli sebelum menyalin kode ini mentah-mentah.

### 6. Handler HTTP

Handler hanya: decode request, validasi bentuk (bukan aturan bisnis), ambil
`userID` dari context (`middleware.UserIDFromContext(ctx)`, dari task 04),
panggil usecase, tulis response lewat `httpx.Respond` / `problem.Write`.

```go
func (h *TodosHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	todoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		problem.Write(w, r, httpx.ErrBadRequest("invalid todo id"))
		return
	}
	todo, err := h.completeTodo.Execute(r.Context(), userID, todoID)
	if err != nil {
		problem.Write(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, toTodoResponse(todo))
}
```

**`PATCH` parsial — masalah klasik dan solusinya.** `UpdateTodo` (misal lewat
`PATCH /todos/{id}`) menerima body JSON di mana tiap field opsional. Masalah:
`{"notes": null}` (klien sengaja mengosongkan catatan) dan body yang sama
sekali tidak menyertakan `notes` (klien tidak mau mengubah catatan) berbeda
maknanya, tapi kalau field Go-nya `Notes string` biasa, `encoding/json` tidak
bisa membedakan keduanya — dua-duanya jadi string kosong setelah decode.

Solusi yang dipakai di sini: **field pointer** di DTO request.

```go
type UpdateTodoRequest struct {
	Title    *string    `json:"title"`
	Notes    *string    `json:"notes"`
	Priority *string    `json:"priority"`
	DueAt    *time.Time `json:"due_at"`
}
```

`Notes *string` yang `nil` berarti "tidak dikirim, jangan ubah". `Notes`
yang menunjuk ke string kosong (`""`) berarti "dikirim, kosongkan". Ini
membedakan "tidak dikirim" dari "dikirim null" **selama field itu sendiri
punya tipe pointer** — catatan pentingnya: `json.Unmarshal` men-set pointer
jadi `nil` baik untuk field yang tidak ada di body maupun field yang
eksplisit `null` di body. Kalau perbedaan "tidak ada di body" vs "eksplisit
null" itu sendiri perlu dibedakan (tidak dibutuhkan endpoint mana pun di
task ini, tapi sebutkan sebagai batas solusi), satu-satunya cara andal adalah
`map[string]json.RawMessage` atau tipe `Optional[T]` custom dengan
`UnmarshalJSON` sendiri yang melacak "apakah field ini muncul" — tidak
diperlukan di sini karena tidak ada field todo yang perlu tiga keadaan
sekaligus (unset / null / value); dua keadaan (unset vs ada nilai) cukup
untuk semua field todo di scope PRD ini.

Handler meneruskan pointer itu apa adanya ke usecase (yang sudah menerima
bentuk pointer sejak task 06) — jangan dereference lalu isi ulang jadi
`""` di handler, itu menghilangkan informasi "tidak dikirim" sebelum sampai
usecase.

### 7. `internal/modules/task/module.go`

```go
package task

type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	IDGen  func() (uuid.UUID, error)
}

type Module struct {
	routes http.Handler
}

func New(deps Deps) *Module {
	// rakit repository, usecase, handler; bungkus jadi router chi
}

func (m *Module) Routes() http.Handler {
	return m.routes
}
```

Sesuai ADR-002: hanya file ini yang boleh dilihat dari luar `internal/modules/task/`.

### 8. Wiring `cmd/api/main.go`

```go
taskModule := task.New(task.Deps{Pool: pool, Clock: clock.System{}, IDGen: id.New})

r.Route("/v1", func(r chi.Router) {
	r.Use(middleware.Auth(identityModule.Authenticator()))
	r.Mount("/", taskModule.Routes())
})
```

Modul `task` dipasang **di belakang** middleware auth — semua endpoint list
dan todo wajib token valid. Route publik (`/healthz`, `/auth/*`) tetap di
luar grup ini.

### 9. Integration test

Build tag `//go:build integration`, testcontainers-go, migrasi goose
dijalankan sekali di awal paket (`TestMain` atau helper setup bersama —
ikuti pola yang sudah dipakai modul `identity` kalau sudah ada dari task
sebelumnya).

Wajib:
- **CRUD penuh lewat HTTP** untuk list dan todo: create → get → update →
  delete, masing-masing mengecek status code dan body sesuai
  `openapi.yaml`.
- **Pagination menembus lebih dari satu halaman.** Buat lebih banyak todo
  dari satu `limit` (misal `limit=5`, buat 12 todo), tarik halaman berturut
  lewat cursor, kumpulkan semua ID yang didapat, dan assert: tidak ada ID
  duplikat, tidak ada ID yang hilang dibanding total yang dibuat, urutan
  konsisten dengan sort yang diminta.
- **Isolasi user.** User A membuat list dan todo. User B mencoba
  `GET`/`PATCH`/`DELETE` resource milik A lewat ID yang benar (bukan ID
  acak) — harus **404**, bukan 403 dan bukan 200. Uji ini untuk list dan
  todo, dan untuk `MoveTodo` (user B mencoba memindahkan todo A ke list B,
  atau memindahkan todo B ke list A — dua-duanya harus gagal dengan not
  found yang tepat).

## Kriteria selesai

```bash
cd apps/go-chi-api
make sqlc
go build ./...
go vet ./...
make lint
make test
make test-integration
make openapi-lint
```

- Semua di atas hijau.
- `curl -s -X POST localhost:8080/v1/lists -H "Authorization: Bearer $TOKEN" -d '{"name":"Kerja"}'`
  (server jalan via `make run`, sudah login) mengembalikan 201 dengan body
  sesuai `openapi.yaml`.
- Integration test isolasi user (butir wajib di langkah 9) ada dan lulus —
  grep nama testnya untuk memastikan bukan cuma ditulis tapi tidak
  dijalankan.
- `grep -rn "SELECT\|UPDATE\|DELETE" internal/modules/task/internal/adapter/postgres/queries/*.sql`
  ditinjau manual: setiap baris punya `user_id` di predikatnya (kecuali
  `INSERT` yang menyertakan `user_id` sebagai kolom).
- `git diff --stat` untuk `sqlc.yaml` menunjukkan entry baru untuk `task`,
  bukan menimpa entry `identity`.

## Jebakan

- Query hasil sqlc di-generate ulang (`make sqlc`) tiap `queries/*.sql`
  berubah — lupa menjalankan ulang membuat kode Go lama tidak sinkron dengan
  SQL, dan errornya baru muncul di compile time dengan pesan yang
  membingungkan (field hilang) bukan pesan yang menunjuk ke SQL.
- Tiga query sort terpisah gampang divergen diam-diam (satu diperbaiki, dua
  lainnya lupa) — kalau menambah predikat filter baru di masa depan, ubah
  ketiganya sekaligus, dan pertimbangkan test yang membandingkan bahwa
  ketiganya menghasilkan jumlah baris sama untuk filter yang sama (hanya
  urutan yang beda).
- `mapPgError` yang mengembalikan `ErrTodoNotFound` untuk kasus yang
  sebenarnya "list tidak ditemukan" (atau sebaliknya) membuat pesan error ke
  klien membingungkan meski status code-nya tetap benar (sama-sama 404) —
  tinjau tiap pemanggilan, jangan biarkan satu fungsi pemetaan generik
  menebak jenis resource.
- Cookie refresh (task 04) tidak relevan di modul ini — endpoint task hanya
  butuh access token `Authorization: Bearer`. Jangan menambah logika cookie
  apa pun di sini.
- Field pointer di `UpdateTodoRequest` yang di-dereference terlalu dini di
  handler (sebelum sampai usecase) menghilangkan sinyal "tidak dikirim" —
  lihat catatan di langkah 6.
- Integration test yang membagi baris database yang sama antar test (tidak
  membersihkan atau tidak memberi data unik per test) akan flaky begitu
  dijalankan paralel — ikuti aturan `AGENTS.md`: test tidak berbagi baris
  dengan test lain.
