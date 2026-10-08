# Maintainer handover

Updated 7 October 2026. Start with [AGENTS.md](../AGENTS.md), [product behavior](PLAN.md) and [architecture](ARCHITECTURE.md); UI work also uses [UI.md](UI.md) and the project UI skill. The [documentation index](README.md) identifies the current guides. Previous handovers and decision logs are in the [archive](archive/README.md).

## Working checkout

Edit `/home/douw/finance-tracker` directly in Ubuntu WSL; the Windows Finance Tracker folder links to it. Preserve existing uncommitted work. Run commands through `wsl -d Ubuntu` from the source directory. Go is `work/toolchain/go/bin/go`; Linux Node is `/home/douw/.nvm/versions/node/v22.21.1/bin`.

`make dev` serves http://127.0.0.1:5173 using `data/dev/finance.sqlite`. Use isolated synthetic fixtures for tests and the shared [demo](DEMO.md) for screenshots. Never copy production data implicitly. Follow [REFACTORING.md](REFACTORING.md) for module ownership and shared authorization/write services. Current owner policy permits automated checks, with no computer-use UI inspection.

## Current work

Main is synchronized through PR #47 (commit b50c0de). The three merged worktrees were removed; ignored files were archived under backups/worktree-cleanup-2026-10-07 before cleanup. Earlier feature notes below are historical; their changes are now on main.

Merged PR #47 came from codex/mcp-tools-context-ci. Issues #6–#8 add pull-request CI, a single allocation query for each review-queue page, and permission-scoped financial summaries/period comparisons. Existing budget tools remain authoritative for limits and carry-forward. Aggregates share transaction filtering, count parent cash movement once, calculate spending/income from allocations, exclude transfers from those totals, and preserve merchant/account consent and private-account budgeting boundaries.

Schema 22 adds personal MCP context. The owner chose one context per user, with explicit sharing permission for each connection. Existing and new connections default to no context sharing. Browser context editing uses optimistic versions, authorized atomic saves and audits containing size/version only. Initialization receives permitted context without mutating shared server instructions; get_session_context rechecks current consent. Clients are instructed to refresh at the beginning of each conversation, but server delivery cannot guarantee a client follows those instructions. Context is intentionally shared verbatim; ordinary financial output allowlists remain unchanged.

The MCP settings page separates connection setup, personal context, agents and proposals. Permission forms explain read access, account scope, proposal capabilities and optional automatic approval. Shared validation, toasts, request locks and focus behavior remain in use; settings tabs preserve drafts. Connection and proposal presentation have dedicated feature components.

CI runs race tests, vet, frontend tests/build and synthetic Chromium workflows. The browser runner builds once and gives each spec fresh disposable databases, preventing import/security tests from contaminating later specs. No production financial data, deployment or live bank/client testing is involved. See VERIFICATION.md for final evidence.

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

## Mobile core workflows — issues #48, #49, #51 and focused #53 coverage

Implementation is on `codex/mobile-core-workflows` in the primary WSL checkout, created after fetching and fast-forward checking main at b50c0de. Navigation adds the fresh Review shortcut, moves Accounts into More, tracks active queues/secondary pages, supports More dismissal/focus return and shares measured navigation clearance. Phone headers, selection targets, signed amounts, account/period filters and status text are readable and touchable. Editor essentials/split inputs stack, and save/close actions remain reachable; shared dialogs follow visual viewport size/offset and lock background scrolling while retaining nested drafts/focus and existing validation.

The owner also requested lasting mobile standards in the project UI skill; `.agents/skills/finance-tracker-ui/SKILL.md` now routes to the mobile core section of UI.md and records the practical conventions and verification approach. The owner subsequently requested compact dashboard refinements from #50; broader budget restyling, Settings layout work (#52) and full #53 coverage remain follow-ups.

MCP impact: browser presentation/verification only. No backend financial services, tools, input schemas, output allowlists, permissions, consent, proposals or audits changed. Existing money, acceptance, personal seen and authorized write behavior is preserved. No production data/deployment/live banking was used.

