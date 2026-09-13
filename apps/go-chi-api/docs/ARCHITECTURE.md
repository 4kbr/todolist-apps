# Arsitektur — apps/go-chi-api

Dokumen ini menjelaskan **bentuk** kodenya. Alasan di balik tiap pilihan ada di
[`DECISIONS.md`](DECISIONS.md).

## Bentuk besar

Modular monolith. Satu proses, satu database, batas modul ditegakkan oleh
compiler Go lewat direktori `internal/` bertingkat.

Dua modul domain:

| Modul      | Isi                            |
| ---------- | ------------------------------ |
| `identity` | User, session, login, refresh  |
| `task`     | List sebagai agregat, todo     |

Di luar itu ada `platform` — kode lintas modul yang tidak punya domain sama
sekali (config, logger, pool DB, helper HTTP, middleware).

## Pohon direktori

```
cmd/api/main.go                  wiring: config → logger → pool → modul → router → serve
internal/
  platform/
    config/                      env → struct, divalidasi saat start
    logger/                      slog JSON, diperkaya request id
    postgres/                    pool pgx, tx manager (Unit of Work)
    httpx/                       decode, validate, respond, parse query pagination
    problem/                     error RFC 9457 + pemetaan error domain → HTTP
    middleware/                  request id, recover, access log, CORS, timeout
    id/                          UUIDv7
    clock/                       interface waktu, supaya usecase bisa di-test
  modules/
    identity/
      module.go                  SATU-SATUNYA pintu keluar modul
      internal/
        domain/                  entity, error domain, port repository
        application/             usecase, DTO, orkestrasi transaksi
        adapter/
          postgres/              queries/*.sql, hasil sqlc, implementasi port
          http/                  handler, request/response struct
    task/
      module.go
      internal/{domain,application,adapter/{postgres,http}}
db/migrations/                   goose, satu schema untuk semua modul
sqlc.yaml                        satu file, satu entry per modul
docs/                            ARCHITECTURE, DECISIONS, HANDS-OFF, openapi, tasks
```

## Kenapa `internal/` bertingkat

Go melarang paket di luar `internal/modules/identity/` meng-import apa pun di
`internal/modules/identity/internal/...`. Artinya modul `task` **tidak bisa**
menyentuh isi dalam `identity` walaupun mau — `go build` yang menolak.

Batas modul di sini bukan konvensi yang bisa luntur. Dia dipaksakan toolchain.

Satu-satunya file yang terlihat dari luar modul adalah `module.go`.

## `module.go`

Namanya `module.go`, bukan `factory.go`, karena isinya bukan sekadar bikin
objek. Dia kontrak publik modul:

```go
package identity

// Module adalah pintu keluar modul identity.
type Module struct {
    routes http.Handler
    auth   *application.TokenVerifier
}

func New(deps Deps) *Module

// Routes mengembalikan subrouter modul, dipasang main.go ke router utama.
func (m *Module) Routes() http.Handler

// Authenticator dipakai middleware auth di luar modul ini.
func (m *Module) Authenticator() Authenticator
```

`Deps` berisi apa yang modul butuh dari luar: pool, logger, clock, config.
Disuntikkan `main.go`, bukan diambil sendiri dari variabel global.

## Arah dependency

```
adapter  →  application  →  domain
```

Selalu menunjuk ke dalam, tidak pernah terbalik.

- `domain` tidak import apa pun dari modul ini selain dirinya. Tidak tahu
  Postgres, tidak tahu HTTP, tidak tahu Chi.
- `domain` mendeklarasikan port repository sebagai **interface**.
- `application` memakai port itu, tidak pernah tahu implementasinya.
- `adapter/postgres` mengimplementasikan port. `adapter/http` memanggil usecase.

Efeknya: usecase bisa di-unit test dengan fake repository, tanpa Docker dan
tanpa database.

## Komunikasi antar modul

Aturannya: **modul yang butuh sesuatu mendeklarasikan interface-nya sendiri**,
lalu `main.go` menyuntikkan implementasi. Ini idiom Go "accept interfaces,
return structs", dan ini yang mencegah import cycle dua arah.

Di MVP hanya ada satu kopling nyata: router butuh memverifikasi access token.

```go
// internal/platform/middleware/auth.go
type TokenVerifier interface {
    VerifyAccessToken(ctx context.Context, raw string) (userID uuid.UUID, err error)
}

func Auth(v TokenVerifier) func(http.Handler) http.Handler
```

`identity.Module` kebetulan memenuhi interface itu. Middleware tidak pernah
import `identity`.

**Modul `task` tidak import `identity` sama sekali.** `userID` datang dari
context hasil middleware, dan integritas data dijaga foreign key di database.
Kopling tidak dikarang supaya terlihat modular.

## Aliran satu request

