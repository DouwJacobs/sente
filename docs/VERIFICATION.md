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

## Sente README, branding and synthetic demo — 2026-10-07

Separate `codex/sente-readme-demo` worktree from main; refactoring worktree and owner databases were not edited. Frontend: all 8 unit tests and production TypeScript/Vite build passed. Demo regression: fresh migrated database, exact allocation totals, zero net linked transfers, category review fixture, aggregate/group budget equality, foreign keys/integrity and refused second creation with unchanged database hash passed. Focused backend MCP public setup/privacy and repeated-resource OAuth tests passed (3 tests). Docker Compose configuration validation passed. No Docker image build/push, production deployment, real FNB connection or manual visual sign-off was performed. Both existing handover browser workflows passed against the isolated synthetic server: Sente and private workspace titles, stale naming edits, persistence, desktop/mobile light/dark overflow and budget draft preservation (2 tests). The full backend run timed out after 10 minutes in SQLite fsync during fixture migration; full-suite success is unverified. The focused MCP/OAuth checks passed independently. Default browser startup was replaced with a temporary single-server configuration using the already healthy synthetic service to avoid redundant database initialization.
