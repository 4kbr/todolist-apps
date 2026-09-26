# Task 05 — Adapter identity: Postgres, HTTP, wiring

**Phase:** 3 — Identity
**Bergantung pada:** 04
**Status:** belum dikerjakan

## Tujuan

Menyambungkan domain/usecase identity (task 04) ke Postgres dan HTTP, sesuai
kontrak `docs/openapi/openapi.yaml` (task 03). Setelah task ini, kelima
endpoint auth benar-benar bisa dipanggil lewat `docker compose up` + `curl`.

## Rujukan

- `apps/go-chi-api/docs/ARCHITECTURE.md` — bentuk `module.go`, aliran satu
  request, pemetaan error → `platform/problem`, pola `Querier` dari context
  (task 02 — baca file `internal/platform/postgres/` hasil task 02 sebelum
  menulis repository).
- `apps/go-chi-api/docs/DECISIONS.md` — ADR-002 (module.go), ADR-003
  (interface dideklarasikan konsumen), ADR-006 (cookie refresh), ADR-007/008
  (sqlc per modul), ADR-013 (integration test testcontainers).
- `apps/go-chi-api/docs/tasks/04-identity-domain.md` — signature usecase,
  DTO, port repository yang diimplementasikan di sini.
- `docs/openapi/openapi.yaml` / `bundle.yaml` — bentuk request/response
  persis, status code persis. Response HARUS cocok; kalau tidak cocok,
  handler yang diperbaiki, bukan spec (kecuali lewat prosedur version bump
  di task 03).
- `apps/go-chi-api/AGENTS.md` — migrasi lewat `make migrate-create`, tidak boleh
  disunting setelah commit; `make sqlc` setelah ubah query; handler cuma
  decode/validate/panggil usecase/respond.

## File yang dibuat atau disentuh

```
internal/modules/identity/internal/adapter/postgres/
  queries/users.sql
  queries/sessions.sql
  <hasil sqlc generate, paket internal ke modul ini>
  user_repository.go
  session_repository.go
  user_repository_test.go       (//go:build integration)
  session_repository_test.go    (//go:build integration)

internal/modules/identity/internal/adapter/http/
  handler.go
  register.go
  login.go
  refresh.go
  logout.go
  me.go
  dto.go                          request/response struct HTTP (beda dari DTO usecase task 04)
  handler_test.go                 (//go:build integration, httptest + Postgres asli)

internal/platform/middleware/
  auth.go                         SUDAH ada dari task 02 - hanya diverifikasi, bukan dibuat ulang
  auth_test.go

internal/modules/identity/module.go

cmd/api/main.go            (wiring: tambah pemanggilan identity.New(...))
```

## Langkah

### 1. Pastikan migrasi sudah jalan

Tabel `users` dan `sessions` **sudah dibuat di task 01**. Jangan membuat migrasi
baru di sini dan jangan menyunting migrasi yang sudah ada.

```bash
make migrate-status    # users dan sessions harus tercatat applied
```

Kalau ternyata ada kolom yang kurang, itu migrasi **baru** — bukan suntingan
migrasi lama (lihat `AGENTS.md`).

Satu hal yang perlu diingat dari task 01: kolom `id` tidak punya
`DEFAULT gen_random_uuid()`. ID dibuat di Go (UUIDv7, ADR-009) dan dikirim
sebagai parameter insert, supaya generator ID tetap satu tempat
(`platform/id`) dan tidak tersebar antara Go dan SQL.

### 2. Query sqlc

`queries/users.sql`:

```sql
-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: FindUserByEmail :one
SELECT id, email, password_hash, name, created_at, updated_at
FROM users
WHERE email = $1;

-- name: FindUserByID :one
SELECT id, email, password_hash, name, created_at, updated_at
FROM users
WHERE id = $1;
```

`queries/sessions.sql`:

```sql
-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FindSessionByRefreshTokenHash :one
SELECT id, user_id, refresh_token_hash, user_agent, ip, expires_at, revoked_at, created_at
FROM sessions
WHERE refresh_token_hash = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = $2 WHERE id = $1;

-- name: RevokeAllSessionsForUser :exec
UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL;
```

