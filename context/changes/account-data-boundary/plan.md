# Minimalny kontrakt własności danych — Implementation Plan

## Overview

F-01 doda provider-neutralny kontrakt właściciela danych, który późniejsza warstwa sesji będzie mogła zasilić identyfikatorem konta. Kontrakt obejmie typ `AccountID`, bezpieczne helpers dla `context.Context` oraz middleware odrzucające żądania bez poprawnej tożsamości.

Zmiana pozostaje celowo mniejsza niż pełny system auth: nie dodaje rejestracji, logowania, sesji, tabel, migracji ani endpointu domenowego. Ma przygotować granicę, którą S-01 zasili prawdziwym resolverem sesji, a kolejne slice’y wykorzystają przy każdym zapytaniu do danych konta.

## Current State Analysis

Repozytorium zawiera minimalny serwer Go w `cmd/nest-cash/main.go`: publiczne `GET /`, publiczne `GET /healthz`, opcjonalne połączenie z PostgreSQL oraz wspólny zapis JSON. Nie ma jeszcze `internal/`, middleware, auth, modelu konta, migracji, tabel ani store’a domenowego (`cmd/nest-cash/main.go:16-47`, `cmd/nest-cash/main.go:62-94`).

Wymagania PRD nakazują prywatność danych finansowych, brak plaintextowych haseł i bezterminową retencję do czasu usunięcia konta (`context/foundation/prd.md:35-38`, `context/foundation/prd.md:80-85`). Roadmapa definiuje F-01 jako minimalną granicę identyfikacji właściciela i odrzucania dostępu poza kontem, bez budowania pełnego auth (`context/foundation/roadmap.md:52-63`).

Obecny kontrakt wdrożeniowy rezerwuje `SESSION_SECRET`, ale kod go jeszcze nie odczytuje; jego użycie należy do S-01 (`context/deployment/deploy-plan.md:148-175`). Istniejący `/healthz` musi pozostać publiczny, aby nie zepsuć smoke checka wdrożenia.

Testy obecnie używają bezpośredniego wywołania handlera i `httptest` (`cmd/nest-cash/main_test.go:8-19`). Baseline `go test ./...` nie został lokalnie rozstrzygnięty, ponieważ pobranie `github.com/jackc/pgx/v5` zakończyło się błędem proxy (`Forbidden`/`EOF`); nie jest to błąd funkcjonalny F-01.

## Desired End State

Pakiet `internal/account` definiuje jedną, małą granicę właściciela:

- `AccountID` jest typowanym, opaque stringiem; pusty lub whitespace-only identyfikator jest nieważny.
- Context helpers przechowują i odczytują wyłącznie poprawny `AccountID` przez prywatny typ klucza.
- `Resolver` jest interfejsem, który przyszły adapter sesji zaimplementuje w S-01.
- `RequireAccount` pobiera właściciela z resolvera, waliduje go, zapisuje w request context i przed wywołaniem handlera odrzuca brak lub błędną tożsamość jako JSON `401`.

Istniejące `/` i `/healthz` nie są opakowane middleware’em. Nie powstaje produkcyjny endpoint testowy ani źródło tożsamości oparte na nagłówku lub parametrze żądania. Przyszłe operacje na danych konta będą przyjmować `AccountID` jawnie i ograniczać zapytania po właścicielu; test dwóch właścicieli z realnym PostgreSQL pozostaje częścią pierwszego slice’a z danymi.

### Key Discoveries:

- `cmd/nest-cash/main.go:40-47` rejestruje wyłącznie publiczne trasy, więc F-01 nie ma jeszcze bezpiecznej trasy domenowej do opakowania.
- `cmd/nest-cash/main.go:62-87` używa request contextu tylko do timeoutu DB; nie istnieje jeszcze context właściciela.
- `go.mod:1-18` zawiera Go 1.22.2 i tylko sterownik PostgreSQL jako główną zależność; kontrakt powinien użyć standardowej biblioteki.
- `context/foundation/roadmap.md:57-63` wyraźnie ogranicza fundament do granicy własności i odsuwa pełny auth do S-01.
- `context/foundation/prd.md:103-112` wyklucza role i współdzielenie danych w MVP; nie należy projektować wieloużytkownikowej autoryzacji.
- `AGENTS.md:7-9` opisuje stan pre-scaffoldowy i jest nieaktualny względem kodu oraz roadmapy; aktualny baseline repozytorium jest źródłem prawdy dla planu.

