# Income and Budget Ratio — Plan Brief

> Full plan: `context/changes/income-and-budget-ratio/plan.md`

## What & Why

Budujemy S-06: zapis przychodu w tym samym formularzu co wydatek (toggle) oraz pasywny wskaźnik `wydatki/przychody` z progiem 80% na stronie podsumowania. Domyka to FR-006 i Business Logic z PRD bez dokładania powiadomień poza zakresem.

## Starting Point

`/transactions` pisze tylko wydatki (`kind` na sztywno w `store.go`), `/summary` agreguje tylko wydatki, UI nie zna przychodów. Kolumna `kind` i kategorie przychodów już istnieją — brakuje tylko ścieżki zapisu, agregacji ratio i badge w UI.

## Desired End State

User przełącza Wydatek/Przychód, zapisuje przychód i widzi go na liście z kolumną Rodzaj. Podsumowanie pokazuje ratio z całego okresu z badge: zielony `<80%`, żółty `80–99%` + „Zbliżasz się do limitu", czerwony `>=100%`; przy braku przychodów `—` + hint.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
|---|---|---|
| Formularz przychodu | Jeden formularz z toggle | Najmniejszy diff, reuse walidacji zamiast duplikacji widoku. |
| API kind | Pole `kind` w POST, default `expense` | Jeden endpoint pasuje do istniejącej kolumny `kind`. |
| Liczenie ratio | SQL sumy + Go `math/big`, stringi | Zero rozjazdów groszy, spójne z regułą S-05. |
| Zero przychodów | `null` → `—` + hint | Brak dzielenia przez zero i mylącego `0%`. |
| Sygnał 80% | Badge 3 stany | Pasywny element wizualny z PRD, zero powiadomień. |
| Zakres ratio | Cały okres, ignoruje filtr kategorii | Budżet to całość vs całość, nie 1 kategoria. |
| Listy | Oba kind z kolumną i filtrem | Przychód musi być widoczny po zapisie. |

## Scope

**In scope:** `Create`/`List` z `kind`, `Summary` + `budget {expense_total, income_total, ratio_pct}`, toggle + Rodzaj + filtr w Operations, karta ratio z badge w Summary, testy Go + build frontu.

**Out of scope:** Konfigurowalny próg, powiadomienia push/email, edycja/usuwanie, wykresy, migracje DB.

## Architecture / Approach

Rozszerzamy pakiet `transactions` (store + handler, te same 3 route'y), jeden `GET /api/summary` zwraca widok filtrowany (S-05 compat) i globalne `budget` — frontend bez drugiego requestu. Backend najpierw, potem Vue na wzór `Operations.vue`/`Categories.vue`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
|---|---|---|
| 1. API przychodów i ratio | `kind` w Create/List, `budget` w Summary, testy Go | Zmiana sygnatur `Create`/`List` sypie istniejące testy seedujące |
| 2. UI przychodów i wskaźnika | Toggle, Rodzaj, karta ratio z badge | Toggle musi czyścić kategorię (ID z innego kind) |

**Prerequisites:** S-05 dowiezione (plan `filtered-expense-summary` zielony); lokalny PG pod `DATABASE_URL` do testów.
**Estimated effort:** ~2 sesje w 2 fazach (backend z testami + frontend z weryfikacją manualną).

## Open Risks & Assumptions

- Sygnatury `Create`/`List` zmieniają wołające testy — trzeba je zaktualizować w tej samej fazie.
- Format `ratio_pct` z 1 decimal — badge czyta progi z parsowanego floatu tylko do kolorów, wyświetla string.
- S-05 w statusie `impl_reviewed` (nie zarchiwizowane) — zakładam jego API jako stabilne.

## Success Criteria (Summary)

- Przychód zapisany toggle'm widać na `/operations` i w filtrach `kind`.
- `/summary` pokazuje ratio z badge 80%/100% i `—` przy zerowych przychodach.
- Izolacja konta trzyma; odpowiedzi `< 2s`; `go test ./...` i `npm run build` zielone.
