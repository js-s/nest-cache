# Edycja i usuwanie transakcji — Plan Brief

> Pełny plan: `context/changes/transaction-edit-and-delete/plan.md`

## What & Why

Domykamy FR-008 (roadmapa S-07): użytkownik może edytować i usuwać własne operacje. To pierwsza operacja modyfikująca istniejące dane finansowe, więc musi respektować izolację konta (mutacja wyłącznie własnego wpisu) i nie może cicho zmienić cudzych ani historycznych danych. Domyka brakujące Update i Delete w CRUD.

## Starting Point

Działa pełny pion odczytu i zapisu: auth z długą sesją, izolacja konta (`account.RequireAccount`), kategorie z podkategoriami, dodawanie wydatków/przychodów oraz filtrowane podsumowanie (S-01–S-06). Pakiet `transactions` ma `Create`/`List`/`Summary` i brak `Update`/`Delete`. `Operations.vue` ma read-only tabelę operacji, a `client.ts` tylko `createTransaction`/`listTransactions`. Nie istnieje żadna trasa z wildcardem `{id}`.

## Desired End State

W `/operations` każdy wiersz ma Edit i Delete. Edit zamienia komórki wiersza w pola (kategoria zawężona do typu wiersza, kwota, data, opis) z Save/Cancel; Delete pokazuje w wierszu potwierdzenie Yes/No. Po mutacji lista i podsumowanie pokazują nowe dane. Typ operacji nie da się zmienić. Obcy lub nieistniejący `id` zwraca 404 i nie zmienia niczyich danych.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Zakres | Edit + delete | FR-008 i outcome S-07 mówią o obu; domyka CRUD | Plan |
| Kontrakt update | `PUT /api/transactions/{id}` (pełna zamiana edytowalnych pól) | Najprostsza walidacja — prawie ten sam kontrakt co `createInput` | Plan |
| Zmienność typu | `kind` niezmienny | Eliminuje przekrojową rewalidację kategorii i reset dropdownu | Plan |
| Obcy/nieznany id | 404 | Nie ujawnia istnienia cudzego zasobu (guardrail prywatności) | Plan |
| Brak znalezienia vs złe wejście | Probe po `ErrNoRows` (404 vs 400) | Jedno zapytanie w szczęśliwej ścieżce, poprawne kody | Plan |
| Miejsce edycji | Inline wiersza w `Operations.vue` | Zostaje w kontekście listy, brak nowego ekranu/modal-a | Plan |
| Potwierdzenie usunięcia | Custom inline (Yes/No) | Spójny wygląd z motywem aplikacji | Plan |
| Kontrolki na Summary | Tylko Operations | Summary pozostaje read-only projekcją; odświeża się przy pobraniu | Plan |
| Testy | Store + handler + izolacja (jak istniejące) | Dowodzi bramki właściciela — głównego ryzyka slice-a | Plan |
| Migracja | Brak | Istniejąca tabela `transactions` pokrywa UPDATE/DELETE | Plan |

## Scope

**In scope:** `Store.Update`/`Store.Delete` + `ErrNotFound`, `Handler.Update`/`Handler.Delete`, trasy `PUT`/`DELETE /api/transactions/{id}`, funkcje klienta, inline edit i custom confirm w `Operations.vue`, testy store/handler/izolacji.

**Out of scope:** zmiana typu operacji, kontrolki na Summary, soft-delete/Undo/kosz, historia i audyt, kolumna wersji, bulk edit/delete, edycja kategorii, nowe zależności, migracje.

## Architecture / Approach

```
PUT  /api/transactions/{id}  -> RequireAccount -> Handler.Update -> Store.Update
DELETE /api/transactions/{id} -> RequireAccount -> Handler.Delete -> Store.Delete
                                      |
                            Operations.vue (inline edit / inline confirm)
                                      |
                            client.ts (updateTransaction / deleteTransaction)
```

Mutacje bramkują `user_id` w tym samym zapytaniu SQL, które zmienia wiersz (wzorzec CTE `INSERT ... SELECT` z `Create`). Update zachowuje istniejący `kind` i wymaga kategorii tego samego typu; po `ErrNoRows` tani probe rozstrzyga 404 (brak wiersza) vs 400 (zła kategoria). DELETE zwraca 204 bez body.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend mutacje | Update/Delete store+handler+trasy, testy | Bramka właściciela — mutacja cudzego wiersza; rozróżnienie 404/400 |
| 2. Frontend | Klient API, inline edit, custom confirm, build | Stan inline edit (jeden wiersz naraz) i zawężenie kategorii do typu |

**Prerequisites:** S-04 ukończony (dodawanie transakcji działa); dostęp do lokalnej bazy dla testów store/integracji.
**Estimated effort:** ~2 sesje, 2 fazy.

## Open Risks & Assumptions

- Brak kolumny wersji → last-write-wins; zakładamy single-user, więc konflikt jest nieistotny.
- Hard-delete trwale usuwa wiersz — zgodne z retencją PRD (dopóki użytkownik nie usunie).
- Probe po `ErrNoRows` nie jest w transakcji z UPDATE; przy równoległym usunięciu możliwy rzadki misclassification 400/404. Akceptowalne przy single-user; `ponytail:` komentarz w kodzie.
- Frontend nie ma runnera testów — bramką jest `npm --prefix web run build` plus ręczna weryfikacja.

## Success Criteria (Summary)

- Użytkownik edytuje i usuwa własne operacje, a zmiany są widoczne na liście i w podsumowaniu.
- Obce i nieistniejące `id` dają 404 i nie zmieniają żadnych danych (izolacja konta).
- `go test ./...` i `npm --prefix web run build` przechodzą.