## What We're NOT Doing

- Rejestracji, logowania, hashowania haseł, cookies, tokenów, resetu haseł ani rotacji `SESSION_SECRET`.
- Integracji z Neon Auth lub wybierania konkretnego providera sesji.
- Nagłówka `X-Account-ID`, stałego konta technicznego, domyślnego konta lub innego spoofowalnego źródła tożsamości.
- Tabeli `accounts`, tabel kategorii/transakcji, migracji, seedów, repozytorium domenowego i PostgreSQL RLS.
- Opakowania `/` lub `/healthz` middleware’em oraz dodawania `GET /api/me` albo innego endpointu probe.
- Usuwania konta, soft-delete, cleanupu, TTL i zmian retencji.
- Dowodu izolacji realnych rekordów; scenariusz dwóch kont wymaga istniejącego modelu danych i zostanie zweryfikowany w późniejszym slice’ie.

## Implementation Approach

Utworzyć flat package `internal/account` oparty wyłącznie na bibliotece standardowej. Rozdzielić czysty kontrakt identyfikatora/contextu od warstwy HTTP, ale utrzymać oba elementy w jednym pakiecie domenowym zgodnie z konwencją repozytorium.

Przepływ chronionego żądania ma być jednokierunkowy:

`Resolver.ResolveAccountID(request)` → walidacja `AccountID` → `WithAccountID(request.Context(), id)` → handler.

Resolver jest jedynym zaufanym źródłem identyfikatora. Middleware nie czyta `account_id` z path/query/body/headera i nie wybiera wartości domyślnej. Brak resolvera, nieudana rezolucja albo nieważny identyfikator kończy się `401` bez wywołania następnego handlera. Przyszły store nie będzie przyjmował operacji account-owned bez jawnego `AccountID`; dostęp do nieistniejącego dla danego właściciela zasobu będzie maskowany jako `404` w slice’ach domenowych.

## Critical Implementation Details

### State sequencing

Resolver musi wykonać się przed przekazaniem requestu dalej, a context z właścicielem musi zostać utworzony przed wywołaniem następnego handlera. Żadna ścieżka nie może uruchomić handlera chronionego z częściowo rozpoznaną lub pustą tożsamością.

### Debug & observability

Odpowiedź odmowy ma pozostać JSON-em z `Cache-Control: no-store` i bez ujawniania wartości identyfikatora, sekretów lub przyczyny wewnętrznej. F-01 nie dodaje per-request loggingu; logowanie auth i korelacja sesji należą do S-01.

## Phase 1: Provider-neutral account identity contract

### Overview

Zdefiniować typ właściciela i bezpieczne operacje na request context, aby kolejne warstwy nie używały surowych stringów ani kluczy contextu opartego na stringu.

### Changes Required:

#### 1. Account identity and context API

**File**: `internal/account/identity.go`

**Intent**: Dodać minimalny, provider-neutralny typ identyfikatora właściciela oraz helpers do bezpiecznego przekazywania go między middleware a handlerem. Pakiet nie będzie wiedział, czy wartość pochodzi z cookie, tokena czy innego mechanizmu S-01.

**Contract**: Eksponować `AccountID`, walidację poprawności, `WithAccountID(context.Context, AccountID) context.Context` oraz `AccountIDFromContext(context.Context) (AccountID, bool)`. Klucz contextu ma być prywatnym typem; odczyt ma zwracać `false` dla braku wartości i nieważnego identyfikatora.

#### 2. Identity contract tests

**File**: `internal/account/identity_test.go`

