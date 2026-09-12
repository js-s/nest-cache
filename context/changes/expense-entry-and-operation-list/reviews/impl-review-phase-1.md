<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Wprowadzenie wydatku i lista operacji

- **Plan**: context/changes/expense-entry-and-operation-list/plan.md
- **Scope**: Phase 1 of 3
- **Date**: 2026-09-12
- **Verdict**: APPROVED
- **Findings**: 0 critical, 1 warning, 4 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Findings

### F1 — Test cleanup deletes against a closed DB

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/transactions/store_test.go:46-49,79
- **Detail**: Tests `defer db.Close()`; `t.Cleanup` runs after defers, so the DELETE cleanup hits a closed pool, errors silently, and rows leak.
- **Fix**: In `openTestDB` register `t.Cleanup(func() { db.Close() })` and drop the `defer db.Close()` lines from the tests. LIFO makes row cleanup run before close.
  - Strength: one helper change + mechanical deletes; makes cleanup actually run.
  - Tradeoff: none meaningful.
  - Confidence: HIGH — standard Go cleanup ordering.
  - Blind spot: `internal/categories/store_test.go` has the same latent pattern (pre-existing, out of scope).
- **Decision**: FIXED via Fix A

### F2 — Malformed UUID yields an internal error, not ok=false

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/transactions/store.go:80
- **Detail**: Only empty `categoryID` was rejected pre-query; `"not-a-uuid"` reached Postgres and came back wrapped as a 500-class error, while other bad input returns `ok=false`.
- **Fix**: Reject non-UUID `category_id` in `Create` as `ok=false` (canonical-shape regex), so every user-input failure shares one path.
  - Strength: no 500 leakage; handler stays thin.
  - Tradeoff: adds a tiny validator.
  - Confidence: HIGH — mirrors the amount/date guards already present.
  - Blind spot: categories store still lets bad UUIDs reach the DB (pre-existing).
- **Decision**: FIXED via Fix A

### F3 — Store invalid-input contract diverges from categories

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/transactions/store.go:76
- **Detail**: `transactions.Create` uses `ok=false` for bad input; categories returns an `"invalid input"` error string that its handler string-matches. Phase 2 handler must honor the `ok` flag.
- **Fix**: Keep the `ok`-flag contract and document it in Phase 2's handler contract so `ok=false` maps to 400, not success.
  - Strength: prevents a handler bug.
  - Tradeoff: none.
  - Confidence: HIGH.
  - Blind spot: categories stays inconsistent until later.
- **Decision**: FIXED (documented in plan Phase 2 Contract)

### F4 — `category_id` FK has no ON DELETE

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Data Safety
- **Location**: internal/db/migrations/0004_transactions.sql:4
- **Detail**: FK blocks orphan transactions (safe). Plan Migration Notes explicitly chose NO ACTION because category deletion is out of MVP (FR-004). Latent surprise when delete lands; no index on `category_id` means a future delete would seq-scan.
- **Fix**: No change now — intent documented. Revisit (and add the index) when category delete lands.
- **Decision**: SKIPPED

### F5 — DB does not enforce `transactions.kind == categories.kind`

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Data Safety
- **Location**: internal/db/migrations/0004_transactions.sql:4-5
- **Detail**: `kind` is independently constrained; only `store.Create` gates consistency. Direct writes could mismatch.
- **Fix**: No change now — store is the sole writer. Revisit when S-06 income writes land.
- **Decision**: SKIPPED

## Triage Summary

```
Fixed:     F1, F2                       (2)
Documented: F3                          (1)
Skipped:   F4, F5                       (2)
```
