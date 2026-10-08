# Verification

## MCP tools, personal context and CI — 7 October 2026

Scope: issues #6–#8 plus owner-requested MCP page/permission clarity and personal context with independent per-connection sharing consent. Schema 22 adds only personal context storage. No financial/source records are rewritten.

Completed with synthetic fixtures:

- Full backend race suite passed (app 73.476 seconds), with TMPDIR and GOTMPDIR set to /dev/shm; Go vet passed. Aggregate tests cover splits, refunds, transfers, category/merchant/date/search filters, scoped/hidden/private accounts, period ordering, paging and invalid inputs, exact month labels, and overflow.
- Context tests cover browser authentication/CSRF, current actor authorization, user isolation, optimistic concurrency, Unicode size limits, disabled default consent, explicit read-only sharing, fresh initialization/tool delivery, concurrent connections without instruction leakage, revocation, public setup exclusion, audit rollback/content exclusion, deletion and schema-21 upgrade preservation.
- Queue tests verify one allocation query for 100 authorized entries, zero for empty pages, stable ordering and subset isolation. A synthetic benchmark measured 100-entry batch hydration at 185302 ns/op versus 1356876 ns/op for per-entry hydration (about 7.3 times faster); timings are local measurements.
- All 10 frontend unit tests and the production TypeScript/Vite build passed. Python browser-runner syntax checks passed.
- All 39 browser spec files passed across isolated runs after fixture/selector repairs; all 10 focused MCP Chromium workflows passed at 1440px/360px in light/dark. They exercise OAuth read/proposal permissions, exact budget proposals, apply-after-approval, connection revocation, context first-blur/live validation, tab-preserved drafts, save/version conflict/reload, sharing/revocation delivery and modal focus/escape behavior.
- Playwright-generated synthetic screenshots of the MCP page and custom permissions were visually inspected at desktop/mobile sizes, including light/dark states. Four final captures have no horizontal page overflow. Long modal content scrolls. No computer-use control or real account data was used.

CI adds race/vet, frontend tests/build and synthetic Chromium workflows, with fresh databases for each spec and one shared backend build. Intermediate broad runs exposed outdated fixtures/selectors: spending cards included income, fresh onboarding assumed a built-in group, keyboard End assumed Security was the last tab, and ledger rows assumed redundant Accepted badges. Tests now scope spending, explicitly create their onboarding group, expect About, and verify acceptance through the authorized ledger response. Additional historical selectors now open filter/rule disclosures, exercise account editing rather than removed manual account creation and Settings banking navigation, allow editor height to follow content while checking width/scroll/focus, and verify the current Needs review acceptance/search reset. The final workflow also uses exact password-field labels to avoid visibility-button ambiguity. Shared financial behavior was not altered for these repairs.

MCP impact: three typed tools added (22 total), explicit aggregate DTO outputs, independent read_context consent defaulting false, per-user initialization delivery and immediate consent rechecks. Existing financial redaction/allowlists, proposals/automatic approval and shared money services remain in force. Context is intentionally verbatim and client use/refresh cannot be guaranteed. GitHub Actions passed backend race/vet, frontend build/tests and the first 21 browser specs, then exposed an onboarding test reopening a different period after asynchronous list reordering. The fixture now names the created period, waits for its saved total and reopens that exact card; no retries or product changes mask the failure. Live external MCP clients, Docker packaging, banking and production deployment remain unverified locally.


## Portable configuration — 7 October 2026

Scope: issues #10–#14 and #28, expanded to multiple repository/file sources, optional default configuration, manual pulls and browser export. Schema 21 adds source tracking and expiring configuration previews. Existing financial records/configuration remain; fresh databases contain no classification catalogue. The generic `sente-config` repository and release `v1.0.0` were created with public non-personal fixtures.

Completed with isolated synthetic data:

- Full `go test -race ./... -timeout=10m` passed, using `TMPDIR=/dev/shm GOTMPDIR=/dev/shm` for synthetic temporary files. Go vet passed. Domain money, ledger, classification and statement checks remain included.
- Race-enabled configuration/ruleset regression tests also passed on ordinary temporary storage. Tests cover preview rollback/audits, current admin/member/account authorization, stale configuration/access evidence, expiry/replay, explicit replacement consent, repeat-import version stability, source coexistence, retention on Forget source, invalid references/unknown fields, normalized duplicates, bounded repository output and public-host validation.
- All 8 frontend unit tests and the production TypeScript/Vite build passed. The synthetic demo explicitly imports the optional starter and preserves its financial/budget/privacy invariants.
- Live public repository fetching and complete synthetic import passed from `DouwJacobs/sente-config`. A second smoke test pinned `v1.0.0`, exercised browser preview/application and a manual repeat pull, and confirmed no changes on an unchanged revision.
- Eight targeted Playwright workflows passed for file import/replacement/export/source retention, Settings drafts/navigation/ordinary-user visibility, transaction-created rules and an empty installation's optional starter import, at 1440px and 360px. The final run passed all 15 workflows, adding rule management and pending-rule application with 1440px/360px and light/dark coverage. The earlier broader run passed 11 but stopped four pending-rule workflows on stale helper text/closed disclosures; their selectors were corrected to the current UI before the successful final rerun.
- Whitespace checks passed, and all 79 local links across the active guides, root README and configuration snapshot docs resolve.

