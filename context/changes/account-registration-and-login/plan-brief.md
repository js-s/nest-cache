# Account registration and login — Plan Brief

> Full plan: `context/changes/account-registration-and-login/plan.md`

## What & Why

Register + login with email/password and 30-day sliding cookie session, wired into existing account-isolation contract. Unblocks all finance slices; reset explicitly deferred to v2 to ship S-01 now.

## Starting Point

Server + SPA shell exist, `/healthz` + `/api` boundary live, `internal/account` gate ready with no Resolver, zero auth tables/routes/UI. Secrets reserved in Fly only.

## Desired End State

User registers/logs in via web UI, session survives reload/restart, logout revokes, second account isolated. Verified by Go tests + web build + manual two-account check.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
|---|---|---|---|
| Password reset | Out of S-01 | Roadmap blocker resolved by deferral; smallest secure scope | Plan |
| Session | 30d opaque DB token, sliding | Revocable, stdlib-only, matches long-session PRD | Plan |
| Migrations | SQL files + Go runner | Zero new deps, reviewable, Fly/Neon-safe | Plan |
| API shape | REST cookie endpoints | No JS token handling, same-origin simple | Plan |
| Registration | Open, unique email | Matches FR-001, no admin bootstrap | Plan |
| Frontend | vue-router + fetch client | Conventional guards, deep links | Plan |
| Hashing | Email normalize + bcrypt 12 | Standard baseline, cheap | Plan |
| Testing | Unit + httptest + PG gate | Proves isolation without new harness | Plan |

## Scope

**In scope:** users/sessions schema + runner, bcrypt, cookie Resolver, register/login/logout/me, router + views + guard.

**Out of scope:** reset/email, OAuth, roles/sharing, JWT, migration lib, e2e runner, `/healthz` changes.

## Architecture / Approach

`migrations/*.sql` → boot runner → `internal/auth` store + Resolver + handlers → `RequireAccount` on `GET me` → Vue router + `api/client.ts` (`VITE_API_BASE_URL`, `credentials:same-origin`) → Register/Login views.

## Phases at a Glance

| Phase | What it delivers | Key risk |
|---|---|---|
| 1. Schema + store | users/sessions + runner | Boot vs degraded-healthz conflict |
| 2. Session API | bcrypt + cookie + 4 endpoints | Secure-cookie localhost break |
| 3. Web auth | Router + views + guard | TS unused-check failures |

**Prerequisites:** `DATABASE_URL` (dev/Neon branch), `SESSION_SECRET` ≥32ch, Node 22 + npm.
**Estimated effort:** ~2–3 sessions across 3 phases.

## Open Risks & Assumptions

- Public URL + open reg accumulates stranger accounts until disable-reg flag (v2).
- No token rotation in S-01; sliding expiry only.
- Preview apps must use separate Neon branch (privacy).

## Success Criteria (Summary)

- `go test ./...` + `go vet` + `npm run build` green.
- Browser register→reload→logout cycle works; two accounts isolated.
