# Maintainer handover

Updated 7 October 2026. Start with [AGENTS.md](../AGENTS.md), [product behavior](PLAN.md) and [architecture](ARCHITECTURE.md); UI work also uses [UI.md](UI.md) and the project UI skill. The [documentation index](README.md) identifies the current guides. Previous handovers and decision logs are in the [archive](archive/README.md).

## Working checkout

Edit `/home/douw/finance-tracker` directly in Ubuntu WSL; the Windows Finance Tracker folder links to it. Preserve existing uncommitted work. Run commands through `wsl -d Ubuntu` from the source directory. Go is `work/toolchain/go/bin/go`; Linux Node is `/home/douw/.nvm/versions/node/v22.21.1/bin`.

`make dev` serves http://127.0.0.1:5173 using `data/dev/finance.sqlite`. Use isolated synthetic fixtures for tests and the shared [demo](DEMO.md) for screenshots. Never copy production data implicitly. Follow [REFACTORING.md](REFACTORING.md) for module ownership and shared authorization/write services. Current owner policy permits automated checks, with no computer-use UI inspection.

## Current work

Issues #4 and #5 are paired on `codex/copy-docs-cleanup`: product/helper copy uses plain terms, and active documentation now describes current behavior without dated implementation logs. Original core guides, test history and connector notes are preserved in `docs/archive/2026-10-07`. Keep new decisions in their owning guide; keep this file short. The frontend build, 8 unit tests, 16 focused synthetic browser workflows, local links and archive-preservation checks passed; see VERIFICATION for intermediate test repairs and coverage limits.

MCP impact: browser wording and documentation only. Tools, input/output fields, permissions, exact proposal effects/audits, consent and shared finance services are unchanged. Existing technical MCP contracts remain in [MCP.md](MCP.md). No migration or financial behavior change.

## Remaining operational checks

- Live FNB compatibility and MFA/layout failures require owner-run checks. Preserve capped-history/overlap limits and never inspect credentials, bank sessions or real reports. See [FNB runtime](FNB-RUNTIME.md).
- Another overlapping OFX export is needed to establish cross-download identifier stability. Synthetic tests cannot establish complete bank history or external MCP-client compatibility.
- Configure Docker Hub secrets before image publishing. Docker is unavailable in the current WSL distro; production packaging/deployment remains separate from local copy/docs work.

[VERIFICATION.md](VERIFICATION.md) records checks for this change and links to the prior audit/refactor/demo evidence. No production deployment or live banking check is part of this work.