The existing `make dev` supervisor runs from this checkout with the separate persistent development database at http://127.0.0.1:5173; health returned 200/ok. Leave it running for owner feedback. Targeted computer-use inspection was explicitly authorized for this batch: 360px light transaction list/filters/split editor/save footer, 360px dark More navigation, and 1440px dark desktop list. Real-device keyboard/safe-area checks remain unverified. Final automated evidence is in VERIFICATION.md.

Final mobile verification: 10 frontend unit tests, production build, 38 focused synthetic browser workflows and UI-skill validation passed. The full historical browser/backend suites were not repeated. Targeted owner-authorized visual coverage and intermediate runner issues are recorded in VERIFICATION.md. Changes remain local on the mobile branch for owner testing; no merge/deployment was performed.


Development preview recovery (7 October 2026): owner reported a black screen at 5173. Browser logged a missing MCPPermissionFields module export; Vite served an empty transformed MCPPermissions.tsx despite the current source containing that export. Restarting the existing make-dev supervisor cleared the stale module cache. Both WSL and Windows HTTP checks passed, and a fresh in-app browser tab rendered the sign-in page with no console errors. Persistent dev data was preserved; no application/MCP contract change was required. The new supervisor log is work/dev/mobile-dev.log. After pulling/switching branches while Vite is running, verify actual dev startup/export delivery and restart if its cached module graph is stale; production-build checks alone do not establish HMR preview health.

## Owner mobile and classification refinements

After the initial mobile batch, owner feedback requested compact balanced dashboard scope/totals, padded group/category figures, quiet navbar search, denser transaction rows with accessible review/seen icons and larger centred merchant logos. Mobile now uses long press to enter selection, row taps to toggle, checkmarks over logos and automatic exit when the last selection is removed. Selection count stays in the existing toolbar; actions use its shared menu and do not insert shifting button/helper rows. Keyboard Space and explicit menu selection remain available, while desktop checkboxes remain. Categories now reuses Spending groups' grid/choice rows while preserving type labels, editing and scoped navigation. Supplied Vault22 screenshots inform spacing and hierarchy only. UI.md and the project skill retain these conventions.

MCP impact: presentation/interaction only; no tools, schemas, output allowlists, permission/consent expansion, proposal/audit changes or backend financial-service changes. Exact-money, category acceptance and personal seen semantics remain intact. Final coverage is recorded in VERIFICATION.md. Keep the dev server at 5173 running for owner feedback; changes remain local on codex/mobile-core-workflows.

## Compact workspace controls (7 October 2026)

Owner requested a commit before further changes. Checkpoint 50e691f (Refine mobile workflows and compact classification lists) contains the prior mobile/category/rules batch. The owner accepted the control refinements on 8 October 2026 and authorized commit, push, merge and closure of completed issues. Dashboard scope is unboxed with compact period navigation; Transactions keeps search and a short scope summary visible while account/period and detailed filters are collapsed behind Filters. Compact headings, tabs, toolbar gaps and select-all header put content higher on both mobile and desktop. Escape closes Filters and restores its trigger focus. Filter drafts, review defaults, imports, financial calculations and permissions remain intact.

MCP impact: browser presentation only; no tools, schemas, allowlists, consent, proposals/audits or shared financial services changed. UI.md and the project skill document the conventions. See VERIFICATION.md for actual check coverage. A requested synthetic visual preview was rejected by automatic approval review due to the standing no-computer-use instruction in AGENTS.md; no visual inspection of this control batch was performed.

## Owner acceptance and merge scope (8 October 2026)

The approved mobile/core and compact-control batch completes issues #48 (workspace navigation), #49 (transaction rows/filters/selection) and #51 (dialogs/forms). Close those through the pull request. Issue #50 has dashboard improvements but broader budget-builder/state coverage remains; #53 has focused mobile coverage but Settings/admin and full acceptance coverage remain. Keep #50, #52 and #53 open and record the delivered portions on their issues. No deployment is included. Retain the local dev server at 5173. Final verification evidence above remains authoritative; no source changed after owner acceptance.