Intermediate checks: one full backend run found the branding test's obsolete assumption that a fresh database seeds 11 groups, then timed out during slow WSL disk syncs after 15 minutes. The assertion now checks preservation of the actual original group count. A subsequent run was stopped when inspection showed Go 1.27's `testing.TempDir` uses `GOTMPDIR`, overriding the initial `TMPDIR` choice; pointing both variables at memory-backed storage allowed the full race suite to pass in about 50 seconds. Application SQLite settings were never relaxed. An initial starter test exposed ambiguous historical Salary category names; explicit empty historical identity keys were added to starter rule references. Starter smoke tests then passed. Cached formatter discovery was used after an offline npm formatter invocation found no matching tarball; no dependency was installed.

MCP impact: shared authorized entity services are reused; browser/host bulk source operations add no MCP tools, input schemas, output allowlists, consent or automatic approval grants. Docker runtime packaging adds Git but the image was not built because Docker is unavailable. Public-repository smoke tests do not validate private Git authentication, which is deliberately unsupported by browser pulls. No manual visual sign-off, live banking inspection or production deployment was performed.

## Issues #4 and #5 — 7 October 2026

Scope: plain product copy and current documentation. UI edits change text only; no financial behavior, runtime configuration, schema, API fields or permission logic changed. Core guides and FNB notes now separate current contracts from archived development history.

Completed with synthetic fixtures:

- All 8 frontend unit tests and the production TypeScript/Vite build passed.
- 16 distinct Playwright workflows passed across focused runs: transaction navigation/drafts/metadata/filters, reports/rebalance/account health, bulk edits/category archive/merchant preview, daily guide/export, FNB connection controls, Income/merchant permissions, and MCP consent/proposal/manual/automatic approval/revocation. Coverage includes 1440px/360px, with MCP light/dark variants.
- All 118 local Markdown links resolve. All nine archived originals retain their complete text, apart from historical notices and relocated links.
- Syntax-tree comparison confirms text/comment-only source edits in six frontend files. Whitespace checks passed.

The first browser run passed 9 workflows; 7 stopped at outdated Permission/Account labels. After those selectors were corrected, the bulk workflow passed and MCP reached a hidden automatic-approval checkbox. That rerun was stopped after the first timeout (one further test interrupted). The test now opens the existing disclosure; all 6 MCP workflows passed on a fresh synthetic service. Tests were aligned with current controls without changing application behavior. Temporary configurations were removed.

The full backend and historical browser suites were not rerun for text/docs-only changes. Browser tests built the unchanged backend and exercised the affected integration workflows.

MCP impact: connection/privacy/approval copy is clearer, but tools, schemas, output fields, grants, proposal evidence/audits, consent and shared write services remain unchanged. No new MCP capability or browser financial operation is introduced.

## Issues #20–#23 — version and support batch, 7 October 2026

Implemented the shared application version source, sidebar version link, About tab and Report an issue link. Build responses are authenticated/non-cached and contain only version, revision/date and modified state. Issue reports whitelist these fields, excluding arbitrary extra user/account/financial fields. No configuration import/export workflow, database migration or financial behavior changed.

Passed:

- 10 frontend unit tests and the TypeScript/Vite production build.
- 8 focused synthetic Chromium workflows: About/report links at 1440px/360px in light/dark; metadata failure/retry; desktop/mobile Settings drafts and keyboard navigation; ordinary-user management-request isolation and About visibility. No external report was submitted. No computer-use/manual visual inspection was performed.
- Browser endpoint authorization for anonymous/admin/ordinary/viewer users, exact public output fields, `no-store` and unsupported-method handling.
- Go vet, temporary Git fixtures for untagged/exact-tag/dirty-tag metadata and malformed linker-input rejection. A temporary executable probe verified injected version, full revision and revision date through `buildinfo.Current()`. Go production compilation with the metadata flags succeeded; Docker packaging is unavailable.
- Whitespace checks and cleanup of temporary browser configs, synthetic service/database and linker probe.

Backend race verification: domain suites (`classification`, `ledger`, `money`, `statements`) passed in the broad run. The full app suite was deliberately interrupted after about 14.5 minutes, and the broad MCP-prefixed run after about 6.8 minutes; neither is a passing full-suite result. The final race run of `TestBuildInfoAuthorizationAndSafeOutput` and `TestMCPPrivacyAndTransport` passed (20.7 seconds), followed by a passing `go vet ./...`.

The initial standard Playwright harness stalled while probing unbound WSL loopback ports; only our runs were interrupted. A temporary harness prestarted its isolated synthetic service before Playwright, then removed its configuration. The first focused run passed 7 tests and found an ambiguous recovery-test selector matching both the inline status and shared error toast. Scoping that selector to About fixed the test; all 8 passed on the next run.

