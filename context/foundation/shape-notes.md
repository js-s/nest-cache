---
project: "NestCash"
context_type: greenfield
created: 2026-05-19
updated: 2026-05-19
product_type: web-app
target_scale:
  users: small
  qps: low
  data_volume: small
timeline_budget:
  mvp_weeks: 3
  hard_deadline: 2026-07-05
  after_hours_only: true
checkpoint:
  current_phase: 8
  phases_completed: [1, 2, 3, 4, 5, 6, 7]
  gray_areas_resolved:
    - topic: "pain category"
      decision: "oba — dane rozproszone I za dużo tarcia"
    - topic: "primary persona scope"
      decision: "ja i moja rodzina / partner"
    - topic: "auth model"
      decision: "email + hasło (MVP); OAuth na przyszłość; wiele kont w jednym gospodarstwie"
    - topic: "role separation"
      decision: "Admin (tworzy kategorie, zaprasza) + Członek (wprowadza, ogląda)"
    - topic: "scope decision"
      decision: "Scope down: bez multi-user, bez wykresów, kategorie user-defined, tabele wystarczą"
    - topic: "domain rule threshold"
      decision: "stały próg 80% w MVP (z config file), edytowalny per-user w v2"
  frs_drafted: 9
  quality_check_status: accepted
---

## Vision & Problem Statement

Domowy budżet ginie w rozproszonych arkuszach i plikach tekstowych bez spójnego formatu. Podwójny problem: dane są rozproszone (nie da się ich sensownie podsumować) ORAZ proces wprowadzania jest zbyt uciążliwy (co prowadzi do porzucenia po kilku tygodniach). NestCash daje jedno miejsce z niskim tarciem wejścia i elastycznym podglądem.

Insight: motywacja to nauka budowania aplikacji, pełna kontrola nad danymi (self-hosted/private) i platforma do dalszego rozwoju (nowe klienty, OCR paragonów, inflacja). To nie jest produkt rynkowy — to narzędzie osobiste z ambicją edukacyjną.

## User & Persona

Osoba zarządzająca budżetem domowym — autor projektu i jego rodzina/partner. Kilka osób we wspólnym gospodarstwie domowym dzielących jeden budżet. Główny użytkownik wprowadza dane i ogląda podsumowania; pozostali członkowie rodziny mogą mieć dostęp do podglądu lub własnego wprowadzania.

## Access Control

Login: email + hasło (MVP). OAuth (Google/Apple) rozważane na przyszłość — poza zakresem MVP.

MVP: jeden użytkownik per konto. Brak ról — jedyny user jest de facto adminem (tworzy kategorie, wprowadza dane, ogląda podsumowania). Multi-user (współdzielenie danych w ramach gospodarstwa domowego) i role (Admin / Członek) odłożone do v2.

## Success Criteria

### Primary
- Użytkownik może zarejestrować się, zalogować, utworzyć kategorie, wprowadzić wydatki/przychody, i zobaczyć tabelaryczne podsumowanie filtrowane po okresie i kategorii.

### Secondary
- Dane nie giną po ponownym logowaniu (persistence działa poprawnie).

### Guardrails
- Dane użytkownika są prywatne — nikt inny nie widzi moich operacji (dopóki multi-user nie wejdzie w v2).
- Hasło nie jest przechowywane w plaintext.

## Deferred to v2
- Multi-user / współdzielenie danych w ramach gospodarstwa domowego (+ role admin/członek)
- Wykresy graficzne (pie chart, bar chart)
- OAuth (Google/Apple)
- Edycja/dezaktywacja kategorii (zamiast usuwania — aby nie osierocić transakcji)
- Edycja/usuwanie wprowadzonych transakcji
- Filtrowanie po produkcie/opisie

## Functional Requirements

### Authentication
- FR-001: User can register with email and password. Priority: must-have
  > Socrates: Counter-argument considered: "rejestracja to bariera wejścia / auth od zera to duży koszt." Resolution: kept; rejestracja konieczna dla prywatności danych, podstawowy formularz nie jest dużą barierą, gotowe SaaS też mają koszt wdrożenia.
- FR-002: User can log in with email and password. Priority: must-have
  > Socrates: Counter-argument considered: "częste logowanie = tarcie zabijające nawyk." Resolution: kept; sesja użytkownika nie powinna wygasać zbyt szybko — long-lived session.

### Categories
- FR-003: User can create expense/income categories; predefined categories available on first use. Priority: must-have
  > Socrates: Counter-argument considered: "wymuszenie kategorii na starcie to bariera." Resolution: enhanced; dodane predefiniowane kategorie na start (plik konfiguracyjny z defaults), użytkownik może tworzyć własne obok nich.
