# Maintainer handover

Updated 9 October 2026. Read [AGENTS.md](../AGENTS.md), [PLAN.md](PLAN.md) and
[ARCHITECTURE.md](ARCHITECTURE.md) before work; UI changes also use UI.md and the
project UI skill. [Documentation index](README.md) maps the current guides.

## Checkout and runtime

Edit `/home/douw/finance-tracker` directly through Ubuntu WSL. The Windows Finance
Tracker folder links to it; do not create another application there. Preserve unrelated
work. Go is `work/toolchain/go/bin/go`; Node is
`/home/douw/.nvm/versions/node/v22.21.1/bin`. Routine dev uses `make dev` and
http://127.0.0.1:5173 with its separate persistent development database. No production
financial data is used for verification. Owner policy excludes computer-use UI checks
unless explicitly requested; automated synthetic workflows remain allowed.

## UI experience work — 9 October 2026

`codex/polished-ui` in `/home/douw/finance-tracker-ui` starts from `origin/development` at `a61c784`. It adds contextual first-use, filtered, completed and failure states with real setup/import/recovery actions; consistent responsive icon/copy/action hierarchy; stable pending buttons; and reduced-motion-aware shared transitions. Dialogs fade without moving their geometry. Existing financial semantics, request locks, drafts and permissions remain.

The reusable [Sente experience skill](../.agents/skills/sente-experience/SKILL.md) records exact copy, visual values and action-routing decisions and is installed in the owner's Windows Codex skills folder. docs/UI.md and the existing UI skill route future work to it. Production build, 15 unit tests and 52 browser cases passed across targeted sequential synthetic batches; see [VERIFICATION.md](VERIFICATION.md#intentional-ui-states-and-interaction-polish--9-october-2026). Phone-light and desktop-dark empty-state screenshots were inspected. No production deployment was performed.

## Current release work — #62, #63, #66