**Intent**: Utrwalić zachowanie context API bez zależności od bazy, HTTP servera ani providera auth.

**Contract**: Testy mają pokryć round-trip poprawnego `AccountID`, brak identyfikatora w pustym context oraz odrzucenie pustego i whitespace-only identyfikatora. Test ma także potwierdzić, że context innego typu nie dostarcza właściciela.

### Success Criteria:

#### Automated Verification:

- `gofmt -d internal/account/identity.go internal/account/identity_test.go` nie zgłasza różnic.
- `go test ./internal/account -run 'TestAccountID'` przechodzi.

#### Manual Verification:

- Przegląd API potwierdza, że pakiet nie zawiera auth, provider-specific types, domyślnego konta ani zależności spoza standardowej biblioteki.

**Implementation Note**: Po zakończeniu fazy i przejściu automatycznej weryfikacji zatrzymaj się na potwierdzenie, że kontrakt identity jest wystarczająco mały i provider-neutralny, zanim przejdziesz do middleware.

---

## Phase 2: HTTP enforcement and contract verification

### Overview

Dodać middleware, który zasila context wynikiem zaufanego resolvera i fail-closed odrzuca żądania bez poprawnego właściciela. Zweryfikować, że istniejące publiczne endpointy pozostają nietknięte.

### Changes Required:

#### 1. Account resolver and required-owner middleware

**File**: `internal/account/middleware.go`

**Intent**: Zdefiniować interfejs, który później zaimplementuje warstwa sesji, oraz middleware chroniące przyszłe endpointy account-owned bez wprowadzania tymczasowego mechanizmu auth.

**Contract**: Eksponować `Resolver` z metodą rozpoznającą `AccountID` z `*http.Request` oraz `RequireAccount(resolver Resolver, next http.Handler) http.Handler`. Middleware ma:

- zwrócić JSON `401` o kształcie `{"error":"unauthorized"}` dla nil resolvera, nieudanej rezolucji lub nieważnego `AccountID`;
- ustawić `Cache-Control: no-store` i `Content-Type: application/json` przed odpowiedzią błędu;
- nie wywołać `next` po odmowie;
- przy poprawnej wartości przekazać request z `AccountID` w context do `next`;
- nie akceptować identyfikatora z nagłówka, query, path lub body bez udziału implementacji `Resolver`.

#### 2. Middleware contract tests

**File**: `internal/account/middleware_test.go`

**Intent**: Sprawdzić granicę HTTP przez `httptest` i testowy resolver, bez tworzenia produkcyjnego endpointu, bazy ani sesji.

**Contract**: Testy mają pokryć brak/nieudaną rezolucję (`401`, brak wywołania downstream), nieważny identyfikator (`401`) oraz poprawnego właściciela (downstream otrzymuje właściwy `AccountID` i może zwrócić `200`). Należy sprawdzić JSON error response i brak wycieku identyfikatora w odpowiedzi odmowy.

#### 3. Preserve public server routes

**File**: `cmd/nest-cash/main.go` (weryfikacja bez zmiany tras)

**Intent**: Zachować istniejący publiczny kontrakt smoke deployu; F-01 dostarcza gotowy middleware dla przyszłych tras, ale nie opakowuje `/` ani `/healthz`.

**Contract**: Rejestracja `GET /` i `GET /healthz` pozostaje publiczna i funkcjonalnie niezmieniona. Nie dodawać endpointu probe wyłącznie na potrzeby F-01.

### Success Criteria:

#### Automated Verification:

- `gofmt -d internal/account/*.go` nie zgłasza różnic.
- `go test ./internal/account` przechodzi i obejmuje wszystkie ścieżki odmowy oraz sukcesu middleware.
- `go test ./...` przechodzi w środowisku z dostępnym pobieraniem zależności.
- `go vet ./...` przechodzi.

#### Manual Verification:

- Uruchomienie serwera bez `DATABASE_URL` nadal pozwala wywołać publiczne `/healthz` i zwraca dotychczasowy `503` z `database: not_configured`.
- Przegląd zmian potwierdza brak `X-Account-ID`, stałego konta, auth/session code, schematu DB i nowej trasy produkcyjnej.
- Przegląd implementacji potwierdza, że przyszły resolver sesji może zasilić ten sam kontrakt bez zmiany handlerów korzystających z contextu.

**Implementation Note**: Po zakończeniu fazy i przejściu automatycznej weryfikacji zatrzymaj się na ręczne potwierdzenie publicznego `/healthz` oraz zakresu zmian przed wdrożeniem kolejnego slice’a.

## Testing Strategy

### Unit Tests:

- `AccountID` round-trip przez context oraz zachowanie dla brakującej, pustej i whitespace-only wartości.
- Resolver/middleware: brak resolvera, `ok == false`, nieważny identyfikator i poprawny identyfikator.
- Brak wywołania downstream po odmowie.
- Przekazanie dokładnie tego właściciela, którego zwrócił zaufany resolver.
- JSON `401` bez ujawniania identyfikatora.

### Integration Tests:

F-01 nie dodaje testu PostgreSQL ani migracji. Test dwóch właścicieli na rzeczywistych rekordach, w tym listowanie, filtrowanie, modyfikacja i powiązanie z kategorią, jest obowiązkowym kryterium odpowiednich późniejszych slice’ów z danymi.

### Manual Testing Steps:

1. Uruchomić `go test ./...` i `go vet ./...` przy działającym proxy zależności.
2. Uruchomić aplikację bez `DATABASE_URL` i sprawdzić, że `GET /healthz` pozostaje publiczny oraz zwraca `503` bez danych auth.
3. Przejrzeć test middleware i potwierdzić, że scenariusz braku tożsamości nie wywołuje chronionego handlera.
4. Sprawdzić diff pod kątem tymczasowych źródeł tożsamości, domyślnego konta lub parametrów requestu używanych jako właściciel.

## Performance Considerations

Middleware wykonuje stałą liczbę operacji in-memory przed handlerem i nie dodaje zapytania do bazy. F-01 nie wymaga cache, indeksów ani limitu obciążenia; oczekiwane opóźnienie jest pomijalne względem przyszłej rezolucji sesji i DB.

## Migration Notes

Brak migracji i zmian danych. Dodanie `AccountID` do tabel oraz ewentualna polityka usuwania danych będą planowane razem z pierwszymi modelami domenowymi, z osobnym rollbackiem schematu.

## References

- `context/foundation/roadmap.md:52-63` — definicja F-01, zakres i ryzyko rozrostu do pełnego auth.
- `context/foundation/prd.md:35-38,80-85,103-112` — prywatność, retencja i single-user access control.
- `context/foundation/tech-stack.md:2-23` — Go 1.22.2, standard library preferred, auth assembled later.
- `cmd/nest-cash/main.go:16-47,62-94` — aktualny serwer, publiczne trasy i JSON response helper.
- `cmd/nest-cash/main_test.go:8-19` — istniejący wzorzec testów `httptest`.
- `context/deployment/deploy-plan.md:148-175` — skonfigurowany `SESSION_SECRET` i publiczny health check, bez zaimplementowanego auth.

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles.

### Phase 1: Provider-neutral account identity contract

#### Automated

- [x] 1.1 `gofmt -d` reports no formatting differences for identity files — 7a2622a
- [x] 1.2 Account identity/context unit tests pass — 7a2622a

#### Manual

- [ ] 1.3 Identity API is provider-neutral and contains no auth or default-account behavior

### Phase 2: HTTP enforcement and contract verification

#### Automated

- [x] 2.1 `gofmt -d` reports no formatting differences for account package files
- [x] 2.2 Middleware contract tests pass for reject and allow paths
- [x] 2.3 `go test ./...` passes with dependencies available
- [x] 2.4 `go vet ./...` passes

#### Manual

- [ ] 2.5 Public `/healthz` remains accessible without an account
- [ ] 2.6 Diff contains no temporary identity source, production probe route, or auth implementation