- FR-004: User can edit/disable categories. Priority: nice-to-have (deferred to v2)
  > Socrates: Counter-argument considered: "usunięcie kategorii z transakcjami = osierocone dane." Resolution: dropped from MVP; w v2 dezaktywacja zamiast usuwania, aby nie zmieniać historycznych podsumowań.

### Transactions
- FR-005: User can enter an expense (amount + category required; date defaults to today; description optional). Priority: must-have
  > Socrates: Counter-argument considered: "4 pola = za dużo tarcia." Resolution: simplified; data domyślnie dziś, opis opcjonalny — minimalne wymagane pola to kwota + kategoria.
- FR-006: User can enter an income (amount, category, date, description). Priority: must-have
  > Socrates: Counter-argument considered: "oddzielny formularz to overkill." Resolution: kept separate; przychody mają inną strukturę niż wydatki, rozdzielenie ma sens.
- FR-007: User can view a paginated list of their operations. Priority: must-have
  > Socrates: Counter-argument considered: "bez paginacji lista stanie się nieczytelna po roku." Resolution: kept with pagination added.
- FR-008: User can edit/delete entered transactions. Priority: nice-to-have (deferred)

### Summaries
- FR-009: User can view a tabular summary with combined filters (period + category simultaneously). Priority: must-have
  > Socrates: Counter-argument considered: "dwa oddzielne filtry vs jeden widok z dwoma filtrami." Resolution: merged FR-009 and FR-010 into one; combined filters are more useful than separate views.

## User Stories

### US-01: Użytkownik wprowadza wydatek i widzi go w podsumowaniu

- **Given** zalogowany użytkownik z co najmniej jedną zdefiniowaną kategorią wydatków
- **When** wprowadza wydatek (kwota + kategoria + data + opis)
- **Then** widzi ten wydatek na liście operacji / w tabeli podsumowania

#### Acceptance Criteria
- Wydatek pojawia się natychmiast po zapisaniu (bez reloadu)
- Wydatek jest widoczny w filtrze po kategorii, do której został przypisany
- Wydatek jest widoczny w filtrze po okresie, w którym wypada jego data

## Business Logic

NestCash oblicza stosunek wydatków do przychodów w danym okresie i sygnalizuje użytkownikowi przekroczenie progu bezpieczeństwa budżetowego (domyślnie 80%).

Inputs: suma wydatków i suma przychodów w wybranym okresie (oba wartości obliczane automatycznie z wprowadzonych transakcji).

Output: wskaźnik budżetowy (ratio wydatki/przychody); przy >= 80% — pasywny monit w UI informujący o zbliżaniu się do limitu.

Encounter: użytkownik widzi wskaźnik na stronie podsumowania jako pasywny element (np. kolorowy banner). Nie jest to push notification ani email — to in-app indicator widoczny tylko kiedy użytkownik ogląda dashboard.

MVP: próg 80% stały (hardcoded). V2: próg edytowalny przez użytkownika.

Dodatkowe reguły:
- Calculation: automatyczna agregacja sum per kategoria i okres.
- Validation: wymuszanie spójności danych (kategoria musi istnieć, kwota > 0).

## Non-Functional Requirements

- Czas odpowiedzi: użytkownik widzi rezultat każdej operacji w < 2 sekundy.
- Prywatność danych: dane finansowe użytkownika nie są dostępne dla nikogo poza nim samym. Hasła hashowane, nigdy w plaintext.
- Przeglądarki: aplikacja działa poprawnie w najnowszych 2 wersjach Chrome, Firefox, Safari i Edge.
- Retencja danych: dane użytkownika przechowywane bezterminowo (dopóki użytkownik nie usunie konta).

## Non-Goals

- Skanowanie paragonów / OCR — zbyt duża złożoność na MVP; wymaga integracji z zewnętrznym serwisem lub modelem ML.
- Integracja z bankiem (API bankowe, scraping) — dane wprowadzane ręcznie; automatyczny import to odrębny moduł.
- Multi-user / współdzielenie danych w ramach rodziny — odłożone do v2; MVP jest single-user.
- Push notifications / email alerts — aplikacja nie wysyła nic proaktywnie; jedyny sygnał to pasywny wskaźnik w UI.
- Offline-first / praca bez internetu — aplikacja wymaga połączenia z serwerem do działania.

## Forward: tech-stack

Notatki informacyjne dla downstream tech-stack-selection (NIE część PRD):

- Architektura API-first: wspólny backend API dla wszystkich klientów (web teraz, mobile w przyszłości). Frontend i backend jako oddzielne deployable units.
- Próg budżetowy (80%) w pliku konfiguracyjnym — nie hardcoded w kodzie, łatwy do zmiany bez redeployu.
- Predefiniowane kategorie w oddzielnym pliku/seed — łatwe do modyfikacji bez zmiany kodu.