MCP initialization now advertises the shared application version rather than an independent hardcoded version. No tools/input schemas, financial output fields, grants/consent, exact proposals/audits or shared writes changed. Detailed build/About/report links remain browser-only support features. Docker image build/publishing, live banking and manual visual sign-off are outside this verification.

## Previous evidence

The [archived verification record](archive/2026-10-07/VERIFICATION.md) retains the detailed audit/refactor/demo results, including intermediate failures and reruns. The refactor covered 168 distinct backend tests with race detection across two runs and 55 distinct synthetic browser workflows across main/focused runs. The combined branding/demo branch passed 8 frontend tests, a production build, demo invariants, 3 focused MCP/OAuth tests, Go vet and 2 handover workflows. These are previous results, not a full-suite rerun for this change.

## External limits

- Live FNB login/MFA/layout compatibility and complete history require owner-run checks; recent pages may be incomplete.
- OFX identifier stability across separate overlapping downloads needs another external sample.
- External MCP clients require their own connection/consent checks.
- Docker is unavailable in the current WSL distro; image build/publishing and production deployment have not been verified here.
- Current owner policy excludes computer-use UI inspection. Automated workflow/DOM/geometry checks do not provide manual visual sign-off.

No production data, credentials or live banking session was inspected. No production deployment is part of this change.


## Issues #15–#17 user security — 2026-10-07

Branch/worktree: `codex/user-security`, `/home/douw/sente-user-security`, based on main. Schema 20 is an additive nullable deletion marker. Synthetic migration checks preserve users/sessions; deletion preserves ledger/source/rule/audit/review foreign keys and reserves historical usernames while removing active access and credentials. Current-session retention for self-service is explicit; administrator resets revoke every target session. Every password path revokes MCP connections and user-bound pending OAuth consent, with no MCP tool/schema/allowlist/capability expansion. Browser-only account administration uses shared validation/dialog/toast controls.

Passed: 18 distinct focused backend tests with race detection across the main security/OAuth/authentication/setup run (16 tests) and migration/disabled-reset follow-up (2 tests); all four financial domain packages; `go vet ./...`; final frontend production build; 8 frontend unit tests; 7 synthetic Playwright workflows (4 new security workflows at 1440/360px plus 3 settings regressions); `git diff --check`. Security coverage includes current-password checks, byte bounds, administrator permissions, CSRF, stale versions/concurrent resets, session/agent/OAuth revocation, retained disabled status, last-administrator/self-deletion protection, history/foreign-key preservation and atomic rollback when audit fails.

The initial migration fixture incorrectly removed its schema marker and was corrected to represent version 19 before passing. The initial standard browser runner stalled at localhost readiness despite a healthy synthetic server; it was stopped. The successful run used a temporary one-server config and local proxy bypass on port 18580; the temporary config and test servers were removed/stopped. Tests used disposable synthetic databases only. Full historical backend/browser suites were not repeated. No manual/computer-use visual inspection, real financial data, live banking or production deployment.


Password visibility/user-menu follow-up (2026-10-07): final frontend build and 8 unit tests passed. Nine synthetic browser workflows passed across the main/follow-up runs: password toggles and menu editing at 1440/360px, four existing security workflows, and three onboarding/budget workflows. Coverage checks all ten password inputs across setup, sign-in, self-service, add/reset users and FNB credentials; value preservation, re-masking on clear, keyboard toggling, labelled input linkage, 44px targets, transparent hover background/border, validation and menu focus restoration. Initial onboarding failures came from a partial selector matching the new eye button; exact field selectors and keyboard expectations were corrected, and all three onboarding workflows then passed on a fresh disposable server. Temporary runner configs/test servers were removed/stopped; the worktree dev server on 5175 remains running. No manual visual inspection, banking interaction or backend/MCP contract changes.

Integration before merge (2026-10-07): incorporated main’s PR #43 documentation/copy cleanup, retained its active/archive organization and this branch’s security notes, and repeated the final production frontend build, 8 unit tests and six desktop/mobile password/menu/security browser workflows successfully. Backend implementation is unchanged by that integration. Temporary integration server/config removed; dev on port 5175 remains available.

## About presentation refinement — 7 October 2026

At the owner's request, the sidebar now shows only a compact version label (Development for `dev`) rather than the full commit/modified string. About adds the existing Sente mark, grouped icon/text resource links, a primary Report an issue link and an inset installation-information surface. Technical details remain available in About and the sidebar tooltip. No build data or issue-report fields changed.

The production TypeScript/Vite build and all 8 focused synthetic About/Settings Chromium workflows passed. Checks cover 1440px/360px, light/dark, compact sidebar height, aligned desktop columns and stacked mobile sections, no page overflow, keyboard tabs, ordinary-user access, draft preservation, metadata retry and safe report URLs. Whitespace checks passed. The temporary harness and synthetic service were cleaned up. No computer-use/manual visual inspection was performed; the owner's existing development server was retained and Vite hot-reloaded these changes.

