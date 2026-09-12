# Income and Budget Ratio Implementation Plan

## Overview

Dopinamy S-06 (FR-006 + Business Logic): jeden formularz operacji z przełącznikiem wydatek/przychód, `POST /transactions` z polem `kind`, listy pokazujące oba rodzaje oraz pasywny wskaźnik `wydatki/przychody` z progiem 80% na `/summary`. Bez migracji — kolumna `kind` już istnieje.

## Current State Analysis

`Store.Create` pisze wyłącznie `expense` na sztywno (`internal/transactions/store.go:95` — `SELECT ... 'expense'`), więc przychód nie ma dziś ścieżki zapisu mimo gotowej kolumny `kind TEXT CHECK (kind IN ('expense','income'))` (`internal/db/migrations/0004_transactions.sql:5`). `Summary` filtruje `t.kind = 'expense'` (`internal/transactions/summary.go:87`) i zwraca tylko agregat wydatków; brak sumy przychodów i ratio. `Operations.vue` pokazuje wyłącznie grupy `kind==='expense'`, `Summary.vue` wyłącznie agregaty wydatków. Kategorie przychodów istnieją w seedzie (`resources/categories.yaml:47-52`, płaskie: Wynagrodzenie, Premie…), więc gate własności+kind ma na czym pracować.

## Desired End State

Użytkownik na `/operations` przełącza Wydatek/Przychód w jednym formularzu, zapisuje przychód (kwota + kategoria przychodowa + data + opis) i widzi go na liście z kolumną Rodzaj. Na `/summary` ten sam filtr okresu pokazuje: dotychczasową tabelę wydatków per kategoria, sekcję przychodów (suma) oraz wskaźnik ratio z badge `<80%` zielony / `80–99%` żółty + tekst „Zbliżasz się do limitu" / `>=100%` czerwony. Przy zerowych przychodach ratio to `—` + hint „Brak przychodów w okresie", nie liczba. Ratio liczone z całego okresu, niezależnie od zaznaczonych kategorii wydatków.

### Key Discoveries:

- `internal/transactions/store.go:80-114` — `Create` z atomowym `INSERT ... SELECT` gatejącym ownership+kind; rozszerzenie o parametr `kind` to jedna ścieżka, nie nowy endpoint.
- `internal/db/migrations/0004_transactions.sql:5,12` — schemat gotowy, indeks `(user_id, occurred_on DESC, created_at DESC)` pokrywa nowe agregaty bez migracji.
- `internal/transactions/summary.go:29` — sygnatura `Summary(...)` do rozszerzenia o sumy globalne; `handler.go:141-185` — szkielet `Summary` do skopiowania dla nowych pól.
- `web/src/views/Operations.vue:35-49` — wzorzec `categoryIds` z hierarchii; ten sam wzorzec działa dla `kind==='income'` (grupy płaskie).
- `context/foundation/lessons.md:9` — każdy błąd znad DB wrapować `fmt.Errorf("transactions: <op>: %w", err)`.

## What We're NOT Doing

- Konfigurowalnego progu per-user (PRD: próg 80% na sztywno w MVP, konfigurowalny w przyszłości).
- Zewnętrznych powiadomień push/email (Non-Goal; tylko pasywny badge w UI).
- Edycji/usuwania transakcji i kategorii (FR-004/FR-008 → v2).
- Wykresów graficznych (parked; tabela + badge wystarczą).
- Nowej migracji DB ani osobnego pakietu `internal/summary`.
- Paginacji listy w summary (limit 50 jak w S-05).

## Implementation Approach

Najmniejszy diff w istniejącym pakiecie `transactions`: dodać `kind` do `Create` (default `expense` dla kompatybilności), dodać filtr `kind` do `List`, rozszerzyć `Summary` o dwie globalne sumy (cały okres, bez filtra kategorii) + ratio liczone w Go na `math/big` bez floatów. Jeden request `GET /api/summary` zwraca jednocześnie filtrowany widok wydatków (kompatybilny z S-05) i globalne pola ratio — frontend nie robi drugiego requestu. Najpierw backend z testami Go, potem frontend z `npm run build` jako bramką.

## Critical Implementation Details

