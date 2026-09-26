# Task 01 — Database & Migrations

**Phase:** 1 — Fondasi
**Bergantung pada:** Task 00 — Foundation
**Status:** sudah dikerjakan, belum direview agent

## Tujuan

Setelah task ini selesai, `make migrate-up` membuat seluruh skema (`users`,
`sessions`, `lists`, `todos`) di database kosong, `make migrate-down` membalikkannya
bersih sampai kosong lagi, dan `sqlc.yaml` sudah terpasang dengan satu entry
per modul domain (`identity`, `task`) walau belum ada satu query pun ditulis.
Query sungguhan menyusul di task 05 (`identity`) dan task 07 (`task`).

## Rujukan

- `ARCHITECTURE.md` bagian "Tabel" dan "Database"
- ADR-007 (sqlc + pgx, bukan ORM)
- ADR-008 (kode hasil sqlc terkurung per modul) — alasan `sqlc.yaml` punya
  satu entry per modul, bukan satu paket `db` global
- ADR-009 (UUIDv7 sebagai primary key)
- ADR-011 (`todos.user_id` didenormalisasi)
- `AGENTS.md` bagian "SQL dan migrasi"

## File yang dibuat atau disentuh

| Path                                                                                   | Kenapa                                          |
| -------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `apps/go-chi-api/db/migrations/00001_create_users.sql`                                 | tabel `users` + extension `citext`              |
| `apps/go-chi-api/db/migrations/00002_create_sessions.sql`                              | tabel `sessions`                                |
| `apps/go-chi-api/db/migrations/00003_create_lists.sql`                                 | tabel `lists`                                   |
| `apps/go-chi-api/db/migrations/00004_create_todos.sql`                                 | tabel `todos`                                   |
| `apps/go-chi-api/sqlc.yaml`                                                            | konfigurasi sqlc, satu entry per modul          |
| `apps/go-chi-api/internal/modules/identity/internal/adapter/postgres/queries/.gitkeep` | direktori query identity, kosong sampai task 05 |
| `apps/go-chi-api/internal/modules/task/internal/adapter/postgres/queries/.gitkeep`     | direktori query task, kosong sampai task 07     |

## Langkah

### 1. Buat migrasi lewat goose, jangan tulis nama file manual

```bash
make migrate-create name=create_users
make migrate-create name=create_sessions
make migrate-create name=create_lists
make migrate-create name=create_todos
```

Ini menghasilkan nama file dengan timestamp goose (`NNNNN_create_users.sql`
dst, urut). Isi tiap file sesuai DDL di langkah 2-5. Nama file di tabel atas
adalah ilustrasi urutan — nomor asli mengikuti apa yang goose hasilkan.

### 2. `00001_create_users.sql`

```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name          text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
DROP EXTENSION IF EXISTS citext;
```

`citext` dipakai untuk `email` supaya perbandingan dan constraint unique
case-insensitive terjadi di database, bukan dihafal tiap kali menulis query
(`lower(email) = lower($1)` gampang lupa ditulis konsisten di satu tempat
tapi lupa di tempat lain).

`pgcrypto` sengaja TIDAK dipasang. Tidak ada satu pun kolom yang memakainya:
ID dibuat di Go, password di-hash argon2id di Go, refresh token di-hash
SHA-256 di Go. Extension yang tidak dipakai tetap harus dimigrasikan,
di-backup, dan ikut naik versi — jangan pasang apa pun "jaga-jaga".

`id` tidak punya `DEFAULT gen_random_uuid()` — ID UUIDv7 dibuat di sisi Go
lewat `internal/platform/id` (task 00), bukan di database, karena UUIDv7
butuh timestamp yang tersemat dan Postgres native belum generate v7 tanpa
extension tambahan (ADR-009).

### 3. `00002_create_sessions.sql`

```sql
-- +goose Up
CREATE TABLE sessions (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  bytea NOT NULL UNIQUE,
    user_agent          text,
    ip                  inet,
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);

-- +goose Down
DROP TABLE sessions;
```