Tambahkan entry di `sqlc.yaml` root untuk modul `identity` kalau belum ada
dari task 02 (mengarah ke direktori `queries/` di atas, output ke paket
`internal/modules/identity/internal/adapter/postgres`, ADR-008). Jalankan
`make sqlc` dan commit hasil generate.

### 3. Repository — ambil `Querier` dari context

Ikuti pola dari task 02: pool/tx disimpan di context oleh
`platform/postgres`, repository memanggil helper (mis.
`postgres.QuerierFromContext(ctx)`) untuk mendapat `Querier` yang benar
(pool biasa di luar transaksi, tx di dalam `UoW.Do`). **Jangan** repository
menyimpan `*pgxpool.Pool` sebagai field dan memakainya langsung — itu
melewati transaksi yang dibuka usecase.

```go
type UserRepository struct {
    // tidak menyimpan pool langsung; Querier diambil per-panggilan dari ctx
}

func (r *UserRepository) Create(ctx context.Context, u domain.User) error {
    q := postgres.QuerierFromContext(ctx)
    err := q.CreateUser(ctx, generated.CreateUserParams{ /* ... */ })
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return domain.ErrEmailTaken
        }
        return fmt.Errorf("create user: %w", err)
    }
    return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
    q := postgres.QuerierFromContext(ctx)
    row, err := q.FindUserByEmail(ctx, email.String())
    if errors.Is(err, pgx.ErrNoRows) {
        return domain.User{}, domain.ErrUserNotFound
    }
    if err != nil {
        return domain.User{}, fmt.Errorf("find user by email: %w", err)
    }
    return mapUser(row), nil
}
```

Pemetaan error Postgres → domain WAJIB memeriksa `*pgconn.PgError` lewat
`errors.As`, membandingkan `pgErr.Code == "23505"` — **bukan**
`strings.Contains(err.Error(), "duplicate key")`. Kode error Postgres
(`23505` = `unique_violation`) adalah kontrak stabil dari Postgres sendiri
di semua versi dan semua locale; pesan teksnya bisa berubah antar versi
Postgres atau berbeda tergantung bahasa server, jadi mencocokkan string
gampang diam-diam berhenti bekerja setelah upgrade Postgres tanpa ada yang
sadar sampai bug muncul di produksi.

`pgx.ErrNoRows` dipetakan ke error not-found domain yang sesuai
(`ErrUserNotFound` di `UserRepository`, `ErrSessionNotFound` di
`SessionRepository`) — jangan biarkan `pgx.ErrNoRows` mentah bocor ke lapisan
di atas `adapter/postgres`.

### 4. Handler HTTP — hanya decode/validate/panggil/respond

Contoh `register.go`:

```go
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    var req registerRequest
    if err := httpx.Decode(r, &req); err != nil {
        problem.Write(w, r, err)
        return
    }
    if err := req.Validate(); err != nil {
        problem.Write(w, r, err)
        return
    }
    out, err := h.register.Execute(r.Context(), application.RegisterInput{
        Email:    req.Email,
        Password: req.Password,
        Name:     req.Name,
    })
    if err != nil {
        problem.Write(w, r, err) // problem.Write memetakan domain.ErrEmailTaken -> 409, dst
        return
    }
    httpx.Respond(w, http.StatusCreated, registerResponse{UserID: out.UserID})
}
```

Tidak ada `if` aturan bisnis di sini (mis. tidak ada pengecekan email
duplikat manual) — itu sudah terjadi di usecase task 04. Handler murni
plumbing.

### 5. Cookie refresh token

Set saat login dan refresh:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "refresh_token",
    Value:    out.RefreshToken,
    Path:     "/auth/refresh",
    HttpOnly: true,
    Secure:   cfg.AppEnv != "local",
    SameSite: http.SameSiteStrictMode,
    MaxAge:   int(cfg.RefreshTokenTTL.Seconds()),
})
```

Hapus saat logout:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "refresh_token",
    Value:    "",
    Path:     "/auth/refresh",
    HttpOnly: true,
    Secure:   cfg.AppEnv != "local",
    SameSite: http.SameSiteStrictMode,
    MaxAge:   -1,
})
```

