# Lessons Learned

> Append-only register of recurring rules and patterns. Re-read at start by /10x-frame, /10x-research, /10x-plan, /10x-plan-review, /10x-implement, /10x-impl-review.

## Wrap DB errors with operation context

- **Context**: `internal/auth/store.go` — store methods over `database/sql`.
- **Problem**: Bare `return err` at DB boundaries; logs show driver errors with no table/op hint, while sibling `migrate.go` wraps with `%w` context.
- **Rule**: Wrap every DB-boundary error as `fmt.Errorf("<pkg>: <op>: %w", err)`.
- **Applies to**: All Go store/repository code.

## Register DB close via t.Cleanup, not defer

- **Context**: Integration tests using `openTestDB` plus `t.Cleanup` row deletes (e.g. `internal/categories/*_integration_test.go`).
- **Problem**: `defer db.Close()` runs when the test function returns, i.e. BEFORE `t.Cleanup` callbacks; cleanup `DELETE`s then hit a closed pool, the error is dropped (`_, _ =`), and rows leak into the scratch DB.
- **Rule**: Register pool close with `t.Cleanup(func() { _ = db.Close() })` immediately after `openTestDB(t)` — LIFO then runs row deletes first, pool close last.
- **Applies to**: All Go DB-backed tests.

## Prefix test names with the domain when packages mirror each other

- **Context**: `internal/transactions/` and `internal/categories/` with mirrored integration tests.
- **Problem**: Identical test names (`TestCrossAccountIsolationThroughSessions`, `TestValidationRejectsNothingWritten`) in both packages; a failure in `go test ./...` does not say which package broke.
- **Rule**: Prefix mirrored test names with the domain (`TestTransactions…` / `TestCategories…`).
- **Applies to**: All Go tests in mirrored packages.
