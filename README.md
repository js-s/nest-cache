# NestCash

Osobisty tracker budżetu domowego: Go API + SPA (Vue 3 + TypeScript), jeden obraz kontenera, deploy na Fly.io z bazą Neon PostgreSQL.

NestCash nie jest produktem rynkowym — to narzędzie single-user z ambicją edukacyjną. Źródłem prawdy o założeniach projektu jest `context/foundation/` (PRD, roadmapa, tech-stack, infrastruktura, plan testów).

## Problem i cel

Domowy budżet ginie w rozproszonych arkuszach i plikach bez spójnego formatu, a uciążliwy proces wprowadzania danych prowadzi do porzucenia po kilku tygodniach. NestCash daje jedno miejsce z niskim progiem wejścia i elastycznym podglądem.

**Persona:** autor projektu i jego rodzina/gospodarstwo domowe. MVP jest single-user — jeden użytkownik per konto, brak ról. Multi-user i współdzielenie budżetu odłożone do v2.

**Kryterium sukcesu (primary):** użytkownik może się zarejestrować, zalogować, utworzyć kategorie, wprowadzić wydatki/przychody i zobaczyć tabelaryczne podsumowanie filtrowane jednocześnie po okresie i kategorii.

### Zakres MVP

| Obszar | Zaimplementowane |
|---|---|
| Auth | Rejestracja, logowanie, wylogowanie, 30-dniowa sesja (sliding window) |
| Kategorie | Predefiniowane + własne, hierarchia kategoria → podkategoria, osobno wydatki i przychody |
| Transakcje | Dodawanie, edycja, usuwanie, paginowana lista, filtr po typie |
| Podsumowanie | Sumy per kategoria w okresie + filtr kategorii, wskaźnik budżetowy (wydatki/przychody) z progiem 80% |

### Poza zakresem MVP

Skanowanie paragonów/OCR, integracja z bankiem, multi-user, powiadomienia push/email, praca offline, edycja/dezaktywacja kategorii, alternatywne metody auth (OAuth).

### Logika domenowa

Wskaźnik budżetowy = suma wydatków / suma przychodów w wybranym okresie. Przy ≥ 80% podsumowanie pokazuje pasywny sygnał wizualny. Próg 80% jest stały w MVP, konfigurowalny per-user w v2. Brak przychodu w okresie → wskaźnik nie jest liczony (`ratio_pct: null`, brak dzielenia przez zero).

## Architektura

Jeden binarny serwer Go serwuje zarówno JSON API (`/api`), health check (`/healthz`), jak i zbudowany SPA (`web/dist`) z fallbackiem historii Vue. Jedno pochodzenie (same-origin) eliminuje CORS i upraszcza cookie sesyjne.

```
Przeglądarka
   │  fetch credentials: same-origin
   ▼
Go net/http  ── /healthz        → ping DB (200 / 503 degraded)
             ── /api/...        → JSON, account.RequireAccount, 404 JSON dla nieznanych
             ── /*              → statyki web/dist + fallback index.html
   │
   ▼
Neon PostgreSQL (pgx, pooled URL, migracje przy starcie)
```

### Backend (root, Go 1.22)

- `cmd/nest-cash/` — entrypoint, tablica tras (`spa.go`), serwowanie SPA, `main.go` (DB, migracje, fail-closed auth).
- `internal/account/` — provider-neutralny kontrakt tożsamości właściciela danych (`AccountID` w kontekście) i middleware `RequireAccount`.
- `internal/auth/` — użytkownicy, sesje, hasła, rate limit, resolver. Sesja to nieprzejrzysty token 256-bit; cookie `nest_session` trzyma surowy token, w bazie leży tylko `sha256(token)`. Hasła: bcrypt cost 12.
- `internal/categories/` — kategorie i podkategorie, seed z `resources/categories.yaml`.
- `internal/transactions/` — transakcje, paginacja i agregacje podsumowania (`summary.go`).
- `internal/db/` — migracje SQL uruchamiane przy starcie (`migrations/*.sql`).

Informacja o właścicielu **zawsze** pochodzi z resolvera/kontekstu sesji, nigdy z danych żądania. Każda operacja na transakcji/kategorii filtruje po `user_id` — to fundament izolacji kont (roadmap F-01).

### Frontend (`web/`, Vue 3 + Vite)

- `web/src/router.ts` — trasy: `/login`, `/register`, `/summary`, `/categories`, `/operations`; strażnik `beforeEach` wymusza `ensureSession()` dla tras chronionych.
- `web/src/api/client.ts` — typowany klient JSON, `ApiError` ze statusem i kodem błędu, bazowy URL z `VITE_API_BASE_URL` (domyślnie `/api`).
- `web/src/views/` — ekrany domenowe; `web/src/auth.ts` — zarządzanie sesją po stronie klienta.

W dev Vite proxuje `/api` do `:8080` (`web/vite.config.ts`). Na produkcji SPA i API są w tym samym obrazie.

### API

