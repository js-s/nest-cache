<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Filtered Expense Summary

- **Plan**: context/changes/filtered-expense-summary/plan.md
- **Scope**: Phase 1–2 of 2 (full)
- **Date**: 2026-09-12
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 1 warning, 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

Notes: Scope Discipline PASS — `App.vue` + `style.css` nav to świadoma adaptacja na zgłoszenie użytkownika (brak możliwości opuszczenia widoku), nie creep. Success Criteria PASS — automaty zielone (`go test ./...`, `npm run build`); manual 1.5/1.6 pokryty testem UI użytkownika + testami 400/401 (odnotowane w commicie p1), 2.3–2.6 testowane na żywo przez użytkownika.

## Findings

### F1 — Zły ostatni dzień miesiąca (strefa czasowa)

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/src/views/Summary.vue:56
- **Detail**: `lastDayOfMonth` liczył `new Date(y, m, 0).toISOString().slice(0, 10)`. Konstruktor w strefie lokalnej + serializacja UTC cofa dzień w CET (wrzesień → 09-29). Wydatki z ostatniego dnia miesiąca wypadały z sum.
- **Fix**: Dzień kalendarzowy bez UTC: `new Date(y, m, 0).getDate()` + składanie `YYYY-MM-DD`.
  - Strength: Jedna funkcja, zero nowych zależności; ten sam kształt co wcześniej.
  - Tradeoff: Daty domyślne (`today`) dzielą antywzorzec z Operations.vue — zostawione (pre-existing, niska stawka).
  - Confidence: HIGH — arytmetyka kalendarzowa niezależna od strefy.
  - Blind spot: None significant.
- **Decision**: FIXED

### F2 — Wiersz sumy po angielsku, plan mówił Razem

- **Severity**: 🔭 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: web/src/views/Summary.vue:171
- **Detail**: Plan mówił o wierszu Razem, kod renderował Altogether. Całe UI jest po angielsku, więc Altogether pasowało do konwencji — nieprecyzyjny plan, nie błąd kodu.
- **Fix**: Zmiana na Razem decyzją użytkownika (jedna linijka).
- **Decision**: FIXED (użytkownik wybrał Razem mimo angielskiego UI)

### F3 — Brak limitu liczby category_id w query

- **Severity**: 🔭 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: internal/transactions/summary.go:45
- **Detail**: Każde UUID to osobny placeholder w `IN (...)`; URL ~8kB to ~200 UUID. Przy osobistej skali nierealne, cap to jedna linijka (`len(ids) > 100 → ok=false`).
- **Fix**: Cap 100 odrzucony przez użytkownika — osobista skala.
- **Decision**: SKIPPED

### F4 — Zdublowany helper labeli kategorii

- **Severity**: 🔭 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: web/src/views/Summary.vue:34
- **Detail**: `categoryLabel` + `itemLabel` robiły to samo na różnych typach; Operations.vue ma jeden helper.
- **Fix**: Scalone w jeden `categoryLabel` na `{ category_name, parent_name }`.
- **Decision**: FIXED
