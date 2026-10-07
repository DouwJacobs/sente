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


## User security — issues #15, #16 and #17 (2026-10-07)

`codex/user-security` in `/home/douw/sente-user-security` adds administrator password resets and explicit user deletion, and hardens the existing self-service change. Schema 20 adds nullable `users.deleted_at` without rewriting financial history. Deletion is permanent access removal: retain the username/ID as a reserved historical identity for imports, rules, audit and review attribution; clear its password, roles and membership, hide it from user lists, remove grants/personal seen markers/sessions/FNB credentials and discoveries, and revoke MCP connections plus pending OAuth consent. Accounts, source provenance, allocations, classification rules and financial history remain unchanged. Deleted identities cannot be restored through update/reset/grants. Keep an enabled administrator; self-deletion requires another administrator.

Administrator resets target another user with optimistic version checks, leave disabled status unchanged, and revoke all target sessions and MCP credentials. Self-service requires the current password, deliberately retains the initiating session and revokes other sessions/all MCP connections. Both use the existing 12–72-byte rule and audit without password material. Offline recovery shares the credential-revocation service and records its recovery method. Login rechecks the verified password and active user atomically with session creation; security mutations recheck the actor session/administrator role within the write transaction. Audit failures roll back all mutation/revocation effects.

MCP impact: account administration/passwords remain browser-only (offline recovery also remains available). No tool, input schema, output allowlist, capability or consent expansion. Connection deletion uses existing cascades for proposals, OAuth codes, access and refresh tokens, and also removes pending user-bound authorization requests. Financial audit evidence remains. Frontend uses shared forms/fields, first-blur/live errors, dependent password confirmation, native dialogs, explicit typed-username deletion and overlay toasts. No production database, banking credentials, deployment or manual visual inspection.

Verification for #15–#17: 18 focused backend tests passed with race detection, all four domain packages and vet passed, and the final frontend build/8 unit tests/7 desktop-mobile browser workflows passed. Full historical suites and manual visual inspection were not repeated. See VERIFICATION.md for exact coverage and the isolated browser-runner workaround.


User security UI follow-up: shared Field adds password eye controls across all browser password inputs; user rows now have a single hamburger menu with Edit rather than a separate pencil. This changes browser presentation only: MCP tools, schemas, output allowlists, permissions, proposal previews/audits and shared financial write services are unchanged. The dev server remains on port 5175 for this worktree; verification is recorded below/in VERIFICATION.md.

Password/menu verification: final frontend build, 8 unit tests and 9 synthetic desktop/mobile password/menu/security/onboarding workflows passed. Eye hover is transparent/borderless; keyboard focus remains visible. See VERIFICATION.md for exact coverage and corrected test selectors.
