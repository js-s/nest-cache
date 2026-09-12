# Spójna stylizacja interfejsu webowego — Plan implementacji

## Overview

Nadać SPA NestCash schludny, spójny wygląd bez dodawania zależności: przepisać resztkowy `style.css` ze startera Vite na zestaw tokenów CSS + klasy bazowe, uporządkować shell (`#app`, `App.vue`, `index.html`), a następnie oprzeć na nich ekrany logowania i rejestracji oraz placeholder `Summary`. Efekt: panel logowania/rejestracji wygląda porządnie, pola są większe, układ logiczny, całość działa w jasnym i automatycznym ciemnym motywie.

## Current State Analysis

- `web/src/style.css` to w całości pozostałość `create-vite` — klasy `.hero`, `.counter`, `#next-steps`, `#social`, `.ticks`, `.logo` nie są używane przez żaden komponent (`web/src/views/*.vue`, `web/src/App.vue`). Jednocześnie wymusza niezamierzony layout: `#app { width: 1126px; border-inline: 1px solid; text-align: center }` (`web/src/style.css:159-169`) i `h1 { font-size: 56px }` (`web/src/style.css:64-72`).
- `web/src/App.vue:19-30` renderuje goły `<header><nav>` + `<router-view />`, bez wspólnego kontenera ani klas; logika (`ensureSession`, `logout`) jest minimalna i poprawna — nie ruszamy jej.
- `web/src/views/Login.vue:39-50` i `web/src/views/Register.vue:38-48`: `<main>` z `h1`, formularz z inline `<label>Tekst <input></label>`, `<p role="alert">` na błąd, `<button>`. Brak kontenera, spacingu, stanów focus/hover, wspólnego rozmiaru pól.
- `web/src/views/Summary.vue:7-13`: placeholder `main` + akapit.
- Brak frameworka CSS i biblioteki komponentów — zależności to wyłącznie `vue` i `vue-router` (`web/package.json:11-14`).
- Brak testów i lintera frontendu; jedyna bramka jakości to `npm --prefix web run build` (`vue-tsc -b && vite build`, `web/package.json:8`).
- Motyw ciemny obsługiwany już przez `prefers-color-scheme` (`web/src/style.css:33-51`) — zachowujemy ten mechanizm.
- Martwe assety startera: `web/src/assets/hero.png`, `web/src/assets/vite.svg`, `web/src/assets/vue.svg` nie są importowane nigdzie w `web/src`.

## Desired End State

Ekran logowania i rejestracji to wyśrodkowana karta z etykietami nad polami, wyraźnie większymi polami, widocznym focusem, spójnym komunikatem błędu i przyciskiem z jasnym stanem disabled. Header ma czytelny brand, e-mail zalogowanego i przycisk wylogowania; treść siedzi we wspólnym kontenerze o rozsądnej szerokości. To samo w ciemnym motywie i na wąskim ekranie. W `style.css` zostają tokeny i klasy wielokrotnego użytku (m.in. `.card`, `.field`, `.btn`, `.table`), na których kolejne widoki domenowe (S-03+) mogą się oprzeć bez przepisywania globalnych stylów.

Weryfikacja: `npm --prefix web run build` przechodzi; `npm --prefix web run dev` pokazuje login/rejestrację/summary w obu motywach bez rozjechanego layoutu i bez poziomego przewijania.

### Key Discoveries:

- Stary `style.css` jest w całości martwy, ale aktywnie psuje layout (`#app` o stałej szerokości i obramowaniu) — wymiana w całości, nie dopisywanie.
- Komponenty auth trzymają logikę walidacji/API w `<script setup>` (`Login.vue:13-36`, `Register.vue:13-35`) — plan dotyka wyłącznie `<template>` i klas, bez zmian zachowania.
- Brak jakiegokolwiek runnera testów frontendu — nie dodajemy go w tym change; bramką pozostaje build + weryfikacja manualna.
- Rozmiar pól: obecne inputy dziedziczą font 18px, ale bez paddingu/min-height; podniesienie do `min-height` ~2.75rem i `font-size` 1rem w bazie załatwia wszystkie ekrany naraz.

## What We're NOT Doing

- Bez dodawania frameworka CSS / biblioteki komponentów (Pico, Tailwind itp.) i bez nowych zależności.
- Bez zmian w logice auth, routingu, kliencie API i backendzie.
- Bez zmian copy/treści formularzy i bez i18n (teksty pozostają jak są).
- Bez projektowania konkretnych przyszłych widoków domenowych — dostarczamy tylko wspólne klasy/ kontrakt, nie ekrany kategorii czy operacji.
- Bez brandingu (logo, animacje, własna paleta marek) i bez setupu testów/lintera frontendu.