- **Pieniądze jako string.** `NUMERIC(12,2)` i `SUM(amount)` skanować do `string` (`::text`), ratio liczyć na `math/big.Rat` z parsowania stringów, nigdy `float64`. Frontend tylko wyświetla.
- **Ratio z całego okresu.** `expense_total` w `rows/total` to widok filtrowany kategoriami (S-05); `ratio` + `income_total` + `ratio_expense_total` to agregaty globalne per `from/to` bez filtra `category_id`. Inaczej filtr 1 kategorii dawałby nonsense budżetowy.
- **Zero przychodów.** `income_total = '0'` → `ratio_pct = null` (JSON `null`), frontend rysuje `—` + hint. Nie `0%`, nie `∞`.
- **Format ratio.** `ratio_pct` jako string z 1 miejscem po przecinku (np. `"83.3"`), liczony jako `expenses*100/incomes`. `big.Rat.FloatString(1)` po mnożeniu — zaokrąglenie half-away nie jest krytyczne przy 1 decimal dla badge.
- **Toggle czyści kategorię.** Przełączenie Wydatek/Przychód resetuje `categoryId` (ID z jednego kind nie istnieje w drugim) i podmienia listę grup; `localStorage` trzyma osobny klucz per kind.

## Phase 1: API przychodów i ratio

### Overview

`Create` z `kind`, `List` z filtrem `kind`, `Summary` z globalnymi sumami + ratio, testy Go. Po tej fazie `curl` z ciasteczkiem zwraca przychody i ratio.

### Changes Required:

#### 1. Store Create + List z kind

**File**: `internal/transactions/store.go`, `internal/transactions/store_test.go`

**Intent**: `Create` przyjmuje `kind` (`expense|income`, puste = `expense`) i gatejście kategorii wymaga zgodnego `kind` (`c.kind = $kind`). `List` przyjmuje filtr `kind` (`all|expense|income`, default `all`) — zwraca obie kategorie z joinem nazw.

**Contract**: `func (s *Store) Create(ctx, userID, categoryID, kind, amount, occurredOn, description string) (Transaction, bool, error)` — `ok=false` przy złym `kind`, obcej kategorii lub niezgodnym `kind` kategorii. `func (s *Store) List(ctx, userID string, page, limit int, kind string)`. Istniejące wywołania w testach (`store_test.go`, `summary_test.go` seed) do aktualizacji o `KindExpense`. Błędy DB wrapowane `transactions: create/list ...: %w`.

#### 2. Store Summary z ratio

**File**: `internal/transactions/summary.go` (rozszerzyć), `internal/transactions/summary_test.go` (rozszerzyć)

**Intent**: Zachować dotychczasowe `rows/total/items` (wydatki, filtr kategorii) i dołożyć trzy pola globalne z tego samego `from/to` bez filtra kategorii: suma wydatków, suma przychodów, ratio.

**Contract**: Sygnatura zwraca dodatkową strukturę lub rozszerzony tuple — np. `func (s *Store) Summary(...) (rows []SummaryRow, total string, items []Transaction, budget BudgetRatio, ok bool, err error)` gdzie `type BudgetRatio struct { ExpenseTotal, IncomeTotal string; RatioPct *string }`. Dwie dodatkowe kwerendy `COALESCE(SUM ... ::text,'0')` z filtrem `user_id + kind + BETWEEN`, bez `category_id`. Ratio w Go na `math/big`: parse obu stringów do `Rat`, `ratio = expenses*100/incomes`, `FloatString(1)`; przy `incomes == 0` → `RatioPct = nil`. Gate kategorii jak dziś (tylko `expense`, obce → `ok=false`).

#### 3. Handler + walidacja

**File**: `internal/transactions/handler.go`, `internal/transactions/handler_test.go`

**Intent**: `POST /transactions` czyta opcjonalne `kind` z body (brak/białe = `expense`, inne niż `expense|income` → 400). `GET /transactions?kind=` waliduje (`all|expense|income`, default `all`). `GET /api/summary` zwraca rozszerzony kształt z sekcją `budget`.

