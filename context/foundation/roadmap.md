---
project: NestCash
version: 1
status: draft
created: 2026-09-11
updated: 2026-09-12
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

**S-05: Użytkownik filtruje zapisany wydatek w tabelarycznym podsumowaniu** — to pierwszy przepływ, który bezpośrednio sprawdza główny cel opisany w `US-01` i kryterium sukcesu dotyczące filtrowanego podsumowania.

> Zasada realizacji: każdy slice poniżej obejmuje jednocześnie rezultat w interfejsie webowym, obsługę po stronie API oraz niezbędną obsługę danych. Frontend nie jest osobną, odroczoną fazą projektu.

## At a glance

| ID | Change ID | Outcome (user can …) | Prerequisites | PRD refs | Status |
|---|---|---|---|---|---|
| F-01 | account-data-boundary | (foundation) żądania API mają minimalny kontrakt identyfikowania właściciela danych i odrzucania dostępu do danych spoza jego konta; konkretne przepływy konta i finansów integrują ten kontrakt w kolejnych pionowych przyrostach. | — | Access Control, NFR (privacy), NFR (retention) | ready |
| S-01 | account-registration-and-login | użytkownik może zarejestrować konto i zalogować się w interfejsie webowym przez email i hasło oraz korzystać z długiej sesji. | F-01 | FR-001, FR-002, Access Control | blocked |
| S-02 | web-ui-styling | użytkownik widzi spójny, schludny interfejs webowy: uporządkowany panel logowania i rejestracji, większe pola formularzy i logiczny układ ekranów. | — | FR-001, FR-002, NFR (cross-browser) | done |
| S-03 | predefined-and-custom-categories | użytkownik może zobaczyć w interfejsie webowym predefiniowane kategorie i podkategorie przy pierwszym użyciu i utworzyć własne kategorie oraz podkategorie wydatków lub przychodów. | S-01 | FR-003 | proposed |
| S-04 | expense-entry-and-operation-list | użytkownik może wprowadzić wydatek w interfejsie webowym z kwotą, kategorią, datą i opcjonalnym opisem oraz zobaczyć go na paginowanej liście własnych operacji. | S-03 | US-01, FR-005, FR-007 | proposed |
| S-05 | filtered-expense-summary | użytkownik może zobaczyć zapisany wydatek w interfejsie webowym, w tabelarycznym podsumowaniu filtrowanym jednocześnie po okresie i kategorii. | S-04 | US-01, FR-009 | blocked |
| S-06 | income-and-budget-ratio | użytkownik może wprowadzić przychód w interfejsie webowym, a podsumowanie pokazuje pasywny wskaźnik relacji wydatków do przychodów i sygnalizuje próg 80% lub więcej. | S-05 | FR-006, Business Logic, NFR (response time) | proposed |

## Baseline

Stan repozytorium na `2026-09-11` (automatycznie zbadany i potwierdzony). Istniejący scaffold frontendowy jest punktem startowym dla pionowych slice’ów, a nie osobną jednostką roadmapy.

- **Frontend:** partial — istnieje scaffold klienta webowego, konfiguracja budowania, proxy API w trybie deweloperskim i serwowanie zbudowanego SPA; brak ekranów domenowych, połączenia z API i obsługi auth.
- **Backend / API:** partial — istnieje serwer HTTP z `/healthz`, granicą JSON `/api` i fallbackiem SPA; brak endpointów domenowych budżetu.
- **Data:** partial — istnieje opcjonalne połączenie PostgreSQL i sprawdzenie zdrowia połączenia; brak schematu, migracji, seedów i zapytań domenowych.
- **Auth:** partial — istnieje provider-neutralny kontrakt tożsamości i middleware izolacji konta; brak dostawcy, rejestracji, logowania, sesji oraz podłączenia middleware do tras.
- **Deploy / infra:** present — CI buduje frontendowy artefakt, testuje Go, a Fly wdraża API i SPA w jednym obrazie; istnieje także ręczny workflow rollbacku.
- **Observability:** partial — istnieją `/healthz`, logowanie, kontrola Fly i smoke testy wdrożenia; brak metryk, śledzenia błędów i dashboardów aplikacyjnych.

## Foundations

