# Edycja i usuwanie transakcji — Implementation Plan

## Overview

Domykamy FR-008 (roadmapa S-07): użytkownik może edytować i usuwać własne operacje. Dodajemy `PUT /api/transactions/{id}` oraz `DELETE /api/transactions/{id}` — obie mutacje bramkowane właścicielem, z `kind` niezmiennym po utworzeniu — a w `Operations.vue` edycję wiersza w miejscu oraz potwierdzenie usunięcia w wierszu.

## Current State Analysis

- CRUD transakcji jest niekompletny: `internal/transactions/store.go` ma tylko `Create`, `List`, `Summary`. Brak `Update` i `Delete`.
- Każde zapytanie w istniejącym slice gating `user_id`; `Create` używa jednego atomowego `INSERT ... SELECT` bramkującego właściciela i zgodność `kind` kategorii (`internal/transactions/store.go:105`).
- Konwencja store: `(T, ok bool, err error)`, gdzie `ok=false` znaczy „odrzucone wejście” → 400 (np. `store.go:86`). Realny błąd DB jest jedynym `err`.
- Trasy: Go 1.22 `ServeMux` z wzorcami metoda+ścieżka w `cmd/nest-cash/spa.go:31-35`; nie istnieje jeszcze żadna trasa z wildcardem `{id}`.
- Handler: `internal/transactions/handler.go` czyta konto z kontekstu, waliduje shapecia wejścia przed store, mapuje 201/400/401/405/500.
- Frontend: `Operations.vue` ma formularz dodawania i read-only tabelę; `client.ts` ma tylko `createTransaction`/`listTransactions`; `Summary.vue` pokazuje te same wiersze read-only. Frontend nie ma runnera testów — `npm --prefix web run build` jest bramką.
- Brak jakiejkolwiek kolumny wersji/kopii zapasowej na transakcjach — mutacje są hard-delete/hard-update.

### Key Discoveries:

- Brak potrzeby migracji: `0004_transactions.sql` już niesie wszystkie kolumny potrzebne do UPDATE/DELETE.
- Wzorzec CTE z `Create` (`store.go:106-116`) przenosi się wprost na UPDATE zwracający nazwy kategorii do DTO.
- `uuidRe` jest nieeksportowany w pakiecie `transactions` (`store.go:34`), więc handler (ten sam pakiet) może walidować `{id}` przed wywołaniem store.
- `client.ts` `request<T>` już zwraca `undefined` dla 204 (`client.ts:26`) — DELETE nie wymaga zmian w helperze.
- Lekcje: nazwy testów w lustrzanych pakietach prefiksuj domeną (`context/foundation/lessons.md`).

## Desired End State

W `/operations` każdy wiersz ma akcje Edit i Delete. Edit zamienia komórki wiersza w pola formularza (kategoria zawężona do `kind` tego wiersza, kwota, data, opis) z Save/Cancel; Save zapisuje przez API i odświeża stronę 1. Delete pokazuje w wierszu potwierdzenie („Delete? Yes/No”); Yes usuwa przez API i odświeża listę. `kind` nie da się zmienić. Po mutacji podsumowanie pokazuje zaktualizowane sumy przy następnym pobraniu. Obcy lub nieistniejący `id` daje 404 i nie zmienia niczyich danych.

### Weryfikacja końcowa:

- `go test ./...` przechodzi (w tym nowe testy store/handler i rozszerzony test izolacji).
- `npm --prefix web run build` przechodzi (type-check + build).
- Ręcznie: edycja zmienia wartości i sumy w Summary; usunięcie znika z listy i z sum; Cancel i No nie zmieniają danych; brak kontrolki zmiany typu.

## What We're NOT Doing

- Brak zmiany typu (expense↔income) przy edycji — `kind` niezmienny.
- Brak kontrolek edycji/usuwania na `Summary.vue` — pozostaje widokiem read-only, odświeża się przy następnym pobraniu.
- Brak soft-delete, kosza, Undo i endpointu przywracania.
- Brak historii zmian, audytu i kolumny wersji (single-user, last-write-wins).
- Brak bulk edit/delete i akcji wielokrotnego wyboru.
- Brak edycji/usuwania kategorii (nadal FR-004 odłożone do v2).
- Brak nowych zależności frontendowych ani backendowych.
- Brak migracji bazy.

## Implementation Approach

Pionowy przyrost w dwóch fazach: najpierw backend (store → handler → trasy → testy), potem frontend (klient API → inline edit + custom confirm w `Operations.vue`). Backend najpierw, bo jego kontrakt (404 dla obcego/nieznanego, 204 dla DELETE, `kind` niezmienny) determinuje zachowanie UI.