MCP impact: browser presentation only; initialization metadata, tool schemas/outputs, consent, permissions, proposals/audits and shared services are unchanged.

Sidebar spacing follow-up: removed shared button styling, margin/padding and inherited control minimum height from the version text. It retains semantic activation and visible keyboard focus, with no hover surface. The initial synthetic DOM check detected the inherited minimum height; after explicitly resetting it, light/dark computed-spacing, compact-height, hover and keyboard-focus checks passed. The final production frontend build passed. This is browser presentation only; MCP and build contracts are unchanged.

PR integration check: rebased the version/About batch onto main containing PR #44 user security. Resolved append-only documentation/style conflicts by retaining both features, including each CSS block's closing braces. Integrated code passed 10 frontend unit tests, production frontend build, focused build-endpoint/MCP initialization race tests and Go vet, plus 10 synthetic About/Settings/password-menu workflows at desktop/mobile sizes with About light/dark coverage. No configuration import/export implementation is included.

## Portable configuration integration — 7 October 2026

Integrated current main's account-security and About changes. Portable configuration uses schema 21, following account security's schema 20; additive migration fixtures cover upgrade preservation. Configuration forms now have equal gaps above/below their row: 24px desktop and the active theme's 20px mobile.

Final checks passed: full backend race suite (app 70 seconds), Go vet, 10 frontend unit tests, production build, live expanded public starter pull/import/repeat-pull, and all 10 desktop/mobile Configuration, Settings and About browser workflows. The first browser run passed nine and detected a four-pixel mobile spacing mismatch; correcting it produced the final ten passes. Unbound WSL port probes delayed the standard harness; prestarted isolated synthetic services allowed the final run in 10 seconds. Temporary harness and synthetic services were removed. No manual visual inspection or production deployment was performed.

The maintained configuration repository now offers the expanded 26-category starter and optional South African, detailed-category and local-merchant packs, without Gifts received or personal transfer rules. Its main branch is current; the bundled offline application snapshot and v1.0.0 config tag remain deliberate older snapshots. Repository reads/imports remain manual, and MCP tools/consent/output contracts are unchanged.

## Mobile core workflows — 7 October 2026