`Secure` hanya boleh dimatikan (`false`) ketika `APP_ENV=local`, dan itu
dibaca dari config (task 00/01), bukan hardcode. Alasannya: browser modern
menolak mengirim ulang cookie `Secure` lewat koneksi `http://` biasa —
development lokal seringkali jalan di `http://localhost` tanpa TLS, jadi
kalau `Secure` selalu `true`, cookie refresh tidak akan pernah tersimpan
saat development dan login lokal terlihat "rusak" padahal sengaja diblokir
browser. Di `staging`/`production`, `APP_ENV` bukan `local`, jadi `Secure`
otomatis `true` dan tidak ada jalan untuk lupa mengaktifkannya.

### 6. Middleware auth (interface dideklarasikan sendiri, ADR-003)

```go
// internal/platform/middleware/auth.go
package middleware

type TokenVerifier interface {
    VerifyAccessToken(raw string) (userID uuid.UUID, err error)
}

type contextKey int

const userIDKey contextKey = iota

func Auth(v TokenVerifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
            if raw == "" {
                problem.Write(w, r, ErrMissingToken)
                return
            }
            userID, err := v.VerifyAccessToken(raw)
            if err != nil {
                problem.Write(w, r, ErrInvalidToken)
                return
            }
            ctx := context.WithValue(r.Context(), userIDKey, userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
    id, ok := ctx.Value(userIDKey).(uuid.UUID)
    return id, ok
}
```

`platform/middleware` TIDAK import apa pun dari
`internal/modules/identity/...`. `identity.Module` cocok dengan
`TokenVerifier` secara struktural (Go interface implisit) tanpa modul
identity tahu middleware ini ada, dan tanpa middleware tahu identity ada —
`cmd/api/main.go` yang menyambungkan keduanya. Ini yang membuat middleware
bisa dites (task ini, langkah 9) dengan `TokenVerifier` palsu tanpa modul
identity sama sekali, dan yang mencegah import cycle kalau suatu saat
modul lain juga perlu diverifikasi lewat middleware yang sama.

### 7. `module.go`

```go
package identity

type Deps struct {
    Pool   *pgxpool.Pool
    Clock  clock.Clock
    Config Config // JWT secret, access/refresh TTL, dari platform/config
}

type Module struct {
    routes http.Handler
    auth   *jwtauth.Verifier
}

func New(deps Deps) *Module {
    // rakit repository, hasher, issuer, usecase, handler, router di sini
}

func (m *Module) Routes() http.Handler { return m.routes }

func (m *Module) Authenticator() middleware.TokenVerifier { return m.auth }
```

Ini SATU-SATUNYA file di akar modul (ADR-002). Semua perakitan (`New`)
terjadi di sini, bukan di `cmd/api/main.go` — `main.go` cuma memanggil
`identity.New(deps)` dan memasang hasilnya ke router utama.

### 8. Wiring `cmd/api/main.go`

```go
identityModule := identity.New(identity.Deps{
    Pool:   pool,
    Clock:  clock.Real{},
    Config: identity.Config{ /* dari platform/config */ },
})

router.Mount("/auth", identityModule.Routes())

protected := router.With(middleware.Auth(identityModule.Authenticator()))
// endpoint task/list/todo (task 06-09) dipasang di sini nantinya
```

### 9. Integration test

Build tag `//go:build integration`, testcontainers-go menyalakan Postgres,
migrasi goose dijalankan sebelum test (rujuk ADR-013 dan pola dari task 01/02
untuk helper penyalaan container test).

Kasus wajib di `handler_test.go`:

1. **Alur penuh** register → login → refresh → logout lewat HTTP asli
   (`httptest.Server` atau `httptest.NewRequest` ke router identity):
   - register → `201`
   - login → `200`, `accessToken` ada, cookie `refresh_token` ter-set.
   - panggil endpoint terproteksi (`GET /auth/me`) dengan access token →
     `200`, `email` cocok.
   - refresh (kirim cookie dari login) → `200`, access token baru berbeda
     dari yang lama, cookie refresh baru berbeda dari yang lama.
   - logout → `204`, cookie di response `Set-Cookie` punya `Max-Age=0`
     atau tanggal lampau (hasil dari `MaxAge: -1`).
