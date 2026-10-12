# FNB runtime setup

Sente supports administrator-owned FNB discovery, balances and recent posted transaction imports. Manual and scheduled runs use visible, mapped editable accounts. Scheduling starts disabled; provider/MFA/layout failures need owner attention. Bank snapshots do not prove complete history, and capped recent-history pages may omit older activity.

The standard Docker image includes the Go app, web frontend, Node, the active
FNB worker and Chromium. One container serves the UI and performs refreshes;
there is no separate connector service, API endpoint or browser port to configure.
The image sets `FNB_NODE_EXECUTABLE=/usr/local/bin/node` and
`FNB_RUNNER_PATH=/opt/sente/fnb/refresh.mjs`. GitHub tests and publishes this same
image, including an offline sandboxed-browser startup check.

Use the current Compose file and `docker/chromium-seccomp.json` beside it.
Compose provides a non-root, read-only runtime, init process, private shared memory,
zero core dumps and a default-deny seccomp profile permitting Chromium's namespace
sandbox. Keep these settings; do not use privileged mode, host IPC or
`--no-sandbox`. See [sandbox configuration](../docker/README.md).
Hosts must permit unprivileged user namespaces; incompatible host policies stop
browser startup rather than silently disabling the sandbox.

For an existing deployment, preserve the Compose project name and named volumes,
add the key mount and sandbox/init/shared-memory settings from the new Compose
file, and pull a release containing the runtime. Download the profile from the
same release tag as the image. The profile is a Docker-host file, not a secret:

```bash
mkdir -p docker
curl --fail --location https://raw.githubusercontent.com/DouwJacobs/sente/<release-tag>/docker/chromium-seccomp.json -o docker/chromium-seccomp.json
```

Replace `<release-tag>` with the published Git tag (for example `v<image-version>`),
not an older release that predates this runtime. Restart with
`docker compose pull finance` and `docker compose up -d finance`.
The previous `0.1.3-beta.34` image does not contain the browser runtime.
No manual key generation is needed for a first connection.

Check startup without banking credentials or contacting FNB:

```bash
docker compose run --rm --no-deps --entrypoint node finance /opt/sente/fnb/runtime-check.mjs
```

The check only opens a synthetic page and Chromium's internal sandbox page;
release CI additionally disables container networking. It prints fixed PASS text,
never bank data. FNB refresh remains owner initiated and owner verified.
Docker runs headless: leave **Show Chrome** disabled. Complete phone-based bank
approvals yourself if requested. No graphical desktop or remote browser viewer is
exposed; approvals requiring direct browser interaction are not supported by this
headless deployment and must stop with owner attention.

Outside Docker, install Node 22.12+, the pinned `connectors/fnb/owner` dependencies
and Chrome/Edge. Executable/worker overrides remain available; WSL defaults to
Windows Node at `/mnt/c/Program Files/nodejs/node.exe`. See
[connector details](../connectors/fnb/README.md).

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

The worker receives credential JSON on the first stdin line and cancellation-only frames afterward. Parent-pipe loss or Linux signals request abort. Go waits up to 30 seconds for logout/browser/profile cleanup, then terminates the relevant Linux process tree or Windows worker tree through WSL. Go owns/removes each generated temporary profile and rejects cancelled or cleanup-failed results. Manual-run persistence rechecks the initiating browser session as well as current owner/account access. Synthetic process tests cover Linux and WSL/Windows Node without bank pages. Live MFA/layout/logout/overlap compatibility remains owner-run; OFX identifiers still need another overlapping export. The packaged container uses the same lifecycle and temporary-profile cleanup.

## Credential security and password recovery

Administrator/offline Sente password recovery disconnects the affected
user's FNB connection and cancels its active banking run after recovery commits.
The user must enter their bank credentials again. Accounts, transaction history
and discovery preferences remain; self-service password changes keep FNB.
Keys and database snapshots still require separate private backups. Encryption
does not protect against host/runtime or client-browser compromise.
