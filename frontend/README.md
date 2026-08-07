# Frontend — Weather Fusion

Next.js frontend for the Weather Fusion platform. Displays aggregated weather data, consensus results, confidence scores, and LLM-generated summaries.

---

## Stack

- **Next.js** — React framework (App Router)
- **TypeScript**
- **Tailwind CSS**

---

## Project Structure

```
frontend/
├── src/
│   ├── app/          Pages and layouts (App Router)
│   └── components/   UI components
├── public/           Static assets
└── next.config.ts    Next.js config
```

---

## Getting Started

```bash
cp .env.local.example .env.local
# Edit .env.local if needed

npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

---

## Environment Variables

See [`.env.local.example`](.env.local.example).

| Variable              | Description                        |
|-----------------------|------------------------------------|
| `NEXT_PUBLIC_API_URL` | Backend API base URL               |

---

## Make Targets

| Target              | Description            |
|---------------------|------------------------|
| `make dev`          | Start dev server       |
| `make build`        | Production build       |
| `make lint`         | Run ESLint             |
| `make format`       | Run Prettier           |
