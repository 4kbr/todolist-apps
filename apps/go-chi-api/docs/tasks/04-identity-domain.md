# Task 04 — Domain dan usecase identity

**Phase:** 3 — Identity
**Bergantung pada:** 03
**Status:** belum dikerjakan

## Tujuan

Membangun modul `identity` lapis `domain` dan `application` secara penuh,
tanpa database dan tanpa HTTP. Semuanya harus bisa di-unit test dengan fake
repository tulis tangan. Task 05 baru menyambungkannya ke Postgres dan Chi.

## Rujukan

- `apps/go-chi-api/docs/ARCHITECTURE.md` — arah dependency `adapter →
  application → domain`, skema tabel `users`/`sessions`, alur auth lengkap.
- `apps/go-chi-api/docs/DECISIONS.md` — ADR-005 (JWT + refresh opaque),
  ADR-006 (transport token), ADR-009 (UUIDv7).
- `apps/go-chi-api/AGENTS.md` — `domain` tidak boleh import Chi/pgx/sqlc/net/http;
  error dibungkus konteks; repository sebagai interface di `domain`.
- `docs/openapi/openapi.yaml` (hasil task 03) — bentuk request/response
  `/auth/*`, supaya DTO usecase tidak meleset dari kontrak nanti di task 05.

## File yang dibuat atau disentuh

```
internal/modules/identity/internal/domain/
  user.go                  entity User
  session.go               entity Session + method IsActive/IsRevoked
  errors.go                error sentinel
  email.go                 value object Email
  repository.go            interface UserRepository, SessionRepository
  user_test.go / session_test.go / email_test.go

internal/modules/identity/internal/application/
  ports.go                 interface PasswordHasher, TokenIssuer, TokenVerifier, UnitOfWork
  register.go / register_test.go
  login.go / login_test.go
  refresh.go / refresh_test.go
  logout.go / logout_test.go
  me.go / me_test.go
  dto.go                   input/output tiap usecase

internal/modules/identity/internal/application/argon2/
  hasher.go                implementasi PasswordHasher, argon2id
  hasher_test.go

internal/modules/identity/internal/application/jwtauth/
  issuer.go                implementasi TokenIssuer/TokenVerifier, JWT HS256
  issuer_test.go

internal/modules/identity/internal/application/refreshtoken/
  generator.go             crypto/rand 32 byte + SHA-256 hash
  generator_test.go
```

Tidak menyentuh `adapter/`, tidak menyentuh `module.go` (itu task 05). Tidak
menjalankan `go` atau `git` dari dokumen ini — task ini dieksekusi di sesi
lain saat implementasi, dokumen ini hanya menuliskan rencana dan kriterianya.

## Langkah

### 1. Entity domain

```go
// domain/user.go
type User struct {
    ID           uuid.UUID
    Email        Email
    PasswordHash string
    Name         string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

```go
// domain/session.go
type Session struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    RefreshTokenHash []byte
    UserAgent        string
    IP               string
    ExpiresAt        time.Time
    RevokedAt        *time.Time
    CreatedAt        time.Time
}

func (s Session) IsRevoked() bool {
    return s.RevokedAt != nil
}

func (s Session) IsActive(now time.Time) bool {
    return !s.IsRevoked() && now.Before(s.ExpiresAt)
}
```

`IsActive`/`IsRevoked` dipakai usecase `Refresh` supaya keputusan "boleh
dipakai atau tidak" adalah aturan domain, bukan `if` yang bertebaran di
usecase.

### 2. Value object `Email`

```go
// domain/email.go
type Email struct{ value string }

func NewEmail(raw string) (Email, error) {
    trimmed := strings.ToLower(strings.TrimSpace(raw))
    if trimmed == "" || !strings.Contains(trimmed, "@") {
        return Email{}, fmt.Errorf("new email: %w", ErrInvalidEmail)
    }
    return Email{value: trimmed}, nil
}

