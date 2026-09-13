# Makefile root. Target aplikasi diteruskan ke apps/go-chi-api supaya tidak perlu
# cd ke sana; `cd apps/go-chi-api && make dev` tetap bekerja sama persis.
#
# Target container berprefix dockerdev- supaya tidak tertukar dengan db-*.
# Keduanya mengurus hal berbeda: dockerdev-up menyalakan container,
# db-up menjalankan migrasi di dalamnya.

.PHONY: dockerdev-up dockerdev-down dockerdev-logs dockerdev-ps dockerdev-psql

BACKEND := apps/go-chi-api
COMPOSE := docker compose --env-file .env.docker -f docker-compose.dev.yml

## Container pengembangan ----------------------------------------------------

dockerdev-up:
	$(COMPOSE) up -d

dockerdev-down:
	$(COMPOSE) down

dockerdev-logs:
	$(COMPOSE) logs -f

dockerdev-ps:
	$(COMPOSE) ps

dockerdev-psql:
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-postgres} -d $${POSTGRES_DB:-todolist_dev}

## Diteruskan ke backend -----------------------------------------------------
.PHONY: dev run build test test-integration lint \
        migrate-up migrate-down migrate-reset migrate-status migrate-create 
dev dev-worker run run-worker build test test-integration lint \
db-up db-down db-reset db-status db-create tools docs-serve:
	$(MAKE) -C $(BACKEND) $@ $(if $(name),name=$(name))

## Bootstrap -----------------------------------------------------------------

.PHONY: setup
setup:
	@test -f .env.docker || cp .env.docker.example .env.docker
	@test -f $(BACKEND)/.env || cp $(BACKEND)/.env.example $(BACKEND)/.env
	@echo "Menunggu postgres siap..."
	$(MAKE) dockerdev-up
	@until $(COMPOSE) exec -T postgres pg_isready -q; do sleep 1; done
	@# Sampai task 01 mengisi migrations/, goose wajar melaporkan tidak ada file.
	@$(MAKE) db-up || echo "  (belum ada migrasi - itu task 01)"
	@echo
	@echo "Siap. Jalankan: make dev"
