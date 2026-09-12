<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Spójna stylizacja interfejsu webowego

- **Plan**: context/changes/web-ui-styling/plan.md
- **Scope**: Phases 1–3 of 3
- **Date**: 2026-09-12
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical 3 warnings 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------
| Plan Adherence | WARNING |
| Scope Discipline | WARNING |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Verification

- `npm --prefix web run build` → PASS (37 modules, ~149ms, vite v8.2.2).
- Dead refs (`hero|counter|#next-steps|#social|hero.png|vite.svg|vue.svg` in `web/src`) → PASS, no matches.
- Plan "NOT doing" respected (no new deps, no auth/routing/API/backend/copy changes).
- Manual items 1.3, 1.4, 2.2–2.5, 3.2–3.4 all `- [ ]` — pending, not rubber-stamped.

## Findings

### F1 — toggleTheme crashes when storage blocked

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/theme.ts:24
- **Detail**: `localStorage.setItem` unguarded in `toggleTheme()`. Throws (SecurityError/QuotaExceededError) in private mode or denied storage → unhandled exception on click. Inconsistent: inline script in `web/index.html:9-12` wraps same key in try/catch.
- **Fix**: Wrap in try/catch, still update DOM + ref.
- **Decision**: FIXED

### F2 — light-dark() with no fallback breaks old browsers

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/style.css:4-16
- **Detail**: All theme vars use `light-dark()` with no `@supports` fallback. Pre-Chrome 111 / Safari 16.4 / FF 120 keep token stream in custom property → `background: var(--color-bg)` resolves invalid. PRD cites NFR cross-browser; roadmap S-02 claims it.
- **Fix**: Add static light fallback block under `@supports not (background: light-dark(#fff, #000))`.
- **Decision**: FIXED

### F3 — missing mobile breakpoint (~640px)

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: web/src/style.css (plan.md:47)
- **Detail**: Plan requires one mobile breakpoint (~640px) + global `box-sizing`. Box-sizing present; no `@media`/breakpoint found in `style.css`. Fluid `.page` + `.auth` may cover it — unproven until manual 380px check (1.4/2.5 pending).
- **Fix**: Add single `@media (max-width: 640px)` tightening `.page` padding + `.card` padding, or record manual 380px PASS as evidence none needed.
- **Decision**: FIXED

### F4 — router-view not wrapped in main.page

- **Severity**: 🔎 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: web/src/App.vue:33
- **Detail**: Plan contract: `<router-view />` into `<main class="page">`. Actual: bare `<router-view />`. Harmless — Login/Register carry `main.page.auth`, Summary carries `main.page.app-main.stack` — but future views without own wrapper lose centering.
- **Fix**: Wrap in `<main class="page">` and drop duplicate `.page` from views, or accept current shape (works today).
- **Decision**: FIXED (shell holds single main.page; views use div.auth / div.app-main.stack — valid HTML, no nested main)

### F5 — roadmap + categories seed sit outside web/ scope

- **Severity**: 🔎 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Scope Discipline
- **Location**: context/foundation/roadmap.md, resources/categories.yaml
- **Detail**: Working tree holds `roadmap.md` edit (adds S-02 web-ui-styling, renumbers S-03..S-06) + untracked `resources/categories.yaml` (future S-03 seed, 789B). Related bookkeeping, not code creep — but plan Changes Required is web/ only. Land roadmap separately from styling impl; confirm categories seed belongs to S-03.
- **Fix**: Commit roadmap independently; move categories seed to S-03 or keep with note.
- **Decision**: ACCEPTED (roadmap bookkeeping stays; categories seed to be claimed by S-03 — no code edit, commit left to user)

### F6 — matchMedia bare at import + svh-only

- **Severity**: 🔎 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/theme.ts:8,28; web/src/style.css:47
- **Detail**: `window.matchMedia` at module scope, no existence guard, separate calls, `addEventListener` only (Safari <14 needs `addListener`), side effect on import. `min-height: 100svh` without `100vh` fallback line. Vite SPA — low blast radius, cheap to harden.
- **Fix**: Guard `typeof window`, cache one MediaQueryList, add `addListener` fallback; prepend `min-height: 100vh`.
- **Decision**: FIXED
