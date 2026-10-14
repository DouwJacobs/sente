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
Docker runs non-headless Chromium on a private virtual display: leave **Show
Chrome** disabled. The browser stays inside the container, including scheduled
refreshes; nothing appears on your desktop. Complete phone-based bank approvals
yourself if requested. No remote browser viewer is exposed; approvals requiring
direct browser interaction must stop with owner attention.

The image defaults `FNB_BROWSER_MODE=virtual-display`; no new Compose setting or
port is needed. Each run starts its own Xvfb display with TCP disabled and a random
0600 authorization file inside the private temporary profile. Browser shutdown
stops the display and removes its authorization. The browser retains its namespace
and seccomp sandbox, private debugging pipe, approved-origin checks and confirmed
logout requirement. Profiles and banking cookies are never retained across runs.
For troubleshooting only, `FNB_BROWSER_MODE=headless` restores headless launch.
Native development retains its existing Show Chrome behavior.

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


## Local Docker investigation on the development PC

Run `bash scripts/run-fnb-local.sh` from this checkout to build `sente:fnb-local`,
run full disposable image acceptance and start http://127.0.0.1:5180. Nothing is
pushed to GitHub or a registry. The `sente-fnb-local` Compose project uses its own
persistent database, backups and FNB key volumes, separate from development and
homelab data. The owner creates the test login and enters banking credentials in
that UI; agents do not inspect or run live banking. Keep Show Chrome off.
Repeat the command after source changes to rebuild/recreate the local service.
Do not delete its volumes when replacing the container.

Unknown phase-5 failures now include only fixed numeric diagnostics:
`login_step` 1 navigation, 2 login fields, 3 form validation, 4 username entry,
5 password entry, 6 submission, 7 account discovery, 8 normalization.
Additional count-only flags identify navigation failure, timeout, DNS, TLS,
connection failure or closed browser. Raw errors, URLs, bank HTML and credentials
are never returned. These flags narrow a failure; they do not establish invalid
credentials or prove live compatibility. The image's offline check intercepts
all requests and exercises a synthetic login/account page with network disabled.


### Owner-run Chromium navigation probe

When login_step is 1, the owner can run:

```bash
docker exec sente-fnb-local-finance-1 node /opt/sente/fnb/navigation-check.mjs
```

This opens only the public FNB home page in a new private temporary browser
profile. It never opens the database/key, enters credentials, submits a login,
reads page content or screenshots, or returns exception text/URLs. It returns
fixed numeric status/diagnostics and removes the profile. Agents do not run this
public bank navigation probe; the owner shares its count-only output.

`login_error_kind`: 1 Error, 2 TypeError, 3 ReferenceError, 4 SyntaxError,
5 ProtocolError, 6 TimeoutError, 7 TargetCloseError, 8 other.
`login_network_error` is a fixed Chromium code index: 1 aborted, 2 failed,
3 empty response, 4 HTTP/2 protocol, 5 HTTP response code, 6 invalid response,
7 excessive redirects, 8 blocked by client, 9 blocked by response, 10 network
access denied, 11 DNS name resolution, 12 DNS timeout, 13 certificate authority,
14 certificate date, 15 certificate name, 16 SSL protocol, 17 connection closed,
18 reset, 19 refused, 20 connection timeout, 21 disconnected, 22 network changed,
23 proxy connection, 24 tunnel connection. Unrecognized codes are not exported.
`login_protocol_failed` flags browser-control protocol errors.


The probe also reports `login_chromium_error`, the absolute value of a registered
Chromium network-error enum (249 recognized symbols from the public
`net/base/net_error_list.h` snapshot of 13 October 2026). For example, 15 means
socket not connected, 109 means address unreachable, 200 means certificate name
mismatch, and 202 means certificate authority invalid. Unknown strings do not
cross the diagnostic boundary. Frame detachment, page crash, browser disconnect,
navigation blocklist and lifecycle termination each have a separate fixed flag.
Only the main navigation request is observed; no subresource URLs/text are read
or returned. Nested causes are bounded to four with cycle protection.
Source: https://github.com/chromium/chromium/blob/main/net/base/net_error_list.h


