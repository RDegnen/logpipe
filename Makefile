.PHONY: up down reset build run-gateway topics

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