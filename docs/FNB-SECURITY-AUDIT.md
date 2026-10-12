# FNB credential security audit

Date: 12 October 2026. Scope: Sente's active credential storage, browser/API
boundaries, SQL access, banking worker, Docker deployment and dependencies on
`codex/fnb-docker-key-docs`. Starting revision: `7ce1cff`; changes and checks
below apply to the follow-up security patch. This is a code review, dependency
scan and synthetic adversarial verification, not a production penetration test
or proof that every possible attack is prevented.

## Outcome

Confirmed gaps were corrected in key/build exclusions, key-file access, banking
form validation and vulnerable runtime packages. The tested API paths did not
disclose credentials or permit the tested SQL injection, CSRF or unauthorized
access attempts. Automatic Docker key creation and separate key recovery remain
functional. One Windows/WSL synthetic browser-startup check remains unresolved;
the full cross-platform test run is therefore not wholly green.

The storage design cannot protect secrets from a privileged host operator or
arbitrary code execution in the trusted application/worker. It also cannot
protect credentials while they are entered in a compromised client browser.
Those are explicit trust boundaries, not claims of immunity.

## Threat model and coverage

Assets are the bank username/password, raw encryption key, encrypted connection
record, active browser session and financial data. Considered attackers include
anonymous HTTP clients, ordinary Sente users, another connection owner, malicious
websites, malformed input/report senders and local users able to influence unsafe
key storage. Host root/Docker administrators, application administrators, the
bank's own scripts, installed browser/worker binaries and approved dependencies
are trusted components with the limits below.

| Area | Reviewed control and evidence |
| --- | --- |
| Encryption | Both bank fields use AES-256-GCM with a random key/nonce and owner-bound associated data. Wrong-owner and tampered ciphertext tests reject decryption. Sente account passwords use bcrypt separately; bank credentials require reversible encryption for login. |
| Keys | Separate persistent volume, 0700 runtime-owned directory and 0600 raw 32-byte key. Missing keys do not regenerate while stored connections exist. Hardened reads/creation use a pinned directory, no-follow file opening and descriptor checks. |
| Authentication | FNB endpoints require a current enabled user, browser session and administrator role. MCP bearer tokens do not authorize browser banking endpoints. Expired sessions reject. |
| Authorization | Connection queries bind the authenticated owner ID, not a supplied user ID. Another administrator's status read cannot retrieve the owner's connection. Manual financial persistence rechecks session/role/access after provider work; scheduled runs recheck the owner/grants. |
| CSRF and transport | Write routes require a CSRF token and reject foreign Origin headers. Non-loopback HTTP bank credential saving rejects. HTTPS production access and Secure/HttpOnly/SameSite cookies rely on correct PUBLIC_URL/proxy configuration. |
| SQL and input | FNB credential/owner/version values use SQL parameters. Reviewed dynamic identifiers in labels, metadata paging, configuration and recovery come from fixed internal choices; search values are parameters. Synthetic SQL strings remain literal encrypted credential data. Unknown owner fields, malformed versions and oversized password fields reject. |
| Exposure and caching | Status uses an explicit non-secret SELECT. Credentials/key are absent from tested API outputs and audit details. Generic server failures log error types, not request bodies. Banking/API responses use no-store. Guessed key/static traversal paths do not serve the key. |
| Browser storage | FNB fields stay in component memory and clear after a successful save/cancel/disconnect. The API client sends JSON in the request body, not URLs. Production code does not put credentials in local/session storage; the service worker excludes API/authentication traffic from caching. |
| Worker | Credentials travel over stdin rather than command arguments/environment. Worker stdout is bounded and parsed with an explicit schema; provider error codes and numeric diagnostics are allowlisted before exposure. Browser uses a fresh context/profile and pipe debugging; the active worker does not disable the browser sandbox. Logout/profile/process cleanup remain mandatory. |
| External execution | Runner/runtime paths are host configuration, not browser inputs. Git configuration fetches use argument arrays, HTTPS-only transport, a restricted environment, public-address checks/DNS pinning, disabled hooks/redirects and bounded output. No browser API is an arbitrary command/file-read endpoint. |
| Docker | Non-root service, read-only root, dropped capabilities, no-new-privileges, private separate key storage, loopback host binding by default and disabled core dumps. No key is generated while building the image or baked into a layer. |
| Dependencies | Active web/worker npm audits, call-aware Go scan, local offline container package/secret scan, production build and synthetic image acceptance. |

## Findings corrected

1. **Key recovery copies could be accidentally committed or sent to builds.**
   The backup filename in README was `fnb.key.backup`, but Git ignored only
   `fnb.key` and Docker did not exclude keys. Both exclude `fnb.key*`; Docker
   also explicitly excludes nested copies. A disposable Docker context test
   proved root/nested key markers were excluded while a public marker remained.
   This reduces accidental disclosure; it does not prevent deliberate force-add
   or an operator choosing an unrelated filename.