The local image retains the current Alpine/Chromium package and includes a narrow
positional-write compatibility library for its reproduced child-sandbox crash.
It never disables a sandbox or certificate checking. GitHub/image acceptance
checks both synthetic login and real loopback HTTP and fails on child seccomp
crashes. Re-run the owner public navigation probe after replacing the image,
then use the local UI for owner-only live compatibility verification.


When navigation succeeds but bank_origin is zero, the public probe returns
numeric checks for HTTPS/HTTP, approved bank hostname, default port, userinfo,
browser error page and blank page. It also compares the HTTP response's approved
origin and reports a bounded redirect count. No hostname, URL, query, page text
or credential is returned. The actual login origin/form checks remain unchanged;
never broaden them solely to make a rejected navigation pass.


If a public-page redirect cannot be identified from numeric counters, the owner
may explicitly run the standalone probe with `--show-public-host`. It prints only
the final/response DNS hostnames, excluding URL paths, query strings, userinfo,
page contents and bank credentials. This option reads no database or stored key;
it is absent from the live worker/API/MCP. Hostnames can reveal network topology,
so the owner reviews them before sharing. Agents do not invoke this public-bank
probe or inspect the resulting page/session.


### Bank browser validation

The owner's container probe redirected to `validate.perfdrive.com`, documented by
Radware as a Bot Manager CAPTCHA/block endpoint. The worker recognizes only that
exact HTTPS default-port origin without userinfo, returns APPROVAL_REQUIRED with
bank_validation_required=1, and types no credentials or inspects challenge content.
Recognition does not add it to credential-approved banking origins. Headless
Docker does not provide an interactive challenge-completion path; statement
imports remain available. A future owner-assisted browser flow needs explicit
interactive setup and fresh approved bank/form checks before credential entry.
Source: https://portals.radware.com/ReleaseNotes/Alteon_Release_Notes_34_5_2_0/Alteon_Release_Notes/AlteonOS-34_5_2_0-Release_Notes.1.04.html


Before declaring bank validation unresolved, the connector/public probe waits at
most 15 seconds for ordinary navigation away from the known validation origin.
It observes only the URL, never inspects or interacts with CAPTCHA/block content,
never enters credentials on that origin, and checks the approved FNB origin/form
again afterward. Unresolved verification returns APPROVAL_REQUIRED and pauses
scheduling; a redirect to an unrelated origin remains LOGIN_LAYOUT_CHANGED.


### Local non-headless comparison

`docker/Dockerfile.fnb-headful-test` derives a disposable local image from
`sente:fnb-local` with Xvfb. `scripts/fnb-headful-test.sh` starts a local-only
virtual display and runs the selected diagnostic with `--visible`. No app
volumes, stored credentials, copied browser profiles or remote display are used.
The production image and live worker behavior are unchanged. This is an owner-run
public-navigation experiment, not a manual verification interface or a guarantee
that bank validation will clear. MCP contracts and consent remain unchanged.

Build: `docker build -f docker/Dockerfile.fnb-headful-test -t sente:fnb-headful-test .`

Offline check: `docker run --rm --init --network none --read-only --tmpfs /tmp:size=128m,mode=1777 --shm-size 256m --cap-drop ALL --security-opt no-new-privileges:true --security-opt seccomp=./docker/chromium-seccomp.json --ulimit core=0:0 sente:fnb-headful-test`

Owner public check: the same command with `--network none` removed and
`navigation-check.mjs` appended. Only numeric public navigation diagnostics are
returned. No bank login is attempted.

Individual Chromium protocol calls are bounded to 15 seconds, below the worker and Go run deadlines. Login-stage protocol/target/timeout failures are propagated rather than swallowed as approval waits; fixed numeric diagnostics remain the only reported error details. Owner testing found a retry starts a fresh worker, so the observed spinner is an in-flight request, not an image mismatch or permanently locked button. No API/MCP schema or consent change is required.
