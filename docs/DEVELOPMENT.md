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

## Application version

The backend's authenticated `GET /api/build` endpoint is the source used by the sidebar and Settings → About. It returns only version, commit, revision date and modified state, with `Cache-Control: no-store`. It does not expose Go build settings, host paths, environment, users or financial data. About is available to every signed-in user; its Report an issue link prefills only these public build fields and opens GitHub for the user to review and submit.

Release versions come from Git tags matching `v[0-9]*`. `make build` embeds `git describe`, full commit and commit date through `scripts/build-metadata.sh`; untagged repositories use `dev`. Ordinary Go builds, including `make dev`, use `dev` plus embedded Go VCS revision/date/modified metadata. The frontend package version is package metadata and is not the displayed application version.

Docker builds exclude `.git`, so pass `SENTE_VERSION`, `SENTE_COMMIT` and `SENTE_BUILD_TIME` as build arguments when building a known release. Without arguments the image reports `dev` and unavailable commit/date. The image workflow fetches tags and resolves these arguments from its checked-out revision. Build revision date is the commit date, not a claim about wall-clock compilation time. All linker metadata is restricted to safe single-token characters. Repository metadata currently declares no licence; About says so rather than assigning one.
