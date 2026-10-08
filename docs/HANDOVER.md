# Maintainer handover

Updated 8 October 2026. Read [AGENTS.md](../AGENTS.md), [PLAN.md](PLAN.md) and
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

Based on clean main at f651682 (PR #74), on `codex/release-channels`. The other agent's
budget editor and PR #73 scoped alerts are merged. Owner chose public source/images and standard GPL-3.0 after
being informed that forks and sales are permitted; preserve upstream notices. Owner
confirmed development pushes should create beta tags/images and main pushes stable
versions/main/latest. [Release guide](RELEASES.md) owns version allocation, image
verification, publication, compatibility and upgrade/recovery procedures.

Local release checks and stable/beta production-image acceptance passed; see VERIFICATION.md for coverage and host-specific test limits. Review and final-revision CI are pending. No release tag, image publication, merge or
production deployment has occurred. Owner will configure `DOCKERHUB_USERNAME` and
`DOCKERHUB_TOKEN` GitHub secrets. Publication credentials never enter builds/tests.

MCP impact: existing initialization already uses the shared build version; release
metadata is verified through that contract. No tools, input schemas, output allowlists,
capabilities/consent, proposal previews/audits or financial services change. About's
GPL/source links remain browser-only. No database migration is introduced.

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
