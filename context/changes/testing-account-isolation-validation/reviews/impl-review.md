<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Account Isolation & Validation Hardening Tests

- **Plan**: context/changes/testing-account-isolation-validation/plan.md
- **Scope**: Phases 1–4 of 4 (all completed)
- **Date**: 2026-09-13
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 1 warning, 2 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Findings

### F1 — `defer db.Close()` races `t.Cleanup` row deletes in categories tests

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/categories/isolation_integration_test.go:60, internal/categories/validation_integration_test.go:14
- **Detail**: `defer db.Close()` runs when the test function returns, before `t.Cleanup` callbacks. The `DELETE FROM users` cleanups (registered in `sessionOwner`/`createTestUser`, errors ignored via `_, _ =`) then hit a closed pool, fail silently, and rows leak into the scratch DB. Same pre-existing pattern in `internal/categories/store_test.go:63,93,129`. Sibling `internal/transactions/store_test.go:26` does it right via `t.Cleanup(func() { _ = db.Close() })` registered before row-delete cleanups (LIFO runs deletes first). Assertions still pass (all queries scoped by `user_id`), so this is hygiene/flakiness-proofing, not a correctness bug.
- **Fix**: In the two new categories test files, replace `defer db.Close()` with `t.Cleanup(func() { _ = db.Close() })` immediately after `openTestDB(t)`.
- **Decision**: FIXED + ACCEPTED-AS-RULE (lesson: "Register DB close via t.Cleanup, not defer")

### F2 — Duplicate test names across packages blur `go test ./...` output

- **Severity**: 🔭 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/transactions/isolation_integration_test.go, internal/categories/isolation_integration_test.go
- **Detail**: Both packages declare `TestCrossAccountIsolationThroughSessions`; both validation files declare `TestValidationRejectsNothingWritten`-style names. Legal Go, but failures in a full-suite run are ambiguous about which package broke.
- **Fix**: Prefix with the domain, e.g. `TestTransactionsCrossAccountIsolation` / `TestCategoriesCrossAccountIsolation`.
- **Decision**: FIXED + ACCEPTED-AS-RULE (lesson: "Prefix test names with the domain when packages mirror each other")

### F3 — Off-topic `fmt.Errorf` line under test-plan §6.1

- **Severity**: 🔭 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: context/foundation/test-plan.md:136
- **Detail**: The "Command:" bullet under "6.1 Adding a Go unit test" carries a DB-error-wrapping command that belongs to `lessons.md`, not to unit-test instructions. Copy-paste from the cookbook fill-in.
- **Fix**: Drop the line (the rule already lives in `context/foundation/lessons.md`).
- **Decision**: FIXED

## Evidence

- Drift: all 5 planned test files MATCH contract (categories isolation has one benign EXTRA: B-own-create 201 case); `test-plan.md` §6.2/.3 refs, §3 `complete`, §2 #4 wording all MATCH; no `file:line` anchors added.
- Scope: `git diff b23816f^..HEAD --name-only` shows only `*_test.go`, `test-plan.md`, change-folder files — zero production-code changes.
- Automated: `CGO_ENABLED=0 go test ./cmd/nest-cash/... -count=1` → ok; `gofmt -l` clean; `go vet` on cmd + both packages clean; no `TBD — see §3 Phase 1` remains in §6.1–.3. DB-backed suites skip without `DATABASE_URL` locally; CI passes recorded in Progress (e48606b, 5c9a568).
- Manual: deliberate-break checks recorded per phase in Progress with SHAs; Phase 4 read-through user-confirmed before the p4 commit.