Kluczowa zasada bezpieczeństwa: każda mutacja musi bramkować `user_id` w tym samym zapytaniu SQL, które zmienia wiersz — wzorem `Create`. Obcy `id` nigdy nie może trafić w cudzy wiersz.

## Critical Implementation Details

- **`kind` niezmienny wymusza bramkę zgodności kategorii.** Update nie przyjmuje `kind` w body; nowa kategoria musi należeć do użytkownika i mieć `c.kind = t.kind`. Niezgodna kategoria to 400 (odrzucone wejście), a nie próba zmiany typu.
- **Rozróżnienie 404 i 400 wymaga istnienia wiersza.** Jeden UPDATE … CTE bramkuje zarówno właściciela, jak i kategorię; przy `sql.ErrNoRows` nie wiadomo, czy brakuje wiersza (404), czy kategoria jest zła (400). Dlatego po `ErrNoRows` wykonaj tani probe `SELECT 1 FROM transactions WHERE id=$1 AND user_id=$2`, aby sklasyfikować. Szczęśliwa ścieżka to jedno zapytanie.
- **Konwencja wyników mutacji.** `Store.Update` zwraca `(Transaction, ok bool, err error)`: `ok=false, err=nil` → 400; `errors.Is(err, ErrNotFound)` → 404; inny `err` → 500. `Store.Delete` zwraca `(bool, error)`: malformed `id` → `false, nil` (400); brak/obcy → `false, ErrNotFound` (404); sukces → `true, nil` (204). Handler waliduje kształt `uuidRe` już wcześniej, więc malformed praktycznie nie dojdzie do store.
- **Custom confirm to stan wiersza, nie `window.confirm`.** `confirmingId` decyduje, czy w komórce akcji pokazać „Delete? Yes/No”.
- **DTO Update musi nieść nazwy kategorii.** CTE zwraca wiersz, a zewnętrzny `SELECT` dołącza `c.name` i `p.name` (jak w `Create`), inaczej klient nie ma `category_name`/`parent_name`.

## Phase 1: Backend — mutacje Update i Delete

### Overview

Dodaje `Store.Update`/`Store.Delete` z bramką właściciela, `Handler.Update`/`Handler.Delete` z mapowaniem kodów oraz dwie trasy z wildcardem `{id}`. Testy store, handler i izolacji między kontami.

### Changes Required:

#### 1. Store: Update i Delete

**File**: `internal/transactions/store.go`

**Intent**: Dodać dwie metody mutujące, obie gates `user_id` w SQL, oraz sentinel `ErrNotFound`. Update zachowuje istniejący `kind` wiersza i wymaga, by nowa kategoria miała ten sam `kind`.

**Contract**:
- `var ErrNotFound = errors.New("transactions: not found")` (dodać import `errors`).
- `func (s *Store) Update(ctx context.Context, userID, txID, categoryID, kind, amount, occurredOn, description string) (Transaction, bool, error)` — bez `kind` w sygnaturze: `func (s *Store) Update(ctx context.Context, userID, txID, categoryID, amount, occurredOn, description string) (Transaction, bool, error)`.
  - Trim wejść; walidacja: `userID != "" && uuidRe.MatchString(txID) && uuidRe.MatchString(categoryID) && positiveAmount(amount)`, data parsuje się przez `time.Parse("2006-01-02", …)`; w razie niepowodzenia `return Transaction{}, false, nil`.
  - `description = truncateRunes(strings.TrimSpace(description), MaxDescriptionLen)`.
  - Jedno zapytanie CTE (wzorzec z `Create`):
    ```sql
    WITH upd AS (
        UPDATE transactions t
        SET category_id = c.id, amount = $3::numeric, occurred_on = $4::date, description = $5
        FROM categories c
        WHERE t.id = $1 AND t.user_id = $2
          AND c.id = $6 AND c.user_id = $2 AND c.kind = t.kind
        RETURNING t.id, t.kind, t.category_id, t.amount, t.occurred_on, t.description, t.created_at
    )
    SELECT upd.id, upd.kind, upd.category_id, c.name, p.name, upd.amount::text, upd.occurred_on, upd.description, upd.created_at
    FROM upd
    JOIN categories c ON c.id = upd.category_id
    LEFT JOIN categories p ON p.id = c.parent_id
    ```
    Argumenty: `userID, txID, amount, occurredOn, description, categoryID`.
  - `sql.ErrNoRows` → probe `SELECT 1 FROM transactions WHERE id = $1 AND user_id = $2` (txID, userID); jeśli wiersz istnieje → `Transaction{}, false, nil` (400, zła kategoria); jeśli nie → `Transaction{}, false, ErrNotFound`.
  - Inny `err` → `fmt.Errorf("transactions: update: %w", err)`. Sukces → `t, true, nil` (z `ParentName` z `sql.NullString`).
