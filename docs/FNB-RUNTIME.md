# FNB runtime setup

Sente supports administrator-owned FNB discovery, balances and recent posted transaction imports. Manual and scheduled runs use visible, mapped editable accounts. Scheduling starts disabled; provider/MFA/layout failures need owner attention. Bank snapshots do not prove complete history, and capped recent-history pages may omit older activity.

The standard Docker image contains the Go app and web frontend, not the browser worker. Configure a compatible Node runtime, the dependencies in `connectors/fnb/owner`, a supported Chrome/Edge browser and the worker path before enabling live connections. `FNB_NODE_EXECUTABLE` and `FNB_RUNNER_PATH` override the local bridge defaults. WSL defaults to Windows Node at `/mnt/c/Program Files/nodejs/node.exe` and an installed Windows browser. See [connector details](../connectors/fnb/README.md).

Credentials entered in Settings → Banking are encrypted using owner-bound AES-256-GCM. The generated key normally lives at `~/.config/finance-tracker/fnb.key`, outside the source tree and database backups. Existing key paths and configuration identifiers are retained for compatibility with installed systems. `FNB_KEY_FILE` may select another absolute path outside workspace/backups. Provide persistent, private writable key storage for a container runtime. Back up the key separately; losing it requires recovery or disconnecting and entering credentials again. Same-host encryption cannot exclude a privileged host operator.

Debug mode can show the browser for owner interaction. Never publish bank screenshots, credentials, page dumps, exports or runtime keys. Layout diagnostics contain fixed error codes and counts. Live compatibility remains owner-verified; synthetic fixtures do not establish it.