func (e Email) String() string { return e.value }
```

Normalisasi (trim + lowercase) terjadi di konstruktor domain, **bukan** di
handler HTTP, karena `Email` dipakai juga oleh test unit usecase dan
(potensial) proses lain yang tidak lewat HTTP sama sekali (mis. seed data,
job admin). Kalau normalisasi ditaruh di handler, tiap pemanggil baru wajib
mengingat untuk menormalisasi ulang — sumber bug "duplicate user karena
`Foo@x.com` vs `foo@x.com`" kalau ada satu jalur yang lupa. Invariant milik
value object, ditegakkan sekali di titik pembuatannya.

### 3. Error sentinel

```go
// domain/errors.go
var (
    ErrInvalidEmail       = errors.New("invalid email")
    ErrEmailTaken         = errors.New("email already taken")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrUserNotFound       = errors.New("user not found")
    ErrSessionNotFound    = errors.New("session not found")
    ErrSessionExpired     = errors.New("session expired")
    ErrSessionRevoked     = errors.New("session revoked")
)
```

`ErrInvalidCredentials` dipakai untuk DUA kasus berbeda: email tidak
terdaftar, dan email ada tapi password salah. Usecase `Login` **tidak
pernah** membedakan keduanya ke pemanggil. Kalau dibedakan (mis.
`ErrUserNotFound` untuk kasus pertama), respons API otomatis membocorkan
apakah sebuah email terdaftar — penyerang tinggal mencoba banyak email dan
membaca perbedaan status/pesan untuk memetakan akun mana yang ada
(enumerasi akun). Satu error, satu pesan, satu status code, tidak peduli
mana dari dua kasus yang sebenarnya terjadi.

### 4. Port repository (interface di domain)

```go
// domain/repository.go
type UserRepository interface {
    Create(ctx context.Context, u User) error
    FindByEmail(ctx context.Context, email Email) (User, error)
    FindByID(ctx context.Context, id uuid.UUID) (User, error)
}

type SessionRepository interface {
    Create(ctx context.Context, s Session) error
    FindByRefreshTokenHash(ctx context.Context, hash []byte) (Session, error)
    Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
    RevokeAllForUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
}
```

`FindByEmail`/`FindByRefreshTokenHash` yang tidak ketemu mengembalikan
`ErrUserNotFound`/`ErrSessionNotFound` masing-masing — pemetaan dari
`pgx.ErrNoRows` terjadi di adapter (task 05), domain hanya kenal sentinel-nya.

### 5. Port application (hasher, token, transaksi)

```go
// application/ports.go
type PasswordHasher interface {
    Hash(password string) (string, error)
    Verify(password, encodedHash string) (bool, error)
}

type TokenIssuer interface {
    IssueAccessToken(userID uuid.UUID) (token string, expiresAt time.Time, err error)
}

type TokenVerifier interface {
    VerifyAccessToken(token string) (userID uuid.UUID, err error)
}

type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}
```

`UnitOfWork` di sini adalah interface yang dideklarasikan modul `identity`
sendiri (bukan import dari `platform/postgres` langsung ke usecase) — pola
sama seperti ADR-003, supaya usecase tetap bisa di-unit-test dengan
`UnitOfWork` palsu yang cuma menjalankan `fn` tanpa transaksi sungguhan.
Implementasi asli disuntik dari `platform/postgres` di `module.go` (task 05).

### 6. DTO input/output terpisah dari entity

```go
// application/dto.go
type RegisterInput struct {
    Email    string
    Password string
    Name     string
}

type RegisterOutput struct {
    UserID uuid.UUID
}

type LoginInput struct {
    Email     string
    Password  string
    UserAgent string
    IP        string
}

type LoginOutput struct {
    AccessToken           string
    AccessTokenExpiresAt  time.Time
    RefreshToken          string // token mentah, cuma dibaca sekali oleh adapter/http untuk dijadikan cookie
    RefreshTokenExpiresAt time.Time
    User                  domain.User
}
```

DTO dipisah dari entity `domain.User`/`domain.Session` karena dua alasan:
(1) DTO berisi field yang tidak pernah ada di entity (`RefreshToken` token
mentah — entity `Session` cuma menyimpan hash-nya, tidak pernah menyimpan
token mentah di memori lebih lama dari yang perlu); (2) mengubah bentuk
request/response HTTP (menambah field opsional, dst) tidak boleh memaksa
mengubah struct domain yang jadi pusat aturan bisnis — dua alasan berubah
yang berbeda harus punya dua tipe yang berbeda.

### 7. Usecase `Register`

```go
type Register struct {
    Users  domain.UserRepository
    Hasher application.PasswordHasher
    Clock  clock.Clock
    NewID  func() uuid.UUID
}

