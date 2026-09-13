# HANDS-OFF — apps/go-chi-api

Keadaan app ini sekarang. Ini **state**, bukan history — kalau sesuatu berubah,
timpa barisnya, jangan ditumpuk.

## Status

Perencanaan selesai. **Belum ada satu baris kode Go.**

| Bagian | Keadaan |
| --- | --- |
| Dokumen arsitektur | selesai — `docs/ARCHITECTURE.md` |
| Keputusan teknis | selesai — `docs/DECISIONS.md`, ADR-001 s.d. ADR-013 |
| Aturan kode | selesai — `AGENTS.md` |
| Daftar task | selesai — `docs/tasks/`, 10 task dalam 5 phase |
| Kontrak OpenAPI | kosong — diisi task 03 |
| Kode | belum ada — mulai dari task 00 |

## Langkah berikutnya

Kerjakan [`docs/tasks/00-foundation.md`](tasks/00-foundation.md). Satu task per
sesi, berurutan.

## Yang perlu diketahui sebelum mulai

- `go.mod` sekarang memakai module path `apps/go-chi-api` dan `go 1.26.5`.
  Task 00 mengangkat pertanyaan apakah module path diganti jadi path unik
  (`github.com/<user>/todolist/apps/go-chi-api`). Ini keputusan user.
- `Makefile` root meneruskan target ke app ini, tapi **Makefile app-nya belum ada**.
  Jadi semua target (`make dev`, `make test`, dst) belum jalan sampai task 00 selesai.
- `Makefile` root masih punya target `dev-worker` dan `run-worker` warisan template.
  Project ini tidak punya worker. Task 00 menghapusnya.
- Migrasi database belum ada, jadi `make setup` akan melaporkan goose tidak
  menemukan file migrasi. Itu wajar sampai task 01 selesai.

## Target Makefile

Belum ada yang jalan. Daftar final diisi setelah task 00 dan diperbarui di task 09.

## Batasan

- `apps/cms` tidak disentuh dari sini.
- `docs/openapi/` adalah kontrak beku dengan `apps/cms`. Mengubahnya butuh version
  bump dan pemberitahuan ke kedua sisi.