2. **Akses endpoint terproteksi tanpa token** → `401`, body
   `application/problem+json` sesuai schema `Problem`.
3. **Akses dengan access token kedaluwarsa** — pakai `clock.Fixed` yang
   dimajukan melewati TTL, atau terbitkan token dengan `exp` di masa lalu
   langsung lewat issuer → `401`.
4. **Reuse refresh token yang sudah dicabut** — login, refresh sekali
   (dapat token baru + token lama sekarang revoked), lalu coba refresh
   LAGI pakai cookie/token LAMA yang sudah dicabut → error, DAN verifikasi
   lewat query langsung ke `sessions` bahwa SEMUA session user tersebut
   sekarang `revoked_at IS NOT NULL`.

`auth_test.go` untuk middleware bisa unit test biasa (tanpa Postgres) pakai
`TokenVerifier` palsu — tidak perlu build tag `integration`.

### 10. Cocokkan dengan OpenAPI

Untuk tiap endpoint, bandingkan manual field request/response dan status
code terhadap `docs/openapi/bundle.yaml` dari task 03. Kalau ada field yang
beda nama (mis. usecase pakai `UserID` tapi spec bilang `id`), atau status
code beda (spec bilang `201` tapi handler menulis `200`), **perbaiki
handler**. Jangan mengubah `docs/openapi/` dari task ini — itu keluar dari
path yang jadi tanggung jawab task ini, dan mengubah kontrak butuh prosedur
version bump tersendiri (task 03 langkah 10).

## Kriteria selesai

```bash
cd apps/go-chi-api
make lint
make test                 # unit, cepat, tanpa Docker
make test-integration      # testcontainers, kasus 1-4 di langkah 9 hijau
make openapi-lint          # kontrak masih valid (tidak disentuh task ini)
```

- `go build ./...` sukses dari `cmd/api`.
- `curl -X POST localhost:PORT/auth/register -d '{...}'` (lewat
  `docker compose up`, manual, bukan bagian CI) mengembalikan `201` dengan
  body sesuai `openapi.yaml`.
- `grep -n "identity" internal/platform/middleware/auth.go` tidak ada hasil
  (middleware tidak import identity).
- `grep -rn "strings.Contains" internal/modules/identity/internal/adapter/postgres/`
  tidak dipakai untuk mendeteksi error Postgres (harus `*pgconn.PgError` +
  `errors.As`).
- Test reuse detection (langkah 9 kasus 4) membuktikan SEMUA session user
  tercabut, bukan cuma satu.
- Response tiap endpoint dicek cocok field-by-field dan status-code-by-status-code
  dengan `docs/openapi/bundle.yaml`.

## Jebakan

- Jangan menaruh `*pgxpool.Pool` sebagai field langsung di repository dan
  memakainya tanpa lewat context — itu melewati transaksi `UoW.Do` dari
  usecase `Refresh` di task 04, dan reuse-detection test akan gagal secara
  tidak kentara (kadang lolos kadang tidak, tergantung race).
- Jangan mendeteksi `unique_violation` dengan mencocokkan pesan teks error.
- Jangan biarkan `Secure` cookie selalu `true` (login lokal jadi terlihat
  rusak) atau selalu `false` (bug keamanan di staging/production) — harus
  ikut `APP_ENV`.
- Jangan menambah interface `TokenVerifier` kedua di modul identity untuk
  dipakai middleware — middleware sudah mendeklarasikan interface-nya
  sendiri (langkah 6); modul identity cukup memenuhi bentuknya secara
  implisit.
- Jangan mengubah `docs/openapi/openapi.yaml`/`bundle.yaml` dari task ini.
  Kalau tersandung ketidakcocokan yang menurutmu berarti spec-nya yang
  salah, catat sebagai temuan dan ajukan lewat prosedur version bump —
  jangan diam-diam menimpa file kontrak.
- Jangan lupa `revoked_at IS NULL` di query `RevokeAllSessionsForUser` —
  tanpa filter itu, query akan menulis ulang `revoked_at` session yang
  sebelumnya sudah dicabut lebih awal dengan timestamp baru, menghapus
  jejak kapan sebenarnya sesi itu pertama kali dicabut.
