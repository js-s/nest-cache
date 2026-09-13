# Repository Guidelines

NestCash is a single-user personal budget tracker in one repository: a Go 1.22.2 JSON API at the root serving the built Vue 3 + TypeScript + Vite SPA from `web/dist`. Deployment targets Fly.io; see `@context/foundation/tech-stack-web.md` and `@context/deployment/deploy-plan.md`.

## Hard rules

- Read `@context/foundation/prd.md` before implementing features. Preserve API-first boundary, single-user scope, privacy, and MVP non-goals.
- Keep frontend code under `web/`. Auth is a same-origin cookie session (`nest_session`); API base defaults to `/api`, proxied to `:8080` by `@web/vite.config.ts` in dev.
- Preserve account isolation: owner identity must come from the trusted resolver/context contract in `internal/account`; never accept an account ID from request data alone.
- Keep secrets in env vars or GitHub/Fly secrets. Never commit `.env` values, tokens, or credentials; both `.gitignore` files exclude local secret files. `DATABASE_URL` requires `SESSION_SECRET` (≥32 chars) or boot fails closed.

## Project structure

- `cmd/nest-cash/` — Go entrypoint, route table (`spa.go` serves the built SPA), and DB setup.
- `internal/account/` — account identity and `RequireAccount` authorization middleware; `internal/auth/` owns users, sessions, and the resolver.
- `internal/categories/`, `internal/transactions/` — feature handlers and stores, with co-located tests; `internal/db/migrations/` — SQL migrations.
- `web/src/` — Vue app: `views/` (routed screens), `api/client.ts` (typed client), `router.ts`, `auth.ts`; `web/public/` — static assets.
- `context/foundation/` — requirements, roadmap, and decisions; `context/deployment/` and `context/changes/` hold deploy plans and active change docs.
- `Dockerfile`, `fly.toml`, `.github/workflows/fly.yml`, and `fly-rollback.yml` — container and Fly.io deploy/rollback configuration.

## Build, test, and development commands

- `go test ./...` — run all backend tests. If binaries abort on macOS with `dyld: missing LC_UUID`, retry `CGO_ENABLED=0 go test ./...` (local toolchain quirk).
- `go run ./cmd/nest-cash` — run the local API; reads `PORT`, `DATABASE_URL`, `SESSION_SECRET`, and `STATIC_DIR` (default `web/dist`). Without `DATABASE_URL` it serves `/healthz` only.
- `go build ./...` — compile-check backend.
- `npm --prefix web install` — install frontend deps (`web/package-lock.json`).
- `npm --prefix web run dev` — start Vite development; `npm --prefix web run build` — type-check and production build.
- `flyctl deploy` — deploy the backend; pushes to `main` trigger `.github/workflows/fly.yml`; `fly-rollback.yml` handles rollback.

## Coding and testing conventions

Run `gofmt` on changed Go files. Keep backend tests in `*_test.go` files beside the package; DB-backed tests auto-skip unless `DATABASE_URL` is set. Vue components use `<script setup lang="ts">`; TS configs enforce unused-code checks. No frontend test/lint script; `npm --prefix web run build` is the validation gate.

Recent commits use `feat(...)`, `chore(...)`, `fix(...)`, and `docs(...)` with a descriptive scope. Follow `@context/opencode.json` for command permissions, keep foundation docs and active plans authoritative, and never modify `context/archive/`.
