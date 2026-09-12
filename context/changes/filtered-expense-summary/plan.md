# Filtered Expense Summary Implementation Plan

## Overview

Budujemy filtrowane podsumowanie wydatków (S-05, FR-009): tabelaryczny widok agregatów per kategoria + lista do 50 najnowszych operacji, filtrowany jednocześnie po okresie (miesiąc / rok / zakres custom) i kategoriach (multi-select). To gwiazda przewodnia roadmapy — pierwszy przepływ sprawdzający główny cel z US-01.

## Current State Analysis

S-04 dowiezione: `POST/GET /api/transactions` działa z izolacją konta, walidacją kwot i paginacją. `Summary.vue` to placeholder bez wywołania API. Brak jakiejkolwiek agregacji `SUM/GROUP BY` w Go — jedyny agregat to `COUNT(*)` do paginacji.

## Desired End State

Użytkownik na `/summary` wybiera tryb okresu (miesiąc / rok / zakres), zaznacza 0..N kategorii i widzi: tabelę sum per kategoria + wiersz Razem oraz listę do 50 najnowszych transakcji z tego filtra. Wydatek zapisany w Operations pojawia się w podsumowaniu bez reloada strony (po ponownym pobraniu filtra). Filtry nieprawidłowe dają czytelny błąd, pusty poprawny okres daje zera.

### Key Discoveries:

- `internal/transactions/store.go:118` — `List` z `COUNT(*)` + join kategorii; indeks `(user_id, occurred_on DESC, created_at DESC)` pokrywa filtry S-05 bez nowej migracji (`internal/db/migrations/0004_transactions.sql:12`).
- `internal/transactions/handler.go:100` — wzorzec `List`: `RequireAccount` + `parseBounded` + `writeJSON`; nowy endpoint kopiuje ten szkielet.
- `cmd/nest-cash/spa.go:31-34` + `cmd/nest-cash/main.go:81,87` — `transactionsHandler` już wwired; nowy route to 1 linia w `spa.go`, zero zmian w `main.go`.
- `web/src/views/Operations.vue:35-49` — gotowy wzorzec wyliczania `categoryIds` z hierarchii grupa→liść oraz `localStorage nestcash:last-category`; `web/src/api/client.ts:20` — jedno `request<T>` + `ApiError`.
- `context/foundation/lessons.md:9` — każdy błąd z DB wrapować `fmt.Errorf("<pkg>: <op>: %w", err)`.

## What We're NOT Doing

- Przychody w podsumowaniu (zostają w S-06 z ratio 80%; `kind` na sztywno `expense`).
- Wykresy graficzne (parked w roadmapie; tabela wystarcza).
- Edycja/usuwanie transakcji i kategorii (FR-004/FR-008 → v2).
- Paginacja listy w summary (limit 50 bez stron; pełna lista na `/operations`).
- Nowa migracja DB (schemat gotowy).
- Osobny pakiet `internal/summary` (rozszerzamy `transactions` — mniejszy diff).

## Implementation Approach

Najmniejszy diff: rozszerzyć istniejący pakiet `transactions` o metodę agregującą (SQL liczy `SUM`, Go/JS nie dotyka floatów), dołożyć cienki handler `GET /api/summary` za tym samym `RequireAccount`, wpiąć 1 route w `spa.go`, przepisać `Summary.vue` na wzór `Operations.vue`/`Categories.vue`. Najpierw backend z testami Go (walidacja + izolacja w SQL), potem frontend z `npm run build` jako bramką.

## Critical Implementation Details

- **Pieniądze jako string.** `NUMERIC(12,2)` skanować do `string` (`amount::text`, `SUM(amount)::text`), nigdy `float/ParseFloat` — inaczej rozjazdy groszy. Frontend tylko wyświetla.
- **Daty jako `from/to`.** Frontend przelicza tryb (month/year/custom) na ISO `YYYY-MM-DD`; backend parsuje `time.Parse("2006-01-02")` i wymaga `from <= to`, inaczej 400. Miesiąc → pierwszy/ostatni dzień; rok → 01-01/12-31.
- **Multi-select jako CSV.** `?category_id=id1,id2` (pusto = wszystkie). Każdy element musi pasować do `uuidRe`; obcy/nieistniejący `expense` → 400, nie ciche 0. Grupa nadrzędna bez dzieci jest sama liściem (jak w Operations).