## Sequential database migrations — issue #9 (2026-10-07)

Implemented on `codex/database-migrations` in the isolated WSL worktree `/home/douw/sente-database-migrations`, based on main at b50c0de. The active mobile checkout and dev server were left in place. Schema 23 adopts a named, consecutive migration registry with one atomic transaction for all pending callbacks and version records. Registry/history validation rejects missing sequential entries and newer schemas. The pinned connection owns table-rebuild foreign-key settings and restores enforcement after success/failure; foreign-key checking gates commit.

The version-23 bridge freezes the former version-22 additive schema snapshot and retains sparse historical markers without claiming reconstructed historical release migrations. Named legacy data steps preserve the existing version gates, classification seeds for older installations only, acceptance/seen migration, budget conversion, merchant IDs/rules, source fingerprints and legacy MCP permissions. Fresh installs still have no classification seeds. After adoption, current-version startup no longer replays the snapshot or category/source/permission backfills. Future schema changes must append migrations rather than edit the snapshot. AGENTS.md now makes the append-only registry, runner-owned transaction and preservation/failure-test requirements explicit. Previously ignored category-backfill errors now roll back the entire upgrade, including merchant rebuilds and added columns.

MCP impact: persistence refactor shared by browser/MCP/offline startup only. No MCP tool, input schema, output allowlist, capability, consent, proposal preview/audit or authorized financial write contract changed. Existing grants, explicit permissions, contexts and financial provenance remain intact. No production database, bank session, deployment or UI changes were involved. The worktree uses the existing WSL Go toolchain; no dev service was started here.

Focused upgrade tests, all eight new migration test groups with race detection, the full backend race suite (app 1142.545s; domain packages cached), Go vet and synthetic demo invariants passed. Final commands and coverage are recorded in VERIFICATION.md. Frontend/browser checks are outside this backend-only change.

## Notification foundation — 8 October 2026

Migration issue #9 merged in PR #56 (f3f068c) after mobile PR #54. This task reviewed the final runner/compatibility bridge, preservation/failure tests and agent evidence; independent sequential runner tests passed. Migration PR CI was still running when notification work began. The notification branch is codex/notification-foundation in /home/douw/sente-notifications, based on that merged main; the primary/mobile checkout and dev data were preserved.

Issues #29–#30 groundwork plus backend preparation for #31: schema 24, shared permission-scoped event service, transactional in-app adapter/receipts, bounded inbox/list/count/read/dismiss APIs, versioned personal preferences and startup/daily retention maintenance. No financial producer, frontend centre, Settings form, PWA or push was enabled. NOTIFICATIONS.md describes exact scope, immutable retry/reset requirements, source/account permission rechecks and 90-day payload/180-day receipt retention with per-user caps. Soft user deletion clears the new personal state. Existing migration tests keep their synthetic 24/25 registry based explicitly on baseline 23 so future real migrations do not invalidate their failure scenarios.

MCP impact: browser-only personal communication/preference workflows. Existing MCP consent, tools, schemas, output allowlists, proposals/audits and financial write behavior are unchanged; no message enters saved MCP context. Backend synthetic verification only, without production data, banking, deployment or UI inspection. Verification results are recorded in VERIFICATION.md.

Notification verification completed: full backend race run passed (app 811.390s), all 11 focused notification groups passed, the added backup/restore group separately passed, corrected legacy upgrade/default coverage passed, Go vet/diff hygiene and synthetic demo checks passed. Draft PR #57 contains the groundwork. The primary main checkout was pulled to include migrations as requested. No dev or production notification feature was activated.

## Mobile budgets, Settings and regression matrix — issues #50, #52, #53 (8 October 2026)

