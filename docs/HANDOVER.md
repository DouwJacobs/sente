# Maintainer handover

Updated 7 October 2026. Start with [AGENTS.md](../AGENTS.md), [product behavior](PLAN.md) and [architecture](ARCHITECTURE.md); UI work also uses [UI.md](UI.md) and the project UI skill. The [documentation index](README.md) identifies the current guides. Previous handovers and decision logs are in the [archive](archive/README.md).

## Working checkout

Edit `/home/douw/finance-tracker` directly in Ubuntu WSL; the Windows Finance Tracker folder links to it. Preserve existing uncommitted work. Run commands through `wsl -d Ubuntu` from the source directory. Go is `work/toolchain/go/bin/go`; Linux Node is `/home/douw/.nvm/versions/node/v22.21.1/bin`.

`make dev` serves http://127.0.0.1:5173 using `data/dev/finance.sqlite`. Use isolated synthetic fixtures for tests and the shared [demo](DEMO.md) for screenshots. Never copy production data implicitly. Follow [REFACTORING.md](REFACTORING.md) for module ownership and shared authorization/write services. Current owner policy permits automated checks, with no computer-use UI inspection.

## Current work

Portable configuration implements issues #10–#14 and #28 on `codex/portable-configuration`, in `/home/douw/finance-tracker-portable`. The main checkout is untouched. Fresh installations contain no classification configuration; upgrades preserve existing records. Settings → Configuration supports several public HTTPS repositories and JSON files, manual pulls, validated one-use previews, explicit replacement confirmation, source history and JSON export. Imports affect future transactions only. [RULESETS.md](RULESETS.md) documents format version 2, source precedence, account identity mapping and limits.

The official generic source is [DouwJacobs/sente-config](https://github.com/DouwJacobs/sente-config), release `v1.0.0`. It contains 21 categories, five spending groups and four fallback rules, with no personal EFT patterns. Starter pulls use the public repository; an explicit bundled offline snapshot is also available. Git is added to the runtime image. Private repository credentials are not used; upload a private configuration file instead.

Schema 21 adds source history and expiring configuration previews. Browser, offline and MCP catalogue/rule operations share the authorized writers. Bulk configuration remains browser/host only: no MCP tool, output field, consent or automatic-approval capability is added. Configuration forms and preview presentation have dedicated frontend modules; SQL snapshots, preview, apply and repository fetching have separate backend modules.

Full backend race tests, Go vet, 8 frontend unit tests, the production build, synthetic demo invariants and public/pinned starter pull/import/repeat-pull checks passed. All 15 targeted browser workflows passed across desktop/mobile, including light/dark rule application. Details are recorded in [VERIFICATION.md](VERIFICATION.md). No production deployment, real financial data or live banking inspection is part of this work.

## Version and support batch — issues #20–#23

Prepared on `codex/version-about-support` in the shared WSL checkout: Git/build-derived backend version metadata, a sidebar version button, Settings → About for every signed-in user, and safe GitHub issue-report prefills. Release builds and Docker workflow arguments share `internal/buildinfo`; MCP initialization now uses its version too. Configuration import/export workflows are outside this batch. Release/image publishing remains covered by the repository workflow; no production application deployment was performed.

Verification: 10 frontend unit tests, production build, 8 focused synthetic browser workflows at 1440px/360px in light/dark, focused backend endpoint authorization/privacy and MCP initialization race tests, Go vet, synthetic tagged/untagged/modified Git metadata checks and executable linker-metadata probe passed. Domain race suites passed; broad app/MCP runs were interrupted and are not full-suite passes (see VERIFICATION). Temporary test harnesses were removed. Docker/production publishing and manual visual inspection remain unverified.

MCP impact: initialization version now shares the application version; tools, schemas, financial output allowlists, permissions, consent, proposal previews/audits and write services are unchanged. About and detailed build/report links remain browser-only support workflows. No schema migration or financial behavior change. The repository currently declares no licence, which About states explicitly.

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

Integration before merge (2026-10-07): incorporated main’s PR #43 documentation/copy cleanup, retained its active/archive organization and this branch’s security notes, and repeated the final production frontend build, 8 unit tests and six desktop/mobile password/menu/security browser workflows successfully. Backend implementation is unchanged by that integration. Temporary integration server/config removed; dev on port 5175 remains available.

## About presentation follow-up

Owner-requested refinement: sidebar version is a single quiet label; About now uses Sente branding, grouped project links, a prominent report action and inset build details. Production frontend build and 8 focused synthetic desktop/mobile/light/dark workflows passed; no manual visual sign-off. The existing dev server at 5173 hot-reloads these changes. MCP/build/report data contracts are unchanged.

The sidebar version now uses plain footer text styling with zero spacing and no hover surface, at the owner's request. Focus remains visible. Final frontend build and light/dark synthetic structural checks passed; no MCP changes.

## Portable configuration final integration

Ready for main after owner acceptance: card spacing matches the active theme on desktop/mobile, current account-security/About features are preserved, and portable configuration uses schema 21. Full backend race tests, Go vet, 10 frontend unit tests, production build, live public starter pull and all 10 focused integrated browser workflows passed; see VERIFICATION.md. The development server at 127.0.0.1:5173 is running from the portable worktree. The config repository's main branch has the expanded starter and three optional packs; Gifts received and personal transaction rules are excluded from public packs. The bundled offline snapshot remains the original release snapshot. MCP contracts and consent are unchanged.
