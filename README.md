# Household Finance Tracker

A self-hosted household finance tracker for FNB South Africa. React/TypeScript + Vite frontend, one Go HTTP service, and SQLite. ZAR only.

## Included

- Household/private accounts, administrator-created users, viewer/editor grants.
- Dashboard with pending-inclusive spending, category limits, and dated bank-reported balances.
- FNB CSV and OFX 1.02 imports, including multi-file ZIP uploads; preview, row errors, source provenance, exact and possible duplicates.
- Mandatory individual/bulk review, category splits, refunds, transfer linking, reusable account-scoped description rules.
- Category groups, limits without rollover, editable periods with gaps/overlaps, automatic/manual/outside assignments, reassignment previews.
- Light/dark responsive interface and daily integrity-checked backups retaining 14 snapshots.

## Local setup in WSL

Use Node 22.21+ and Go 1.27.1. For this workspace a project-local Go toolchain is available at `work/toolchain/go/bin/go`; Node is installed under `~/.nvm`.

```bash
cd ~/finance-tracker
export PATH="$PWD/work/toolchain/go/bin:$HOME/.nvm/versions/node/v22.21.1/bin:$PATH"
make build
./bin/finance serve
```

Open http://localhost:8080. A fresh installation shows first-time setup: choose your administrator username, password, and confirmation in the browser. Setup signs you in and permanently closes once an administrator exists, including a disabled administrator. Create your wife's user in Settings, then create your FNB accounts and categories. Household members automatically have editor access to household accounts. Private accounts need explicit grants.

## Hot reload development in WSL

```bash
cd ~/finance-tracker
make dev
# Or: python3 scripts/dev.py
```

Open http://127.0.0.1:5173. React/CSS edits update through Vite hot reload; Go source, embedded SQL, or module-file changes automatically rebuild and restart the backend on port 8081. Compilation errors leave the last working backend running until fixed. Ctrl+C stops both servers.

The runner finds the existing project-local Go toolchain and Linux Node installation, including nvm installations. It runs npm ci if frontend dependencies are missing. After changing package.json/package-lock.json, run npm ci in web and restart development.

Development defaults to a separate persistent database at data/dev/finance.sqlite and backups/dev. Create a development administrator through browser onboarding. The Docker database on port 8080 is separate. The development browser uses 127.0.0.1 while production uses localhost, so their session cookies do not replace one another. DATABASE_PATH and BACKUP_DIR can override the dev paths deliberately.

Optional settings: DEV_PORT=5173, DEV_API_PORT=8081, and DEV_PUBLIC_URL=http://127.0.0.1:5173. The browser URL must match DEV_PUBLIC_URL exactly for authenticated writes. Ports are fixed; an occupied port produces an error rather than switching silently.

No Docker rebuild is needed for this workflow. Docker remains the production build/deployment path. Schema edits reload embedded SQL but still need a migration when changing existing tables.


## Docker deployment

```bash
cp .env.example .env
# Set PUBLIC_URL to the exact browser-facing origin, e.g. https://finance.example.com.
docker compose build
docker compose up -d
```

Administrators can save PUBLIC_URL/trusted-proxy overrides in Settings → Network, effective after restart. Use your existing HTTPS reverse proxy to forward to port 8080; see [Reverse proxy setup](docs/REVERSE-PROXY.md) for Nginx/Caddy examples, trusted client IPs and refresh timeouts. Cookies are Secure when PUBLIC_URL begins with https. The default port binding is loopback. Set BIND_ADDRESS deliberately if your proxy is on another machine; use HTTPS for remote access. Named volumes contain the database and backups; do not run `docker compose down -v` when preserving data.

Open the application to create the first administrator in the browser. No setup script is required. The initial setup is accessible only until an administrator exists; finish it before exposing a fresh installation to remote users. The `create-admin` CLI and `scripts/setup-local.sh` remain available as optional offline alternatives. Password recovery still uses the offline `reset-password` command.

The container runs as a non-root user with a read-only root filesystem. Its only persistent writable locations are data and backups. Health is available at /api/health.

## Import behavior

Choose an account before uploading; embedded account identity must match. Limits: 20 supported files, 25 MiB compressed/uploaded data, 100 MiB ZIP expansion, 100,000 rows per file. Archives are read in memory and never extracted into the filesystem.