**Contract**: `POST {amount, category_id, occurred_on, description?, kind?}` → `201 transactionDTO` (z polem `kind`). `GET /api/transactions?kind=income&page=&limit=` → `listDTO` bez zmian kształtu. `GET /api/summary?from=&to=&category_id=` → `200 {from, to, rows[], total, items[], budget: {expense_total, income_total, ratio_pct: string|null}}`. `total` = filtrowany (S-05 compat), `budget.expense_total` = globalny. Brak zmian w `cmd/nest-cash/spa.go` (te same 3 route'y).

### Success Criteria:

#### Automated Verification:

- Cały backend zielony: `CGO_ENABLED=0 go test ./...`
- Testy store: `DATABASE_URL=... go test ./internal/transactions/ -run 'TestCreate|TestList|TestSummary' -v` (zapis income, gate niezgodnego kind → ok=false, List filtr kind, Summary ratio 62.5 / null przy zerze / izolacja A/B)
- Testy handlera: `DATABASE_URL=... go test ./internal/transactions/ -run TestSummaryHandler` + Create/List (201/200, 400 na zły kind, 401 bez konta)
- `gofmt -l internal/transactions` pusty

#### Manual Verification:

- `curl -b session POST /transactions {kind:income, amount, kategoria przychodowa}` → 201 z `"kind":"income"`; z kategorią wydatkową → 400
- `curl GET /api/transactions?kind=income` pokazuje tylko przychody; `?kind=all` miesza oba
- `curl GET /api/summary?from=2026-09-01&to=2026-09-30` zwraca `budget {expense_total, income_total, ratio_pct}` zgodne z ręczną sumą z `/transactions`
- Okres bez przychodów → `"ratio_pct": null`; zły `kind=foo` → 400; bez ciasteczka → 401

**Implementation Note**: Po zielonych testach automatycznych pauza na ręczny `curl` przed fazą 2. Bloczki faz używają zwykłych `- `; checkboxy żyją tylko w `## Progress` na dole.

---

## Phase 2: UI przychodów i wskaźnika

### Overview

Jeden formularz z toggle w Operations, kolumna Rodzaj + filtr kind, sekcja ratio z badge w Summary. Po tej fazie pełny slice działa w przeglądarce.

### Changes Required:

#### 1. Klient API

**File**: `web/src/api/client.ts` (rozszerzyć)

**Intent**: Typy i fetchery pod nowe pola, w istniejącym `request<T>`.

**Contract**: `createTransaction(input: {amount, category_id, occurred_on, description?, kind?: 'expense'|'income'})`, `listTransactions(page, limit, kind?: 'all'|'expense'|'income')` (default `all`; `all` = pomiń parametr), `interface Budget {expense_total: string; income_total: string; ratio_pct: string|null}`, `SummaryResponse` + `budget: Budget`.

#### 2. Widok Operations z toggle

**File**: `web/src/views/Operations.vue` (przepisać formularz + tabelę)

**Intent**: Radio Wydatek/Przychód nad selectem kategorii; select pokazuje grupy filtrowane po wybranym kind (expense: hierarchia grupa→liście jak dziś; income: grupy płaskie z `resources/categories.yaml`). Tabela operacji dostaje kolumnę Rodzaj + filtr All/Wydatki/Przychody nad listą.

**Contract**: `kind` jako `ref<'expense'|'income'>`; `groupsForKind` computed; przełączenie kind czyści `categoryId`; `LAST_CATEGORY_KEY` per kind (`nestcash:last-category:expense|income`). Submit wysyła `kind`, sypie 400 → komunikat o kwocie/kategorii/dacie, 401 → redirect `/login` (lokalnie jak dziś). Klasy `.card/.stack/.field/.table/.btn/.alert` z `style.css`, bez nowych zależności.

#### 3. Widok Summary z ratio

**File**: `web/src/views/Summary.vue` (dopisać sekcję)

**Intent**: Nowa karta „Budżet okresu" nad tabelą kategorii: `Wydatki X / Przychody Y / Ratio Z%` + badge stanu. Ratio z `budget` (globalne), nie z filtrowanej tabeli.

**Contract**: `ratio_pct === null` → `—` + hint „Brak przychodów w okresie". Badge: `<80` zielony, `80–99.9` żółty + tekst „Zbliżasz się do limitu", `>=100` czerwony + tekst „Wydatki przekraczają przychody". Parse `ratio_pct` do `Number` tylko dla progów kolorów (wyświetlanie ze stringa, bez przeliczeń). Kategorie-income nie wchodzą do multi-selecta (ten zostaje expense-only jak w S-05).

### Success Criteria:

#### Automated Verification:

- Build frontu przechodzi: `npm --prefix web run build`
- Backend nadal zielony: `CGO_ENABLED=0 go test ./...`

#### Manual Verification:

- Zapis przychodu przez toggle: pojawia się na `/operations` z Rodzaj=income i w filtrze `kind=income`; przełączenie toggle czyści kategorię
- `/summary` bieżący miesiąc: badge zielony <80%, żółty + „Zbliżasz się do limitu" przy 80–99%, czerwony przy >=100% (sprawdzić na 2–3 seedowanych okresach)
- Okres bez przychodów → `—` + hint, nie 0% ani błąd
- Zmiana multi-selecta kategorii nie zmienia ratio (ratio globalne), zmienia tylko tabelę wydatków
- Odpowiedź `< 2s` na lokalnym PG przy kilkuset transakcjach; izolacja: drugi user nie widzi cudzych sum

---

## Testing Strategy

### Unit Tests:

- Store `Create`: zapis income, default `expense` przy pustym kind, odrzut złego kind, odrzut kategorii o niezgodnym kind, obca kategoria → `ok=false`
- Store `List`: filtr `all/expense/income`, paginacja z mieszanymi kind, sortowanie newest-first bez zmian
- Store `Summary`: ratio np. `12.50/20.00 → "62.5"`, `income 0 → nil`, `from>to`/zły UUID → `ok=false`, izolacja użytkowników, sumy jako stringi bez floatów
- Handler: 200 kształt z `budget`, 400 na zły kind/from>to/obcą kategorię, 401 bez konta, 405 na POST do summary

### Integration Tests:

- Seed: 1 user × 2 kategorie expense + 1 income × transakcje na granicach miesięcy; assert `rows` filtrowane vs `budget` globalne + limit 50 items z mieszanymi kind

### Manual Testing Steps:

1. Zarejestruj usera, dodaj wydatek 50.00 i przychód 100.00 dziś → Summary pokazuje ratio 50.0% zielony badge
2. Dodaj wydatek 40.00 → ratio 90.0% żółty + „Zbliżasz się do limitu"
3. Dodaj wydatek 20.00 → ratio 110.0% czerwony
4. Nowy miesiąc bez przychodów → `—` + hint
5. Drugi user nie widzi cudzych przychodów ani ratio

## Performance Considerations

Wolumen osobisty (setki–tysiące wierszy): dwie dodatkowe `SUM` po indeksie `idx_transactions_user_occurred` (`WHERE user_id + kind + occurred_on BETWEEN`) bez seq-scana; bez `COUNT(*)` w summary. Response summary rośnie o 3 pola — pomijalnie. Denormalizacja dopiero gdyby `EXPLAIN` pokazał problem.

## Migration Notes

Brak migracji — `kind` i indeks istnieją od `0004`. Rollback = revert handlera/store (stare sygnatury) + stare `Operations.vue`/`Summary.vue`; dane nietknięte (przychodowe wiersze po rollbacku po prostu niewidoczne).

## References

- PRD FR-006: `context/foundation/prd.md:71-72`; Business Logic: `context/foundation/prd.md:89-97`; Non-Goal powiadomień: `context/foundation/prd.md:112`
- Roadmap S-06: `context/foundation/roadmap.md:136-146`
- Store: `internal/transactions/store.go:80-114`; Summary: `internal/transactions/summary.go:29,87`; Handler: `internal/transactions/handler.go:72-185`
- Wiring (bez zmian): `cmd/nest-cash/spa.go:31-35`
- UI: `web/src/views/Operations.vue:35-49`, `web/src/views/Summary.vue:1-99`, `web/src/api/client.ts:99-136`
- Seed przychodów: `resources/categories.yaml:47-52`
- Lekcja: `context/foundation/lessons.md:9`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: API przychodów i ratio

#### Automated

- [x] 1.1 Cały backend zielony — b52595e
- [x] 1.2 Testy store Create/List/Summary z kind i ratio — b52595e
- [x] 1.3 Testy handlera 200/400/401 — b52595e
- [x] 1.4 gofmt pusty — b52595e

#### Manual

- [x] 1.5 curl zapis income + filtry kind działają — b52595e
- [x] 1.6 curl summary zwraca budget zgodny z transactions; zero-income daje null; złe inputy 400, brak sesji 401 — b52595e

### Phase 2: UI przychodów i wskaźnika

#### Automated

- [x] 2.1 Build frontu przechodzi — c6b954e
- [x] 2.2 Backend nadal zielony — c6b954e

#### Manual

- [x] 2.3 Toggle zapisuje przychód, lista z Rodzaj i filtrem kind — c6b954e
- [x] 2.4 Badge zielony/żółty/czerwony na seedowanych okresach — c6b954e
- [x] 2.5 Zero przychodów daje myślnik + hint; multi-select nie rusza ratio — c6b954e
- [x] 2.6 Odpowiedź < 2s + izolacja drugiego usera — c6b954e
