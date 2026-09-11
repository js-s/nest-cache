---
project: NestCash
version: 1
status: draft
created: 2026-09-10
updated: 2026-09-10
prd_version: 1
main_goal: speed
top_blocker: time
---

# Roadmap: NestCash

> Derived from `context/foundation/prd.md` (v1) + automatycznie zbadany i potwierdzony baseline repozytorium.
> Edytuj w miejscu; archiwizuj dopiero po pełnym zastąpieniu.
> Elementy poniżej są ułożone według zależności. Tabela „At a glance” jest indeksem.

## Vision recap

Domowy budżet ginie w rozproszonych arkuszach i plikach tekstowych bez spójnego formatu, a uciążliwy proces wprowadzania danych prowadzi do porzucenia po kilku tygodniach. NestCash daje jedno miejsce z niskim progiem wejścia i elastycznym podglądem; jest osobistym narzędziem do nauki budowania aplikacji i zachowania kontroli nad danymi finansowymi.

## North star

„North star” (gwiazda przewodnia) oznacza tutaj najmniejszą pełną ścieżkę użytkownika od wprowadzenia danych do otrzymania użytecznego rezultatu, która pokazuje, że produkt działa.

**S-04: Użytkownik filtruje zapisany wydatek w tabelarycznym podsumowaniu** — to pierwszy przepływ, który bezpośrednio sprawdza główny cel opisany w `US-01` i kryterium sukcesu dotyczące filtrowanego podsumowania.

## At a glance

| ID | Change ID | Outcome (user can …) | Prerequisites | PRD refs | Status |
|---|---|---|---|---|---|
| F-01 | account-data-boundary | (foundation) żądania API mają minimalny kontrakt własności danych i izolacji konta | — | Access Control, NFR (privacy), NFR (retention) | ready |
| S-01 | account-registration-and-login | zarejestrować konto, zalogować się i utrzymać długą sesję | F-01 | FR-001, FR-002, Access Control | blocked |
| S-02 | predefined-and-custom-categories | otrzymać kategorie startowe i utworzyć własne kategorie wydatków lub przychodów | S-01 | FR-003 | blocked |
| S-03 | expense-entry-and-operation-list | wprowadzić wydatek i zobaczyć go na paginowanej liście operacji | S-02 | US-01, FR-005, FR-007 | proposed |
| S-04 | filtered-expense-summary | zobaczyć zapisany wydatek w tabelarycznym podsumowaniu filtrowanym jednocześnie po okresie i kategorii | S-03 | US-01, FR-009 | blocked |
| S-05 | income-and-budget-ratio | wprowadzić przychód i zobaczyć pasywny wskaźnik relacji wydatków do przychodów | S-04 | FR-006, Business Logic, NFR (response time) | proposed |

## Baseline

Stan repozytorium na `2026-09-10` (automatycznie zbadany i potwierdzony). Celowo nie odtwarzam tu frontendu: klient jest osobnym wdrażanym komponentem zgodnie z architekturą API-first.

- **Frontend:** absent — brak kodu UI, narzędzi budowania i routingu w tym repozytorium; osobna jednostka wdrożeniowa.
- **Backend / API:** partial — minimalny serwer JSON z `/` i `/healthz` w `cmd/nest-cash/main.go`; brak endpointów domenowych.
- **Data:** partial — połączenie i ping PostgreSQL są używane przez `cmd/nest-cash/main.go`; brak schematu, migracji i seedów.
- **Auth:** absent — brak rejestracji, logowania, sesji i middleware; instrukcja Neon Auth w `context/deployment/deploy-plan.md` nie została wykonana ani zintegrowana.
- **Deploy / infra:** partial — `Dockerfile`, `fly.toml` i `.github/workflows/fly.yml` istnieją; workflow testów/lintu oraz część ręcznych kontroli pozostają poza repozytorium.
- **Observability:** partial — `/healthz` i podstawowe logowanie istnieją; brak logów strukturalnych, metryk, śledzenia błędów i dashboardów aplikacyjnych.

## Foundations

### F-01: Minimalny kontrakt własności danych

- **Outcome:** (foundation) żądania API mają minimalny kontrakt identyfikowania właściciela danych i odrzucania dostępu do danych spoza jego konta; konkretne przepływy konta i finansów integrują ten kontrakt w kolejnych pionowych przyrostach.
- **Change ID:** account-data-boundary
- **PRD refs:** Access Control, NFR (privacy), NFR (retention)
- **Unlocks:** S-01, S-02, S-03, S-04, S-05
- **Prerequisites:** —
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Fundament musi pozostać minimalną granicą własności danych; rozbudowanie go do kompletnego systemu Auth przed pierwszym przepływem zwiększyłoby koszt i opóźniło cel szybkości.
- **Status:** ready

## Slices

### S-01: Rejestracja i logowanie

- **Outcome:** użytkownik może zarejestrować konto przez email i hasło, zalogować się oraz korzystać z długiej sesji.
- **Change ID:** account-registration-and-login
- **PRD refs:** FR-001, FR-002, Access Control
- **Prerequisites:** F-01
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:**
  - Czy odzyskiwanie lub reset hasła jest obowiązkowe w MVP, a jeśli tak, jaki jest jego minimalny przepływ? — Owner: user. Block: yes.
  - Czy opcja Neon Auth została faktycznie aktywowana i ma być używana, czy obowiązuje podejście z `tech-stack.md` oparte na bibliotekach Go? — Owner: user. Block: yes.
- **Risk:** Auth jest nieobecny w kodzie, a prywatność danych jest warunkiem uruchomienia; nieuzgodniony zakres odzyskiwania hasła lub źródło sesji może zatrzymać cały łańcuch.
- **Status:** blocked

### S-02: Kategorie startowe i własne

