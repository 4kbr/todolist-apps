# Task 00 — Foundation

**Phase:** 1 — Fondasi
**Bergantung pada:** tidak ada
**Status:** belum dikerjakan

## Tujuan

Setelah task ini selesai, `apps/go-chi-api` adalah kerangka aplikasi Go yang
bisa dijalankan: `make dev` menyalakan server dengan live reload, `GET
/healthz` menjawab, shutdown rapi saat `Ctrl-C`, dan seluruh tooling
(`air`, `goose`, `sqlc`, `golangci-lint`) terpasang lewat satu perintah.
Belum ada endpoint domain, belum ada tabel — itu task 01 dan seterusnya.

## Rujukan

- ADR-001, ADR-002, ADR-003 (bentuk modul, dipakai nanti tapi wiring dasarnya
  disiapkan di sini lewat `Deps`-style constructor di `main.go`)
- ADR-009 (UUIDv7) — dasar untuk `internal/platform/id`
- `ARCHITECTURE.md` bagian "Pohon direktori" dan "Bentuk besar"
- `AGENTS.md` bagian "Aturan struktur" dan "Aturan kode"

## File yang dibuat atau disentuh

| Path | Kenapa |
| --- | --- |
| `Makefile` (root) | hapus target `dev-worker`, `run-worker` — warisan template, project ini tidak punya worker |
| `apps/go-chi-api/Makefile` | seluruh target yang dijanjikan root, lihat langkah 1 |
| `apps/go-chi-api/.env.example` | placeholder konfigurasi, tanpa kredensial asli |
| `apps/go-chi-api/go.mod` | ganti module path dari `apps/go-chi-api` ke path unik |
| `apps/go-chi-api/.air.toml` | konfigurasi live reload |
| `apps/go-chi-api/.golangci.yml` | konfigurasi linter |
| `apps/go-chi-api/internal/platform/config/config.go` | baca env → struct, validasi saat start |
| `apps/go-chi-api/internal/platform/logger/logger.go` | `slog` JSON handler |
| `apps/go-chi-api/internal/platform/clock/clock.go` | interface `Clock` + `System` + `Fixed` |
| `apps/go-chi-api/internal/platform/id/id.go` | pembungkus UUIDv7 |
| `apps/go-chi-api/internal/platform/postgres/pool.go` | `pgxpool` dengan konfigurasi sehat + `Ping` |
| `apps/go-chi-api/cmd/api/main.go` | wiring, router `chi`, `/healthz`, graceful shutdown |

## Langkah

### 1. Perbaiki Makefile root

Root `Makefile` saat ini meneruskan target berikut ke `apps/go-chi-api`:

```
dev dev-worker run run-worker build test test-integration lint \
db-up db-down db-reset db-status db-create tools docs-serve:
	$(MAKE) -C $(BACKEND) $@ $(if $(name),name=$(name))
```

`dev-worker` dan `run-worker` adalah warisan template lama. Project ini tidak
dan tidak akan punya worker terpisah — semua berjalan dalam satu proses HTTP
API. Hapus `dev-worker` dan `run-worker` dari baris target itu, sehingga jadi:

```
dev run build test test-integration lint \
db-up db-down db-reset db-status db-create tools docs-serve:
	$(MAKE) -C $(BACKEND) $@ $(if $(name),name=$(name))
```

Root Makefile juga belum meneruskan `openapi-lint`, `openapi-bundle`,
`openapi-bundle-check` — tambahkan ketiganya ke baris target yang sama supaya
`make openapi-lint` dari root ikut bekerja (dibutuhkan mulai task 03).

Ini satu-satunya perubahan yang boleh terjadi pada file di luar
`apps/go-chi-api/` selama mengerjakan task ini.

### 2. Tulis `apps/go-chi-api/Makefile`

Target yang wajib ada, semuanya `.PHONY`:

```makefile
dev:                    ## live reload lewat air
	air -c .air.toml

run:                    ## jalankan tanpa reload
	go run ./cmd/api

build:                  ## build binary ke bin/api
	go build -o bin/api ./cmd/api

test:                   ## unit test saja, cepat, tanpa Docker
	go test ./... -short

test-integration:       ## unit + integration, butuh Docker (testcontainers)
	go test ./... -tags=integration

lint:
	golangci-lint run ./...

db-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

db-down:
	goose -dir db/migrations postgres "$$DATABASE_URL" down

db-reset:
	goose -dir db/migrations postgres "$$DATABASE_URL" reset

db-status:
	goose -dir db/migrations postgres "$$DATABASE_URL" status

db-create:
	goose -dir db/migrations create $(name) sql

sqlc:
	sqlc generate

openapi-lint:
	redocly lint docs/openapi/openapi.yaml

openapi-bundle:
	redocly bundle docs/openapi/openapi.yaml -o docs/openapi/bundled.yaml

openapi-bundle-check:   ## gagal kalau bundle belum di-regenerate setelah edit spec
	redocly bundle docs/openapi/openapi.yaml -o /tmp/openapi-bundled-check.yaml
	diff docs/openapi/bundled.yaml /tmp/openapi-bundled-check.yaml

tools:                  ## pasang semua tool CLI yang dipakai target di atas
	go install github.com/air-verse/air@v1.61.1
	go install github.com/pressly/goose/v3/cmd/goose@v3.24.1
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

docs-serve:             ## pratinjau OpenAPI lokal
	redocly preview-docs docs/openapi/openapi.yaml
```