2. **Custom key storage lacked a safe directory boundary and bounded reads.**
   The former Lstat-then-ReadFile sequence separately checked and opened the
   filename, and ReadFile could allocate for an arbitrarily large file. Key
   access now rejects directories owned by another user or writable by group/
   others, opens relative to a pinned directory descriptor without following
   final symlinks, validates the opened regular file's owner/permissions, and
   reads at most 33 bytes. FIFOs open nonblocking and reject. Creation remains
   exclusive and syncs the file/directory. Tests cover a public file, writable
   directory, symlink, 1 GiB sparse file, FIFO, relative path and backup/symlink
   path, plus a valid private key. Custom deployments may need ownership/
   permission correction; no weaker permission fallback is introduced.

3. **Login form validation did not reject GET submission or button overrides.**
   The worker now checks the actual document and form immediately before
   username/password entry and submission, requires HTTPS bank URLs on the
   default TLS port without URL userinfo, requires POST, and validates submit
   button `formaction`/`formmethod`. Redirects after username entry stop before
   password typing. Credentials are cleared from the worker's references on
   failure cleanup too. Synthetic tests reject foreign/suffix-lookalike origins,
   HTTP, a different port, userinfo, GET and foreign/GET submit overrides.
   This does not establish that FNB's real page is uncompromised or that its
   current live layout passes the stricter checks.

4. **The Go 1.27.1 runtime had known vulnerable called symbols.**
   The baseline official Go scanner found ten called-symbol advisories:
   GO-2026-6603, 6605, 6607, 6608, 6609, 6610, 6611, 6612, 6613 and 6617.
   These cover HTTP/TLS/multipart issues, including denial of service and
   connection desynchronization. Some protocol paths depend on deployment;
   scanner call traces alone do not prove each issue's exploitability.
   Docker is pinned to the verified Go 1.27.2 image digest and go.mod requests
   the patched toolchain. The repeat call-aware scan reports zero called or
   imported-package vulnerabilities.

5. **The container included a vulnerable zlib package.**
   Local Trivy found CVE-2026-85091 in zlib 1.3.2-r0, a heap buffer overflow
   advisory. The runtime build now applies Alpine package security updates;
   the rebuilt image has 1.3.2-r1 and the final scan reports zero Alpine package
   vulnerabilities. The scan's result is inventory evidence, not proof this
   application could reach the vulnerable zlib operation.

6. **Password recovery could inherit another user's banking connection.**
   An application administrator could reset a connection owner's Sente password
   and then use that owner's saved FNB connection. The owner approved disconnecting
   FNB for administrator/offline recovery. That deletion now shares the existing
   atomic password/session/agent/audit transaction; after commit, the active run
   is cancelled without waiting for the connector lock. Actual provider work also
   rechecks the exact encrypted credential record at every FNB persistence boundary.
   Failed/audit-rolled-back recovery retains the connection and does not cancel
   work. Self-service password changes retain their authenticated session and FNB.

7. **Core-dump exposure was not explicitly disabled.**
   Compose now sets soft/hard core limits to zero. An actual stopped disposable
   Compose container's HostConfig and the running synthetic image check verify
   the limit. Reverse-proxy guidance now explicitly forbids Banking request-body
   capture in logs/tracing/analytics. Memory cannot be guaranteed absent from
   host-controlled dumps, swap or a compromised browser.

## Verification evidence

- 45 top-level tests passed in the broad backend security/FNB race-detector run,
  including authorization, revocation, wrong-owner/tamper rejection, SQL input,
  API redaction, static paths, scheduling and financial persistence. No race
  warning was emitted. The run exited unsuccessfully because one Windows/WSL
  blank-Chrome startup subtest failed; its parent accounts for the remaining
  top-level test. Elapsed time: 259.727 seconds.
- Four final focused adversarial key/API/HTTP tests passed under the race detector
  (13.934 seconds), including the added backup-symlink and remote-HTTP cases.
- The approved recovery follow-up passed 14 top-level backend tests under the
  race detector (118.404 seconds): user-specific admin/offline disconnection,
  self-service retention, preserved history/other connections, audit rollback,
  active scheduled-run cancellation and rejection of removed/replaced credential
  records, together with existing password/session/scheduling checks.
- Six final API/MCP/FNB persistence integration checks passed under the race
  detector (69.030 seconds).
- All 48 active FNB worker tests passed; no live banking was used.
- All 20 frontend unit tests and the production build passed. Four isolated
  user-security browser workflows passed at 360px and 1440px, including the FNB
  recovery notice, reset/deletion validation and self-service session retention.
  No computer-use or manual visual verification was performed.
