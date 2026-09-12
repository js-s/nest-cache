<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Account registration and login

- **Plan**: context/changes/account-registration-and-login/plan.md
- **Scope**: Phase 1 of 3
- **Date**: 2026-09-11
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 4 warnings, 6 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | WARNING |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Findings

### F1 — Migration check-then-insert races on parallel boot

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: internal/db/migrate.go:35-60
- **Detail**: Per-file SELECT-miss then DDL+INSERT is not atomic across processes. Two boots racing both pass the miss; both DDLs succeed (idempotent IF NOT EXISTS) but the second INSERT into schema_migrations hits PK conflict → log.Fatalf in main. Single-machine Fly autostop makes this unlikely, but two machines booting at once hit it.
- **Fix A ⭐ Recommended**: Record with INSERT ... ON CONFLICT (filename) DO NOTHING instead of plain INSERT.
  - Strength: One-line change, keeps idempotency without locks; matches the ON CONFLICT pattern already used in store CreateUser.
  - Tradeoff: Second boot silently skips already-applied files — correct here since DDL is idempotent, but masks partial-apply edge (mitigated: DDL+record stay in one tx).
  - Confidence: HIGH — same conflict-tolerant idiom as store.go CreateUser.
  - Blind spot: Haven't tested dual-boot against real PG.
- **Fix B**: Wrap the file loop in pg_advisory_xact_lock.
  - Strength: True mutual exclusion, textbook for multi-instance migrate-at-boot.
  - Tradeoff: More code + lock-key convention to document; overkill for single-machine scale.
  - Confidence: MEDIUM — standard approach but heavier than the problem warrants now.
  - Blind spot: Lock key collision review not done.
- **Decision**: FIXED via Fix A

### F2 — Case-insensitive email uniqueness enforced app-side only

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: internal/db/migrations/0001_auth.sql:3
- **Detail**: Column is TEXT UNIQUE; Foo@x.com + foo@x.com are distinct rows at DB level. Only Store.NormalizeEmail prevents doubles. Any future writer bypassing the store (seed, admin SQL, Phase 2+ code path) can split one login into two accounts. Plan mentioned citext variant; implementation chose TEXT + app normalize.
- **Fix A ⭐ Recommended**: Add CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email)) in a 0002 migration (or fold into 0001 pre-first-user).
  - Strength: Invariant lives in the database, survives all writers; no extension needed (unlike CITEXT).
  - Tradeoff: Needs a second migration file or 0001 edit before any prod user exists; index build trivial at this scale.
  - Confidence: HIGH — plain Postgres, zero app-code change.
  - Blind spot: Whether prod Neon already has rows (believed empty — verify before choosing fold-vs-0002).
- **Fix B**: Keep app-only, add a test asserting NormalizeEmail on every store entry path.
  - Strength: Zero schema churn.
  - Tradeoff: Convention-based safety; rots as writers multiply.
  - Confidence: LOW — relies on discipline, the exact failure mode flagged.
  - Blind spot: Future writers unknown.
- **Decision**: FIXED via Fix A (0002 migration)

### F3 — Bare DB errors without operation context

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/auth/store.go:55,70,85,96,114,125,131,138
- **Detail**: All store DB-boundary errors return bare (return err), while sibling migrate.go wraps with %w migrate-context. Logs will show pq errors with no table/op hint.
- **Fix**: Wrap per method as fmt.Errorf("auth: <op>: %w", err), mirroring migrate.go.
- **Decision**: FIXED + ACCEPTED-AS-RULE: Wrap DB errors with operation context

### F4 — Test cleanup orphaned rows on mid-test failure

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/store_test.go:99-101
- **Detail**: DELETE FROM users cleanup runs only on the full-success path; any Fatalf at lines 45-97 leaves user + session rows in the real PG used for testing.
- **Fix**: Register t.Cleanup deleting the test user right after CreateUser; drop the trailing delete.
- **Decision**: FIXED

### F5 — TouchSession ships with zero coverage

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: internal/auth/store_test.go:64-95
- **Detail**: Sliding-expiry path (store.go:120) never exercised; siblings cover every exported branch. Live-touch (expiry moves forward) and expired-touch (stays unresolvable) cases missing.
- **Fix**: Add both cases to the store test using the existing live/expired fixtures.
- **Decision**: FIXED

### F6 — TouchSession silent no-op undocumented

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/store.go:120-126
- **Detail**: TouchSession on expired/unknown hash returns nil with 0 rows — indistinguishable from success. DeleteSession documents its no-op; TouchSession does not.
- **Fix**: One-line doc comment stating the no-op, or return (bool, error) if Phase 2 callers need to know.
- **Decision**: FIXED (doc comment)

### F7 — No empty-input guards before SQL

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/store.go:43,91
- **Detail**: CreateUser(ctx,"","") and CreateSession(ctx,"",...) hit the DB (empty-string row / FK error) instead of failing fast at the trust boundary.
- **Fix**: Early error return on empty email/passwordHash and tokenHash/userID.
- **Decision**: FIXED (guards on CreateSession/TouchSession; CreateUser/Find validation lands in Phase 2 handlers)

### F8 — Migration runner executes any embedded non-SQL file

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/db/migrate.go:29
- **Detail**: File filter is !IsDir() with no .sql suffix check; a stray embedded file under migrations/ would be Exec'd as SQL.
- **Fix**: Skip names without .sql suffix.
- **Decision**: FIXED

### F9 — FindUserByID surfaces UUID syntax errors as errors

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/auth/store.go:76-80
- **Detail**: Malformed UUID string returns a PG syntax error, not ok=false. Correct fail-closed behavior, but Phase 2 callers must map it to 500, not 404.
- **Fix**: One-line doc comment stating malformed id → error.
- **Decision**: FIXED (doc comment)

### F10 — Monolithic TestStore vs table-driven siblings

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: internal/auth/store_test.go:36
- **Detail**: Single ~65-line TestStore aborting on first Fatalf hides later failures; siblings (middleware_test, identity_test) use focused TestX-per-behavior style. Skip-guard deviation (real PG vs httptest) is justified and kept.
- **Fix**: Split into TestCreateUser_FindUser, TestSessionLifecycle, TestDeleteExpiredSessions sharing openTestDB.
- **Decision**: FIXED (TestCreateUserFindUser, TestSessionLifecycle, TestDeleteExpiredSessions)

## Notes

- Benign plan drifts (no action): schema path internal/db/migrations/ (go:embed forbids ..), TEXT+normalize instead of CITEXT, Migrate(ctx,db) without injected FS, FindUserByID/TouchSession as untested Phase 2 foreshadow (see F5).
- Automated verification: gofmt clean, go vet clean, CGO_ENABLED=0 go test ./... green (TestStore SKIP without DATABASE_URL by design; PASS on real PG16 during implementation). Manual 1.4-1.6 confirmed by human.
- Local toolchain note: plain go test/go run abort on macOS dyld (missing LC_UUID) with go1.22.2; CGO_ENABLED=0 works around it. Pre-existing, hits untouched packages too.
