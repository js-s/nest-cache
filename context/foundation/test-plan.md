# Test Plan

> Phased test rollout for this project. Strategy is frozen at the top
> (§1–§5); cookbook patterns at the bottom (§6) fill in as phases ship.
> Read before writing any new test.
>
> Refresh: re-run `/10x-test-plan --refresh` when stale (see §8).
>
> Last updated: 2026-09-13

## 1. Strategy

Tests follow three non-negotiable principles for this project:

1. **Cost × signal.** The cheapest test that gives a real signal for the
   risk wins. Do not promote to e2e because e2e "feels safer." Do not put a
   vision model on top of a deterministic diff that already catches the
   regression.
2. **User concerns are first-class evidence.** Risks anchored in "the
   team is worried about X, and the failure would surface somewhere in
   <area>>" carry the same weight as PRD lines or hot-spot data.
3. **Risks are scenarios, not code locations.** This plan documents *what
   could fail* and *why we believe it's likely* — drawn from documents,
   interview, and codebase *signal* (churn, structure, test base). It does
   NOT claim to know which line owns the failure. That knowledge is
   produced by `/10x-research` during each rollout phase. If the plan and
   research disagree about where the failure lives, research is the
   ground truth.

Hot-spot scope used for likelihood weighting: `cmd/`, `internal/`, `web/src/`.

## 2. Risk Map

The top failure scenarios this project must protect against, ordered by
risk = impact × likelihood. Risks are failure scenarios in user / business
terms, not test names. The Source column cites the *evidence that surfaced
this risk* — never a specific file as "where the failure lives" (that is
research's job, see §1 principle #3).

| # | Risk (failure scenario) | Impact | Likelihood | Source (evidence — not anchor) |
|---|-------------------------|--------|------------|--------------------------------|
| 1 | One account reads or writes another account's transactions or categories through the API | High | Medium | PRD privacy NFR L83, Access Control L105; roadmap F-01 L67; interview Q1 |
| 2 | Combined period+category summary returns the wrong subset or totals — a real expense is missing from the table | High | High | US-01 AC L49–51, FR-009 L78; roadmap S-05 risk L133; hot-spot `web/src` (43 commits/30d) + `internal/transactions` (19) |
| 3 | Budget ratio or the 80% passive signal is miscalculated or shown at the wrong time | Medium | Medium | PRD Business Logic L89–97; roadmap S-06 L145 |
| 4 | Invalid input is accepted (amount ≤ 0, nonexistent category, semantically invalid filter) → corrupt data, wrong summary, or a 500 | High | Medium | PRD validation L101; roadmap S-04 risk L120; abuse lens (untrusted input) |
| 5 | Backend JSON contract drifts from the Vue client and a flow breaks silently at runtime | Medium | Medium | test-base profile (frontend `none`); hot-spot `web/src/api/client.ts` (5); tech-stack-web.md |
| 6 | A protected route is served unauthenticated, or a long-lived session drops unexpectedly | High | Low | FR-002 L59; privacy NFR L38; hot-spot `internal/auth` (15) |
| 7 | A saved transaction does not persist and is lost after re-login | Medium | Low | PRD Secondary success criterion L34; retention NFR L85 |

Abuse / security lens: Risk #1 covers authorization / IDOR (does the
endpoint verify ownership, not just authentication?). Risk #4 covers
untrusted input and server-side validation parity, including semantically invalid
filter parameters (bad `kind`, bad/`from>to` dates, foreign/unknown/income category).
`page`/`limit` are lenient by design (malformed values fall back to defaults, not a 400)
and over-long `description` is truncated, not rejected. Both are scored on the same impact × likelihood axes.

### Risk Response Guidance

