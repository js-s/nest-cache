# Account registration and login — Implementation Plan

## Overview

S-01 delivers vertical register/login/logout/me: Postgres users+sessions, bcrypt passwords, opaque cookie session wired into existing `internal/account` Resolver, plus Vue register/login views behind vue-router. No password reset in S-01 (documented v2).

## Current State Analysis

Backend serves `GET /healthz` + `/api` JSON boundary + SPA fallback, optional `DATABASE_URL` pgx pool, no domain routes (`cmd/nest-cash/main.go:21-54`, `cmd/nest-cash/spa.go:11-24`). `internal/account` defines `AccountID` context contract + `RequireAccount(resolver,next)` fail-closed 401, no Resolver impl (`internal/account/middleware.go:9-41`); comment says session layer lands in S-01.

Data: no `.sql`, no migrations, no schema. Deps: pgx v5.6.0 direct, x/crypto v0.17.0 indirect only (`go.mod:7-16`). Secrets `DATABASE_URL` + `SESSION_SECRET` live in Fly only, code reads neither except `DATABASE_URL` (`context/deployment/deploy-plan.md:150-154`).

Frontend is stock Vite Vue TS: no router/store/api client, `VITE_API_BASE_URL=/api` unread, proxy `/api→localhost:8080` ready (`web/package.json:11-13`, `web/vite.config.ts:8-13`, `web/src/main.ts:1-5`, `web/src/App.vue:1-7`). Build gate `vue-tsc -b && vite build` with unused checks (`web/tsconfig.app.json:9-12`).

Blocker resolved in planning: password reset OUT of S-01.

## Desired End State

User registers with email+password in web UI, logs in, stays logged across restarts (30d sliding cookie), hits `GET /api/auth/me` 200, logs out to 401. Second account sees none of first account's future data (isolation via `AccountID`). Verify: `go test ./...`, `go vet ./...`, `npm --prefix web run build`, manual register→reload→logout flow + two-account PG isolation query.

### Key Discoveries:

- `cmd/nest-cash/spa.go:18` already uses Go 1.22 method-patterns (`GET /healthz`) — new `POST /api/auth/*` patterns fit, longer paths beat `/api/` catch-all, order irrelevant.
- `internal/account/middleware.go:18-41` is reuse-as-is; S-01 only implements `Resolver`.
- `go.mod:14` x/crypto present — promote to direct, zero new backend deps.
- `web/vite.config.ts:8-13` proxy ready; only missing client using `VITE_API_BASE_URL` + `credentials`.
- `context/deployment/deploy-plan.md:151` reserves 64-char `SESSION_SECRET` — S-01 reads it fail-closed.

## What We're NOT Doing

- Password reset/recovery, email delivery (v2; note in UI copy).
- OAuth, roles, multi-user sharing, account deletion (PRD non-goals).
- JWT, external session stores, CSRF tokens beyond SameSite (same-origin; revisit if SPA host splits).
- Migration lib (goose/migrate), ORM, frontend e2e runner.
- Touching `/healthz` public contract, Fly secrets values, CI workflow.

## Implementation Approach

DB-first vertical: schema+migration runner → session Resolver+auth endpoints → web client+views. stdlib + pgx + bcrypt only; opaque 256-bit token, sha256 hash stored, cookie carries raw value. `SESSION_SECRET` signs nothing in opaque scheme — used as startup presence gate + future HMAC domain separator; missing/weak → fail-closed (no auth routes registered, log once). Keep `/healthz` degraded-mode behavior.

## Critical Implementation Details

- **State sequencing** — migration runs once at boot with 5s timeout before `ListenAndServe`; per-request migration forbidden. Resolver DB lookup precedes `WithAccountID`; never call downstream on unresolved identity.
- **Cookie gotchas** — `Path=/` mandatory (default scopes to `/api/auth`), omit `Domain` (host-only), `Secure` gated on TLS/env or localhost login breaks, `SameSite=Lax` (Strict breaks top-level flow). Logout clears with identical Path+Name, `MaxAge:-1`.
- **Debug & observability** — 401/404 bodies stay `{"error":...}` + `no-store`, never leak email/exists. Login failures return identical shape for bad-email vs bad-password. Log auth failures without PII.

## Phase 1: Postgres schema, migration runner, store

### Overview

Versioned `.sql` + tiny Go runner at boot + `users`/`sessions` store over existing pgx pool.

### Changes Required:

#### 1. Versioned schema files

**File**: `migrations/0001_auth.sql` (new)

