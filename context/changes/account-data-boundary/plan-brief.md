# Minimalny kontrakt własności danych — Plan Brief

> Full plan: `context/changes/account-data-boundary/plan.md`

## What & Why

F-01 ustanawia minimalną granicę, dzięki której przyszłe endpointy będą mogły działać wyłącznie w kontekście jednego właściciela danych. Jest to fundament prywatności wymagany przez PRD, ale nie jest jeszcze rejestracją, logowaniem ani pełnym systemem sesji.

## Starting Point

Repozytorium ma minimalny serwer Go z publicznymi `/` i `/healthz`, opcjonalnym pingiem PostgreSQL oraz jednym testem handlera. Nie ma middleware, auth, tabel, migracji, store’a ani endpointów domenowych.

## Desired End State

`internal/account` dostarcza typowany `AccountID`, bezpieczne helpers contextu, interfejs resolvera oraz middleware `RequireAccount`. Brak lub błędna tożsamość kończy się JSON `401`; poprawny właściciel trafia do handlera przez request context.

Istniejące endpointy publiczne pozostają bez zmian. Późniejszy auth zasili ten sam kontrakt, a przyszłe store’y będą wymagały `AccountID` przy każdej operacji na danych konta.

## Key Decisions Made

- **Zakres:** tylko provider-neutralny plumbing i testy; bez auth, DB, migracji i endpointu probe. Źródło: roadmapa + decyzje planowania.
- **Tożsamość:** typowany opaque string `AccountID`, bez przedwczesnego wyboru UUID lub providera. Źródło: planowanie.
- **Resolver:** interfejs i testowy fake teraz; prawdziwy adapter sesji dopiero w S-01. Źródło: planowanie.
- **Enforcement:** resolver → walidacja → context → handler; brak identyfikatora z request input. Źródło: PRD + planowanie.
- **Błędy:** `401` dla braku/błędu tożsamości; przyszły cudzy zasób maskowany jako `404`. Źródło: planowanie.
- **Retencja:** bez TTL, cleanupu i delete flow w F-01. Źródło: PRD + planowanie.

## Scope

**In scope:**

- `internal/account` z identity/context contract.
- Resolver interface i fail-closed HTTP middleware.
- Testy jednostkowe oraz testy middleware przez `httptest`.
- Zachowanie publicznego `/healthz`.

**Out of scope:**

- Email/password auth, sesje, cookies, reset haseł i Neon Auth.
- Tabele, migracje, store, RLS i testy PostgreSQL.
- Kategorie, transakcje, filtrowanie i realny test dwóch kont.
- Usuwanie konta, soft-delete i zmiany retencji.

## Architecture / Approach

Chronione żądanie przechodzi przez resolver, który jest jedynym zaufanym źródłem `AccountID`. Middleware odrzuca brak lub niepoprawną wartość, a po sukcesie zapisuje właściciela w context. Nie będzie tymczasowego nagłówka ani stałego konta. `/` i `/healthz` pozostają poza middleware’em, ponieważ obecnie nie obsługują danych konta.

## Phases at a Glance

- **Phase 1 — Provider-neutral account identity contract:** typ `AccountID`, context helpers i testy ich zachowania. Główne ryzyko: przypadkowe związanie fundamentu z providerem auth.
- **Phase 2 — HTTP enforcement and contract verification:** resolver interface, `RequireAccount`, testy reject/allow i regresja publicznego health checka. Główne ryzyko: dopuszczenie spoofowalnego źródła tożsamości.

**Prerequisites:** Go 1.22.2 i działające pobieranie modułu `pgx` dla pełnego `go test ./...`; do samego pakietu F-01 wystarcza standardowa biblioteka.

**Estimated effort:** ~1 sesja implementacji i weryfikacji w 2 małych fazach; bez migracji i ręcznych zmian infrastruktury.

## Open Risks & Assumptions

- Aktualny `AGENTS.md` opisuje starszy stan pre-scaffoldowy; plan opiera się na aktualnym kodzie i roadmapie.
- Pełne `go test ./...` wymaga rozwiązania lokalnego problemu z pobraniem `pgx` (`Forbidden`/`EOF` z proxy).
- Izolacja realnych rekordów nie może być dowiedziona przed powstaniem schematu; będzie obowiązkowym kryterium przyszłych slice’ów danych.
- Status `404` dla zasobu spoza konta zostanie zastosowany dopiero przez przyszłe handlery/store’y, ponieważ F-01 nie ma jeszcze zasobów.

## Success Criteria (Summary)

- Brakująca lub nieważna tożsamość nie uruchamia chronionego handlera i zwraca JSON `401`.
- Poprawny resolver przekazuje właściwy `AccountID` przez context.
- Istniejący publiczny `/healthz` zachowuje dotychczasowe zachowanie.
- Implementacja nie wprowadza auth, tymczasowego identity headera, tabel ani produkcyjnego endpointu probe.
