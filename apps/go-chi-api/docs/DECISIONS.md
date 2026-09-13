# Catatan Keputusan Arsitektur

Setiap entri mencatat sebuah keputusan, alternatif yang dipertimbangkan, dan
alasan pemilihannya. Format ini sengaja dipakai supaya alasan di balik kode
masih bisa dilacak berbulan-bulan kemudian — termasuk oleh diri sendiri.

Status: `diterima` · `ditinjau ulang` · `diganti oleh ADR-XXX`

---

## ADR-001 — Modular monolith, batas dijaga `internal/` bertingkat

**Status:** diterima

**Konteks.** Dua area domain yang jelas terpisah: identitas dan tugas. Godaan
memecahnya jadi service terpisah selalu ada karena terdengar lebih "scalable".
Di sisi lain, monolith berlapis tanpa modul batasnya selalu luntur dalam
hitungan minggu.

**Keputusan.** Satu codebase, satu database. Tiap modul tinggal di
`internal/modules/<nama>/`, dan isi dalamnya disembunyikan di
`internal/modules/<nama>/internal/`. Satu-satunya pintu keluar adalah
`module.go`.

**Alternatif.**
- *Microservice sejak awal* — batas keras, tapi menambah service discovery,
  transaksi terdistribusi, dan dua pipeline deployment untuk produk tanpa pengguna.
- *Layered monolith (`handlers/`, `services/`, `repositories/`)* — paling
  familiar, tapi memotong kode berdasarkan lapisan teknis, bukan domain. Semua
  service bisa memanggil semua repository, jadi tidak ada batas sama sekali.

**Konsekuensi.** `go build` yang menolak kalau batas dilanggar, bukan reviewer.
Harganya: satu lapis direktori tambahan yang terasa berlebihan di awal.

---

## ADR-002 — `module.go` sebagai satu-satunya permukaan publik modul

**Status:** diterima

**Konteks.** Perlu satu tempat yang jelas untuk menjawab "apa yang boleh dipakai
modul lain dari modul ini".

**Keputusan.** Tiap modul punya `module.go` di akar modul yang memuat `Module`,
`New(Deps)`, `Routes()`, dan interface publik. Tidak ada file lain di akar modul.

**Alternatif.**
- *`factory.go`* — namanya menyiratkan tugasnya cuma merakit objek, padahal isinya
  kontrak. Nama yang salah mengarahkan orang menaruh hal yang salah di dalamnya.
- *Beberapa file publik di akar modul* — permukaan modul jadi kabur; tidak ada
  satu tempat untuk membacanya sekaligus.

**Konsekuensi.** Mau tahu sebuah modul menyediakan apa, cukup baca satu file.

---

## ADR-003 — Modul lain tidak di-import langsung; interface dideklarasikan konsumen

**Status:** diterima

**Konteks.** Kalau modul A import modul B dan suatu saat B butuh A, muncul import
cycle yang hanya bisa diselesaikan dengan membongkar salah satunya.

**Keputusan.** Modul yang **butuh** sesuatu mendeklarasikan interface-nya sendiri,
sekecil mungkin. `main.go` yang menyuntikkan implementasi. Idiom Go:
"accept interfaces, return structs".

Di MVP koplingnya cuma satu: `platform/middleware` mendeklarasikan
`TokenVerifier`, dan `identity.Module` kebetulan memenuhinya.

**Alternatif.**
- *Import `identity` langsung dari middleware* — lebih pendek, tapi mengunci
  middleware ke satu implementasi auth dan bikin middleware mustahil di-test
  tanpa modul identity.
- *Event bus internal* — memutus kopling paling jauh, tapi menukar panggilan yang
  bisa dilacak compiler dengan panggilan yang baru ketahuan salah saat runtime.

**Konsekuensi.** Modul `task` sama sekali tidak menyentuh `identity`. `userID`
datang dari context, integritas dari foreign key. Kopling tidak dikarang supaya
terlihat modular.

---

## ADR-004 — List dan Todo satu modul, bukan dua

**Status:** diterima

**Konteks.** Terdengar wajar memberi tiap entity satu modul. Tapi List dan Todo
selalu berubah bersama, di-query bersama, dan terikat foreign key dengan cascade.

**Keputusan.** Satu modul `task`. `List` adalah agregat root, `Todo` hidup di
dalamnya.

**Alternatif.**
- *Modul `list` + modul `todo`* — lebih banyak latihan komunikasi antar modul,
  tapi keduanya butuh transaksi yang sama. Batas yang harus dilubangi terus
  bukan batas.

**Konsekuensi.** Hanya dua modul domain di MVP. Itu cukup untuk melatih batas
modul tanpa memecah yang memang satu.

---

## ADR-005 — JWT access token + refresh token opaque di database