Branch: `codex/mobile-core-workflows`, based on up-to-date main b50c0de (merged PR #47). Covers navigation #48, transaction presentation #49, dialog/editor #51 and a focused subset of #53. Dashboard/budget layouts #50, Settings layouts #52 and the broader #53 matrix remain follow-ups.

Final verification passed:

- 10 frontend unit tests (`npm test`).
- Production TypeScript/Vite build (`tsc -b` and `vite build`).
- 38 focused synthetic browser workflows, with the existing disposable-database runner: `mobile-core.spec.ts` (15), `mobile-edge-layout.spec.ts` (4), `editor-layout.spec.ts` (2), `review-defaults.spec.ts` (5), `transaction-filters.spec.ts` (4), `transaction-editor.spec.ts` (6) and `toasts.spec.ts` (2).
- Mobile core coverage includes 360/390/430px phones, 640x360 landscape, light/dark, navigation/More dismissal and focus, fresh Review defaults, independent selection, stacked scope controls, non-overlapping selection headers, split validation/focus/live recovery, nested category picker draft/focus retention, exact saved allocation sums and final-page navigation clearance. Breakpoint checks cover 699/700/701/760/761/1024/1440px. Edge-layout tests use synthetic response overrides for long household/merchant/group/account names, large signed values and non-member navigation presentation; they do not establish server authorization.
- Existing editor/filter/review/toast workflows preserve scoped navigation, classification defaults, seen actions, linked-transfer guards and accessible overlay feedback on desktop/mobile.
- Updated project UI skill passed `skill-creator/scripts/quick_validate.py`; its relative UI-guide link resolves. `git diff --check` passed.

Intermediate evidence: an initial mobile clearance assertion only scrolled an element into the viewport, which can leave it behind fixed navigation. It now checks reachability after scrolling to the page end, and all eight phone/theme cases pass. One intermediate review run timed out at sign-in while a build was being replaced; the clean final repeat passed. Two concurrent browser runs collided in their shared Playwright trace/output directory (ENOENT during context cleanup); affected suites were repeated sequentially with clean passes. A later owner steering turn ended an in-progress tool session; remaining suites were rerun with a persistent log at `work/mobile-verification.log`. These intermediate runs are not counted as suite passes.

The owner explicitly authorized computer-use layout inspection during this batch. Inspected actual rendered 360px light filters/transaction rows/split editor/save footer, 360px dark More navigation and 1440px dark desktop list, using a disposable synthetic service. The visual check exposed cramped scope labels and a header checkbox/text overlap; both were corrected and included in automated regression assertions. No general visual sign-off across every viewport/state is claimed. Real-device on-screen keyboards, physical safe areas and non-Chromium behavior remain unverified.

MCP impact: browser presentation and verification only. No backend/shared financial services, tool/input schemas, output allowlists, permissions, consent, proposals or audit changes. No backend tests/vet were rerun because backend code is unchanged. No production financial data, bank sessions or production deployment were used.

Development remains served by the existing `make dev` supervisor from `/home/douw/finance-tracker` at http://127.0.0.1:5173. Health returns 200/ok and Vite serves this branch's Review navigation. It uses the separate persistent development database and remains running for owner feedback.


Development preview recovery (7 October 2026): owner reported a black screen at 5173. Browser logged a missing MCPPermissionFields module export; Vite served an empty transformed MCPPermissions.tsx despite the current source containing that export. Restarting the existing make-dev supervisor cleared the stale module cache. Both WSL and Windows HTTP checks passed, and a fresh in-app browser tab rendered the sign-in page with no console errors. Persistent dev data was preserved; no application/MCP contract change was required. The new supervisor log is work/dev/mobile-dev.log. After pulling/switching branches while Vite is running, verify actual dev startup/export delivery and restart if its cached module graph is stale; production-build checks alone do not establish HMR preview health.

## Owner mobile feedback and category-list follow-up (7 October 2026)

Final production TypeScript/Vite build, 10 frontend unit tests, UI-skill validation and git diff --check passed. Final source passed 30 focused synthetic browser workflows: mobile-feedback (6), mobile-selection (5), mobile-core (15), mobile-edge-layout (4). These cover compact dashboard scope/insets, quiet search, status labels, exact amount visibility, logo centring, long-press initiation and click suppression, movement/cancel recovery, keyboard/menu alternatives, stable list positions, empty-selection exit, authorized personal seen action, category/group grid equality and category editing/scoped navigation. Existing mobile editor/navigation checks still pass across phone/landscape/breakpoints and desktop. Earlier dashboard-buckets (7) and global-search (6) passed during the same feedback batch before the subsequent selection refinements.

Intermediate failures were resolved or isolated: a theme rule overrode transaction padding and offset checkbox alignment before checkbox removal; corrected compact-row geometry passed. The first core selection run used a role locator that did not find the shared menu summary; switching to its explicit accessible label passed. Random local fixture ports encountered malformed readiness responses; immediate fixed-port reuse also encountered an occupied port. The final runs used separate reserved ports 18220/18230/18240/18250, sequentially, with clean passes. An earlier shell check was denied because automatic approval review hit its usage limit; later authorized calls resumed normally. These interrupted/failed runs are not counted as passes.

Owner-authorized computer-use coverage: actual 360px dark dashboard/compact status rows before long-press refinement; final 390px dark transaction selection, empty-selection exit and category list, and 390px light category list. Synthetic service only. Screenshots saved in the Windows workspace as mobile-selection.jpg and mobile-categories.jpg. No complete visual sign-off or physical-device gesture/keyboard/safe-area verification is claimed. Backend financial code is unchanged, so backend suites/vet were not repeated. MCP tools, schemas, output allowlists, permissions/consent and proposal/audit services are unchanged. The dev backend at 8081 returned ok and the Vite server at 5173 remains available for owner feedback.

## Classification rule cleanup before checkpoint (7 October 2026)

Before checkpoint 50e691f, the final rules/category presentation passed the production build, 10 frontend unit tests and 16 focused synthetic browser checks: rule-layout (6), rules (3), classification-defaults (3), merchant-global (2) and filtered ui-backlog (2). Mobile-selection (5) also passed after the shared status presentation changed. Checks cover circular uploaded logos, compact state icons, menu actions and existing rule/classification behavior. Earlier owner-authorized synthetic visual checks covered mobile automatic rules and merchant rules in light/dark; their screenshot is merchant-rules-cleanup.jpg in the Windows workspace. No backend/MCP contracts changed.

## Compact dashboard and transaction controls (7 October 2026)

Checkpoint 50e691f was committed before this batch, as requested. Final TypeScript/Vite production build, 10 frontend unit tests, project UI-skill validation and git diff --check passed. Final compact-controls (6) and review-defaults (5) checks passed in both themes at 360/430/1440px and 360/1440px respectively. During this batch mobile-core (15), transaction-filters (4) and dashboard-buckets (7) also passed before the last spacing/focus refinements; these 26 workflows were not repeated after those refinements. The final 11 checks cover collapsed scope, search, disclosure/filter persistence, clear, Escape focus return, review queue defaults/counts, page overflow and 44px dashboard controls. Geometry checks assert dashboard totals begin within 265px on phones/240px on desktop, collapsed filters occupy less than 90px and the first transaction begins within 420px on phones/360px on desktop.

Initial geometry checks exposed inherited page/header/toolbar margins and a 69px desktop select-all header. Compacting these resolved the checks without reducing 44px targets. Escape handling was moved to the complete filter region so it also works when focus remains on its trigger; the final workflow passes. A review regression initially looked for the obsolete Seen text pill; it now asserts the accessible Seen icon. Failed/intermediate runs are not counted as passing.

Automatic approval review rejected opening a synthetic browser preview because AGENTS.md currently prohibits computer-use inspection. No workaround or visual inspection was performed for this batch. Automated Chromium checks are permitted by that instruction. Real-device keyboard, safe areas and visual sign-off remain unverified. Windows HTTP checks returned 200 for the live Vite page and transformed TransactionFilters module; backend health returned ok. The persistent make-dev preview remains at 5173 for owner feedback. Temporary synthetic preview 18390 was stopped. No financial data, backend services or MCP tools/schemas/allowlists/permissions/consent/proposals/audits changed.

## Merge verification follow-up (8 October 2026)

First PR #54 CI run passed backend race tests, Go vet, frontend tests/build, About and account-discovery workflows, then failed legacy account-icons phone tests that still navigated to Accounts in the bottom bar. Existing broader browser tests were updated to use More for Accounts, accessible review/seen icons and the approved mobile keyboard/menu selection flow. Product source remains unchanged after owner acceptance. Focused affected workflows and full CI are required before merge; subsequent actual results are recorded below; full CI status is available on PR #54.

Final affected local browser checks passed: account-icons (4), cohesive-flow (10), core-improvements (6), pending-rules (4), seen (4), transaction-entrypoints (3), ui-backlog (4), ui-hierarchy (4), workflows (3): 42 total. The first workflows run failed an obsolete Description contains label assertion; the final rerun asserts the saved rule row and all three cases pass. No product source changes were made during merge verification.

Transaction-editor (6) also passed after its remaining Needs category badge-text assertion was changed to the accessible Needs review icon. Total affected local merge regression coverage is 48 workflows. Application code remains at the owner-accepted implementation; merge follow-ups change tests/docs only.

The next full CI run passed through the earlier browser specs and failed polish (6) because it expected the former 30/36px dashboard headings. Updated that assertion to the approved compact 24/28px heading sizes. Final polish (6) passed locally at 360/900/1440px in both themes, retaining whole-app geometry, contrast, keyboard, validation and empty-state assertions. Total affected local merge verification is 54 workflows. The previous successful full CI runs took about 16 minutes; full CI remains the merge gate.

Full CI subsequently passed the first 33 browser specs, including polish, then failed the Settings phone test because its separate navigation helper still treated Accounts as a bottom-bar page. Updated that helper to use More on phones. Settings (3) passed locally, including preserved management drafts and ordinary-user navigation. The remaining 12 specs starting with Settings are also being checked locally; final results and full CI are linked from PR #54. Product source and permissions remain unchanged.

## Issue #9 — sequential database migrations (2026-10-07)

Worktree: `/home/douw/sente-database-migrations`, branch `codex/database-migrations`, base b50c0de. All databases and injected failures are disposable synthetic fixtures.

- Focused check passed: `go test ./internal/app -run 'SequentialMigration|Migration|Default|IndependentSpendingGroups|MCPProposalResultSchemaUpgrade|MCPProposalLookupDatabaseFailure' -count=1 -timeout=8m` (208.677s). Existing malformed historical fixtures were corrected to lower all later version markers when removing older fields/tables; current-version startup intentionally no longer repairs a manually damaged schema.
- `go vet ./...` passed.
- `GO=/home/douw/finance-tracker/work/toolchain/go/bin/go python3 scripts/test_demo.py` passed: synthetic fixture creation, exclusive creation, exact splits, balanced transfers and authoritative budget totals; no bank/MCP connection data.
- Frozen Git schema snapshots at versions 9, 19 and 22 exercise source/edit/provenance preservation, split amounts, grants/private scope, historical reviews, sessions, budget limits, merchant IDs/rules and explicit MCP permissions. Existing workflow tests cover earlier shapes and acceptance/seen, branding, FNB/network, configuration and MCP-context upgrades.
- Runner tests cover registry ordering/gaps/duplicates, newer/invalid version refusal, fresh installs without defaults, current-version non-replay, ordered future migrations, failed-batch rollback/retry and integrity checks. Additional failure injections cover first-install rollback, failed version recording and rollback after the legacy merchant rebuild/category conversion; these are included in the full race run below.
- All eight new migration test groups passed with race detection: `go test -race ./internal/app -run SequentialMigration -count=1 -timeout=5m` (197.064s), including the additional first-install/version-record/merchant-rebuild failure injections.
- Full backend race suite passed: `go test -race ./... -timeout=30m`; app suite 1142.545s, all domain packages passed from cache, command/buildinfo/problem packages have no tests. The completed log and exit status are in ignored `work/migration-race.log` and `work/migration-race.status`.
- Final `git diff --check` passed.

MCP assessment: no transport, schema, output, permission, consent, proposal/audit or financial write-service changes. The bridge retains the former legacy consent conversion; current explicit permission JSON is preserved. No real financial data, production migration/deployment, frontend changes, browser checks or live banking checks are claimed. The schema baseline is 23; older binaries reject the upgraded database, so rollback uses a pre-upgrade backup rather than downgrading in place.

## Notification foundation — 8 October 2026

Independent existing sequential migration tests passed (101.887s). Ten notification test groups passed with race detection (47.744s); the later inbox cap/startup/shutdown maintenance group separately passed with race detection (3.114s). These synthetic tests cover concurrent retry uniqueness and immutable payload conflicts, recipient isolation, payload allowlists, read/dismiss/read-all behavior, page limits, grant revocation/hidden accounts/source moves/deletion, household/private budget scope, preference defaults/optimistic versions/consent/audit rollback, adapter/dependency failure atomicity and retry, triggering-write rollback, age/cap retention and non-resurrection, authentication/CSRF, soft user deletion, baseline-23 and legacy-22 upgrade failure/retry/preservation, restart/connection persistence, validation and canonical account scope. No frontend/visual/bank/production verification is claimed. Full final backend results are recorded below when complete.

Integration follow-up: the first full-suite attempt was stopped after identifying legacy workflow fixtures that rewound version history while retaining schema-24 tables. Their centralized synthetic fixture helper now removes post-baseline notification tables before simulating the old version; production migrations remain strict and once-only. The interrupted run is not a pass. The initial demo check lacked a worktree-local Go runtime; rerunning with GO=/home/douw/finance-tracker/work/toolchain/go/bin/go passed the synthetic setup/invariants/overwrite-refusal test (4.106s). Final verification includes the additional budget-type/source binding guard.

Final focused run: all 11 notification groups passed with race detection (110.150s), including the final budget-source guard and startup/shutdown cleanup. Final Go vet and git diff --check passed. Main was freshly fetched/pulled at the owner's request; both primary and notification checkouts contain f3f068c (migration PR #56 plus mobile PR #54). Final upgrade and full backend integration runs remain pending at this checkpoint.

Corrected legacy upgrade/default coverage passed with race detection (394.126s). The additional notification backup/restore check passed with race detection (12.330s), preserving inbox/read/preference/receipt state and revoking restored sessions. The disk-fixture full-suite attempt was stopped and replaced by a final full race run using isolated disposable TMPDIR under /dev/shm with GOTMPDIR=/tmp; SQLite configuration and application behavior are unchanged. That final run includes all backend packages and emits JSON progress. The merged migration PR #56 GitHub verify job passed (17m25s).

Final full backend race run passed: all four domain packages plus internal/app (811.390s; 203 top-level app groups, including the 11 notification groups compiled for that run). No failed groups or race reports. The additional backup/restore group was added during this run and independently compiled/passed afterward; it is not counted in that full-run total. All application code is the same final version across these checks. Disposable memory fixtures were removed after completion. Final Go vet/diff hygiene are repeated at publication. Frontend/browser/manual inspection and deployment were not performed because this change adds backend infrastructure only. PR #57 remains a draft for review; GitHub CI is separate from the local checks recorded here.

## Mobile budgets, Settings and regression coverage — 8 October 2026

Branch `codex/mobile-budgets-settings`, based on main 76af29f. Implementation/automated coverage addresses #50, #52 and #53. The owner accepted the presentation and authorized push/merge/closure on 8 October 2026 without requiring CI to pass; the previously recorded local verification remains authoritative. Source, financial behavior and API/MCP contracts are separate from the synthetic presentation overrides described below.

Passed on final product source:

- `cd web && npm test`: 10 unit tests.
- `cd web && npm run build`: production TypeScript/Vite build.
- `cd web && npm run test:mobile`: all 11 tagged cases, with the existing disposable-fixture runner. CI configuration runs this first with a five-minute bound, then excludes `@mobile-smoke` from its remaining browser tests. Local commands passed; no new remote CI run is claimed.
- `npm run test:e2e -- mobile-budget-settings.spec.ts mobile-core.spec.ts mobile-edge-layout.spec.ts dashboard-buckets.spec.ts budget-presentation.spec.ts settings.spec.ts configuration.spec.ts mcp.spec.ts about.spec.ts user-security.spec.ts password-visibility.spec.ts fnb-connection.spec.ts polish.spec.ts editor-layout.spec.ts workspace-controls.spec.ts onboarding.spec.ts --grep-invert @mobile-smoke`: all 101 cases passed sequentially on fresh per-spec synthetic databases.
- `npm run test:e2e -- mobile-budget-settings.spec.ts --grep 'empty.*builder'`: four additional final empty-builder/list cases passed. The new specification now contains 43 cases; its first 39 were covered by the smoke/remaining commands above, without double-counting earlier matrix runs. Total final distinct browser coverage is 116.
- `git diff --check`: passed. Existing development preview HTTP returned 200 at 5173; no persistent owner data/preferences were modified by testing.

New coverage uses 360x800, 390x844, 430x932, 640x360 landscape, 900x720 and 1440x900 in both themes. It asserts complete monetary text and non-overlapping label/value geometry, large positive/negative totals, long period/group/category/account/user/agent names, overspent/no-target/private-account and empty/populated states, independent category display across groups, income transfer wording, aligned support widths and absence of trailing group separators. Builder checks cover amount validation/live recovery, removal/add targets, nested draft retention, unobstructed save actions, empty groups and long budget lists.

Settings coverage checks every administrator section and ordinary-user section visibility at all three phone widths, keyboard Home/End and active-tab containment, 44px scrolling/menu/permission targets, long account/user menu bounds and Escape focus return. General/Configuration/MCP drafts and pending import previews survive section changes; Banking credentials clear. Configuration comparisons, replacement confirmation error/recovery, long endpoints/JSON previews, permission consent reset and reachable modal cancellation are covered. Delayed loading and request-wide errors use contained shared toasts. Presentation role/response overrides do not establish server authorization; existing real synthetic Settings/MCP/security workflows provide separate permission/consent regression evidence.

The new breakpoint checks cover 599/600/601, 759/760/761, 899/900/901 and 1024px; existing core checks cover 699/700/701, 760/761, 1024 and 1440px. Existing budget tests also save independent limits and verify group inheritance; MCP cases exercise OAuth, exact approvals, account scope, automatic approval and context consent; security/onboarding cases retain their validation/focus/session contracts. Browser failure diagnostics now include screenshots, traces and new viewport/theme/fixture attachments with element-specific geometry messages. No screenshot baselines were introduced.

Intermediate failures led to fixes for long permission-account text wrapping, clipped desktop overview totals and missing inline required-checkbox errors. Incorrect initial fixture syntax/proposal shape and a reused fixture port were corrected; those attempts are not counted as passes. The full historical browser suite and backend race/vet suites were not rerun: backend code is unchanged, though the fixture runner rebuilt the backend successfully. No production data, live banking/client interaction, deployment, manual/computer-use visual inspection, physical-device keyboard/safe-area or non-Chromium behavior was verified.

MCP assessment: browser-only layout/validation and test/CI changes. Tool/input schemas, output allowlists, permissions/consent, exact proposal previews/audits and authorized financial write services are unchanged. Existing MCP JSON is displayed within bounded regions; no field or grant was added. No MCP.md contract update is required.

## Notification centre/preferences — issues #31–#32 (8 October 2026)

Final production frontend build and 10 existing frontend unit tests passed. Final `python3 scripts/test-browser.py notifications.spec.ts about.spec.ts` passed 8 workflows using two isolated synthetic fixture sets. The 3 notification workflows cover actual recipient/privacy-filtered inbox/count, read/dismiss/read-all, non-dismissible records, paging, authorized transaction/account/budget links, transaction focus return, empty/read/unread states, persisted individual preferences and mounted Settings drafts. Outage/unavailable account target/stale-save responses are simulated at the API boundary; ordinary reads/writes use the backend. The initial run passed 2 of 3 notification cases and caught stale reload retaining the draft; the corrected reload generation passed all 3 in the final run.

Automated geometry covers 360/390/430/760/761/1440px, both themes and a short 430px-height landscape case: no page-wide overflow and >=44px bell targets. All 5 existing About/navigation/retry cases passed. No manual visual sign-off or physical-device keyboard/safe-area verification.

`go test -race ./internal/app -run TestNotification -count=1 -timeout=5m` passed all 12 existing notification groups in 56.232s, with disposable fixtures under `/dev/shm` and Go temporary binaries under `/tmp`. Covers ownership/privacy, concurrent retries, preference audit rollback, state APIs, retention/caps/lifecycle, migration and backup/restore. Backend unchanged; full backend tests were not repeated for this UI batch. Diff hygiene passed. Dedicated notification fixture seeding is enabled only for its browser spec; no notification creation endpoint is exposed.

MCP impact: notification messages and preferences remain browser-only personal workflows; existing connections have no notification consent. Tools, schemas, allowlists, permissions, proposal previews/audits and financial write services are unchanged. No production data, deployment or computer-use inspection. #33–#38 producers/suppression, #39 push/PWA and #40 diagnostics remain.

Publication: automatic approval review rejected push/PR creation pending owner authorization to export source to GitHub. Local work is preserved; no push, PR or issue closure occurred in this batch.

## Combined notification preference save (8 October 2026)

Owner-requested single Save changes button replaces per-type saves. The browser submits only changed preferences to the new batch endpoint; all writes/version checks/audits and the response snapshot share one transaction. Existing single-item requests use the same service. Tests verify a stale second item and failed second audit roll back preceding changes, successful changes affect only the requester, and duplicates/empty batches reject.

Final production build passed. All 3 synthetic notification browser workflows passed with isolated output `/tmp/sente-single-save-browser-85e59b6`; coverage now saves two changed types together and confirms one button, persistence, preserved drafts and stale-save reload. The first browser attempt reported a missing trace file in the shared artifact directory; isolated output resolved that collision. `go test -race ./internal/app -run TestNotificationPreference -count=1 -timeout=5m` passed both preference groups (15.427s), and Go vet for internal/app passed. No full suite or manual visual check repeated. MCP remains browser-only with unchanged tools/consent/allowlists. No financial detection, thresholds, schema or production data change; publication remains pending owner authorization.
