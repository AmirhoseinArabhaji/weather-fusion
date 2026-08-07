# Backend — Weather Fusion

Go backend for the Weather Fusion platform. Responsible for fetching weather data from multiple providers, running the consensus engine, generating LLM summaries, and serving the REST API.

---

## Stack

- **Go** — main language
- **Gin** — HTTP router
- **PostgreSQL** — persistent storage
- **Redis** — caching layer
- **pgx** — PostgreSQL driver
- **go-redis** — Redis client
- **Air** — hot-reload for development

---

## Project Structure

```
backend/
├── cmd/api/            Entry point — wires dependencies, starts server
├── internal/
│   ├── config/         Typed config loaded from environment variables
│   ├── handlers/       HTTP handlers and router (Gin)
│   ├── middleware/     Request logger, recovery, CORS, validator
│   ├── services/       Business logic (weather aggregation, LLM calls)
│   ├── repositories/   Data access interfaces + PostgreSQL implementations
│   ├── providers/      WeatherProvider interface + per-provider adapters
│   ├── llm/            LLMService interface + OpenAI client
│   ├── consensus/      Multi-provider result merging and confidence scoring
│   ├── cache/          Cache interface + Redis implementation
│   └── models/         Domain types
└── pkg/
    ├── logger/         slog-based structured logger
    ├── response/       JSON response helpers
    └── apperror/       Error types and HTTP status mapping
```

---

## Getting Started

```bash
cp .env.example .env
# Edit .env and fill in your credentials and API keys
```

Start the database and Redis first (from the repo root):

```bash
make infra-up
```

Run the server:

```bash
go run main.go
# or with hot-reload:
air
```

---

## Environment Variables

See [`.env.example`](.env.example) for all available variables. Key ones:

| Variable          | Description                        |
|-------------------|------------------------------------|
| `DB_HOST`         | PostgreSQL host                    |
| `DB_PORT`         | PostgreSQL port (default 5432)     |
| `DB_USER`         | PostgreSQL user                    |
| `DB_PASSWORD`     | PostgreSQL password                |
| `DB_NAME`         | PostgreSQL database name           |
| `REDIS_HOST`      | Redis host                         |
| `REDIS_PASSWORD`  | Redis password                     |
| `OPENAI_API_KEY`  | OpenAI API key for LLM summaries   |
| `APP_ENV`         | `development` or `production`      |
| `PORT`            | HTTP server port (default 8080)    |

---

## API Endpoints

| Method | Path      | Description  |
|--------|-----------|--------------|
| GET    | /health   | Health check |

More endpoints will be added as features are implemented.

---

## Make Targets

Run from the repo root or this directory:

| Target             | Description                  |
|--------------------|------------------------------|
| `make dev-backend` | Run with Air (hot-reload)    |
| `make build`       | Compile binary               |
| `make test`        | Run tests                    |
| `make lint`        | Run golangci-lint            |