Pulled main at 76af29f and created `codex/mobile-budgets-settings` in the primary WSL checkout. Owner accepted the browser presentation on 8 October 2026 and authorized commit, push, merge and closure of #50, #52 and #53 without waiting for CI. No additional tests or production deployment are part of that authorization. The existing development preview remains available at http://127.0.0.1:5173 (HTTP 200); its persistent data was preserved.

Dashboard figure tracks adapt to their own available width, keeping labels associated with complete signed values. Exceptional overview totals get wider tracks and responsive type while ordinary totals retain the compact layout. Budget builders stack long headers, totals, amount/schedule fields and separate removal controls on phones; empty builders/groups and long budget-period lists are covered. Settings adds visible 44px tab scrolling controls and reveals the active tab on changes/resizing while preserving keyboard navigation and mounted General/Configuration/MCP drafts. Banking credentials still clear on leave. Long user names, menu placement, Configuration comparisons and MCP permission/proposal content stay bounded. Shared Field now shows inline errors for unchecked required confirmations, including live recovery, without changing server consent requirements.

Final verification: 10 frontend unit tests, production build and 116 distinct synthetic browser workflows passed. This includes 43 new budget/Settings/state cases, the existing core/breakpoint and financial/permission/security workflows, and a tested 11-case `npm run test:mobile` smoke command. CI runs that bounded set first and excludes its tags from the remaining suite. UI.md and DEVELOPMENT.md record conventions/commands; VERIFICATION.md lists actual coverage. No full historical browser/backend suite, manual/computer-use visual inspection, physical-device keyboard/safe-area or non-Chromium verification is claimed.

MCP impact: browser presentation/validation and synthetic verification only. No backend/shared financial services, MCP tools/input schemas/output allowlists, consent/capabilities, proposal previews/audits or API data contracts changed. MCP permission choices and exact proposal data retain their existing semantics; UI wrapping/confirmation errors do not expand consent. These workflows remain browser-only, so docs/MCP.md needs no contract update. No production financial data or bank/client session was used.

## Notification centre and preferences — 8 October 2026

After the other agent completed and PR #58 merged, main was pulled to 272f080 and implementation began on `codex/notifications-centre-preferences` in the primary WSL source. Issues #31–#32 now have browser workflows: header unread indicator, persistent paged centre, read/read-all/dismiss, authorized source links, and personal in-app preferences with optimistic saves and draft-preserving Settings panels. Counts and inboxes refresh on visible 30-second intervals/focus and after state writes. Preferences default according to the existing backend contract; explicit reload replaces unsaved drafts. No financial alert producer is enabled, so a normal inbox can stay empty until the #33–#38 producer/suppression work ships. #39 push/PWA and #40 diagnostics also remain.

MCP impact: notifications/preferences remain personal browser-only communication workflows because connections have no notification-message consent. Existing tools, input/output schemas, financial allowlists, permissions, proposal previews/audits and shared financial write services are unchanged. Backend schema/delivery/retention APIs are unchanged. Dedicated tests seed only disposable synthetic inbox records; no owner financial data, production database, deployment or computer-use inspection.

Notification UI verification completed: frontend production build, 10 unit tests, 3 notification and 5 About/navigation synthetic browser workflows, all 12 notification backend race groups (56.232s), and diff hygiene passed. The final run confirms the stale-draft reload correction. Geometry covers 360/390/430/760/761/1440px and both themes, including short landscape. No manual visual inspection or deployment. Automatic approval review rejected push/PR creation because source export to GitHub was not explicitly authorized; work remains local for review and #31–#32 stay open.

## Notification preference save refinement — 8 October 2026

Owner requested one save action for all notification types. Settings now uses one shared Form and Save changes button; draft values survive tab changes and failed saves. `PUT /api/notifications/preferences/batch` accepts only changed, distinct registered in-app preferences (maximum six), uses the existing session/version checks and audits, and commits all changes together. The existing single-item API remains compatible. Stale versions or audit failures roll back the entire batch. No schema change. Notification detection/threshold values remain producer work in #33–#38; the current six controls choose delivery eligibility, not detector parameters.

