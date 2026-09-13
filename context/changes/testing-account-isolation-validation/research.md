---
date: 2026-09-13T17:02:26+0200
researcher: Janusz Smogorzewski
git_commit: f65dd59396e8c41088cb7ca4eaaaf4904084cd18
branch: main
repository: nest-cache
topic: "Ground rollout Phase 1 of context/foundation/test-plan.md — account isolation (#1), input validation (#4), protected routes (#6)"
tags: [research, codebase, account, auth, transactions, categories, validation, isolation, integration-tests]
status: complete
last_updated: 2026-09-13
last_updated_by: Janusz Smogorzewski
---

# Research: Ground rollout Phase 1 — account isolation, validation, unauthenticated access

**Date**: 2026-09-13T17:02:26+0200
**Researcher**: Janusz Smogorzewski
**Git Commit**: [f65dd59](https://github.com/js-s/nest-cache/blob/f65dd59396e8c41088cb7ca4eaaaf4904084cd18)
**Branch**: main
**Repository**: nest-cache

> Permalinks below are pinned to commit `f65dd59`. The working tree also holds
> untracked `context/foundation/test-plan.md` and this change folder — not yet
> committed, so they are referenced by path only.

## Research Question

Ground rollout Phase 1 of `context/foundation/test-plan.md`. Read the change folder fully.
Risks to verify: **#1** (one account reads/writes another's data), **#4** (invalid input accepted), **#6** (protected route served unauthenticated / session drop).
For each risk: ground the real failure path in code, quote relevant lines, verify or correct the test-plan's response guidance, locate existing tests, identify the cheapest useful test layer, and flag speculative risks or misleading hot-spot evidence.

## Summary

All three risks are **real and code-enforced, not speculative** — nothing in the change folder needs dropping or reframing.

- **#1 (isolation/IDOR)** — Protection is enforced **in SQL, per query**, not by middleware convention. `RequireAccount` only proves *authentication*; ownership is proven by `user_id = $1` on every category/transaction/summary statement, plus an atomic ownership+kind gate on the two request-supplied resource IDs (`category_id`, `parent_id`). No endpoint accepts a transaction ID. The only existing cross-account HTTP-level test drives `/api/auth/me`; no test drives `transactions`/`categories`/`summary` through the real middleware with two real owners.
- **#4 (validation)** — Server-side validation is real and layered (handler → store → DB CHECK). But the test-plan's wording "malformed filter" is **partially wrong**: malformed/pathological `page`/`limit` are *silently defaulted*, not rejected with 400, and over-long `description` is *silently truncated*, not rejected. Semantically invalid values (bad `kind`, missing/bad dates, `from>to`, foreign/income/unknown category, bad JSON, amount ≤ 0) do return 400 and write nothing.
- **#6 (unauth access)** — Every protected route is wrapped in `RequireAccount`, which fails closed (401, `{"error":"unauthorized"}`, no data, no `Set-Cookie`). The strongest counterexample to "login page renders ⇒ auth works" is **degraded mode**: with no `DATABASE_URL`, the SPA still serves the login form while every auth/API route returns 404. No current test builds `routes()` with a non-nil resolver and asserts 401 per protected route.

Cheapest reliable layer: **#6 is fully testable DB-free** (handler reads the account from context before touching the store; and `routes()` can be built in-package with a nil-store resolver that fails closed). **#1 and #4 require real Postgres** — the stores are concrete `*sql.DB`, the ownership gate *is* SQL, and CI already provisions `postgres:16`.

## Detailed Findings

### Risk #1 — Account A reads/writes Account B's data (isolation / IDOR)

**The trust chain (owner identity never comes from request input):**

| Step | Location | Behavior |
|---|---|---|
| Session cookie read | [`internal/auth/resolver.go:26-29`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/resolver.go#L26-L29) | `r.Cookie(CookieName)`; empty → `("", false)` |
| Server-side session lookup | [`internal/auth/store.go:112-116`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/store.go#L112-L116) | `WHERE s.token_hash = $1 AND s.expires_at > now()` |
| AccountID construction | [`internal/auth/resolver.go:34-38`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/resolver.go#L34-L38) | `account.AccountID(user.ID)` + validity gate |
| Middleware | [`internal/account/middleware.go:30-40`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/account/middleware.go#L30-L40) | 401 if unresolved; else `WithAccountID` on context |
| Handler re-check | e.g. [`internal/transactions/handler.go:87-91`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L87-L91) | `AccountIDFromContext`; 401 on failure (defense in depth) |
| Route guard | [`cmd/nest-cash/spa.go:27-35`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa.go#L27-L35) | all category/transaction/summary routes wrapped in `RequireAccount` |

There is **no code path that reads an owner id from request data.** The only request-supplied identifiers are the resource IDs `category_id` and `parent_id`.

**Ownership is scoped in SQL on every query** (verified exhaustively):

| Endpoint | Owner-scoped statement |
|---|---|
| GET /api/categories | [`categories/store.go:61`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store.go#L61) `... WHERE user_id = $1` |
| POST /api/categories (group) | [`categories/store.go:112-114`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store.go#L112-L114) `INSERT ... (user_id, kind, name)` |
| POST /api/categories (sub) | [`categories/store.go:136`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store.go#L136) `SELECT kind FROM categories WHERE id = $1 AND user_id = $2 AND parent_id IS NULL` |
| GET /api/transactions | [`transactions/store.go:165`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L165), [:174-180](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L174-L180) `... WHERE t.user_id = $1` |
| POST /api/transactions | [`transactions/store.go:106-116`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L106-L116) atomic `INSERT ... SELECT ... WHERE c.id = $5 AND c.user_id = $1 AND c.kind = $6` |
| GET /api/summary | [`summary.go:112-114`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L112-L114), [:123](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L123), [:164-166](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L164-L166), [:202-210](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L202-L210) — all `user_id = $1` |

**Resource-ID ownership is enforced, not assumed:**
- Transaction create gate is one atomic `INSERT ... SELECT` ([`store.go:102-116`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L102-L116)) — no SELECT-then-INSERT race; a foreign/unknown/mismatched-kind category yields `created=false` → 400.
- Summary gates **every** requested `category_id` against `user_id` + `kind='expense'` before querying ([`summary.go:98-121`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L98-L121)); any foreign/unknown/income id → `ok=false` → 400.
- Subcategory parent is gated twice: handler lists the caller's own groups ([`categories/handler.go:113-128`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L113-L128)) and the store re-checks `id=$1 AND user_id=$2` ([`store.go:136`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store.go#L136)).
- **No transaction-ID endpoint exists** (no GET-by-id / update / delete), so the transaction IDOR surface is only the owner-scoped list/summary — which is present.

**Non-findings (checked, not leaks):**
- `JOIN categories` / `LEFT JOIN categories p` in transaction and summary queries do not re-assert `c.user_id`, but the driving row set is already owner-filtered and the join target is an owned category. No cross-owner rows can enter.
- Summary `total` and budget sums run on `s.db` outside the read-only tx ([`summary.go:92`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L92) vs `:164,202,208`). This is a *snapshot-consistency* nuance, not an isolation hole — owner scoping still applies. Out of Phase 1 scope (belongs to #2/#3).
- `GET /api/categories` performs `EnsureSeeded` — a write on a read route ([`categories/handler.go:54`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L54)); seed inserts are owner-scoped ([`seed.go:100,119`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/seed.go#L100)). Not isolation-relevant, but note it when asserting "GET writes nothing".

**Test-plan guidance verdict for #1:** accurate. "Challenge *being logged in implies ownership is checked*" is exactly right — the code proves ownership separately. "Avoid sharing one fixture owner across cases" is well-founded (categories are per-`user_id`; `EnsureSeeded` only seeds when that owner's count is 0, [`seed.go:80-91`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/seed.go#L80-L91)). "Assert it mutates nothing, not just that a request succeeded" matches the DB-side-effect assertion the plan requires.

**Existing coverage / gap:**
- Store-level isolation exists: [`transactions/store_test.go:176-224`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store_test.go#L176-L224), [`categories/store_test.go`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store_test.go) (cross-user `List`, foreign-parent reject), [`categories/handler_test.go:174-194`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler_test.go#L174-L194).
- **Gap:** no HTTP-level test drives `transactions`/`categories`/`summary` through `RequireAccount` with **two real sessions** and asserts the response *and* the unchanged row counts. [`auth/handler_test.go:175-206`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/handler_test.go#L175-L206) does the two-cookie pattern, but only for `/api/auth/me`.

### Risk #4 — Invalid input accepted

**Validation is layered and server-side:** handler checks → store guards (`transactions/store.go:94-100`, `categories/store.go:107-109,131-133`) → DB constraints (`migrations/0003_categories.sql`, `0004_transactions.sql:5-8`). Error shape is always `{"error":"<value>"}` via `writeJSON`. Exact 400 branches:

| Endpoint | 400 conditions (location) |
|---|---|
| POST /api/transactions | bad JSON / body > 4KB ([`handler.go:93-97`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L93-L97)); empty `amount` or `category_id` ([:105-108](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L105-L108)); `kind` not empty/expense/income ([:109-112](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L109-L112)); unparseable `occurred_on` ([:113-116](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L113-L116)); store `created==false` ([:124-127](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L124-L127)) — covers amount format (`^\d{1,10}(\.\d{1,2})?$`, all-zero rejected, [`store.go:30,61-71`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L61-L71)), UUID shape, foreign/income/unknown category |
| GET /api/transactions | `kind` not `all\|expense\|income` ([`handler.go:144-151`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L144-L151)) |
| GET /api/summary | missing `from`/`to` ([`handler.go:179-182`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L179-L182)); store `ok==false` ([:192-195](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L192-L195)) — bad date, `from>to`, bad UUID, foreign/unknown/income category ([`summary.go:58-82,112-120`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/summary.go#L58-L120)) |
| POST /api/categories | bad JSON / >4KB ([`handler.go:78-82`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L78-L82)); bad `kind` / empty `name` / name > 80 runes ([:83-88](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L83-L88)); parent not owned or kind mismatch ([:125-128](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L125-L128)) |

**Corrections / nuances the test plan should not gloss over** (see "Corrections" below): malformed `page`/`limit` are *not* 400 (silently defaulted), and over-long `description` is *not* 400 (silently truncated to 500 runes).

**Anti-pattern note:** `categories/handler.go:97,131` classify store errors with `strings.Contains(err.Error(), "invalid input")` — the tests must assert on the **HTTP status + generic `invalid_request` body**, never on internal error text (matches the test-plan anti-pattern).

**Existing coverage / gap:** validation matrices already exist at handler level ([`transactions/handler_test.go:81-106`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler_test.go#L81-L106), [`categories/handler_test.go`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler_test.go)). **Gap:** none of them assert the *no-write side-effect* after a 400 (rows unchanged), which is the specific "writes nothing" claim Phase 1 must prove.

### Risk #6 — Protected route served unauthenticated / session drop

**Sole gate:** `account.RequireAccount` ([`middleware.go:18-41`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/account/middleware.go#L18-L41)) — nil/typed-nil resolver → 401 fail-closed; nil `next` → 500; unresolved/invalid ID → 401; success → context + `Cache-Control: no-store`.

**Protected vs open (from `routes()`, [`spa.go:20-38`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa.go#L20-L38)):**
- Protected: `GET /api/auth/me`, `GET|POST /api/categories`, `GET|POST /api/transactions`, `GET /api/summary`.
- Open: `GET /healthz`, `POST /api/auth/{register,login,logout}` (register/login rate-limited), `/api` + `/api/` → 404 JSON, `/` → SPA.

**Failure response is uniform and leaks nothing:** 401, `Content-Type: application/json`, `Cache-Control: no-store`, body `{"error":"unauthorized"}`, **no `Set-Cookie`**, no `WWW-Authenticate`, no identity ([`middleware.go:57-70`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/account/middleware.go#L57-L70)). No-data-leak is already asserted at unit level ([`middleware_test.go:83-99`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/account/middleware_test.go#L83-L99)).

**Session lifecycle:** 30-day sliding window ([`session.go:17-20`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/session.go#L17-L20)); refresh happens **only** in `Handler.Me` ([`handler.go:158-162`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/handler.go#L158-L162)); live-session filter `expires_at > now()` ([`store.go:112-118`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/store.go#L112-L118)); expired request → 401 **without clearing the stale cookie** (only `Logout` clears, [`handler.go:224-235`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/handler.go#L224-L235)); a resolver DB error collapses to 401 (indistinguishable from logged-out, [`resolver.go:30-33`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/resolver.go#L30-L33)).

**Counterexample to "login page renders ⇒ auth works" (grounding for the "must challenge"):** in degraded mode (`DATABASE_URL` unset) the handlers stay nil ([`main.go:73-83`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/main.go#L73-L83)), **no protected route is registered**, auth endpoints fall to `/api/` → 404, yet `/` still serves the login SPA ([`spa.go:38,83-92`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa.go#L83-L92)).

**SPA fallback cannot leak account data:** `/api` and `/api/` are more specific mux patterns and always win over `/`; the SPA handler only serves files under `staticDir` + `index.html` ([`spa.go:57-93`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa.go#L57-L93)).

**Existing coverage / gap:** 401 is well covered at unit level ([`account/middleware_test.go:30-137`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/account/middleware_test.go#L30-L137)) and handler-direct ([`transactions/handler_test.go:228-254`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler_test.go#L228-L254), `categories/handler_test.go:68-83`). **Gap:** no test builds `routes()` with a non-nil `resolver` and asserts 401 **per protected route through the mux**; the only route-level test passes nil auth/handlers ([`spa_test.go:24`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa_test.go#L24)), so an accidentally unwrapped protected registration would not be caught. Also, the existing unauth handler tests are *accidentally* DB-gated because they call `openTestDB` first.

### Test harness & cheapest useful layer

**Conventions (verified):**
- No `httptest.NewServer` anywhere — every test uses `httptest.NewRequest` + `httptest.NewRecorder` and calls the handler method directly.
- Owner is attached in tests with the package-local helper `authedRequest(...)` → `req.WithContext(account.WithAccountID(...))` ([`transactions/handler_test.go:14-24`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler_test.go#L14-L24), categories equivalent).
- Test fixtures live **per package** (no shared `testutil`): `openTestDB` ([`transactions/store_test.go:15-33`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store_test.go#L15-L33)) migrates and `t.Cleanup(db.Close)`; `createTestUser` ([`store_test.go:38-53`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store_test.go#L38-L53)) inserts a distinct `users` row + cleanup; `createCategory` ([`store_test.go:55-65`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store_test.go#L55-L65)).
- Stores are **concrete and hard-wired to `*sql.DB`** — `type Store struct { db *sql.DB }`, `NewStore(db *sql.DB)` ([`transactions/store.go:50-57`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L50-L57), [`categories/store.go:34-41`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/store.go#L34-L41)). No interface, so no fake without a refactor — and a fake would not exercise the SQL ownership gate anyway.
- DB tests **skip** without `DATABASE_URL` ([`transactions/store_test.go:18-20`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store_test.go#L18-L20), `:29-31`) or on unreachable DB. Local `go test ./...`: 22 pass / 32 skip / 0 fail (with `CGO_ENABLED=0` on this Mac).
- **CI already provisions Postgres**: `go-test` job with `postgres:16` service + `DATABASE_URL` + `go test ./...` ([`.github/workflows/fly.yml:64-92`](https://github.com/js-s/nest-cache/blob/f65dd59/.github/workflows/fly.yml#L64-L92)).
- No `testify`/`sqlmock` direct dependency (module `github.com/user/nest-cash`, [`go.mod:1`](https://github.com/js-s/nest-cache/blob/f65dd59/go.mod#L1)).

**Recommended layer per risk:**

| Risk | Layer | Why |
|---|---|---|
| #6 | **DB-free HTTP test** | Handlers read the account from context before the store ([`transactions/handler.go:87-91`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L87-L91), [`categories/handler.go:49-53`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/categories/handler.go#L49-L53)), so `NewHandler(nil)` 401s without a DB. **Route-wiring variant also DB-free:** in `package main`, build `application{resolver: &auth.Resolver{}, categories: NewHandler(nil), transactions: NewHandler(nil)}` → `routes()` registers the protected patterns; `&auth.Resolver{}` has a nil store so `ResolveAccountID` fails closed → 401 per protected route. No DB needed to prove no protected route is unwrapped. |
| #1, #4 | **Handler-level over the real store + Postgres**, attach owner via `account.WithAccountID`, then assert status + body **and** DB side-effect (re-read via store or `SELECT count(*)`). | The ownership gate and the no-write guarantee are both SQL-level; a fake would not test them. This matches existing store/handler tests and the test-plan's own "assert side-effects" mandate. |

Optionally add **one** true cookie-path case (`register` two owners → `RequireAccount(resolver, handler)` per [`auth/handler_test.go:175-206`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/handler_test.go#L175-L206)) to prove the real session→owner chain, since `WithAccountID` injection skips the resolver.

**Practical gotchas to bake into the plan:**
- `Limit` wraps register/login ([`spa.go:22-23`](https://github.com/js-s/nest-cache/blob/f65dd59/cmd/nest-cash/spa.go#L22-L23)); `clientIP` keys on `Fly-Client-IP` then `RemoteAddr` ([`ratelimit.go:61-70`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/auth/ratelimit.go#L61-L70)). Uniform-IP auth bursts can 429 and mask assertions — vary the client IP header or avoid >5 rapid auth calls.
- Run DB tests with `CGO_ENABLED=0` on macOS (AGENTS.md; confirmed here — plain `go test` aborts with `dyld: missing LC_UUID`).
- Lesson from `lessons.md`: do not assert wrapped DB error strings; assert status + generic body.

## Corrections to test-plan §2 (post-research backport candidates)

1. **Risk #4 wording "malformed filter" is imprecise.** Malformed/pathological `page`/`limit` are **silently defaulted**, not 400 — [`transactions/handler.go:215-227`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/handler.go#L215-L227) treats unparseable/out-of-range as absent and clamps `limit` to 100. The 400 filter cases are: bad `kind`, missing `from`/`to`, unparseable date, `from>to`, foreign/unknown/income `category_id`. Suggested response-guidance edit: replace "malformed filter" with "semantically invalid filter (bad `kind`, bad/`from>to` dates, foreign/unknown/income category)" and add a note that lenient `page`/`limit` is intended (test the *documented* behavior, don't invent a 400).
2. **Risk #4: over-long `description` is silently truncated**, not rejected ([`transactions/store.go:100`](https://github.com/js-s/nest-cache/blob/f65dd59/internal/transactions/store.go#L100)) — do not assert a 400 for it.

No risk in Phase 1 is speculative and no hot-spot citation is misleading — the §2 Source column ("PRD privacy NFR, roadmap F-01, interview Q1") correctly points at a real, code-enforced boundary.

## Code References

- `internal/account/middleware.go:18-70` — `RequireAccount`, fail-closed 401, `no-store`, error bodies
- `internal/account/identity.go:11-35` — `AccountID`, `WithAccountID`, `AccountIDFromContext`
- `internal/auth/resolver.go:22-39` — session cookie → AccountID, fail-closed
- `internal/auth/store.go:112-116` — live-session lookup (`expires_at > now()`)
- `internal/auth/session.go:14-37` — cookie name, 30d window, token hashing
- `internal/transactions/store.go:94-116` — atomic ownership+kind insert gate
- `internal/transactions/summary.go:98-121` — every category_id ownership/kind gate
- `internal/categories/store.go:61,112-114,136` — owner-scoped list/insert/parent
- `internal/transactions/handler.go:82-211` — create/list/summary validation branches
- `internal/categories/handler.go:44-143` — list/create/subcategory validation branches
- `cmd/nest-cash/spa.go:13-41` — route table with `RequireAccount` wiring
- `cmd/nest-cash/main.go:73-83` — degraded mode (auth handlers nil when no DB)
- `.github/workflows/fly.yml:64-92` — CI Postgres so isolation tests are not vacuous

## Architecture Insights

- **Two separate concerns, deliberately decoupled:** authentication (`RequireAccount`) and ownership (SQL `user_id` predicates + atomic resource-ID gates). This is why "logged in implies ownership checked" is false and worth challenging — the code proves ownership independently, and a test that only asserts a 200 proves nothing about isolation.
- **Fail-closed default:** nil resolver → 401 (not open); resolver DB error → 401; unknown/expired token → 401. All map to one uniform body with no side channel.
- **Convention over framework:** plain `net/http` mux (Go 1.22 method patterns), stdlib `testing` + `httptest`, no mocking library, no test util package. New tests should mirror the local per-package helpers.
- **Validation owned server-side and layered** (handler → store → DB CHECK); client-side is not trusted. Error routing by string-match (`"invalid input"`) is brittle but stable; tests should pin status + generic body, not text.
- **Degraded mode is the sharpest auth story:** a rendered login page coexists with 404 auth APIs — a test targeting only the SPA/UI would falsely pass.

## Historical Context (from prior changes)

- `context/archive/2026-09-10-account-data-boundary/plan.md:57,180` — F-01: resolver is the sole trusted source; two owners over real records is mandatory for later slices. `reviews/impl-review.md:30` added the typed-nil guard; `:50` added `no-store`.
- `context/archive/2026-09-11-account-registration-and-login/plan.md:39,44,45,119` — opaque 30d sliding cookie, HttpOnly/Lax, uniform 401, 409 duplicate, 400 validation; `reviews/impl-review.md:42-49` warns DB-skip guards make green CI vacuous (→ CI Postgres).
- `context/archive/2026-09-12-predefined-and-custom-categories/plan.md:103,118-119,178` — isolation in SQL; 400/401/409 matrix; foreign parent rejected.
- `context/archive/2026-09-12-expense-entry-and-operation-list/plan.md:59,126,217-218` — ownership+kind validated atomically; `ok=false` → 400, never success; store tests cover isolation + bad amount/foreign category.
- `context/archive/2026-09-12-income-and-budget-ratio/plan.md:72,82,151` — kind validation, handler 400/401, summary isolation + `from>to`/bad UUID → `ok=false`.
- `context/changes/filtered-expense-summary/plan.md:39-40,56,64,71` — `from<=to` → 400, foreign/nonexistent category → 400 (not silent 0); A/B isolation tests.
- No `research.md` exists anywhere under `context/` yet; prior decisions live in each change's `plan.md` + `reviews/impl-review.md`.

## Related Research

- None yet — this is the first `research.md` in the repository.

## Open Questions

- **Route-wiring test scope:** should Phase 1 include the DB-free `application{...}.routes()` test that asserts 401 per protected route (closes the "unwrapped registration" gap), or rely on handler-level 401s? The former costs almost nothing since it needs no DB.
- **Cookie-path coverage:** is one real register→cookie→`RequireAccount` cross-account case (for `transactions`/`categories`, not just `me`) required, or is `account.WithAccountID` injection sufficient for Phase 1? Injection skips the resolver, so it does not prove the session chain.
- **Lenient `page`/`limit`:** treat as intended behavior to document (assert default), or flag as a validation gap to fix in a later phase? Research found no evidence of intent in the foundation docs.
- **Expired-session cookie not cleared:** assert 401-only, or also assert the absence of `Set-Cookie`? Current behavior replays the stale cookie until `Logout`.
