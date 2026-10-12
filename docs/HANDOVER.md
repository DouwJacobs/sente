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