Frontend nie dostaje osobnej foundation: jego scaffold i ścieżka wdrożeniowa są już obecne, a pierwsza właściwa praca produktowa nad interfejsem wchodzi bezpośrednio do S-01 i kolejnych pionowych slice’ów.

### F-01: Minimalny kontrakt własności danych

- **Outcome:** (foundation) żądania API mają minimalny kontrakt identyfikowania właściciela danych i odrzucania dostępu do danych spoza jego konta; konkretne przepływy konta i finansów integrują ten kontrakt w kolejnych pionowych przyrostach.
- **Change ID:** account-data-boundary
- **PRD refs:** Access Control, NFR (privacy), NFR (retention)
- **Unlocks:** S-01, S-03, S-04, S-05, S-06
- **Prerequisites:** —
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Fundament musi pozostać minimalną granicą własności danych; rozbudowanie go do kompletnego systemu uwierzytelniania przed pierwszym przepływem zwiększyłoby koszt i opóźniło cel szybkości.
- **Status:** done

## Slices

Każdy poniższy slice jest planowany jako jeden pionowy przyrost: interfejs webowy, kontrakt API, logika i dane są rozwijane oraz weryfikowane razem. Pole „Parallel with” oznacza wyłącznie niezależne slice’y, nie równoległość frontendowej i backendowej warstwy wewnątrz slice’a.

### S-01: Rejestracja i logowanie

- **Outcome:** użytkownik może zarejestrować konto i zalogować się w interfejsie webowym przez email i hasło oraz korzystać z długiej sesji.
- **Change ID:** account-registration-and-login
- **PRD refs:** FR-001, FR-002, Access Control
- **Prerequisites:** F-01
- **Parallel with:** S-02
- **Blockers:** —
- **Unknowns:**
  - Czy odzyskiwanie lub reset hasła jest obowiązkowe w MVP, a jeśli tak, jaki jest jego minimalny przepływ? — Owner: user. Block: yes.
- **Risk:** Ekrany rejestracji i logowania muszą zostać domknięte razem z API sesji; nieuzgodniony zakres odzyskiwania hasła może zatrzymać pierwszy działający przepływ.
- **Status:** ready

### S-02: Spójny, schludny interfejs webowy

- **Outcome:** użytkownik widzi spójny i czytelny interfejs webowy — panel logowania i rejestracji jest uporządkowany, pola formularzy są większe i wygodniejsze, a układ ekranów jest logiczny zamiast surowy.
- **Change ID:** web-ui-styling
- **PRD refs:** FR-001, FR-002, NFR (cross-browser)
- **Prerequisites:** —
- **Parallel with:** S-01, S-03, S-04, S-05, S-06
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Ma to być lekkie uporządkowanie istniejących ekranów i wspólnych stylów, nie pełny system designu; wykonanie go wcześnie ustawia bazę wizualną przed ekranami kategorii i operacji i nie blokuje gwiazdy przewodniej.
- **Status:** done

### S-03: Kategorie i podkategorie startowe oraz własne

- **Outcome:** użytkownik może zobaczyć w interfejsie webowym predefiniowane kategorie i podkategorie przy pierwszym użyciu oraz utworzyć własne kategorie i podkategorie wydatków lub przychodów (np. grupa „Codzienne” z podkategoriami „Artykuły spożywcze” i „Chemia gospodarcza”).
- **Change ID:** predefined-and-custom-categories
- **PRD refs:** FR-003
- **Prerequisites:** S-01
- **Parallel with:** S-02
- **Blockers:** —
- **Unknowns:** —
- **Risk:** FR-003 mówi o kategoriach, a wprowadzenie dwóch poziomów (kategoria → podkategoria) rozszerza zakres, więc model danych, API i widok muszą od początku nieść hierarchię. Predefiniowana lista pochodzi z `resources/categories.yaml` (wydatki w grupach, przychody płaskie), co domyka wcześniejsze otwarte pytanie.
- **Status:** proposed

### S-04: Wprowadzenie wydatku i lista operacji

