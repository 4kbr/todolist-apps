# Makefile app go-chi-api.
# untuk migrasi, menjalankan server, dan lain-lain.

# Muat .env (kalau ada) dan export semua variabelnya ke tiap target di bawah.
# config.Load() baca os.Getenv langsung tanpa library dotenv (lihat
# internal/platform/config/config.go) — jadi variabelnya harus sampai ke
# environment proses dari sini, bukan dari Go.
-include .env
export

## Menjalankan server & go test ----------------------------------------------------
.PHONY: dev
dev:	 					## live reload server
	@echo "Menjalankan server go-chi-api dengan air live reload di port: $(HTTP_PORT) (tekan Ctrl+C untuk berhenti)"
	air

.PHONY: run
run:						## menjalankan server tanpa live reload
	@echo "Menjalankan server go-chi-api (tekan Ctrl+C untuk berhenti)"
	go run ./cmd/api

.PHONY: build
build:						## membangun binary server
	@echo "Membangun binary server go-chi-api"
	go build -o bin/api ./cmd/api

.PHONY: test
test:						## menjalankan unit test
	@echo "Menjalankan unit test go-chi-api"
	go test -v ./... -short

.PHONY: test-integration
test-integration:		## menjalankan integration test
	@echo "Menjalankan integration test go-chi-api"
	go test -v ./... -tags=integration

.PHONY: lint
lint:						## menjalankan linter
	@echo "Menjalankan linter go-chi-api"
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## fix linter errors
	@echo "Menjalankan linter fix go-chi-api"
	golangci-lint run --fix ./...


# Migrasi database ----------------------------------------------------

.PHONY: migrate-up migrate-down migrate-reset migrate-status migrate-create sqlc

migrate-up:				## menjalankan migrasi ke versi terbaru
	@echo "Menjalankan migrasi ke versi terbaru"
	goose -dir db/migrations postgres "$${DATABASE_URL}" up

migrate-down:				## membatalkan migrasi terakhir
	@echo "Membatalkan migrasi terakhir"
	goose -dir db/migrations postgres "$${DATABASE_URL}" down

migrate-reset:			## membatalkan semua migrasi
	@echo "Membatalkan semua migrasi"
	goose -dir db/migrations postgres "$${DATABASE_URL}" reset

migrate-status:			## menampilkan status migrasi
	@echo "Menampilkan status migrasi"
	goose -dir db/migrations postgres "$${DATABASE_URL}" status

migrate-create:			## membuat file migrasi baru
	@echo "Membuat file migrasi baru"
	goose -dir db/migrations postgres "$${DATABASE_URL}" create $(name) sql

sqlc:
	@echo "Menjalankan sqlc generate"
	sqlc generate


# OpenAPI -----------------------------------------------------------------

.PHONY: openapi-lint openapi-bundle openapi-bundle-check docs-serve

openapi-lint:
	redocly lint docs/openapi/openapi.yaml

openapi-bundle:
	redocly bundle docs/openapi/openapi.yaml -o docs/openapi/bundled.yaml

openapi-bundle-check:   ## gagal kalau bundle belum di-regenerate setelah edit spec
	redocly bundle docs/openapi/openapi.yaml -o /tmp/openapi-bundled-check.yaml
	diff docs/openapi/bundled.yaml /tmp/openapi-bundled-check.yaml

docs-serve:             ## pratinjau OpenAPI lokal
	redocly preview-docs docs/openapi/openapi.yaml

# developemnt tools  -----------------------------------------------------------------

.PHONY: tools

GOBIN := $(shell go env GOPATH)/bin

tools: ## pasang tool CLI yang belum ada
	@echo "🔍 Checking installed tools..."
	@command -v air >/dev/null 2>&1 || (echo "📦 Installing air..." && go install github.com/air-verse/air@v1.67.3)
	@command -v goose >/dev/null 2>&1 || (echo "📦 Installing goose..." && go install github.com/pressly/goose/v3/cmd/goose@v3.28.0)
	@command -v sqlc >/dev/null 2>&1 || (echo "📦 Installing sqlc..." && go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1)
	@command -v golangci-lint >/dev/null 2>&1 || (echo "📦 Installing golangci-lint..." && curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(GOBIN) v2.13.2)
	@echo "✅ All tools ready!"
	@echo ""
	@echo "Installed tools:"
	@command -v air >/dev/null 2>&1 && echo "  ✓ air: $$(air --version 2>/dev/null || echo 'installed')"
	@command -v goose >/dev/null 2>&1 && echo "  ✓ goose: $$(goose --version 2>/dev/null | head -1)"
	@command -v sqlc >/dev/null 2>&1 && echo "  ✓ sqlc: $$(sqlc version 2>/dev/null)"
	@command -v golangci-lint >/dev/null 2>&1 && echo "  ✓ golangci-lint: $$(golangci-lint version 2>/dev/null | head -1)"
