# Development

Read [AGENTS.md](../AGENTS.md), [accepted plan](PLAN.md) and [architecture](ARCHITECTURE.md). UI work uses [UI.md](UI.md) and the project UI skill.

Install Go 1.27.1, Linux Node 22.21+ and Python 3. `make build` produces `bin/finance` and the frontend. `make dev` runs Vite hot reload and a watched Go backend at http://127.0.0.1:5173 using `data/dev/finance.sqlite` and `backups/dev`. A fresh dev database uses browser onboarding. Never copy production data to it.

The runner discovers Linux nvm installations and the optional `work/toolchain/go/bin/go`. You can set `GO` to an installed executable. In the owner's WSL checkout, that toolchain is `/home/douw/finance-tracker/work/toolchain/go/bin/go`. After dependency changes run `npm ci` in `web` and restart. `DEV_PORT`, `DEV_API_PORT` and `DEV_PUBLIC_URL` select exact ports/origin; occupied ports fail rather than switching.

For reproducible demo data and screenshots use [DEMO.md](DEMO.md). Demo ports 5174/8082 are separate from routine development 5173/8081 and Docker 8080.

```bash
make test
cd web
npx playwright install chromium
npm run test:e2e
```

Browser tests build an isolated synthetic service, never a production database. Schema changes need migrations; watched builds alone do not migrate existing schemas. Docker builds verify production packaging.

## Image publishing

The Docker image workflow publishes `douwjacobs/sente:latest` from pushes to `main`. Manual runs publish `development` from the selected Git ref; merge/refactoring coordination remains explicit. Configure repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` with access to `douwjacobs/sente`. No image is published by creating a demo database or opening a pull request.

The regression check `GO=/path/to/go python3 scripts/test_demo.py` validates exclusive creation, exact allocation totals, transfer balance, budget aggregates and absence of bank/MCP connections.

The binary (`finance`), Go module (`finance-tracker`), environment variables, database names, key directory and existing Compose service/volumes retain their compatibility identifiers. Product-facing names use Sente.
