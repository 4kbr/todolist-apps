# Task 03 — Kontrak OpenAPI

**Phase:** 2 — Kontrak
**Bergantung pada:** 02
**Status:** belum dikerjakan

## Tujuan

Menulis kontrak API di `docs/openapi/` SEBELUM satu baris handler HTTP pun
dibuat (ADR-012). Kontrak ini adalah kesepakatan beku dengan `apps/cms`
(lihat `AGENTS.md` root, bagian "Kontrak beku"). Task 05 nanti menyesuaikan
kode ke sini, bukan sebaliknya.

## Rujukan

- `docs/PRD.md` — ruang lingkup dan non-goal (jangan menambah endpoint di
  luar ruang lingkup, mis. share list, sub-todo, OAuth).
- `apps/go-chi-api/docs/ARCHITECTURE.md` — skema tabel `users`, `sessions`,
  `lists`, `todos`; alur auth.
- `apps/go-chi-api/docs/DECISIONS.md` — ADR-005/006 (JWT + refresh cookie),
  ADR-010 (RFC 9457), ADR-012 (contract-first).
- Root `AGENTS.md` bagian "Kontrak beku".
- RFC 9457 (`application/problem+json`).

## File yang dibuat atau disentuh

- `apps/go-chi-api/docs/openapi/openapi.yaml` (entry point, boleh memecah ke
  `paths/*.yaml` dan `components/*.yaml` dengan `$ref` relatif; hasil bundle
  yang dipakai CMS ditulis ke `docs/openapi/bundle.yaml` oleh `make openapi-bundle`)
- `apps/go-chi-api/Makefile` — tambah target `openapi-lint`,
  `openapi-bundle`, `openapi-bundle-check` (target ini sudah dijanjikan
  daftar forward di root `Makefile`; kalau nama target belum ada di daftar
  forward root, tambahkan — root `Makefile` bukan milik eksklusif agent
  manapun untuk baris forward generik ini, tapi JANGAN sentuh bagian lain
  root `Makefile`)
- `apps/go-chi-api/.tool-versions` atau bagian versi pinned di dokumentasi
  tooling (lihat langkah 5) — taruh pin versi di tempat yang sudah ada
  konvensinya di repo (mis. `package.json` devDependencies kalau memakai
  npx redocly/spectral, atau komentar versi di Makefile kalau memakai binary
  Go-installable). Jangan membuat sistem pin baru yang tidak perlu.

Tidak menyentuh file Go apa pun. Tidak menjalankan `go` atau `git`.

## Langkah

1. **Tentukan tooling lint/bundle.** Pilih **Redocly CLI** (`@redoclyx/cli`
   a.k.a. `redocly`), pin versi eksplisit, mis. `redocly@1.25.11`, dijalankan
   lewat `npx --yes @redocly/cli@1.25.11 <cmd>` supaya tidak menambah
   dependency Node permanen ke repo Go ini. (Alternatif spectral juga valid,
   tapi pilih satu saja dan jangan campur — dokumen ini memakai Redocly
   sebagai contoh konkret di seluruh langkah berikut.)

   Di `apps/go-chi-api/Makefile`:

   ```makefile
   OPENAPI_ROOT  := docs/openapi/openapi.yaml
   OPENAPI_OUT   := docs/openapi/bundle.yaml
   REDOCLY       := npx --yes @redocly/cli@1.25.11

   .PHONY: openapi-lint openapi-bundle openapi-bundle-check

   openapi-lint:
   	$(REDOCLY) lint $(OPENAPI_ROOT)

   openapi-bundle:
   	$(REDOCLY) bundle $(OPENAPI_ROOT) -o $(OPENAPI_OUT)

   openapi-bundle-check: openapi-bundle
   	@git diff --exit-code -- $(OPENAPI_OUT) || \
   	  (echo "bundle.yaml basi - jalankan 'make openapi-bundle' dan commit hasilnya" && exit 1)
   ```

   `openapi-bundle-check` ada karena `bundle.yaml` adalah artefak yang
   di-commit (CMS membaca file tunggal ini, bukan struktur `$ref` yang
   terpecah). Kalau seseorang menyunting `paths/*.yaml` tapi lupa
   menjalankan `make openapi-bundle`, CMS akan membaca kontrak yang basi
   tanpa ada yang sadar — CI harus menangkap ini sebelum merge, bukan
   manusia yang mengingat langkah manual.

