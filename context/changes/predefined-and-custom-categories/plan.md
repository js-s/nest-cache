# Kategorie predefiniowane i własne — Implementation Plan

## Overview

Dodajemy kategorie z podkategoriami (2 poziomy): startowa lista z `resources/categories.yaml` kopiowana na konto użytkownika + ekran do przeglądania i dodawania własnych. To odblokowuje S-04 (wpis wydatku).

## Current State Analysis

Dziś nie ma nic z kategorii w kodzie — tylko plik `resources/categories.yaml` i wpis w roadmapie. Auth i sesja działają (S-01 done), izolacja konta działa przez `account.RequireAccount`. Baza ma tylko `users` + `sessions`.

Co trzeba dobudować:
- nowa tabela na grupy i podkategorie z właścicielem `user_id`,
- kopiowanie startowych przy pierwszym użyciu,
- 2 endpointy API + 1 ekran `Categories.vue`.

## Desired End State

Po zalogowaniu użytkownik otwiera `/categories` i widzi grupy (np. „Codzienne”) z podkategoriami (np. „Artykuły spożywcze”). Może dodać własną grupę i podkategorię przez prosty formularz. Po odświeżeniu strony dane zostają. Cudzych kategorii nie widać.

### Key Discoveries:

- `cmd/nest-cash/spa.go:19-28` — nowe trasy API wpina się przez `mux.Handle("GET /api/...", account.RequireAccount(...))`, dłuższe ścieżki wygrywają z catch-all `/api/`.
- `internal/account/middleware.go:18-40` — właściciel zawsze z kontekstu (`AccountIDFromContext`), nigdy z treści żądania.
- `internal/db/migrate.go:12-67` — nowa migracja = plik `internal/db/migrations/0003_*.sql` z `IF NOT EXISTS`, kolejność po nazwie.
- `web/src/api/client.ts:20-34` — wszystkie wywołania API idą przez `request<T>`, sesja to ciasteczko (`credentials: same-origin`).
- `web/src/router.ts:20-25` — nowy ekran = wpis z `meta: { requiresAuth: true }`.
- `context/foundation/lessons.md:9` — każdy błąd bazy opakowujemy `fmt.Errorf("categories: <op>: %w", err)`.

## What We're NOT Doing

- Edycja / usuwanie / wyłączanie kategorii (FR-004, v2).
- Ikony, kolory, sortowanie drag-and-drop.
- Wspólna globalna tabela kategorii — każdy ma własną kopię.
- Użycie kategorii w formularzu wydatku (to S-04).
- Testy klikania w przeglądarce (Playwright) — tylko testy Go + ręczne sprawdzenie ekranu.

## Implementation Approach

Najpierw baza (migracja + zapis/odczyt), potem cienkie API na tym samym wzorze co auth, na końcu ekran kopiujący wzór `Login.vue` (stan `ref` + `busy/error` + `ApiError`). Seed startowych robimy leniwie przy pierwszym `GET` (jeśli użytkownik ma 0 wierszy → kopiuj z YAML), żeby nie ruszać rejestracji.

## Critical Implementation Details

- **Timing & lifecycle** — seed musi być idempotentny: dwa równoległe `GET` nie mogą zdublować kategorii. Rozwiązanie: kopiowanie w jednej transakcji z `ON CONFLICT DO NOTHING` na unikalności `(user_id, lower(name), parent)`.
- **User experience spec** — formularz ma 3 pola: typ (wydatek/przychód), grupa (wybór z listy lub nowa nazwa), nazwa podkategorii. Błąd walidacji pokazuje się jako czerwony pasek nad formularzem, nie znika wpisana treść.
- **State sequencing** — ekran najpierw woła `GET`, potem dopiero pokazuje formularz. Inaczej użytkownik doda kategorię do pustej listy typów.

## Phase 1: Baza i startowe kopiowanie

### Overview

Tabela + funkcje zapisu/odczytu + kopiowanie z YAML. Bez tego API nie ma na czym działać.

### Changes Required:

#### 1. Migracja kategorii

**File**: `internal/db/migrations/0003_categories.sql` (nowy)

**Intent**: Jedna tabela na grupy i podkategorie. Grupa ma `parent_id = NULL`, podkategoria wskazuje na grupę.

**Contract**: Kolumny: `id UUID PK gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, parent_id UUID NULL REFERENCES categories(id) ON DELETE CASCADE, kind TEXT NOT NULL (expense|income), name TEXT NOT NULL, created_at`. Unikalność: `UNIQUE(user_id, parent_id NULLS NOT DISTINCT, lower(name))` lub równoważny indeks + `CREATE INDEX ON categories(user_id)`. Wszystko `IF NOT EXISTS`.

#### 2. Pakiet categories (store + seed)

**File**: `internal/categories/store.go` (nowy), `internal/categories/seed.go` (nowy), `internal/categories/*_test.go` (nowy)

