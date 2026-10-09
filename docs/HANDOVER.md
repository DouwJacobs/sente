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
