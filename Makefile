DB_URL ?= postgres://app:app@localhost:5432/tgtg?sslmode=disable
MIGRATE = migrate -path migrations -database "$(DB_URL)"

.PHONY: migrate-up migrate-down migrate-reset migrate-version migrate-force migrate-new migrate-test migrate-test-docker

migrate-up:      ; $(MIGRATE) up
migrate-down:    ; $(MIGRATE) down 1          # roll back the last migration only
migrate-reset:   ; $(MIGRATE) down -all       # roll back everything (dev only!)
migrate-version: ; $(MIGRATE) version
migrate-force:   ; $(MIGRATE) force $(v)      # after fixing a "dirty" state: make migrate-force v=3
migrate-new:     ; migrate create -ext sql -dir migrations -seq $(name)
migrate-test:    ; bash scripts/test_migrations_local.sh   # no Docker needed
migrate-test-docker: ; bash scripts/test_migrations.sh
