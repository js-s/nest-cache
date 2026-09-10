# Plan wdrożenia NestCash na Fly.io + Neon

**Platforma:** Fly.io (app) + Neon.tech (Postgres)
**Region:** `fra` (Frankfurt)
**Szacowany koszt:** ~$0–2/mo (app autostop) + $0 (Neon free tier)

---

## Faza 0: Wymagania wstępne

### Krok 0.1: Instalacja narzędzi
```bash
# flyctl
curl -L https://fly.io/install.sh | sh

# Weryfikacja
flyctl version
```

### Krok 0.2: Konta i uwierzytelnianie
- [x] Konto Fly.io z dodaną kartą kredytową (wymagane, nawet na free usage)
- [x] Konto Neon.tech (free tier, bez karty)
- [x] Uwierzytelnianie flyctl: `flyctl auth login`

### Krok 0.3: Neon — utworzenie bazy danych
- [x] Utworzenie projektu Neon: region `eu-central-1` (Frankfurt, blisko Fly `fra`)
- [x] Skopiowanie connection stringa (pooled URL z PgBouncer) - wartość zapisana wyłącznie jako sekret Fly `DATABASE_URL`; nie przechowujemy jej w repozytorium
- [ ] Zanotowanie: host, port, user, password, database name
Otrzymałem tylko connection string - ponoć to wystarczy. Nie mam więcej danych.


#### Prompt od Neon tech: set up Neon with your coding agent
Set up this Neon project in the current working directory.

1. `npm i -g neon@latest && neon login`
2. `neon skills -y`
3. `neon mcp -y`
4. `neon link --project-id green-butterfly-54283179 --branch production -y`
5. `neon config init`
6. Update `neon.ts`:

```ts
import { defineConfig } from "@neon/config/v1";

export default defineConfig({
  auth: true,
});
```

7. `neon deploy`


**Weryfikacja Fazy 0:**
- [ ] `flyctl auth whoami` zwraca email
- [ ] Neon dashboard pokazuje aktywny projekt w eu-central-1
- [ ] Connection string działa: `psql "<NEON_POOLED_URL>"` łączy się

---

## Faza 1: Dockerfile + fly.toml

### Krok 1.1: Dockerfile (multi-stage)
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /nest-cash ./cmd/nest-cash

# Run stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /nest-cash /usr/local/bin/nest-cash
EXPOSE 8080
CMD ["nest-cash"]
```

### Krok 1.2: fly.toml
```toml
app = "nest-cash"
primary_region = "fra"

[build]

[env]
  PORT = "8080"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true
  min_machines_running = 0

[[vm]]
  size = "shared-cpu-1x"
  memory = "256mb"
```

### Krok 1.3: .dockerignore
```
.git
.env
*.test
context/
.ai/
.opencode/
```

**Weryfikacja Fazy 1:**
- [ ] `docker build -t nest-cash .` buduje obraz bez błędów
- [ ] `docker run -p 8080:8080 nest-cash` startuje lokalnie (bez DB — oczekiwany błąd połączenia)
- [ ] `fly.toml` jest w repozytorium (wymagane dla CI/CD)

---

## Faza 2: Pierwsze wdrożenie na Fly.io

### Krok 2.1: Utworzenie aplikacji
```bash
flyctl launch --name nest-cash --region fra --no-deploy
```
> **UWAGA:** Odrzucić propozycję utworzenia Fly Postgres / MPG! Wybierz "no database".
> Znany bug #4871: przypadkowe tworzenie klastrów MPG = $38+/mo.

### Krok 2.2: Ustawienie secrets
```bash
flyctl secrets set DATABASE_URL="<NEON_POOLED_CONNECTION_STRING>"
flyctl secrets set SESSION_SECRET="<wygenerowany-losowy-string-64-znaki>"
```

### Krok 2.3: Deploy
```bash
flyctl deploy
```

### Krok 2.4: Weryfikacja
```bash
flyctl status                    # app running, 1 machine
flyctl logs                      # brak errorów
flyctl open                      # otwiera w przeglądarce
```

**Weryfikacja Fazy 2:**
- [ ] `flyctl status` pokazuje maszynę w stanie `started`
- [ ] `flyctl logs` nie zawiera błędów połączenia z bazą
- [ ] `curl https://nest-cash.fly.dev/healthz` zwraca 200
- [ ] Rejestracja i logowanie działają end-to-end
- [ ] Dashboard Fly.io: billing pokazuje $0.00 (lub grosze)

---

## Faza 3: CI/CD — GitHub Actions

### Krok 3.1: Deploy token
```bash
flyctl tokens create deploy -x 999999h
# Skopiować CAŁY output, włącznie z "FlyV1 " na początku
```

### Krok 3.2: GitHub secret
- Settings → Secrets and variables → Actions → New repository secret
- Name: `FLY_API_TOKEN`
- Value: skopiowany token z kroku 3.1

### Krok 3.3: Workflow `.github/workflows/fly.yml`
```yaml
name: Fly Deploy
on:
  push:
    branches:
      - main
jobs:
  deploy:
    name: Deploy app
    runs-on: ubuntu-latest
    concurrency: deploy-group
    steps:
      - uses: actions/checkout@v4
      - uses: superfly/flyctl-actions/setup-flyctl@master
      - run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

**Weryfikacja Fazy 3:**
- [ ] Push do `main` triggeruje GitHub Action
- [ ] Action kończy się statusem "success"
- [ ] `flyctl releases` pokazuje nową wersję
- [ ] Aplikacja działa po deploy z CI

---

## Faza 4: Monitoring i bezpieczeństwo

### Krok 4.1: Health check endpoint
Aplikacja powinna eksponować `/healthz`:
- Sprawdza połączenie z Neon (`SELECT 1`)
- Zwraca 200 OK lub 503

### Krok 4.2: Logowanie i alerty
```bash
# Live tail
flyctl logs

