# Lessons Learned

> Append-only register of recurring rules and patterns. Re-read at start by /10x-frame, /10x-research, /10x-plan, /10x-plan-review, /10x-implement, /10x-impl-review.

## Wrap DB errors with operation context

- **Context**: `internal/auth/store.go` — store methods over `database/sql`.
- **Problem**: Bare `return err` at DB boundaries; logs show driver errors with no table/op hint, while sibling `migrate.go` wraps with `%w` context.
- **Rule**: Wrap every DB-boundary error as `fmt.Errorf("<pkg>: <op>: %w", err)`.
- **Applies to**: All Go store/repository code.