## Implementation Approach

Dwie fazy bazowe plus rozszerzenie: Faza 1 buduje kontrakt wizualny (tokeny + klasy bazowe z motywem zależnym od ustawienia systemu), Faza 2 sprowadza ekrany do tego kontraktu, a Faza 3 (dodana po review) dokłada ręczny przełącznik jasny/ciemny z zapisem wyboru. Wcześniej plan świadomie pomijał przełącznik; decyzja zmieniona na życzenie użytkownika, który chciał kontroli w aplikacji.

## Critical Implementation Details

- **Kolejność**: klasy bazowe i tokeny muszą powstać w Fazie 1, bo `<template>` Fazy 2 odwołują się do ich nazw (`.card`, `.field`, `.btn`, `.stack`, `.alert`). Nie zmieniać nazw klas między fazami.
- **`#app`**: obecne `width`/`border-inline`/`text-align: center` (`style.css:159-169`) łamie układ formularzy i header. Zastąpić kontenerem (`.page` / `.shell`) z `margin-inline: auto`, `max-width`, bez wymuszonego `text-align`.
- **Breakpointy**: brak istniejącej siatki; wystarczy jeden breakpoint mobilny (~640px) i `box-sizing: border-box` globalnie. Nie budować rozbudowanego systemu responsywnego.
- **Dark mode**: nadpisywać wyłącznie wartości tokenów w `@media (prefers-color-scheme: dark)` (wzorzec już obecny w `style.css:33-51`) — nie duplikować reguł komponentów.

## Phase 1: Bazowy styl globalny i shell

### Overview

Ustanowić tokeny i klasy bazowe oraz uporządkować shell aplikacji; usunąć martwe style i assety startera.

### Changes Required:

#### 1. Globalny styl bazowy

**File**: `web/src/style.css`

**Intent**: Zastąpić całą zawartość pliku (starterowe `.hero`, `.counter`, `#next-steps`, `#social`, `.ticks` są martwe) tokenami i klasami bazowymi, na których oprą się wszystkie ekrany. Zachować wzorzec dark mode przez `prefers-color-scheme`.

**Contract**: Zdefiniować tokeny: `--color-bg`, `--color-surface`, `--color-text`, `--color-text-muted`, `--color-border`, `--color-accent`, `--color-accent-contrast`, `--color-danger`, `--space-1..6`, `--radius`, `--shadow`, `--font-sans`, `--content-width`. Klasy: `.page` (wyśrodkowany kontener o `max-width: var(--content-width)`), `.card` (powierzchnia z border/padding/radius/shadow), `.stack` (pionowy rytm przez `gap`), `.muted`, `.alert`, `.alert-error`, `.field` (label + kontrolka), `.btn`, `.btn-primary`, `.btn-secondary`, `.table`. Bazowe elementy: `box-sizing: border-box` globalnie, `body` bez marginesu, inputy/select/textarea o `min-height: 2.75rem`, `font-size: 1rem`, padding i `border-radius`, z `:focus-visible` (widoczny ring) oraz `:disabled`. Ciemny motyw = tylko nadpisanie tokenów.

#### 2. Shell aplikacji

**File**: `web/src/App.vue`

**Intent**: Owinąć header i treść w spójny layout i dodać klasy; nie zmieniać logiki `ensureSession`/`logout` ani warunków `v-if`.

**Contract**: `header` z klasą (np. `.app-header`) zawiera `.page` z brandem, e-mailem użytkownika i przyciskiem wylogowania (`btn btn-secondary`); `<router-view />` trafia do głównego kontenera (np. `<main class="page">`). Układ: header na górze, treść pod nim, bez wymuszonego `text-align: center`.

#### 3. Tytuł dokumentu

**File**: `web/index.html`

**Intent**: Zmienić `<title>` z „web” na „NestCash”.

**Contract**: Tylko zawartość `<title>`; `lang`, viewport i favicon bez zmian.

#### 4. Usunięcie martwych assetów startera

**File**: `web/src/assets/hero.png`, `web/src/assets/vite.svg`, `web/src/assets/vue.svg`

**Intent**: Usunąć nieużywane pliki startera (potwierdzone: brak importów w `web/src`). Zostawić `web/public/favicon.svg` i `web/public/icons.svg`.

**Contract**: Usunięcie plików; brak zmian w referencjach (żadnych).

### Success Criteria:

#### Automated Verification:

- Build przechodzi: `npm --prefix web run build`
- Brak odwołań do martwych klas/assetów: w `web/src` nie występują `hero`, `counter`, `#next-steps`, `#social`, `hero.png`, `vite.svg`, `vue.svg`

#### Manual Verification:

- Shell (header + kontener treści) wygląda spójnie w jasnym i ciemnym motywie
- Na szerokości desktop oraz ~380px brak poziomego przewijania i rozjechanych elementów

**Implementation Note**: Po Fazie 1 i przejściu weryfikacji automatycznej zatrzymać się po potwierdzenie manualne przed Fazą 2. Checkboxy dla tych pozycji są w sekcji `## Progress` na końcu planu.

---

## Phase 2: Ekrany auth i Summary

### Overview

Sprowadzić ekrany logowania, rejestracji i placeholder Summary do klas bazowych z Fazy 1, zachowując całą logikę skryptów.

### Changes Required:

#### 1. Ekran logowania

**File**: `web/src/views/Login.vue`

**Intent**: Owinąć treść kartą i ułożyć formularz logicznie: nagłówek, pola z etykietami nad kontrolkami, komunikat błędu, akcja, link do rejestracji. Zachować bez zmian `valid()`, `submit()`, `v-model`, `:disabled`, `role="alert"` i komunikaty.

**Contract**: `<main class="stack">` → `.card` z `h1`, formularz z `.field` na każde pole (email, hasło), `.alert .alert-error` dla błędu, `.btn .btn-primary` dla submitu; stopka z linkiem `router-link`. Pola większe przez klasy bazowe, nie przez inline style.

#### 2. Ekran rejestracji

**File**: `web/src/views/Register.vue`

**Intent**: Analogicznie do logowania; zachować różnice copy („Create account”, „Password (min 8)”) i logikę `email_taken`.

**Contract**: Ten sam wzorzec `.card` / `.field` / `.alert` / `.btn` co w `Login.vue`; link do logowania w stopce.

#### 3. Placeholder Summary

**File**: `web/src/views/Summary.vue`

**Intent**: Sprawić, by placeholder nie wyglądał na goły tekst — użyć kontenera/karty i klasy `.muted`.

**Contract**: `<main class="page stack">` z `h1` i akapitami; brak zmian w skrypcie.

### Success Criteria:

#### Automated Verification:

- Build przechodzi: `npm --prefix web run build`

#### Manual Verification:

- Login i Register: etykiety nad polami, pola wyraźnie większe, logiczny układ, widoczny focus
- Walidacja bez regresji: przycisk disabled przy nieprawidłowym formularzu, błąd pokazany w spójnym stylu
- Summary korzysta z kontenera/karty i nie wygląda na goły tekst
- Oba ekrany poprawne w ciemnym motywie i na wąskim (~380px) ekranie

**Implementation Note**: Zatrzymać się po Fazie 2 i potwierdzić manualnie łącznie z Fazą 1.

---

## Phase 3: Przełącznik motywu jasny/ciemny

### Overview

Dodać ręczny przełącznik motywu w headerze. Domyślnie motyw idzie za systemem; po pierwszym kliknięciu wybór jest zapamiętywany (bez trybu auto — świadoma decyzja, żeby nie mnożyć stanów).

### Changes Required:

#### 1. Moduł motywu

**File**: `web/src/theme.ts` (nowy)

**Intent**: Trzymać reaktywny stan motywu i logikę przełączania z zapisem w `localStorage`; nasłuchiwać zmian systemowych tylko dopóki użytkownik nie wybrał ręcznie.

**Contract**: Eksport `theme: Ref<'light' | 'dark'>` i `toggleTheme(): void`. Klucz storage `nestcash-theme`. `effectiveTheme()` czyta `document.documentElement.dataset.theme`, a gdy brak — `matchMedia('(prefers-color-scheme: dark)')`. `toggleTheme()` ustawia `data-theme`, zapisuje wybór i aktualizuje ref.

#### 2. Przycisk w headerze

**File**: `web/src/App.vue`

**Intent**: Pokazać przełącznik zawsze (także na ekranach auth), obok ewentualnego e-maila i wylogowania. Etykieta pokazuje docelowy motyw.

**Contract**: `import { theme, toggleTheme } from './theme'`; `<button type="button" class="btn btn-secondary" @click="toggleTheme">` z tekstem `theme === 'dark' ? 'Light mode' : 'Dark mode'`. Kontener `.app-header__user` renderuje się zawsze (e-mail i wylogowanie nadal pod `v-if="user"`).

#### 3. Tokeny sterowane `data-theme`

**File**: `web/src/style.css`

**Intent**: Umożliwić ręczne nadpisanie motywu bez duplikowania tokenów i bez migotania.

