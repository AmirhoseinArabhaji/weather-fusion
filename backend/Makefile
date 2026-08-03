BINARY_NAME   := weather-fusion
BUILD_DIR     := ./tmp
CMD_DIR       := ./cmd/api
MIGRATE_DIR   := ./migrations
DB_URL        ?= postgres://weather:weather_secret@localhost:5432/weather_fusion?sslmode=disable

.PHONY: dev build run test lint migrate-up migrate-down migrate-create tidy clean help

dev:
	@which air > /dev/null 2>&1 || go install github.com/air-verse/air@latest
	air

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

test:
	go test -v -race -count=1 ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	@which golangci-lint > /dev/null 2>&1 || \
	  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	golangci-lint run ./...

tidy:
	go mod tidy
	go mod verify

clean:
	rm -rf $(BUILD_DIR)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
