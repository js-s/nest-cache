---
project: "NestCash"
version: 1
status: draft
created: 2026-05-19
context_type: greenfield
product_type: web-app
target_scale:
  users: small
  qps: low
  data_volume: small
timeline_budget:
  mvp_weeks: 3
  hard_deadline: 2026-07-05
  after_hours_only: true
---

## Vision & Problem Statement

Domowy budżet ginie w rozproszonych arkuszach i plikach tekstowych bez spójnego formatu. Podwójny problem: dane są rozproszone (nie da się ich sensownie podsumować) ORAZ proces wprowadzania jest zbyt uciążliwy (co prowadzi do porzucenia po kilku tygodniach). NestCash daje jedno miejsce z niskim progiem wejścia i elastycznym podglądem.

Insight: motywacja to nauka budowania aplikacji, pełna kontrola nad własnymi danymi finansowymi i platforma do dalszego rozwoju (nowe klienty, skanowanie paragonów, śledzenie inflacji, alertowanie). To nie jest produkt rynkowy — to narzędzie osobiste z ambicją edukacyjną.

## User & Persona

Osoba zarządzająca budżetem domowym — autor projektu i jego rodzina/żona. Kilka osób we wspólnym gospodarstwie domowym dzielących jeden budżet. Główny użytkownik wprowadza dane i ogląda podsumowania; pozostali członkowie rodziny mogą mieć dostęp do podglądu lub własnego wprowadzania w przyszłych wersjach.

## Success Criteria

### Primary
- Użytkownik może zarejestrować się, zalogować, utworzyć kategorie, wprowadzić wydatki/przychody, i zobaczyć tabelaryczne podsumowanie filtrowane po okresie i kategorii.

### Secondary
- Dane nie giną po ponownym logowaniu (persistence działa poprawnie).

### Guardrails
- Dane użytkownika są prywatne — nikt inny nie widzi moich operacji.
- Dane uwierzytelniające nie są przechowywane w formie umożliwiającej ich odczytanie.

## User Stories

### US-01: Użytkownik wprowadza wydatek i widzi go w podsumowaniu

- **Given** zalogowany użytkownik z co najmniej jedną zdefiniowaną kategorią wydatków
- **When** wprowadza wydatek (kwota + kategoria + data + opis)
- **Then** widzi ten wydatek na liście operacji / w tabeli podsumowania

#### Acceptance Criteria
- Wydatek pojawia się natychmiast po zapisaniu
- Wydatek jest widoczny w filtrze po kategorii, do której został przypisany
- Wydatek jest widoczny w filtrze po okresie, w którym wypada jego data

## Functional Requirements

### Authentication
- FR-001: User can register with email and password. Priority: must-have
  > Socrates: Counter-argument considered: "rejestracja to bariera wejścia / auth od zera to duży koszt." Resolution: kept; rejestracja konieczna dla prywatności danych, podstawowy formularz nie jest dużą barierą, gotowe rozwiązania auth też mają koszt wdrożenia.
- FR-002: User can log in with email and password. Priority: must-have
  > Socrates: Counter-argument considered: "częste logowanie = utrudnienie zabijające nawyk." Resolution: kept; sesja użytkownika nie powinna wygasać zbyt szybko — long-lived session.

### Categories
- FR-003: User can create expense/income categories; predefined categories available on first use. Priority: must-have
  > Socrates: Counter-argument considered: "wymuszenie kategorii na starcie to bariera." Resolution: enhanced; dodane predefiniowane kategorie na start, użytkownik może tworzyć własne obok nich.
- FR-004: User can edit/disable categories. Priority: nice-to-have (deferred to v2)
  > Socrates: Counter-argument considered: "usunięcie kategorii z transakcjami = osierocone dane." Resolution: dropped from MVP; w v2 dezaktywacja zamiast usuwania, aby nie zmieniać historycznych podsumowań.

### Transactions
- FR-005: User can enter an expense (amount + category required; date defaults to today; description optional). Priority: must-have
  > Socrates: Counter-argument considered: "4 pola = za dużo wysiłku przy wprowadzaniu danych." Resolution: simplified; data domyślnie dziś, opis opcjonalny — minimalne wymagane pola to kwota + kategoria. Ostatnio wybrana kategoria powinna być domyślnie ustawiona.
