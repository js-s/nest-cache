---
starter_id: vite-vue
package_manager: npm
project_name: nest-cash-web
hints:
  language_family: js
  team_size: solo
  deployment_target: fly-unified-image
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
---

## Why this stack

NestCash potrzebuje jednego, prostego klienta webowego konsumującego istniejące API Go, bez dublowania backendu, auth ani bazy danych. Vite + Vue 3 + TypeScript pasuje do tego modelu jako lekki SPA, zachowuje jawne typy kontraktów JSON i ma konwencjonalny, zweryfikowany starter dla solo developera. Ustalony układ to jedno monorepo: backend pozostaje w root, frontend powstaje w `web/`, a wspólny `context/` pozostaje na poziomie root; nie scaffoldować frontendu bezpośrednio do rootu. Z innego środowiska, po `npm ping` i `npm view create-vite version`, uruchomić `npm create vite@latest web -- --template vue-ts --no-interactive`, następnie `cd web && npm install`, `npm run dev` lub `npm run build`; lokalnie zachować firmowe registry. Produkcyjny workflow GitHub Actions używa publicznego npm wyłącznie dla tych publicznych zależności: w ephemeral checkout przepisuje hosty tarballi w kopii lockfile, uruchamia Node 22 + `npm ci` + `npm run build`, a committed `package-lock.json` i tokeny pozostają bez zmian. Frontend budować jako `web/dist` w workflow wdrożeniowym Fly.io; backend z rootu pozostaje na Fly.io. Auth pozostaje funkcją klienta i istniejącego API, nie Supabase; adres API trzymać w `VITE_API_BASE_URL`.