func (u *Register) Execute(ctx context.Context, in RegisterInput) (RegisterOutput, error)
```

Alur: normalisasi email lewat `domain.NewEmail`, cek `FindByEmail` —
kalau ketemu, `ErrEmailTaken`. Hash password. `Users.Create`. Race
antara cek-dan-create ditangani di task 05 lewat unique constraint
Postgres (`23505` → `ErrEmailTaken`), jadi usecase ini tidak perlu
transaksi eksplisit untuk kasus ini.

### 8. Usecase `Login`

Alur: `FindByEmail` — kalau tidak ketemu, kembalikan `ErrInvalidCredentials`
(bukan `ErrUserNotFound`, lihat langkah 3). `Hasher.Verify` — kalau salah,
`ErrInvalidCredentials` juga. Kalau benar: `TokenIssuer.IssueAccessToken`,
generate refresh token (langkah 11), simpan hash-nya lewat
`Sessions.Create`, kembalikan token mentah di `LoginOutput`.

### 9. Usecase `Refresh` — WAJIB satu transaksi + reuse detection

```go
type Refresh struct {
    Sessions domain.SessionRepository
    Issuer   application.TokenIssuer
    UoW      application.UnitOfWork
    Clock    clock.Clock
    NewID    func() uuid.UUID
}
```

Alur `Execute(ctx, in RefreshInput)`:

1. Hash refresh token mentah dari input (SHA-256), `Sessions.FindByRefreshTokenHash`.
   Tidak ketemu → `ErrSessionNotFound`.
2. **Kalau `session.IsRevoked()` == true**: ini reuse — token yang sudah
   dicabut dipakai lagi, tanda kuat token bocor dan dipakai penyerang
   setelah pemilik asli sudah rotasi. Respons: `Sessions.RevokeAllForUser`
   untuk SEMUA session user itu (bukan cuma yang ini), lalu kembalikan
   error (mis. `ErrSessionRevoked`). Kenapa cabut semua, bukan cuma yang
   dipakai ulang: penyerang yang mencuri satu refresh token biasanya sudah
   memutar rotasi sendiri beberapa kali sebelum ketahuan — sesi hasil
   rotasi curian itu juga harus mati, dan satu-satunya cara memutus
   seluruh rantai yang tidak diketahui panjangnya adalah mencabut semua
   sesi milik user tersebut, memaksa dia login ulang di semua device.
3. Kalau `!session.IsActive(now)` (kedaluwarsa tapi belum revoked) →
   `ErrSessionExpired`.
4. Kalau aktif: **dalam satu `UoW.Do`** — cabut session lama
   (`Sessions.Revoke`), buat session baru dengan refresh token baru
   (`Sessions.Create`). Keduanya wajib satu transaksi: kalau revoke sukses
   tapi create gagal (atau sebaliknya) di luar transaksi, ada jendela di
   mana user tidak punya sesi valid sama sekali, atau lebih parah, dua
   sesi valid untuk satu refresh mentah yang sama beredar.
5. Terbitkan access token baru lewat `Issuer`. Kembalikan
   `RefreshOutput{AccessToken, RefreshToken (baru), ...}`.

### 10. Usecase `Logout` dan `Me`

`Logout{Sessions, Clock}.Execute(ctx, LogoutInput{RefreshToken string})` —
hash token, cari session, `Sessions.Revoke`. Kalau tidak ketemu, anggap
sukses (idempotent) — logout dua kali tidak boleh jadi error ke user.

`Me{Users}.Execute(ctx, MeInput{UserID uuid.UUID})` — `Users.FindByID`,
kembalikan `domain.User` yang sudah dipetakan ke `MeOutput` tanpa
`PasswordHash`.

### 11. Argon2id hasher

`application/argon2/hasher.go`, pakai `golang.org/x/crypto/argon2`:

```go
const (
    argonTime    = 1
    argonMemory  = 64 * 1024 // KiB
    argonThreads = 4
    argonKeyLen  = 32
    saltLen      = 16
)

func Hash(password string) (string, error) {
    salt := make([]byte, saltLen)
    if _, err := rand.Read(salt); err != nil {
        return "", fmt.Errorf("generate salt: %w", err)
    }
    hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
    encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, argonMemory, argonTime, argonThreads,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash))
    return encoded, nil
}