```
HTTP  →  middleware (request id, log, recover, timeout, auth)
      →  adapter/http handler        decode, validate, panggil usecase
      →  application usecase         aturan orkestrasi, buka transaksi kalau perlu
      →  domain                      aturan bisnis, keputusan valid/tidak
      →  adapter/postgres repo       SQL lewat sqlc
      ←  error domain
      ←  problem.Write               error domain dipetakan ke problem+json
```

Handler tidak berisi aturan bisnis. Usecase tidak tahu HTTP. Domain tidak tahu
apa pun kecuali dirinya.

## Error

Error domain didefinisikan di `domain` sebagai nilai sentinel atau tipe:

```go
var ErrListNotFound = errors.New("list not found")
```

Satu tempat memetakannya ke HTTP: `platform/problem`. Handler cukup memanggil
`problem.Write(w, r, err)`. Kalau error tidak dikenali, jadi 500 dan yang
tercatat di log adalah error aslinya — yang keluar ke klien tidak pernah memuat
detail internal.

Format response mengikuti RFC 9457 `application/problem+json`.

## Transaksi

Usecase yang menyentuh lebih dari satu repository membuka transaksi lewat tx
manager di `platform/postgres`:

```go
err := uow.Do(ctx, func(ctx context.Context) error {
    if err := repo.RevokeSession(ctx, old.ID); err != nil { return err }
    return repo.CreateSession(ctx, next)
})
```

Transaksi dibawa di `context`, jadi repository tidak perlu tahu dia sedang di
dalam transaksi atau tidak. Repository tidak pernah membuka transaksi sendiri —
itu keputusan usecase.

## Database

Satu database, satu direktori migrasi (`db/migrations/`, goose). Schema milik
bersama; pemisahan modul ada di kode, bukan di schema.

Query ditulis sebagai SQL asli di `internal/modules/<modul>/internal/adapter/postgres/queries/`,
lalu `sqlc` menghasilkan kode Go type-safe ke paket yang sama. Jadi kode hasil
generate ikut terkurung di dalam modulnya — tidak ada paket query bersama yang
diam-diam menembus batas modul.

### Tabel

`users` — `id uuid pk`, `email citext unique`, `password_hash`, `name`, timestamps

`sessions` — `id`, `user_id fk cascade`, `refresh_token_hash bytea unique`,
`user_agent`, `ip inet`, `expires_at`, `revoked_at`, `created_at`

`lists` — `id`, `user_id fk cascade`, `name`, `position`, timestamps,
unique `(user_id, name)`

`todos` — `id`, `list_id fk cascade`, `user_id fk cascade`, `title`, `notes`,
`status` (`todo` / `done`, CHECK), `priority`, `due_at`, `completed_at`, timestamps

Index: `sessions(user_id)`, `todos(user_id, status, due_at)`,
`todos(list_id, created_at desc, id desc)`.

## Autentikasi

Access token JWT umur pendek + refresh token opaque yang disimpan di database.

1. **Register** — password di-hash argon2id.
2. **Login** — terbitkan access JWT HS256 (15 menit, claim `sub`/`jti`/`iat`/`exp`)
   dan refresh token acak 32 byte. Yang disimpan di `sessions` adalah **SHA-256
   dari refresh token**, bukan tokennya. Refresh dikirim sebagai cookie
   `HttpOnly; Secure; SameSite=Strict; Path=/auth/refresh`.
3. **Refresh** — rotasi: session lama dicabut, yang baru diterbitkan, dalam satu
   transaksi.
4. **Reuse detection** — kalau refresh token yang sudah dicabut dipakai lagi,
   seluruh session milik user itu dicabut. Itu tanda token bocor.
5. **Logout** — cabut session, hapus cookie.

Access token tidak bisa dicabut sebelum kedaluwarsa — itu memang harga JWT.
Umurnya dibuat pendek supaya jendela itu sempit.

## Testing

| Lapis                   | Cara                                          |
| ----------------------- | --------------------------------------------- |
| `domain`                | unit test murni, tanpa mock                   |
| `application`           | unit test, fake repository + clock palsu      |
| `adapter/postgres`      | integration, Postgres asli via testcontainers |
| `adapter/http`          | integration, `httptest` + Postgres asli       |

Fake repository ditulis tangan sebagai struct biasa, bukan hasil generate mock.
Lebih mudah dibaca dan tidak menambah tool.

Integration test dijaga build tag supaya `make test` tetap cepat:

```go
//go:build integration
```

## Yang belum diimplementasikan

Fungsi yang belum jadi sengaja `panic` dengan pesan yang menunjuk file task-nya:

```go
panic("belum diimplementasikan - lihat docs/tasks/06-task-domain.md")
```

Jangan dihapus massal. Itu penanda pekerjaan, bukan kelalaian.