MCP impact remains browser-only personal preferences, without tools, schemas, output allowlists, permission or consent changes. The combined endpoint calls the same authorized preference service as the existing single-item endpoint. Publication remains pending the earlier explicit approval request; this refinement is local.

Combined-save verification: final production build, all 3 notification browser workflows (isolated artifacts), both backend preference race groups (15.427s), Go vet and diff hygiene passed. Browser workflow changes save two types together and assert a single action. No production deployment or manual UI inspection.

Notification preference density refinement (8 October 2026): owner requested less whitespace. Labels/selects now share compact rows (two desktop columns, one phone column), repeated channel hints are removed, helper copy is shorter, and Save changes/Reload saved preferences share a compact footer. Shared Field/Form and 44px controls remain. Final production build and all 3 notification browser workflows passed; an additional run of the existing persistent workflow verified preference-select 44px heights and no page overflow at 360px and 1440px. Diff hygiene passed. No manual visual inspection or production deployment. UI-only change: backend/API, MCP tools/schemas/allowlists/consent, audits and financial services remain unchanged. Local publication approval remains pending; unrelated temporary browser configuration was preserved.

Notification publication authorization (8 October 2026): owner explicitly approved push, merge and closure of completed issues. This supersedes the earlier pending publication approval. The notification UI includes the accepted single atomic Save changes action and compact rows. #29–#30 were implemented by merged PR #57; this publication closes those foundation issues alongside #31–#32. #33–#40 and PWA #24–#26 remain follow-ups. No production deployment is included.

## Security hardening — #60, #61, #64, #65 (8 October 2026)

Implemented locally on `codex/security-hardening` in the isolated WSL checkout `/home/douw/sente-security-hardening`, based on main `8f9d673`. The primary checkout had notification producer/push changes in progress; those changes and its development service were preserved. The Codex managed-worktree helper could not resolve the WSL commit through the Windows folder link, so the isolated worktree was created directly with WSL Git. No publication, issue closure, migration, production deployment, live bank access, PWA or versioning work is included.

The shared browser transaction boundary refreshes session/CSRF, enabled/deleted state, administrator/member roles and identity before authorized mutation/audit. Browser user creation and its audit are now atomic after hashing. Login/OAuth bookkeeping shares a 4096-live-key cap and expiry pruning; serving cleans expired credentials at startup/hourly without losing live refresh-reuse evidence. Manual connector persistence carries its initiating session through network work, while scheduled runs retain current owner/grant checks. A stdin cancellation protocol gives Linux and WSL/Windows workers 30 seconds for cleanup before forced tree termination; Go-owned profiles are removed after cancellation and cancelled output cannot persist.

[SECURITY-HARDENING.md](SECURITY-HARDENING.md) records the write inventory, separate read guarantees, retention/cancellation policies and remaining owner-run FNB/OFX checks. Module ownership and architecture are updated. MCP impact: no tool/schema/output allowlist, consent/capability or proposal/audit expansion; existing authorized financial services/reuse detection remain authoritative. User/password administration and connector lifecycle remain browser/offline-only. See VERIFICATION.md for actual coverage.

Security verification completed: full backend race suite passed (app 1729.842s), followed by focused final guard/security/credential/interop coverage (154.090s), transaction/import and IPC checks (13.054s), manual backup/restore checks (105.814s), and final eight-case Linux/WSL/Windows subprocess plus real blank-page Chromium/Chrome cleanup coverage. Go vet/diff hygiene, frontend build/10 unit tests, all 46 worker tests and 24 selected synthetic browser workflows passed. Full-suite coverage was on the initial implementation; later refinements have the focused final checks listed in VERIFICATION.md. No publication, live banking or production deployment was performed.

Security publication authorization (8 October 2026): owner explicitly requested pushing this branch and creating a pull request for #60, #61, #64 and #65. Publication is authorized; merge, deployment, PWA and versioning remain outside this request.