**Contract**: Wartości kolorów i cieni przez `light-dark(light, dark)`; `--shadow` złożony z tokenów `--shadow-1/2`. `:root[data-theme='light'] { color-scheme: light }` i `:root[data-theme='dark'] { color-scheme: dark }`. Dotychczasowy blok `@media (prefers-color-scheme: dark)` usunięty — bez `data-theme` `color-scheme: light dark` nadal podąża za systemem.

#### 4. Brak migotania przy starcie

**File**: `web/index.html`

**Intent**: Ustawić `data-theme` z `localStorage` przed pierwszym malowaniem, żeby zapisany motyw nie mrugał, gdy różni się od systemowego.

**Contract**: Krótki inline `<script>` w `<head>` czytający `nestcash-theme` i ustawiający `document.documentElement.dataset.theme` w `try/catch`.

### Success Criteria:

#### Automated Verification:

- Build przechodzi: `npm --prefix web run build`

#### Manual Verification:

- Klik w headerze przełącza jasny/ciemny na login, rejestracji i summary
- Wybór przetrwa odświeżenie strony; brak mignięcia przy zapisanym motywie innym niż systemowy
- Brak wyboru = motyw nadal podąża za systemem

**Implementation Note**: Rozszerzenie zakresu po review — potwierdzić manualnie.

---

## Testing Strategy

### Unit Tests:

- Brak — frontend nie ma runnera testów i ten change go nie wprowadza (poza zakresem; roadmapa S-02 jest czysto prezentacyjny).

### Integration Tests:

- Brak — brak harnessu; bramką jest `npm --prefix web run build`.

### Manual Testing Steps:

1. `npm --prefix web run dev`, otworzyć `/login` i `/register` w jasnym motywie; sprawdzić układ, rozmiar pól, focus (Tab), stan disabled przy pustym/błędnym formularzu.
2. Wywołać błąd (złe hasło / zajęty e-mail) i sprawdzić styl komunikatu oraz brak regresji w logice.
3. Przełączyć systemowy motyw na ciemny i powtórzyć kroki 1–2.
4. Zmniejszyć szerokość do ~380px — brak poziomego przewijania, pola i przyciski pełnej szerokości.
5. Wejść na `/summary` po zalogowaniu — placeholder w kontenerze/karcie.

## Performance Considerations

Bez zmian funkcjonalnych; wymiana CSS na mniejszy, tokenowy arkusz. Brak nowych assetów/ fontów zewnętrznych (używać `system-ui`), więc brak dodatkowych requestów.

## Migration Notes

Brak migracji danych. Cofnięcie = revert commitów Fazy 1–2; `style.css` jest przepisywany w całości, więc rollback przywraca poprzednią wersję pliku.

## References

- Roadmapa: `context/foundation/roadmap.md` (S-02, Change ID `web-ui-styling`)
- PRD: `context/foundation/prd.md` (FR-001, FR-002, NFR – przeglądarki)
- Stack: `context/foundation/tech-stack-web.md` (Vite + Vue 3 + TS, brak frameworka CSS)
- Komponenty: `web/src/App.vue:19-30`, `web/src/views/Login.vue:39-50`, `web/src/views/Register.vue:38-48`, `web/src/views/Summary.vue:7-13`
- Style do wymiany: `web/src/style.css`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Bazowy styl globalny i shell

#### Automated

- [x] 1.1 Build przechodzi: `npm --prefix web run build`
- [x] 1.2 Brak odwołań do martwych klas/assetów w `web/src`

#### Manual

- [ ] 1.3 Shell spójny w jasnym i ciemnym motywie
- [ ] 1.4 Brak poziomego przewijania na desktop i ~380px

### Phase 2: Ekrany auth i Summary

#### Automated

- [x] 2.1 Build przechodzi: `npm --prefix web run build`

#### Manual

- [ ] 2.2 Login/Register: etykiety nad polami, większe pola, widoczny focus
- [ ] 2.3 Walidacja bez regresji (disabled + komunikat błędu)
- [ ] 2.4 Summary w kontenerze/karcie
- [ ] 2.5 Ciemny motyw i wąski ekran bez regresji

### Phase 3: Przełącznik motywu jasny/ciemny

#### Automated

- [x] 3.1 Build przechodzi: `npm --prefix web run build`

#### Manual

- [ ] 3.2 Przełącznik działa na login/rejestracji/summary
- [ ] 3.3 Wybór przetrwa odświeżenie; brak mignięcia przy starcie
- [ ] 3.4 Brak wyboru = motyw podąża za systemem