2. **Susun struktur file.** Boleh satu file monolitik atau dipecah:

   ```
   docs/openapi/
     openapi.yaml            entry point: info, servers, tags, $ref ke paths/components
     bundle.yaml             hasil `make openapi-bundle`, DI-COMMIT, dipakai CMS
     paths/
       auth.yaml
       lists.yaml
       todos.yaml
     components/
       schemas.yaml
       parameters.yaml
       responses.yaml
       security.yaml
   ```

   Kalau memilih file tunggal, itu juga sah — tulis catatan di
   `openapi.yaml` bagian atas bahwa struktur ini boleh dipecah nanti tanpa
   dianggap perubahan kontrak (selama hasil bundle-nya identik).

3. **Definisikan `info`, `servers`, `security` global.**

   ```yaml
   openapi: 3.1.0
   info:
     title: Todolist API
     version: 0.1.0
   servers:
     - url: /api/v1
   security:
     - bearerAuth: []
   ```

   `securitySchemes`:

   ```yaml
   components:
     securitySchemes:
       bearerAuth:
         type: http
         scheme: bearer
         bearerFormat: JWT
   ```

   `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh` dioverride
   dengan `security: []` (tidak butuh access token; refresh pakai cookie,
   bukan bearer).

4. **Schema `Problem` (RFC 9457).** Dipakai SEMUA response error lewat
   `$ref`, tidak didefinisikan ulang per endpoint.

   ```yaml
   components:
     schemas:
       Problem:
         type: object
         required: [type, title, status]
         properties:
           type:
             type: string
             format: uri
             example: "https://todolist.example/problems/validation-failed"
           title:
             type: string
             example: "Validation failed"
           status:
             type: integer
             example: 422
           detail:
             type: string
             example: "One or more fields are invalid."
           instance:
             type: string
             format: uri
             example: "/todos"
           errors:
             type: array
             description: Rincian per field, hanya diisi untuk error validasi.
             items:
               type: object
               required: [field, message]
               properties:
                 field:
                   type: string
                   example: "email"
                 message:
                   type: string
                   example: "must be a valid email address"
     responses:
       ValidationError:
         description: Input tidak valid.
         content:
           application/problem+json:
             schema:
               $ref: '#/components/schemas/Problem'
             example:
               type: "https://todolist.example/problems/validation-failed"
               title: "Validation failed"
               status: 422
               detail: "One or more fields are invalid."
               instance: "/todos"
               errors:
                 - field: "title"
                   message: "must not be empty"
                 - field: "dueAt"
                   message: "must be a valid RFC 3339 timestamp"
       Unauthorized:
         description: Token akses tidak ada, tidak valid, atau kedaluwarsa.
         content:
           application/problem+json:
             schema: { $ref: '#/components/schemas/Problem' }
       NotFound:
         description: Resource tidak ditemukan atau bukan milik pengguna ini.
         content:
           application/problem+json:
             schema: { $ref: '#/components/schemas/Problem' }
       Conflict:
         description: Konflik state, mis. email sudah dipakai.
         content:
           application/problem+json:
             schema: { $ref: '#/components/schemas/Problem' }
   ```

   Setiap operasi mereferensikan `responses.ValidationError` /
   `Unauthorized` / `NotFound` / `Conflict` lewat `$ref`, bukan menulis ulang
   schema Problem di tiap operasi.

5. **Parameter bersama: pagination keyset + filter todo.**

   ```yaml
   components:
     parameters:
       Limit:
         name: limit
         in: query
         schema: { type: integer, minimum: 1, maximum: 100, default: 20 }
       Cursor:
         name: cursor
         in: query
         description: >
           Opaque cursor dari field `nextCursor` response sebelumnya.
           Merepresentasikan posisi (created_at, id) keyset terakhir yang dibaca.
         schema: { type: string }
       TodoStatus:
         name: status
         in: query
         schema: { type: string, enum: [todo, done] }
       TodoListId:
         name: listId
         in: query
         schema: { type: string, format: uuid }
       TodoDueBefore:
         name: dueBefore
         in: query
         schema: { type: string, format: date-time }
       TodoDueAfter:
         name: dueAfter
         in: query
         schema: { type: string, format: date-time }
       TodoSort:
         name: sort
         in: query
         description: >
           Kolom pengurutan. Prefix `-` untuk descending, contoh `-dueAt`.
         schema:
           type: string
           enum: [dueAt, -dueAt, priority, -priority, createdAt, -createdAt]
           default: -createdAt
   ```

   Response list memakai amplop pagination seragam:

   ```yaml
   components:
     schemas:
       PagedTodos:
         type: object
         required: [items, nextCursor]
         properties:
           items:
             type: array
             items: { $ref: '#/components/schemas/Todo' }
           nextCursor:
             type: string
             nullable: true
             description: null kalau tidak ada halaman berikutnya.
   ```

