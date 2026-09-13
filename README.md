# Todolist

Todolist multi-user. Backend Go + Chi berbentuk modular monolith, CMS menyusul
setelah API stabil.

Project ini dipakai untuk belajar membangun backend Go yang benar — lihat
[`docs/PRD.md`](docs/PRD.md) untuk alasan dan batasannya.

## Struktur

```
docs/PRD.md            apa yang dibangun dan kenapa
apps/go-chi-api/       Go + Chi, modular monolith
apps/cms/              CMS - belum dikerjakan, menunggu API stabil
```

## Menjalankan

**Prasyarat:** Go 1.26+, Docker, `make`.

```bash
make tools   # air, goose, sqlc, golangci-lint
make setup   # salin .env, nyalakan Postgres, jalankan migrasi
make dev     # http://localhost:8080, dengan live reload
```

`curl localhost:8080/healthz` harus menjawab `{"status":"ok"}`.

Semua target dijalankan dari root — tidak perlu `cd apps/go-chi-api`. Daftar
lengkapnya ada di
[`apps/go-chi-api/docs/HANDS-OFF.md`](apps/go-chi-api/docs/HANDS-OFF.md).

## Mulai dari mana

1. [`docs/PRD.md`](docs/PRD.md) — ruang lingkup dan non-goal
2. [`apps/go-chi-api/docs/ARCHITECTURE.md`](apps/go-chi-api/docs/ARCHITECTURE.md) — bentuk kodenya
3. [`apps/go-chi-api/docs/DECISIONS.md`](apps/go-chi-api/docs/DECISIONS.md) — kenapa bentuknya begitu
4. [`apps/go-chi-api/docs/tasks/`](apps/go-chi-api/docs/tasks/README.md) — urutan pengerjaan

## Status

Perencanaan selesai. Arsitektur, skema, dan sepuluh task sudah tertulis.
Kode belum ada — mulai dari task `00-foundation.md`.

Setiap fungsi yang belum diimplementasikan `panic` dengan pesan yang menunjuk ke
file task yang menjelaskannya.
