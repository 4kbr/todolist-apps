# AGENTS.md — apps/go-chi-api

Aturan khusus untuk kode Go di app ini. Aturan umum repo ada di `AGENTS.md` root
dan tetap berlaku.

## Baca dulu

| File                   | Kapan                                   |
| ---------------------- | --------------------------------------- |
| `docs/ARCHITECTURE.md` | sebelum menyentuh struktur modul        |
| `docs/DECISIONS.md`    | sebelum mengubah keputusan teknis       |
| `docs/tasks/<n>.md`    | sebelum mengerjakan task tersebut       |
| `docs/openapi/`        | sebelum menambah atau mengubah endpoint |

## Aturan struktur

**Satu pintu keluar per modul.** Hanya `internal/modules/<modul>/module.go` yang
boleh dilihat dari luar modul. Jangan menambah file lain di akar modul.

**Arah dependency tidak pernah terbalik.** `adapter → application → domain`.
Kalau `domain` perlu import `application`, yang salah adalah desainnya, bukan
aturannya.

**`domain` tidak tahu dunia luar.** Tidak ada import Chi, pgx, sqlc, atau
`net/http` di dalam `domain`. Kalau butuh waktu, pakai `clock.Clock`. Kalau butuh
ID, terima dari pemanggil.

**Repository dideklarasikan sebagai interface di `domain`,** diimplementasikan di
`adapter/postgres`. Bukan sebaliknya.

**Modul tidak import modul lain.** Deklarasikan interface yang kamu butuhkan di
paket yang membutuhkannya, biarkan `cmd/api/main.go` yang menyuntikkan. Lihat
ADR-003.

**Transaksi dibuka usecase, bukan repository.** Repository menerima `context` dan
tidak peduli sedang di dalam transaksi atau tidak.

## Aturan kode

**Error selalu dibungkus dengan konteks:** `fmt.Errorf("create session: %w", err)`.
Jangan mengembalikan `err` telanjang dari lapisan dalam.

**Error domain adalah nilai sentinel atau tipe di `domain`.** Dipetakan ke HTTP
hanya di `platform/problem`. Handler tidak pernah menulis status code langsung
untuk kasus error.

**`context.Context` selalu parameter pertama.** Jangan menyimpannya di struct.

**Terima interface, kembalikan struct.** Interface dideklarasikan di sisi
pemakai, sekecil mungkin.

**Jangan ada state global.** Tidak ada `var db *pgxpool.Pool` di level paket.
Semua dependency disuntikkan lewat konstruktor.

**Nama paket pendek dan tanpa pengulangan.** `identity.Module`, bukan
`identity.IdentityModule`.

**Pesan error, nama identifier, dan pesan commit berbahasa Inggris.** Komentar
dan dokumen boleh Indonesia.

**Kode yang mudah dibaca menang atas kode yang pintar.** Kalau butuh komentar
untuk menjelaskan _apa_ yang dilakukan sebaris kode, tulis ulang barisnya.
Komentar dipakai untuk menjelaskan _kenapa_.

## SQL dan migrasi

- Migrasi dibuat lewat `make migrate-create name=<nama>`, tidak pernah ditulis manual
  di direktori migrasi.
- Migrasi yang sudah di-commit **tidak pernah disunting**. Perbaikan berarti
  migrasi baru.
- Setiap migrasi wajib punya bagian `-- +goose Down` yang benar-benar jalan.
- Setelah mengubah `queries/*.sql`, jalankan `make sqlc` dan commit hasil
  generate-nya.
- Jangan menulis SQL langsung di `application` atau `domain`.

## Testing

- Fitur baru ikut test. Usecase minimal punya satu test jalur bahagia dan satu
  jalur gagal.
- Fake repository ditulis tangan sebagai struct biasa. Jangan menambah library
  mock.
- Test yang menyentuh Postgres diberi build tag `//go:build integration` dan
  dijalankan lewat `make test-integration`.
- Test tidak bergantung pada urutan eksekusi dan tidak berbagi baris database
  dengan test lain.

## Sebelum bilang selesai

```bash
make lint
make test
make test-integration     # kalau menyentuh SQL, repo, atau handler
make openapi-lint         # kalau menyentuh kontrak
```

Semua harus hijau. Jangan mengklaim selesai tanpa menjalankannya.

Lalu perbarui `docs/HANDS-OFF.md` dan catat konteks di `temp/contexts/`.

## Yang belum jadi

Fungsi yang belum diimplementasikan sengaja `panic` sambil menunjuk file task-nya:

```go
panic("belum diimplementasikan - lihat docs/tasks/06-task-domain.md")
```

Jangan dihapus massal, jangan diganti stub yang mengembalikan nilai kosong.
Ganti hanya kalau kamu memang sedang mengerjakan task itu.
