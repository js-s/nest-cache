# Filtered Expense Summary — Plan Brief

> Full plan: `context/changes/filtered-expense-summary/plan.md`

## What & Why

Budujemy filtrowane podsumowanie wydatków: tabela sum per kategoria + lista do 50 operacji, z połączonymi filtrami okres (miesiąc/rok/zakres) i kategorie (multi-select). To gwiazda przewodnia (S-05) — pierwszy widok dowodzący, że dane da się sensownie podsumować.

## Starting Point

S-04 dowiezione: zapis wydatku i paginowana lista działają (`GET/POST /api/transactions`). `Summary.vue` to placeholder, brak endpointu agregującego. Indeks `(user_id, occurred_on)` już pokrywa przyszłe filtry.

## Desired End State

Na `/summary` użytkownik wybiera okres i kategorie, widzi sumy per kategoria z wierszem Razem i 50 najnowszych transakcji. Zapisany wydatek widać w podsumowaniu po filtrach; pusty okres daje zera, zły filtr daje czytelny błąd.

## Key Decisions Made

| Decision         | Choice                  | Why (1 sentence)                                          |
| ---------------- | ----------------------- | --------------------------------------------------------- |
| Granulacja okresu| Miesiąc / rok / custom  | Presety kryją 90% użytku, custom daje resztę bez mnożenia logiki |
| Kształt tabeli   | Agregat + lista 50      | Prawdziwe podsumowanie z PRD plus kontekst bez drugiej paginacji |
| Filtr kategorii  | Multi-select CSV        | Porównania A-vs-B w jednym widoku, pusto = wszystkie      |
| Kontrakt API     | Nowy `GET /api/summary` | Czysty kontrakt pod agregaty; nie rusza `List` ani nie sumuje w JS |
| Przychody        | Tylko wydatki           | Income zostaje w S-06 z ratio 80%                         |
| Brzegi           | Strict 400 + puste 200  | Spójne z transactions, łatwe testy                        |

## Scope

**In scope:**
- `GET /api/summary?from&to&category_id` z `SUM` w SQL, tylko `expense`
- `Summary.vue`: presety, multi-select, 2 tabele, stany loading/error/empty
- Testy Go (store + handler) i `npm run build`

**Out of scope:**
- Przychody, ratio 80%, wykresy, paginacja w summary, edycja transakcji/kategorii, nowa migracja

## Architecture / Approach

Rozszerzamy pakiet `transactions` (nie nowy pakiet): `Summary()` w store liczy `SUM(amount)::text GROUP BY category`, handler mapuje `ok=false → 400`, 1 linia route w `spa.go` za tym samym `RequireAccount`. Frontend liczy `from/to` z trybu i woła `getSummary()`.

## Phases at a Glance

| Phase             | What it delivers              | Key risk                        |
| ----------------- | ----------------------------- | ------------------------------- |
| 1. API podsumowania | Agregacja + route + testy Go | Rozjazd groszy (mitygacja: string, SUM w SQL) |
| 2. Ekran podsumowania | Filtry + tabele w Vue       | Dryf multi-select vs hierarchia (mitygacja: wzorzec z Operations) |

**Prerequisites:** S-04 wdrożone, `DATABASE_URL` do testów, decyzja o granulacji (zapadła tutaj).
**Estimated effort:** ~2 sesje w 2 fazach (backend z testami + frontend z weryfikacją ręczną).

## Open Risks & Assumptions

- Multi-select + agregat+lista to cięższy wariant niż minimum — jeśli faza 2 urośnie, tniemy listę do linku na `/operations`.
- Założenie: wolumen osobisty (setki wierszy), indeks wystarcza bez denormalizacji.

## Success Criteria (Summary)

- Wydatek widać w Summary po filtrze kategorii i okresu (US-01).
- Zły filtr → 400 z czytelnym komunikatem; pusty → zera.
- `go test ./...` i `npm run build` zielone; odpowiedź < 2s.
