<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Transaction edit and delete

- **Plan**: context/changes/transaction-edit-and-delete/plan.md
- **Scope**: Phase 1-2 of 2
- **Date**: 2026-09-13
- **Verdict**: APPROVED
- **Findings**: 0 critical 2 warnings 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | WARNING |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Automated verification (re-run during review)

- `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestUpdate|TestDelete'` → PASS (pre-rename names; post-rename compile verified via `-run 'TestTransactionsHandler|TestTransactionsUpdate|TestTransactionsDelete'`, SKIP without DATABASE_URL like before)
- `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestHandler'` → PASS
- `CGO_ENABLED=0 go test ./internal/transactions/ -run Isolation` → SKIP (DATABASE_URL unset; plan records pass at 38873a4)
- `CGO_ENABLED=0 go test ./... && go build ./...` → PASS
- `gofmt -l internal/transactions cmd/nest-cash` → empty → PASS
- `npm --prefix web run build` → PASS (also after F3/F5 fixes)

## Findings

### F1 — Unplanned web/src/style.css layout change

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Scope Discipline
- **Location**: web/src/style.css:27,272-274
- **Detail**: Plan listed no style.css change. Commit c5d9b6d widens --content-width 720px→960px and adds .table-wrap overflow-x. Supports the new Actions column, no functional creep, no NOT-doing violation. Benign but unrecorded scope.
- **Fix**: Add plan addendum line for style.css width/wrap.
- **Decision**: FIXED (addendum appended to Phase 2 contract in plan.md)

### F2 — New test names lack Transactions prefix

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/transactions/store_test.go:321,364,414; handler_test.go:295,345,389
- **Detail**: lessons.md requires domain-prefixed mirrored test names (TestTransactions…/TestCategories…). New TestUpdateTransaction, TestUpdateRejectsInvalidInput, TestDeleteTransaction, TestHandlerUpdate(+Validation/Delete) lacked the prefix. No collision today; weaker `go test ./...` attribution.
- **Fix**: Rename to TestTransactionsUpdate, TestTransactionsUpdateRejectsInvalidInput, TestTransactionsDelete, TestTransactionsHandlerUpdate, TestTransactionsHandlerUpdateValidation, TestTransactionsHandlerDelete.
- **Decision**: FIXED (all 6 renamed; gofmt clean; test binary compiles)

### F3 — Client path ids not encoded

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/api/client.ts:128,135
- **Detail**: updateTransaction/deleteTransaction interpolated `/transactions/${id}` raw. Server rejects non-UUID (400), ids originate server-side — exploitability ~nil. First path-param call in client; rest uses URLSearchParams.
- **Fix**: Wrap with encodeURIComponent(id) in both functions.
- **Decision**: FIXED (`npm --prefix web run build` passes)

### F4 — Isolation test skips without DATABASE_URL

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: internal/transactions/isolation_integration_test.go:87
- **Detail**: Local re-run SKIPs TestTransactionsCrossAccountIsolationThroughSessions (DATABASE_URL unset). Plan marks 1.3 [x] at 38873a4; code path exists (serveTx helper + PUT/DELETE 404 asserts). Env-only gap, not a code fault.
- **Fix**: None; re-run 1.3 with DATABASE_URL set if strict proof wanted.
- **Decision**: SKIPPED

### F5 — Minor frontend row-state roughness

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/views/Operations.vue:44,205-224,240-251
- **Detail**: Three cosmetic items under single-action UI: (a) rowBusy single string slot — concurrent saves clobber; (b) saveEdit lacked empty-date guard (server 400 covered with generic message); (c) remove() errors went to distant listError. TOCTOU probe in store.go already disclosed via ponytail comment; single-user scope.
- **Fix**: (a) busyRows Set<string> with .has(t.id) bindings; (b) 'Select a date.' guard; (c) row-scoped deleteError shown in confirming row, cleared on askDelete/No.
- **Decision**: FIXED (`npm --prefix web run build` passes)
