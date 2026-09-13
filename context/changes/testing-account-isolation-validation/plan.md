# Account Isolation & Validation Hardening Tests — Implementation Plan

## Overview

Rollout Phase 1 of `context/foundation/test-plan.md`: add HTTP-level integration tests that prove (a) one account cannot read or write another account's data (#1), (b) semantically invalid input is rejected with 400 and writes nothing while valid input is accepted (#4), and (c) every protected route rejects unauthenticated requests and leaks nothing (#6). Finish by reconciling `test-plan.md` so the cookbook and risk wording match reality.

This phase adds **tests and documentation only** — no production code changes.

## Current State Analysis

- Isolation is enforced in SQL, per query (`user_id = $1` everywhere; atomic ownership+kind gates on `category_id`/`parent_id`). `RequireAccount` proves authentication only. See `context/changes/testing-account-isolation-validation/research.md`.
- Stores are concrete `*sql.DB` (`internal/transactions/store.go:50`, `internal/categories/store.go:34`) — no interface to fake. #1/#4 need real Postgres.
- Test conventions: `httptest.NewRequest` + `NewRecorder`; owner attached via `account.WithAccountID` (`internal/transactions/handler_test.go:14`); DB helpers `openTestDB`/`createTestUser`/`createCategory` live per package and `t.Skip` without `DATABASE_URL` (`internal/transactions/store_test.go:15-65`).
- Existing 401 tests are handler-direct or unit-level; `spa_test.go:24` builds `routes()` with nil auth, so an unwrapped protected route would pass today.
- A real two-cookie chain exists only for `/api/auth/me` (`internal/auth/handler_test.go:175-206`).
- CI provisions `postgres:16` and runs `go test ./...` (`.github/workflows/fly.yml:64-92`).
- Research corrections: malformed `page`/`limit` are silently defaulted (`internal/transactions/handler.go:215-227`), over-long `description` is silently truncated (`internal/transactions/store.go:100`) — neither returns 400.

## Desired End State

`go test ./...` (with `DATABASE_URL` in CI) runs new integration tests that fail if: an account can reach another's rows, a 400 path writes a row, or a protected route is served unauthenticated. `test-plan.md` §3 Phase 1 is `complete`, §6 documents the shipped patterns and reference tests, and §2 risk #4 wording matches actual behavior. Verify by running the suite and by the deliberate-break checks below.

### Key Discoveries:

- A **DB-free** route-wiring test is possible: handlers read the account from context before touching the store, and a zero-value `&auth.Resolver{}` (nil store) fails closed → 401. Build `application{...}.routes()` in `package main` with nil-store handlers to prove every protected route is wrapped (`cmd/nest-cash/spa.go:21-35`).
- Cross-account isolation can be driven through the **real session chain** without the rate limiter by calling `auth.Store.CreateUser` + `auth.NewToken` + `Store.CreateSession` directly (the limiter only wraps register/login inside `routes()`, `internal/auth/ratelimit.go:47-56`).
- No-write must be asserted with an explicit row count (`SELECT count(*) ... WHERE user_id = $1`), not by the response alone — the test-plan anti-pattern is "asserting only that a request succeeded".
- All 400 responses use the generic public body `{"error":"invalid_request"}`; internal store errors are classified by `strings.Contains(err.Error(), "invalid input")` (`internal/categories/handler.go:97,131`) and must never be asserted.

## What We're NOT Doing

- Fixing the lenient `page`/`limit` behavior or the `description` truncation. Tests only; the gaps are captured as a §2 documentation note and remain open follow-ups.
- Changing any production code or migrations.
- Wiring a frontend test runner (Phase 3 of the test plan).
- Testing framework/mux internals, or cookie flag details (already covered by `internal/auth/handler_test.go`).
- Adding a shared `testutil` package — helpers stay package-local.

## Implementation Approach