**Status:** diterima

**Konteks.** "JWT" dan "session" sering dianggap dua pilihan yang bersaing.
JWT stateless cepat diverifikasi tapi tidak bisa dicabut. Session database bisa
dicabut tapi menyentuh DB tiap request.

**Keputusan.** Pakai dua-duanya untuk peran yang berbeda. Access token JWT HS256
umur 15 menit, tidak disimpan di mana pun. Refresh token acak 32 byte, umur 30
hari, disimpan di tabel `sessions` — yang disimpan SHA-256-nya, bukan tokennya.

Refresh dirotasi: sekali pakai, langsung dicabut dan diganti. Kalau refresh token
yang sudah dicabut muncul lagi, seluruh session user itu dicabut — itu tanda
token bocor.

**Alternatif.**
- *Session cookie server-side saja* — paling aman by default dan paling sederhana,
  tapi tidak mengajarkan apa pun soal JWT.
- *JWT stateless murni* — logout jadi bohong: token tetap sah sampai kedaluwarsa.

**Konsekuensi.** Verifikasi request normal tidak menyentuh database. Harganya:
access token tetap tidak bisa dicabut dalam jendela 15 menit. Itu memang sifat
JWT, dan jendelanya sengaja dibuat sempit.

---

## ADR-006 — Access token di header, refresh token di cookie HttpOnly

**Status:** diterima

**Konteks.** Menaruh token di `localStorage` membuatnya bisa dibaca skrip apa pun
yang berhasil disuntikkan. Menaruh semuanya di cookie mewajibkan proteksi CSRF dan
menyulitkan klien non-browser.

**Keputusan.** Access token dikirim `Authorization: Bearer <token>`. Refresh token
dikirim sebagai cookie `HttpOnly; Secure; SameSite=Strict; Path=/auth/refresh`.

**Alternatif.**
- *Dua-duanya cookie* — paling aman dari XSS, tapi wajib token CSRF dan bikin
  `curl` serta test integrasi merepotkan.
- *Dua-duanya di body* — paling gampang di-test, tapi memaksa frontend menyimpan
  refresh token di tempat yang bisa dibaca JavaScript.

**Konsekuensi.** XSS paling banter mencuri access token 15 menit, tidak bisa
menyentuh refresh token. `Path` yang sempit berarti cookie itu tidak ikut terkirim
di request biasa. CSRF tidak jadi masalah karena endpoint lain tidak pakai cookie.

---

## ADR-007 — sqlc + pgx, bukan ORM

**Status:** diterima

**Konteks.** Menulis `rows.Scan()` manual itu boilerplate dan mudah salah urutan
kolom. ORM menghilangkan boilerplate tapi ikut menyembunyikan SQL-nya.

**Keputusan.** Query ditulis sebagai SQL asli, `sqlc` menghasilkan kode Go
type-safe dari SQL itu, dijalankan di atas pgx v5.

**Alternatif.**
- *pgx + Scan manual* — paling transparan, tapi tiap penambahan kolom berarti
  menyunting beberapa tempat dan salahnya baru ketahuan saat runtime.
- *GORM* — paling cepat menulis CRUD, tapi query yang dihasilkan sulit diprediksi
  dan tujuan project ini justru memahami SQL yang benar-benar jalan.

**Konsekuensi.** Salah nama kolom ketahuan saat `sqlc generate`, bukan saat
request masuk. Harganya: satu langkah generate yang wajib dijalankan ulang tiap
query berubah.

---

## ADR-008 — Kode hasil sqlc ikut terkurung di dalam modul

**Status:** diterima

**Konteks.** Cara paling umum memakai sqlc adalah satu direktori query global dan
satu paket `db` hasil generate yang dipakai semua orang.

**Keputusan.** Tiap modul punya direktori query dan output sqlc sendiri di
`internal/modules/<modul>/internal/adapter/postgres/`. `sqlc.yaml` punya satu
entry per modul. Migrasi tetap satu direktori bersama di `db/migrations/`.

**Alternatif.**
- *Paket `db` global* — lebih sedikit konfigurasi, tapi jadi pintu belakang:
  modul `task` bisa memanggil query `users` tanpa melanggar aturan import apa pun.

**Konsekuensi.** Batas modul tetap utuh sampai ke lapisan database. Harganya:
`sqlc.yaml` tumbuh satu entry tiap modul baru.

---

## ADR-009 — UUIDv7 sebagai primary key

**Status:** diterima

**Konteks.** Primary key berurutan (`bigint identity`) bocor ke publik: pengguna
bisa menebak jumlah data dan menelusuri resource satu per satu. UUIDv4 tidak bocor,
tapi acak total sehingga sisipan tersebar ke seluruh index B-tree.

