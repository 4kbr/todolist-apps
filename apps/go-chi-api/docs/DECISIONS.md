# Catatan Keputusan Arsitektur

Setiap entri mencatat sebuah keputusan, alternatif yang dipertimbangkan, dan
alasan pemilihannya. Format ini sengaja dipakai supaya alasan di balik kode
masih bisa dilacak berbulan-bulan kemudian — termasuk oleh diri sendiri.

Status: `diterima` · `ditinjau ulang` · `diganti oleh ADR-XXX`

---

<!-- EXAMPLE

## ADR-001 — Modular monolith, bukan microservice

**Status:** diterima

**Konteks.** Produk ini punya empat area domain yang jelas terpisah. Godaan
untuk langsung memisahkannya jadi service terpisah cukup besar, apalagi karena
itu terdengar lebih "scalable".

**Keputusan.** Satu codebase, satu database, batas modul dipaksakan lewat
direktori `internal/` milik Go.

**Alternatif.**
- *Microservice sejak awal* — memberi batas yang keras, tapi menambah
  service discovery, distributed transaction, dan empat pipeline deployment
  untuk produk yang belum punya pengguna.
- *Layered monolith tanpa modul* — lebih cepat di awal, tapi batasnya akan
  luntur dalam hitungan minggu dan tidak bisa dipisah nanti tanpa penulisan ulang.

**Konsekuensi.** Kita membayar sedikit disiplin di awal untuk mendapat opsi
memisahkan nanti. Kalau satu modul benar-benar butuh di-scale sendiri, kontrak
publiknya sudah menjadi batas yang jelas.

---
-->