**Intent**: Zapis/odczyt tylko w obrębie `user_id` + funkcja `EnsureSeeded(ctx, userID)` kopiująca `resources/categories.yaml` (wydatki grupowane, przychody płaskie — każda pozycja przychodu to grupa bez dzieci albo grupa+1 dziecko o tej samej nazwie; do ustalenia przy implementacji, udokumentować wybór w kodzie).

**Contract**: Metody: `List(ctx, userID)`, `CreateGroup(ctx, userID, kind, name)`, `CreateSubcategory(ctx, userID, parentID, name)`. YAML wczytany przez `embed` + `gopkg.in/yaml.v3` (już w go.mod? jeśli nie — użyć prostego parsera lub dodać zależność, preferowane: sprawdzić go.mod). Błędy opakowane `fmt.Errorf("categories: <op>: %w", err)`.

### Success Criteria:

#### Automated Verification:

- Migracja nakłada się czysto: `go test ./internal/db/...`
- Testy pakietu przechodzą: `go test ./internal/categories/...`
- Cały backend zielony: `go test ./...`
- Formatowanie: `gofmt -l internal/categories cmd`

#### Manual Verification:

- Podgląd SQL po migracji pokazuje tabelę `categories`
- Plik YAML ładuje się bez błędu (ręczny test seed na lokalnej bazie)

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase. Phase blocks use plain bullets — the corresponding `- [ ]` checkboxes for these items live in the `## Progress` section at the bottom of the plan.

---

## Phase 2: API kategorii

### Overview

Dwa endpointy z izolacją konta: podgląd drzewa + dodawanie. Thin handler, cała logika w store z Fazy 1.

### Changes Required:

#### 1. Handler kategorii

**File**: `internal/categories/handler.go` (nowy)

**Intent**: `GET /api/categories` zwraca drzewo grup z dziećmi dla zalogowanego; `POST /api/categories` tworzy grupę lub podkategorię po walidacji.

**Contract**: `GET` → `200 [{id, kind, name, children: [{id, name}]}]`, przed listą woła `EnsureSeeded`. `POST {kind, name, parent_id?}` → `201` lub błędy `400 invalid_request` (pusta nazwa, >80 znaków, zły kind, parent z innego typu/konta), `401 unauthorized` (brak konta), `409 conflict` (duplikat). Limit ciała `MaxBytesReader 4KB`, odpowiedzi z `Cache-Control: no-store`, koperta błędu `{"error":"..."}` jak w auth.

#### 2. Wpięcie tras

**File**: `cmd/nest-cash/spa.go`, `cmd/nest-cash/main.go`

**Intent**: Wystawić endpointy tylko gdy baza + resolver istnieją (ten sam `if` co auth).

**Contract**: `mux.Handle("GET /api/categories", account.RequireAccount(a.resolver, ...))` i `mux.Handle("POST /api/categories", ...)`. Handler kategorii trzymany w `application` obok `auth` (nowe pole + konstrukcja w `main.go:67-75`).

### Success Criteria:

#### Automated Verification:

- Testy handlera (httptest + RequireAccount): `go test ./internal/categories/...`
- Niezalogowany dostaje 401: pokryte testem
- Użytkownik B nie widzi kategorii A: pokryte testem
- Całość: `go test ./...` i `go vet ./...`

#### Manual Verification:

- `curl` z ciasteczkiem: GET zwraca grupy z YAML, POST dodaje własną, duplikat zwraca 409 z polskim komunikatem na ekranie w Fazie 3
- Bez ciasteczka: 401

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase. Phase blocks use plain bullets — the corresponding `- [ ]` checkboxes for these items live in the `## Progress` section at the bottom of the plan.

---

## Phase 3: Ekran Kategorie w przeglądarce

### Overview

Strona `/categories`: lista grup z podkategoriami + formularz dodawania. Kopiuje istniejące wzory, zero nowych bibliotek. (Nawigacja = linki u góry strony; formularz = pola + przycisk Zapisz.)

### Changes Required:

#### 1. Klient API

**File**: `web/src/api/client.ts`

**Intent**: Typy + 2 funkcje wołające nowe endpointy.

**Contract**: `export interface CategoryGroup { id, kind: 'expense'|'income', name, children: {id, name}[] }`, `listCategories(): Promise<CategoryGroup[]>` → `request('/categories')`, `createCategory(input: {kind, name, parent_id?})`.

#### 2. Ekran i trasa

**File**: `web/src/views/Categories.vue` (nowy), `web/src/router.ts`

**Intent**: Lista (tabela `.table`, style z `style.css`) + formularz (typ select, grupa select + „nowa…”, nazwa input). Stan jak w `Login.vue`: `ref` na dane/błędy/`busy`, ładowanie w `onMounted`, błąd łapany jako `ApiError` i pokazany w `.alert-error`.

