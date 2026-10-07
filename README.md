# Sente

Sente is a self-hosted household finance app for South Africa. Track spending, build budgets, and review transactions from FNB in a private workspace on your own server. Amounts are in ZAR.

- Share household accounts and keep personal accounts private with viewer/editor access.
- Budget by period, spending group and category, with recurring or one-time limits.
- Import FNB CSV, OFX and ZIP statements; use rules to categorize transactions automatically.
- Review missing categories, split purchases, record refunds and link transfers.
- See spending trends, dated bank balances and import health in a responsive light or dark interface.
- Connect compatible MCP agents through browser-approved OAuth and explicit permissions.
- Keep daily integrity-checked SQLite backups.

## Install with Docker

Install Docker Engine and the Compose plugin, then clone this repository:

```bash
git clone https://github.com/DouwJacobs/sente.git
cd sente
cp .env.example .env
# Edit .env: PUBLIC_URL must match the exact browser-facing origin.
docker compose up -d --build
```

Open [http://localhost:8080](http://localhost:8080) and create your first administrator. Complete setup before exposing a fresh installation to other people. Create additional users in Settings, add accounts, and choose household sharing or explicit private-account grants.

The image destination is `douwjacobs/sente:latest`; `douwjacobs/sente:development` is the development channel. Until an image is published, use the source build above. Once published, use `docker compose pull && docker compose up -d` to install/update it. Set `SENTE_TAG=development` in `.env` to select that channel. Use a separate Compose project and volumes when trying development images.

## Configuration

| Setting | Purpose | Default |
| --- | --- | --- |
| `PUBLIC_URL` | Exact browser origin, without a URL subpath | `http://localhost:8080` |
| `BIND_ADDRESS` | Host interface exposed by Docker | `127.0.0.1` |
| `HOST_PORT` | Host port | `8080` |
| `TRUSTED_PROXIES` | Explicit trusted proxy IPs/CIDRs | Empty |
| `SENTE_TAG` | Docker image channel | `latest` |

Use an HTTPS reverse proxy for remote access. [Reverse proxy setup](docs/REVERSE-PROXY.md) covers Nginx/Caddy, trusted addresses and refresh timeouts. Settings → Network can save URL/proxy overrides; they take effect after restart.

The Compose service uses a non-root user, a read-only root filesystem and persistent `finance_data` / `finance_backups` volumes. Keep those volume names when upgrading an existing installation. `docker compose down -v` deletes them.

Fresh installations start without categories or rules. In Settings → Configuration, import the optional [Sente starter](https://github.com/DouwJacobs/sente-config), add public Git sources or upload JSON files. Several sources can coexist; updates are manual and replacements require a preview and confirmation. Export your categories, spending groups, merchants and rules to a portable JSON file. See [portable configuration](docs/RULESETS.md) for the format and account mapping.

## Import and review

Add an account, then upload statements in Transactions → Import activity. Choose the matching account before upload. Exact duplicates are skipped; possible duplicates, invalid rows and conflicting bank IDs need attention. Original upload bytes are discarded after staging.

Complete categorized transactions are accepted automatically. Missing categories and rule conflicts stay in Needs review. Seen/unseen is personal and separate from acceptance. Rules match descriptions; categories and spending groups stay independent. Explicit transfers are excluded from income/spending, refunds reduce expense spending, and splits preserve the original total.

Settings → Banking supports FNB discovery, balance refresh and recent posted transaction fetching. Scheduling starts off. Live compatibility depends on the bank's current layout and requires owner verification. The standard Docker image does not bundle the connector browser/runtime: see [FNB runtime setup](docs/FNB-RUNTIME.md) before enabling this feature. Statement uploads work independently.

## Backups and recovery

Automatic backups run on startup and at least every 24 hours while serving, retaining 14 snapshots. Use Settings → Back up now for an immediate snapshot, and copy backups off the host. Backups contain private financial data.

Stop the service before offline recovery:

```bash
docker compose stop finance
docker compose run --rm -T finance restore /app/backups/finance-SNAPSHOT.sqlite
docker compose up -d
```

Restore validates integrity/schema, preserves the previous database and invalidates sessions. For password recovery, stop the service and pipe a new password from standard input to `docker compose run --rm -T finance reset-password USERNAME`. Restart afterwards. Connector encryption keys require a separate private backup; database snapshots do not contain them.

## Develop with synthetic data

Use Go 1.27.1, Node 22.21+ and Python 3 in Linux/WSL:

```bash
make demo
make dev-demo
```

Open [http://127.0.0.1:5174](http://127.0.0.1:5174). Sign in as `demo` with `sente-demo-password`. The versioned [synthetic household](fixtures/demo/household.sql) provides the same invented accounts, periods, budgets, transactions, splits, refunds and transfers to every developer. It contains no owner data or bank credentials.

Demo data lives in `data/demo/finance.sqlite`, with backups in `backups/demo`. Creation refuses an existing database; normal `make dev` continues to use its separate `data/dev` database. Keep demo services local because their password is public. See [demo and screenshot workflow](docs/DEMO.md) and [development guide](docs/DEVELOPMENT.md).

## Documentation

The [documentation index](docs/README.md) separates current guides from [development history](docs/archive/README.md).

- [MCP setup and permissions](docs/MCP.md)
- [Offline rulesets](docs/RULESETS.md)
- [Architecture](docs/ARCHITECTURE.md), [accepted behavior](docs/PLAN.md), and [UI guide](docs/UI.md)
- [Maintainer handover](docs/HANDOVER.md) and [verification record](docs/VERIFICATION.md)
