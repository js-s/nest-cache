# Wprowadzenie wydatku i lista operacji — Implementation Plan

## Overview

Dodajemy pierwszy pełny pion finansowy (roadmapa S-04): użytkownik zapisuje wydatek (kwota + kategoria + data + opcjonalny opis) przez formularz webowy, żądanie trafia przez API do bazy, a zapisany wydatek pojawia się na paginowanej liście własnych operacji. To odblokowuje S-05 (filtrowane podsumowanie).

## Current State Analysis

Działa rejestracja/logowanie/długa sesja (S-01), izolacja konta (`account.RequireAccount`) i kategorie z podkategoriami w `resources/categories.yaml` → tabela `categories` (S-03). Baza ma `users`, `sessions`, `categories`. Nie istnieje żadna tabela ani kod transakcji.

Co trzeba dobudować:

- tabela `transactions` z właścicielem `user_id`, `kind`, dwiema cyframi po kropce i walidacją,
- store: atomiczny zapis wydatku z walidacją przynależności i typu kategorii; paginowany odczyt,
- 2 endpointy API (`POST`/`GET /api/transactions`) wpięte jak kategorie,
- nowy ekran `/operations` z formularzem i infinite listą, plus funkcje w kliencie API.

### Key Discoveries:

- `cmd/nest-cash/spa.go:19-33` — trasy API przez `mux.Handle("GET /api/...", account.RequireAccount(a.resolver, ...))`; catch-all `/api/` łapie resztę.
- `internal/account/middleware.go:30-40` — właściciel zawsze z kontekstu (`account.AccountIDFromContext`), nigdy z treści żądania.
- `internal/categories/store.go:105-124` — wzorzec store: walidacja wejścia, `ON CONFLICT DO NOTHING`, `fmt.Errorf("categories: <op>: %w", err)` (lekcja `context/foundation/lessons.md`).
- `internal/categories/handler.go:78-88` — `http.MaxBytesReader` 4KB, trim, walidacja długości runami, `writeInvalid/writeConflict`.
- `internal/db/migrate.go:12-67` — nowa migracja = `internal/db/migrations/0004_*.sql`, idempotentna, alfabetycznie po nazwie.
- `internal/categories/handler_test.go:14-24` i `store_test.go:15-50` — testy handlerów wstrzykują `account.WithAccountID`; testy store `t.Skip` bez `DATABASE_URL` i tworzą użytkownika z `t.Cleanup`.
- `web/src/api/client.ts:20-34` — jedno `request<T>`, cookie `same-origin`; `ApiError(status, code)`.
- `web/src/views/Categories.vue` — wzorzec ekranu: `ref` + `busy/loading/error` + `message(e)`; klasy CSS `card/stack/field/table/btn/alert` z `web/src/style.css`.
- `web/src/router.ts:14-15` — nowy ekran = trasa z `meta: { requiresAuth: true }`.

## Desired End State

Po zalogowaniu użytkownik otwiera `/operations`, wybiera kategorię wydatku, wpisuje kwotę (domyślna data = dziś), opcjonalny opis i zapisuje. Wydatek natychmiast pojawia się na liście na tej samej stronie. Lista pokazuje datę, kategorię (grupa → podkategoria), opis i kwotę, posortowana od najnowszych, z „Wczytaj więcej”. Po odświeżeniu dane zostają. Konto B nie widzi operacji konta A. Kwoty liczą się dokładnie (bez float).

Weryfikacja: lokalna baza + `go test ./...` z `DATABASE_URL`, `npm --prefix web run build`, ręczny przepływ w przeglądarce.

### Key Discoveries:

- Kwota jako `NUMERIC(12,2)` przechodzi przez Go jako `string` (skan i insert), więc JSON też niesie `string` — brak konwersji na float po drodze.
- Walidacja właściciela i typu kategorii jest atomowa: jeden `INSERT ... SELECT` z `categories WHERE id=$ AND user_id=$ AND kind='expense'`; brak wiersza = 400 (nie trzeba osobnego lookupu + race).
- Infinite scroll implementujemy na offsetach (`page`/`limit`/`total`) — najprostszy kontrakt obsługujący „Wczytaj więcej” bez kursora.