**Keputusan.** UUIDv7, dibuat di sisi Go, disimpan di kolom `uuid` native Postgres.

**Alternatif.**
- *`bigint identity`* — paling kecil dan paling cepat, tapi ID publik yang
  berurutan adalah kebocoran informasi.
- *`bigint` internal + ULID publik* — paling optimal, tapi dua identitas untuk satu
  entity adalah sumber bug yang tidak sebanding manfaatnya sekarang.

**Konsekuensi.** ID aman dibuka ke publik dan tetap berurutan waktu, jadi index
tidak terfragmentasi seperti UUIDv4. Harganya: 16 byte per baris, dan ID tidak
enak dibaca saat debugging manual.

---

## ADR-010 — Error API memakai RFC 9457 `application/problem+json`

**Status:** diterima

**Konteks.** Setiap project cenderung mengarang format error sendiri, dan tiap
klien harus mempelajari format karangan itu dari nol.

**Keputusan.** Semua error memakai `application/problem+json` sesuai RFC 9457:
`type`, `title`, `status`, `detail`, `instance`, ditambah `errors[]` untuk
kesalahan validasi per field. Satu tempat memetakan error domain ke bentuk ini:
`platform/problem`.

**Alternatif.**
- *Envelope `{success, data, error}` untuk semua response* — familiar, tapi melawan
  semantik HTTP: status code jadi tidak bermakna dan payload sukses bersarang
  tanpa guna.
- *Envelope error karangan sendiri* — lebih pendek, tapi mendesain ulang sesuatu
  yang sudah distandarkan.

**Konsekuensi.** Handler cukup memanggil `problem.Write(w, r, err)`. Detail
internal tidak pernah bocor ke klien; yang masuk log adalah error aslinya.

---

## ADR-011 — `todos.user_id` didenormalisasi

**Status:** diterima

**Konteks.** Pemilik todo sebenarnya bisa diturunkan lewat `todos.list_id → lists.user_id`.
Tapi hampir setiap query harus di-scope ke pemilik, jadi join itu terjadi di
semua tempat.

**Keputusan.** `todos` menyimpan `user_id` sendiri, dengan foreign key ke `users`.

**Alternatif.**
- *Selalu join ke `lists`* — normalisasi murni, tidak ada risiko data tidak
  konsisten, tapi setiap query scoping bayar satu join dan setiap index jadi
  lebih rumit.

**Konsekuensi.** Query scoping jadi satu predikat sederhana dan bisa diindeks
langsung. Harganya: `todos.user_id` wajib ikut berubah saat todo dipindahkan
antar list, dan itu harus dijaga di satu tempat (usecase pindah list) plus diuji.

---

## ADR-012 — OpenAPI ditulis lebih dulu, bukan di-generate dari kode

**Status:** diterima

**Konteks.** `AGENTS.md` menyebut `docs/openapi/` sebagai kontrak beku antara
backend dan `apps/cms`. Kontrak yang di-generate dari kode tidak bisa beku — dia
berubah tiap kode berubah.

**Keputusan.** `docs/openapi/openapi.yaml` ditulis sebelum handler dibuat, dan
di-lint di CI. Handler menyesuaikan spec, bukan sebaliknya.

**Alternatif.**
- *Code-first (swaggo)* — spec tidak pernah basi, tapi anotasi mengotori handler
  dan kontrak kehilangan sifat mengikatnya.
- *Ditulis belakangan* — paling cepat mulai, tapi CMS tidak punya pegangan dan
  spec cenderung tidak pernah benar-benar ditulis.

**Konsekuensi.** CMS bisa mulai dikerjakan sebelum backend selesai. Harganya:
mengubah kontrak butuh version bump dan pemberitahuan ke kedua sisi.

---

## ADR-013 — Integration test pakai Postgres asli lewat testcontainers

**Status:** diterima

**Konteks.** Repository yang di-mock hanya membuktikan bahwa mock-nya dipanggil.
Salah SQL, salah constraint, dan salah migrasi tidak akan pernah tertangkap.

**Keputusan.** Domain dan usecase di-unit test dengan fake repository tulis tangan.
Repository dan handler di-test lawan Postgres asli yang dinyalakan
`testcontainers-go`, dengan migrasi goose dijalankan di awal. Dijaga build tag
`integration` supaya `make test` tetap cepat.

**Alternatif.**
- *Unit test saja* — cepat dan tanpa Docker, tapi bug SQL lolos ke produksi.
- *SQLite in-memory* — cepat, tapi dialeknya beda; `citext`, `inet`, dan `uuid`
  tidak ada. Lulus test tidak berarti jalan di Postgres.

**Konsekuensi.** Bug SQL ketahuan di CI. Harganya: butuh Docker dan test
integrasi berjalan lebih lambat.