6. **Endpoint auth.** `security: []` untuk register/login/refresh.

   - `POST /auth/register` — body `{ email, password, name }` → `201` body
     `User` (tanpa `passwordHash`). `409` (`Conflict`) kalau email sudah
     dipakai. `422` (`ValidationError`) kalau input tidak valid.
   - `POST /auth/login` — body `{ email, password }` → `200` body
     `{ accessToken, user }`, plus `Set-Cookie` refresh token
     (`HttpOnly; Secure; SameSite=Strict; Path=/auth/refresh`). `401`
     (`Unauthorized`) untuk kredensial salah — dokumentasikan bahwa pesan
     dan status SAMA baik email tidak ada maupun password salah (rujuk
     `04-identity-domain.md`, alasan anti-enumerasi akun).
   - `POST /auth/refresh` — tidak ada body, refresh token dibaca dari
     cookie. → `200` body `{ accessToken }` + `Set-Cookie` refresh token
     baru (rotasi). `401` kalau cookie tidak ada/kedaluwarsa/dicabut.
   - `POST /auth/logout` — butuh access token (security default berlaku).
     → `204`, plus `Set-Cookie` yang menghapus cookie refresh
     (`Max-Age=-1`).
   - `GET /auth/me` — butuh access token. → `200` body `User`.

7. **Endpoint list.**

   - `GET /lists` — paginated, → `200` `{ items: List[], nextCursor }`.
   - `POST /lists` — body `{ name }` → `201` body `List`, header `Location`.
   - `GET /lists/{listId}` — → `200` `List`, `404` kalau bukan milik user.
   - `PATCH /lists/{listId}` — body parsial `{ name? }` → `200` `List`.
     **PATCH, bukan PUT**, karena update selalu parsial (rename saja) dan
     klien tidak pernah mengirim representasi penuh resource — PUT
     menyiratkan replace-seluruhnya yang tidak sesuai bentuk operasi ini.
   - `DELETE /lists/{listId}` — → `204`. Cascade menghapus todo di
     dalamnya (lihat skema `lists`/`todos`, ON DELETE CASCADE — dijelaskan
     di description operasi, bukan hanya di komentar SQL).

8. **Endpoint todo di bawah list, dan todo langsung.**

   - `GET /lists/{listId}/todos` — parameter `Limit`, `Cursor`, `status`
     (tanpa `listId` karena sudah di path), `dueBefore`, `dueAfter`, `sort`.
     → `200` `PagedTodos`.
   - `POST /lists/{listId}/todos` — body `{ title, notes?, priority?, dueAt? }`
     → `201` `Todo`.
   - `GET /todos/{todoId}` → `200` `Todo`, `404`.
   - `PATCH /todos/{todoId}` — body parsial `{ title?, notes?, priority?, dueAt? }`
     → `200` `Todo`. Alasan PATCH sama seperti list.
   - `DELETE /todos/{todoId}` → `204`.
   - `POST /todos/{todoId}/complete` → `200` `Todo` dengan `status: done`,
     `completedAt` terisi. Endpoint terpisah (bukan `PATCH status`) karena
     ini transisi state dengan efek samping (`completedAt`), bukan sekadar
     ubah field — memodelkannya sebagai aksi lebih jelas daripada
     menyembunyikannya di semantik PATCH.
   - `POST /todos/{todoId}/reopen` → `200` `Todo` dengan `status: todo`,
     `completedAt: null`.
   - `POST /todos/{todoId}/move` — body `{ listId }` → `200` `Todo` dengan
     `listId` baru. Endpoint aksi terpisah karena pindah list mengubah
     `todos.user_id` yang didenormalisasi (ADR-011) — dijelaskan di
     description operasi supaya konsisten dengan alasan di ADR.

   Ada satu endpoint global tanpa `GET /todos` di seluruh scope milik user
   tanpa filter list — untuk MVP, listing lintas-list ada di `GET /lists/{listId}/todos`
   per-list. **Tidak menambah endpoint di luar 12 yang disebut task ini**
   kecuali disepakati ulang lewat prosedur version bump (langkah 10).

9. **Schema entity.** `User`, `List`, `Todo` — field camelCase (JSON),
   cocokkan dengan kolom tabel di `ARCHITECTURE.md` tapi tanpa
   `password_hash`/`refresh_token_hash`:

   ```yaml
   components:
     schemas:
       User:
         type: object
         required: [id, email, name, createdAt, updatedAt]
         properties:
           id: { type: string, format: uuid }
           email: { type: string, format: email }
           name: { type: string }
           createdAt: { type: string, format: date-time }
           updatedAt: { type: string, format: date-time }
       List:
         type: object
         required: [id, name, createdAt, updatedAt]
         properties:
           id: { type: string, format: uuid }
           name: { type: string }
           position: { type: integer }
           createdAt: { type: string, format: date-time }
           updatedAt: { type: string, format: date-time }
       Todo:
         type: object
         required: [id, listId, title, status, createdAt, updatedAt]
         properties:
           id: { type: string, format: uuid }
           listId: { type: string, format: uuid }
           title: { type: string }
           notes: { type: string, nullable: true }
           status: { type: string, enum: [todo, done] }
           priority: { type: string, enum: [low, medium, high], nullable: true }
           dueAt: { type: string, format: date-time, nullable: true }
           completedAt: { type: string, format: date-time, nullable: true }
           createdAt: { type: string, format: date-time }
           updatedAt: { type: string, format: date-time }
   ```