## What We're NOT Doing

- Edycja/usuwanie operacji (FR-008 — v2).
- Formularz przychodu i wskaźnik budżetowy (S-06).
- Filtrowanie listy po okresie/kategorii (S-05).
- Wiele walut, pole `currency`, konwersje kursowe.
- Godzina transakcji, strefy czasowe, znaczniki audytowe poza `created_at`.
- Osobne ekrany / osobny stan na formularz i listę.
- Testy E2E (Playwright) — brak runnera; gate frontendu = `npm --prefix web run build`.

## Implementation Approach

Ta sama kolejność i wzorce co w S-03: najpierw migracja + store (walidacja i izolacja w SQL), potem cienki handler na wzór `categories`, na końcu ekran kopiujący wzorzec `Categories.vue`. Schemat od razu niesie `kind`, więc S-06 dołoży przychód bez zmiany kontraktu. Kwoty trzymamy jako string od formularza do Postgresa.

## Critical Implementation Details

- **Pieniądze** — żadnego `float`/`ParseFloat` na kwocie. Waliduj kształt stringa regexem, potem przekaż string do Postgresa; przy odczycie skanuj `NUMERIC` do `string`. To jedyne miejsce, gdzie błąd zaokrągleń byłby kosztowny w S-05/S-06.
- **Walidacja kategorii** — użyj `INSERT ... SELECT` z warunkami `user_id` i `kind='expense'`; `sql.ErrNoRows` oznacza „kategoria nie istnieje / nie twoja / nie wydatkowa” → 400. Nie robić osobnego `SELECT` przed `INSERT` (race + więcej kodu).
- **State sequencing (frontend)** — po udanym zapisie przeładuj pierwszą stronę listy (reset `items` i `page`), a nie dopisuj na początek: data może być starsza niż ostatni wpis, więc dopisywanie łamie sortowanie od najnowszych.
- **Infinite scroll** — utrzymuj `page`, `total` i `loading`; ukryj „Wczytaj więcej”, gdy `items.length >= total`. Przy błędzie kolejnej strony nie czyść już wczytanych elementów.

## Phase 1: Baza i store

### Overview

Tabela `transactions` + store z atomicznym zapisem i paginowanym odczytem. Bez tego API nie ma na czym pracować.

### Changes Required:

#### 1. Migracja transakcji

**File**: `internal/db/migrations/0004_transactions.sql` (nowy)

**Intent**: Tabela operacji z właścicielem, typem, kwotą `NUMERIC(12,2)`, datą `DATE` i opcjonalnym opisem. Indeksy pod najczęstsze zapytanie: filtr po właścicielu + sortowanie po dacie.

**Contract**: `transactions(id UUID PK default gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, category_id UUID NOT NULL REFERENCES categories(id), kind TEXT NOT NULL CHECK (kind IN ('expense','income')), amount NUMERIC(12,2) NOT NULL CHECK (amount > 0), occurred_on DATE NOT NULL, description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500), created_at TIMESTAMPTZ NOT NULL DEFAULT now())`. Indeks: `(user_id, occurred_on DESC, created_at DESC)`. Wszystko `IF NOT EXISTS` zgodnie z `migrate.go`.

#### 2. Store transakcji

**File**: `internal/transactions/store.go` (nowy pakiet `transactions`)

**Intent**: `Create` zapisuje wydatek użytkownika, walidując kwotę/datę/opis i przynależność kategorii w jednym zapytaniu; `List` zwraca stronę operacji z totalem i nazwami kategorii do wyświetlenia breadcrumbu.