Order by cost × signal: cheapest and always-on first (DB-free #6), then the two DB-backed risks by priority (#1 before #4). Each DB phase uses the existing package helpers and asserts both response and side-effect. A final documentation phase reconciles the test plan.

## Critical Implementation Details

- **Rate limiter**: only `routes()`-wired register/login are throttled. New tests that call handlers or `auth.Store` directly bypass `Limit`, so no 429 handling is needed. Do not drive auth through `routes()` in these tests.
- **DB gating**: #1/#4 tests must call the package `openTestDB` (skips without `DATABASE_URL`). The #6 route test must NOT call it, so it always runs locally and in CI.
- **macOS**: run DB tests with `CGO_ENABLED=0` to avoid the `dyld: missing LC_UUID` abort noted in `AGENTS.md`.
- **Cleanup ordering**: in `internal/transactions`, `openTestDB` registers `db.Close` before per-user cleanups, so LIFO runs row deletes first, then closes the pool. Reuse the existing helpers; don't re-register closes.
- **Nil-store safety in the #6 test**: handlers built with `NewHandler(nil)` are never invoked because `RequireAccount` returns 401 first; constructing them must not dereference the store.

## Phase 1: Route-wiring auth guard (Risk #6, DB-free)

### Overview

Prove through the real mux that every protected route is behind `RequireAccount` and rejects unauthenticated requests with a non-leaking 401 — without a database.

### Changes Required:

#### 1. New route-auth test

**File**: `cmd/nest-cash/routes_auth_test.go`

**Intent**: Guard against a protected route being registered without `RequireAccount`; today no test would catch that because `spa_test.go` wires nil auth.

**Contract**: `package main` test. Build `application{staticDir: t.TempDir(), auth: auth.NewHandler(nil, false), resolver: auth.NewResolver(nil), categories: categories.NewHandler(nil), transactions: transactions.NewHandler(nil)}`, call `.routes()`. Table over the protected routes `GET /api/auth/me`, `GET|POST /api/categories`, `GET|POST /api/transactions`, `GET /api/summary`; for each assert: no cookie → 401, `Content-Type: application/json`, body error `unauthorized`, and no `Set-Cookie`; repeat with a bogus `nest_session` cookie → 401. Add open-route control cases (`/healthz` 503, `/api/unknown` 404, `/` 200 with an `index.html` written to `staticDir`) to prove the harness distinguishes protected from open.

### Success Criteria:

#### Automated Verification:

- `CGO_ENABLED=0 go test ./cmd/nest-cash/...` passes with no `DATABASE_URL` set, and the new test is not skipped
- Every protected route returns 401 with `{"error":"unauthorized"}` and no `Set-Cookie`

#### Manual Verification:

- Deliberate-break: temporarily register one protected route without `RequireAccount`; the new test fails; restore the wrapper
- The test table enumerates all protected routes from `cmd/nest-cash/spa.go` (none silently omitted)

---

## Phase 2: Cross-account isolation through the real session chain (Risk #1, DB)

### Overview

Prove Account B cannot read Account A's transactions/categories or write using A's resource ids, and that A's rows are unchanged.

### Changes Required:

#### 1. Transactions isolation test

**File**: `internal/transactions/isolation_integration_test.go`

**Intent**: Exercise the real resolver/session chain (not `WithAccountID`) for the transactions/summary endpoints and assert DB side-effects.

**Contract**: `openTestDB`; build two owners via `auth.NewStore(db).CreateUser` + `auth.NewToken` + `auth.Store.CreateSession`, and cookies `&http.Cookie{Name: auth.CookieName, Value: raw}`; wrap handlers with `account.RequireAccount(auth.NewResolver(auth.NewStore(db)), http.HandlerFunc(h.Create/List/Summary))`. Cases: A seeds a category + transaction; B's list/summary return no A ids and zero totals; B's `POST /api/transactions` with A's `category_id` → 400; B's `GET /api/summary?category_id=<A's>` → 400. After each rejected write, assert `SELECT count(*) FROM transactions WHERE user_id = A` is unchanged.

#### 2. Categories isolation test

**File**: `internal/categories/isolation_integration_test.go`

**Intent**: Same chain for categories, including the parent-ownership gate.

**Contract**: A seeds categories (via `List` so ids exist); B's `List` contains none of A's names; B's `POST /api/categories` with A's `parent_id` → 400; assert A's category count (via the package `count`) is unchanged after the rejected write.

### Success Criteria:

#### Automated Verification:

- `CGO_ENABLED=0 DATABASE_URL=<test db> go test ./internal/transactions/... ./internal/categories/...` passes
- B never observes A's rows in list/summary; every cross-account write returns 400 with A's row counts unchanged

#### Manual Verification:

- Deliberate-break: remove one `user_id = $1` predicate (or the `c.user_id = $1` gate in the transaction insert); the matching isolation test fails; restore it

---

## Phase 3: Validation + no-write (Risk #4, DB)

### Overview

Prove semantically invalid input is rejected with 400 + the generic error code and writes nothing, while valid input is accepted; pin the documented lenient `page`/`limit` behavior.

### Changes Required:

#### 1. Transactions validation test

**File**: `internal/transactions/validation_integration_test.go`

**Intent**: Extend the response-only validation matrix with no-write side-effect assertions and a documented boundary case.

**Contract**: `openTestDB`, owner via `createTestUser`, `NewStore(db)`, `authedRequest`. Valid case → 201 and transaction count +1. Invalid matrix (reuse existing cases: bad JSON, zero/negative/too-many-decimals/non-numeric/empty amount, bad date, empty/foreign/income/unknown category, bad kind, kind mismatch; plus `GET /api/transactions?kind=grant`, summary missing/bad dates, `from>to`, bad/unknown `category_id`) → 400, body error `invalid_request`, and count unchanged. Boundary: `GET /api/transactions?page=abc&limit=-1` → 200 with defaulted `page`/`limit` (documents the lenient behavior).

#### 2. Categories validation test

**File**: `internal/categories/validation_integration_test.go`

**Intent**: Same no-write guarantee for category creation.

**Contract**: owner via `createTestUser`, `NewStore(db)`, `authedRequest`. Invalid matrix (empty/whitespace name, missing/bad kind, name > 80 runes, bad JSON, foreign parent, kind mismatch, unknown parent) → 400 + `invalid_request`; assert the owner's category count is unchanged after each. Valid group → 201 and count +1.

### Success Criteria:

#### Automated Verification:

- `CGO_ENABLED=0 DATABASE_URL=<test db> go test ./internal/transactions/... ./internal/categories/...` passes
- Each invalid case returns 400 + `invalid_request` with zero rows written; each valid case returns 201 with one row written
- Malformed `page`/`limit` returns 200 with defaults (documented, not a 400)

#### Manual Verification:

- Deliberate-break: loosen one guard (e.g. accept `0` amount); the matching 400 case fails; restore it

---

## Phase 4: Test-plan reconciliation (docs)

### Overview

Make `test-plan.md` reflect what shipped.

### Changes Required:

#### 1. Cookbook + status + freshness

**File**: `context/foundation/test-plan.md`

**Intent**: Replace the "TBD — see §3 Phase 1" cookbook stubs with the shipped patterns and reference tests, mark Phase 1 complete, and update the freshness ledger.

**Contract**: §6.2 and §6.3 name the new reference tests (`cmd/nest-cash/routes_auth_test.go`, `internal/transactions/isolation_integration_test.go`, `internal/transactions/validation_integration_test.go`, categories analogs) and the run command including `CGO_ENABLED=0` + `DATABASE_URL`; §3 row 1 Status → `complete`; §8 "Strategy last reviewed" updated.

#### 2. Risk #4 wording corrections

**File**: `context/foundation/test-plan.md`

**Intent**: Correct the §2 response-guidance cell so it does not imply a 400 for behavior that is lenient.

**Contract**: In §2 Risk Response Guidance, replace "malformed filter" with "semantically invalid filter (bad `kind`, bad/`from>to` dates, foreign/unknown/income category)"; add that `page`/`limit` are lenient by design and over-long `description` is truncated, neither a 400. No file:line anchors added (principle #3).

### Success Criteria:

#### Automated Verification:

- `test-plan.md` §6.1–.3 contain no "TBD — see §3 Phase 1" and name the shipped reference tests
- §3 Phase 1 Status reads `complete`

#### Manual Verification:

- Read-through: §6 matches the shipped test files; §2 #4 wording matches actual handler behavior

---

## Testing Strategy

### Unit Tests:

- None added — the three risks are integration-level; `internal/account` middleware 401 unit tests already exist.

### Integration Tests:

- Route-wiring 401 per protected route (DB-free).
- Cross-account read/write isolation through real sessions (DB).
- Validation 400 + no-write matrix and valid-accepted (DB).

### Manual Testing Steps:

1. Run `CGO_ENABLED=0 go test ./...` with `DATABASE_URL` unset — the #6 test runs, DB tests skip, everything is green.
2. Run with `DATABASE_URL` pointed at a scratch Postgres — all new tests run and pass.
3. Deliberate-break each guard (unwrapped route, removed `user_id` predicate, loosened validation) and confirm exactly the intended test fails; restore.

## Performance Considerations

None. Test-only change; the DB tests run against the CI Postgres service.

## Migration Notes

None. No schema or data changes.

## References

- Related research: `context/changes/testing-account-isolation-validation/research.md`
- Test plan: `context/foundation/test-plan.md` (§2 Risk #4, §3 Phase 1, §6.2/.3)
- Existing patterns: `internal/transactions/handler_test.go:14`, `internal/transactions/store_test.go:15-65`, `internal/auth/handler_test.go:23-49,175-206`, `cmd/nest-cash/spa_test.go:24`
- Wrapping/route table: `internal/account/middleware.go:18`, `cmd/nest-cash/spa.go:21-35`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Route-wiring auth guard (Risk #6, DB-free)

#### Automated

- [x] 1.1 `CGO_ENABLED=0 go test ./cmd/nest-cash/...` passes with no `DATABASE_URL`; new route-auth test not skipped
- [x] 1.2 Every protected route returns 401 `{"error":"unauthorized"}` with no `Set-Cookie`

#### Manual

- [ ] 1.3 Deliberate-break: unwrapped protected route makes the test fail (then restore)
- [ ] 1.4 Test table enumerates all protected routes from `routes()`

### Phase 2: Cross-account isolation through the real session chain (Risk #1, DB)

#### Automated

- [ ] 2.1 `CGO_ENABLED=0 DATABASE_URL=<test db> go test ./internal/transactions/... ./internal/categories/...` passes
- [ ] 2.2 B observes none of A's rows in list/summary; cross-account writes return 400

#### Manual

- [ ] 2.3 Deliberate-break: removing a `user_id` predicate / ownership gate fails the isolation test (then restore)

### Phase 3: Validation + no-write (Risk #4, DB)

#### Automated

- [ ] 3.1 Invalid cases return 400 + `invalid_request` with zero rows written; valid cases return 201 with one row
- [ ] 3.2 Malformed `page`/`limit` returns 200 with defaults (documented, not a 400)

#### Manual

- [ ] 3.3 Deliberate-break: loosening a validation guard fails the matching 400 case (then restore)

### Phase 4: Test-plan reconciliation (docs)

#### Automated

- [ ] 4.1 `test-plan.md` §6.1–.3 name the shipped reference tests; §3 Phase 1 Status is `complete`; §8 updated
- [ ] 4.2 §2 Risk #4 wording corrected (semantic filters; `page`/`limit` lenient; `description` truncation)

#### Manual

- [ ] 4.3 Read-through: §6 matches shipped tests and §2 #4 matches handler behavior