| Risk | What would prove protection | Must challenge | Context `/10x-research` must ground | Likely cheapest layer | Anti-pattern to avoid |
|------|-----------------------------|----------------|--------------------------------------|-----------------------|-----------------------|
| #1 | Account A's request referencing Account B's resource returns no B data and mutates nothing | "Being logged in implies ownership is checked" | Where owner identity comes from; how resource ids resolve; whether isolation is enforced at handler or store | integration (HTTP) | Asserting only that a request succeeded; sharing one fixture owner across cases |
| #2 | A known set of transactions appears — and only those — under combined period+category filters, with matching totals | "An empty result means no matching data"; "dates filter inclusively because the code says so" | Date-boundary inclusivity from requirements; timezone of the date filter; pagination interaction with filters | integration / store-level | Oracle lifted from the SQL under test; happy-path-only |
| #3 | Ratio = expenses/income for a chosen period; ≥ 80% shows the indicator, < 80% does not | "The ratio is right because the code computes it that way" | Rounding and precision; zero-income division; threshold boundary exactly at 80% | unit / integration | Assertion copied from the production formula; ignoring divide-by-zero |
| #4 | Bad amount / category / semantically invalid filter (bad `kind`, bad/`from>to` dates, foreign/unknown/income category) is rejected with 400 and writes nothing; valid input is accepted. `page`/`limit` are lenient by design and over-long `description` is truncated, neither a 400 | "Client-side validation is enough" | Server-side validation as source of truth; existing error mapping; category-must-exist rule | integration (HTTP) | Testing only the happy path; asserting internal error strings |
| #5 | A real user flow works against the actual backend response shape | "Build passing means the contract is fine" | Exact JSON field names and shape the client consumes; API base URL wiring | contract + one flow | Snapshotting markup; testing framework internals |
| #6 | An unauthenticated request to a protected route is rejected and leaks nothing | "The login page rendering proves auth works" | Which routes sit behind the middleware; the session cookie contract | integration (HTTP) | Only testing the login form UI |
| #7 | Two sessions of the same account both see a transaction created in the first | "An in-memory test store proves persistence" | Whether tests use the real store or a mock; migration/connection setup | integration against DB | Passing against a mock store that hides persistence bugs |

## 3. Phased Rollout

Each row is a discrete rollout phase that will open its own change folder
via `/10x-new`. Status moves left-to-right through the values below; the
orchestrator updates Status as artifacts appear on disk.

| # | Phase name | Goal (one line) | Risks covered | Test types | Status | Change folder |
|---|------------|-----------------|----------------|------------|--------|---------------|
| 1 | Account isolation & validation hardening | Prove each account only reaches its own data, and that bad input cannot write | #1, #4, #6 | integration (HTTP) | complete | context/changes/testing-account-isolation-validation/ |
| 2 | Summary & ratio correctness | Prove the filtered summary returns the right subset/totals and the 80% signal is honest | #2, #3, #7 | integration + store-level | not started | — |
| 3 | Frontend test bootstrap & contract | Bootstrap a frontend runner, pin the API contract the client consumes, cover one critical flow | #5 | contract + flow | not started | — |
| 4 | Quality-gates wiring | Lock the floor: run Go and frontend suites on PR/CI | floor for all | gates | not started | — |

**Status vocabulary:** `not started` → `change opened` → `researched` →
`planned` → `implementing` → `complete`.

## 4. Stack

The classic test base for this project. AI-native tools (if any) carry a
`checked:` date so future readers can see which lines need re-verification.

| Layer | Tool | Version | Notes |
|-------|------|---------|-------|
| Go unit + integration | `go test ./...` (stdlib `testing`) | Go 1.22.2 | 15 `*_test.go` files across `internal/*` and `cmd/`; the current gate |
| HTTP handler tests | `net/http/httptest` (stdlib) | Go 1.22.2 | existing handler tests already use it |
| Frontend unit/component | none yet — see §3 Phase 3 | — | no vitest/jest/playwright config; zero test files under `web/src` |
| API contract | none yet — see §3 Phase 3 | — | client/API boundary is unverified |
| e2e | none — not planned | — | backend integration covers the flows; revisit only if a browser-only failure mode appears |
| CI gates | GitHub Actions (`fly.yml`) | — | currently builds/deploys; wire test gates in §3 Phase 4 |

Frontend build (`npm --prefix web run build`) is today's only frontend
validation gate; it type-checks but does not test behavior.

**Stack grounding tools (current session):**
- Docs: none — no Context7 or framework docs MCP available in current session; checked: 2026-09-13
- Search: web search tool available — not used; local manifests/configs were sufficient; checked: 2026-09-13
- Runtime/browser: none — no Playwright/browser MCP; not used (e2e not planned); checked: 2026-09-13
- Provider/platform: GitLab and Kubernetes MCPs exposed, but repo is GitHub-hosted and deploys to Fly.io — not relevant; checked: 2026-09-13

## 5. Quality Gates

