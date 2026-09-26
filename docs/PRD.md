# PRD — Todolist

## Kenapa project ini ada

Tujuan utamanya belajar membangun backend Go yang benar: idiom yang lazim,
batas modul yang ditegakkan, autentikasi yang tidak asal jalan, dan test yang
benar-benar menangkap bug. Todolist dipilih sebagai bahan latihan karena
domainnya cukup kecil untuk dipahami seluruhnya, tapi cukup nyata untuk
memaksa keputusan sungguhan — kepemilikan data, pagination, transaksi,
rotasi token.

Konsekuensinya: kalau ada dua cara mengerjakan sesuatu, yang dipilih adalah
yang mengajarkan hal benar, bukan yang paling cepat jadi.

## Pengguna

Satu jenis pengguna: orang yang mencatat pekerjaannya sendiri. Tidak ada
peran admin, tidak ada tim.

## Ruang lingkup

**Akun**
- Register dengan email dan password
- Login, logout
- Refresh session tanpa login ulang
- Lihat profil sendiri

**List**
- Buat, ubah nama, hapus list
- Lihat semua list milik sendiri
- Menghapus list ikut menghapus todo di dalamnya

**Todo**
- Buat todo di dalam sebuah list
- Ubah judul, catatan, prioritas, tenggat
- Tandai selesai dan batal selesai
- Hapus todo
- Pindahkan todo antar list

**Membaca daftar todo**
- Saring berdasarkan status, list, dan rentang tenggat
- Urutkan berdasarkan tenggat, prioritas, atau waktu dibuat
- Pagination

**Yang berlaku di semua endpoint**
- Pengguna hanya pernah melihat datanya sendiri
- Error memakai format yang sama di seluruh API
- Kontrak API tertulis di OpenAPI sebelum handler-nya dibuat

## Non-goal

Ini keputusan produk, bukan fitur yang belum sempat. Jangan ditambahkan
meskipun terlihat mudah.

- **Berbagi list dan kolaborasi tim.** Mengubah seluruh model kepemilikan dan
  otorisasi. Kalau suatu saat dibutuhkan, itu penulisan ulang yang disengaja,
  bukan tempelan.
- **Todo berulang dan sub-todo.** Menambah kerumitan penjadwalan dan struktur
  pohon yang tidak mengajarkan hal baru untuk tujuan project ini.
- **Notifikasi dan pengingat.** Butuh scheduler, antrean, dan integrasi kirim
  pesan — project sendiri, bukan bagian dari ini.
- **Realtime sync dan websocket.** REST cukup. Realtime mengubah bentuk seluruh
  transport.
- **Lampiran file.** Butuh object storage dan kebijakan retensi.
- **Login pihak ketiga (OAuth, SSO).** Justru menyembunyikan mekanisme auth yang
  ingin dipelajari.
- **Fitur AI apa pun.** Tidak ada isi data pengguna yang dikirim ke penyedia AI.
- **Soft delete dan pemulihan.** Hapus berarti hapus.

## Selesai kalau

- Semua endpoint di `apps/go-chi-api/docs/openapi/` jalan dan sesuai spec
- `make test` dan `make test-integration` hijau
- `make lint` bersih
- CI hijau, dan `docker compose up` menyalakan API siap pakai
- `apps/cms` bisa mulai dikerjakan hanya berbekal file OpenAPI