Catatan `DATABASE_URL`: target `db-*` membaca dari environment, bukan dari
file `.env` langsung — dokumentasikan di komentar Makefile bahwa developer
wajib `export $(grep -v '^#' .env | xargs)` atau memakai `direnv`/`dotenv`
sebelum memanggil target itu. Jangan menambah dependency baru hanya untuk
memuat `.env` di Makefile.

`sqlc` bukan target yang diminta root Makefile secara eksplisit di
daftar task ini, tapi dibutuhkan task 01 — tetap tulis sekarang supaya task
01 tidak perlu menyunting Makefile lagi.

Versi tool di atas adalah versi yang dipin saat task ini ditulis. Kalau versi
itu sudah tidak ada saat dikerjakan, cek rilis terbaru tiap tool dan perbarui
pin-nya — jangan pakai `@latest` karena build jadi tidak reproducible.

### 3. `apps/go-chi-api/.env.example`

```env
APP_ENV=development
HTTP_PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/todolist_dev?sslmode=disable
JWT_SECRET=change-me-to-a-random-32-byte-value
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
LOG_LEVEL=debug
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

Tidak ada kredensial asli. Nilai `DATABASE_URL` di sini harus cocok dengan
`.env.docker.example` di root (`POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB`, `POSTGRES_PORT`) — itu sudah aturan `setup` di root Makefile.

### 4. Ganti module path

`go.mod` sekarang:

```
module apps/go-chi-api
```

Ganti jadi path unik, misal:

```
module github.com/<username-github>/todolist/apps/go-chi-api
```

Module path adalah identitas global paket Go — dipakai `go get` siapa pun
yang meng-import modul ini, dan dipakai `go.sum` untuk mengenali versi. Path
generik seperti `apps/go-chi-api` bentrok kalau suatu saat project ini
diimpor sebagai dependency dari repo lain, atau kalau di-publish ke proxy Go
publik. Ini keputusan yang perlu dikonfirmasi user sebelum dieksekusi —
**tandai sebagai keputusan user**: tanyakan username/org GitHub yang mau
dipakai sebelum mengubah `go.mod` dan seluruh import path di kode yang sudah
ada.

Setelah mengganti, jalankan `go mod tidy` dan pastikan tidak ada file lain di
task ini yang meng-hardcode path lama.

### 5. `internal/platform/config`

```go
package config

type Config struct {
	AppEnv              string
	HTTPPort            string
	DatabaseURL         string
	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	LogLevel            slog.Level
	CORSAllowedOrigins  []string
}

func Load() (Config, error)
```

Baca dari `os.Getenv` langsung, tanpa library seperti `viper` atau `envconfig`.
Alasan: konfigurasi di project ini kecil dan tetap (delapan variabel), jadi
`os.Getenv` + helper kecil (`mustDuration`, `splitCSV`) sudah cukup jelas
dibaca dan tidak menambah dependency yang harus dipelajari. Tambah dependency
baru harus sepadan dengan masalah yang diselesaikan — di sini tidak ada
masalah yang butuh diselesaikan.

`Load()` **gagal cepat** (`return Config{}, fmt.Errorf(...)`) kalau:
- `JWT_SECRET` kosong atau lebih pendek dari 32 karakter
- `DATABASE_URL` tidak bisa di-parse (`url.Parse`, cek scheme `postgres`)
- `ACCESS_TOKEN_TTL` / `REFRESH_TOKEN_TTL` gagal `time.ParseDuration`
- `HTTP_PORT` kosong

`main.go` memanggil `config.Load()` sekali di awal dan `log.Fatal` kalau error
— aplikasi tidak boleh menyala dengan konfigurasi setengah valid.

### 6. `internal/platform/logger`

```go
package logger

func New(level slog.Level, appEnv string) *slog.Logger
```

`slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})`. Level
datang dari `Config.LogLevel`. Tidak perlu library logging pihak ketiga —
`log/slog` sudah bagian standard library sejak Go 1.21 dan sudah terstruktur.

### 7. `internal/platform/clock`

```go
package clock

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time { return time.Now() }

