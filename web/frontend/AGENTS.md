# Trezutil Frontend (Homepage)

React 19 + TypeScript + Vite 8 + Tailwind CSS v4 SPA that implements the
Trezutil homepage. The source of truth for the visual design is the Figma Make
export at `D:\dev\Projects\trezutil_foundation v1\HomepageDesign`.

## Development

```bash
cd web/frontend
pnpm install
pnpm run dev       # http://localhost:5173
pnpm run build     # production build → web/frontend/dist/
pnpm run typecheck # TypeScript type checking
pnpm run preview   # preview production build locally
```

## Project Structure

```
src/
├── App.tsx              # All visual components (reproduce Figma Make layout)
├── index.css            # Global CSS + Tailwind v4 theme + custom styles
├── main.tsx             # React entrypoint
├── vite-env.d.ts        # Vite TypeScript reference
├── types/
│   └── index.ts         # TypeScript types mirroring OpenAPI contracts
├── api/
│   └── client.ts        # Typed API client for /api/v1/* backend
└── data/
    └── mockData.ts      # TEMPORARY mock data (replace with real API calls)
```

## Architecture

- The SPA is served by the Go api-gateway from `WebRoot` (default `./web/frontend/dist`).
- API calls go to `/api/v1/*` (proxied by the gateway to microservices).
- Dev proxy forwards `/api`, `/auth`, `/health` to the gateway on port 8080.
- See `cmd/api-gateway` and `internal/gateway/` for backend serving details.

## Styling

- Tailwind CSS v4 via `@tailwindcss/vite`.
- Global CSS variables and custom styles live in `src/index.css`.
- No Tailwind config file needed (v4 is configured via the Vite plugin).
- Dark mode: toggled via the Navbar moon/sun button, persisted to localStorage.
- Responsive breakpoints: 1200px, 900px, 640px (in index.css @media rules).
