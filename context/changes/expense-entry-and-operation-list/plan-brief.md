# Wprowadzenie wydatku i lista operacji — Plan Brief

> Pełny plan: `context/changes/expense-entry-and-operation-list/plan.md`

## What & Why

Budujemy pierwszy pełny pion finansowy NestCash (roadmapa S-04): zapis wydatku przez formularz webowy i paginowana lista własnych operacji. Motywacja z US-01/FR-005/FR-007: bez działającego przepływu zapisu nie da się zbudować podsumowania (S-05), a uciążliwe wprowadzanie danych to według PRD jedna z dwóch przyczyn porzucania budżetu.

## Starting Point

Działa auth z długą sesją (S-01), izolacja konta (`account.RequireAccount`) i kategorie z podkategoriami kopiowanymi z `resources/categories.yaml` (S-03). Baza ma `users`, `sessions`, `categories`; brak jakiejkolwiek tabeli i kodu transakcji. Wzorce store/handler/ekranu są ustalone w pakiecie `categories` i widoku `Categories.vue`.

## Desired End State

Na `/operations` użytkownik wybiera kategorię, wpisuje kwotę i opis, zapisuje — wydatek natychmiast pojawia się na liście od najnowszych, z „Wczytaj więcej”. Po odświeżeniu dane zostają, domyślna kategoria jest podpowiedziana, a inne konto nie widzi operacji. Kwoty liczą się dokładnie (bez float).

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Reprezentacja kwoty | `NUMERIC(12,2)`, string w JSON | Dokładne sumy w SQL dla S-05/S-06 bez pułapek float | Plan |
| Waluta | Jedna (PLN), bez kolumny | PRD nie przewiduje multi-currency; pole byłoby spekulacją | Plan |
| Poziom kategorii | Dowolny węzeł typu `expense` | Nie blokuje własnych grup bez dzieci, prosta reguła | Plan |
| Zakres listy | Schemat ma `kind`, lista zwraca wszystkie | S-06 dołoży przychód bez zmiany schematu i kontraktu | Plan |
| Paginacja | Infinite scroll na offsetach `page`/`limit`/`total` | Najprostszy kontrakt dla „Wczytaj więcej” bez kursora | Plan |
| Data | `DATE`, domyślne „dziś” z przeglądarki | Data kalendarzowa, brak pułapek stref czasowych, natywny input | Plan |
| Układ ekranu | Jedna strona: formularz + lista | Zapisany wydatek pojawia się od razu (US-01) | Plan |
| Domyślna kategoria | `localStorage` (klient) | Spełnia FR-005 bez migracji i zmian w API | Plan |

## Scope

**In scope:** migracja `transactions`; store z atomicznym zapisem i paginowanym odczytem; `POST/GET /api/transactions`; ekran `/operations` (formularz + infinite lista); domyślna kategoria z `localStorage`; testy Go + ręczna weryfikacja.

**Out of scope:** edycja/usuwanie operacji, przychody i wskaźnik budżetowy, filtrowanie listy, wiele walut, godzina transakcji/strefy, testy E2E.

## Architecture / Approach

Pion przez trzy warstwy, wzorzec skopiowany z S-03: `0004_transactions.sql` → `internal/transactions` (store, potem handler) → trasy w `spa.go` za `account.RequireAccount` → `web/src/views/Operations.vue` + funkcje w `client.ts`. Walidacja właściciela i typu kategorii dzieje się atomowo w jednym `INSERT ... SELECT` z tabeli `categories`, więc obce dane nigdy nie wejdą. Kwota wędruje jako string od formularza do Postgresa.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Baza i store | Migracja + zapis/odczyt z walidacją i izolacją | Błąd walidacji kwoty/kategorii podważy późniejsze sumy |
| 2. API i trasy | `POST`/`GET /api/transactions` za sesją | Regresja kontraktu kwoty (string vs liczba) |
| 3. Frontend | Ekran `/operations` z formularzem i infinite listą | Sortowanie przy dopisywaniu — rozwiązane przeładowaniem 1. strony |

**Prerequisites:** lokalny Postgres z `DATABASE_URL` do testów integracyjnych; działające S-01/S-03.
**Estimated effort:** ~1 sesja na fazę, 3 fazy.

## Open Risks & Assumptions

- Brak `DATABASE_URL` w CI/local sprawia, że testy store się pomijają — ręczna weryfikacja na bazie jest wymagana.
- Frontend nie ma runnera testów; gate to wyłącznie `npm --prefix web run build` + testy ręczne.
- Infinite scroll bez kursora zakłada, że kolejne strony są stabilne między żądaniami (single-user — spełnione).

## Success Criteria (Summary)

- Użytkownik zapisuje wydatek i widzi go natychmiast na liście, a po odświeżeniu dane zostają.
- Kwoty są dokładne i widoczne z kategorią (grupa → podkategoria).
- Operacje jednego konta są niewidoczne dla innego.
