# Weather Fusion

A weather intelligence platform that collects forecasts from multiple free APIs, measures agreement between them, and uses an LLM to explain the results in plain language.

Different weather services often disagree. One might say rain, another says sunny. Instead of picking one and trusting it blindly, Weather Fusion compares all of them, calculates a confidence score, and lets an LLM summarize what's actually likely to happen.

```
weather-fusion/
├── backend/    Go · Gin · PostgreSQL · Redis
└── frontend/   Next.js · TypeScript
```

---

## How It Works

1. Fetch forecasts from five providers at once (OpenWeather, WeatherAPI, Open-Meteo, Visual Crossing, Tomorrow.io)
2. Normalize all responses into a common schema
3. Run a consensus engine — calculate averages, standard deviation, and provider agreement
4. Produce a confidence score based on how much providers agree
5. Stream provider results and the consensus to the frontend as they're ready, then pass the consensus to an LLM for a plain-language summary

The LLM acts as an interpreter, not a predictor. It explains the data — it does not decide what the weather will be.

City search (the autocomplete dropdown) is a separate lookup, not tied to any weather provider — it goes through its own geocoding service (Photon, OpenStreetMap-based) so every provider gets fetched with the exact same coordinates.

---

## Architecture

```
Weather APIs (fetched concurrently)
    ├── OpenWeather
    ├── WeatherAPI
    ├── Open-Meteo
    ├── Visual Crossing
    └── Tomorrow.io

            ↓

Data Normalization Layer

            ↓

Consensus & Confidence Engine
            │
            ├──→ streamed to frontend immediately (provider results + consensus)
            │
            └──→ LLM summary (generated after, streamed separately)

Geocoding (Photon) is a parallel, independent path — resolves a typed city
name into coordinates for the autocomplete dropdown, no weather provider involved.
```

---

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker & Docker Compose

### Run infrastructure (PostgreSQL + Redis)

```bash
make infra-up
```

### Run backend

```bash
cd backend
cp .env.example .env
# fill in your API keys
go run main.go
```

### Run frontend

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

---

## Make Targets

| Target              | Description                     |
|---------------------|---------------------------------|
| `make infra-up`     | Start PostgreSQL + Redis        |
| `make infra-down`   | Stop infrastructure             |
| `make dev-backend`  | Run backend with hot-reload     |
| `make dev-frontend` | Run Next.js dev server          |
| `make build`        | Build backend binary            |
| `make test`         | Run backend tests               |
| `make lint`         | Run golangci-lint               |

---

## Sub-project READMEs

- [backend/README.md](backend/README.md)
- [frontend/README.md](frontend/README.md)
