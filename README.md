# 🌦️ Weather Intelligence Platform

A production-ready monorepo for a multi-provider weather intelligence platform with LLM-powered insights.

```
weather-fusion/
├── backend/   # Go 1.25 · Gin · PostgreSQL · Redis · Clean Architecture
└── frontend/  # Next.js 14 · TypeScript · Tailwind CSS · App Router
```

---

## Quick Start

### Prerequisites
- Go 1.25+
- Node.js 20+
- Docker & Docker Compose

### 1. Clone & configure

```bash
git clone https://github.com/yourorg/weather-fusion.git
cd weather-fusion

# Back-end env
cp backend/.env.example backend/.env
# Front-end env
cp frontend/.env.local.example frontend/.env.local
```

### 2. Start infrastructure (DB + Redis)

```bash
make infra-up
```

### 3. Run database migrations

```bash
make migrate-up
```

### 4. Start backend (hot-reload)

```bash
make dev-backend
```

### 5. Start frontend (dev server)

```bash
make dev-frontend
```

Or run everything at once with Docker Compose:

```bash
docker compose up --build
```

---

## Architecture

### Backend — Clean Architecture

```
cmd/api/          → Entry point, DI wiring, graceful shutdown
internal/
  config/         → Typed env-based configuration
  handlers/       → HTTP layer (Gin handlers + router)
  middleware/     → Logger, Recovery, CORS, Auth, Validator
  services/       → Business logic interfaces + implementations
  repositories/   → Data access interfaces + PostgreSQL implementations
  providers/      → WeatherProvider interface (OpenWeather, WeatherAPI, …)
  llm/            → LLMService interface (OpenAI, …)
  consensus/      → Multi-provider result merging engine
  scheduler/      → Cron-based background jobs
  cache/          → Cache interface + Redis implementation
  models/         → Domain models & error types
pkg/
  logger/         → slog-based structured logging
  response/       → Unified JSON response helpers
  apperror/       → Centralized error types & HTTP mapping
migrations/       → SQL migration files
```

### Frontend — Next.js App Router

```
src/
  app/             → Pages (layout, dashboard)
  components/
    dashboard/     → Dashboard widgets (Weather, Forecast, AI Insights, …)
    ui/            → Reusable primitives (Card, Badge, Spinner)
  lib/api/         → Axios client + typed API calls
  types/           → TypeScript domain types
```

---

## API

| Method | Path                         | Description           |
|--------|------------------------------|-----------------------|
| GET    | /health                      | Health check          |
| GET    | /api/v1/weather/current      | Current conditions    |
| GET    | /api/v1/weather/forecast     | 7-day forecast        |
| POST   | /api/v1/users                | Create user           |
| GET    | /api/v1/users/:id            | Get user by ID        |

---

## Make Targets

| Target              | Description                        |
|---------------------|------------------------------------|
| `make dev-backend`  | Run backend with Air hot-reload    |
| `make dev-frontend` | Run Next.js dev server             |
| `make build`        | Build backend binary               |
| `make test`         | Run backend unit tests             |
| `make lint`         | Run golangci-lint                  |
| `make migrate-up`   | Apply SQL migrations               |
| `make migrate-down` | Roll back last migration           |
| `make infra-up`     | Start PostgreSQL + Redis           |
| `make infra-down`   | Stop infrastructure containers     |
| `make docker-build` | Build all Docker images            |

---

## Environment Variables

See [`backend/.env.example`](backend/.env.example) and [`frontend/.env.local.example`](frontend/.env.local.example).