## Phase 1: API podsumowania

### Overview

Agregacja SQL + handler + route + testy Go. Po tej fazie `curl` z ciasteczkiem sesji zwraca poprawne sumy.

### Changes Required:

#### 1. Store agregujący

**File**: `internal/transactions/summary.go` (nowy) + `internal/transactions/summary_test.go` (nowy)

**Intent**: Dwie kwerendy z filtrem właściciela: (a) `SUM per category_id` z joinem nazw, (b) 50 najnowszych transakcji z tym samym filtrem.

**Contract**: `func (s *Store) Summary(ctx, userID, from, to string, categoryIDs []string) (rows []SummaryRow, total string, items []Transaction, ok bool, err error)`. `ok=false` = złe wejście (zła data, from>to, zły UUID, puste userID). Błędy DB wrapowane `transactions: summary ...: %w`. Kategorie obce/nie-`expense` → `ok=false` (gate przez join z `categories WHERE user_id + kind='expense'`).

#### 2. Handler + route

**File**: `internal/transactions/handler.go` (metoda `Summary`), `internal/transactions/handler_test.go` (testy), `cmd/nest-cash/spa.go` (1 linia route)

**Intent**: Cienka warstwa HTTP jak `List`: auth z kontekstu, parsowanie query, mapowanie `ok=false → 400`, `err → 500`.

**Contract**: `GET /api/summary?from=YYYY-MM-DD&to=YYYY-MM-DD&category_id=uuid,uuid` → `200 {from, to, rows[{category_id, category_name, parent_name?, total}], total, items[transactionDTO ≤50]}`. Brak `from/to` → 400. `category_id` puste = wszystkie. `writeJSON` z `no-store`. Route: `mux.Handle("GET /api/summary", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.Summary)))` obok linii 32-33.

### Success Criteria:

#### Automated Verification:

- Migracje czyste (brak nowych, ale baza startuje): `DATABASE_URL=... go test ./internal/db/...`
- Testy store przechodzą: `DATABASE_URL=... go test ./internal/transactions/ -run Summary -v` (SUM per kategoria, filtr dat, multi-select, izolacja A/B, obca kategoria → ok=false)
- Testy handlera przechodzą: `DATABASE_URL=... go test ./internal/transactions/ -run TestSummaryHandler` (200/400/401)
- Cały backend zielony: `CGO_ENABLED=0 go test ./...`
- `gofmt -l internal/transactions cmd/nest-cash` pusty

#### Manual Verification:

- `curl -b session GET /api/summary?from=2026-09-01&to=2026-09-30` zwraca sumy zgodne z `/api/transactions` ręcznie zsumowanymi
- Zły `from>to` i obcy `category_id` zwracają `400 {"error":"invalid_request"}`
- Bez ciasteczka → `401`

**Implementation Note**: Po zielonych testach automatycznych pauza na ręczny `curl` przed fazą 2. Bloczki faz używają zwykłych `- `; checkboxy żyją tylko w `## Progress` na dole.

---

## Phase 2: Ekran podsumowania

### Overview

Przepisanie `Summary.vue` z placeholdera na filtry + dwie tabele, zintegrowane z API z fazy 1.

### Changes Required:

#### 1. Klient API

**File**: `web/src/api/client.ts` (dopisać)

**Intent**: Typy i fetcher pod nowy endpoint, w istniejącym `request<T>`.

**Contract**: `interface SummaryRow {category_id, category_name, parent_name?, total: string}`, `interface SummaryResponse {from, to, rows: SummaryRow[], total: string, items: Transaction[]}`, `getSummary(from: string, to: string, categoryIds: string[]): Promise<SummaryResponse>` (pusta lista = pomiń parametr `category_id`).

#### 2. Widok Summary

**File**: `web/src/views/Summary.vue` (przepisać, wzorzec `Operations.vue` + `Categories.vue`)

**Intent**: Kontrolki filtra (tryb miesiąc/rok/zakres + multi-select kategorii jako checkboxy grupowane), tabela agregatów z wierszem Razem, tabela do 50 operacji, stany `loading/error/empty`, link do `/operations` po więcej.

