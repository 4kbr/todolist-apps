# HANDS-OFF — apps/go-chi-api

Keadaan app ini sekarang. Ini **state**, bukan history — kalau sesuatu berubah,
timpa barisnya, jangan ditumpuk.

## Status

**Task 00 (foundation) selesai.** `make lint`, `make test`, `make run`
(dengan Postgres jalan) semua hijau, `/healthz` menjawab, graceful shutdown
jalan.

**Task 01 (database & migrations) — file sudah dibuat, belum direview.**
Empat migrasi goose (`users`, `sessions`, `lists`, `todos`) dan `sqlc.yaml`
(modul `identity` dan `task`) ada di `main`. Nomor migrasi yang dipakai adalah
seri `20260921`–`20260922`; seri `20260918` dari branch sementara tidak ikut
karena membuat tabel yang sama. Checklist task 01 mencatat uji migrasi terhadap
Postgres sudah lulus; hasilnya belum direview ulang dalam merge ini.

| Bagian             | Keadaan                                                                                                                   |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| Dokumen arsitektur | selesai — `docs/ARCHITECTURE.md`                                                                                          |
| Keputusan teknis   | selesai — `docs/DECISIONS.md`, ADR-001 s.d. ADR-013                                                                       |
| Skema database     | empat migrasi task 01 sudah ada, belum direview; `docs/DB-SCHEMA.md` sebagai referensi                                    |
| Aturan kode        | selesai — `AGENTS.md`                                                                                                     |
| Daftar task        | selesai — `docs/tasks/`, 10 task dalam 5 phase                                                                            |
| Kontrak OpenAPI    | kosong — diisi task 03                                                                                                    |
| Kode               | task 00 selesai — `Makefile`, config, logger, clock, id, pool pgx, chi, `/healthz`, graceful shutdown, air, golangci-lint |

## Langkah berikutnya

Review [`docs/tasks/01-database-migrations.md`](tasks/01-database-migrations.md)
beserta bukti verifikasi migrasi terhadap Postgres. Setelah task 01 direview,
lanjut task 02. Satu task per sesi, berurutan.

## Yang perlu diketahui sebelum mulai

- `go.mod` masih memakai module path `apps/go-chi-api` (belum diganti ke path
  unik seperti `github.com/<user>/todolist/apps/go-chi-api`). Ini keputusan
  user yang masih tertunda — beresin sebelum ada import internal baru ditulis,
  supaya tidak perlu refactor ulang semua import path nanti.
- Penamaan identifier pakai `camelCase` standar Go (`revive.var-naming`
  aktif tanpa exception di `.golangci.yml`).
- `.golangci.yml` pakai config **v2** (butuh `version: "2"` di kepala file,
  `linters.default: none` bukan `disable-all`, exclude path pindah ke
  `linters.exclusions.paths`) — `golangci-lint` v2.x (yang kepasang lewat
  `make tools`) **tidak backward-compatible** dengan config v1. Jangan
  disamain sama contoh literal di `docs/tasks/00-foundation.md` langkah 12,
  itu ditulis dengan asumsi `golangci-lint@v1.62.2`.
- Semua package di `internal/platform/` dan `cmd/api` wajib punya package
  comment (`// Package x ...` persis di atas `package x`) dan tiap fungsi/
  method exported wajib punya doc comment — `revive` (`package-comments`,
  `exported`) aktif penuh, gak ada exception.
- `Makefile` app meng-`-include .env` dan `export` semua variabelnya di baris
  paling atas, supaya `make run`/`make dev`/target lain otomatis dapat
  environment dari `.env` tanpa perlu `export` manual tiap kali. `config.Load()`
  sendiri tetap baca `os.Getenv` langsung tanpa library dotenv (lihat
  `internal/platform/config/config.go`) — pemuatan `.env` murni tanggung
  jawab Makefile, bukan kode Go.
- Migrasi database sudah ada di `db/migrations/` (4 file). `make setup` /
  `make run` tetap butuh Postgres jalan dan `DATABASE_URL` valid.
- `sqlc generate` (`make sqlc`) **gagal** selama `internal/modules/identity/
  internal/adapter/postgres/queries/` dan `.../task/.../queries/` masih
  benar-benar kosong (cuma `.gitkeep`) — `sqlc` v1.31.1 (versi yang dipasang
  `make tools`) melempar error `no queries contained in paths`, bukan sukses
  dengan output kosong seperti diasumsikan draft `docs/tasks/
  01-database-migrations.md` langkah 8. Ini bukan bug di kode kita, cuma
  behavior sqlc versi ini — akan hilang sendiri begitu task 05 (identity)
  dan task 07 (task) mengisi `queries/*.sql`. Jangan tambah query
  placeholder di sini cuma buat bikin `make sqlc` lulus.

## Target Makefile

`dev run build test test-integration lint migrate-up migrate-down
migrate-reset migrate-status migrate-create sqlc openapi-lint openapi-bundle
openapi-bundle-check tools docs-serve` — semua sudah ada dan bisa dipanggil
dari root lewat forwarding. Catatan: task 00 menamainya `migrate-*` (root
Makefile forward pakai nama ini), bukan `db-*` seperti draft awal task —
kalau nanti nemu referensi `db-up` dkk di dokumen lama, itu sudah usang.

## Batasan

- `apps/cms` tidak disentuh dari sini.
- `docs/openapi/` adalah kontrak beku dengan `apps/cms`. Mengubahnya butuh version
  bump dan pemberitahuan ke kedua sisi.