**Contract**:
- `const MaxDescriptionLen = 500`, `const MaxAmount = "9999999999.99"` (górna granica 12,2).
- `type Transaction struct { ID, Kind, CategoryID, CategoryName string; ParentName string; Amount string; OccurredOn time.Time; Description string; CreatedAt time.Time }`.
- `func (s *Store) Create(ctx, userID, categoryID, amount, occurredOn, description string) (Transaction, bool, error)` — `ok=false` gdy kategoria obca/nieistniejąca/nie-`expense` lub kwota poza zakresem; błąd tylko dla faktycznej awarii DB. Insert jako `INSERT INTO transactions (...) SELECT $1, c.id, 'expense', $2::numeric, $3::date, $4 FROM categories c WHERE c.id = $5 AND c.user_id = $1 AND c.kind = 'expense' RETURNING id, ...`. Walidacja kwoty: regex `^\d{1,10}(\.\d{1,2})?$` i odrzucenie wartości `0.00`; data przez `time.Parse("2006-01-02", ...)`; opis trimowany do `MaxDescriptionLen`.
- `func (s *Store) List(ctx, userID string, page, limit int) ([]Transaction, int, error)` — zwraca stronę i `total`. Zapytanie: `LEFT JOIN categories c ON c.id = t.category_id LEFT JOIN categories p ON p.id = c.parent_id`, `WHERE t.user_id = $1`, `ORDER BY t.occurred_on DESC, t.created_at DESC`, `LIMIT $2 OFFSET $3`; `total` osobnym `COUNT(*)` dla tego samego `user_id`.
- Wszystkie błędy DB opakowane `fmt.Errorf("transactions: <op>: %w", err)`.

### Success Criteria:

#### Automated Verification:

- Kompilacja: `go build ./...`
- Testy z lokalną bazą przechodzą: `DATABASE_URL=... go test ./internal/transactions/...`
- Testy całości przechodzą: `DATABASE_URL=... go test ./...`
- Migracja aplikuje się idempotentnie (dwa uruchomienia bez błędu).

#### Manual Verification:

- W `psql` widać tabelę `transactions` z indeksem i constraintami (`amount > 0`, `kind`, długość opisu).

**Implementation Note**: Po fazie i przejściu testów automatycznych zatrzymaj się po ręczne potwierdzenie, że migracja i store działają na lokalnej bazie, zanim przejdziesz do fazy 2.

---

## Phase 2: API i trasy

### Overview

Dwa endpointy w stylu kategorii: `POST` tworzy wydatek, `GET` zwraca paginowaną listę operacji zalogowanego konta.

### Changes Required:

#### 1. Handler transakcji

**File**: `internal/transactions/handler.go` (nowy)

**Intent**: Cienka warstwa HTTP: odczyt właściciela z kontekstu, dekodowanie wejścia z limitem rozmiaru, walidacja, mapowanie na kody 201/400/401/405/409/500. Kwota i data przechodzą jako stringi.

**Contract**:
- DTO odpowiedzi: `{ id, amount (string), kind, category_id, category_name, parent_name?, occurred_on (YYYY-MM-DD), description }`. `parent_name` tylko gdy kategoria ma rodzica.
- `POST` body: `{ amount: string, category_id: string, occurred_on: string, description?: string }`; sukces → 201 z DTO. Walidacja: `MaxBytesReader` 4KB, `amount` i `category_id` niepuste, `occurred_on` w formacie `2006-01-02`.
- Store `Create` sygnalizuje odrzucenie wejścia jako `ok=false` (bez błędu); handler mapuje `ok=false` → 400, a `err` → 500. Nie traktować `ok=false` jako sukcesu.
- `GET /api/transactions?page=&limit=` — `page` domyślnie 1 (min 1), `limit` domyślnie 20 (max 100); odpowiedź `{ items: [...], page, limit, total }`.
- Brak konta w kontekście → 401; zła metoda → 405 (wzorzec `categories/handler.go`).

#### 2. Wpięcie tras

**File**: `cmd/nest-cash/spa.go`, `cmd/nest-cash/main.go`

**Intent**: Zarejestrować handler w `application` i podpiąć trasy za `account.RequireAccount`, wzorem kategorii.

**Contract**: pole `transactions *transactions.Handler` w `application`; w `main.go` inicjalizacja `transactions.NewHandler(transactions.NewStore(db))` w bloku `if db != nil`; w `routes()`:
`mux.Handle("GET /api/transactions", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.List)))` oraz to samo dla `POST`.

### Success Criteria:

#### Automated Verification:

- Kompilacja: `go build ./...`
- Testy handlerów przechodzą: `DATABASE_URL=... go test ./internal/transactions/...`
- Testy całości przechodzą: `DATABASE_URL=... go test ./...`

#### Manual Verification:

- `curl` bez ciasteczka → 401; z sesją poprawny `POST` → 201 i wiersz w `GET`.
- `POST` z obcą/nieistniejącą kategorią lub `kind=income` → 400; `amount` ≤ 0 lub poza formatem → 400.
- `GET ?page=1&limit=2` zwraca ≤ 2 elementy i poprawne `total`.

**Implementation Note**: Zatrzymaj się po ręczne potwierdzenie `curl`-em, zanim przejdziesz do fazy 3.

---

## Phase 3: Frontend

### Overview

Klient API rozszerzony o tworzenie i listowanie operacji; ekran `/operations` z formularzem u góry, infinite listą pod nim i domyślną kategorią z `localStorage`.

### Changes Required:

#### 1. Klient API

**File**: `web/src/api/client.ts`

**Intent**: Typy i funkcje dla transakcji, korzystające z istniejącego `request<T>` i `ApiError`.

**Contract**: `interface Transaction { id; amount: string; kind: 'expense'|'income'; category_id; category_name; parent_name?: string; occurred_on: string; description: string }`; `createTransaction(input: { amount: string; category_id: string; occurred_on: string; description?: string }): Promise<Transaction>`; `listTransactions(page: number, limit?: number): Promise<{ items: Transaction[]; page: number; limit: number; total: number }>`.

#### 2. Ekran operacji

**File**: `web/src/views/Operations.vue` (nowy)

**Intent**: Formularz wydatku (kategoria, kwota, data domyślnie dziś, opis) + lista operacji z „Wczytaj więcej”. Po sukcesie przeładuj pierwszą stronę. Zapamiętaj wybraną kategorię w `localStorage` i podpowiedz przy następnym wejściu. Gdy brak kategorii wydatków — pokaż odnośnik do `/categories` i wyłącz zapis.

**Contract**:
- Ładowanie kategorii przez `listCategories()`, wybór grupowany `<optgroup>` po grupie; dozwolone grupy expense bez dzieci oraz dzieci (grupy income pomijamy).
- Data: `<input type="date">`, domyślnie `new Date().toISOString().slice(0,10)`; kwota `<input type="number" min="0.01" step="0.01">`, do API wysyłana jako string.
- `localStorage` klucz `nestcash:last-category`; przy `onMounted` wybierz zapamiętaną kategorię, jeśli nadal istnieje w liście.
- Infinite scroll: `page`, `total`, `loading`; `loadMore()` dociąga kolejną stronę i dopisuje do `items`; „Wczytaj więcej” ukryte gdy `items.length >= total`.
- Błędy mapowane przez `ApiError` (400 → komunikat walidacji, 401 → przekierowanie przez router jak dotychczas).

#### 3. Trasa i nawigacja

**File**: `web/src/router.ts`, `web/src/views/Summary.vue`

**Intent**: Dodać trasę `/operations` z `meta: { requiresAuth: true }` i link z podsumowania.

**Contract**: `{ path: '/operations', component: Operations, meta: { requiresAuth: true } }`; w `Summary.vue` link `<router-link to="/operations">Operations</router-link>` obok istniejącego linku do kategorii.

### Success Criteria:

#### Automated Verification:

- Build + typecheck przechodzą: `npm --prefix web run build`

#### Manual Verification:

- Po zalogowaniu `/operations` pokazuje formularz i pustą listę (lub istniejące operacje).
- Zapis wydatku natychmiast pokazuje go na górze listy; po odświeżeniu strony nadal jest.
- „Wczytaj więcej” dociąga kolejną stronę i nie duplikuje elementów.
- Wybrana kategoria jest domyślnie ustawiona przy kolejnym wejściu (localStorage).
- Konto B nie widzi operacji konta A.
- Brak kategorii wydatków → formularz zablokowany z odnośnikiem do `/categories`.

**Implementation Note**: Po fazie zatrzymaj się po ręczne potwierdzenie pełnego przepływu w przeglądarce.

---

## Testing Strategy

### Unit/Integration Tests (Go):