func Verify(password, encoded string) (bool, error) {
    // parse params, salt, hash dari string $argon2id$...
    // hitung ulang hash dengan parameter yang sama
    computed := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(hash)))
    return subtle.ConstantTimeCompare(hash, computed) == 1, nil
}
```

`Verify` WAJIB pakai `subtle.ConstantTimeCompare`, bukan `bytes.Equal` atau
`==`. Perbandingan biasa berhenti di byte pertama yang beda — waktu
eksekusinya jadi bocor info seberapa jauh tebakan penyerang benar (timing
attack). `ConstantTimeCompare` selalu memeriksa seluruh panjang byte
berapa pun hasilnya, jadi waktunya tidak bergantung pada isi.

### 12. JWT issuer/verifier

`application/jwtauth/issuer.go`, pakai `github.com/golang-jwt/jwt/v5`,
HS256, claim `sub` (user id), `jti` (uuid acak per token), `iat`, `exp`.

```go
func (i *Issuer) IssueAccessToken(userID uuid.UUID) (string, time.Time, error) {
    now := i.Clock.Now()
    exp := now.Add(15 * time.Minute)
    claims := jwt.RegisteredClaims{
        Subject:   userID.String(),
        ID:        uuid.New().String(),
        IssuedAt:  jwt.NewNumericDate(now),
        ExpiresAt: jwt.NewNumericDate(exp),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString(i.secret)
    return signed, exp, err
}

func (v *Verifier) VerifyAccessToken(raw string) (uuid.UUID, error) {
    token, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return v.secret, nil
    })
    // ...
}
```

Verifier WAJIB memeriksa `t.Method` secara eksplisit di dalam
`keyFunc`, sebelum mengembalikan key. Ini menutup dua serangan:

- **`alg: none`** — token dengan header `{"alg":"none"}` tidak punya
  signature sama sekali; library yang tidak mengecek method bisa
  menganggapnya valid tanpa pernah memverifikasi apa pun.
- **Algorithm confusion (HS256 vs RS256)** — kalau server juga pernah
  memakai RSA di tempat lain dan verifier menerima kunci publik RSA sebagai
  `key` untuk `alg` apa pun, penyerang yang tahu kunci publik itu (kunci
  publik memang publik) bisa membuat token HS256 yang di-sign pakai kunci
  publik itu sebagai secret HMAC — verifier yang tidak memeriksa method akan
  menerimanya sebagai valid.

Dengan mengecek `t.Method.(*jwt.SigningMethodHMAC)` secara eksplisit,
verifier menolak token apa pun yang bukan HS256, apa pun isi headernya.

### 13. Refresh token generator

`application/refreshtoken/generator.go`:

```go
func Generate() (raw string, hash []byte, err error) {
    buf := make([]byte, 32)
    if _, err := crand.Read(buf); err != nil { // crypto/rand
        return "", nil, fmt.Errorf("generate refresh token: %w", err)
    }
    raw = base64.RawURLEncoding.EncodeToString(buf)
    sum := sha256.Sum256([]byte(raw))
    return raw, sum[:], nil
}
```

`math/rand` (dan `math/rand/v2` tanpa seed dari sumber kripto) tidak boleh
dipakai untuk apa pun yang jadi kredensial. `math/rand` deterministik dari
seed-nya — kalau penyerang bisa menebak atau mengetahui seed (mis. seed
default berbasis waktu proses), seluruh urutan angka yang akan dihasilkan
bisa diprediksi, yang berarti refresh token masa depan bisa ditebak.
`crypto/rand` mengambil entropi dari sumber acak sistem operasi
(`/dev/urandom` di Linux) yang dirancang khusus supaya output-nya tidak
bisa diprediksi bahkan oleh penyerang yang tahu output-output sebelumnya.

### 14. Test

Fake repository tulis tangan, contoh bentuk:

```go
type fakeUserRepo struct {
    byID    map[uuid.UUID]domain.User
    byEmail map[string]domain.User
}

