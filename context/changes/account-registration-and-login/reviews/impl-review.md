<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Account registration and login

- **Plan**: context/changes/account-registration-and-login/plan.md
- **Scope**: All phases (1–3) of 3
- **Date**: 2026-09-12
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 6 warnings, 4 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------
| Plan Adherence | WARNING |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | WARNING |

## Findings

### F1 — Login timing oracle enumerates registered emails

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:91
- **Detail**: `!found || !CheckPassword(...)` short-circuits; unknown email skips bcrypt while known email pays cost-12. Timing leaked account existence.
- **Fix**: On `!found`, compare against a fixed cost-12 dummy hash before returning 401.
- **Decision**: FIXED — `dummyPasswordHash` const + explicit branch; unit test `TestDummyPasswordHash`.

### F2 — bcrypt auth endpoints unthrottled (CPU/pool exhaustion)

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:51,91
- **Fix B (chosen)**: `golang.org/x/time/rate` per-IP token bucket.
- **Decision**: FIXED via Fix B — new `internal/auth/ratelimit.go` (1/s, burst 5, keyed by Fly-Client-IP/RemoteAddr) wired to register+login in `spa.go`; dep pinned x/time v0.5.0 (go1.22-compatible); unit test `TestLimitByIP`.

### F3 — Core auth tests SKIP without DATABASE_URL; CI green vacuous

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Success Criteria
- **Location**: internal/auth/handler_test.go:17, store_test.go:17, .github/workflows/fly.yml:64-76
- **Fix A (chosen)**: Postgres service container + DATABASE_URL in CI; assert `expires_at` moves in the TouchSession test.
- **Decision**: FIXED via Fix A — `go-test` job gains `postgres:16` service + job-level `DATABASE_URL`; `TestSessionLifecycle` now reads `expires_at` back and asserts extension.

### F4 — Unbounded request body on unauthenticated endpoints

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:150-155
- **Fix**: `http.MaxBytesReader(w, r.Body, 4<<10)`.
- **Decision**: FIXED (`maxRequestBodyBytes`).

### F5 — http.Server has no timeouts (slowloris / slow-body)

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: cmd/nest-cash/main.go:76-79
- **Fix**: ReadHeaderTimeout 5s, ReadTimeout 15s, WriteTimeout 30s, IdleTimeout 60s.
- **Decision**: FIXED.

### F6 — Sliding refresh fires only on GET /me, not "authed hits"

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: internal/auth/handler.go:140-144, resolver.go:22-38
- **Fix**: Amend plan wording to "refresh on GET /api/auth/me" (S-01's only authed route).
- **Decision**: FIXED — plan.md contract updated.

### F7 — Logout leaves cookie set when DeleteSession errors

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:109-112
- **Fix**: clearCookie before writeServerError.
- **Decision**: FIXED.

### F8 — DeleteExpiredSessions is dead code

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/store.go:153
- **Fix**: call at boot + hourly ticker.
- **Decision**: FIXED — `purgeExpiredSessions` goroutine in main.go.

### F9 — validEmail too permissive

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:218-224
- **Fix**: net/mail.ParseAddress + domain label checks.
- **Decision**: FIXED — unit test `TestValidEmail`.

### F10 — /me does 2 user SELECTs + needless UPDATE

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/handler.go:131, resolver.go:30
- **Detail**: resolver loads full users row (incl. password_hash) then Me queries FindUserByID again; TouchSession UPDATEs every GET.
- **Fix**: single resolve; drop password_hash; conditional refresh.
- **Decision**: FIXED (partial) — dropped `password_hash` from `FindSessionUser` (data minimization). Single-resolve + conditional refresh deferred: both need the locked F-01 `account.Resolver` contract to surface profile/expiry (injecting them requires an optional interface or an auth-owned middleware replacing `RequireAccount`). Negligible at personal scale; revisit in S-02 when more authed routes land.
- **Follow-up**: context/changes/account-registration-and-login/follow-ups/review-fixes.md

## Notes

- Automated re-run GREEN after fixes: `CGO_ENABLED=0 go test ./...`, `go vet ./...`, `gofmt -l` clean, `npm --prefix web run build` OK.
- Phase-1 prior findings F1–F10 re-verified fixed.
- Accepted by design (no action): register 409 `email_taken` account oracle (plan.md:119); duplicate JSON error writers across auth/account/main; in-handler method guards unreachable under Go 1.22 method patterns; migrate DDL race on concurrent first boot (single-machine Fly); auth-failure logging not emitted (zero PII logged).
- F10 partial leaves a known low-impact perf gap, not a correctness/security gap.