**Intent**: Create `users` (id uuid PK, email citext/unique normalized, password_hash text, created_at) and `sessions` (token_hash PK, user_id FK, expires_at, created_at). Idempotent via `CREATE TABLE IF NOT EXISTS`.

**Contract**: Single up-only file; index on `sessions(user_id, expires_at)`; no down migration in S-01.

#### 2. Migration runner at startup

**File**: `cmd/nest-cash/main.go`, new `internal/db/migrate.go`

**Intent**: Apply pending `.sql` once at boot with timeout + advisory courtesy; preserve degraded `/healthz` when `DATABASE_URL` unset.

**Contract**: `Migrate(ctx, db, embedFS)` called after `sql.Open`, before routes; `DATABASE_URL` empty → skip (degraded as today); migration error → `log.Fatal` (fail-closed, explicit); embed via `go:embed`.

#### 3. Users/sessions store

**File**: `internal/auth/store.go` (new package `auth`; session Resolver lives Phase 2)

**Intent**: CRUD for users by email/id + sessions by token_hash with expiry; normalization (trim+lowercase) at boundary.

**Contract**: `CreateUser(ctx,email,passwordHash)`, `FindUserByEmail`, `CreateSession`, `FindSession→user_id`, `DeleteSession`, `DeleteExpiredSessions`; `$1` placeholders via `database/sql`+pgx; expired sessions never resolve.

### Success Criteria:

#### Automated Verification:

- `go test ./internal/auth -run 'TestStore'` passes against real PG (`DATABASE_URL` set; skip-guard otherwise)
- `gofmt -d` clean on new files
- `go vet ./...` passes

#### Manual Verification:

- Fresh Neon branch + boot applies 0001 once; reboot idempotent
- Two users visible, emails unique case-insensitively
- Boot without `DATABASE_URL` still serves `GET /healthz` 503 degraded

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Cookie session Resolver + auth endpoints

### Overview

bcrypt passwords, opaque session cookie, `account.Resolver` impl, four endpoints wired into `routes()`.

### Changes Required:

#### 1. Password + token primitives

**File**: `internal/auth/password.go`, `internal/auth/session.go`

**Intent**: Hash/compare via x/crypto bcrypt cost 12; 32-byte `crypto/rand` tokens, base64url, sha256-hex store.

**Contract**: `HashPassword`, `CheckPassword` (72-byte limit enforced with clear 400); `NewToken→(raw, hash)`; never persist/log raw token.

#### 2. Session Resolver + handlers

**File**: `internal/auth/handler.go`, `internal/auth/resolver.go`

**Intent**: `POST /api/auth/register` (validate→hash→create→auto-login), `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` (behind `RequireAccount`).

**Contract**: Routes `POST /api/auth/register|login|logout`, `GET /api/auth/me`; cookie `nest_session`, `Path=/`, `HttpOnly`, `SameSite=Lax`, `Secure` iff TLS/prod, `MaxAge=2592000` + sliding refresh on `me`/authed hits; `me` returns `{id,email}`; logout deletes server session + clears cookie; bad credentials → uniform 401 `{"error":"unauthorized"}`; validation failures → 400 `{"error":"invalid_request"}`; `Register` duplicate email → 409 `{"error":"email_taken"}` without revealing login-oracle beyond register.

#### 3. Wiring + startup gate

**File**: `cmd/nest-cash/main.go`, `cmd/nest-cash/spa.go`

**Intent**: Read `SESSION_SECRET` (min 32 chars) fail-closed; construct store→resolver→handlers; register patterns; keep `/healthz` public.

**Contract**: Missing/weak secret → auth routes unregistered + fatal log; `GET /api/auth/me` wrapped in existing `RequireAccount`; no change to `/`, `/api` fallback, SPA handler.

### Success Criteria:

#### Automated Verification:

- `go test ./internal/auth` passes (bcrypt round-trip, token resolve/expiry, handler register→login→me→logout via httptest)
- `go test ./...` passes
- `go vet ./...` + `gofmt -d` clean

#### Manual Verification:

- `curl` register→cookie set; restart client keeps session; logout clears; expired token 401
- Two-account PG check: session A never resolves user B rows
- Dev `http://localhost` login works (Secure gate); prod `https` sets Secure

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 3: Web register/login + session guard

### Overview

vue-router, typed API client, Register/Login views, auth guard, logout; replace HelloWorld stub.

### Changes Required:

#### 1. Router + API client

**File**: `web/src/router.ts` (new), `web/src/api/client.ts` (new), `web/src/main.ts`

**Intent**: Routes `/login /register /summary` (summary placeholder in S-01); client reads `VITE_API_BASE_URL` with `credentials: same-origin`, JSON helpers, typed `me()`.

