# Task

Urutan pengerjaan backend. Satu file satu task, dikerjakan berurutan — task N
hanya bergantung pada task sebelum N, tidak pernah sebaliknya.

Tiap file memuat tujuan, file yang disentuh, langkah, dan **kriteria selesai
berupa perintah yang harus hijau**. Jangan mengklaim sebuah task selesai tanpa
menjalankan perintah itu.

Kerjakan satu task per sesi. Jangan menyerempet task berikutnya walaupun terlihat
sepele — pemecahannya disengaja.

## Phase 1 — Fondasi

Aplikasi yang bisa dijalankan, database yang bisa dimigrasi, dan pondasi HTTP
yang dipakai semua modul. Belum ada satu pun aturan bisnis.

| Task | Isi |
| --- | --- |
| [`00-foundation.md`](00-foundation.md) | Makefile, config, logger, clock, pool pgx, chi, `/healthz`, graceful shutdown, air, golangci-lint |
| [`01-database-migrations.md`](01-database-migrations.md) | goose, empat migrasi, `sqlc.yaml` per modul |
| [`02-platform-http.md`](02-platform-http.md) | `problem` (RFC 9457), `httpx`, middleware, keyset pagination, tx manager |

## Phase 2 — Kontrak

Kontrak API ditulis sebelum handler mana pun dibuat. Lihat ADR-012.

| Task | Isi |
| --- | --- |
| [`03-openapi-contract.md`](03-openapi-contract.md) | `docs/openapi/openapi.yaml` lengkap, `make openapi-lint` |

## Phase 3 — Identity

Modul user dan autentikasi.

| Task | Isi |
| --- | --- |
| [`04-identity-domain.md`](04-identity-domain.md) | Entity, port, argon2id, JWT, usecase register/login/refresh/logout/me, unit test |
| [`05-identity-adapter.md`](05-identity-adapter.md) | Repo sqlc, handler, middleware auth, `module.go`, integration test |

## Phase 4 — Task

Modul list dan todo. List adalah agregat root (ADR-004).

| Task | Isi |
| --- | --- |
| [`06-task-domain.md`](06-task-domain.md) | Entity, aturan domain, usecase, unit test |
| [`07-task-adapter.md`](07-task-adapter.md) | Repo sqlc, handler CRUD, filter/sort/pagination, integration test |

## Phase 5 — Rilis

| Task | Isi |
| --- | --- |
| [`08-hardening.md`](08-hardening.md) | Rate limit, timeout, security header, audit log, `/readyz` |
| [`09-ci-docker.md`](09-ci-docker.md) | GitHub Actions, Dockerfile multistage, `docker-compose.yml` demo |

Setelah task 09, backend dianggap selesai dan `apps/cms` boleh mulai dikerjakan
berbekal `docs/openapi/`.

## Cara menandai selesai

Task tidak punya kolom status di file ini supaya tidak ada dua sumber kebenaran.
Status ada di kepala tiap file task dan di `../HANDS-OFF.md`.
