SHELL := /bin/sh

export GOFLAGS=-mod=mod

run-api:
	cd api && go run ./cmd/server

migrate-up:
	sh api/migrate.sh up

migrate-down:
	sh api/migrate.sh down

.PHONY: run-api migrate-up migrate-down
