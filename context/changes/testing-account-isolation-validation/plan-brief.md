# Account Isolation & Validation Hardening Tests — Plan Brief

> Full plan: `context/changes/testing-account-isolation-validation/plan.md`
> Research: `context/changes/testing-account-isolation-validation/research.md`

## What & Why

Rollout Phase 1 of the project test plan: add HTTP-level integration tests that prove one account cannot read or write another's data (#1), invalid input is rejected with 400 and writes nothing (#4), and every protected route rejects unauthenticated requests without leaking (#6). The phase exists because isolation is enforced in SQL (not by the login middleware), so "a request succeeded" proves nothing about ownership.

## Starting Point

Isolation already works in code — `user_id` predicates on every category/transaction/summary query, atomic ownership+kind gates on `category_id`/`parent_id`, and a fail-closed `RequireAccount` wrapper on every protected route. What's missing is proof: existing 401 tests bypass the mux, cross-account tests stop at `/api/auth/me`, and no test asserts that a rejected write left the database unchanged.

## Desired End State

`go test ./...` fails if an account can reach another's rows, if a 400 path writes a row, or if a protected route is served unauthenticated. The test plan's §6 cookbook points at the new reference tests, §3 Phase 1 is `complete`, and §2 risk #4 wording matches actual behavior.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Scope | Tests only; no production fixes | Matches Phase 1's "prove protection" mandate and keeps the diff low-risk | Plan |
| #6 approach | DB-free `routes()` wiring test | Catches an unwrapped route and runs without Postgres (zero-value resolver fails closed) | Research + Plan |
| Session coverage | One real cookie-chain isolation case | Proves the session→AccountID→owner path, not just context injection | Research + Plan |
| Test location | New focused files per package | Clear separation, reuses existing package-local helpers | Plan |
| 400 assertions | Status + generic `invalid_request` body | Pins the public contract without asserting internal error strings | Research + Plan |
| Validation gaps | Document, don't fix (lenient `page`/`limit`, truncated `description`) | Research found behavior is lenient by design; fixing is out of a test phase | Research |
| Doc updates | §6 + §3 + §2 corrections | Keeps the test plan authoritative and truthful | Plan |

## Scope

**In scope:** route-wiring 401 guard; cross-account read/write isolation through real sessions; validation 400 + no-write matrix; documented lenient `page`/`limit`; test-plan reconciliation.

**Out of scope:** production code or migration changes; frontend test runner; fixing the two validation gaps; cookie-flag details already covered; a shared test util package.

## Architecture / Approach

Three test layers: (1) DB-free `routes()` test in `package main` proving every protected route is gated; (2) DB-backed isolation tests per package that build two real sessions via `auth.Store` and assert both response and unchanged row counts; (3) DB-backed validation tests asserting 400 + generic error code + zero writes. A docs phase updates `test-plan.md`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Route-wiring auth guard | DB-free 401 test per protected route | Test imperfectly models real wiring; mitigated by an open-route control case |
| 2. Cross-account isolation | B blocked from A's data, A's rows unchanged | Cookie-chain setup complexity; avoid the per-IP rate limiter |
| 3. Validation + no-write | 400 + no-write matrix, valid accepted | Re-using the response-only matrix without the count assertions |
| 4. Test-plan reconciliation | §6/§3/§2 updated | §2 backport must not add file anchors |

**Prerequisites:** local or CI Postgres for Phases 2–3; `CGO_ENABLED=0` on macOS.
**Estimated effort:** ~2 sessions across 4 phases.

## Open Risks & Assumptions

- The DB-backed phases skip without `DATABASE_URL`, so local runs are green-but-vacuous; CI's `postgres:16` service is the real gate.
- The DB-free #6 test relies on `RequireAccount` short-circuiting before nil-store handlers — if handler ordering changes, the test needs revisiting.
- Deliberate-break checks are manual; if skipped, the tests could pass for the wrong reason.

## Success Criteria (Summary)

- The new suite fails when an ownership predicate, an auth wrapper, or a validation guard is removed.
- Every protected route returns a non-leaking 401 without a database.
- `test-plan.md` §3/§6/§2 reflect the shipped tests and actual handler behavior.
