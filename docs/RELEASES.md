# Releases, compatibility and recovery

## Branches and versions

`development` is the beta channel; `main` is the stable channel. Each push requires
full source coverage for that exact source tree, builds one linux/amd64 OCI image
with SBOM/provenance, and exercises that image using disposable synthetic volumes.
Only a successful candidate may be published. Branch pushes use the source verification
job inside the publication workflow; the separate CI entry point handles PRs/manual
checks, avoiding a duplicate full source suite for each branch push. A PR never receives registry secrets.

Successful verification records a 30-day source-tree receipt. Publication searches
up to 100 recent receipts and the latest 15 successful runs of each
verification/publication workflow within a bounded lookup and reuses
race/vet, regression, worker/demo and frontend unit/DOM checks only when the entire
Git tree matches. PR receipts record the browser specs actually covered; publication
runs the remaining specs. A subsequent stable release of the same beta source can
reuse complete source coverage. Merge/squash commit IDs may differ while their
contents match. Changed trees, missing/expired receipts, fork PRs or unavailable
Actions evidence run checks normally. Manual publication also uses this evidence;
manual CI verification remains a way to rerun all source checks.
Production images are always built and accepted with their channel-specific version
metadata. Source reuse never substitutes for image acceptance.

Documentation-only PRs keep the required verification job but skip toolchains and
source suites. Routine Dependabot updates are grouped per ecosystem/directory,
limited to one open version PR per configuration, and checked weekly on Monday
at 06:00 Africa/Johannesburg against development. Separate default-branch entries
disable routine version PRs and group security updates; security fixes are alert-driven
and are not delayed until the weekly version schedule. Dependabot reads this configuration from the default branch; changes merged only
to development take effect after promotion to main. Every dependency PR still
receives normal scope-based verification and matching-source reuse after merge.

| Source | Git tag | Immutable image version | Mutable aliases |
| --- | --- | --- | --- |
| `development` push | `v0.1.0-beta.<workflow-run-number>` | `0.1.0-beta.<workflow-run-number>` | `beta`, `development` |
| First `main` push | `v0.1.0` | `0.1.0` | `main`, `latest` |
| Later `main` push | e.g. `v0.1.1` | `0.1.1` | `main`, `latest` |
| Explicit SemVer tag | The pushed tag | Version without `v` | Stable or beta aliases according to prerelease status |

`VERSION` declares the planned stable version. If it is greater than existing stable
tags, it becomes the next release; otherwise a new revision increments the latest
stable patch. Set it to `0.2.0` for the next feature or breaking-change batch.
Development pushes preview that next stable target with a beta suffix. The workflow
run number is stable on reruns. Versions are strictly validated SemVer; release tags
exclude build metadata. Git tags are the permanent version authority once published.

Stable images also receive `sha-<full-commit>`; beta images receive
`sha-<full-commit>-beta` to distinguish metadata when the same source is released
in both channels. These and version tags are never overwritten. A rerun reuses and
re-tests an already-published version digest. A conflicting version/SHA identity or
a downgrade of a mutable alias fails. Publication is serialized across channels;
GitHub may replace an older pending run with the newest pending run. The channels
are the latest successful publication, not a promise that every intermediate push
receives an image. An interrupted publication can leave an immutable version without
all aliases; rerun to finish. Alias changes across a registry are not atomic.

The publisher creates the Git tag and GitHub release after image acceptance/upload,
attaches generated change notes, exact source archive/checksum and image digest, then
updates aliases. Token-created tags do not recursively trigger another Actions run.
Manual dispatch is allowed only on `main` or `development`; old tag publication is
refused if it would downgrade the release line or channels. Explicit tags must identify
source reachable from one of those branches. Do not force-move or delete release tags.

The owner chose public source and public images. The publisher refuses to upload
while the GitHub source repository is private, and verifies anonymous read access to the
published digest before creating its release or updating channel aliases. Repository visibility changes remain
a separate owner-reviewed action.

