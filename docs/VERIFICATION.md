# Verification

## Current audit outcome — 7 October 2026

All nine repository/UI audit findings were repaired in source:
1. MCP budget approvals freeze canonical group IDs and exact before/after effects; stale or unresolved proposals require fresh preparation.
2. Rebalance SQL columns are qualified and existing spending/version checks remain.
3. Budget writes preserve explicit scopes, recurrence, empty groups and legacy aggregate-only limits.
4. Categories stay flat; transaction, rule and budget group/category fields are independent.
5. Categories and search share the validated spending-group editor.
6. Search uses accessible keyboard, loading/error/retry, focus restoration and mobile controls.
7. Ruleset imports use atomic authorized services, versioning and audit.
8. Ruleset export/import preserves exact priorities, directions and explicit account mapping with bounded validation.
9. Browser regressions and current documentation reflect the implemented behavior.

Completed implementation plans and superseded visual audits were consolidated into the current guides on 7 October 2026. Historical visual checks used synthetic data; they do not constitute a current visual sign-off.

## Outstanding external checks

- Live FNB compatibility, MFA, bank layout changes and complete historical coverage require owner-run checks. Synthetic tests cannot establish live reliability.
- Transaction identifier stability across separate overlapping OFX exports needs another owner-supplied sample. Equivalent CSV/OFX content alone does not establish stable IDs; candidate matching remains available.
- External MCP clients require their own connection/consent checks; source tests do not prove every client works.

## 2026-10-07 audit repair verification

Authorized scope: the nine repository/UI audit findings summarized below; flat categories and independent MCP groups/categories are preserved.

Completed on isolated synthetic data:
- Production web build (TypeScript/Vite): passed after final shared modal focus/restore changes.
- Frontend unit suite: 8 tests across 4 files passed.
- Final one-server Playwright run: **25 passed** (3.3 minutes). Search/group editor keyboard navigation, active-descendant, Clear with Enter, first-blur/live trim validation, save, category editing without ownership controls, close/focus restore, mobile 44px trigger and no horizontal overflow at 1440/360px in light/dark; announced loading/failure/retry; exact grouped MCP approval after-state; compact transaction split edit/mismatch workflows; Income/merchant permission layout; whole-app populated/empty geometry and contrast at 1440/900/360px in both themes; review defaults and seen/pending filter reset; workspace hierarchy.
- Focused Go financial/MCP run: 7 passed (119.505 seconds), covering rebalance spent/version/total rechecks, independent atomic group/category budgets, merchant consent/scope, exact name-to-ID budget approvals, category metadata changes, guarded replay, stale group state, old unresolved proposal denial, explicit empty groups/stopped recurrence, flat expense/income categories and omitted vs explicit ungrouped scopes.
- Strict ruleset run: 4 tests passed (92.598 seconds excluding the empty-budget test in the combined run), covering local logo import, complete scoped/custom/built-in round trip, zero/negative priorities, direction separation, idempotency, versions/audits, revoked account access and scoped exports, invalid JSON/version/unknown fields/references/logos/duplicate entries, bounds, rollback and archive/restore protections. Final follow-up of all 4 ruleset tests also passed (82.497 seconds), adding accurate restore summaries and disabled-operator import/export denial.
- Synthetic offline CLI and both Python wrappers: passed explicit database/operator requirements, refusal to overwrite the database, owner-only atomic export, and full seeded configuration round trip in a disposable temporary database.
- Shared-control/single-writer structural checks: passed. Search contains no independent raw dialog/form; group editor uses shared Modal/Form/Field/Button; approval displays exact grouped after-state; Python scripts contain no SQL implementation.

The original/intermediate browser failures were used to repair superseded dashboard selectors and shared modal opening/restoration behavior; the final run above is green. The intermediate broad Go run was superseded while repairs were still being completed. The broad backend run completed in 1,333.181 seconds (go test ./... -count=1 -timeout=30m -v): 164 top-level tests passed, TestSuppliedFNBPair deliberately skipped because no external banking sample was supplied, and TestMetadataPagesSearchSnapshotsAndPartialLimits failed. The failing preservation invariant was repaired in source: orphaned legacy aggregate limits are promoted to No spending group within the transaction before totals rebuild, with audit and exact MCP before/after coverage.

