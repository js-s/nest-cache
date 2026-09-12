<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Minimalny kontrakt własności danych

- **Plan**: `context/changes/account-data-boundary/plan.md`
- **Scope**: Phase 1–2 of 2
- **Date**: 2026-09-11
- **Verdict**: APPROVED
- **Findings**: 0 critical, 3 warnings, 1 observation — all resolved

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Findings

### F1 — Middleware może wywołać panic przy niepoprawnych zależnościach

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: `internal/account/middleware.go:17-30`
- **Detail**: `resolver == nil` obsługiwał nil interface, ale nie typed-nil resolver. Przy poprawnym ID nil `next` również kończył się panicem. Testy pokrywały tylko zwykły nil resolver.
- **Fix**: Dodano walidację zwykłych i typed-nil zależności przez `reflect.Value.IsNil`. Brak resolvera zwraca `401`, a brak downstreamu deterministyczny `500`; dodano testy obu przypadków.
- **Decision**: FIXED — Fix now

### F2 — Plan nadal wymagał niezmienionego zachowania publicznego `/`

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: `context/changes/account-data-boundary/plan.md:11,28,140-144`; `cmd/nest-cash/main.go:47`; `cmd/nest-cash/spa.go:11-21`
- **Detail**: Plan mówił, że `/` i `/healthz` pozostają funkcjonalnie niezmienione. Po integracji GUI `/` serwuje shell SPA, natomiast `/healthz` zachowuje publiczny kontrakt 503 bez bazy. Sama granica F-01 nie została rozszerzona.
- **Fix**: Zaktualizowano kontrakt Phase 2, referencje do tras oraz dodano notę o późniejszej integracji GUI. `/healthz` pozostaje niezmieniony, a publiczny `/` może serwować SPA.
- **Decision**: FIXED — plan addendum

### F3 — Sukces middleware nie ustanawiał domyślnej polityki `no-store`

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: `internal/account/middleware.go:30`
- **Detail**: `Cache-Control: no-store` było ustawiane tylko dla odmowy. Udana odpowiedź przyszłego chronionego endpointu mogła zostać zbuforowana, jeśli handler nie ustawiłby nagłówka samodzielnie.
- **Fix**: Middleware ustawia `Cache-Control: no-store` jako bezpieczny default także na ścieżce sukcesu; test ścieżki allow sprawdza ten nagłówek.
- **Decision**: FIXED — Fix A

### F4 — Testy Go nie były początkowo reprodukowalnie uruchamialne

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: N/A — środowisko uruchomieniowe
- **Detail**: Przy Go 1.22.2 i włączonym cgo test binaries kończyły się na macOS błędem `dyld: missing LC_UUID load command`, przed uruchomieniem testów.
- **Fix**: Ponowiono weryfikację z `CGO_ENABLED=0`: `go test ./internal/account` zakończył się wynikiem 12 testów, a `CGO_ENABLED=0 go test ./...` wynikiem 22 testów. Ręczny `/healthz` zweryfikowano jako `503` z `database: not_configured` i `Cache-Control: no-store`.
- **Decision**: FIXED — rerun with `CGO_ENABLED=0`

## Verification

- `gofmt -d internal/account/*.go` — PASS
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `CGO_ENABLED=0 go test ./...` — PASS, 22 tests
- `ReadLints` dla zmienionych plików — brak błędów
- Publiczny `/healthz` bez `DATABASE_URL` — `503`, `database: not_configured`, `Cache-Control: no-store`

## Scope Conclusion

Rozszerzenie roadmapy o GUI nie zmieniło zakresu F-01. Kontrakt pozostaje provider-neutralnym fundamentem dla kolejnych slice’ów; rejestracja, logowanie, sesje i podłączenie resolvera do tras domenowych nadal należą do S-01.
