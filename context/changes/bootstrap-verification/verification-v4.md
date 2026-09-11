---
bootstrapped_at: 2026-09-11T12:42:06Z
starter_id: vite-vue
starter_name: "Vite + Vue"
project_name: nest-cash-web
language_family: js
package_manager: npm
cwd_strategy: native-cwd
bootstrapper_confidence: verified
phase_3_status: ok
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

NestCash potrzebuje jednego, prostego klienta webowego konsumującego istniejące API Go, bez dublowania backendu, auth ani bazy danych. Vite + Vue 3 + TypeScript pasuje do tego modelu jako lekki SPA, zachowuje jawne typy kontraktów JSON i ma konwencjonalny, zweryfikowany starter dla solo developera. Ustalony układ to jedno monorepo: backend pozostaje w root, frontend powstaje w `web/`, a wspólny `context/` pozostaje na poziomie root; nie scaffoldować frontendu bezpośrednio do rootu. Z innego środowiska, po `npm ping` i `npm view create-vite version`, uruchomić `npm create vite@latest web -- --template vue-ts --no-interactive`, następnie `cd web && npm install`, `npm run dev` lub `npm run build`; zachować firmowe registry zamiast przełączać się na publiczne npm. Frontend wdrażać z `web/dist` na Cloudflare Pages przez osobny workflow GitHub Actions, a backend z rootu na Fly.io. Auth pozostaje funkcją klienta i istniejącego API, nie Supabase; adres API trzymać w `VITE_API_BASE_URL`.

## Pre-scaffold verification

| Signal       | Value                                | Severity | Notes                                                        |
| ------------ | ------------------------------------ | -------- | ------------------------------------------------------------ |
| npm package  | create-vite v9.2.0 published 2026-09-10 | fresh    | resolved from cmd_template (`npm create vite`)               |
| GitHub repo  | not run                              | n/a      | card.docs_url is vuejs.org (not a GitHub repo); no signal    |

## Scaffold log

**Resolved invocation**: `npm create vite@latest web -- --template vue-ts --no-interactive`
**Strategy**: native-cwd (hand-off monorepo note: scaffold into `web/`, leave root backend untouched)
**Exit code**: 0
**Pre-flight files-to-touch**: `web/` (did not exist before this run)
**Files written by CLI**: 19 non-`node_modules` files (`.vscode/`, `public/`, `src/`, `.gitignore`, `README.md`, `index.html`, `package.json`, `tsconfig*.json`, `vite.config.ts`), plus `package-lock.json` and `node_modules/` from the follow-up `npm install`
**Pre-existing files preserved**: none in `web/`; root `context/` and Go backend files untouched
**Post-scaffold `npm install`**: 48 packages added, exit 0

## Post-scaffold audit

**Tool**: npm audit --json
**Summary**: 0 CRITICAL, 0 HIGH, 0 MODERATE, 0 LOW
**Direct vs transitive**: 0 findings; prod 25, dev 49, optional 27, total 73 dependencies

#### CRITICAL findings

None.

#### HIGH findings

None.

#### MODERATE findings

None.

#### LOW / INFO findings

None.

## Hints recorded but not acted on

| Hint                    | Value                |
| ----------------------- | -------------------- |
| bootstrapper_confidence | verified             |
| quality_override        | false                |
| path_taken              | custom               |
| self_check_answers      | typed: true; from_official_starter: true; conventions: true; docs_current: false; can_judge_agent: false |
| team_size               | solo                 |
| deployment_target       | cloudflare-pages     |
| ci_provider             | github-actions       |
| ci_default_flow         | auto-deploy-on-merge |
| has_auth                | true                 |
| has_payments            | false                |
| has_realtime            | false                |
| has_ai                  | false                |
| has_background_jobs     | false                |

## Next steps

Next: a future skill will set up agent context (CLAUDE.md, AGENTS.md). For now, your project is scaffolded and verified — happy hacking.

Useful manual steps in the meantime:
- `git init` (if you have not already) to start your own repo history.
- Review any `.scaffold` siblings the conflict policy created and decide which version of each file to keep.
- Address audit findings per your project's risk tolerance — the full breakdown is in this log.
- Wire `VITE_API_BASE_URL` to the Go API base URL before running `npm run dev`.
- Add the separate GitHub Actions workflow deploying `web/dist` to Cloudflare Pages (v1 bootstrapper does not generate CI files).