- `func (s *Store) Delete(ctx context.Context, userID, txID string) (bool, error)`
  - Walidacja: `userID != "" && uuidRe.MatchString(txID)`; inaczej `return false, nil`.
  - `DELETE FROM transactions WHERE id = $1 AND user_id = $2`; `RowsAffected()==0` → `false, ErrNotFound`; sukces → `true, nil`; błąd → `fmt.Errorf("transactions: delete: %w", err)`.

#### 2. Handler: Update i Delete

**File**: `internal/transactions/handler.go`

**Intent**: Cienkie warstwy HTTP dla obu mutacji: konto z kontekstu, walidacja `{id}` i body, mapowanie na 200/204/400/401/404/405/500. Dodać `updateInput` bez pola `kind` (typ niezmienny) oraz helper `writeNotFound`.

**Contract**:
- `type updateInput struct { Amount string; CategoryID string; OccurredOn string; Description string }` z tagami `json` jak `createInput` (bez `kind`).
- `func (h *Handler) Update(w http.ResponseWriter, r *http.Request)`:
  - `r.Method != http.MethodPut` → `methodNotAllowed`.
  - brak konta → `writeUnauthorized`.
  - `id := r.PathValue("id")`; `!uuidRe.MatchString(id)` → `writeInvalid`.
  - decode przez `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)`; błąd → `writeInvalid`.
  - trim pól; `amount == "" || categoryID == ""` → `writeInvalid`; `time.Parse("2006-01-02", occurredOn)` błąd → `writeInvalid`.
  - `tx, ok, err := h.store.Update(...)`; `errors.Is(err, ErrNotFound)` → `writeNotFound`; `err != nil` → `writeServerError`; `!ok` → `writeInvalid`; sukces → `writeJSON(w, http.StatusOK, toDTO(tx))`.
- `func (h *Handler) Delete(w http.ResponseWriter, r *http.Request)`:
  - `r.Method != http.MethodDelete` → `methodNotAllowed`; brak konta → `writeUnauthorized`.
  - `id := r.PathValue("id")`; `!uuidRe.MatchString(id)` → `writeInvalid`.
  - `deleted, err := h.store.Delete(...)`; `errors.Is(err, ErrNotFound)` → `writeNotFound`; `err != nil` → `writeServerError`; `!deleted` → `writeInvalid`; sukces → `w.WriteHeader(http.StatusNoContent)`.
- `func writeNotFound(w http.ResponseWriter)` → `writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})`.
- Dodać import `errors`.

#### 3. Trasy

**File**: `cmd/nest-cash/spa.go`

**Intent**: Zarejestrować dwie mutacje za `account.RequireAccount`, wzorem istniejących tras transakcji.

**Contract**: w bloku `if a.transactions != nil && a.resolver != nil`:
```go
mux.Handle("PUT /api/transactions/{id}", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.Update)))
mux.Handle("DELETE /api/transactions/{id}", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.Delete)))
```

#### 4. Testy backendu

**File**: `internal/transactions/store_test.go`, `internal/transactions/handler_test.go`, `internal/transactions/isolation_integration_test.go`

**Intent**: Pokryć szczęśliwe ścieżki, walidację i — najważniejsze — bramkę właściciela na mutacjach.

**Contract**:
- `store_test.go`: `TestUpdateTransaction` (zmiana kategorii/kwoty/daty/opisu, odczyt przez `List`, nazwy kategorii w wyniku), `TestUpdateRejectsInvalidInput` (malformed txID, zła kwota/data, obca kategoria, kategoria innego `kind`, nieznany txID → `ErrNotFound`), `TestDeleteTransaction` (usunięcie własnego, drugie usunięcie → `ErrNotFound`).
- `handler_test.go`: `TestHandlerUpdate` (PUT → 200 + DTO, następnie list odzwierciedla), `TestHandlerUpdateValidation` (bad json, puste/`złe pola`, zły date, malformed id → 400, nieznany id → 404, obca kategoria → 400), `TestHandlerDelete` (DELETE → 204 pusty body; powtórny DELETE → 404; malformed id → 400), oraz rozszerzyć `TestHandlerMethodNotAllowed`/`TestHandlerUnauthorizedWithoutAccount` o nowe metody.
- `isolation_integration_test.go`: rozszerzyć `TestTransactionsCrossAccountIsolationThroughSessions` — B wykonuje `PUT` i `DELETE` na `id` transakcji A, oba → 404, licznik i zawartość wierszy A bez zmian.

