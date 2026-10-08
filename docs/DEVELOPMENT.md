# Development

Read [AGENTS.md](../AGENTS.md), [accepted plan](PLAN.md) and [architecture](ARCHITECTURE.md). UI work uses [UI.md](UI.md) and the project UI skill.

Install Go 1.27.1, Linux Node 22.21+ and Python 3. `make build` produces `bin/finance` and the frontend. `make dev` runs Vite hot reload and a watched Go backend at http://127.0.0.1:5173 using `data/dev/finance.sqlite` and `backups/dev`. A fresh dev database uses browser onboarding. Never copy production data to it.

Repository pulls need Git on the Sente host; the Docker runtime includes it. Worktrees may reuse a local `web/node_modules` symlink, which is ignored like ordinary installed dependencies.

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


## Continuous integration

`.github/workflows/ci.yml` verifies pull requests and pushes to main with Go race tests/vet, lockfile-based frontend installation, Vitest, production build and synthetic Chromium workflows. Dependencies are cached by Go modules/npm lockfile; failed browser traces, screenshots and fixture details are retained for seven days. A bounded responsive smoke set runs first with a five-minute CI limit; the remaining browser suite excludes its tagged cases to avoid duplicate execution. The verification job has read-only repository access and never publishes images or uses production data.


The browser runner builds the backend once, then starts three fresh disposable synthetic databases for each spec file, including dedicated empty-installation services. It reserves local ports, waits for health with proxy bypass and tears down every fixture. Files cannot inherit transactions, credentials, rules or settings changed by earlier specs. Pass filenames/options through `npm run test:e2e -- mcp.spec.ts`. The ordinary Playwright configuration remains available for direct single-session runs.

## Database migrations

Schema 23 adopts an explicit sequential runner. Versions 1–22 used additive schema reconciliation and sparse version markers, so the one-time baseline bridge retains their existing records and runs only the required historical data conversions. Its frozen version-22 `internal/app/schema.sql` snapshot is also used for empty installations, which skip historical classification seeds. No owner database is required for migration development.

For a new change, append a named `schemaMigration` with the next consecutive version in `internal/app/migrate.go`, increment `schemaVersion`, and put its SQL/Go callback beside its owning persistence workflow. Do not edit applied callbacks, the legacy bridge or the frozen snapshot to add future fields. A callback uses only the supplied SQL transaction; it must not open a separate transaction or query the pooled database. Use `rebuildTables` only for SQLite table rebuilds requiring temporarily disabled foreign keys; the runner pins the connection, checks all references before commit and restores enforcement on success/failure. All pending migrations and their version records commit together; a failed batch rolls back and can be retried.

Add synthetic upgrade, preservation, failure/retry and restart coverage for the affected data. `go test ./internal/app -run 'SequentialMigration|Migration'` exercises the runner and older workflow fixtures; the full backend race suite remains the integration check. Frozen historical schema fixtures from Git versions 9, 19 and 22 live in `internal/app/testdata/migrations`. The pre-23 ledger may be sparse; from 23 onward gaps, duplicate/out-of-order registrations, invalid version markers and newer databases are rejected. Existing older binaries reject a database upgraded to 23. Current-version startup does not repair manual schema damage or repeatedly backfill financial/configuration data.

## Mobile responsive verification

Run `cd web && npm run test:mobile` for the bounded `@mobile-smoke` set through the same disposable-fixture runner. It covers a core phone editor/navigation workflow, long transaction values, dashboard/budget and Settings workflows on a phone, short landscape and desktop, ordinary-user Settings visibility and a loading/error case. CI runs it before `npm run test:e2e -- --grep-invert @mobile-smoke`.

For the complete affected matrix, run `npm run test:e2e -- mobile-budget-settings.spec.ts mobile-core.spec.ts mobile-edge-layout.spec.ts`. Budget/Settings checks cover 360x800, 390x844, 430x932, 640x360, 900x720 and 1440x900 in both themes, plus 599/600/601, 759/760/761, 899/900/901 and 1024px breakpoints. The existing core checks cover the editor's 699/700/701px boundary. New fixture attachments record viewport/theme/scenario; geometry failures identify the offending control or figure. Failure screenshots/traces are diagnostics, without reference screenshot baselines or a claim of visual sign-off. Run browser suites sequentially because they share an output directory. Keyboard/safe-area/device-specific behavior needs separate evidence.

## Notification delivery checks

Detectors start with the serving maintenance lifecycle; offline commands do not send. Synthetic focused backend checks: `go test -race ./internal/app -run TestNotification -timeout=5m`. Browser checks: `python3 scripts/test-browser.py notifications.spec.ts notification-delivery.spec.ts`. The new delivery spec mocks browser permission/subscription and the test-send endpoint, so it never contacts a real push provider. Vitest tests the worker's fixed text, safe click links and absence of fetch/caching handlers. For an owner-run HTTPS check, enable one browser in Settings → Notifications, enable a Browser push type and Save changes, Send test, then remove that device; check the centre/direct link and a denied/unsupported device separately. OS permissions, service-worker background delivery, provider response/display and physical iOS/PWA behavior cannot be established by these mocks. VAPID private keys/subscription secrets live only in the protected database/private backups. General PWA install/offline/update work remains separate.