func (f *fakeUserRepo) Create(ctx context.Context, u domain.User) error {
    if _, exists := f.byEmail[u.Email.String()]; exists {
        return domain.ErrEmailTaken
    }
    f.byID[u.ID] = u
    f.byEmail[u.Email.String()] = u
    return nil
}
// FindByEmail, FindByID sejalan, mengembalikan domain.ErrUserNotFound kalau tidak ada
```

`clock.Fixed(t time.Time) clock.Clock` dipakai supaya `ExpiresAt`,
`CreatedAt` bisa diprediksi di assertion.

Kasus minimal wajib (tiap satu `_test.go` file terpisah per usecase):

1. `Register` — sukses (email baru tersimpan, password ter-hash bukan
   plaintext).
2. `Register` — email duplikat → `ErrEmailTaken`, tidak ada baris baru.
3. `Login` — password salah → `ErrInvalidCredentials`; dan email tidak ada
   → `ErrInvalidCredentials` juga (assert kedua kasus mengembalikan error
   yang SAMA lewat `errors.Is`).
4. `Refresh` — sukses: session lama ter-revoke, session baru dibuat, token
   baru berbeda dari yang lama.
5. `Refresh` — token kedaluwarsa (`ExpiresAt` di masa lalu menurut
   `clock.Fixed`) → `ErrSessionExpired`, tidak ada rotasi terjadi.
6. `Refresh` — token yang sudah `RevokedAt != nil` dipakai lagi → error,
   DAN assert bahwa SEMUA session lain milik user itu (bukan cuma yang
   dipakai ulang) ikut ter-revoke di fake repo.
7. `argon2.Hash`/`Verify` — hash lalu verify dengan password benar → true;
   password salah → false; format encoded string sesuai pola
   `$argon2id$v=...$m=...,t=...,p=...$...$...`.
8. `jwtauth` — issue lalu verify → userID cocok; token yang di-tamper
   (ubah satu karakter signature) → error; token dengan `alg: none` yang
   dirakit manual → error (test ini yang membuktikan mitigasi langkah 12
   benar-benar bekerja, bukan cuma dijelaskan di komentar).

## Kriteria selesai

```bash
cd apps/go-chi-api
go build ./internal/modules/identity/...
go vet ./internal/modules/identity/...
go test ./internal/modules/identity/... -run . -v
```

- Semua 8 kasus test di langkah 14 ada dan hijau.
- `go test ./internal/modules/identity/internal/domain/... ./internal/modules/identity/internal/application/...`
  tidak menyentuh network/filesystem/Docker sama sekali (murni in-memory).
- `grep -rn "net/http\|jackc/pgx\|go-chi/chi\|sqlc" internal/modules/identity/internal/domain/`
  kosong.
- `errors.Is(err, domain.ErrInvalidCredentials)` bernilai true untuk DUA
  kasus di `Login` (email tidak ada, password salah) — dibuktikan test,
  bukan cuma dijelaskan di komentar.
- `make lint` bersih untuk paket yang disentuh.

## Jebakan

- Jangan taruh normalisasi email di usecase atau (nanti) di handler HTTP —
  itu tanggung jawab `domain.NewEmail`. Kalau usecase memanggil
  `strings.ToLower` sendiri, berarti invariant-nya bocor keluar domain.
- Jangan biarkan `Login` mengembalikan error yang berbeda untuk "email
  tidak ada" vs "password salah" — sekecil apa pun bedanya (pesan, wrap,
  status), itu kebocoran enumerasi akun.
- Jangan lupa reuse detection mencabut SEMUA session, bukan cuma
  memperbarui yang dipakai ulang. Mencabut satu session saja tidak
  menutup kebocoran kalau penyerang sudah membuat beberapa sesi turunan.
- `Refresh` tanpa transaksi (revoke dan create terpisah, tanpa `UoW.Do`)
  akan lolos test unit dengan fake repository (fake tidak tahu soal
  atomicity) tapi salah secara nyata begitu masuk Postgres asli di task 05
  — pastikan kode benar-benar memanggil `UoW.Do` sekarang, jangan menunda
  ke task 05.
- Jangan taruh implementasi argon2id/JWT/refresh-token-generator di luar
  `internal/modules/identity/internal/application/...` — mereka bukan
  utilitas lintas modul, mereka detail modul identity.
- Jangan memakai `math/rand` di mana pun untuk refresh token, salt, atau
  `jti`.