# JSON format (do parsowania)
flyctl logs --json
```

### Krok 4.3: Billing guard
- [ ] Ustaw billing alert w Fly.io dashboard ($5/mo threshold)
- [ ] Neon dashboard: monitoruj storage usage (limit 0.5GB free)
- [ ] Comiesięczny review: `flyctl bills`

**Weryfikacja Fazy 4:**
- [ ] `/healthz` zwraca 200 z informacją o DB
- [ ] `flyctl logs` pokazuje requesty w formacie JSON
- [ ] Billing alert skonfigurowany na $5

---

## Przypadki brzegowe i znane problemy

### 1. Cold start (autostop)
**Problem:** Po kilku minutach bezruchu maszyna się zatrzymuje. Pierwszy request: ~200-500ms opóźnienia.
**Mitigacja:** Akceptowalne dla personal MVP. Jeśli będzie problem: `min_machines_running = 1` (~$2/mo).

### 2. Suspended machines bug (#4707)
**Problem:** Jeśli `auto_stop_machines = "suspend"`, maszyny nie startują po deploy — nowe env vars nie są aplikowane.
**Mitigacja:** Używamy `"stop"` zamiast `"suspend"` w fly.toml. Po deploy: `flyctl machine start <id>` jeśli maszyna nie wystartuje automatycznie.

### 3. Accidental MPG clusters (#4871)
**Problem:** `flyctl launch` może zaproponować utworzenie MPG klastra ($38+/mo).
**Mitigacja:** Zawsze wybieraj "no database" przy `fly launch`. Używaj Neon. Nigdy nie uruchamiaj `fly mpg create` ani `fly postgres create`.

### 4. Neon free tier limits
**Problem:** 0.5GB storage, 190 compute hours/mo, projekt pauzuje po 7 dniach bez aktywności.
**Mitigacja:** Dla personal MVP z kilkoma transakcjami dziennie — wystarczające. Storage monitoring w Neon dashboard. Upgrade do $19/mo jeśli potrzeba.

### 5. Rollback
```bash
flyctl releases                  # lista wersji
flyctl releases rollback         # cofnij do poprzedniej
```
**UWAGA:** Migracje bazy NIE cofają się automatycznie. Schema changes wymagają ręcznego revertu.

### 6. IPv4
**Problem:** Fly.io domyślnie daje shared IPv4 + dedicated IPv6. Niektóre zewnętrzne API wymagają dedicated IPv4.
**Mitigacja:** Dla MVP nie potrzebujemy. Jeśli będzie konieczne: `$2/mo` za dedicated IPv4.

### 7. Neon connection pooling
**Problem:** Go app może trzymać zbyt wiele połączeń do Neon.
**Mitigacja:** Używaj **pooled** connection string (PgBouncer) z Neon dashboard. Ustaw `pool_max_conns=5` w connection stringu.

### 8. `fly certs --json` bug (#4724)
**Problem:** Flaga `--json` jest ignorowana dla `fly certs`.
**Mitigacja:** Nie używaj `--json` z `fly certs`. Parsuj tekst output.

---

## Zewnętrzne integracje

| Komponent | Usługa | Połączenie |
|-----------|--------|------------|
| App hosting | Fly.io | `fra` region, shared-cpu-1x, 256MB |
| Database | Neon.tech | eu-central-1, pooled URL via PgBouncer |
| CI/CD | GitHub Actions | `superfly/flyctl-actions`, deploy on push to `main` |
| TLS | Fly.io | Automatyczny Let's Encrypt via `force_https = true` |
| DNS | Fly.io | `nest-cash.fly.dev` (darmowa subdomena) |
| Secrets | Fly.io vault | `flyctl secrets set`, env vars at runtime |

---

## Kosztorys MVP

| Komponent | Koszt/mo |
|-----------|----------|
| Fly.io app (autostop, ~0 traffic) | ~$0.00–0.50 |
| Fly.io app (light usage, ~100 req/day) | ~$1–2 |
| Neon Free Tier | $0.00 |
| GitHub Actions | $0.00 (2000 min/mo free) |
| **Total** | **~$0–2/mo** |

---

## Checklisty podsumowujące

### Pre-deploy
- [ ] flyctl zainstalowany i zautentykowany
- [ ] Konto Fly.io z kartą kredytową
- [ ] Projekt Neon utworzony w eu-central-1
- [ ] Dockerfile i fly.toml w repozytorium
- [ ] `docker build` przechodzi lokalnie
- [ ] `.dockerignore` wyklucza niepotrzebne pliki

### Post-deploy
- [ ] `flyctl status` = maszyna running
- [ ] `curl /healthz` = 200
- [ ] Rejestracja + logowanie = działa
- [ ] CRUD transakcji = działa
- [ ] Podsumowanie z filtrami = działa
- [ ] Billing < $5/mo
- [ ] CI/CD deploy z `main` = automatyczny
- [ ] Billing alert ustawiony

### Operacje ręczne (human-only)
- Usuwanie aplikacji Fly.io
- Usuwanie wolumenów
- Zmiana planu billingowego
- Rotacja SESSION_SECRET
- Migracje bazy danych (schema changes)