`refresh_token_hash` bertipe `bytea`, bukan `text` — yang disimpan adalah
SHA-256 mentah (32 byte biner) dari refresh token (ADR-005), bukan hex atau
base64-nya. Encoding jadi teks hanya buang tempat dan CPU tanpa manfaat kalau
kolomnya tidak pernah dibaca manusia langsung. `UNIQUE` mencegah hash token
yang sama tersimpan di dua baris. Sekali pakai ditegakkan oleh rotasi token
dan `revoked_at` di aplikasi, bukan oleh constraint ini.

### 4. `00003_create_lists.sql`

```sql
-- +goose Up
CREATE TABLE lists (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        text NOT NULL,
    position    integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

-- +goose Down
DROP TABLE lists;
```

### 5. `00004_create_todos.sql`

```sql
-- +goose Up
CREATE TABLE todos (
    id            uuid PRIMARY KEY,
    list_id       uuid NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         text NOT NULL,
    notes         text,
    status        text NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'done')),
    priority      integer NOT NULL DEFAULT 0,
    due_at        timestamptz,
    completed_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX todos_user_status_due_idx ON todos(user_id, status, due_at);
CREATE INDEX todos_list_created_id_idx ON todos(list_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE todos;
```

`status` pakai `text` + `CHECK`, bukan `CREATE TYPE status_enum AS ENUM (...)`.
Enum Postgres kelihatan lebih "type-safe" di awal, tapi menambah nilai baru ke
enum yang sudah ada butuh `ALTER TYPE ... ADD VALUE` yang tidak bisa dijalankan
di dalam transaksi yang sama dengan pemakaiannya, dan menghapus nilai enum
sama sekali tidak didukung langsung — perlu buat tipe baru dan migrasi ulang
seluruh kolom. `CHECK` di kolom `text` diubah dengan `ALTER TABLE ... DROP
CONSTRAINT` + `ADD CONSTRAINT`, satu migrasi biasa, tidak ada kasus khusus.
Harga yang dibayar: validasi nilai tidak muncul di tipe kolom saat `\d todos`,
tapi tetap ditegakkan database di setiap write.

`todos_list_created_id_idx` memakai `(list_id, created_at DESC, id DESC)`
karena ini index yang dipakai keyset pagination "daftar todo per list, terbaru
dulu" (lihat task 02 dan task 07) — urutan kolom index harus cocok persis
dengan `ORDER BY` supaya Postgres bisa index-scan tanpa sort tambahan.

### 6. Kenapa semua timestamp `timestamptz`

`timestamp` (tanpa `tz`) menyimpan angka jam-menit-detik apa adanya, tanpa
zona waktu — kalau server dan klien beda zona, atau server pernah pindah
zona (mis. deploy region baru), nilai yang tersimpan jadi ambigu: `14:00` itu
14:00 di zona mana? `timestamptz` disimpan Postgres sebagai UTC secara
internal dan dikonversi ke zona sesi saat dibaca, jadi nilainya selalu
merujuk satu titik waktu absolut yang sama di mana pun dibaca. Ini bukan
soal gaya — proyek yang salah pilih `timestamp` biasanya baru sadar saat
data lintas zona sudah telanjur salah dan tidak bisa diperbaiki tanpa tahu
zona asli tiap baris.

### 7. `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/modules/identity/internal/adapter/postgres/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "postgres"
        out: "internal/modules/identity/internal/adapter/postgres"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "timestamptz"
            go_type: "*time.Time"
            nullable: true
  - engine: "postgresql"
    queries: "internal/modules/task/internal/adapter/postgres/queries"
    schema: "db/migrations"
    gen:
      go:
        package: "postgres"
        out: "internal/modules/task/internal/adapter/postgres"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "timestamptz"
            go_type: "*time.Time"
            nullable: true