**Contract**: Trasa `{ path: '/categories', component: Categories, meta: { requiresAuth: true } }`. Bez zmian w `App.vue`, `auth.ts`, `style.css`. Link do `/categories` w nawigacji (jeśli istnieje wspólny pasek — inaczej przycisk w `Summary.vue`).

### Success Criteria:

#### Automated Verification:

- Budowanie frontendu: `npm --prefix web run build`
- Brak nowych błędów TypeScript (`vue-tsc` w build)

#### Manual Verification:

- Po zalogowaniu `/categories` pokazuje startowe grupy z YAML
- Dodanie własnej grupy i podkategorii działa i widać je po odświeżeniu (F5)
- Pusta nazwa i duplikat pokazują czerwony komunikat, wpisana treść nie znika
- Niezalogowany wchodzi na `/categories` → ląduje na `/login`

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase. Phase blocks use plain bullets — the corresponding `- [ ]` checkboxes for these items live in the `## Progress` section at the bottom of the plan.

---

## Testing Strategy

### Unit Tests:

- Store: seed idempotentny (2x EnsureSeeded → ta sama liczba wierszy), duplikat grupy i podkategorii odrzucony, `List` zwraca tylko wiersze danego `user_id`
- Handler: 401 bez sesji, 409 duplikat, 400 pusta/długa nazwa, parent z innego konta odrzucony

### Integration Tests:

- `GET` na świeżym koncie zwraca drzewo z YAML; `POST` + `GET` pokazuje nową pozycję
- Istniejące `go test ./...` pozostaje zielone

### Manual Testing Steps:

1. Zarejestruj nowe konto → otwórz `/categories` → widzisz grupy startowe
2. Dodaj grupę „Ogród” (wydatek) + podkategorię „Nasiona” → widać po F5
3. Spróbuj duplikat „Ogród” → czerwony komunikat
4. Zaloguj drugim kontem → nie widzisz kategorii pierwszego

## Performance Considerations

Mała skala (jeden użytkownik, <200 kategorii). `List` jednym zapytaniem + złożenie drzewa w Go. Brak paginacji, brak cache. Odpowiedź < 2s per NFR.

## Migration Notes

Migracja `0003` tylko dodaje tabelę — istniejące konta (users/sessions) nietknięte. Seed leniwy przy pierwszym GET, więc starzy użytkownicy dostaną startowe przy pierwszym wejściu na ekran. Rollback: `DROP TABLE IF EXISTS categories`.

## References

- Lista startowa: `resources/categories.yaml`
- PRD: `context/foundation/prd.md:62-64` (FR-003 must-have, FR-004 v2)
- Roadmap: `context/foundation/roadmap.md:99-109` (S-03, hierarchia)
- Wzory tras: `cmd/nest-cash/spa.go:19-28`
- Wzorzec klienta: `web/src/api/client.ts:20-34`
- Wzorzec ekranu: `web/src/views/Summary.vue:1-13`, `web/src/views/Login.vue:17-36`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Baza i startowe kopiowanie

#### Automated

- [x] 1.1 Migracja nakłada się czysto: `go test ./internal/db/...` — 48a63b5
- [x] 1.2 Testy pakietu przechodzą: `go test ./internal/categories/...` — 48a63b5
- [x] 1.3 Cały backend zielony: `go test ./...` — 48a63b5
- [x] 1.4 Formatowanie: `gofmt -l internal/categories cmd` — 48a63b5

#### Manual

- [ ] 1.5 Podgląd SQL po migracji pokazuje tabelę `categories`
- [ ] 1.6 Plik YAML ładuje się bez błędu (ręczny test seed na lokalnej bazie)

### Phase 2: API kategorii

#### Automated

- [x] 2.1 Testy handlera (httptest + RequireAccount): `go test ./internal/categories/...`
- [x] 2.2 Niezalogowany dostaje 401: pokryte testem
- [x] 2.3 Użytkownik B nie widzi kategorii A: pokryte testem
- [x] 2.4 Całość: `go test ./...` i `go vet ./...`

#### Manual

- [ ] 2.5 `curl` z ciasteczkiem: GET zwraca grupy z YAML, POST dodaje własną, duplikat zwraca 409 z polskim komunikatem na ekranie w Fazie 3
- [ ] 2.6 Bez ciasteczka: 401

### Phase 3: Ekran Kategorie w przeglądarce

#### Automated

- [ ] 3.1 Budowanie frontendu: `npm --prefix web run build`
- [ ] 3.2 Brak nowych błędów TypeScript (`vue-tsc` w build)

#### Manual

- [ ] 3.3 Po zalogowaniu `/categories` pokazuje startowe grupy z YAML
- [ ] 3.4 Dodanie własnej grupy i podkategorii działa i widać je po odświeżeniu (F5)
- [ ] 3.5 Pusta nazwa i duplikat pokazują czerwony komunikat, wpisana treść nie znika
- [ ] 3.6 Niezalogowany wchodzi na `/categories` → ląduje na `/login`