- Go vet passed. Web and active worker npm audit each reported zero advisories.
- Official govulncheck v1.9.0 with Go 1.27.2 reported zero vulnerabilities in
  called code or imported packages. It reported one unused module advisory,
  GO-2026-5932, for x/crypto's unmaintained OpenPGP package. Sente uses other
  packages in that module and does not import OpenPGP.
- Final local Trivy 0.75.0 scan: zero Alpine package vulnerabilities, zero detected
  embedded secrets, and only the same unused OpenPGP module advisory. No
  suppression was added to hide that warning. The scan ran on an exported local
  image with networking/telemetry disabled; only the public database download
  had network access, with no Sente image mounted.
- Hardened production Docker build and complete synthetic image acceptance
  passed: onboarding, metadata/MCP, persistence, backup/restore, frozen schema-22
  migration, automatic key creation, permissions, container replacement, missing
  key refusal, separate key restoration and disabled core dumps.
- Git key-backup exclusion, real disposable Docker-context exclusion and actual
  Compose core-limit inspection passed. Diff checks passed.
- Local images use dev metadata and the pre-audit revision for smoke checks;
  they are unpublished verification artifacts, not production releases.

## Remaining risks and limits

**Runtime or host compromise can expose banking secrets.** Encryption separates
a stolen database/snapshot from its key. It cannot prevent an attacker controlling
the application/worker, the Docker daemon, host root, the bank origin or the
user's browser from reading credentials or using an authenticated session. Go/
JavaScript strings and browser memory cannot be reliably wiped by clearing
references. Keep the host and actual browser patched, preserve the sandbox,
restrict administrative access, use HTTPS and bank approvals, and keep key
backups independently private. Statement uploads avoid storing bank credentials
if this trust model is unacceptable.

**Administrator recovery no longer inherits an FNB connection.** The owner
approved disconnection on administrator/offline password recovery. The security
patch deletes the recovered user's encrypted connection atomically with password/
session/agent/audit changes, then cancels that owner's active banking run. An
old credential-record fingerprint cannot authorize later financial persistence.
Accounts/history/discovery preferences and other users' connections remain.
Self-service password changes retain FNB. Administrator privileges still permit
account administration and Sente-user impersonation after recovery; they are
not a general financial-privacy boundary, and host administrators remain trusted.

**Windows/WSL browser cleanup is not certified by this run.** The synthetic
`wsl-windows-chromium-hung` check failed with CONNECTOR_START_FAILED before its
blank-page worker became ready, both in the broad run and isolated recheck.
Other cancellation variants passed. No live profile/credential was inspected
and no failure assertion was weakened. Reproduce/fix that environment-specific
startup failure before claiming Windows/WSL browser acceptance.

**This is not an exhaustive production-host assessment.** The production proxy,
TLS configuration, firewall, secrets/backup destinations, real client extensions,
real browser version, live FNB forms/MFA/logout and external attack traffic were
not tested. No independent penetration test, complete route fuzzing or unknown
vulnerability assurance is claimed. Legacy vendored upstream and disconnected
mock-service experiments are outside the active deployment certification; their
historical dependency warnings remain documented.

**Availability needs deployment controls.** Rate limiting/request limits and
timeouts were reviewed/tested where noted; high-volume denial of service still
needs proxy/firewall resource limits. Disconnecting removes active credential
records, but historical encrypted database snapshots can retain them; manage
snapshot retention and protect the matching key.

## Operational gate

Keep the PR in draft until its required CI/image checks pass. Deploy only an image
containing the patched runtime/key changes; do not generate a new key for existing
ciphertext. Enable live banking only after the owner verifies the intended
Node/browser runtime and HTTPS deployment. Do not put real passwords, keys, bank
pages or profiles into test fixtures, CI logs, screenshots or audit reports.

MCP impact: no tools, input schemas, output allowlists, consent/permissions,
proposals/audits or shared financial services expand. Credentials remain browser
write-only and worker-internal. No database migration is introduced.

## Reference guidance

- [OWASP cryptographic storage](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html):
  threat modeling, authenticated encryption, random keys and key/data separation.
- [OWASP SQL injection prevention](https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html):
  parameters and allowlisted dynamic identifiers.
- [Go vulnerability database example](https://pkg.go.dev/vuln/GO-2026-6617)
  and [unused OpenPGP advisory](https://pkg.go.dev/vuln/GO-2026-5932).
- [Trivy image/archive scanning](https://trivy.dev/docs/latest/target/container_image/)
  and [telemetry controls](https://trivy.dev/docs/latest/advanced/telemetry/).

Automatic approval review rejected Docker Scout because it could send local image
metadata to an external service without destination-specific authorization.
The local offline scan above completed the package/secret check instead.
