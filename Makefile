.PHONY: dev-backend dev-frontend build test lint migrate-up migrate-down \
        infra-up infra-down docker-build help

dev-backend:
	$(MAKE) -C backend dev

dev-frontend:
	$(MAKE) -C frontend dev

build:
	$(MAKE) -C backend build

test:
	$(MAKE) -C backend test

lint:
	$(MAKE) -C backend lint

migrate-up:
	$(MAKE) -C backend migrate-up

migrate-down:
	$(MAKE) -C backend migrate-down

infra-up:
	docker compose up -d postgres redis

infra-down:
	docker compose stop postgres redis

docker-build:
	docker compose build

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