The final targeted backend run then passed all 7 affected checks (45.653 seconds): TestCoreArchivedBudgetLegacyWriteCannotRestartCarry, TestGroupCategoryBudgetsIndependentAndAtomic, TestMCPRulesCategoriesBudgetsAndRevocation, TestMCPBudgetFreezesExactGroupScope (now also preserving legacy limits), TestBudgetExplicitEmptyGroupsAndRecurrence, TestFlatCategoryIndependentBudgetGroups and TestMetadataPagesSearchSnapshotsAndPartialLimits. Final vet and diff checks passed. The entire 22-minute backend suite was not repeated after this last correction; unaffected broad-run coverage and post-fix affected coverage are distinct, and there is no remaining known failing regression.

No production deployment, commit, real financial database/credential inspection, live banking, manual/computer-use visual inspection or exhaustive browser/device/security coverage. Browser coverage is automated structural/geometry/workflow verification, not a visual sign-off. Generated fixtures/logs remain outside committed application data.


## Issues #1 and #2 module refactor — 7 October 2026

Branch: `codex/domain-feature-refactor`. This is a structural refactor of backend domains and frontend feature ownership, plus future-change guidance in AGENTS.md and REFACTORING.md. Deployment, routes, schemas, JSON contracts, financial behavior and MCP consent/capabilities remain unchanged.

Completed with synthetic fixtures:
- **168 distinct top-level backend tests passed with race detection; one expected external-sample skip** (`TestSuppliedFNBPair`). The `go test -race ./... -timeout=30m -json` invocation reached its process time limit during its final test, after 167 passes and the expected skip, without assertion failures or race reports. Enumeration against the final test list found only `TestEmptyLimitPagesCreateExpenseWithoutSavingLimits` unfinished; its separate race-enabled run passed in 9.211 seconds. This covers every enumerated backend test across the two runs; the complete suite did not finish in one invocation.
- Final shared masked-credit predicate checks passed: domain race suites, `TestFNBRefreshExactBalancesPrivateCreationHideAndRestore` (20.707 seconds) and `TestFNBOwnerSelectedMaskedCreditDiscoveryStagingAndVisibility` (16.567 seconds). All final packages/test files compiled (`go test ./... -run '^$'`); final vet passed.
- Structural comparison preserved **309 original production function bodies** after accounting for moved package names. Thin model/error/parser facades, classification result adaptation and allocation category lookup were reviewed separately. Moved models retain their JSON fields/tags; the browser and MCP still use the same serialized authorized write services.
- Frontend production TypeScript/Vite build passed; all **8 unit tests across 4 files passed**. Runtime dependency manifests/lockfiles and Go module files did not change. The pinned formatter was an execution-only tool, not an application dependency.
- **55 distinct synthetic browser workflows passed across the main run and focused reruns.** Coverage includes startup/setup, persistent login/session-expiry feedback, theme synchronization, account/import screens, typed categories and atomic rules, clean/exception import handling, review filters/seen state, settings drafts/non-admin access, nested current-record editing and focus/draft return, grouped MCP effects, search/loading/retry/keyboard behavior and desktop/mobile geometry in light/dark.

The initial 140-test browser attempt was stopped after 15 passes, six failures and one interrupted test exposed historical selector/port assumptions. The subsequent 55-workflow run had 43 passes and 12 failures from superseded menu/tab expectations, shared-fixture assumptions and an async geometry race. Test repairs use the existing native menu labels, transaction dialog identity, current MCP tab, API acceptance state, configurable empty-installation port and atomic/polled layout measurement. The four empty-state/menu workflows then passed (5.5 seconds); all 11 review/settings/current-record workflows passed on a fresh isolated service (21.0 seconds). Those reruns include all previously failing selected workflows and three already-passing controls, yielding 55 distinct successful checks. The entire 140-test historical browser suite was not rerun.

Final whitespace and dependency-boundary checks passed. Feature modules do not import App; domain packages do not import app or HTTP/MCP transports. Settings drafts remain mounted, access drafts stay parent-owned through their hook, and transaction financial drafts and request locks retain their existing owner.

MCP impact: shared classification, exact-money parsing and allocation validation now live in domain packages reached by existing authorized services. No tool/input schema, output allowlist, permission, proposal preview/audit or consent expansion is needed. No financial migration, production-data/credential inspection, deployment, live banking or manual/computer-use visual sign-off. Synthetic browser checks establish automated workflow/DOM coverage only.