type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T }
```

Alasan interface ini penting: usecase auth (task 04) penuh logika kedaluwarsa
— access token 15 menit, refresh token 30 hari, deteksi reuse. Kalau usecase
memanggil `time.Now()` langsung, unit test untuk "refresh token yang
kedaluwarsa 1 detik lalu ditolak" mustahil ditulis tanpa `time.Sleep` yang
flaky. Dengan `Clock` disuntikkan, test cukup memakai `clock.Fixed{T: masaLalu}`.

### 8. `internal/platform/id`

```go
package id

func New() (uuid.UUID, error) {
	return uuid.NewV7()
}
```

Pakai `github.com/google/uuid`. Tambahkan ke `go.mod` lewat `go get
github.com/google/uuid`. Dibungkus jadi fungsi sendiri (bukan dipanggil
langsung `uuid.NewV7()` di tiap tempat) supaya kalau strategi ID pernah
berubah, titik ubahnya satu.

### 9. `internal/platform/postgres` — pool

```go
package postgres

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
```

Angka-angka di atas nilai awal yang wajar untuk single-instance dev/small
prod, bukan hasil tuning — boleh disesuaikan nanti tanpa mengubah bentuk
kode. `Ping` saat start supaya aplikasi gagal secepat mungkin kalau database
tidak bisa dijangkau, bukan gagal diam-diam di request pertama.

Tx manager (`UnitOfWork`) **tidak** ditulis di task ini — itu task 02, karena
dia bagian dari platform HTTP/transaksi lintas modul, bukan bagian dasar
koneksi.

### 10. `cmd/api/main.go`

Urutan wiring:

```go
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
}
```

`http.Server` tanpa `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/
`IdleTimeout` adalah bug default Go yang gampang terlewat: nilai default-nya
tanpa batas waktu sama sekali, jadi satu klien lambat (sengaja atau tidak)
bisa menahan goroutine selamanya (Slowloris-style). Empat timeout di atas
wajib ada sebelum server ini menerima trafik apa pun, termasuk saat dev.

Module untuk `identity` dan `task` **belum** disuntikkan di sini — itu
menyusul saat modulnya ada (task 04+). Task ini hanya menyiapkan slot
`chi.NewRouter()` yang nanti dipasangi `r.Mount("/v1", module.Routes())`.

### 11. `.air.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/api ./cmd/api"
bin = "./tmp/api"
include_ext = ["go"]
exclude_dir = ["tmp", "bin", "docs"]
delay = 200

[log]
time = true
```

### 12. `.golangci.yml`

```yaml
run:
  timeout: 3m

linters:
  disable-all: true
  enable:
    - errcheck
    - govet
    - staticcheck
    - revive
    - ineffassign
    - bodyclose
    - sqlclosecheck
    - gosec
    - misspell

issues:
  exclude-dirs:
    - tmp
    - bin
```

## Kriteria selesai

- [ ] `cd apps/go-chi-api && go build ./...` sukses
- [ ] `make tools` memasang `air`, `goose`, `sqlc`, `golangci-lint` tanpa error
- [ ] `make lint` bersih (tidak ada finding, karena kode masih minimal)
- [ ] `make run` lalu `curl -s localhost:8080/healthz` mengembalikan
      `{"status":"ok"}`
- [ ] `Ctrl-C` saat `make run` berjalan menghasilkan log "shutting down" dan
      proses keluar tanpa hang
- [ ] `make dev` menyalakan `air` dan mendeteksi perubahan file `.go`
- [ ] `grep -n 'dev-worker\|run-worker' Makefile` di root tidak menghasilkan apa pun
- [ ] `config.Load()` mengembalikan error (bukan panic, bukan lolos diam-diam)
      kalau `JWT_SECRET` dikosongkan — cek manual dengan unit test kecil

## Jebakan

- Jangan lupa `pool.Close()` juga dipanggil kalau `Ping` gagal di
  `NewPool` — kalau tidak, koneksi bocor tiap kali start gagal saat retry.
- `signal.NotifyContext` harus dipakai, bukan `signal.Notify` manual dengan
  channel sendiri — lebih mudah salah dan `context` sudah jadi idiom standar
  di Go modern.
- Root Makefile memakai tab untuk resep — kalau menyunting dengan editor yang
  mengubah tab jadi spasi, `make` akan gagal dengan "missing separator" tanpa
  pesan yang jelas.
- `go.mod` module path harus diganti **sebelum** ada import internal apa pun
  yang menunjuk `apps/go-chi-api/...` — task 00 ini kode pertamanya, jadi
  aman dilakukan sekarang. Kalau ditunda, task-task berikutnya harus
  refactor ulang semua import path.
- `HealthCheckPeriod` di pgxpool bukan endpoint `/healthz` aplikasi — jangan
  disatukan; `HealthCheckPeriod` murni internal pgxpool untuk mendaur ulang
  koneksi mati.
