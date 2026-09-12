# Spójna stylizacja interfejsu webowego — Plan Brief

> Full plan: `context/changes/web-ui-styling/plan.md`

## What & Why

Obecny SPA to resztki startera Vite: `web/src/style.css` pełen martwych klas (`.hero`, `.counter`, `#next-steps`) i wymuszony layout (`#app` 1126px z obramowaniem, `h1` 56px), a ekrany logowania/rejestracji to gołe `<main>` z inline labelami przy polach. Chcemy, żeby panel logowania i rejestracji wyglądał schludnie, pola były większe, a układ logiczny — bez dokładania zależności. Cel roadmapy S-02: spójny, czytelny interfejs.

## Starting Point

Vue 3 + vue-router, zero frameworka CSS i biblioteki komponentów (`web/package.json`). Logika auth (walidacja, `v-model`, `:disabled`, komunikaty) jest już poprawna w `Login.vue:13-36` i `Register.vue:13-35` — plan nie rusza skryptów. Motyw ciemny obsługiwany już przez `prefers-color-scheme` (`style.css:33-51`). Brak testów/lintu frontendu; jedyna bramka to `npm --prefix web run build`.

## Desired End State

Login i rejestracja to wyśrodkowana karta z etykietami nad polami, wyraźnie większymi polami, widocznym focusem, spójnym komunikatem błędu i przyciskiem ze stanem disabled. Header ma brand, e-mail i wylogowanie oraz przełącznik jasny/ciemny z zapamiętanym wyborem; treść w kontenerze o rozsądnej szerokości. Wszystko działa w jasnym i ciemnym motywie oraz na ~380px. `style.css` zawiera tokeny + klasy wielokrotnego użytku (`.card`, `.field`, `.btn`, `.table`), na których oprą się kolejne widoki domenowe.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Zakres | Shell + auth + kontrakt pod przyszłe widoki | Roadmapa S-02 mówi „spójny shell”, a wspólne klasy redukują przeróbki w S-03+. |
| Podejście | Czysty CSS z tokenami, bez zależności | Zgodne z `tech-stack-web.md` i minimalizmem; pełna kontrola i trywialny rollback. |
| Motyw | Jasny + automatyczny dark mode | Mechanizm `prefers-color-scheme` już istnieje — zachowujemy jako domyślny. |
| Przełączanie motywu | Ręczny przełącznik jasny/ciemny, wybór w `localStorage` | Użytkownik chciał kontroli w aplikacji; bez nowych zależności. |
| Podział faz | Baza/shell osobno od ekranów | Łatwy review i cofnięcie; brak zmian w logice skryptów. |

## Scope

**In scope:** przepisanie `web/src/style.css` na tokeny + klasy; uporządkowanie `App.vue`; `index.html` title; restyl `Login.vue`, `Register.vue`, `Summary.vue` (tylko `<template>`/klasy); usunięcie martwych assetów startera; przełącznik jasny/ciemny w headerze z zapisem wyboru.

**Out of scope:** frameworki CSS / Tailwind, nowe zależności, trzeci stan „auto” po ręcznym wyborze, zmiany logiki auth/API/routingu, zmiany copy/i18n, projektowanie przyszłych widoków domenowych, branding, setup testów/lintera.

## Architecture / Approach

Dwie fazy. Faza 1 ustanawia kontrakt wizualny w jednym miejscu (`style.css`): tokeny (kolory, spacje, promienie, typografia) z wariantem ciemnym oraz klasy bazowe (kontener, karta, stos, pola, przyciski, alert, tabela), plus shell. Faza 2 sprowadza istniejące widoki do tych klas, głównie w `<template>`, zachowując skrypty bez zmian.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Bazowy styl globalny i shell | Tokeny + klasy bazowe, uporządkowany `#app`/header, czysty `style.css` | Wymiana całego `style.css` — trzeba usunąć wszystkie zależności od starych klas |
| 2. Ekrany auth i Summary | Schludny login/rejestracja i placeholder na klasach bazowych | Rozjazd z nazwami klas z Fazy 1; regresja logiki przy edycji `<template>` |
| 3. Przełącznik motywu | Przycisk jasny/ciemny w headerze z zapisem wyboru | Mignięcie przy starcie; kolizja z systemowym motywem (rozwiązane inline scriptem) |

**Prerequisites:** brak — zmiana samodzielna, frontend-only. Node/npm dostępne do `npm --prefix web run build`.
**Estimated effort:** ~1 sesja, 3 fazy (diff mały: 1 plik CSS + `theme.ts` + 4 pliki Vue + `index.html` + usunięcie 3 assetów).

## Open Risks & Assumptions

- Brak testów i lintera frontendu → realną weryfikacją jest build + manualny przegląd w obu motywach i na wąskim ekranie.
- `Summary.vue` to placeholder — jego wygląd zmieni się jeszcze w S-03/S-05; traktujemy go tylko minimalnie.
- Dobór dokładnych kolorów/akcentu zostaje do implementacji (neutralny motyw + jeden subtelny akcent), bez osobnej decyzji.

## Success Criteria (Summary)

- `npm --prefix web run build` przechodzi; brak odwołań do martwych klas/assetów.
- Login/rejestracja: większe pola, etykiety nad polami, widoczny focus, spójny błąd, bez regresji walidacji.
- Spójny wygląd w jasnym i ciemnym motywie oraz na ~380px bez poziomego przewijania.