## Sente README, branding and synthetic demo — 2026-10-07

Separate `codex/sente-readme-demo` worktree from main; refactoring worktree and owner databases were not edited. Frontend: all 8 unit tests and production TypeScript/Vite build passed. Demo regression: fresh migrated database, exact allocation totals, zero net linked transfers, category review fixture, aggregate/group budget equality, foreign keys/integrity and refused second creation with unchanged database hash passed. Focused backend MCP public setup/privacy and repeated-resource OAuth tests passed (3 tests). Docker Compose configuration validation passed. No Docker image build/push, production deployment, real FNB connection or manual visual sign-off was performed. Both existing handover browser workflows passed against the isolated synthetic server: Sente and private workspace titles, stale naming edits, persistence, desktop/mobile light/dark overflow and budget draft preservation (2 tests). The full backend run timed out after 10 minutes in SQLite fsync during fixture migration; full-suite success is unverified. The focused MCP/OAuth checks passed independently. Default browser startup was replaced with a temporary single-server configuration using the already healthy synthetic service to avoid redundant database initialization.


Integration after PR #42: retained the feature module layout and applied Sente branding in App, auth and workspace components. Preserved both handover/verification histories. Production frontend build, 8 unit tests, demo integrity/financial/overwrite regression, 3 focused MCP/OAuth tests, Go vet and 2 synthetic handover browser workflows passed on the combined branch. Documentation links and whitespace checks passed. Docker is unavailable in this WSL distro, so Compose validation was not repeated; the earlier branch result remains recorded above. Docker Hub repository secrets are not configured, so automatic image publishing requires setup. No production deployment or live banking check.


## Issues #15–#17 user security — 2026-10-07

Branch/worktree: `codex/user-security`, `/home/douw/sente-user-security`, based on main. Schema 20 is an additive nullable deletion marker. Synthetic migration checks preserve users/sessions; deletion preserves ledger/source/rule/audit/review foreign keys and reserves historical usernames while removing active access and credentials. Current-session retention for self-service is explicit; administrator resets revoke every target session. Every password path revokes MCP connections and user-bound pending OAuth consent, with no MCP tool/schema/allowlist/capability expansion. Browser-only account administration uses shared validation/dialog/toast controls.

Passed: 18 distinct focused backend tests with race detection across the main security/OAuth/authentication/setup run (16 tests) and migration/disabled-reset follow-up (2 tests); all four financial domain packages; `go vet ./...`; final frontend production build; 8 frontend unit tests; 7 synthetic Playwright workflows (4 new security workflows at 1440/360px plus 3 settings regressions); `git diff --check`. Security coverage includes current-password checks, byte bounds, administrator permissions, CSRF, stale versions/concurrent resets, session/agent/OAuth revocation, retained disabled status, last-administrator/self-deletion protection, history/foreign-key preservation and atomic rollback when audit fails.

The initial migration fixture incorrectly removed its schema marker and was corrected to represent version 19 before passing. The initial standard browser runner stalled at localhost readiness despite a healthy synthetic server; it was stopped. The successful run used a temporary one-server config and local proxy bypass on port 18580; the temporary config and test servers were removed/stopped. Tests used disposable synthetic databases only. Full historical backend/browser suites were not repeated. No manual/computer-use visual inspection, real financial data, live banking or production deployment.


Password visibility/user-menu follow-up (2026-10-07): final frontend build and 8 unit tests passed. Nine synthetic browser workflows passed across the main/follow-up runs: password toggles and menu editing at 1440/360px, four existing security workflows, and three onboarding/budget workflows. Coverage checks all ten password inputs across setup, sign-in, self-service, add/reset users and FNB credentials; value preservation, re-masking on clear, keyboard toggling, labelled input linkage, 44px targets, transparent hover background/border, validation and menu focus restoration. Initial onboarding failures came from a partial selector matching the new eye button; exact field selectors and keyboard expectations were corrected, and all three onboarding workflows then passed on a fresh disposable server. Temporary runner configs/test servers were removed/stopped; the worktree dev server on 5175 remains running. No manual visual inspection, banking interaction or backend/MCP contract changes.