- FR-006: User can enter an income (amount, category, date, description). Priority: must-have
  > Socrates: Counter-argument considered: "oddzielny formularz to overkill." Resolution: kept separate; przychody mają inną strukturę niż wydatki, rozdzielenie ma sens.
- FR-007: User can view a paginated list of their operations. Priority: must-have
  > Socrates: Counter-argument considered: "bez paginacji lista stanie się nieczytelna po roku." Resolution: kept with pagination added.
- FR-008: User can edit/delete entered transactions. Priority: nice-to-have (deferred)

### Summaries
- FR-009: User can view a tabular summary with combined filters (period + category simultaneously). Priority: must-have
  > Socrates: Counter-argument considered: "dwa oddzielne filtry vs jeden widok z dwoma filtrami." Resolution: merged; combined filters are more useful than separate views.

## Non-Functional Requirements

- Czas odpowiedzi: użytkownik widzi rezultat każdej operacji w < 2 sekundy.
- Prywatność danych: dane finansowe użytkownika nie są dostępne dla nikogo poza nim samym. Dane uwierzytelniające nigdy nie są przechowywane w formie umożliwiającej ich odczytanie.
- Przeglądarki: aplikacja działa poprawnie w najnowszych 2 wersjach czterech głównych przeglądarek desktopowych.
- Retencja danych: dane użytkownika przechowywane bezterminowo (dopóki użytkownik nie usunie konta).

## Business Logic

NestCash oblicza stosunek wydatków do przychodów w danym okresie i sygnalizuje użytkownikowi przekroczenie progu bezpieczeństwa budżetowego (domyślnie 80%).

Inputs: suma wydatków i suma przychodów w wybranym okresie (oba wartości obliczane automatycznie z wprowadzonych transakcji).

Output: wskaźnik budżetowy (ratio wydatki/przychody); przy >= 80% — pasywny monit informujący o zbliżaniu się do limitu.

Encounter: użytkownik widzi wskaźnik na stronie podsumowania jako pasywny element wizualny. Nie jest to zewnętrzne powiadomienie — to wskaźnik widoczny tylko kiedy użytkownik ogląda podsumowanie.

Próg ustalony na 80% w początkowej wersji. W przyszłych wersjach próg konfigurowalny per-user.

Dodatkowe reguły:
- Calculation: automatyczna agregacja sum per kategoria i okres.
- Validation: wymuszanie spójności danych (kategoria musi istnieć, kwota > 0).

## Access Control

Login: email + hasło. Jeden użytkownik per konto. Brak ról — jedyny user jest de facto adminem (tworzy kategorie, wprowadza dane, ogląda podsumowania). Multi-user i podział ról odłożone do przyszłych wersji.

## Non-Goals

- Skanowanie paragonów — zbyt duża złożoność na MVP; wymaga integracji z zewnętrznym serwisem rozpoznawania obrazów.
- Automatyczne pobieranie danych z banku — dane wprowadzane ręcznie; automatyczny import to odrębny moduł.
- Multi-user / współdzielenie danych w ramach rodziny — odłożone do v2; MVP jest single-user.
- Zewnętrzne powiadomienia (push, email) — aplikacja nie wysyła nic proaktywnie; jedyny sygnał to pasywny wskaźnik w interfejsie.
- Praca bez połączenia z internetem — aplikacja wymaga połączenia z serwerem.
- Edycja/dezaktywacja kategorii — odłożona do v2; w MVP kategorie po utworzeniu są stałe.
- Edycja/usuwanie wprowadzonych transakcji — odłożone do v2.
- Alternatywne metody uwierzytelniania — poza zakresem MVP.

## Open Questions

1. **Jakie predefiniowane kategorie powinny być dostępne na start?** — Do ustalenia przy implementacji. Lista domyślnych kategorii wydatków/przychodów dla nowego użytkownika. Owner: user.
2. **Jakie opcje granulacji okresów w filtrze podsumowania?** — Dzień, tydzień, miesiąc, rok, zakres niestandardowy? Do ustalenia przy projektowaniu interfejsu. Owner: user.
3. **Jak wygląda flow resetowania/odzyskiwania hasła?** — Wymagane przez FR-001/002, ale nie opisane w user stories. Owner: user.
