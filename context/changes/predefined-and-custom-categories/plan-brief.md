# Kategorie predefiniowane i własne — Plan Brief

> Full plan: `context/changes/predefined-and-custom-categories/plan.md`

## What & Why

Budujemy kategorie z podkategoriami (2 poziomy), żeby użytkownik miał porządek od pierwszego otwarcia i mógł dodawać własne. Bez tego nie da się wprowadzić wydatku (kolejny krok S-04 wymaga kategorii).

## Starting Point

Kategorii nie ma wcale w kodzie. Jest tylko plik `resources/categories.yaml` (wydatki w grupach, przychody płasko) i działające logowanie z izolacją konta. Baza ma tylko użytkowników i sesje.

## Desired End State

Strona `/categories` pokazuje grupy z podkategoriami. Użytkownik dodaje własną grupę i podkategorie prostym formularzem. Dane zostają po odświeżeniu. Każdy widzi tylko swoje.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
|---|---|---|
| Hierarchia | 2 poziomy (grupa → podkategoria) | Tak jest w pliku startowym i roadmapie. |
| Startowe | Kopia na konto przy pierwszym GET | Prosta prywatność, każdy ma własne wiersze. |
| Typy | kind na grupie (wydatek/przychód) | Formularz wydatku potem łatwo odfiltruje. |
| Tworzenie | Grupa + podkategoria | Pełne FR-003, użytkownik doda np. „Ogród”. |
| Edycja/usuwanie | Poza MVP | PRD odkłada do v2, chroni historię. |
| Błędy | Blokuj + polski komunikat | Czyste dane, użytkownik wie co poprawić. |
| Testy | Go: seed, duplikaty, izolacja | Łapie wyciek cudzych danych. |

## Scope

**In scope:**
- Tabela kategorii + kopiowanie z YAML
- API GET (lista-drzewo) + POST (dodaj)
- Ekran `/categories`: lista + formularz

**Out of scope:**
- Edycja, usuwanie, wyłączanie kategorii
- Ikony, kolory, przeciąganie kolejności
- Użycie w formularzu wydatku (S-04)
- Testy klikania (Playwright)

## Architecture / Approach

Baza (1 tabela z `parent_id`) → cienkie API za `RequireAccount` (ten sam wzór co auth) → ekran kopiujący `Login.vue` (ref/busy/error). Seed leniwy: pierwszy GET kopiuje YAML w transakcji z `ON CONFLICT DO NOTHING`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
|---|---|---|
| 1. Baza i startowe kopiowanie | Tabela + seed z YAML | Podwójny seed przy równoległych GET |
| 2. API kategorii | GET/POST z izolacją konta | Parent z cudzego konta / zły typ |
| 3. Ekran Kategorie | Lista + formularz na `/categories` | Zgubiony komunikat błędu |

**Prerequisites:** Działające logowanie (S-01 done), lokalna baza `DATABASE_URL`.
**Estimated effort:** ~2-3 sesje, 3 fazy.

## Open Risks & Assumptions

- Przychody w YAML są płaskie — implementer wybierze kształt (grupa bez dzieci) i opisze w kodzie.
- Biblioteka YAML: sprawdzić `go.mod`, w razie braku dodać `gopkg.in/yaml.v3`.
- Limit nazwy 80 znaków — propozycja, do potwierdzenia przy implementacji.

## Success Criteria (Summary)

- Po zalogowaniu widać startowe grupy, własna kategoria pojawia się po dodaniu i zostaje po F5.
- Cudzych kategorii nie widać, niezalogowany dostaje 401 / ląduje na loginie.
- `go test ./...` zielone, `npm --prefix web run build` przechodzi.