```

Dua entry, satu per modul domain (ADR-008): `identity` dan `task`. Masing-masing
punya `queries` dan `out` sendiri di dalam `internal/modules/<modul>/internal/
adapter/postgres/`, walau `schema` yang dirujuk sama-sama `db/migrations` —
skema database memang satu, tapi kode Go yang dihasilkan terkurung per modul.
Kalau ada paket `db` global yang dipakai bersama, modul `task` bisa
memanggil query `users` tanpa `go build` pernah komplain — itu justru pintu
belakang yang meniadakan gunanya `internal/` bertingkat.

Override nullable pada konfigurasi ini hanya berlaku untuk kolom
`timestamptz` yang nullable (`revoked_at`, `due_at`, `completed_at`), yang
dipetakan ke `*time.Time`. Kolom nullable bertipe `text` dan `inet` belum
punya override; tipe Go hasilnya mengikuti default sqlc dan perlu diperiksa
setelah query task 05 dan task 07 ditulis.

### 8. Direktori query kosong

Buat direktori query tiap modul supaya `sqlc generate` (target `make sqlc`
dari task 00) tidak gagal karena path `queries:` tidak ada:

```bash
mkdir -p internal/modules/identity/internal/adapter/postgres/queries
mkdir -p internal/modules/task/internal/adapter/postgres/queries
touch internal/modules/identity/internal/adapter/postgres/queries/.gitkeep
touch internal/modules/task/internal/adapter/postgres/queries/.gitkeep
```

Belum ada file `.sql` query apa pun di sini. Pada sqlc v1.31.1, `make sqlc`
gagal dengan `no queries contained in paths` selama direktori query masih
kosong. Jalankan ulang setelah task 05 dan task 07 mengisi query masing-masing;
jangan tambah query placeholder hanya untuk membuat perintah ini lulus.

## Kriteria selesai

- [x] `make migrate-up` sukses dari database kosong, keempat tabel muncul di
      `\dt` psql
- [x] `make migrate-status` menunjukkan keempat migrasi berstatus applied
- [x] `make migrate-down` empat kali (atau `make migrate-reset`) mengembalikan database
      ke kosong tanpa error — termasuk `DROP EXTENSION`
- [x] `psql -c '\d todos'` menunjukkan CHECK constraint pada kolom `status`
- [x] `psql -c '\di'` menunjukkan `sessions_user_id_idx`,
      `todos_user_status_due_idx`, `todos_list_created_id_idx`, dan unique
      index `lists_user_id_name_key` (nama otomatis dari `UNIQUE(user_id, name)`)
- [ ] `make sqlc` jalan tanpa error setelah task 05 dan task 07 menambah query

## Jebakan

- Urutan migrasi penting: `sessions`, `lists`, `todos` semua punya foreign
  key ke `users`, dan `todos` juga ke `lists`. Kalau nomor urut goose
  tertukar, `migrate-up` gagal dengan error foreign key ke tabel yang belum ada.
- Jangan menyunting migrasi yang sudah pernah `migrate-up` lalu di-commit. Kalau
  ada salah kolom setelah commit, migrasi baru yang memperbaiki, bukan
  edit file lama (`AGENTS.md`).
- `CREATE EXTENSION IF NOT EXISTS citext` harus ada sebelum kolom pertama
  yang memakainya dideklarasikan di migrasi yang sama — taruh di baris
  paling atas `-- +goose Up`.
- `+goose Down` yang melakukan `DROP EXTENSION` bisa gagal kalau ada tabel
  lain (dari migrasi yang belum di-down) masih memakai tipe dari extension
  itu — makanya urutan down harus persis kebalikan urutan up. `make migrate-reset`
  menangani ini otomatis; jangan `migrate-down` manual dengan urutan acak.
- `sqlc.yaml` versi `"2"` punya skema field yang beda dari versi `"1"` —
  jangan campur contoh dari dokumentasi versi berbeda saat menambah field
  baru nanti.
- Index `(list_id, created_at DESC, id DESC)` hanya berguna kalau query task
  07 benar-benar `ORDER BY created_at DESC, id DESC` — kalau urutan `ORDER
BY` berubah nanti, index ini harus ikut berubah, jangan dibiarkan basi.

## Catatan saat pengerjaan

- `make sqlc` gagal saat dijalankan, lognya:

  ```bash
  $ make sqlc
  Menjalankan sqlc generate
  sqlc generate
  # package postgres
  error parsing queries: no queries contained in paths /.../apps/go-chi-api/internal/modules/identity/internal/adapter/postgres/queries
  # package postgres
  error parsing queries: no queries contained in paths /.../apps/go-chi-api/internal/modules/task/internal/adapter/postgres/queries
  make: *** [Makefile:74: sqlc] Error 1
  ```

- nama migrations file bukan 00001, 0002, 0003, dll tapi sesuai goose timestamp (`20240606123456_create_users.sql` dst) — urutan di tabel atas hanya ilustrasi