Original upload bytes are discarded after preview. Normalized rows, hashes, row errors, duplicate decisions, and transaction provenance remain in SQLite. Exact file duplicates cannot be committed twice. FITID is scoped to the account; conflicting source data is blocked unless skipped. Date/amount/description candidates require a keep/skip decision, so legitimate identical purchases can be retained.

FNB ID stability between separate overlapping OFX downloads still needs a second sample. The supplied CSV/OFX pair confirms equivalent transaction data, not ID stability. CSV fallback matching remains available.

Imports always require review. Rules are explicit case-insensitive description substrings per account, ordered by descending priority then creation ID. Saving a rule does not change existing transactions.

## Budget semantics

Period start and end dates are inclusive and independent. Latest start wins overlaps, with newest period ID breaking ties. Unmatched household transactions appear in the assignment queue. Explicit manual assignments remain after date edits and are flagged when outside dates; explicit outside assignments do not appear in the queue.

Private accounts never contribute to shared category limits or totals. When viewing a private account with a selected period, its transactions are filtered by those dates only. Transfers exclude principal, while refunds in expense categories reduce spending. Splits never add cash movement.

Bank balances are imported snapshots, not live balances or proof of complete history. Category limits reset per period; creating a period copies limits from the latest-starting existing period.

## Backup and recovery

The service checks every hour and makes a snapshot when the last successful backup is at least 24 hours old, including on startup. Settings shows successful backups and automatic backup failure status. Backup directory is configurable with BACKUP_DIR; DATABASE_PATH and STATIC_DIR are also configurable.

While serving, use Settings → Back up now. Offline commands require the service stopped and an exclusive database lock:

```bash
./bin/finance backup
# Copy snapshots to a separate disk/server for protection against host failure.
./bin/finance restore /absolute/path/to/finance-SNAPSHOT.sqlite
# Password recovery revokes all sessions for that user.
read -rsp 'New password: ' password; printf '\n'
printf '%s\n' "$password" | ./bin/finance reset-password your-username
unset password
```

Docker recovery uses the same commands through `docker compose run --rm -T finance …` after `docker compose stop finance`. A snapshot inside the named volume can be referenced as `/app/backups/finance-SNAPSHOT.sqlite`. Restart with `docker compose up -d`.

Restore checks integrity and schema compatibility, preserves any previous database as `*.before-restore-TIMESTAMP`, and deletes restored sessions. Backups contain personal financial data; protect their storage and copy them off-host. Staged imports and provenance are included.

## Verification

```bash
make test
cd web
npx playwright install chromium # once, before the first browser-test run
npm run test:e2e
```

Browser tests start an isolated synthetic-data service; they never import personal data into a production database. Backend tests cover import idempotency/concurrency, permissions/revocation, review/splits, refunds/transfers, period previews/assignments, authentication, and backup restoration.

## Maintained development guidance

Read AGENTS.md, docs/PLAN.md, docs/ARCHITECTURE.md, and docs/UI.md. UI changes must use the project skill at .agents/skills/finance-tracker-ui/SKILL.md and update the shared tokens/components and guide together.

## Future direction

Provider-agnostic agent access for transaction updates, financial overviews, and spending-based budget setup is recorded in [docs/FUTURE.md](docs/FUTURE.md). MCP is the proposed finance-tool interface; ACP is a possible later agent-session integration. This is planned future work, not a feature enabled in this release.

## Handover additions

Administrators can change the workspace display name in Settings. Signed-in navigation and the browser title use that value; the sign-in screen remains generic. In Budgets → Edit limits, Add expense category creates a flat expense category with a zero draft target. Existing draft amounts stay intact, and Save limits is still required to change the budget.

FNB live syncing is not yet available. The pinned source review, exact-money mock adapter and separately packaged mock service are documented in [connectors/fnb/README.md](connectors/fnb/README.md). Deployment is intended for a separate host outside agent permissions. No live credentials are accepted or stored by this prototype.

For the owner-operated live account-discovery test, see [connectors/fnb/owner/README.md](connectors/fnb/owner/README.md). Run the Windows workspace's Test-FnbAccounts.ps1 yourself, sign in directly in the temporary browser, open Accounts and confirm in the terminal. Import the resulting Downloads JSON through Settings → Accounts → Import discovered accounts. This test does not import transactions/balances or store bank credentials. Live layout compatibility remains unverified until the owner runs it.

