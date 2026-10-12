# FNB runtime setup

Sente supports administrator-owned FNB discovery, balances and recent posted transaction imports. Manual and scheduled runs use visible, mapped editable accounts. Scheduling starts disabled; provider/MFA/layout failures need owner attention. Bank snapshots do not prove complete history, and capped recent-history pages may omit older activity.

The standard Docker image contains the Go app and web frontend, not the browser worker. Configure a compatible Node runtime, the dependencies in `connectors/fnb/owner`, a supported Chrome/Edge browser and the worker path before enabling live connections. `FNB_NODE_EXECUTABLE` and `FNB_RUNNER_PATH` override the local bridge defaults. WSL defaults to Windows Node at `/mnt/c/Program Files/nodejs/node.exe` and an installed Windows browser. See [connector details](../connectors/fnb/README.md).

Credentials entered in Settings → Banking are encrypted using owner-bound AES-256-GCM. Outside Docker, the generated key normally lives at `~/.config/finance-tracker/fnb.key`, outside the source tree and database backups. Existing key paths and configuration identifiers are retained for compatibility with installed systems. `FNB_KEY_FILE` may select another absolute path outside workspace/backups. Provide persistent, private writable key storage for a container runtime. Back up the key separately; losing it requires recovery or disconnecting and entering credentials again. Same-host encryption cannot exclude a privileged host operator.

Debug mode can show the browser for owner interaction. Never publish bank screenshots, credentials, page dumps, exports or runtime keys. Layout diagnostics contain fixed error codes and counts. Live compatibility remains owner-verified; synthetic fixtures do not establish it.

## Production Docker key storage

The included Dockerfile and Compose configuration provide a private, persistent
`finance_fnb_key` volume mounted at `/config/fnb` and set
`FNB_KEY_FILE=/config/fnb/fnb.key`. The image prepares the directory with mode
`0700` and ownership `10001:10001`. Docker copies those directory permissions
into a new named volume. Sente generates a 32-byte key with mode `0600` when the
first FNB credentials are saved, provided no FNB connections already exist.

Follow the [README setup and backup procedure](../README.md#fnb-encryption-key-for-production-docker).
The key survives container replacement, but deleting its volume destroys it.
Database backups never include it. This automatic setup requires an image built
from this Dockerfile or a published image containing this change; older images
do not prepare the key directory.

## Custom key storage and recovery

If you already have a persistent config volume, put `fnb.key` there and set
`FNB_KEY_FILE` to its absolute container path, for example `/config/fnb.key`.
The path must be outside the workspace (`/app`) and backup directory
(`/app/backups`). Pass this environment setting and mount into the Compose
service or your container manager; setting it only in `.env` does not pass it
into the service.

For a **new key with no stored credentials**, generate it in a directory without
an existing `fnb.key`:

```bash
(umask 077; set -C; openssl rand 32 > fnb.key)
wc -c fnb.key
```

The file must contain exactly **32 raw bytes**, not hexadecimal or Base64 text.
The command refuses to overwrite an existing file. Copy it into your persistent
config volume, or restore the exact original key if credentials already exist.
For the default volume, with the service running:

```bash
docker compose exec -T finance sh -c 'umask 077; set -C; cat > "$FNB_KEY_FILE"' < ./fnb.key
docker compose exec finance sh -c 'test -r "$FNB_KEY_FILE" && test "$(wc -c < "$FNB_KEY_FILE")" -eq 32'
```

The first command writes the file as Sente's user with private permissions and
refuses to overwrite an existing key. It requires a missing destination key and a
writable config directory. The second checks readability and length without
showing the key. If you copy the file using a container manager instead, ensure it
is owned by the runtime user (UID/GID `10001:10001` in the standard image). Keep the directory private (`0700`) and owned by the runtime user; group or other writable directories are rejected.
Sente rejects symlink/nonregular key files, files owned by another user and any group or other file permissions.

Never overwrite a working key. A new key cannot decrypt previously stored
credentials. If Sente reports "FNB encryption key unavailable. Restore the key
before saving credentials.", restore the original key. If it is unrecoverable,
disconnect all affected FNB connections before creating a replacement and saving
credentials again. Disconnecting retains accounts and transaction history.
Back up the exact key separately in private, encrypted off-host storage, and
restore it before using FNB after database recovery or moving hosts. Same-host
encryption cannot exclude a privileged host operator.

## Cancellation and profile cleanup

The worker receives credential JSON on the first stdin line and cancellation-only frames afterward. Parent-pipe loss or Linux signals request abort. Go waits up to 30 seconds for logout/browser/profile cleanup, then terminates the relevant Linux process tree or Windows worker tree through WSL. Go owns/removes each generated temporary profile and rejects cancelled or cleanup-failed results. Manual-run persistence rechecks the initiating browser session as well as current owner/account access. Synthetic process tests cover Linux and WSL/Windows Node without bank pages. Live MFA/layout/logout/overlap compatibility remains owner-run; OFX identifiers still need another overlapping export. This does not change the standard image's exclusion of the connector runtime/browser.

## Credential security and password recovery

See [the credential security audit](FNB-SECURITY-AUDIT.md) for verified protections
and limits. Administrator/offline Sente password recovery disconnects the affected
user's FNB connection and cancels its active banking run after recovery commits.
The user must enter their bank credentials again. Accounts, transaction history
and discovery preferences remain; self-service password changes keep FNB.
Keys and database snapshots still require separate private backups. Encryption
does not protect against host/runtime or client-browser compromise.