**Contract**: Tryb `month` = `<input type="month">`, `year` = `<input type="number">`, `custom` = dwa `<input type="date">`; konwersja do `from/to` w komponencie. Kategorie ładowane przez `listCategories()`, filtrowane do `kind==='expense'`. Błąd 400 → komunikat o dacie/kategorii; 401 → redirect `/login` (lokalnie jak w Operations, bez globalnego interceptora). Klasy `.card/.stack/.field/.table/.btn/.alert` z istniejącego `style.css`.

### Success Criteria:

#### Automated Verification:

- Build frontu przechodzi: `npm --prefix web run build` (type-check + vite)
- Backend nadal zielony: `CGO_ENABLED=0 go test ./...`

#### Manual Verification:

- US-01: wydatek zapisany w Operations widoczny w Summary po filtrze kategorii i okresu bez reloada aplikacji
- Presety miesiąc/rok/custom przeliczają poprawny zakres (wrzesień = 01–30, rok przestępny OK)
- Multi-select: odznaczone wszystko = wszystkie; 1 kategoria = tylko jej suma; suma wierszy = Razem
- Pusty miesiąc → komunikat o braku + zera, nie error
- `< 2s` odpowiedzi na lokalnym PG przy kilkuset transakcjach

---

## Testing Strategy

### Unit Tests:

- Store `Summary`: SUM per kategoria z podkategoriami, `from/to` inclusive, CSV 1/N/puste, `from>to`/zły format/zły UUID → `ok=false`, izolacja użytkowników, `SUM` jako string bez floatów
- Handler: 200 kształt, 400 na każdy zły input, 401 bez konta, 405 na POST

### Integration Tests:

- Seed: 2 userów × 2 kategorie × transakcje na granicach miesięcy; assert sumy + limit 50 + sortowanie newest-first zgodne z `List`

### Manual Testing Steps:

1. Zarejestruj usera, dodaj wydatek 12.34 dziś → Summary bieżący miesiąc pokazuje go w agregacie i na liście
2. Zmień filtr na zeszły miesiąc → zero, komunikat o braku
3. Zaznacz 1 kategorię → tylko jej wiersz; odznacz wszystko → wszystkie
4. `from>to` ręcznie w URL → 400 i czytelny komunikat
5. Drugi user nie widzi cudzych sum (2 przeglądarki / sesje)

## Performance Considerations

Osobisty wolumen (setki–tysiące wierszy): istniejący indeks `idx_transactions_user_occurred` pokrywa `WHERE user_id + occurred_on BETWEEN + ORDER BY`; `SUM/GROUP BY` po tym samym filtrze bez seq-scana. Bez `COUNT(*)` w summary. Limit 50 na liście trzyma response mały. Denormalizacja dopiero gdyby `EXPLAIN` pokazał problem — nie teraz.

## Migration Notes

Brak migracji. Rollback = revert 1 linii route + stare `Summary.vue` (placeholder); dane nietknięte.

## References

- PRD FR-009: `context/foundation/prd.md:77-78`; US-01 akceptacja: `context/foundation/prd.md:48-51`
- Roadmap S-05: `context/foundation/roadmap.md:123-134`
- Wzorzec store/handler: `internal/transactions/store.go:118`, `internal/transactions/handler.go:100`
- Wiring: `cmd/nest-cash/main.go:81`, `cmd/nest-cash/spa.go:31-33`
- UI: `web/src/views/Operations.vue:35-49`, `web/src/api/client.ts:20`
- Lekcja: `context/foundation/lessons.md:9`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: API podsumowania

#### Automated

- [x] 1.1 Migracje czyste (brak nowych, ale baza startuje) — 531e56d
- [x] 1.2 Testy store Summary (SUM, filtry, izolacja) — 531e56d
- [x] 1.3 Testy handlera (200/400/401) — 531e56d
- [x] 1.4 Cały backend zielony + gofmt pusty — 531e56d

#### Manual

- [x] 1.5 curl zwraca sumy zgodne z transactions — 531e56d
- [x] 1.6 Złe filtry dają 400, brak sesji 401 — 531e56d

### Phase 2: Ekran podsumowania

#### Automated

- [x] 2.1 Build frontu przechodzi
- [x] 2.2 Backend nadal zielony

#### Manual

- [x] 2.3 US-01: wydatek widoczny w Summary po filtrach
- [x] 2.4 Presety miesiąc/rok/custom poprawne
- [x] 2.5 Multi-select i wiersz Razem zgodne
- [x] 2.6 Pusty miesiąc daje zera + odpowiedź < 2s