### FNB accounts and balance refresh (local WSL development)

Settings → Banking → FNB connection lets an administrator enter their FNB username/password, then Refresh now to log in automatically and refresh names, bank numbers and ledger balances. Complete any phone approval; enable/save debug mode if browser interaction is required. Successful discovery creates private accounts and updates existing accounts only with current editor access; sharing is preserved. No transactions are fetched by this connection yet. Manual metadata-file import remains available.

Choose Off, every 6/12 hours, daily or weekly and Save refresh schedule. Scheduling starts off and runs only while the server is running. Approval/layout/provider failures pause automatic attempts; use Refresh now or replace credentials to recover. Do not assume successful manual discovery proves automatic login or balance compatibility: these need owner-run live verification. Unsupported account-number entries are counted/skipped; dated existing balances are preserved when the bank does not supply a current balance.

Hide account persists across refreshes, removes it from normal views/totals and suppresses connector balance updates. Existing financial history remains stored. Use Show hidden discovered accounts → Show account to restore it. Disconnect removes encrypted credentials and scheduling while keeping history/hide preferences. Hidden discoveries will also be excluded by any future transaction connector.

Credential encryption uses an owner-bound AES-256-GCM blob in SQLite and a 0600 generated key at the OS config directory's finance-tracker/fnb.key (normally ~/.config/finance-tracker/fnb.key in WSL), outside source and database backups. FNB_KEY_FILE may point to another absolute location outside workspace/backups. Back up that key separately and privately; database snapshots alone cannot restore a connection. Losing the key requires restoring it or disconnecting all affected connections and entering credentials again. Never put the key in Git, ordinary backups or chat. The owner accepts that same-host encryption cannot technically exclude an agent with unrestricted access.

The local bridge defaults to Windows Node at /mnt/c/Program Files/nodejs/node.exe, modern puppeteer-core in connectors/fnb/owner and installed Windows Chrome/Edge. Optional FNB_NODE_EXECUTABLE and FNB_RUNNER_PATH configure a Linux/other-host runtime without storing bank credentials in environment variables. The current Docker image does not include this worker/browser or writable external key storage and requires deployment configuration before using the connection.

FNB refresh is now headless by default. To watch it, enable Debug mode — show Chrome during refresh and select Save browser mode before Refresh now. The saved setting also applies to scheduled refreshes. For BALANCE_LAYOUT_CHANGED, retry and open Layout diagnostics (counts only); share only BALANCE_COUNTS and the fixed error code. Bank details, page dumps and screenshots are not included. No need to replace credentials for a balance-layout failure.

Settings now groups General, Banking, Accounts, Users & access, Backups and Security into subtabs. FNB credentials/schedules/debug/discovery visibility are in Banking; manual account setup/sharing and discovered-file imports are in Accounts. The main Accounts page shows balances and links to Manage accounts. Admin-only sections remain restricted.


### Set up transaction rules

Open **Categories**, which starts on **Rules**. Choose **Add rule**, enter a description match, choose the category and optional spending group, then save. A new rule applies to all current enabled accounts you can edit; **More options** restricts it to one account or money in/out. Matching account rules appear as one row with Edit, Pause/Resume and Delete. Categories can be created while adding a rule.

Use **Check an example** when needed. Conflicting outputs remain unclassified for review. Rules never designate transfers or approve transactions. Existing ledger data and budgets are preserved.

Vault22 was a development reference only. There is no Vault22 upload/mapping setup screen. Live FNB will supply new transactions in the subsequent sync milestone; the connection currently refreshes accounts and balances.


Starter categories and rules are included automatically, covering everyday expenses, salary, interest and common FNB charges. Built-in rules apply to all enabled accounts, including new accounts; edit, pause or delete them as needed. Custom account rules take precedence. Defaults are added once, preserving existing categories, budgets and transaction history.

While editing a transaction, search the category picker, type a new name and press Enter (or Create) to create and select it. New categories default to Expense; choose Income when appropriate. Select the optional checkbox to remember the category and spending group for similar descriptions in that account. Saving the transaction and rule is atomic; reusing the same account/pattern/direction updates that rule.
