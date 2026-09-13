# Standup Bot

Standup harian yang menulis dirinya sendiri. Memantau aktivitas GitHub tim,
meringkasnya dengan LLM, mengirimkannya ke channel chat pada jadwal yang
ditentukan.

## Struktur

```
docs/PRD.md          apa yang dibangun dan kenapa
apps/backend/        Go, modular monolith
apps/frontend/       CMS - belum dikerjakan
```

## Menjalankan

**Prasyarat:** Go 1.25+, Docker, `make`.

```bash
make tools   # air, goose, golangci-lint
make setup   # salin .env, nyalakan Postgres, jalankan migrasi
make dev     # http://localhost:8080, dengan live reload
```

`curl localhost:8080/healthz` harus menjawab `{"status":"ok"}`.

Semua target dijalankan dari root — tidak perlu `cd apps/backend`. Daftar lengkapnya ada
di [`apps/backend/docs/HANDS-OFF.md`](apps/backend/docs/HANDS-OFF.md).

## Mulai dari mana

1. Baca [`docs/PRD.md`](docs/PRD.md) — ruang lingkup dan non-goal
2. Baca [`apps/backend/docs/ARCHITECTURE.md`](apps/backend/docs/ARCHITECTURE.md) — bentuk kodenya
3. Mulai dari [`apps/backend/docs/tasks/`](apps/backend/docs/tasks/README.md)

## Status

Fondasi selesai (task 00): konfigurasi, logging, pool Postgres, helper HTTP, dua entry
point dengan graceful shutdown, dan CI. Logika bisnis belum ada.

Setiap fungsi yang belum diimplementasikan `panic` dengan pesan yang menunjuk ke file
task yang menjelaskannya.
