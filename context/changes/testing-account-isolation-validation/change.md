---
change_id: testing-account-isolation-validation
title: Account isolation and validation hardening tests
status: implementing
created: 2026-09-13
updated: 2026-09-13
archived_at: null
---

## Notes

Open a change folder for rollout Phase 1 of context/foundation/test-plan.md: "Account isolation & validation hardening".
Risks covered: #1 (one account reads/writes another's data), #4 (invalid input accepted), #6 (protected route served unauthenticated / session drop).
Test types planned: integration (HTTP).
Risk response intent:
- #1: prove Account A's request referencing Account B's resource returns no B data and mutates nothing; challenge "being logged in implies ownership is checked"; avoid asserting only that a request succeeded, and avoid sharing one fixture owner across cases.
- #4: prove bad amount/category/filter is rejected with 400 and writes nothing, while valid input is accepted; challenge "client-side validation is enough"; avoid happy-path-only and asserting internal error strings.
- #6: prove an unauthenticated request to a protected route is rejected and leaks nothing; challenge "the login page rendering proves auth works"; avoid only testing the login form UI.