### Success Criteria:

#### Automated Verification:

- Testy store przechodzą: `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestUpdate|TestDelete'`
- Testy handlera przechodzą: `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestHandler'`
- Test izolacji przechodzi: `CGO_ENABLED=0 go test ./internal/transactions/ -run Isolation`
- Cały pakiet i build przechodzą: `CGO_ENABLED=0 go test ./... && go build ./...`
- Formatowanie: `gofmt -l internal/transactions cmd/nest-cash` zwraca pustą listę

#### Manual Verification:

- `curl -X PUT` na własny `id` zwraca 200 z nowymi wartościami; na obcy/nieznany → 404; obca kategoria → 400.
- `curl -X DELETE` na własny `id` zwraca 204 bez body; powtórnie → 404.
- Po edycji i usunięciu `GET /api/transactions` oraz `GET /api/summary` pokazują zaktualizowane dane.

**Implementation Note**: Po fazie i przejściu testów automatycznych zatrzymaj się po ręczne potwierdzenie `curl`-em, zanim przejdziesz do fazy 2.

---

## Phase 2: Frontend — inline edit i delete w Operations

### Overview

Dodaje funkcje klienta API, edycję wiersza w miejscu oraz potwierdzenie usunięcia w wierszu. Po mutacji lista przeładowuje stronę 1, więc Summary pokazuje nowe sumy przy następnym pobraniu.

### Changes Required:

#### 1. Klient API

**File**: `web/src/api/client.ts`

**Intent**: Dodać `updateTransaction` i `deleteTransaction`, korzystając z istniejącego `request<T>` (204 → `undefined`).

**Contract**:
```ts
export function updateTransaction(
  id: string,
  input: { amount: string; category_id: string; occurred_on: string; description?: string },
): Promise<Transaction> {
  return request<Transaction>(`/transactions/${id}`, { method: 'PUT', body: JSON.stringify(input) })
}

export function deleteTransaction(id: string): Promise<void> {
  return request<void>(`/transactions/${id}`, { method: 'DELETE' })
}
```

#### 2. Ekran Operations

**File**: `web/src/views/Operations.vue`

**Intent**: W tabeli operacji dodać kolumnę Actions z Edit/Delete; Edit przełącza wiersz w tryb edycji (kategoria zawężona do `kind` wiersza + kwota + data + opis) z Save/Cancel; Delete pokazuje w wierszu potwierdzenie Yes/No. Po sukcesie przeładować stronę 1 (wzorem `submit`). `kind` pozostaje tylko do odczytu.

**Contract**:
- Nowy stan: `editingId: string | null`, `confirmingId: string | null`, `rowBusy: string | null`, `edit = { amount, category_id, occurred_on, description }`, `editError: string`.
- Helper `groupsFor(k: 'expense' | 'income')` → `groups.value.filter((g) => g.kind === k)` (istniejące `kindGroups` zostaje dla formularza dodawania).
- `startEdit(t)` kopiuje pola wiersza do `edit` i ustawia `editingId`; `cancelEdit()` czyści `editingId` i `editError`.
- `saveEdit()` waliduje kwotę tym samym `amountRe` co `submit` i wywołuje `updateTransaction(editingId, …)`; sukces → `cancelEdit()` + `await loadFirstPage()`; błąd → `handle(e, editError)`.
- `remove(id)` wywołuje `deleteTransaction(id)`; sukces → `confirmingId = null` + `await loadFirstPage()`; błąd → `handle(e, listError)`.
- W komórce akcji: gdy `editingId === t.id` → Save/Cancel; gdy `confirmingId === t.id` → tekst „Delete?” + Yes/No; inaczej Edit/Delete. Dodać nagłówek kolumny `Actions`.
- W trybie edycji komórki Category (select z `groupsFor(t.kind)` w układzie optgroup/children jak w formularzu dodawania) / Amount / Date / Description zamieniają się w inputy; Type i Date pozostają widoczne.
- `rowBusy` (na `id`) blokuje przyciski w trakcie żądania. Zachować istniejące wzorce `handle`/`message` i redirect na 401.

### Success Criteria:

#### Automated Verification:

- Type-check i build przechodzą: `npm --prefix web run build`
- Brak nieużywanych symboli (tsconfig wymusza) — build nie zgłasza błędów TS6133/noUnusedLocals

#### Manual Verification:

- Edit na wierszu pokazuje pola w miejscu; zmiana kwoty/kategorii/daty/opisu i Save aktualizuje wiersz bez przeładowania strony.
- Cancel oraz No przy usunięciu nie zmieniają danych.
- Delete → Yes usuwa wiersz z listy; brak potwierdzenia nie usuwa.
- Brak jakiejkolwiek kontrolki zmiany typu (expense↔income) w trybie edycji.
- Po edycji/usunięciu przejście na Summary pokazuje zaktualizowane sumy i listę.
- Błąd walidacji (np. kwota „abc”) pokazuje komunikat i nie zapisuje.
- Sprawdzenie w dwóch przeglądarkach (NFR cross-browser).

**Implementation Note**: Po fazie zatrzymaj się po ręczne potwierdzenie pełnego przepływu w przeglądarce.

---

## Testing Strategy

### Unit/Integration Tests (Go):

- Store: happy path Update (w tym nazwy kategorii i brak float), odrzucenia (zła kwota/data/kategoria/kind), `ErrNotFound`, Delete + powtórny Delete.
- Handler: kody 200/204/400/401/404/405 dla obu mutacji, odzwierciedlenie w `List`.
- Izolacja: B nie może zmienić ani usunąć wiersza A (404, stan A bez zmian) — rozszerzenie istniejącego testu sesyjnego.

### Manual Testing Steps:

1. Zaloguj się, wejdź na `/operations`, dodaj wydatek.
2. Edit wiersza: zmień każdy z czterech edytowalnych atrybutów osobno, Save, potwierdź wiersz.
3. Cancel w trakcie edycji — brak zmian.
4. Delete → Yes usuwa; Delete → No nie usuwa.
5. Wejdź na `/summary` w okresie operacji — sumy i lista odzwierciedlają zmiany.
6. Spróbuj zmienić typ przez API (`curl PUT` z kategorią innego `kind`) → 400, wiersz bez zmian.
7. `curl` na obcy `id` (drugie konto) → 404.

## Performance Considerations

Bez zmian wydajnościowych. UPDATE po kluczu głównym + DELETE po kluczu głównym to pojedyncze indeksowane operacje. Brak nowych indeksów i migracji.

## Migration Notes

Brak migracji i zmian schematu. Mutacje działają na istniejącej tabeli `transactions`. Hard-delete trwale usuwa wiersz (zgodnie z „retencja bezterminowa dopóki użytkownik nie usunie”); brak kolumny wersji oznacza last-write-wins, co jest akceptowalne dla single-user.

## References

- Roadmapa: `context/foundation/roadmap.md` (S-07, FR-008)
- PRD: `context/foundation/prd.md` (FR-008, Non-Goals)
- Wzorzec store/handler/CTE: `internal/transactions/store.go:86`, `internal/transactions/handler.go:82`
- Wzorzec tras method+path: `cmd/nest-cash/spa.go:31-35`
- Poprzedni pion: `context/archive/2026-09-12-expense-entry-and-operation-list/plan.md`
- Lekcje: `context/foundation/lessons.md`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend — mutacje Update i Delete

#### Automated

- [x] 1.1 Store Update/Delete tests pass: `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestUpdate|TestDelete'` — 38873a4
- [x] 1.2 Handler tests pass: `CGO_ENABLED=0 go test ./internal/transactions/ -run 'TestHandler'` — 38873a4
- [x] 1.3 Isolation test passes: `CGO_ENABLED=0 go test ./internal/transactions/ -run Isolation` — 38873a4
- [x] 1.4 Full test suite + build pass: `CGO_ENABLED=0 go test ./... && go build ./...` — 38873a4
- [x] 1.5 `gofmt -l internal/transactions cmd/nest-cash` returns empty — 38873a4

#### Manual

- [x] 1.6 `curl` PUT own id → 200; foreign/unknown → 404; foreign category → 400 — 38873a4
- [x] 1.7 `curl` DELETE own id → 204 empty; repeat → 404 — 38873a4
- [x] 1.8 `GET /api/transactions` and `GET /api/summary` reflect mutations — 38873a4

### Phase 2: Frontend — inline edit i delete w Operations

#### Automated

- [x] 2.1 Type-check and build pass: `npm --prefix web run build`

#### Manual

- [ ] 2.2 Inline edit saves changes in place
- [ ] 2.3 Cancel and delete-No make no changes
- [ ] 2.4 Delete-Yes removes the row
- [ ] 2.5 No kind-change control in edit mode
- [ ] 2.6 Summary reflects edits and deletes
- [ ] 2.7 Validation error shows message and does not save
- [ ] 2.8 Verified in two browsers
