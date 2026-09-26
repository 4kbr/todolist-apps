# AGENTS.md

Panduan untuk coding agent yang bekerja di repositori ini.
Manusia: mulai dari `README.md`.

## Tentang project

Todolist App. ya todo list app crud. Backendnya Go, frontendnya CMS. Backendnya modular monolith.

Monorepo dengan dua aplikasi (saat ini):

```
apps/<backend>/   , untuk saat ini apps/go-chi-api,  Go + Chi, modular monolith. Satu-satunya yang aktif dikerjakan (saat ini).
apps/cms/   CMS. Sengaja belum dikerjakan.
docs/PRD.md      Ruang lingkup dan non-goal.
```

**Bekerja di `apps/<backend>` kecuali diminta sebaliknya.** Ada `AGENTS.md`
terpisah di sana dengan aturan yang lebih spesifik — baca file itu sebelum
menyentuh kode Go.

## Sebelum mengubah apa pun

Baca dulu, sesuai relevansi:

| File                                  | Kapan dibaca                                                 |
| ------------------------------------- | ------------------------------------------------------------ |
| `docs/PRD.md`                         | Sebelum menambah fitur apa pun                               |
| `apps/<backend>/docs/ARCHITECTURE.md` | Sebelum menyentuh struktur modul                             |
| `apps/<backend>/docs/DECISIONS.md`    | Sebelum mengubah keputusan teknis                            |
| `apps/<backend>/docs/tasks/`          | Sebelum mengimplementasikan sesuatu yang bertanda belum jadi |

Project ini punya dokumentasi keputusan yang lengkap. Kalau sebuah keputusan
terlihat aneh, kemungkinan besar alasannya sudah tertulis di `DECISIONS.md` —
periksa di sana sebelum menganggapnya sebagai kesalahan yang perlu diperbaiki.

## Status implementasi

Fondasi. Kontrak modul sudah ditetapkan; sebagian besar implementasi belum ada.

Fungsi yang belum diimplementasikan sengaja `panic` dengan pesan yang menunjuk
ke file task-nya:

```go
panic("belum diimplementasikan - lihat docs/tasks/03-activity-module.md")
```

**Jangan menghapus panic ini secara massal atau menggantinya dengan stub yang
mengembalikan nilai kosong.** Panic tersebut adalah penanda pekerjaan yang
disengaja. Ganti hanya kalau kamu memang sedang mengimplementasikan task
tersebut.

## Aturan yang berlaku di seluruh repo

**Jangan mengerjakan lebih dari yang diminta.** Kalau diminta mengerjakan
task 03, jangan sekalian mengimplementasikan task 04. Task dipecah dengan
sengaja.

**Hormati non-goal.** `docs/PRD.md` mencantumkan hal-hal yang sengaja tidak
dibangun — ranking produktivitas, notifikasi realtime per event, mengirim isi
kode ke penyedia AI. Ini keputusan produk, bukan fitur yang terlewat. Jangan
menambahkannya meskipun terlihat mudah dan bermanfaat.

**Frontend tetap kosong.** `apps/cms/` menunggu API backend stabil. Jangan
membuat scaffolding di sana kecuali diminta eksplisit.

**Bahasa dokumentasi Indonesia, bahasa kode Inggris.** Komentar kode boleh
Indonesia, tapi nama identifier, pesan commit, dan pesan error tetap Inggris.

## Guide Agents