- `internal/transactions/store_test.go` — realne Postgres, `t.Skip` bez `DATABASE_URL` (wzorzec `categories/store_test.go`): zapis poprawny; kwota ≤ 0 / zły format → odrzucone; kategoria obca lub `kind=income` → `ok=false`; izolacja między użytkownikami (`List` B nie zwraca A); paginacja zwraca poprawne `total` i kolejne strony bez nakładania.
- `internal/transactions/handler_test.go` — `account.WithAccountID` (wzorzec `categories/handler_test.go`): 201 poprawny; 400 dla złego JSON/kwoty/kategorii; 401 bez konta; 405 zła metoda; `Cache-Control: no-store`.

### Manual Testing Steps:

1. Zaloguj się, dodaj wydatek (kwota 12.34, kategoria, dziś, opis), potwierdź obecność na górze listy.
2. Odśwież stronę — wydatek nadal widoczny.
3. Dodaj > 20 wydatków (lub obniż `limit`), sprawdź „Wczytaj więcej” bez duplikatów.
4. Wyjdź i wróć na `/operations` — domyślna kategoria nadal ustawiona.
5. Zaloguj się na drugie konto — brak operacji pierwszego.

## Performance Considerations

Single-user, niska skala. Paginacja offsetowa wystarcza; indeks `(user_id, occurred_on DESC, created_at DESC)` pokrywa sortowanie i filtr właściciela. `COUNT(*)` per żądanie jest tanie przy małej objętości — `ponytail:` komentarz w store, ulepszyć dopiero gdy liczba wierszy to uzasadni.

## Migration Notes

Nowa, wyłącznie dodająca migracja `0004_transactions.sql`; brak zmian w istniejących tabelach. Rollback nie jest wymagany w MVP — tabela pusta do pierwszego zapisu. `category_id` ma FK do `categories` bez `ON DELETE`, bo usuwanie kategorii jest poza MVP (FR-004).

## References

- PRD: `context/foundation/prd.md` (US-01, FR-005, FR-007, NFR prywatności)
- Roadmapa: `context/foundation/roadmap.md` (S-04)
- Wzorzec store/handler: `internal/categories/store.go`, `internal/categories/handler.go`
- Wzorzec ekranu: `web/src/views/Categories.vue`
- Lekcja: `context/foundation/lessons.md:5` (opakowywanie błędów DB)

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Baza i store

#### Automated

- [x] 1.1 Kompilacja: `go build ./...` — b7a46df
- [x] 1.2 Testy z lokalną bazą przechodzą: `DATABASE_URL=... go test ./internal/transactions/...` — b7a46df
- [x] 1.3 Testy całości przechodzą: `DATABASE_URL=... go test ./...` — b7a46df
- [x] 1.4 Migracja aplikuje się idempotentnie (dwa uruchomienia bez błędu). — b7a46df

#### Manual

- [x] 1.5 W `psql` widać tabelę `transactions` z indeksem i constraintami. — b7a46df

### Phase 2: API i trasy

#### Automated

- [ ] 2.1 Kompilacja: `go build ./...`
- [ ] 2.2 Testy handlerów przechodzą: `DATABASE_URL=... go test ./internal/transactions/...`
- [ ] 2.3 Testy całości przechodzą: `DATABASE_URL=... go test ./...`

#### Manual

- [ ] 2.4 `curl` bez ciasteczka → 401; z sesją poprawny `POST` → 201 i wiersz w `GET`.
- [ ] 2.5 `POST` z obcą kategoria/`kind=income` → 400; `amount` ≤ 0 lub poza formatem → 400.
- [ ] 2.6 `GET ?page=1&limit=2` zwraca ≤ 2 elementy i poprawne `total`.

### Phase 3: Frontend

#### Automated

- [ ] 3.1 Build + typecheck przechodzą: `npm --prefix web run build`

#### Manual

- [ ] 3.2 `/operations` pokazuje formularz i listę po zalogowaniu.
- [ ] 3.3 Zapis wydatku natychmiast pokazuje go na górze; po odświeżeniu nadal jest.
- [ ] 3.4 „Wczytaj więcej” dociąga kolejną stronę bez duplikatów.
- [ ] 3.5 Wybrana kategoria domyślnie ustawiona przy kolejnym wejściu (localStorage).
- [ ] 3.6 Konto B nie widzi operacji konta A.
- [ ] 3.7 Brak kategorii wydatków → formularz zablokowany z odnośnikiem do `/categories`.
