# Setup and everyday use

Install Sente on your own server, bring in your statements and keep your household workspace backed up. For a product tour, return to the [README](../README.md).

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

Use an HTTPS reverse proxy for remote access. [Reverse proxy setup](REVERSE-PROXY.md) covers Nginx/Caddy, trusted addresses and refresh timeouts. Settings → Network can save URL/proxy overrides; they take effect after restart.

The Compose service uses a non-root user, a read-only root filesystem and persistent `finance_data` / `finance_backups` volumes. Keep those volume names when upgrading an existing installation. `docker compose down -v` deletes them.

Fresh installations start without categories or rules. In Settings → Configuration, import the optional [Sente starter](https://github.com/DouwJacobs/sente-config), add public Git sources or upload JSON files. Several sources can coexist; updates are manual and replacements require a preview and confirmation. Export your categories, spending groups, merchants and rules to a portable JSON file. See [portable configuration](RULESETS.md) for the format and account mapping.

## Import and review

Add an account, then upload statements in Transactions → Import activity. Choose the matching account before upload. Exact duplicates are skipped; possible duplicates, invalid rows and conflicting bank IDs need attention. Original upload bytes are discarded after staging.

Complete categorized transactions are accepted automatically. Missing categories and rule conflicts stay in Needs review. Seen/unseen is personal and separate from acceptance. Rules match descriptions; categories and spending groups stay independent. Explicit transfers are excluded from income/spending, refunds reduce expense spending, and splits preserve the original total.

Settings → Banking supports FNB discovery, balance refresh and recent posted transaction fetching. Scheduling starts off. Live compatibility depends on the bank's current layout and requires owner verification. The standard Docker image does not bundle the connector browser/runtime: see [FNB runtime setup](FNB-RUNTIME.md) before enabling this feature. Statement uploads work independently.

## Backups and recovery

Automatic backups run on startup and at least every 24 hours while serving, retaining 14 snapshots. Use Settings → Back up now for an immediate snapshot, and copy backups off the host. Backups contain private financial data.

Stop the service before offline recovery:

```bash
docker compose stop finance
docker compose run --rm -T finance restore /app/backups/finance-SNAPSHOT.sqlite
docker compose up -d
```

Restore validates integrity/schema, preserves the previous database and invalidates sessions. For password recovery, stop the service and pipe a new password from standard input to `docker compose run --rm -T finance reset-password USERNAME`. Restart afterwards. Connector encryption keys require a separate private backup; database snapshots do not contain them.