1. Gunakan skill `caveman` sebagai default respons, tanpa mengurangi kualitas jawaban.
2. Readable, maintainable, and testable code is preferred over clever code. Saya ingin kode baik yang bisa saya update sendiri tanpa harus mengandalkan AI. Jangan menulis kode yang sengaja sulit dibaca atau dipecahkan.
3. Effort `high` untuk planning, `medium` atau `low` untuk implementasi. Planning yang matang membuat implementasi murah.
4. **Subagent berbasis kriteria, bukan refleks.** Pakai subagent ketika kerja bisa dipecah paralel DAN tiap bagian menyentuh path yang berbeda — misalnya scaffolding banyak direktori sekaligus. Kerjakan sendiri kalau task-nya kecil, berurutan, atau menyentuh file yang sama: tiap subagent mulai tanpa konteks dan harus membaca ulang, jadi memecah task kecil justru lebih boros token daripada mengerjakannya langsung.
5. **Setiap subagent wajib diberi daftar path yang dia miliki eksklusif**, plus daftar path milik agent lain yang haram disentuh. Ini satu-satunya hal yang mencegah dua agent saling menimpa tulisan.
6. Setelah task selesai, perbarui `HANDS-OFF.md` milik app yang dikerjakan (`apps/<backend>/docs/HANDS-OFF.md` atau `apps/<frontend>/docs/HANDS-OFF.md`). `docs/HANDS-OFF.md` di root hanya untuk kerja level root seperti merge dan koordinasi rilis. (jika aplikasinya monorepo)
7. Simpan context ataupun catatan agent di `temp/contexts/01...md` file ini untuk refrensi agent sekaligus mencatat context saat ini, file ini mirip hands-off tapi bersifat history, bukan state. ini untuk menghemat token, karena agent tidak perlu membaca ulang semua file untuk mengetahui context saat ini. Catatan ini tidak dicommit dan bisa dihapus kapan saja.
8. Sebelum memulai implementasi update dan perubahan, selalu buat branch baru misal implement task backend 01, `git checkout -b feature/backend-task-01-....` atau `git checkout -b feature/<backend>/<task atau nama feature yang dikerjakan>` dan _tanpa di commit setelah selesai_ cukup pindah branch saja, jika error dan tidak bisa karena ada perubahan yang belum disimpan di git atau lainnya, maka lanjutkan dengan cara biasa tanpa perlu checkout
9. Jika diminta oleh user buatkan task nya di file, itu berarti buatkan tasknya dalam bentuk file di `apps/<aplikasi>/docs/...<nama-plan>md` (jika monorepo) jika hanya satu app maka cukup di root `docs/...<nama-plan>.md` untuk format isi dan nama filenya bisa pakai existing jika sudah ada atau jika belum ada, maka bisa pakai tanggal saat ini misal `2026-01-02-1....`, `2026-01-02-2....` dan isi filenya berisi misal apa saja file yang terkena impact atau yang diupdate dan file baru, panduan atau langkah-langkap detail implement-nya, bagian yang diupdate dan pengaruh atau dampak perubahannya, anggap file tersebut sebagai panduan untuk user implement sendiri. keyword: `buat tasknya menjadi file..., buatkan task untuk saya kerjakan..., buat task di file..., dan yang serupa`
10. Custom sub-agent local ada di `.agents/agents/`, tidak wajib digunakan tapi rekomendasi dipakai jika sesuai delegasi task

## Aturan lintas app

- **Kontrak beku.** `apps/<backend>/docs/openapi` adalah kesepakatan worktrees. Mengubahnya butuh version bump dan pemberitahuan ke kedua sisi — jangan menambah atau mengganti field diam-diam.
- **Satu worktree, satu app.** Agent yang bekerja di `apps/<backend>` tidak menyentuh `apps/<frontend>`, dan sebaliknya. Perubahan yang melintasi keduanya dikerjakan di level root.
- Baca juga `AGENTS.md` di dalam app yang sedang dikerjakan (jika ada) di situ aturan spesifiknya.

## Guard

- Jangan baca file environment (`.env` dan sejenisnya) tanpa izin user. Yang boleh dibaca hanya `.env.example` sebagai template.
- Jangan menambah co-author di commit tanpa diminta.
- Jangan commit atau push tanpa diminta.
- Rahasia, kredensial, dan token tidak pernah masuk repo, log, atau pesan error.
- `temp/` tidak di-commit (sudah di `.gitignore` root).
- Tidak ada kredensial sungguhan yang boleh masuk ke repositori. `.env` sudah ada di `.gitignore`; `.env.example` hanya berisi placeholder.
- Kalau menemukan sesuatu yang terlihat seperti token sungguhan di dalam kode, hentikan dan laporkan alih-alih diam-diam menggantinya.
