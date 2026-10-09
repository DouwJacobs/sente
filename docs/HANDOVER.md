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

## CI runtime reduction  9 October 2026

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
