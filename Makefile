.PHONY: up down reset build run-gateway topics migrate full-start

up:
	docker compose -f infra/docker-compose.yml up -d

down:
	docker compose -f infra/docker-compose.yml down

reset:
	docker compose -f infra/docker-compose.yml down -v

build:
	go build ./...

topics:
	./scripts/create-topics.sh

run-gateway:
	go run ./cmd/ingest-gateway

run-processor:
	go run ./cmd/processor

run-sink:
	go run ./cmd/sink-writer

migrate:
	go run ./cmd/migrator

# This only seems to work some of the time if migrate runs to quickly before up seems to be done
full-start: up migrate topics