- **Outcome:** użytkownik może otrzymać predefiniowane kategorie przy pierwszym użyciu i utworzyć własne kategorie wydatków lub przychodów.
- **Change ID:** predefined-and-custom-categories
- **PRD refs:** FR-003
- **Prerequisites:** S-01
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:**
  - Jakie predefiniowane kategorie wydatków i przychodów powinny być dostępne na start? — Owner: user. Block: yes.
- **Risk:** Kategorie są wymagane przed wpisaniem operacji; brak krótkiej listy startowej albo zbyt szeroki katalog zwiększy tarcie w ścieżce do pierwszego wydatku.
- **Status:** blocked

### S-03: Wprowadzenie wydatku i lista operacji

- **Outcome:** użytkownik może wprowadzić wydatek z kwotą, kategorią, datą i opcjonalnym opisem oraz zobaczyć go na paginowanej liście własnych operacji.
- **Change ID:** expense-entry-and-operation-list
- **PRD refs:** US-01, FR-005, FR-007
- **Prerequisites:** S-02
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:** —
- **Risk:** To pierwszy pionowy przepływ domenowy; błąd w walidacji kwoty, kategorii lub właściciela danych podważy późniejsze podsumowania.
- **Status:** proposed

### S-04: Filtrowane podsumowanie wydatku

- **Outcome:** użytkownik może zobaczyć zapisany wydatek w tabelarycznym podsumowaniu filtrowanym jednocześnie po okresie i kategorii.
- **Change ID:** filtered-expense-summary
- **PRD refs:** US-01, FR-009
- **Prerequisites:** S-03
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:**
  - Jakie opcje granulacji okresu mają być dostępne w filtrze: dzień, tydzień, miesiąc, rok czy zakres niestandardowy? — Owner: user. Block: yes.
- **Risk:** Ten element jest gwiazdą przewodnią roadmapy i powinien nastąpić tak wcześnie, jak pozwala zapis wydatku; niejasna granulacja może wymusić zmianę kontraktu filtrowania.
- **Status:** blocked

### S-05: Przychód i wskaźnik budżetowy

- **Outcome:** użytkownik może wprowadzić przychód, a podsumowanie pokazuje pasywny wskaźnik relacji wydatków do przychodów i sygnalizuje próg 80% lub więcej.
- **Change ID:** income-and-budget-ratio
- **PRD refs:** FR-006, Business Logic, NFR (response time)
- **Prerequisites:** S-04
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Wskaźnik ma sens dopiero po połączeniu przychodów z istniejącym podsumowaniem; wynik musi pozostać pasywnym sygnałem, bez dokładania powiadomień poza zakresem.
- **Status:** proposed

## Backlog Handoff

| Roadmap ID | Change ID | Suggested issue title | Ready for `/10x-plan` | Notes |
|---|---|---|---|---|
| F-01 | account-data-boundary | Ustanowić minimalną granicę własności danych | yes | Odblokowuje cały łańcuch funkcji finansowych. |
| S-01 | account-registration-and-login | Dodać rejestrację, logowanie i długą sesję | no | Najpierw rozstrzygnąć reset hasła i status Neon Auth. |
| S-02 | predefined-and-custom-categories | Dodać kategorie startowe i własne | no | Zależne od S-01 i listy kategorii startowych. |
| S-03 | expense-entry-and-operation-list | Dodać wprowadzanie wydatków i listę operacji | no | Zależne od S-02; brak własnego pytania blokującego. |
| S-04 | filtered-expense-summary | Dodać filtrowane podsumowanie wydatków | no | Zależne od S-03 i decyzji o granulacji okresów. |
| S-05 | income-and-budget-ratio | Dodać przychody i wskaźnik budżetowy | no | Zależne od S-04; brak własnego pytania blokującego. |

## Open Roadmap Questions

1. **Jakie predefiniowane kategorie powinny być dostępne na start?** — Owner: user. Block: S-02.
2. **Jakie opcje granulacji okresów mają być dostępne w filtrze podsumowania?** — Owner: user. Block: S-04.
3. **Jak wygląda flow resetowania/odzyskiwania hasła?** — Owner: user. Block: S-01.
4. **Czy Neon Auth zostało aktywowane i ma być źródłem kont oraz sesji, czy pozostajemy przy podejściu opisanym w `tech-stack.md`?** — Owner: user. Block: S-01.
5. **Czy termin `2026-07-05` zapisany w PRD jest nadal wiążący?** — Owner: user. Block: roadmap-wide (wpływa na kolejność, ale nie blokuje samego planowania elementów).

## Parked

- **Edycja lub dezaktywacja kategorii (`FR-004`).** — Odłożone do v2 zgodnie z PRD, aby nie zmieniać historycznych podsumowań.
- **Edycja lub usuwanie transakcji (`FR-008`).** — Odłożone do v2; nie należy rozszerzać pierwszego przepływu poza zapis i odczyt.
- **Multi-user i współdzielenie budżetu.** — Poza MVP; obecny model pozostaje single-user.
- **Wykresy graficzne.** — Poza MVP; tabela wystarcza do pierwszego sprawdzenia przepływu.
- **Skanowanie paragonów i integracja z bankiem.** — Poza MVP; dane są wprowadzane ręcznie.
- **Push, alerty email i praca offline.** — Poza MVP; jedynym sygnałem budżetowym pozostaje wskaźnik widoczny w podsumowaniu.
- **Pełny proces testów i lintu w CI.** — Obecny deploy jest skonfigurowany, ale przy celu szybkości ta poprawa operacyjna nie wyprzedza pierwszego działającego przepływu użytkowego.

## Done

(Puste przy pierwszym wygenerowaniu. `/10x-archive` dopisuje tu zarchiwizowane elementy i zmienia ich status na `done`.)
