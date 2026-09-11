---
bootstrapped_at: 2026-09-11T12:12:59Z
starter_id: vite-vue
starter_name: "Vite + Vue"
project_name: nest-cash-web
language_family: js
package_manager: npm
cwd_strategy: subdir-then-move
bootstrapper_confidence: verified
phase_3_status: failed
audit_command: "npm audit --json"
---

## Hand-off

```yaml
starter_id: vite-vue
package_manager: npm
project_name: nest-cash-web
hints:
  language_family: js
  team_size: solo
  deployment_target: cloudflare-pages
  ci_provider: github-actions
  ci_default_flow: auto-deploy-on-merge
  bootstrapper_confidence: verified
  path_taken: custom
  quality_override: false
  self_check_answers:
    typed: true
    from_official_starter: true
    conventions: true
    docs_current: false
    can_judge_agent: false
  has_auth: true
  has_payments: false
  has_realtime: false
  has_ai: false
  has_background_jobs: false
```

### Why this stack

NestCash potrzebuje jednego, prostego klienta webowego konsumującego istniejące API Go, bez dublowania backendu, auth ani bazy danych. Vite + Vue 3 + TypeScript pasuje do tego modelu jako lekki SPA, zachowuje jawne typy kontraktów JSON i ma konwencjonalny, zweryfikowany starter dla solo developera. Frontend będzie wdrażany niezależnie na Cloudflare Pages, a GitHub Actions uruchomi testy/build i auto-deploy po merge do `main`; backend pozostaje na Fly.io. Auth pozostaje funkcją klienta i istniejącego API, nie Supabase. Dokumentację oraz praktyki Vue należy doprecyzować w instrukcjach projektu przed implementacją.

## Pre-scaffold verification

| Signal      | Value    | Severity | Notes |
|-------------|----------|----------|-------|
| npm package | not run  | n/a      | Firmowe proxy zwróciło `403`; ponowienie przez `registry.npmjs.org` zakończyło się `ECONNRESET` przed pobraniem `create-vite`. |
| GitHub repo | not run  | n/a      | `https://vuejs.org` nie jest adresem repozytorium GitHub; brak sygnału repozytoryjnego. |

## Scaffold log

**Resolved invocation**: `NPM_CONFIG_REGISTRY=https://registry.npmjs.org npm_config_yes=true npm_config_fund=false npm_config_audit=false npm create vite@latest web -- --template vue-ts`
**Strategy**: subdir-then-move, dostosowane do monorepo przez wskazanie katalogu `web/`
**Exit code**: 1
**Stderr (last 20 lines)**:

```text
npm error code ECONNRESET
npm error errno ECONNRESET
npm error network request to https://registry.npmjs.org/create-vite failed, reason: Client network socket disconnected before secure TLS connection was established
```

**Scaffold target**: `web/` nie został utworzony; nie powstały częściowe pliki frontendu.
**.bootstrap-scaffold cleanup**: not applicable — monorepo adaptation did not create a temporary scaffold directory.

## Post-scaffold audit

**Audit not run**: scaffold halted at the CLI step; no project to audit.

## Hints recorded but not acted on

| Hint                    | Value              |
|-------------------------|--------------------|
| bootstrapper_confidence | verified           |
| quality_override        | false              |
| path_taken              | custom             |
| self_check_answers      | typed/official/conventions=true; docs_current/can_judge_agent=false |
| team_size               | solo               |
| deployment_target       | cloudflare-pages   |
| ci_provider             | github-actions     |
| ci_default_flow         | auto-deploy-on-merge |
| has_auth                | true               |
| has_payments            | false              |
| has_realtime            | false              |
| has_ai                  | false              |
| has_background_jobs     | false              |

## Next steps

Scaffold CLI exited with status 1 because the npm registry connection was reset. Re-invoke `/10x-bootstrapper @context/foundation/tech-stack-web.md` once npm registry access is available; no frontend files were created by this attempt.

Useful manual steps in the meantime:

- Keep `context/foundation/tech-stack-web.md` as the frontend hand-off.
- Preserve the existing backend verification logs; this attempt is recorded in `verification-v3.md`.
- Re-run the scaffold from the repository root with the frontend target `web/`.