The full set of gates that must pass before a change reaches production.
"Required for §3 Phase <N>" means the gate is enforced once that rollout
phase lands; before that, the gate is `planned`.

| Gate | Where | Required? | Catches |
|------|-------|-----------|---------|
| `gofmt` clean | local | required | formatting drift |
| `go build ./...` | local + CI | required | compile errors |
| `go test ./...` | local + CI | required | backend logic regressions |
| Frontend build + typecheck | local + CI | required | frontend type/compile drift |
| Frontend tests | local + CI | required after §3 Phase 3 | frontend behavior + contract drift |
| Backend integration on PR | CI on PR | required after §3 Phase 1 | broken critical API paths |
| Test gates fail the pipeline | CI on PR | required after §3 Phase 4 | regressions reaching main |

## 6. Cookbook Patterns

How to add new tests in this project. Each sub-section is filled in once
the relevant rollout phase ships; before that, the sub-section reads
"TBD — see §3 Phase <N>."

### 6.1 Adding a Go unit test

- **Location**: `*_test.go` beside the package under test (e.g. `internal/transactions/`).
- **Naming**: `<file>_test.go`, `TestXxx` functions.
- **Reference test**: `internal/transactions/summary_test.go`.
- **Run locally**: `go test ./...` (on macOS, prefix `CGO_ENABLED=0` if the `dyld` quirk appears).

### 6.2 Adding a Go HTTP integration test

- **Pattern**: drive the handler through `net/http/httptest`, assert status → body shape AND side-effects (writes/no-writes), plus the unauthenticated and cross-account cases.
- **Reference tests**: `cmd/nest-cash/routes_auth_test.go` (DB-free 401 wiring for every protected route), `internal/transactions/isolation_integration_test.go` + `internal/categories/isolation_integration_test.go` (cross-account read/write isolation through the real session chain), `internal/transactions/validation_integration_test.go` + `internal/categories/validation_integration_test.go` (400 + no-write matrix, valid accepted).
- **Run locally**: `CGO_ENABLED=0 go test ./...` (DB-free tests always run); `CGO_ENABLED=0 DATABASE_URL=<test db> go test ./...` for the DB-backed isolation/validation tests (they skip without `DATABASE_URL`).

### 6.3 Adding a test for a new API endpoint

- **Test type**: integration (preferred) through the real handler and store.
- **Pattern**: assert request → response shape AND side-effects. Cover 400 (bad input), 401 (no session), and cross-account rejection — not just the happy path.
- **Reference tests**: `cmd/nest-cash/routes_auth_test.go` for the 401 wiring guard; `internal/transactions/isolation_integration_test.go` / `internal/transactions/validation_integration_test.go` (and the `internal/categories/` analogs) for ownership and validation coverage.
- **When to add e2e instead**: only if the failure requires the deployed browser + cookie + handler crossing; not currently planned.

### 6.4 Adding a frontend test

- TBD — see §3 Phase 3.

### 6.5 Adding an API contract test

- TBD — see §3 Phase 3.

### 6.6 Per-rollout-phase notes

(Optional. After each phase lands, `/10x-implement` appends a 2–3 line note
here capturing anything surprising the rollout phase taught.)

## 7. What We Deliberately Don't Test

Exclusions agreed during the rollout (Phase 2 interview, Q5). Future
contributors should respect these unless the underlying assumption changes.

- **Trivial / UI-markup tests** — snapshot or render tests for static
  markup, styling, and router wiring. Re-evaluate if a layout regression
  escapes review repeatedly. (Source: Phase 2 interview Q5.)
- **Vue component internals in isolation** — test real flows and the API
  contract, not every component's internals. Re-evaluate if a component's
  logic becomes non-trivial. (Source: Phase 2 interview Q5.)
- **Go stdlib / DB driver internals** — `pgx`, `net/http`, and the mux are
  the dependencies' responsibility, not ours. (Source: Phase 2 interview Q5.)

## 8. Freshness Ledger

- Strategy (§1–§5) last reviewed: 2026-09-13
- Stack versions last verified: 2026-09-13
- AI-native tool references last verified: 2026-09-13

Refresh (`/10x-test-plan --refresh`) when:

- a new top-3 risk surfaces from the roadmap or archive,
- a recommended tool's `checked:` date is older than three months,
- the project's tech stack changes (new framework, new test runner),
- §7 negative-space no longer matches what the team believes.