- **Outcome:** użytkownik może wprowadzić wydatek w interfejsie webowym z kwotą, kategorią, datą i opcjonalnym opisem oraz zobaczyć go na paginowanej liście własnych operacji.
- **Change ID:** expense-entry-and-operation-list
- **PRD refs:** US-01, FR-005, FR-007
- **Prerequisites:** S-03
- **Parallel with:** S-02
- **Blockers:** —
- **Unknowns:** —
- **Risk:** To pierwszy pełny przepływ finansowy przez formularz webowy, API i bazę; błąd w walidacji kwoty, kategorii lub właściciela danych podważy późniejsze podsumowania.
- **Status:** proposed

### S-05: Filtrowane podsumowanie wydatku

- **Outcome:** użytkownik może zobaczyć zapisany wydatek w interfejsie webowym, w tabelarycznym podsumowaniu filtrowanym jednocześnie po okresie i kategorii.
- **Change ID:** filtered-expense-summary
- **PRD refs:** US-01, FR-009
- **Prerequisites:** S-04
- **Parallel with:** S-02
- **Blockers:** —
- **Unknowns:**
  - Jakie opcje granulacji okresu mają być dostępne w filtrze: dzień, tydzień, miesiąc, rok czy zakres niestandardowy? — Owner: user. Block: yes.
- **Risk:** Ten slice jest gwiazdą przewodnią roadmapy i musi połączyć ekran podsumowania z działającym kontraktem API; niejasna granulacja może wymusić zmianę filtrów po obu stronach.
- **Status:** blocked

### S-06: Przychód i wskaźnik budżetowy

- **Outcome:** użytkownik może wprowadzić przychód w interfejsie webowym, a podsumowanie pokazuje pasywny wskaźnik relacji wydatków do przychodów i sygnalizuje próg 80% lub więcej.
- **Change ID:** income-and-budget-ratio
- **PRD refs:** FR-006, Business Logic, NFR (response time)
- **Prerequisites:** S-05
- **Parallel with:** S-02
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Formularz przychodu, agregacja API i wskaźnik w podsumowaniu mają sens dopiero po działającym przepływie wydatku; wynik musi pozostać pasywnym sygnałem bez dokładania powiadomień poza zakresem.
- **Status:** proposed

## Backlog Handoff

| Roadmap ID | Change ID | Suggested issue title | Ready for `/10x-plan` | Notes |
|---|---|---|---|---|
| F-01 | account-data-boundary | Ustanowić minimalną granicę własności danych | yes | Odblokowuje cały łańcuch funkcji finansowych. |
| S-01 | account-registration-and-login | Dodać rejestrację, logowanie i długą sesję w UI oraz API | no | Najpierw rozstrzygnąć minimalny zakres resetu hasła. |
| S-02 | web-ui-styling | Uporządkować i ujednolicić wygląd interfejsu webowego | yes | Niezależny od reszty; można planować natychmiast, także równolegle z S-01. |
| S-03 | predefined-and-custom-categories | Dodać kategorie i podkategorie w UI oraz API | no | Zależne od S-01; lista startowa w `resources/categories.yaml`. |
| S-04 | expense-entry-and-operation-list | Dodać wprowadzanie wydatków i listę operacji end-to-end | no | Zależne od S-03; obejmuje formularz webowy, API i zapis danych. |
| S-05 | filtered-expense-summary | Dodać filtrowane podsumowanie wydatków w UI oraz API | no | Zależne od S-04 i decyzji o granulacji okresów. |
| S-06 | income-and-budget-ratio | Dodać przychody i wskaźnik budżetowy end-to-end | no | Zależne od S-05; obejmuje formularz, agregację i wskaźnik w UI. |

Ten handoff zachowuje jeden backlog pionowych rezultatów. Nie tworzy osobnych zadań typu „frontend” i „backend”, ponieważ ich rozdzielenie opóźniłoby integrację wymaganą przez każdy slice.

## Open Roadmap Questions

1. **Jakie opcje granulacji okresów w filtrze podsumowania?** — Owner: user. Block: S-05.
2. **Jak wygląda flow resetowania/odzyskiwania hasła?** — Owner: user. Block: S-01.
3. **Czy termin `2026-07-05` zapisany w PRD jest nadal wiążący?** — Owner: user. Block: roadmap-wide (wpływa na kolejność, ale nie blokuje samego planowania elementów).

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