| Metoda | Ścieżka | Auth | Opis |
|---|---|---|---|
| GET | `/healthz` | nie | Status aplikacji i połączenia z DB |
| POST | `/api/auth/register` | nie (rate limit) | Rejestracja email + hasło |
| POST | `/api/auth/login` | nie (rate limit) | Logowanie |
| POST | `/api/auth/logout` | nie | Wylogowanie |
| GET | `/api/auth/me` | tak | Profil zalogowanego użytkownika |
| GET | `/api/categories` | tak | Kategorie i podkategorie właściciela |
| POST | `/api/categories` | tak | Utworzenie kategorii/podkategorii |
| GET | `/api/transactions` | tak | Paginowana lista (filtr `kind`, `page`, `limit`) |
| POST | `/api/transactions` | tak | Dodanie operacji |
| PUT | `/api/transactions/{id}` | tak | Edycja operacji |
| DELETE | `/api/transactions/{id}` | tak | Usunięcie operacji |
| GET | `/api/summary` | tak | Podsumowanie (filtr `from`, `to`, `category_id`) |

Nieznane ścieżki `/api/*` zwracają JSON `404`, nigdy `index.html` — fallback SPA dotyczy wyłącznie tras przeglądarkowych.

### Model danych

| Tabela | Kluczowe kolumny |
|---|---|
| `users` | `id`, `email` (unikalny case-insensitive), `password_hash`, `created_at` |
| `sessions` | `token_hash` (PK), `user_id`, `expires_at`, `created_at` |
| `categories` | `id`, `user_id`, `parent_id` (NULL = grupa), `kind` (`expense`/`income`), `name` |
| `transactions` | `id`, `user_id`, `category_id`, `kind`, `amount > 0`, `occurred_on`, `description` |

Ograniczenia integralności żyją w schemacie (CHECK, unikalne indeksy na `lower(name)` w obrębie właściciela, `amount > 0`), a nie w kodzie aplikacji. Migracje są addytywne i forward-compatible — rollback obrazu nie cofa schematu.

## Uruchomienie lokalne

Wymagania: Go 1.22.2, Node 22 (dla `web/`), opcjonalnie PostgreSQL.

```bash
# Backend (bez DATABASE_URL serwuje tylko /healthz jako degraded)
go run ./cmd/nest-cash

# Frontend w trybie dev (proxy /api → :8080)
npm --prefix web install
npm --prefix web run dev
```

Zmienne środowiskowe backendu: `PORT` (domyślnie `8080`), `DATABASE_URL` (włącza pełne API; bez niej tylko `/healthz`), `SESSION_SECRET` (wymagane ≥32 znaki, gdy `DATABASE_URL` jest ustawione — inaczej boot fail-closed), `STATIC_DIR` (domyślnie `web/dist`).

## Testy i walidacja

```bash
go test ./...                          # testy backendu
CGO_ENABLED=0 go test ./...            # fallback na macOS przy błędzie dyld LC_UUID
go build ./...                         # compile-check
npm --prefix web run build             # typecheck + build produkcyjny (jedyne bramkowanie frontu)
```

Testy DB-backed (`isolation_integration_test.go`, `validation_integration_test.go`) pomijają się bez `DATABASE_URL`. Na macOS, gdy `go test` przerywa z `dyld: missing LC_UUID`, poprzedź komendę `CGO_ENABLED=0`.

Aktualne bramki CI (`.github/workflows/fly.yml`): build web + skan artefaktu pod sekrety, `go test ./...` na PostgreSQL 16, smoke test tras po deployu. Rzeczy nietestowane i plan rozbudowy — `context/foundation/test-plan.md`.

## Deployment

- **Hosting:** Fly.io, region `fra`, `shared-cpu-1x`, 256 MB, `auto_stop_machines = "stop"`. Aplikacja: `nest-cash.fly.dev`.
- **Baza:** Neon PostgreSQL, `eu-central-1`, pooled connection string (PgBouncer, `pool_max_conns=5`).
- **CI/CD:** push do `main` → `.github/workflows/fly.yml` buduje `web/dist` i deployuje obraz; `fly-rollback.yml` obsługuje rollback.
- **Sekrety:** `DATABASE_URL` i `SESSION_SECRET` wyłącznie przez `fly secrets set` / GitHub Secrets. `VITE_API_BASE_URL=/api` to konfiguracja build-time, nie sekret.

Szczegóły i przypadki brzegowe: `context/deployment/deploy-plan.md`, `context/foundation/infrastructure-web.md`.

## Prywatność i bezpieczeństwo

- Dane finansowe są izolowane per konto na poziomie każdego zapytania i chronione middleware `RequireAccount`.
- Hasła nigdy nie są przechowywane jawnie (bcrypt), sesje są nieprzejrzyste, a ich surowe tokeny nie trafiają do bazy.
- `DATABASE_URL` bez poprawnego `SESSION_SECRET` (≥32 znaki) zatrzymuje boot zamiast serwować nieuwierzytelnione API.
- Sekretów i plików `.env` nie commitujemy; oba `.gitignore` wykluczają lokalne pliki sekretów.

## Dokumentacja projektu

| Dokument | Zawartość |
|---|---|
| `context/foundation/prd.md` | Wymagania funkcjonalne, NFR, non-goals, user stories |
| `context/foundation/roadmap.md` | Pionowe slice'y i ich status |
| `context/foundation/tech-stack.md`, `tech-stack-web.md` | Wybór stacku backendu i frontendu |
| `context/foundation/infrastructure-web.md` | Wybór i analiza platformy deployu |
| `context/foundation/test-plan.md` | Strategia testów, mapa ryzyk, bramki jakości |
| `context/foundation/lessons.md` | Rejestr powtarzalnych reguł |
| `context/deployment/deploy-plan.md` | Krok po kroku wdrożenie Fly.io + Neon |
| `AGENTS.md` | Zasady pracy asystentów AI w tym repo |
