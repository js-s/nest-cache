# Review fixes — follow-ups

From `reviews/impl-review.md` (2026-09-12).

## F10 — single resolve + conditional refresh on /me (deferred)

- **What remains**: `/me` still runs `FindUserByID` after the resolver already
  loaded the user, and `TouchSession` UPDATEs on every GET.
- **Why deferred**: collapsing both into one resolve needs the auth resolver to
  surface the `profile`/session `expires_at` to the handler. The F-01
  `internal/account.Resolver` contract (`ResolveAccountID(r) (AccountID, bool)`)
  cannot carry that today; options are an optional richer interface in
  `internal/account`, or an auth-owned `RequireAccount` replacement — both
  change the locked boundary and deserve their own decision.
- **When**: S-02, when more authed routes make the per-request cost visible.
- **Also**: consider a refresh threshold (touch only when remaining lifetime is
  below half the window) alongside the above.
