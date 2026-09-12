<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Wprowadzenie wydatku i lista operacji

- **Plan**: context/changes/expense-entry-and-operation-list/plan.md
- **Scope**: All phases (1–3 of 3, all Progress checkboxes [x])
- **Date**: 2026-09-12
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 1 warning, 5 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | WARNING |

## Verification

- `go build ./...` → PASS
- `npm --prefix web run build` → PASS (also re-verified after triage fixes)
- `CGO_ENABLED=0 go test ./...` → PASS (transactions DB-backed subtests SKIP — no DATABASE_URL locally, no local Postgres; plan's DB criteria taken from recorded SHAs)
- Migration idempotence / curl / browser flows → not re-runnable here; diff shows evidence for each manual item (form, reload-first-page, loadMore/hasMore, localStorage key, user_id gating, empty-state link)
- Prior phase-1 review (`reviews/impl-review-phase-1.md`, APPROVED) already triaged; its F4/F5 (FK without ON DELETE, kind cross-check) intentionally skipped — not re-reported

## Findings

### F1 — List LEFT JOIN scans nullable category name

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/transactions/store.go:140
- **Detail**: List used `LEFT JOIN categories c` but scanned `c.name` into a plain string (`rows.Scan(&t.CategoryName, ...)`); a NULL would surface as a 500 scan error. Create path already uses inner JOIN; FK blocks orphans today, so latent only.
- **Fix**: Inner `JOIN categories c` in List (keep parent `LEFT JOIN` — parent genuinely optional), matching the Create path.
- **Decision**: FIXED

### F2 — Amount input type drift (text vs planned number)

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: web/src/views/Operations.vue:183
- **Detail**: Plan said `<input type="number" min="0.01" step="0.01">`; built `type="text" inputmode="decimal"` with comma→dot normalization + client regex. Deliberate superset: avoids native-validation block, handles comma decimals.
- **Fix**: Accept as plan addendum, no code change — intent preserved, behavior strictly broader.
- **Decision**: ACCEPTED (addendum, no code change)

### F3 — Overlong descriptions silently truncated

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/transactions/store.go:87
- **Detail**: Store truncates descriptions over 500 runes while sibling categories store rejects overlong names. Frontend `maxlength=500` covers the normal path; only raw API clients hit the divergence.
- **Fix**: Keep truncation — reject would add a failure mode for paste-overshoot.
- **Decision**: ACCEPTED (no change)

### F4 — Generic 400 message hides bad-date failures

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/views/Operations.vue:59
- **Detail**: `message()` mapped every 400 to 'Check the amount and category.', but empty/malformed `occurred_on` also yields 400 — date errors misled.
- **Fix**: Widen to 'Check the amount, category, and date.'
- **Decision**: FIXED

### F5 — Store accepts any limit; page unclamped

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/transactions/store.go:122-127
- **Detail**: `limit` clamped to 100 only in the handler; store accepts any limit, page unclamped (huge OFFSET possible). COUNT + SELECT non-atomic (total can drift under concurrent insert). Single-user scale — harmless.
- **Fix**: No change — handler is the single entry point and already clamps; revisit if the store gains a second caller.
- **Decision**: ACCEPTED (no change)

### F6 — 401 handling diverges from Categories view

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: web/src/views/Operations.vue:63-70
- **Detail**: Operations redirects to `/login` on 401; Categories renders a generic error. Operations behavior is better; already marked `ponytail` (no global interceptor yet).
- **Fix**: No change — shared 401 helper when the third view lands, per the ponytail comment.
- **Decision**: ACCEPTED (no change)

## Triage Summary

```
Fixed:    F1, F4               (2)
Accepted: F2, F3, F5, F6       (4)
Skipped:  —                    (0)
```
