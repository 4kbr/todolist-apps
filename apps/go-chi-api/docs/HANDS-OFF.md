# HANDS-OFF — apps/go-chi-api

Keadaan app ini sekarang. Ini **state**, bukan history — kalau sesuatu berubah,
timpa barisnya, jangan ditumpuk.

## Status

**Task 00 (foundation) selesai.** `make lint`, `make test`, `make run`
(dengan Postgres jalan) semua hijau, `/healthz` menjawab, graceful shutdown
jalan.

| Bagian | Keadaan |
| --- | --- |
| Dokumen arsitektur | selesai — `docs/ARCHITECTURE.md` |
| Keputusan teknis | selesai — `docs/DECISIONS.md`, ADR-001 s.d. ADR-013 |
| Skema database | selesai (dokumen) — `docs/DB-SCHEMA.md`, ERD + index + cascade; migrasi sungguhan masih task 01 |
| Aturan kode | selesai — `AGENTS.md` |
| Daftar task | selesai — `docs/tasks/`, 10 task dalam 5 phase |
| Kontrak OpenAPI | kosong — diisi task 03 |
| Kode | task 00 selesai — `Makefile`, config, logger, clock, id, pool pgx, chi, `/healthz`, graceful shutdown, air, golangci-lint |

## Langkah berikutnya

Kerjakan [`docs/tasks/01-database-migrations.md`](tasks/01-database-migrations.md).
Satu task per sesi, berurutan.

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
- Migrasi database belum ada, jadi `make setup` / `make run` (bagian connect
  DB) butuh Postgres jalan dan `DATABASE_URL` valid. `make db-up` dkk masih
  akan melaporkan goose tidak menemukan file migrasi — itu wajar sampai
  task 01 selesai.

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