**Contract**: New dep `vue-router@4`; `main.ts` installs router; unknown routes redirect `/login` when unauthenticated; client throws typed `ApiError{status}`; no secret/ANYPATH hardcode.

#### 2. Auth views + state

**File**: `web/src/views/Register.vue`, `web/src/views/Login.vue` (new), `web/src/App.vue`

**Intent**: Email+password forms, inline validation (email shape, min 8), server error mapping (409 taken, 401 bad credentials), minimal session check on boot.

**Contract**: Forms POST via client, success → router push `/summary`; `App.vue` drops HelloWorld, hosts `<router-view>` + nav with logout button calling `POST logout` then push `/login`; copy notes "password reset coming in v2".

### Success Criteria:

#### Automated Verification:

- `npm --prefix web run build` passes (vue-tsc + vite)
- No new lint config needed; existing unused-checks clean

#### Manual Verification:

- Register→auto-login→reload keeps session→logout→guard redirects `/login`
- Wrong password shows generic error, no account-oracle
- Proxy dev (`npm run dev` + Go `:8080`) and built SPA via Go static serve both work

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Testing Strategy

### Unit Tests:

- bcrypt hash/compare, long-password 400, email normalization uniqueness
- Token new/resolve/expiry/sliding, raw never stored
- Resolver: valid/invalid/expired/missing → allow/401
- Middleware reuse untouched (existing `internal/account` tests stay green)

### Integration Tests:

- httptest full cycle register→login→me→logout→me(401)
- Real-PG two-account isolation (requires `DATABASE_URL`, else skip with notice)
- Boot migration idempotency

### Manual Testing Steps:

1. Fresh DB boot, register A, register B, cross-check isolation query
2. Browser: register→reload→logout→login→bad-password message
3. `curl -i` cookie flags (HttpOnly, Path=/, SameSite, Secure on https)
4. Reboot server, session persists (30d); delete session row → 401

## Performance Considerations

bcrypt cost 12 ~250ms/login — off hot path, fine for personal scale. Session lookup single indexed PK per authed request; pool max 5 unchanged. Sliding refresh writes expiry only (no token rotation in S-01; rotation v2 if needed). Frontend no bundle concern (one router dep).

## Migration Notes

0001 up-only; rollback = drop `sessions`,`users` (data loss, acceptable pre-first-user). No backfill. Neon branch per preview (privacy). Future 0002 adds reset tokens if v2 approves.

## References

- PRD FR-001/002 + NFR: `context/foundation/prd.md:55-59,82-85`
- S-01 slice + reset blocker: `context/foundation/roadmap.md:73-84,150-153`
- F-01 contract: `context/changes/account-data-boundary/plan.md:19-28,49-57`
- Server + routes: `cmd/nest-cash/main.go:21-54`, `cmd/nest-cash/spa.go:11-24`
- Account gate: `internal/account/identity.go:11-35`, `internal/account/middleware.go:9-41`
- Secrets: `context/deployment/deploy-plan.md:148-175`
- Web scaffold: `web/package.json:6-22`, `web/vite.config.ts:5-14`, `web/src/main.ts:1-5`, `web/tsconfig.app.json:9-12`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Postgres schema, migration runner, store

#### Automated

- [x] 1.1 `go test ./internal/auth -run 'TestStore'` passes against real PG
- [x] 1.2 `gofmt -d` clean on new files
- [x] 1.3 `go vet ./...` passes

#### Manual

- [x] 1.4 Fresh DB boot applies 0001 once; reboot idempotent
- [x] 1.5 Two users visible, emails unique case-insensitively
- [x] 1.6 Boot without `DATABASE_URL` still serves degraded `/healthz`

### Phase 2: Cookie session Resolver + auth endpoints

#### Automated

- [ ] 2.1 `go test ./internal/auth` passes (bcrypt, token, handler cycle)
- [ ] 2.2 `go test ./...` passes
- [ ] 2.3 `go vet ./...` + `gofmt -d` clean

#### Manual

- [ ] 2.4 curl register→cookie; restart-safe; logout clears; expired 401
- [ ] 2.5 Two-account isolation check passes
- [ ] 2.6 Secure-cookie gate correct on localhost vs https

### Phase 3: Web register/login + session guard

#### Automated

- [ ] 3.1 `npm --prefix web run build` passes

#### Manual

- [ ] 3.2 Register→reload→logout→guard redirect works
- [ ] 3.3 Generic bad-credential error, no oracle
- [ ] 3.4 Dev proxy and built SPA serve both work
