# Repository Guidelines

NestCash is a single-user personal budget tracker in one repository: a Go 1.22.2 JSON API at the root and a Vue 3 + TypeScript + Vite SPA in `web/`. Backend deployment is configured for Fly.io; the frontend stack and deployment hand-off are documented in `@context/foundation/tech-stack-web.md`.

## Hard rules

- Read `@context/foundation/prd.md` before implementing features. Preserve the API-first boundary, single-user scope, privacy requirements, and explicit MVP non-goals.
- Keep frontend code under `web/`; do not scaffold it into the repository root. The web client consumes the Go API and reads its base URL from `VITE_API_BASE_URL`.
- Preserve account isolation: owner identity must come from the trusted resolver/context contract in `internal/account`; never accept an account ID solely from request data.
- Keep secrets in environment variables or GitHub/Fly secrets. Never commit `.env` values, tokens, or credentials; both `.gitignore` files exclude local secret files.

## Project structure

- `cmd/nest-cash/` — Go entrypoint, HTTP routes, and database setup.
- `internal/account/` — provider-neutral account identity and authorization middleware, with co-located tests.
- `web/src/` — Vue components, application entrypoint, and styles; `web/public/` contains static assets.
- `context/foundation/` — requirements, stack, roadmap, and infrastructure decisions; `context/changes/` contains plans and verification logs.
- `Dockerfile`, `fly.toml`, and `.github/workflows/fly.yml` — backend container and Fly.io deployment configuration.

## Build, test, and development commands

- `go test ./...` — run all backend tests.
- `go run ./cmd/nest-cash` — run the local API; it uses `PORT` and optionally `DATABASE_URL`.
- `go build ./...` — compile-check the backend.
- `npm --prefix web install` — install frontend dependencies from `web/package-lock.json`.
- `npm --prefix web run dev` — start Vite development; `npm --prefix web run build` — type-check and build; `npm --prefix web run preview` — serve the production build locally.
- `flyctl deploy` — manually deploy the backend; pushes to `main` trigger `.github/workflows/fly.yml`, which runs `flyctl deploy --remote-only`.

## Coding and testing conventions

Run `gofmt` on changed Go files. Keep backend tests in `*_test.go` files beside the package under test. Vue components use `<script setup lang="ts">`; the TypeScript configs enforce unused-code checks. No frontend test runner or lint script is configured yet, so `npm --prefix web run build` is the current frontend validation gate.

Recent commits use `feat(...)`/`chore(...)`; use a descriptive scope. Follow `@context/opencode.json` for command permissions, keep foundation docs and active plans authoritative, and never modify `context/archive/`.
