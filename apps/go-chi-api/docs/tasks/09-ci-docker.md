# Task 09 — CI dan Docker

**Phase:** 5 — Rilis
**Bergantung pada:** task 08
**Status:** belum dikerjakan

## Tujuan

Task terakhir backend. Setelah ini selesai, `apps/go-chi-api` dianggap tuntas
sesuai kriteria "Selesai kalau" di `docs/PRD.md`: CI hijau secara otomatis di
tiap push/PR, image Docker bisa dibangun dan dijalankan tanpa toolchain Go
sama sekali di dalamnya, dan `docker compose up` menyalakan API siap pakai
dari nol. Setelah task ini, `apps/cms` boleh mulai dikerjakan.

## Rujukan

- ADR-013 (integration test testcontainers — CI butuh Docker-in-Docker atau
  runner yang sudah menyediakan Docker)
- ADR-012 (OpenAPI beku — job CI wajib menegaskan `bundled.yaml` tidak basi)
- `ARCHITECTURE.md`, `AGENTS.md` — "Sebelum bilang selesai" (daftar perintah
  yang harus hijau, jadi dasar job CI)
- `docs/PRD.md` bagian "Selesai kalau" — checklist penutup task ini
- Root `Makefile` dan `docker-compose.dev.yml` — `docker-compose.yml` baru di
  task ini **terpisah**, jangan menyunting `docker-compose.dev.yml`
- Task 00 — daftar target Makefile (`tools`, versi pin tool) jadi basis job
  `lint`/`test`/`sqlc` di CI, pakai pin yang sama, jangan pin ulang berbeda

## File yang dibuat atau disentuh

| Path                                | Isi                                                        |
| ----------------------------------- | ---------------------------------------------------------- |
| `.github/workflows/ci.yml`          | job lint, test, test-integration, openapi, migration-check |
| `apps/go-chi-api/Dockerfile`        | multistage build                                           |
| `docker-compose.yml` (root, BARU)   | demo: postgres + api                                       |
| `apps/go-chi-api/.dockerignore`     |                                                            |
| `README.md`                         | bagian "Status" diperbarui                                 |
| `apps/go-chi-api/docs/HANDS-OFF.md` | daftar target Makefile final                               |

## Langkah

### 1. `.github/workflows/ci.yml`

Lima job, jalan paralel kecuali yang bergantung:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

env:
  GO_VERSION: "1.26.5"

jobs:
  lint:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/go-chi-api
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/go-chi-api/go.sum
      - name: gofmt
        run: |
          out=$(gofmt -l .)
          if [ -n "$out" ]; then echo "$out"; exit 1; fi
      - name: go vet
        run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.62.2
          working-directory: apps/go-chi-api

  test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/go-chi-api
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/go-chi-api/go.sum
      - run: go test ./... -short

  test-integration:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/go-chi-api
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/go-chi-api/go.sum
      - run: go test ./... -tags=integration
        # testcontainers-go butuh Docker; ubuntu-latest GitHub-hosted runner
        # sudah menyediakan Docker daemon secara bawaan, tidak perlu setup
        # tambahan.

  openapi:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/go-chi-api
    steps:
      - uses: actions/checkout@v4
      - name: install redocly
        run: npm install -g @redocly/cli@1.25.11
      - run: make openapi-lint
      - run: make openapi-bundle-check

  migration-check:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: todolist_ci
        ports: ["5432:5432"]
        options: >-
          --health-cmd "pg_isready -U postgres"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 10
    defaults:
      run:
        working-directory: apps/go-chi-api
    steps:
      - uses: actions/checkout@v4
      - name: install goose
        run: go install github.com/pressly/goose/v3/cmd/goose@v3.24.1
      - name: migrate up
        run: goose -dir db/migrations postgres "postgres://postgres:postgres@localhost:5432/todolist_ci?sslmode=disable" up
      - name: migrate down to zero
        run: goose -dir db/migrations postgres "postgres://postgres:postgres@localhost:5432/todolist_ci?sslmode=disable" down-to 0
        # down-to 0 memaksa SETIAP migrasi menjalankan Down-nya sampai habis,
        # bukan cuma migrasi paling akhir. Ini yang membuktikan `-- +goose Down`
        # di semua file benar-benar jalan, bukan cuma ditulis dan tidak pernah
        # dieksekusi.