10. **Tulis prosedur version bump** di bagian atas `openapi.yaml` sebagai
    komentar YAML, konkret:

    ```yaml
    # Prosedur perubahan kontrak (wajib diikuti, bukan opsional):
    # 1. Field/endpoint baru yang backward-compatible (properti opsional baru,
    #    endpoint baru): naikkan versi minor di `info.version` (0.1.0 -> 0.2.0).
    # 2. Perubahan breaking (hapus/rename field, ubah tipe, ubah status code,
    #    ubah wajib/opsional): naikkan versi major (0.x.0 -> 1.0.0) DAN buka
    #    isu/pesan ke pemilik apps/cms sebelum merge - jangan diam-diam.
    # 3. Tiap perubahan wajib: edit source (openapi.yaml atau paths/*.yaml),
    #    lalu `make openapi-bundle`, commit openapi.yaml DAN bundle.yaml
    #    dalam commit yang sama.
    # 4. CI menjalankan `make openapi-lint` dan `make openapi-bundle-check`.
    #    Keduanya harus hijau sebelum merge.
    # 5. apps/cms membaca docs/openapi/bundle.yaml, bukan file yang terpecah -
    #    lupa bundle ulang berarti CMS bekerja dengan kontrak basi tanpa error
    #    yang terlihat sampai runtime.
    ```

11. Jalankan `make openapi-lint` dan `make openapi-bundle` secara manual
    (lewat `npx`, tidak butuh Go) untuk memverifikasi spec valid sebelum
    menandai task selesai.

## Kriteria selesai

```bash
cd apps/go-chi-api
make openapi-lint        # exit 0, tanpa error (warning boleh, tapi dicatat kalau ada)
make openapi-bundle       # menghasilkan docs/openapi/bundle.yaml
make openapi-bundle-check # exit 0 setelah bundle di-commit
```

- `docs/openapi/openapi.yaml` (dan/atau `paths/*.yaml`, `components/*.yaml`)
  serta `docs/openapi/bundle.yaml` ada dan valid OpenAPI 3.1.
- Ke-12 endpoint di langkah 6-8 ada, masing-masing dengan status code
  eksplisit untuk sukses dan minimal satu error (`401`/`404`/`409`/`422`
  sesuai konteks) lewat `$ref` ke `components.responses`.
- Schema `Problem` didefinisikan sekali dan dipakai lewat `$ref` di semua
  response error, dengan satu contoh error validasi lengkap di YAML
  (langkah 4).
- `securitySchemes.bearerAuth` ada; tiga endpoint auth publik override
  `security: []`.
- Parameter `limit`, `cursor`, `status`, `listId`, `dueBefore`, `dueAfter`,
  `sort` didefinisikan sekali di `components.parameters` dan direferensikan,
  bukan diulang tiap operasi.
- Prosedur version bump tertulis konkret (bukan "hubungi tim" tanpa
  langkah).

## Jebakan

- Jangan generate spec dari kode (belum ada kode). Ini contract-first
  (ADR-012) — spec ditulis dulu, kode menyesuaikan nanti di task 05.
- Jangan lupa `bundle.yaml` di-commit. Kalau hanya source `$ref` yang
  di-commit tanpa bundle, CMS tidak bisa membaca kontrak sama sekali.
- Jangan menambah endpoint di luar `docs/PRD.md` (contoh: jangan menaruh
  share-list atau sub-todo walau "gampang tambahnya sekarang") — itu
  melanggar non-goal PRD, bukan keputusan task ini untuk diambil sendiri.
- `PATCH` vs `PUT`: jangan taruh `PUT` untuk update parsial — itu salah
  semantik HTTP, dan sudah dijelaskan alasannya di langkah 7/8, jangan
  diulang berbeda di kode task 05 nanti.
- Jangan menaruh detail internal (nama kolom SQL, pesan error mentah)
  di `example` schema `Problem` — pesan harus seperti yang benar-benar
  aman ditunjukkan ke klien.
- Kalau memilih tooling lint selain Redocly, pastikan tetap **satu**
  tool dan versinya di-pin persis (bukan `@latest`) — versi mengambang
  membuat CI lulus hari ini dan gagal besok tanpa perubahan kode apa pun.