Started from f651682 (PR #74), on `codex/release-channels`, then integrated main 3bffea7 (budget hover fix). The other agent's
budget editor and PR #73 scoped alerts are merged. Owner chose public source/images and standard GPL-3.0 after
being informed that forks and sales are permitted; preserve upstream notices. Owner
confirmed development pushes should create beta tags/images and main pushes stable
versions/main/latest. [Release guide](RELEASES.md) owns version allocation, image
verification, publication, compatibility and upgrade/recovery procedures.

PR #76 merged as acfbc37 after green PR checks. The owner approved public source/images: the repository is public, Docker Hub secret names are configured, and development exists from that approved source. Both initial release runs stopped before image publishing because the complete browser gate found an obsolete settings-action locator in the mobile audit test. `codex/release-browser-gate` retains its stacking checks against the current Branding buttons; four targeted audit cases passed. A broader attempt also exposed an unsynchronized review-row click; its loading wait is corrected and all six core-improvements cases passed. All 53 local browser specs (207 cases) passed in the corrected batches; see VERIFICATION.md. GitHub checks and final-revision release publication remain the gate. No new SemVer release/tag/image or production deployment is claimed. Publication credentials never enter builds/tests. See VERIFICATION.md for coverage and host-specific test limits.

Frontend application update awareness is tracked separately in #88; existing PWA refresh notification #26 covers a different behavior.

MCP impact: existing initialization already uses the shared build version; release
metadata is verified through that contract. No tools, input schemas, output allowlists,
capabilities/consent, proposal previews/audits or financial services change. About's
GPL/source links remain browser-only. No database migration is introduced.

The merged budget-hover fix preserves neutral expanded headers, pointer exit and touch behavior; its verification remains in VERIFICATION.md.

## CI runtime reduction — 9 October 2026

CI changes add successful exact-Git-tree receipts to source verification.
Publication reuses source gates and runs only browser specs absent from matching
PR evidence; identical beta/stable source shares complete coverage. Image acceptance
still runs for each channel/version. Missing/expired evidence, changed trees, forks
and API errors retain verification. Documentation-only PRs retain a successful
required job without test/toolchain setup. Dependabot routine updates are grouped,
weekly Monday 06:00 Africa/Johannesburg, with one open version PR per configuration
against development; separate default-branch entries group alert-driven security
fixes without opening routine stable-branch version PRs.
No application/MCP tools, schemas, output allowlists, permissions, consent,
proposal previews/audits, shared financial services or migrations change.
See VERIFICATION.md for actual checks; artifact-backed reuse remains to be
verified in GitHub. Dependabot reads configuration from the default branch, so
its new grouping activates after development is promoted to main.

## Remaining operational limits

- Live FNB layout/MFA compatibility and overlapping real OFX identifier stability
  require owner-run checks; preserve capped-history/coverage warnings.
- External MCP clients, physical PWA/Safari installations and provider/OS push delivery
  are not certified by synthetic CI. Standard images omit the FNB browser runtime.
- Current release support is linux/amd64. An image rollback does not reverse migrations;
  take an explicit pre-upgrade snapshot and separate connector-key/off-host backup.
- Review pinned Action/base-image dependency updates and source/license obligations.

[VERIFICATION.md](VERIFICATION.md) records actual coverage. Dated implementation and
publication notes through the budget editor/product-tour work are retained in
[the prior handover](archive/handover-2026-10-08-before-releases.md); earlier histories
are indexed in [the archive](archive/README.md). Historical notes are not current scope.


## Approved UI audit refinements — 9 October 2026

Continuing on `codex/polished-ui`, implemented all eight audit decisions: compact desktop ledger rows; progressive advanced filters; classification-first editor with draft summary and no duplicate heading/blank errors; intrinsic desktop save; single empty-state action ownership; consistent compact headers and useful-only period navigation; grouped Account tools; and static reduced-motion disclosure angles. The repository and installed Sente experience skills record reproducible values, copy and behavior. See VERIFICATION.md for final checks.

The owner preview remains at http://127.0.0.1:5173 from `/home/douw/finance-tracker-ui`, using the previous development SQLite database at `/home/douw/finance-tracker/data/dev/finance.sqlite` and API port 8081. The preview supervisor log/PID and pre-preview snapshot live in the worktree's ignored `work/dev` directory. Synthetic verification uses disposable databases separately.


## Transaction filter layout and motion follow-up — 9 October 2026

The existing UI worktree now groups filter controls into aligned sections, moves range guidance outside field tracks, uses one searchable/paged Merchant and Tag selector, and puts active restrictions at the footer. Clear filters is a labelled reset-arrow icon in the card header (search-bar equivalent only while collapsed); Import transactions is inside Transaction actions. Shared open/close transitions cover filters, native disclosures, action menus, budget expanders, dialogs and phone More, preserving immediate guards/focus and reduced motion. The installed/repository Sente experience skill records these decisions. See VERIFICATION.md for final checks; owner preview still uses the existing development database.


## Notification inbox UI follow-up — 9 October 2026

The UI branch now tightens the inbox toolbar and message cards: compact count summary, refresh icon, bulk/preferences menu, clear unread/read hierarchy, quieter metadata/timestamps, one-row source/read/menu actions and icon pagination. Explicit read/dismiss, authorized source navigation, push-only messages, empty/error recovery and preference drafts retain their callbacks. Inbox CSS is scoped to avoid changing push-device cards. Reproducible decisions are in the updated Sente experience skill; verification is recorded in VERIFICATION.md. Preview continues on the existing development database.

## Budget period actions — 9 October 2026

Removed the redundant Edit budgets button from period cards, moved View spending into the existing period menu, and retained selected-period groups by default. Collapsed periods use Show spending groups in that menu. Headers keep their menu top-right on phones; Selected period icon/text is hidden only on phones. Dashboard Edit budgets navigation remains. This presentation-only change does not alter financial services, permissions, schemas or MCP contracts. The Sente skill records responsive/action decisions.

## Transaction classification layout — 9 October 2026

Add split now belongs to the Classification card header on desktop and phone, keeping Spending group and Category adjacent. Phone category controls span the available width; single allocations no longer reserve an empty remove track, while split removal sits beside its amount. The reusable Sente skill records exact rules. This remains browser-only presentation: no financial service, schema, MCP contract or permission changes.

## Configuration/PWA paired cards — 9 October 2026

Scoped the single-column 640px General preferences rule to its own class. Configuration and PWA retain the shared two-column equal-height desktop grid and full-width natural-height phone stack. No financial service, database, MCP contract, permission or draft behavior changes. The reusable Sente skill records the scope and responsive layout.

## Mobile search icon visibility — 9 October 2026

Removed the obsolete mobile rule hiding search-button spans, which also hid the shared button-content icon wrapper. Search retains theme-aware text color, quiet chrome treatment, 44px target and existing callback. Reusable Sente guidance records wrapper visibility and contrast checks. Browser-only CSS correction; no financial services, schemas, MCP contracts, permissions or consent changes.


## MCP personal budget alerts — issue #75, 9 October 2026

`codex/mcp-budget-alerts` starts from development `22099eb` in the existing WSL
checkout. Added default-off `read_budget_alerts` and `manage_budget_alerts` consent,
explicit single-scope `get_budget_alert_preferences`, and exact 1–100-scope
`update_budget_alerts` proposals. Group 0 explicitly means No spending group;
current personal versions, enabled/custom 1–100% thresholds and reset-to-defaults
are validated. Shared preference saves and silent current-condition baselines run
inside the existing atomic proposal transaction with exact before/after audit and
once-only replay. Global notification preferences and scope carryover/suppression
remain authoritative. No schema migration is introduced.

OAuth/Settings share the new read/change controls, dependency/scope validation,
explicit confirmation and optional automatic approval. Finance/legacy presets do
not gain access. Household membership and unrestricted connection account scope
are required because settings apply to shared budget combinations. Owner identity
comes exclusively from the authenticated connection. Inbox content, global channel
preferences, push registration/secrets and diagnostics remain browser-only.

Verification: 15 frontend unit tests, production build, Go vet and all 16 MCP/inbox/
push browser cases passed (360px light and 1440px dark for MCP). Four new backend
integration tests passed under the race detector, covering consent/defaults,
ownership, validation/bounds, revoked consent/membership/scope/automatic approval,
stale/renamed scopes, audit rollback, replay, silent baselines, mute/reset, global
precedence and duplicate suppression. The initial baseline `make test` hit Go's
default ten-minute package timeout without assertion failures. The complete
`go test -race ./... -timeout=30m -json` run exercised 252 top-level app tests
and their subtests plus the domain packages (1,485.64s for app). It exited 1
solely because the tool-catalogue assertion still expected 22 tools. Updated that
expectation to 23; `go test -race ./internal/app -run TestMCPPrivacyAndTransport
-count=1 -timeout=5m` then passed. The final four alert tests also passed in a
separate race run (53.21s), including added percentage/batch bounds and revoked
approval checks. No test failures remain after those targeted rechecks; the whole
suite was not repeated after the assertion-only correction. The optional public
starter pull and supplied FNB export checks skipped as configured. Only synthetic
databases were used. No computer-use visual inspection, external MCP
client certification or production deployment is claimed.


## MCP alert permission check and owner preview — 9 October 2026

Rechecked separate read/change grants, default-off legacy/finance behavior,
read dependency, all-account/household scope, automatic approval and consent
rechecks at prepare/approval/apply/replay. OAuth and Settings share the updated
permission fields; the single-scope read tool and alert proposal path use their
respective grants. Existing connection permissions were not changed for the owner.

The owner development preview at http://127.0.0.1:5173 now runs `make dev` from
`/home/douw/finance-tracker` on `codex/mcp-budget-alerts`, replacing the older
`finance-tracker-ui` preview supervisor. It retains the same development database
`/home/douw/finance-tracker/data/dev/finance.sqlite` and backups directory. API
health returned 200; Vite serves both new consent fields; the running backend
embeds application revision `ee4ed8e`. Log/PID records are ignored files
`work/dev/mcp-budget-alerts-preview.log` and `work/dev/mcp-budget-alerts-preview.pid`.
No production database or connection consent was modified.

All four `TestMCPBudgetAlert` integration tests passed again (16.92s) during this
permission/runtime recheck. No application code change was needed.

## Release browser prerequisites and budget handoff - 9 October 2026

Development run 37904586295 failed after PR #90 because the prerequisite audit
expected a disabled Add rule button and obsolete account guidance on the empty
Rules screen. Tests now wait for the loaded empty state and exercise Open Accounts
and Create category directly. Checking the unreached suite also found dashboard
tests using inline View spending, a phone-visible Selected period badge and an
empty-dashboard Edit budgets action; they now follow the period action menu, phone
badge visibility and Set up a budget. Saved group totals wait for the refreshed
authoritative value without changing the expected amount.
The pending-button audit now checks stable size/name and a centered spinner rather
than looking for a direct text node and the former inline spinner gap.
Related source paths select the three affected specs for PR verification, without
expanding the default suite for unrelated changes. This catches these regressions
before the complete release gate. Product behavior and MCP contracts are unchanged.
The fix is isolated in /home/douw/finance-tracker-ci-fix on
codex/release-audit-prerequisites; ongoing MCP edits in the primary checkout remain
untouched. VERIFICATION.md records actual coverage.

## Application release awareness — issue #88, 9 October 2026

`codex/application-updates` starts from development `dcfe4b0` in the primary WSL
source. About now shows release status, exact newer stable/beta version, release
notes, manual checks and pre-upgrade backup/migration guidance. Desktop version
and phone More → Settings show a quiet accessible confirmed-update indicator.
Existing PWA asset refresh remains separate; no container upgrade/restart occurs.

The credential-free server checker uses only fixed public GitHub release metadata,
strict SemVer precedence, serialized process caching, bounded response/pagination/
deadlines and capped provider backoff. Dev/local/modified/unknown builds remain
unsupported. Failed checks never claim current; retained release details are
explicitly outdated. No financial/user/household/connection context is sent upstream.
MCP remains unchanged: no new tools, schemas, output fields, permissions, consent,
proposals/audits or financial services. No schema migration is introduced.

Focused backend race tests, Go vet, 17 frontend unit tests, production build and
seven synthetic About browser cases passed; see VERIFICATION.md. No production
financial data, manual visual inspection, live update-provider certification or
production deployment is claimed. The normal watched development preview will
honestly report that release checks are unsupported for its dev build.

## Dashboard spending Sankey — issue #94, 10 October 2026

Implemented on `codex/dashboard-sankey` in the primary WSL source. The browser dashboard
shows the complete authorized Total spending → Spending groups → Categories breakdown,
independent of budget paging. Split allocations/refunds/uncategorised expenses share the
existing dashboard scope; transfers are excluded. Negative and zero category amounts stay
in the accessible table, with positive-flow/net-refund reconciliation stated explicitly.

Owner feedback clarified that ribbon sizing was correct and labels were misaligned.
Labels now sit immediately beside their actual node centre. The chart fits its card
without internal scrollbars; natural height retains all categories. On narrow screens the
total is in the header and the drawing uses group/category columns; the full breakdown
table uses responsive rows. Uses existing light/dark tokens and request loading/error
recovery. Verification is recorded in VERIFICATION.md; no production data/deployment or
manual visual/physical-device inspection is claimed.

MCP impact: `spending_breakdown` is browser-only and excluded by the existing output
allowlist, with a summary non-disclosure regression assertion. Existing tools, schemas,
permissions, consent, proposals/audits and financial write services remain unchanged.
No migration. No chart dependency added.

## Dashboard action menu and dedicated Sankey page — 10 October 2026

Owner requested a quieter dashboard: removed the embedded chart and moved Edit budgets
and Sankey graph into the Spending by group card's shared hamburger menu. SpendingFlowPage
owns its cancellable aggregate request and loading/error/retry; App shares period/account
selection across dashboard and chart. Back to dashboard preserves scope and focuses main
content. Parent dashboard navigation remains selected on the chart page.

Spending breakdown now has the shared 12px left inset/36px right reserve. Spending menu
items are quiet and borderless at rest/hover, preserving focus and interaction feedback.
UI guidance and the repository Sente experience skill record the accepted behavior.
Tests use the real menu actions before existing budget workflows. This is browser
presentation/navigation only: no server financial arithmetic, aggregate shape, MCP
tools/schema/allowlist/consent, proposal/audit, permissions or migration changes.

## Browser CI timeout — 11 October 2026

Development → main PR #93 run 38063133034 completed all 40 selected browser
specs without an assertion failure, but elapsed 605 seconds exceeded its
10-minute step deadline. The pull-request browser deadline now matches the
complete suite's 20 minutes; the job remains bounded to 30 minutes and every
existing test/scope/evidence gate remains enabled.

This is CI configuration only. No application behavior, financial calculation,
MCP tool/schema/allowlist/permission/consent/proposal/audit/service or migration
changes. CI regression scripts and diff checks are the local verification;
GitHub checks must pass before merging.

## Dependabot frontend compatibility - 11 October 2026

The grouped frontend update keeps React plugin 6 with its Vite 8 peer and adds
explicit vite/client and Node types for TypeScript 7. About uses the existing
Lucide CodeXml icon for the source-repository link because Lucide 1 no longer
exports the Github brand icon. Link text, URL, accessibility and layout remain.

In the isolated /home/douw/finance-tracker-deps WSL worktree, npm ci reported
zero vulnerabilities, all 20 frontend unit tests and the production build passed.
All seven synthetic About/update-awareness cases passed across phone/desktop
and light/dark themes. The configured PR browser suite and GitHub checks are
the remaining merge gates. No manual visual inspection is claimed.

This dependency/type/icon change remains browser-only: no MCP tools, input
schemas, output allowlists, permissions, consent, proposals/audits, shared
financial services or database migrations change.

The upgraded browser run exposed a configuration geometry test race: its three
bounds were measured across the asynchronous starter-source load. The test now
waits for the starter heading and loading-status removal before measuring,
retaining the original equal-spacing assertion. Both desktop and 360px import/
coexistence/replacement/export cases passed; the unchanged dependency baseline
also passed. No product layout change or relaxed geometry assertion is involved.

## Dependabot Docker image gate - 11 October 2026

The grouped Docker update targets development with pinned Node 26.10.0 and
Alpine 3.24. Existing source CI does not build Dockerfile updates. A separate
credential-free PR image workflow now builds the exact checked-out revision on
linux/amd64 and runs the existing hardened image acceptance with disposable
synthetic volumes, including onboarding, metadata, persistence, backup/restore
and the frozen schema-22 upgrade. It never publishes or deploys an image.

Local Docker Desktop startup failed, so no local image pass is claimed.
GitHub image and source checks are required for this merge decision.
No application financial behavior, database migration or MCP tools, schemas,
output allowlists, permissions, consent, proposals/audits or services change.

## Automatic production FNB Docker key setup - 12 October 2026

On codex/fnb-docker-key-docs, Dockerfile prepares private runtime-owned /config/fnb
and Compose mounts finance_fnb_key with FNB_KEY_FILE=/config/fnb/fnb.key. Fresh
installations generate their key automatically on first credential save; existing
connections still require their exact original key. Keep the key volume across
upgrades and back it up separately from database snapshots. Older published
images do not prepare this directory; use an image containing this change.

README describes automatic setup and private key backup; FNB-RUNTIME covers custom
config volumes, manual raw-key generation and non-root restoration without
overwriting an existing key. SETUP and ARCHITECTURE match. Browser runtime setup
remains a separate prerequisite.

Production build, complete synthetic image acceptance, new FNB key
creation/permissions/container replacement/missing-key refusal checks, final
separate key backup/restoration recheck, three focused backend tests and Compose
validation passed. See VERIFICATION.md for exact coverage. No production
deployment or real bank data/credentials were used. Deployment storage only:
no MCP contract, permission, consent, proposal/audit, shared financial service
or migration changes.

## FNB security audit and approved recovery isolation - 12 October 2026

The same PR branch now includes key/build exclusion and filesystem hardening,
POST/submitter/origin checks before bank credential entry, patched Go/Alpine
runtime dependencies and disabled core dumps. Owner approved FNB disconnection
for administrator/offline password recovery. Delete only that user's connection
atomically with recovery; after commit cancel its active provider. Provider
financial persistence rechecks the exact credential record. Self-service retains
FNB; accounts/history/preferences and other users' connections remain.

VERIFICATION.md records final checks and remaining trust/verification limits. Recovery race tests, API/MCP/
persistence checks, worker/frontend tests, phone/desktop recovery workflows,
production build and image/key/recovery acceptance passed. Patched Go/npm scans
have no called/imported or active npm advisories. Offline image scan has no Alpine
vulnerabilities or detected secrets; only unused x/crypto OpenPGP inventory warning.

Keep the PR draft: required CI/image checks and owner deployment verification
remain. The WSL/Windows blank-Chrome startup cancellation fixture failed twice;
do not claim Windows browser certification. No live banking, real credentials,
production data/deployment or manual UI inspection. Host/runtime/client compromise
can still expose plaintext during login; a zero-compromise guarantee is impossible.
No MCP contract/consent/output/permission expansion or database migration.


## Single-container FNB runtime — 12 October 2026

`codex/fnb-container-runtime` packages Node, the active FNB worker and Chromium
in the standard Sente image. Worker and offline startup check share
`browser-runtime.mjs`; packaged executable/path defaults use the existing local
private pipe contract. No second container, remote connector API or browser port.
Compose adds init, 256 MiB private shared memory and the bundled default-deny
Chromium seccomp profile (Playwright origin plus namespace chroot). Retain
non-root, dropped capabilities, no-new-privileges, read-only root and core=0.
Custom deployments need the profile file beside Compose and a new published
image; `0.1.3-beta.34` does not contain this runtime. Preserve the existing data,
backup and FNB key volumes and original keys. Docker has no graphical desktop;
keep Show Chrome off. Owner-only live/MFA verification is outstanding.

48 worker tests, 28 release/CI regression tests and final full synthetic image
acceptance passed locally, including real offline sandbox startup and configured
worker invocation. npm worker audit has zero advisories; offline Trivy scan has
zero new runtime-package vulnerabilities or secret findings, with the unchanged
unused OpenPGP inventory warning documented in VERIFICATION.md. No schema or MCP
contract/consent changes: this is runtime packaging for the existing browser-only
banking workflow. No homelab changes or live banking were performed.


## Local FNB investigation — 13 October 2026

Local-only branch `codex/fnb-local-refresh-diagnostics` adds phase-5 step/failure
counts, privacy tests and network-disabled real Chromium synthetic login coverage.
Image `sente:fnb-local` is built and accepted locally. Repeat with
`bash scripts/run-fnb-local.sh`; the isolated `sente-fnb-local` Compose project
serves http://127.0.0.1:5180 with separate named data/backup/key volumes. Keep
Show Chrome off. Owner live trial supplies the next safe counts; no cause of the
actual bank failure is claimed. No changes pushed, no homelab deployment and no
schema/MCP contract or consent expansion. Once this local UI contains owner bank
credentials, agents must not invoke live refresh or inspect its database/profile.


Owner retry narrowed the actual failure to first navigation, before credential
entry. Extended only fixed exception-kind/network-code counters; added an
owner-run public Chromium navigation probe that loads no stored credentials and
returns no page content/URLs/raw errors. Do not run that probe from agent tools.
The active local project's test credentials/database/profile must not be read.


The owner public probe also fails before login. Expanded diagnostics cover all
249 registered Chromium network-error symbols as fixed numeric values, bounded
nested causes, frame detachment and browser/page crash events. The exact live
cause is still unknown until the next owner probe result; agents did not open
FNB. 53 worker tests cover numeric-only handling, unknown symbols, cyclic causes,
main-request filtering and event-listener cleanup.


The first real networking regression reproduces a Chromium child seccomp crash
on pwritev2 in the Alpine libc/browser pair. `pwrite-compat.c` and its native
build test implement/test standard pwrite64 selection, cancellation/errno and
64-bit offsets; the browser launcher preloads the fixed immutable image path.
No sandbox or TLS bypass. Acceptance now includes real loopback HTTP and child
seccomp crash rejection. Debian candidates were rejected after package scans;
an older-browser Alpine candidate was not chosen. Owner retry on the current
browser plus adapter is needed to establish actual FNB compatibility. Do not
inspect live banking or the owner's persistent local data/profile.


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

The owner reported a headful public-navigation renderer crash followed by a timeout. Five offline synthetic navigation iterations across four tabs with dedicated workers passed; WebGL2 is blocklisted on Xvfb. The public-only probe now emits fixed numeric stderr crash categories and available tmp/shm MiB. Raw stderr, URLs and page content are never returned. No MCP/API allowlist or live worker consent is expanded. The cause of the owner-reported crash remains unconfirmed.

### Production container virtual display

Owner-approved container-only non-headless mode is the Docker default for manual and scheduled workers. No desktop window or remote display port is exposed. Xvfb is per-browser, uses random 0600 local authorization and disables TCP; fresh profiles and confirmed logout remain required. Native development behavior remains unchanged. No new MCP tools, schemas, output fields or consent scopes are introduced: browser runtime selection is host deployment configuration, and the expanded fixed worker diagnostic allowlist remains browser-only. Owner observed a successful public FNB homepage navigation in non-headless mode and a validation redirect in headless mode; actual login, logout and scheduled banking compatibility remain owner-tested and are not established by synthetic CI.

Individual Chromium protocol calls are bounded to 15 seconds, below the worker and Go run deadlines. Login-stage protocol/target/timeout failures are propagated rather than swallowed as approval waits; fixed numeric diagnostics remain the only reported error details. Owner testing found a retry starts a fresh worker, so the observed spinner is an in-flight request, not an image mismatch or permanently locked button. No API/MCP schema or consent change is required.

Owner live diagnostic: initial navigation reports login_page_crashed=1 and timeout before credential entry. Container cgroup OOM/kill counters are zero; after cleanup /tmp and /dev/shm are empty, which does not establish their peak usage. The actual worker now uses the same fixed numeric browser-crash classifier as the public probe (sandbox syscall, GPU/WebGL, memory/storage and signal categories). Raw stderr is bounded, retained only transiently and never exported; counters remain within the browser diagnostic allowlist. No MCP schemas or consent scopes are expanded. Cause remains unconfirmed.

SIGILL root investigation: kernel metadata repeatedly maps to immutable ELF address 0x3d4de49 (UD2 after INT3) in Chromium build ID 82afe5637142447cd4acb07795ea3e94512756cd. Matching official Alpine debug symbols resolve it to v8::internal::Isolate::Throw. No browser memory/core dump or bank-page content was collected. A synthetic caught stack-overflow test reproduces the same offset in Alpine Chromium 152.0.7977.82, both headed and headless; ordinary caught exceptions pass. A 512 KiB V8 stack budget still fails. The same offline fixture passes five iterations across four tabs in Debian Chromium 155.0.8059.39. Distribution and browser versions differ, so this identifies a runtime-specific failure without proving which upstream change fixes it. No crash/sandbox/CFI check was disabled. A patched comparison image is being evaluated before any production runtime change.

Alternative runtime verification: isolated `sente:fnb-ubuntu-chrome` uses pinned Ubuntu 24.04 digest 534baea6a22c03a63003dbc8dbe78fe34bc0d7e595d9a9dc9834884ff530eb55 and official Chrome 155.0.8059.79 (package SHA256 b7726070ca707965e9a17b38e4f15ba283bf6f65c339626c092862af7b19badc). It passes the new caught-stack-overflow regression and complete synthetic production-image acceptance with namespace/seccomp protections. The Alpine pwrite adapter is absent. Final cached Trivy scan: Ubuntu 21 medium, 26 low, zero high/critical; Node packages zero; unchanged unused Go OpenPGP inventory advisory UNKNOWN; zero secrets. The scanner does not prove the absence of unknown flaws or exhaustively cover Google Chrome; the browser version was checked against Google’s October 13 stable release/security notes. Candidate-only Dockerfile and packages remain under ignored work/, not a production runtime change. The local sente-fnb-local stack uses this candidate for owner testing; original data/backups/key volumes are retained. Live bank compatibility and review of remaining findings are outstanding.

Newer native Alpine comparison: isolated `sente:fnb-alpine155-test` uses official edge base digest 020dfcbaaf4cc1078bf2d9c7ba31a8466e334061dcd2f248001d68f79e52c000, reported OS 3.25.0_alpha20260805 and native Chromium 155.0.8059.39-r0. Both the original adapter-equipped image and `sente:fnb-alpine155-noadapter` pass caught JavaScript stack overflow, synthetic login, real loopback networking and namespace/seccomp checks. The no-adapter variant also completes five iterations across four tabs with audio, canvas, workers, storage, WebAssembly/WebGPU requests and full synthetic production-image acceptance (onboarding/MCP, restart, upgrade, backup/restore and FNB key/recovery). No bank network, real credentials, production database or application volumes were used. Cached Trivy scan of the same package inventory reports zero Alpine/Node package vulnerabilities/secrets and the existing unused OpenPGP inventory advisory; it warns that 3.25 is not in its EOL list, and coverage must not be treated as security clearance. Chromium is older than the October 13 Google stable patch release, so security-fix coverage needs confirmation. This rolling/alpha candidate is not integrated into the production Dockerfile or pushed as a release. The local port-5180 stack remains on the healthy Ubuntu/Chrome candidate. Owner-only public FNB navigation is outstanding.

Owner identified the fourth transaction target as a 32-day notice savings account. The DOM product classifier now maps exact known 32 Day Notice/32 Day Flexi Notice labels (optional FNB prefix, hyphen/spacing and Account suffix) to the existing Savings type. Unknown/structured investment or loan labels remain rejected. Identity, ZAR currency, named columns and posted-history evidence remain mandatory; no partial batch is persisted on failure. This changes supported browser-connector labels, not API/MCP schemas, consent, authorization or transaction-import rules. The exact owner-visible Type label and live retry remain owner-verified.

Owner confirmed exact bank Type label "32 Day Flexi Notice". All active worker tests and the offline native-Alpine Chromium synthetic notice DOM check passed after the label mapping fix. The local stack is updated to `sente:fnb-alpine155-notice-test` with its WSLg owner-viewing override retained. Live transaction retry remains owner-tested; no partial imports or unknown/ambiguous layouts are permitted.


## PR 106 consolidation (14 October 2026)

Production Dockerfile now builds the owner-tested native Alpine edge Chromium
155.0.8059.39-r0 runtime directly from pinned public base images. Removed the
positional-write C adapter/preload and obsolete local headful experiment; retained
per-browser authenticated Xvfb, namespace/seccomp, non-root/read-only execution,
private debugging pipe, ephemeral profiles and existing Compose hardening.
Caught stack-overflow image regression prevents recurrence of the Alpine 152
SIGILL. Owner confirmed discovery, repeated balance retrieval and the 32 Day Flexi
Notice transaction retry; unattended scheduling is not live-verified.

Security release caveat: Alpine's available package remains 155.0.8059.39-r0,
whereas Google's 13 October stable update is 155.0.8059.79 with 37 security fixes,
including critical issues. Reviewed Alpine APKBUILD has no documented backports
of that release's fixes. Edge scan coverage is incomplete; cached scan results
cannot establish current browser patch coverage. Resolve this runtime release
choice before merging/publishing with banking credentials.
Source: https://chromereleases.googleblog.com/2026/10/stable-channel-update-for-desktop_01668083673.html
Package: https://pkgs.alpinelinux.org/package/edge/community/x86_64/chromium

MCP impact: existing Savings product mapping and browser runtime only. No tools,
input/output schemas, consent scopes, financial authorization, monetary parsing,
proposal/audit behavior or persistence contract changes; no new disclosure fields.


## Per-rule merchant classification — issue #108, 15 October 2026

`codex/merchant-rule-classification` starts independently from development a3f2aca.
Owner clarified one merchant with several pattern rules (e.g. FNB fees/interest),
each with one category and optional group, rather than multiple-choice or split
allocations. Migration 31 snapshots existing merchant defaults into each rule;
per-rule updates never rewrite merchants/siblings or existing transactions. Ties
with different classifications remain uncategorized and bypass fallback rules.
Import freshness includes merchant rules, category archive guards include active
merchant rules, and portable configuration retains rule-specific/empty assignments.
MCP reuses existing grants/output fields and requires a new preview for pending
pre-change merchant proposals. No consent expansion or banking runtime changes.
Synthetic coverage/results follow; no owner data or manual visual inspection.

Final #108 verification: Go vet and 20 frontend unit tests passed; production
build passed with the existing bundle-size advisory. Both existing merchant/logo
browser cases and both new independent-rule editor cases passed (360px light and
1440px dark for the new cases). New synthetic importer assertions preserve exact
parent/allocation totals and keep ambiguous matches in Needs review. Focused new
merchant/migration/MCP race tests passed (54.901s), followed by final MCP proposal
binding/preview/legacy-proposal race checks (12.980s). A fresh affected MCP/import/
migration recheck also passed (16.915s). The broader merchant, historical migration,
ruleset and configuration race run completed with two new-fixture errors only:
a missing historical group_name column value and omitted required MCP account_id.
Both were corrected and passed in the focused rechecks. Browser fixture mistakes
(unpaged catalogue responses and a native disclosure locator) were corrected;
final new cases passed. No full backend-suite rerun, manual visual sign-off,
external MCP-client certification or deployment is claimed. Only disposable
synthetic databases were used.

MCP preview now deliberately exposes numeric category/group assignments and one
fixed classification_state under existing merchant-read consent; free-text browser
explanations stay excluded. New inherited classification defaults are canonicalized
at preparation, so changing merchant defaults cannot change approved effects.


PR #109 CI follow-up — 15 October 2026: the complete GitHub backend race suite
found three older PWA/scoped-alert downgrade fixtures that removed migration
records but retained migration-31 columns. Reused the synthetic column-removal
helper before those historical replays. Production migration behavior is unchanged.
All three rollback/retry/history-preservation tests passed under race (13.506s).
The full CI gate must pass again on the pushed revision before merging.

## MCP period creation and explicit YOLO consent — 15 October 2026

Issue #107 is implemented on `codex/mcp-period-creation-yolo` from development
`a3f2aca`. MCP period creation shares the browser transaction service, with exact
reassignment and recurring-budget effects, stale checks, once-only apply and audit.
Separate default-off creation consent and automatic approval are included in shared
OAuth/Settings controls and the owner-approved explicit YOLO preset. Existing
connections retain stored grants; no migration or financial read-field expansion.
Verification results are recorded after checks complete. No production data, live
banking, deployment or manual visual inspection is involved.

Final #107 verification: Go vet passed; affected MCP permission/OAuth/automatic/
privacy race run completed with only the revoked-token test-helper expectation
failing. After correcting that assertion, all period tests passed under race
(31.947s); financial domain packages passed. Frontend 21 tests and production
build passed (existing bundle-size advisory). Eight existing MCP browser cases
passed; the new phone/desktop YOLO cases both passed on the final recheck. The
initial desktop fixture reused the phone period dates, correctly hit the duplicate
guard and was corrected to use distinct dates. No manual visual inspection.


Combined PR integration — 15 October 2026: PR #110 passed its complete GitHub
verification gate and merged as 7833b27. Resolved the remaining PR #109 overlap
by retaining both period and merchant proposal evidence, both operation descriptions
and both documentation additions. Combined period/merchant MCP and the three
upgrade-fixture race tests passed (65.422s); Go vet, 21 frontend tests and production
build passed. Final GitHub CI on the integrated #109 revision remains the merge gate.


## Unified Rules and explicit backfill — 15 October 2026

codex/unified-rules starts from freshly pulled development ea3ba1c in the original WSL source checkout. Categories now has one searchable Rules tab, shared editor/global-search entry points, complete grouped account actions, optional merchant/category/group assignments, example testing and active-only manual historical application. Original rule IDs/scopes and two-tier matching/conflict policies remain behind compatibility adapters; no destructive conversion or manufactured merchant catalogue is introduced.

Migration 32 rebuilds category-rule tables only to permit group-only assignments, preserving IDs/authors/versions and financial/access history. Never alter migration 31 or frozen historical schema. Historical schema-30 tests now explicitly slice the registry at 30 rather than assuming the latest registry has only one newer step.

Saving a rule, including from a transaction, affects future imports only. Transaction rules can optionally assign the transaction's selected merchant. The browser offers optional backfill after creation and a manual row action; all previous matches or household transactions assigned to one saved budget period are supported. Signed 15-minute exact previews fill missing merchant/category/group only, exclude transfers/splits, preserve manual values, recheck configuration/period/access/transaction versions and commit updates/acceptance/audit atomically. Changed targets begin unseen. Scope is bounded to 10,000 matches/100,000 scanned entries; narrow to a saved period for larger histories. Shared-logo updates occur once per grouped save. Import preview now includes the group that existing merchant import commits already fill.

Owning modules: unified_rules.go (catalogue/compatibility writes/rollback examples), rule_evaluation.go (shared import/backfill orchestration), rule_backfill.go (exact browser historical application), transaction_rules.go (future-only transaction-created rules), rule_schema.go (migration 32), and web/src/features/rules (shared UI). Old editor exports are thin adapters. Plan/UI/architecture/module/configuration/MCP guides and repository Sente experience guidance are synchronized.

Verification: 21 frontend tests, synchronized production build, Go vet, domain race tests, focused rule/backfill/migration/transaction/MCP races and 72 distinct synthetic browser cases passed. The 946.436s broader affected race run had only two historical registry-fixture failures; corrected tests passed separately. It was not rerun in full after those fixture fixes/final refinements. See VERIFICATION.md for exact batches, coverage and limitations. Existing bundle-size advisory remains. All data was synthetic; no manual visual inspection, owner-data access, live banking, external MCP-client certification or production deployment.

MCP: retain separate category-rule and merchant grants/read fields/automatic approvals. Group-only save_rule uses existing category_id=0 plus spending_group_id fields and is documented in the tool description. Backfill and the optional transaction-created merchant choice remain browser-only. No consent expansion or new output allowlist fields; pending proposals gain no historical mutations. Portable versions 1/2 preserve their identities and remain compatible.


MCP recheck follow-up: corrected list_rules to honor its advertised q filter over patterns/categories/groups before grouped paging, with renamed assignments invalidating page versions. Removed its stale transaction-rule immediate-backfill description. Category/group and merchant tools remain separate under existing consent; historical application remains browser-only.


PR #111 CI follow-up: the first GitHub run passed backend race/static/unit/build gates but found two obsolete transaction-editor browser heading assertions. Updated the heading, scoped transaction/toast locators and explicitly dismissed the new backfill offer. All four transaction-editor cases passed locally with synthetic data. Product/MCP/schema/financial behavior is unchanged; the new PR revision requires its own green CI gate.


## Daily FNB clock-time refresh — 15 October 2026

On codex/daily-bank-refresh in the original WSL checkout, Settings → Banking → Automatic refresh adds Daily at a set time (default 12:00, Africa/Johannesburg). Existing interval/off settings remain compatible. Migration 33 appends default-empty daily_time; shared next-due calculation preserves the clock after manual/scheduled completion and advances past missed days. An overdue schedule runs once on the next serving scheduler tick. Attention-required pauses, owner authorization, account grants, serialized connector runs, version checks and audit stay in place. Scheduling drafts survive status polling. Replacing credentials clears both interval/daily schedules.

MCP impact: FNB credentials/settings/scheduling remain browser-only administration. No MCP tools, input schemas, output allowlists, consent, proposal previews or financial services expand; daily_time is exposed only by the existing owner-admin browser status route. No live bank run, production data access, deployment or manual visual inspection. Actual check results are in VERIFICATION.md.


## Friendly scheduling and Accounts & banking — 15 October 2026

Continuation on codex/daily-bank-refresh: the owner requested cron-like flexibility with plain-language controls and one combined settings tab. The builder offers daily/selected-weekday/monthly schedules, up to 12 distinct clock times, weekday/weekend shortcuts, explicit monthly dates and/or last day, plus custom whole-hour intervals 1–168. All clock times are SAST; missing month dates skip that month, rather than moving to a different date. Live summaries use readable names/times. One overdue run after downtime, attention-required pauses, owner/access checks, versioning and credential-reset scheduling pauses remain.

Migration 34 appends schedule_json without changing migration 33 or older records. Existing daily_time/interval settings work until explicitly replaced; the status route returns validated structured schedule data, excluding raw storage. Legacy migration fixtures now remove both added schedule columns before replay.

Accounts & banking combines the existing account-workspace shortcut and FNB settings. Former accounts settings sections resolve to banking, without a duplicate tab. Account management, discovery and user grants retain their owning screens/permissions. Scheduling drafts survive tab switches; credential drafts retain the existing clear-on-departure policy. No bank session starts from viewing/saving settings.

MCP assessment: scheduling/account administration remain browser-only. No tools, input/output allowlists, connection consent, financial services or proposal effects expand. New browser schedule fields are owner-admin scoped and audited. No real data, live banking, manual visual inspection or deployment. Actual final verification follows in VERIFICATION.md.


## Calm Accounts & banking and pause explanations — 15 October 2026

Owner requested less visual stimulation and a clear pause reason. The tab now has one FNB connection card: plain status, last/next refresh, one stable Refresh now button and quiet Open Accounts/Get transactions footer links. Automatic refresh and Connection options are collapsed; login/browser controls remain reachable, with technical diagnostics and disconnect further collapsed. Removed duplicate account card, status badge and repeated helper/action blocks. Credentials continue clearing on tab departure, and schedule drafts/pending locks remain intact.

Pause copy uses the existing recorded last_error to explain interrupted updates, phone approval, session/logout problems, unavailable runtime/key and verification failures. It gives recovery instructions and explicitly says the saved schedule resumes after success. Unknown reasons use bounded generic copy, without displaying untrusted identifiers in the main explanation. Technical error code/counts remain available under Connection options → Troubleshooting details. Running jobs now display Update in progress instead of the misleading Paused fallback. Saving a schedule still does not unpause a failed connection or initiate a bank run.

FNBConnectionOptions owns optional controls; fnbConnectionStatus owns browser-only readable status/recovery mapping. No backend services, API/MCP fields, grants, consent, proposals or migrations change in this UI follow-up. No live banking, owner data, manual visual inspection or deployment. Actual synthetic verification is recorded in VERIFICATION.md.


PR #112 CI follow-up — 15 October 2026: the complete backend race gate passed 467 tests but found the older optional-rule migration-32 fixture using the entire latest migration registry with a fixed target of 32. Pin its registry slice to schema 32 for failure/retry/restart checks, and require the injected rollback error so registry mismatches cannot masquerade as a successful rollback test. Production migration behavior is unchanged. The corrected test passed locally under race (2.459s). The pushed revision requires a fresh complete GitHub gate before merging to development.


## Independent split groups and automatic remainder — 19 October 2026

Implemented in `/home/douw/finance-tracker`, branch `codex/split-spending-groups`.
Each browser split can select its own spending group or Not set; unchanged splits
inherit the parent's Default spending group. Numbered group/category controls stack
above signed amounts/removal. The last amount is calculated/read-only using integer
cents; earlier inputs, total changes and split removals rebalance it. Incomplete input
clears it; over-allocation validates/focuses the preceding editable amount (read-only
inputs are exempt from browser constraint validation). Returning to one allocation
restores the parent amount and that allocation's effective group.

Schema 35 adds allocation group reference plus an explicit-override flag, preserving
all legacy parent inheritance without financial/version/access rewrites. Dashboard,
Sankey, scoped transaction filters/matched totals, reports, rebalance, notifications
and allocation CSVs use effective groups. Bulk group reassignment explicitly replaces
all split groups; unrelated shared edits preserve them. Split Transfer labels never
change the whole-transaction transfer flag. Module ownership and product/UI contracts
are recorded in architecture, refactoring, PLAN/UI and the repository experience skill.

MCP: no new input schema or permission. Existing financial read fields expose effective
allocation group ID/name; private notes/raw storage flags remain excluded. Exact
proposal snapshots/audits and financial consent comparisons retain split overrides;
existing MCP allocation replacements preserve overrides by position. See docs/MCP.md.

Verification: frontend build, 30 unit tests, Go vet/domain tests, affected backend
financial/migration/MCP checks and 28 distinct synthetic browser cases passed; see
VERIFICATION.md for exact coverage/repair notes. No manual visual inspection, real
financial data, live banking or production deployment. Changes remain uncommitted.


## Duplicate import protection and MCP cleanup — 26 October 2026

Implemented in the original WSL checkout, preserving the uncommitted split-group
work. Owner deferred production deployment/cleanup (Docker runs on another homelab
PC) and requested MCP merge/delete support instead. Production was inspected only
through authorized MCP reads; no production records, credentials or data files are
copied into fixtures or committed. The pending-to-posted explanation is unconfirmed:
the existing connector already excludes explicitly pending entries.

Live matching now prioritizes whole-page strong source/balance/component/fee
matches and tolerates changed references when running-balance evidence agrees.
Uncertain metadata and matching posting-date shifts within seven days stage for
explicit review. Separate identical purchases and service fees retain occurrence
counts. Schema 36 archives deliberately removed transaction/source snapshots;
merge/delete proposals require separate default-off grants, exact versions/current
access, atomic audit and re-import suppression. Existing MCP consent does not expand.
Keeper metadata remains; removed metadata is archived. Linked transfers and
fee-bearing merge limitations are documented in MCP.md. No browser cleanup editor
or public archive/restore read is added. Validation results follow in VERIFICATION.md.

Final cleanup verification: affected backend/domain race tests and vet passed; all
30 frontend unit tests and production build passed; all 12 MCP browser cases and
the final two cleanup rechecks passed. A keeper-tag fixture omission and one
synthetic server-start timeout were corrected/retried. Exact results and limits
are recorded in VERIFICATION.md. Changes remain local and uncommitted.

## Local development image publication — 27 October 2026

Owner authorized committing/pushing the pending split-group and duplicate-cleanup
work and locally building/pushing the development Docker image because Actions
runtime may be exhausted. The source commit uses [skip ci]; local full source and
hardened image verification replace the hosted run for this publication. The
initial build found the old edge Chromium package unavailable; pin the available
155.0.8059.84-r0 patch and verify the actual resulting image. Production application
containers/data remain untouched. Local evidence is retained under ignored
work/local-release-20261027 and publication assets; final results follow.