```

**Kenapa versi tool dipin (`v1.62.2`, `v3.24.1`, `1.25.11`), bukan
`@latest`:** `@latest` membuat build hari ini dan build minggu depan bisa
memakai versi tool yang berbeda tanpa ada perubahan kode apa pun di repo —
kalau versi baru tool itu mengubah perilaku (default lint rule baru yang
lebih ketat, format output goose yang beda), CI bisa merah tiba-tiba di PR
yang tidak menyentuh hal terkait sama sekali, dan penyebabnya sulit
ditemukan karena tidak ada di diff. Versi dipin di sini harus sama persis
dengan yang dipin di `apps/go-chi-api/Makefile` target `tools` (task 00) —
kalau salah satu diperbarui, perbarui juga yang lain di task ini.

Cache modul Go: `actions/setup-go@v5` dengan `cache-dependency-path` sudah
otomatis meng-cache `$GOPATH/pkg/mod` dan cache build (`$GOCACHE`)
berdasarkan hash `go.sum` — tidak perlu action cache manual tambahan.

### 2. `apps/go-chi-api/Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.26.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/api /api
COPY --from=builder /src/db/migrations /migrations

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/api"]
```

`CGO_ENABLED=0` supaya binary statis, tidak bergantung `libc` sistem apa
pun di image akhir. `-trimpath` menghapus path filesystem builder (misal
`/src/...` atau path lokal developer) dari informasi debug binary — tanpa
ini, panic trace atau `runtime.Caller` bisa membocorkan struktur direktori
mesin build. `ldflags -X main.version=...` menyuntik versi dan commit git
saat build (`main.go` perlu deklarasi `var version, commit string` package-level
yang dibaca `/healthz` atau log start-up — kalau belum ada, tambahkan
sebagai bagian kecil task ini).

`gcr.io/distroless/static-debian12:nonroot` dipilih atas `scratch` karena
sudah menyertakan sertifikat CA (dibutuhkan kalau ada koneksi TLS keluar di
masa depan) dan file `/etc/passwd` minimal untuk user `nonroot` — `scratch`
sungguhan kosong total dan butuh usaha ekstra menambahkan CA cert manual
kalau ternyata dibutuhkan nanti. `USER nonroot:nonroot` memastikan proses di
container tidak jalan sebagai root — kalau ada kerentanan di aplikasi yang
memungkinkan eksekusi kode, dampaknya dibatasi hak akses user biasa.

**Kenapa image akhir tidak boleh berisi toolchain Go:** compiler Go dan
seluruh `$GOPATH/pkg/mod` menambah ratusan MB yang tidak pernah dipakai saat
runtime — binary yang sudah dikompilasi statis tidak butuh compiler untuk
jalan. Selain ukuran, toolchain build adalah permukaan serangan tambahan
yang percuma: kalau container ini pernah disusupi, tidak adanya shell,
package manager, atau compiler di dalamnya membuat penyerang jauh lebih sulit
mengunduh dan menjalankan tool tambahan (`distroless` bahkan tidak punya
shell sama sekali) — ini prinsip "attack surface minimal", bukan sekadar
soal ukuran image.

Migrasi disalin ke image (`/migrations`) supaya `docker-compose.yml` bisa
menjalankan `goose up` dari dalam container yang sama sebelum start server,
tanpa perlu image goose terpisah — lihat langkah 4.

### 3. `.dockerignore`

```
.git
tmp/
bin/
*.md
.env
.env.*
docs/
```

Alasan `docs/` diabaikan: tidak dibutuhkan runtime dan memperbesar context
build tanpa guna; `db/migrations` **tidak** masuk daftar ini karena memang
dibutuhkan (lihat langkah 2).

### 4. `docker-compose.yml` (root, BARU)

```yaml
# Demo/production-like stack: build image dari Dockerfile, jalankan migrasi
# otomatis saat start. Terpisah dari docker-compose.dev.yml karena tujuannya
# beda: dev compose cuma menyalakan Postgres (API jalan langsung dari host
# lewat `make dev` demi live reload), compose ini menyalakan STACK LENGKAP
# dari image yang sudah dibangun — ini yang dipakai untuk membuktikan
# "docker compose up menyalakan API siap pakai" di kriteria PRD, dan yang
# dipakai siapa pun yang mau mencoba project ini tanpa memasang toolchain Go
# sama sekali di mesinnya.

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_DB:-todolist}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "pg_isready -U ${POSTGRES_USER:-postgres} -d ${POSTGRES_DB:-todolist}",
        ]
      interval: 5s
      timeout: 5s
      retries: 10

  migrate:
    build:
      context: ./apps/go-chi-api
    image: todolist-api:latest
    entrypoint: ["/bin/sh", "-c"]
    command: >
      "goose -dir /migrations postgres
      postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:-postgres}@postgres:5432/${POSTGRES_DB:-todolist}?sslmode=disable
      up"
    depends_on:
      postgres:
        condition: service_healthy

  api:
    build:
      context: ./apps/go-chi-api
    image: todolist-api:latest
    restart: unless-stopped
    environment:
      APP_ENV: production
      HTTP_PORT: 8080
      DATABASE_URL: postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:-postgres}@postgres:5432/${POSTGRES_DB:-todolist}?sslmode=disable
      JWT_SECRET: ${JWT_SECRET}
      ACCESS_TOKEN_TTL: 15m
      REFRESH_TOKEN_TTL: 720h
      LOG_LEVEL: info
      CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-http://localhost:5173}
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully

volumes:
  postgres-data:
```

Catatan `entrypoint`/`command` di service `migrate`: image `todolist-api`
hasil Dockerfile langkah 2 berbasis distroless **tanpa shell**, jadi
`entrypoint: ["/bin/sh", "-c"]` di atas tidak akan jalan di image itu apa
adanya. Selesaikan salah satu dari dua cara sebelum menganggap langkah ini
selesai — pilih satu, jangan dua-duanya, dan catat pilihannya di komentar
compose file:

1. Tambahkan `goose` sebagai binary statis ke dalam image final (build stage
   terpisah yang meng-`go install` goose, disalin ke image final di path
   `/goose`), lalu `migrate` memanggil `["/goose", "-dir", "/migrations", "postgres", "...", "up"]`
   langsung tanpa shell.
2. Jalankan migrasi sebagai bagian dari `ENTRYPOINT` binary `/api` sendiri
   sebelum `ListenAndServe` (tambahkan flag `--migrate` atau baca env
   `RUN_MIGRATIONS_ON_START=true`), sehingga service `migrate` terpisah tidak
   dibutuhkan sama sekali dan `api` yang menjalankan migrasinya sendiri saat
   start.

Opsi 2 lebih sederhana untuk demo compose (satu service lebih sedikit) tapi
mengubah bentuk `cmd/api/main.go` — pilih berdasarkan mana yang terasa lebih
sesuai dengan wiring `main.go` yang sudah ada dari task-task sebelumnya,
tulis alasannya sebagai komentar singkat di compose file supaya keputusan
ini tidak terlihat seperti kelalaian saat dibaca ulang nanti.

`depends_on: condition: service_healthy` untuk `postgres` memastikan `api`
dan `migrate` tidak mencoba connect sebelum Postgres benar-benar siap
menerima koneksi (bukan cuma proses containernya sudah start) —
`service_completed_successfully` untuk `migrate` di dependency `api`
memastikan API tidak menyala di atas schema yang belum bermigrasi (kalau
memilih opsi 1 di atas; kalau memilih opsi 2, dependency ini dihapus karena
service `migrate` tidak ada).

### 5. `README.md` bagian "Status"

Perbarui daftar status jadi mencerminkan backend selesai: sebutkan modul apa
saja yang sudah ada (`identity`, `task`), bahwa CI dan Docker sudah jalan,
dan bahwa `apps/cms` sekarang boleh mulai dikerjakan berbekal
`docs/openapi/openapi.yaml`. Jangan menghapus riwayat status sebelumnya kalau
formatnya berbentuk log — tambahkan entri baru mengikuti format yang sudah
ada di file itu.

### 6. `apps/go-chi-api/docs/HANDS-OFF.md`

Perbarui (atau buat, kalau belum ada dari task sebelumnya) daftar target
Makefile final yang tersedia dari root, dengan satu baris deskripsi tiap
target — sumbernya adalah gabungan semua target yang sudah ditulis task
00 sampai 09:

```
make setup                bootstrap awal: copy .env, nyalakan postgres dev, migrasi
make dev                   live reload
make run                    jalankan tanpa reload
make build                  build binary ke bin/api
make test                  unit test, cepat, tanpa Docker
make test-integration       unit + integration, butuh Docker
make lint                    golangci-lint
make migrate-up/down/reset/status/create  migrasi goose
make sqlc                    generate kode dari SQL
make openapi-lint            lint kontrak
make openapi-bundle          gabungkan spec jadi satu file
make openapi-bundle-check    gagal kalau bundle belum diperbarui
make tools                   pasang semua CLI yang dibutuhkan
make docs-serve              pratinjau OpenAPI lokal
make dockerdev-up/down/logs/ps/psql   container Postgres dev
docker compose up            stack demo lengkap (Postgres + API + migrasi)
```

Sertakan juga bagian pendek "Checklist penutup backend" yang mengutip
langsung tiap butir "Selesai kalau" dari `docs/PRD.md`, dengan checkbox
kosong — pengecekan sungguhannya dilakukan manual saat task ini
benar-benar dikerjakan, bukan diklaim selesai di dokumen task ini.

## Kriteria selesai

```bash
cd apps/go-chi-api
docker build -t todolist-api:test .
docker run --rm todolist-api:test --help 2>&1 | true   # sekadar bukti binary jalan tanpa crash loader
docker images todolist-api:test --format '{{.Size}}'   # cek ukuran, harus puluhan MB bukan ratusan MB+toolchain
```

```bash
cd <root repo>
docker compose up -d
curl -sf localhost:8080/healthz
curl -sf localhost:8080/readyz
docker compose down -v
```

- Kedua blok perintah di atas berjalan tanpa error dan `curl` mengembalikan
  200 untuk keduanya.
- `git log --oneline -- .github/workflows/ci.yml` (setelah push ke GitHub,
  di luar scope task dokumen ini tapi dicatat sebagai langkah verifikasi
  manual) menunjukkan job `lint`, `test`, `test-integration`, `openapi`,
  `migration-check` semuanya hijau di Actions tab.
- `grep -n '@latest' .github/workflows/ci.yml apps/go-chi-api/Dockerfile`
  tidak menghasilkan apa pun.
- `grep -n 'USER' apps/go-chi-api/Dockerfile` menunjukkan user non-root
  di stage final.
- Semua checkbox "Selesai kalau" di `docs/PRD.md` bisa dicentang dengan
  bukti perintah yang benar-benar dijalankan (bukan asumsi) — ini pengecekan
  manual terakhir sebelum backend dianggap tuntas dan `apps/cms` dimulai.

## Jebakan

- `docker-compose.yml` baru ini gampang tertukar maksud dengan
  `docker-compose.dev.yml` yang sudah ada — jangan menyunting yang lama,
  jangan menyalin isinya mentah-mentah karena tujuannya beda (lihat komentar
  di langkah 4).
- Image distroless final tidak punya shell — kalau debugging container demo
  dibutuhkan, jalankan `docker run --entrypoint /busybox/sh` tidak akan
  bekerja di distroless; solusi debugging sesungguhnya adalah baca log
  (`docker compose logs api`), bukan exec masuk ke container.
- `service_completed_successfully` sebagai kondisi `depends_on` butuh
  Docker Compose versi yang cukup baru (v2.20+) — kalau CI atau mesin
  developer memakai Compose lama, kondisi ini tidak dikenali dan
  errornya membingungkan (compose menolak seluruh file, bukan cuma baris
  itu). Sebutkan syarat versi ini di komentar `docker-compose.yml`.
- Cache Go di CI (`actions/setup-go` dengan `cache-dependency-path`) bisa
  jadi stale kalau `go.sum` tidak ikut berubah padahal dependency tambahan
  ditambah lewat `replace` directive — jarang terjadi di project ini karena
  tidak ada `replace` yang direncanakan, tapi kalau CI tiba-tiba memakai
  versi dependency lama, curigai cache dulu.
- `down-to 0` di job `migration-check` akan gagal keras (dan itu memang
  tujuannya) kalau ada migrasi yang `-- +goose Down`-nya kosong atau salah —
  jangan melemahkan job ini jadi cuma `goose up` supaya "CI hijau"; itu
  menghilangkan satu-satunya pemeriksaan otomatis bahwa rollback migrasi
  benar-benar bisa dipakai.