Configure repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` with write
access to `douwjacobs/sente`. Values are used only for registry login after candidate
verification, never passed as build arguments or recorded in artifacts. GitHub's
workflow token needs contents-write for tags/releases. Actions and base images are
pinned; Dependabot proposes updates. Review dependency/license/security changes.

## Installation and source builds

Only linux/amd64 is advertised and exercised. ARM and other architectures are not
verified. The image runs as UID/GID 10001 with a read-only root, dropped capabilities,
no-new-privileges, a writable temporary filesystem and two persistent named volumes.
It excludes the optional FNB browser worker/runtime.

Use a released version or the digest from its release notes:

```dotenv
SENTE_IMAGE=douwjacobs/sente:0.1.0
# Stronger pin, using the actual digest from the release:
# SENTE_IMAGE=douwjacobs/sente@sha256:REPLACE_WITH_RELEASE_DIGEST
```

```bash
docker compose pull finance
docker compose up -d --no-build finance
```

These are examples of the version scheme, not evidence that v0.1.0 is published.
`main`/`latest` and `beta`/`development` are mutable. Try beta in a separate Compose
project and volumes (`docker compose -p sente-beta ...`), never against a stable
installation's database. Existing `SENTE_TAG` configuration remains supported when
`SENTE_IMAGE` is absent. Explicit `SENTE_IMAGE` takes precedence.

For a local source build, export metadata from the exact checkout before building:

```bash
set -a
. ./.env
set +a
eval "$(python3 scripts/release-metadata.py --output env)"
export SENTE_VERSION SENTE_COMMIT SENTE_BUILD_TIME
docker compose build finance
docker compose up -d --no-build finance
```

The metadata generator validates all values before emitting single shell tokens.
`make build` uses it too. Untagged source builds identify their source as
`0.1.0-dev.0+g<revision>` (or the current target), with `.dirty` for tracked edits.
Build metadata belongs in application metadata; its `+` maps to `_` if a Docker
version tag is needed. Ordinary watched Go development builds still report `dev`
with Go VCS fallback. Docker without explicit arguments reports `dev`; it must not
be represented as a verified release. Browser About, MCP initialize and OCI labels
share the embedded release version/commit. Revision date is the commit date.

## Compatibility policy before 1.0

Patch releases contain compatible fixes. Features or deliberate breaking changes
advance the minor version. Every release must call out database migrations, API/MCP
changes and configuration-format changes. A 0.x version is not a blanket promise of
compatibility across minor releases. Review release notes before updating.

| Consumer | Policy and supported boundary |
| --- | --- |
| Database | Upgrade only through the registered migration runner. Never edit the schema manually. Older binaries reject newer schemas; downgrade requires a pre-upgrade snapshot. |
| Browser/API | Ship the browser and backend from the same image. Internal browser APIs are not a stable public SDK. Remote access uses HTTPS and one exact public origin. |
| MCP | Streamable HTTP, OAuth PKCE, explicit connection consent and current account grants. Initialization returns the image version. Synthetic protocol tests do not certify every external client. |
| Configuration | JSON format version 2, validated preview/import/export and explicit account mapping. Unsupported formats must fail rather than be guessed. |
| Statements | FNB ZAR CSV/OFX and bounded ZIP uploads; account identity/currency must match. Other banks/currencies/formats are not supported. |
| Live FNB | Optional separate Node/browser runtime, owner-entered credentials and live MFA/layout checks. Recent history is capped at 150 bank rows; it is not complete bank history. |
| Browser/PWA/push | Desktop/mobile Chromium covered synthetically. PWA and financial alert producers are included; real Safari/device installation and provider/OS push delivery remain owner-run checks. No offline financial editing. |

## Upgrade with a pre-upgrade backup

Do this before starting the new binary: startup migrations precede automatic backups.
Keep the same Compose project and volume names. Do not use `down -v`.

1. Record the running version/digest and the intended target digest. Review its
   migration/API/MCP/configuration notes and ensure enough disk space.
2. Stop Sente and make a snapshot with the **currently installed image**:

   ```bash
   docker compose stop finance
   docker compose run --rm -T --no-deps --pull never finance backup
   ```

   Record the returned `/app/backups/finance-....sqlite` path. Run this before
   changing `SENTE_IMAGE`; otherwise Compose may select the new binary too soon.
3. Copy the snapshot off the host. Use a stopped temporary container to access the
   volume without starting the application:

   ```bash
   container=$(docker compose run -d --no-deps --pull never --entrypoint sleep finance 300)
   docker cp "$container:/app/backups/finance-SNAPSHOT.sqlite" ./pre-upgrade.sqlite
   docker rm -f "$container"
   ```

   Replace the filename with the actual backup. Protect the copy as private data,
   verify it can be read, and transfer it to private storage on another host.
4. If live FNB is configured, separately back up `FNB_KEY_FILE` (normally
   `~/.config/finance-tracker/fnb.key`) and relevant runtime configuration. Keep
   restrictive permissions and protect it separately; database snapshots omit it.
5. Set `SENTE_IMAGE` to the new version/digest, pull and start with `--no-build`.
   Check container health, sign-in, accounts/periods/balances, a synthetic or known
   non-sensitive workflow, and MCP reconnection. Retain the pre-upgrade snapshot
   until the new installation is accepted.

## Rehearsed restore and rollback

To restore using the same/new compatible version:

```bash
docker compose stop finance
docker compose run --rm -T --no-deps --pull never finance restore /app/backups/finance-SNAPSHOT.sqlite
docker compose up -d --no-build finance
```

Restore validates integrity/schema, preserves the replaced database alongside it,
and invalidates browser and agent credentials. Sign in again and reconnect MCP
clients. Financial data, grants and saved preferences remain; revoked connections
must be recreated with explicit consent. Recover a lost FNB key separately, or
remove/re-enter its bank connection credentials as described in FNB-RUNTIME.md.

An old image does **not** undo migrations. To revert a schema upgrade, stop the new
service, select the previous pinned image and restore the **pre-upgrade** snapshot
using a binary that supports that snapshot, then start the previous image. A newer
snapshot can be rejected by an older binary. Post-snapshot financial changes will
not exist in the restored database; keep the preserved replaced file for recovery.

Rehearse in a separate Compose project with disposable/synthetic data. Never claim
production recovery based only on CI. The release image gate covers fresh onboarding,
version/MCP metadata, persistence across restart, backup/restore, browser/MCP credential
invalidation, a frozen representative schema-22 upgrade, financial/source/audit/grant/
consent preservation, integrity and once-only restart. Source race tests retain the
migration failure/rollback/retry regressions. See VERIFICATION.md for actual evidence.

## v0.1.0 release checklist

- [ ] #62: branch channels, strict SemVer/tag authority, exact-source gate, immutable
  images, consistent metadata, pinned Actions, dependency updates and publication.
- [ ] #63: same candidate image passes hardened synthetic acceptance and historical
  upgrade before publication; pinned installation and rehearsed recovery documented.
- [ ] #66: current scope/compatibility/support limits and licensing/attribution ready.
- [x] Security hardening #60/#61 merged; CI consolidation #55 merged; favicon/PWA
  #19/#24–#26 and scoped financial alerts #69 merged. The older audit's suggested
  deferral of PWA/alert producers is superseded by those accepted implementations.
- [x] Owner chose standard GPL-3.0, permitting forks/sales with GPL obligations.
- [ ] Owner review of release PR, repository credentials configured, and a successful
  GitHub publication of its exact merged SHA/image digest. No tag/image publication
  or production deployment is implied by local test success.

## Licensing and distribution

Sente's original code is GPL-3.0-only, without warranty. Keep LICENSE and NOTICE
with distributions. The fnb-api upstream reference keeps its GPL-3.0 attribution;
third-party dependency licenses remain intact. The standard image includes build
dependency license texts under `/usr/share/licenses/sente/dependencies` and OCI
SBOM/provenance. Release source archives contain the exact source/build scripts and
lockfiles; package managers retrieve the pinned third-party dependencies. Redistributors
must fulfill applicable corresponding-source and dependency-notice obligations.
GPL permits commercial use, selling and forks; no extra no-sale/no-fork terms apply.

## Frontend release checks

Settings → About shows installed build details, stable/beta update status, a newer version’s release notes and this upgrade guide. The version entry point (Settings in phone More) quietly indicates confirmed updates. A deliberate Check for updates action retries without toast noise. Local/dev, modified and unrecognized versions are unsupported. No install, download, restart or data changes occur.

The server anonymously requests only the fixed public GitHub releases endpoint; no version, financial/user/household/connection context or credentials are sent. It compares strict SemVer precedence, including numeric beta identifiers, and never recommends a downgrade or a different channel. Requests have a six-second total deadline, at most three 100-release pages and a 4 MiB limit per page. Incomplete listings, malformed metadata, rate limits and connection failures produce unavailable/unknown rather than up to date. Last successful metadata is labelled outdated after failure. Successes are cached for six hours, failures for five minutes; explicit retries share a one-minute cooldown and honor provider backoff capped at one hour. Browser responses remain authenticated and non-cacheable.

This checks application/container releases independently of Settings → PWA’s already-deployed browser asset refresh. Before manually upgrading, follow the pre-upgrade snapshot procedure above; image rollback does not reverse migrations.
