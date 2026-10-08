# Test review — issue #55

Review and consolidation, 8 October 2026. The implementation below removes repeated presentation combinations and unnecessary assertions while retaining core financial/security tests. Initial findings later in this document record the review evidence.

## Implemented consolidation

The final CI policy is:
- Every PR: all backend race tests/vet, worker/demo invariants and gate/runner regressions.
- Relevant frontend/connector/build/test/CI changes: all frontend unit tests/build, bank DOM and a 23-spec core browser gate **plus every added/edited browser spec**. The browser step has a ten-minute bound.
- Main/manual runs: the complete browser suite once, with a twenty-minute bound. Neither path duplicates mobile-smoke cases in a second fixture run.
- Backend/docs-only PRs skip frontend checks. If scope detection is uncertain, every browser spec joins the PR selection. Changed-spec input is a validated JSON array of root filenames; deletions are ignored and duplicates run once.

### Removal and retained-coverage map

| Change | Before → after cases | Why safe / retained owner |
| --- | ---: | --- |
| mobile-budget-settings | 43 → 20 | Eighteen repeated long interaction combinations removed; same draft/validation/consent/credential clearing interactions run on phone, short landscape and desktop. Ten boundary cases, loading/errors and two empty-builder cases remain. |
| New responsive-geometry | 0 → 2 | Each theme resizes through all six original widths, retaining long Settings labels/panels, terminal-tab keyboard visibility, huge signed money, no clipped figures, label/value separation and reachable save. |
| Ordinary Settings role presentation | 3 mobile copies removed; Settings 3 → 4 total | Phone and desktop role checks have one owner. Explicit ordinary sections, forbidden admin sections, absence of diagnostics/management requests and overflow remain. A mocked role never replaces server denial tests. |
| About | 5 → 3 | Both navigation layouts, both themes across those layouts, link/privacy wiring and failed-metadata retry remain; cheaper unit tests still own the exact public report allowlist. |
| Rule layout | 6 → 2 | Paused/global/active indicators, menus and Escape focus remain on phone/desktop. Duplicate circular-avatar check stays in merchant-global; arbitrary <=16px status-size check removed. |
| Account icons | 4 → 2 | All five accessible account-type meanings and long nicknames remain on phone/desktop; icon semantics do not change with palette. |
| Workspace controls | 6 → 2 | Collapsed/expanded filters, active scope, clear/search behavior, Escape focus, touch height and no overflow remain. Header-dependent top coordinates and arbitrary height ceilings removed. |
| UI hierarchy | 4 → 2 | Hidden drafts, unsaved-change guard, disclosed validation errors, reports/actions and restored focus remain. Arbitrary panel top-coordinate removed. |
| Mobile feedback | 6 → 3 | All three original widths, both themes across them, amount/merchant/selection alignment, private/transfer labels, gesture state and no layout shift remain. Logo must fit rather than equal 36px. |
| Mobile selection | 5 → 3 | Dedicated movement/cancellation/post-hold-click/keyboard case unchanged. Repeated category/group parity workflows keep phone/desktop representatives. |
| Polish | 6 → 3 | Original phone/tablet/desktop widths and both themes across them; contrast, alignment, reduced motion, skip link, roving focus, invalid-field focus and empty states remain. Exact page RGB, heading 24/28px and 14px panel radius removed. |
| Theme states | 4 → 2 | Shared hover/error/selected/disabled/focus contracts tested in both themes and layouts. Literal RGB becomes token consistency plus >=4.5 text contrast; visible keyboard focus replaces fixed 2px outline. Asynchronous settling remains. |
| Budget/income presentation | Same cases | Correct money, progress metadata, account scope and permissions remain. Duplicate hex/RGB literals become shared-token consistency. |
| MCP | 10 → 6 | Phone/desktop connection/context workflows retained with both themes across layouts; standalone OAuth revocation and grouped-budget proposal remain. |
| Seen and pending-rule UI | 4 → 2 each | Same import/edit/seen and account-rule preservation assertions on both navigation layouts; financial invariants stay in backend coverage. |
| Transaction filters | 4 → 2 | Same classification/filter behavior in each navigation layout; CSS geometry has separate coverage. |
| Transaction editor | 6 → 4 | Phone/desktop save/picker workflow retained; both visible and hidden linked-transfer restrictions remain separate. |
| Global search | 6 → 4 | Phone/desktop workflow retained, plus independent failed-load/retry and MCP grouped-budget approval cases. |
| Mobile core | 15 → 13 | Long gesture/split-save workflows remain at narrow portrait and short landscape in both themes. 390/430px now use short editor-fit cases; all existing breakpoint cases remain. |
| Success screenshots | 6 → 0 | No assertions consumed these images. Configured failure screenshots, traces and fixture context remain; shared output-path collisions disappear. |
| Fixture startup | Mostly 3 → 1 services/spec | Reviewed specs only use the primary port. Onboarding retains offsets 0/1 and fresh classification offsets 0/2; PWA retains a private static copy. Unlisted specs conservatively retain all three services. |

The reviewed suite has **200 expanded cases in 50 spec files**, a net reduction of 60 cases across the reviewed groups. The current shared checkout discovers **201 cases in 51 files** because concurrent issue #69 work added a separate budget-alert preference workflow, which this review did not consolidate. The additions include two theme geometry cases and two short editor-width cases. No entire unique financial/security workflow is deleted. Losses are explicit: full interactions no longer run at every palette/width combination, arbitrary exact styling is no longer frozen, and non-core browser groups may first run on main/manual unless their spec is edited. These are the chosen cost/coverage tradeoffs. Main still executes all retained workflows, and responsive geometry stays in the PR core.

### Measured consolidation result

Seven original overlapping groups: **73 cases passed**, 238.215s elapsed. At the consolidation checkpoint, seven groups plus the replacement geometry group: **39 cases passed**, 132.001s elapsed. Summed Playwright process time is **186.101s → 109.958s (40.9% less)**, including all replacement geometry. Build time was 27.457s → 0.649s; do not attribute that warm-cache difference to fewer tests.

Local outputs: web/test-results/run-m00zgufv and run-2_576g7e. Shared-checkout application edits occurred during this work and were preserved; these single local observations are indicative, not a controlled benchmark or an after-change GitHub CI duration. No whole-CI time saving is promised. GitHub after-change measurement remains pending publication.

An initial consolidated run exposed duplicate role-test titles; another exposed an immediate hover read before its CSS transition settled. Both harness issues were corrected and the 39-case comparison passed. Failed attempts are not counted as passes.

### Final validation and expanded inventory

The PR profile plus all then-edited specs passed **154 cases across 35 files** in 540.096s locally (run-ucjzqjil). This measured run informed the ten-minute PR browser bound. Subsequent consolidation removed 16 additional repeated cases. A focused run passed 35 retained cases across MCP, seen, pending rules, filters, search, editor and mobile core (run-pq4mkmfh); the final MCP shape passed all six cases (run-hg8g5qxk), and the final Settings-discovery assertion passed all three representative layouts (run-kncnl6bo). The four-case Settings group also passed separately (run-4djyd9yt). All retained scenarios changed by this consolidation have passing browser evidence across these runs; the exact final PR command and complete 51-file suite have not been rerun.

Frontend production build and all 15 frontend unit tests passed. The four domain packages passed with race detection. CI scope (seven) and runner (six) regression checks passed; worker (46), demo and bank DOM invariants passed during the initial slice. These are this review's verification results, not a claim that this agent ran the full backend suite against every concurrent application edit. No commit, push, PR, merge, issue closure or deployment occurred.

Expanded counts below come from Playwright discovery after consolidation. PR core files run for relevant changes; other files run on main/manual or when that spec is added/edited. Runtime values are single local process observations from the PR validation or a later focused run; some observations precede the last case reduction, so they are not exact forecasts for this inventory. An unmeasured file remains retained, not judged unnecessary.

| Spec | Expanded cases | PR selection | Observed test process seconds |
| --- | ---: | --- | ---: |
| about.spec.ts | 3 | When edited | 3.167 |
| account-discovery.spec.ts | 1 | When edited | Unmeasured |
| account-icons.spec.ts | 2 | When edited | 3.614 |
| audit-followup.spec.ts | 4 | When edited | Unmeasured |
| budget-alert-preferences.spec.ts | 1 | When edited | Unmeasured |
| budget-presentation.spec.ts | 2 | When edited | 3.657 |
| classification-defaults.spec.ts | 3 | Core | 6.997 |
| cohesive-flow.spec.ts | 10 | When edited | Unmeasured |
| configuration.spec.ts | 2 | Core | 4.979 |
| core-improvements.spec.ts | 6 | Core | 17.425 |
| dashboard-buckets.spec.ts | 7 | When edited | Unmeasured |
| dashboard-connections.spec.ts | 4 | When edited | Unmeasured |
| editor-layout.spec.ts | 2 | When edited | Unmeasured |
| fnb-connection.spec.ts | 2 | When edited | Unmeasured |
| fnb-transactions.spec.ts | 7 | When edited | Unmeasured |
| global-search.spec.ts | 4 | Core | 7.49 |
| handover.spec.ts | 2 | When edited | Unmeasured |
| income-permissions.spec.ts | 2 | When edited | 4.513 |
| login-toast.spec.ts | 8 | When edited | Unmeasured |
| mcp.spec.ts | 6 | Core | 21.66 |
| merchant-global.spec.ts | 2 | When edited | Unmeasured |
| mobile-budget-settings.spec.ts | 20 | Core | 39.229 |
| mobile-core.spec.ts | 13 | Core | 18.934 |
| mobile-edge-layout.spec.ts | 4 | Core | 4.352 |
| mobile-feedback.spec.ts | 3 | When edited | 6.894 |
| mobile-selection.spec.ts | 3 | When edited | 6.476 |
| network.spec.ts | 2 | When edited | Unmeasured |
| notification-delivery.spec.ts | 5 | Core | 7.73 |
| notifications.spec.ts | 3 | Core | 5.915 |
| onboarding.spec.ts | 3 | Core | 4.754 |
| password-visibility.spec.ts | 2 | When edited | Unmeasured |
| pending-rules.spec.ts | 2 | Core | 6.454 |
| polish.spec.ts | 3 | When edited | 9.335 |
| pwa.spec.ts | 10 | Core | 23.824 |
| responsive-geometry.spec.ts | 2 | Core | 33.64 |
| review-defaults.spec.ts | 5 | When edited | Unmeasured |
| rule-layout.spec.ts | 2 | When edited | 2.827 |
| rules.spec.ts | 3 | Core | 29.987 |
| seen.spec.ts | 2 | Core | 7.323 |
| settings.spec.ts | 4 | Core | 6.133 |
| theme-states.spec.ts | 2 | When edited | 7.763 |
| theme-toggle.spec.ts | 2 | Core | 3.966 |
| toasts.spec.ts | 2 | Core | 4.997 |
| transaction-editor.spec.ts | 4 | Core | 34.267 |
| transaction-entrypoints.spec.ts | 3 | Core | 14.124 |
| transaction-filters.spec.ts | 2 | Core | 7.941 |
| ui-backlog.spec.ts | 4 | When edited | Unmeasured |
| ui-hierarchy.spec.ts | 2 | When edited | 6.79 |
| user-security.spec.ts | 4 | Core | 12.253 |
| workflows.spec.ts | 3 | When edited | 23.601 |
| workspace-controls.spec.ts | 2 | When edited | 3.634 |

## Initial first slice (superseded by the consolidation above)

The owner chose backend/security coverage on every PR and frontend checks when frontend inputs change. All existing backend race tests stay in the gate with an explicit ten-minute **per-package** timeout, plus vet, all 46 active worker tests and the synthetic demo invariant check. Frontend unit/build, bank DOM, mobile smoke and the remaining complete browser suite run for frontend/connector/test/build/CI inputs and unknown paths. Backend-only (internal/, cmd/, Go dependencies), documentation and demo-only PRs skip frontend checks. Main pushes and manual runs always run everything.

Browser wiring regressions in backend-only PRs are left to the full main/manual run; backend HTTP/authorization/service tests remain required before merge. Run the workflow manually on the PR branch for backend changes that need browser integration evidence. This is a coverage-frequency tradeoff, not proof that Go changes cannot affect the browser. No scheduled coverage has been added.

Scope detection uses the PR merge-base diff, both paths for renames/deletions, and full checkout history. Unknown inputs or failed detection require the frontend gate. The existing verify job/check name is retained.

## Measured evidence

[Successful CI baseline](https://github.com/DouwJacobs/sente/actions/runs/37770086214/job/113287247108), 8 October: job 22m34s; backend race 3m35s; vet 42s; frontend unit/build 12s; Chromium installation 33s; mobile smoke 58s; remaining browser workflows 15m50s. Environment/cache/load differences matter; these are observations from one run.

Local About group, same five cases and existing built frontend:
- Before runner changes: 14.50s total; Playwright 4.9s.
- With timing/output isolation only: 14.175s; build 0.490s, fixture setup 3.171s, test process 5.488s, shutdown 5.025s.
- After fixture signal-handler repair: 9.493s total; all five cases passed.
- Concurrent identical runs: 11.265s and 11.591s, all ten cases passed; shutdown 0.228s/0.278s. Different persistent result directories retained both JSON reports and all server logs.

The fixture signal handler used to re-enter Popen.wait while the main thread held its waitpid lock, hitting the runner's five-second forced-stop deadline. It now forwards termination only and cleans up in the main flow. Production shutdown code is unchanged. Multiplying that observed saving across specs is a hypothesis until a full measured run confirms it.

Worker 46 tests: 0.098s Node-reported duration. Demo invariant: 1.936s unittest duration. Six CI scope regression cases: 0.011s. Bank DOM passed; its runtime was not separately captured.

After-change GitHub duration is **pending publication and a CI run**. No measured whole-suite saving is claimed. Per-group backend/browser runtime inventory is incomplete; do not remove coverage on the basis of missing timings.

## Initial group inventory and recommendations

Counts below are source-level test declarations, not expanded parameterized cases/subtests; dynamic loops increase executed counts. Intent samples come from named tests and support the initial recommendation, not a completed assertion-by-assertion equivalence audit. Pending means per-file elapsed time is not measured yet.

Keep distinct exact-money, splits/refunds/transfers, grants/private-account access, consent/replay, stale versions, atomicity/audits, import multiplicity/idempotency, seen/review, migrations/restore and connector cleanup regressions. Shared fixtures and layering can be reviewed without removing these boundaries.

### Backend — 54 files

| Group | Declarations | Distinct failure/intent samples | Runtime | Initial recommendation |
| --- | ---: | --- | --- | --- | --- |
| [internal/app/acceptance_test.go](../internal/app/acceptance_test.go) | 3 | TestAutomaticAcceptanceAndPersonalSeen; TestSeenBatchAtomicityAndFilters | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/account_types_test.go](../internal/app/account_types_test.go) | 1 | TestAccountsExposeImportTypeWithinAccountPermissions | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/app_test.go](../internal/app/app_test.go) | 18 | TestFNBAdapters; TestZipSafety | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/branding_test.go](../internal/app/branding_test.go) | 2 | TestBrandingPermissionsValidationAndConcurrency; TestBrandingMigrationAndRestart | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/budget_builder_test.go](../internal/app/budget_builder_test.go) | 2 | TestBudgetBuilderUpcomingAndOneTime; TestBudgetBuilderSparseReadsAndZeroEntry | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/build_info_test.go](../internal/app/build_info_test.go) | 1 | TestBuildInfoAuthorizationAndSafeOutput | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/configuration_test.go](../internal/app/configuration_test.go) | 7 | TestConfigurationFreshInstallAndUpgrade; TestConfigurationPreviewAtomicityPermissionsAndExport | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/core_workflows_test.go](../internal/app/core_workflows_test.go) | 9 | TestCoreBulkAtomicSplitPreservationAndPrivacy; TestCoreNavigationOriginalAnchorAndProcessed | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/dashboard_groups_test.go](../internal/app/dashboard_groups_test.go) | 3 | TestDashboardBucketLineage; TestDashboardBucketPagination | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/defaults_test.go](../internal/app/defaults_test.go) | 4 | TestDefaultsSeedOncePreserveHistoryAndCustomOverride; TestDefaultMigrationPreservesConflictingKindsAndNewAccounts | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/fnb_cancellation_test.go](../internal/app/fnb_cancellation_test.go) | 1 | TestFNBCancellationCleansSyntheticProcessTreeAndProfile | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/fnb_connection_test.go](../internal/app/fnb_connection_test.go) | 11 | TestFNBSecretsPermissionsAndSchedule; TestFNBRefreshExactBalancesPrivateCreationHideAndRestore | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/fnb_fixture_test.go](../internal/app/fnb_fixture_test.go) | 0 | Synthetic process/fixture helpers used by other groups. | Pending | Keep shared synthetic fixture support; not an independent test group. |
| [internal/app/fnb_interop_test.go](../internal/app/fnb_interop_test.go) | 2 | TestFNBWindowsInteropRecoversStaleInheritedSocket; TestFNBLocalNodeDoesNotChangeEnvironment | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/fnb_transactions_test.go](../internal/app/fnb_transactions_test.go) | 8 | TestFNBLiveStagingReuseCommitProvenanceAndReview; TestFNBLiveRejectsAtomicMismatchHiddenRevocationAndInvalidMoney | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/group_targets_test.go](../internal/app/group_targets_test.go) | 2 | TestGroupCategoryBudgetsIndependentAndAtomic; TestGroupBudgetMigrationPreservesLegacyLimits | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/import_activity_test.go](../internal/app/import_activity_test.go) | 1 | TestImportActivityGroupsAndScopedLedgerPermissions | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/import_live_test.go](../internal/app/import_live_test.go) | 7 | TestFNBAutomaticOverlappingPagesPreserveMultiplicityFeesAndClassification; TestFNBAutomaticIdenticalRowsWithoutBalanceCountOccurrences | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_aggregates_test.go](../internal/app/mcp_aggregates_test.go) | 3 | TestMCPFinancialSummarySplitsRefundsTransfersAndFilters; TestMCPFinancialSummaryPermissionsPeriodsAndPaging | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_bulk_auto_test.go](../internal/app/mcp_bulk_auto_test.go) | 3 | TestMCPBatchProposalApprovalAtomicIsolation; TestMCPAutomaticApprovalTypesMixedEffectsAndRevocation | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_categorization_test.go](../internal/app/mcp_categorization_test.go) | 6 | TestMCPRuleImpactUsesPrecedenceAndAuthorizedCounts; TestMCPNarrowCategoryAssignmentsPreserveSplitsAndReplay | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_context_test.go](../internal/app/mcp_context_test.go) | 4 | TestMCPContextOwnershipConsentAndInitialization; TestMCPContextValidationStalenessDeletionAndAudit | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_core_coverage_test.go](../internal/app/mcp_core_coverage_test.go) | 3 | TestMCPCoreReadCoverage; TestMCPCoreMetadataPreservedAndPrivate | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_merchants_test.go](../internal/app/mcp_merchants_test.go) | 2 | TestMCPMerchantConsentProposalsAndScope; TestDashboardIncomeAllocationScope | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_oauth_test.go](../internal/app/mcp_oauth_test.go) | 4 | TestMCPOAuthRepeatedResource; TestMCPOAuthIsolationAndLifecycle | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_permissions_test.go](../internal/app/mcp_permissions_test.go) | 8 | TestMCPRestrictedCapabilitiesBlockLegacyBypassesAndReplay; TestMCPGrantChangesBlockAppliedReplay | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_queue_batch_test.go](../internal/app/mcp_queue_batch_test.go) | 1 | TestMCPReviewAllocationBatchQueryBoundAndOrder | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_test.go](../internal/app/mcp_test.go) | 13 | TestMCPPrivacyAndTransport; TestMCPRedactsTextWithoutChangingMetadata | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/mcp_transactions_test.go](../internal/app/mcp_transactions_test.go) | 4 | TestMCPTypedTransactionFilters; TestMCPReviewQueueCursorPrivacyAndSelectors | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/merchant_global_test.go](../internal/app/merchant_global_test.go) | 6 | TestMerchantGlobalRulesPrivacyAndBulk; TestMerchantLogoValidationVersionAndScope | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/migrate_test.go](../internal/app/migrate_test.go) | 8 | TestSequentialMigrationHistoricalSnapshots; TestSequentialMigrationFreshAndNoStartupRepair | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/network_test.go](../internal/app/network_test.go) | 4 | TestNetworkSettingsAuthorizationValidationAndPending; TestNetworkRestartAndOfflineRecovery | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/notification_producers_test.go](../internal/app/notification_producers_test.go) | 9 | TestNotificationBudgetBoundariesResetAndPrivacy; TestNotificationBudgetRefundSplitsTransfersPeriodAndProjection | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/notification_push_test.go](../internal/app/notification_push_test.go) | 7 | TestNotificationPushOwnershipConsentValidationAndCSRF; TestNotificationPushIndependentChannelsAtomicPreferencesAndRetry | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/notification_schedule_test.go](../internal/app/notification_schedule_test.go) | 4 | TestNotificationScheduleUpgradePreservesHistoryAndRollsBack; TestNotificationMaintenanceOnlyEvaluatesSourceChanges | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/notifications_test.go](../internal/app/notifications_test.go) | 13 | TestNotificationConcurrentRetryAndImmutablePayload; TestNotificationInboxReadDismissAndOwnership | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/pending_rules_test.go](../internal/app/pending_rules_test.go) | 2 | TestPendingRuleApplicationPreservesFinancialAndReviewState; TestPendingRuleConflictsAndAtomicRollback | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/proxy_test.go](../internal/app/proxy_test.go) | 3 | TestProxyClientIPTrustBoundary; TestHTTPSReverseProxySessionAndOrigin | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/pwa_test.go](../internal/app/pwa_test.go) | 4 | TestPWAIdentityPrivacyPermissionsConcurrency; TestPWAIconValidationAndRevokedActor | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/rules_test.go](../internal/app/rules_test.go) | 5 | TestRuleDirectionGroupConflictAndReview; TestRulePermissionsVersionsAndPreview | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/ruleset_test.go](../internal/app/ruleset_test.go) | 4 | TestRulesetExportImportRoundTrip; TestRulesetScopesDirectionsVersionsAndIdempotency | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/samples_test.go](../internal/app/samples_test.go) | 1 | TestSuppliedFNBPair | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/search_test.go](../internal/app/search_test.go) | 1 | TestGlobalSearch | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/security_hardening_test.go](../internal/app/security_hardening_test.go) | 8 | TestBrowserAdmittedRevocationPreventsAccountCreation; TestBrowserCurrentRolesGuardManagementAndBudgetWrites | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/setup_test.go](../internal/app/setup_test.go) | 2 | TestBrowserSetupSecurityAndSession; TestConcurrentBrowserSetup | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/spending_groups_test.go](../internal/app/spending_groups_test.go) | 5 | TestIndependentSpendingGroups; TestCategoryPopularityRespectsPermissions | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/transaction_approval_test.go](../internal/app/transaction_approval_test.go) | 2 | TestSaveAndApproveAtomic; TestSaveAndApproveSplitsAndTransfers | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/transaction_filters_test.go](../internal/app/transaction_filters_test.go) | 2 | TestTransactionClassificationFiltersBeforePaging; TestImportClassificationFiltersUseCurrentRulesAndLedger | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/ui_backlog_test.go](../internal/app/ui_backlog_test.go) | 8 | TestRuleDefinitionPagesPreserveGroupsAndPermissions; TestScopedAccountRefreshAndVisibilityVersions | Pending | Keep; inspect setup cost before consolidation. |
| [internal/app/user_security_test.go](../internal/app/user_security_test.go) | 9 | TestSelfPasswordSecurity; TestAdminPasswordSecurity | Pending | Keep; inspect setup cost before consolidation. |
| [internal/classification/rules_test.go](../internal/classification/rules_test.go) | 2 | TestClassificationPrecedenceDirectionAndConflicts; TestMerchantPatterns | Pending | Keep; inspect setup cost before consolidation. |
| [internal/ledger/allocations_test.go](../internal/ledger/allocations_test.go) | 1 | TestAllocationFinancialInvariants | Pending | Keep; inspect setup cost before consolidation. |
| [internal/money/money_test.go](../internal/money/money_test.go) | 1 | TestMoney | Pending | Keep; inspect setup cost before consolidation. |
| [internal/statements/fnb_test.go](../internal/statements/fnb_test.go) | 4 | TestFNBPrototypeExactMoneyCoverageAndReferences; TestFNBPrototypeRejectsInvalidResponsesWithoutEchoingData | Pending | Keep; inspect setup cost before consolidation. |

### Frontend unit — 7 files

| Group | Declarations | Distinct failure/intent samples | Runtime | Initial recommendation |
| --- | ---: | --- | --- | --- | --- |
| [web/src/account-discovery.test.ts](../web/src/account-discovery.test.ts) | 2 | preserves leading zeros and masked numbers for explicit correction; rejects credentials, balances, transactions, duplicates and wrong report types | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/api.test.ts](../web/src/api.test.ts) | 3 | converts without floating point rounding; rejects partial and over-precise inputs | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/features/notifications/pushWorker.test.ts](../web/src/features/notifications/pushWorker.test.ts) | 2 | retains push/click integration and uses generic lock-screen text; refuses injected remote links and tolerates malformed provider data | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/features/pwa/worker.test.ts](../web/src/features/pwa/worker.test.ts) | 3 | financial/session/OAuth/MCP/identity/external requests and mutations never enter cache handling; offline navigation returns only generic cached fallback and never stores HTML or private query URLs | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/network-settings.test.ts](../web/src/network-settings.test.ts) | 2 | requires an HTTP origin with no credentials or subpath; accepts exact proxy addresses/ranges and rejects universal trust | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/ruleProposal.test.ts](../web/src/ruleProposal.test.ts) | 1 | keeps merchant text and removes changing numeric references | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [web/src/shared/buildInfo.test.ts](../web/src/shared/buildInfo.test.ts) | 2 | keeps extra runtime and financial fields out of issue reports; keeps issue reporting available without metadata | Pending | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |

### Worker — 4 files

| Group | Declarations | Distinct failure/intent samples | Runtime | Initial recommendation |
| --- | ---: | --- | --- | --- | --- |
| [connectors/fnb/owner/accounts.test.mjs](../connectors/fnb/owner/accounts.test.mjs) | 7 | exports account metadata only, including masked numbers for owner correction; rejects empty, ambiguous and oversized lists with fixed errors | 0.098s all worker groups | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [connectors/fnb/owner/control.test.mjs](../connectors/fnb/owner/control.test.mjs) | 3 | worker receives credentials once and cancellation separately; oversized first input rejects without credentials | 0.098s all worker groups | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [connectors/fnb/owner/refresh.test.mjs](../connectors/fnb/owner/refresh.test.mjs) | 21 | uses exact signed ZAR decimals and rejects ambiguous money or reward units; filters unsupported identities, excludes hidden balances and strips non-account fields | 0.098s all worker groups | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |
| [connectors/fnb/owner/transactions.test.mjs](../connectors/fnb/owner/transactions.test.mjs) | 15 | exact signed money, strict dates, references and identical purchases survive; credit status rejects uncertainty and skips explicit pending authorizations | 0.098s all worker groups | Keep at cheap layer; distinct parsing/privacy/lifecycle contract. |

### Browser — 49 files

| Group | Declarations | Distinct failure/intent samples | Runtime | Initial recommendation |
| --- | ---: | --- | --- | --- | --- |
| [web/e2e/about.spec.ts](../web/e2e/about.spec.ts) | 2 | About and safe issue reporting at ${width}px in ${theme}; About retries a failed metadata request without losing the report link | 9.493s group, local | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/account-discovery.spec.ts](../web/e2e/account-discovery.spec.ts) | 1 | account-only file review imports private metadata with explicit correction and no ledger changes | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/account-icons.spec.ts](../web/e2e/account-icons.spec.ts) | 1 | account type icons ${width} ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/audit-followup.spec.ts](../web/e2e/audit-followup.spec.ts) | 1 | audit prerequisites and layout ${width} ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/budget-presentation.spec.ts](../web/e2e/budget-presentation.spec.ts) | 1 | over-limit category presentation retains account scope in ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/classification-defaults.spec.ts](../web/e2e/classification-defaults.spec.ts) | 2 | typed category and atomic transaction rule ; fresh installation starts empty and imports optional editable defaults | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/cohesive-flow.spec.ts](../web/e2e/cohesive-flow.spec.ts) | 4 | cohesive import and compact screens ${width} ${theme}; clean export automatically opens out-of-period transactions for review and ledger | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/configuration.spec.ts](../web/e2e/configuration.spec.ts) | 1 | configuration file import, coexistence, replacement and export at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/core-improvements.spec.ts](../web/e2e/core-improvements.spec.ts) | 4 | review navigation, drafts, metadata and filters at ${width}px; budget reports, rebalancing and account health at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/dashboard-buckets.spec.ts](../web/e2e/dashboard-buckets.spec.ts) | 3 | group/category budgets and handoff ${theme} ${width}; limits remain available before spending begins | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/dashboard-connections.spec.ts](../web/e2e/dashboard-connections.spec.ts) | 3 | dashboard budget connections and inline editing at ${width}; aggregate category transactions open on dashboard; unbudgeted categories remain editable | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/editor-layout.spec.ts](../web/e2e/editor-layout.spec.ts) | 1 | compact transaction editor ${width} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/fnb-connection.spec.ts](../web/e2e/fnb-connection.spec.ts) | 1 | FNB connection controls preserve secrecy and explicit refresh choices at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/fnb-transactions.spec.ts](../web/e2e/fnb-transactions.spec.ts) | 4 | live FNB preview requires duplicate decisions and confirmation at ${width}px; live fetch failures show a toast without offering a partial import | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/global-search.spec.ts](../web/e2e/global-search.spec.ts) | 3 | shared search and group editor ; search loading, failed request and retry remain distinct | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/handover.spec.ts](../web/e2e/handover.spec.ts) | 2 | branding validation, stale edits, persistence and long names; budget category creation preserves unsaved limits and chosen period | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/income-permissions.spec.ts](../web/e2e/income-permissions.spec.ts) | 1 | income drilldown and merchant permission layout ${width} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/login-toast.spec.ts](../web/e2e/login-toast.spec.ts) | 2 | sign-in errors remain readable and retryable at ${width}px in ${theme}; sign-in request and expired-session errors persist at ${width}px in ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/mcp.spec.ts](../web/e2e/mcp.spec.ts) | 4 | MCP settings and approved agent changes ${width} ${theme}; MCP budget proposal shows existing limits and applies only after approval | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/merchant-global.spec.ts](../web/e2e/merchant-global.spec.ts) | 1 | global merchants and optional logos ${width} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/mobile-budget-settings.spec.ts](../web/e2e/mobile-budget-settings.spec.ts) | 6 | dashboard and budget states ${viewport.width}x${viewport.height} ${theme}; Settings discovery, drafts and management ${viewport.width}x${viewport.height} ${theme} | Pending | Simplify candidate: keep breakpoint/overflow/touch/keyboard and role checks; run long workflows at representative widths. |
| [web/e2e/mobile-core.spec.ts](../web/e2e/mobile-core.spec.ts) | 2 | mobile core ${width}x${height} ${theme}; navigation and editor breakpoint ${width} | Pending | Simplify candidate: keep breakpoint/overflow/touch/keyboard and role checks; run long workflows at representative widths. |
| [web/e2e/mobile-edge-layout.spec.ts](../web/e2e/mobile-edge-layout.spec.ts) | 2 | long mobile values ${width}; mobile non-member navigation presentation | Pending | Simplify candidate: keep breakpoint/overflow/touch/keyboard and role checks; run long workflows at representative widths. |
| [web/e2e/mobile-feedback.spec.ts](../web/e2e/mobile-feedback.spec.ts) | 1 | compact aligned mobile surfaces ${width} ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/mobile-selection.spec.ts](../web/e2e/mobile-selection.spec.ts) | 2 | mobile selection cancels on movement and cancellation, supports keyboard, and exits when empty; categories reuse spending group list ${width} ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/network.spec.ts](../web/e2e/network.spec.ts) | 1 | network saves and restarts at  | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/notification-delivery.spec.ts](../web/e2e/notification-delivery.spec.ts) | 5 | Push opt-in stays explicit; channel drafts save together; device test and removal; Denied permission and unsupported browsers keep in-app preferences usable | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/notifications.spec.ts](../web/e2e/notifications.spec.ts) | 3 | Persistent inbox, authorized links, preferences and responsive keyboard workflow; List failure retries and unavailable target fails safely | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/onboarding.spec.ts](../web/e2e/onboarding.spec.ts) | 3 | inline validation waits for blur, updates while typing and tracks related fields; first-run setup, keyboard validation, mobile themes and permanent closure | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/password-visibility.spec.ts](../web/e2e/password-visibility.spec.ts) | 1 | password visibility and single user menu at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/pending-rules.spec.ts](../web/e2e/pending-rules.spec.ts) | 1 | transaction rule fills pending uncategorized only ${width} ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/polish.spec.ts](../web/e2e/polish.spec.ts) | 1 | whole-app layout and keyboard contracts at ${width}px in ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/pwa.spec.ts](../web/e2e/pwa.spec.ts) | 7 | Branding and PWA identity preserve drafts, save independently and fit at ${width}px; Production manifest, real worker, private-cache exclusion, offline sign-out and reconnect (${mobile ? "phone" : "desktop"}) | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/review-defaults.spec.ts](../web/e2e/review-defaults.spec.ts) | 2 | review filters are editable defaults ${width} ${theme}; review count includes seen uncategorized transactions and entry clears unrelated filters | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/rule-layout.spec.ts](../web/e2e/rule-layout.spec.ts) | 1 | compact rule indicators ${width} ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/rules.spec.ts](../web/e2e/rules.spec.ts) | 2 | simple rules across enabled accounts ; rule editor can create its category and check overlapping rules | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/seen.spec.ts](../web/e2e/seen.spec.ts) | 1 | automatic acceptance and seen ${width} ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/settings.spec.ts](../web/e2e/settings.spec.ts) | 2 | settings navigation keeps account views clean and preserves drafts at ${width}px; ordinary users see General, PWA, MCP, Notifications, Security and About without management requests | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/theme-states.spec.ts](../web/e2e/theme-states.spec.ts) | 1 | shared interaction states at ${width}px in ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/theme-toggle.spec.ts](../web/e2e/theme-toggle.spec.ts) | 1 | navbar theme preference stays synchronized at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/toasts.spec.ts](../web/e2e/toasts.spec.ts) | 1 | toasts overlay pages and dialogs without shifting layout at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/transaction-editor.spec.ts](../web/e2e/transaction-editor.spec.ts) | 2 | transaction save/approve and stable pickers ${mobile?'mobile':'desktop'} ${theme}; linked transfer group cannot be cleared (${hidden?'hidden':'visible'} counterpart) | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/transaction-entrypoints.spec.ts](../web/e2e/transaction-entrypoints.spec.ts) | 2 | shared editor from categories accounts periods and nested references at ${width}px; import history resolves saved ledger IDs and denied links show a toast | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/transaction-filters.spec.ts](../web/e2e/transaction-filters.spec.ts) | 1 | classification filters across tabs ${width} ${theme} | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/ui-backlog.spec.ts](../web/e2e/ui-backlog.spec.ts) | 2 | shared account actions, refresh scope and pending states at ${width}px; rule pages retain grouped edit actions and recover from failed loads at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/ui-hierarchy.spec.ts](../web/e2e/ui-hierarchy.spec.ts) | 1 | focused workspaces ${width} ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |
| [web/e2e/user-security.spec.ts](../web/e2e/user-security.spec.ts) | 2 | administrator reset and confirmed deletion at ${width}px; self password change validation and session retention at ${width}px | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/workflows.spec.ts](../web/e2e/workflows.spec.ts) | 3 | desktop dashboard, keyboard review, rules, and split approval; 360px mobile upload, review, period preview, and both themes | Pending | Keep interaction/security coverage; review repeated viewport/theme workflows. |
| [web/e2e/workspace-controls.spec.ts](../web/e2e/workspace-controls.spec.ts) | 1 | compact workspace controls ${width} ${theme} | Pending | Consolidation candidate: compare geometry/focus contracts; replace redundant fixed styling only with equivalent evidence. |

## Assertion-level relevance findings — 8 October

This follow-up reads actual assertions/fixtures in the named groups below, plus a suite-wide scan for CSS/font assertions, sleeps and skips. It is stronger evidence than the initial name-based inventory. It does not claim every assertion in all 54 backend and 49 browser files has been individually reviewed.

| Group | Evidence and distinct regression | Recommendation |
| --- | --- | --- |
| money, ledger, classification domain tests | Reject imprecise/overflowed money; require same-sign exact splits; preserve refund/transfer rules; withhold conflicting classification. Browser tests cannot reliably replace these parser/domain boundaries. | Keep all inspected domain cases. |
| acceptance_test.go | Two different users have independent seen state; edits invalidate another user's old seen version; unauthorized/private/hidden batch failures leave no partial effects; seeing does not alter reporting; migration preserves money and only marks historical reviewers seen. | Keep. These cover current accepted product behavior. |
| security_hardening_test.go | Revokes/rotates the credential after middleware admission; checks status and absence of account/audit mutation; injects audit failure then retries user creation. These are real server-state changes, unlike mocked browser roles. | Keep. Distinct revocation/atomicity risks justify subcases. |
| mcp_permissions_test.go | Revoked capabilities/access block application and replay; seen proposals cannot bypass approval, mutate financial versions, duplicate audits or partially write stale batches. | Keep. Do not collapse replay/consent/current-grant checks into generic success tests. |
| migrate_test.go | Real historical schemas 9/19/22; protected financial/source/grant/audit/session records compared before/after; injected DDL/backfill/version-record failures roll back and retry; repeated startup does not reapply. | Keep. Data-preservation comparisons are meaningful even where snapshots look implementation-specific. |
| import_live_test.go | Repeated overlapping pages preserve identical-purchase multiplicity and separate fees, recognize immutable source after user edits, and reject revoked/hidden targets. | Keep. This is distinct from exact-upload-hash/FITID idempotency. |
| mcp_queue_batch_test.go | One query for a 100-entry allocation page, zero for empty input, page-ID-only hydration, deterministic allocation order and output redaction. | Keep. The query bound is an intentional performance regression contract, not an arbitrary implementation count. |
| transaction_approval_test.go | Legacy approve input still has authorization, stale-version, rule/edit/audit atomicity and compatibility metadata behavior; save-only follows automatic acceptance. | Keep compatibility coverage. Rename old test descriptions separately if desired; old names do not establish obsolescence while the API contract remains supported. |
| Frontend unit files | Exact cents parsing/formatting, bounded rule proposals, rejecting account-discovery financial/credential fields, network trust validation, public issue-report allowlist. | Keep. Low cost and meaningful input/privacy boundaries. |
| Push/PWA worker unit files | Execute worker handlers with synthetic event/cache/fetch adapters: generic push text, safe click links, financial/auth cache exclusions, offline fallback, owned-cache pruning and explicit activation. | Keep. These are executable behavior checks, not merely searching source text. Real service-worker browser lifecycle cases add independent platform coverage. |
| polish.spec.ts | Contrast >=4.5, reduced-motion behavior, card alignment, overflow, skip-link focus, roving Settings focus, invalid-field focus and empty states are useful. Exact heading 24/28px, page RGB and 14px radius do not independently protect usability. | Keep behavioral/geometry/accessibility assertions; remove or centralize fixed theme/font/radius literals in a later measured change. Do not remove the entire spec. |
| theme-states.spec.ts | A synthetic page exercises production CSS for hover, selected/disabled stability, >=4.5 contrast, error borders and keyboard focus. Width does not choose a different application workflow in this fixture. | Keep light/dark state coverage; test whether one width suffices. Replace literal RGB/2px outline with token consistency and visible focus/contrast invariants. |
| mobile-budget-settings.spec.ts | Each long budget and Settings workflow repeats at six viewports times two themes. Settings rechecks exact complete tab order, draft retention, validation, bank credential clearing, MCP consent reset and menu focus. Separate ten-width breakpoint tests already check terminal-tab visibility and reachable save. | Consolidate long interaction workflows at representative phone, short landscape and desktop sizes. Retain lightweight geometry at all meaningful breakpoints/themes plus distinct empty/error states. Keep credential-clearing and consent-reset assertions. Any reduction still needs equivalence mapping and measured verification. |
| settings.spec.ts + ordinary Settings matrix | Overlap in draft/roving focus/navigation; only settings.spec.ts records absence of management requests. Both present ordinary-user UI by overriding /api/me while the actual session remains administrator. | Keep absence-of-management-request assertion. One phone and one desktop role-presentation case should suffice if geometry stays elsewhere. These are presentation evidence, not proof of server denial. Backend permissions remain required. |
| about.spec.ts + buildInfo.test.ts | Unit checks assert exact allowlisted issue URL contents with private extra fields; browser checks verify version entry points, mobile Settings keyboard navigation, link wiring, overflow and failed-metadata retry. | Keep unit allowlist and desktop/mobile wiring/retry. Repeating privacy URL checks in both themes adds little independent logic coverage; move theme layout into lighter checks. |
| rule-layout.spec.ts + merchant-global.spec.ts | Both assert circular, square avatars. Rule layout also protects paused/global/active labels and Escape focus; merchant workflow protects logo removal and global-scope persistence. | Keep their distinct interactions. Consolidate duplicate avatar styling in one shared presentation test; <=16px status size alone is weak. |
| workspace-controls.spec.ts + ui-hierarchy.spec.ts + review-defaults.spec.ts | Overlap in collapsed filters/defaults/visible scope; distinct assertions include clearing active filters, editable review defaults, preserving hidden drafts, Escape focus and invalid controls revealing themselves. Absolute top coordinates depend on unrelated header changes. | Map each distinct interaction before merging. Prefer containment/reachability over fixed top positions and compact-height ceilings. Do not discard draft/default behavior. |
| mobile-feedback.spec.ts + mobile-selection.spec.ts | Overlap in selection entry/exit. Selection adds movement/cancellation, synthetic post-hold click consumption, keyboard selection and seen-action clearing; feedback adds amount/merchant alignment, transfer/private labels and no layout shift on selection. | Keep the distinct gesture cases and readable-money/selection geometry. Consolidate round-avatar/grid-border/radius equivalence; do not dismiss gesture waits as arbitrary sleeps because they deliberately span the threshold. |
| budget-presentation.spec.ts | Correct over-limit amount/progress metadata and account-scope behavior; duplicate literal RGB and hex token checks. | Keep semantic budget/scope checks; replace duplicate palette literals with token consistency/contrast checks. |
| Separator-free checks | Mobile budget groups and workspace controls assert absence of trailing separators. AGENTS.md explicitly defines this accepted convention. | Retain focused shared-style coverage; these checks have a documented product reason, unlike arbitrary fixed styling. |

### Coverage limits and optional checks

- samples_test.go requires owner-supplied exports and exact historical sample counts. It is a manual compatibility probe, skipped by ordinary CI. Keep it separate from claims of synthetic CI coverage; do not inspect owner files during this audit.
- ConfigurationPublicStarterPull is opt-in external Git smoke; it is separate from deterministic validation/atomicity tests. Its ordinary skip is expected.
- WSL/Windows/browser cancellation tests are platform-conditional. A Linux CI pass does not claim Windows cleanup coverage. Keep platform-specific synthetic checks for supported environments.
- The real PWA update case is skipped under direct Playwright without its private static copy, but the normal disposable runner supplies that copy. Keep it: unit adapters cannot establish real multi-tab worker activation.
- Browser waits in mobile-selection intentionally check that cancellation remains safe after the gesture threshold; the network test's fixed one-second restart wait deserves separate review for a readiness-based replacement.

### Overall judgment

The inspected suite has substantial relevant coverage. The clearest maintenance waste is repeating long UI interactions across presentation combinations and mixing arbitrary palette/font/radius/top-position literals into behavioral tests. No inspected critical financial/security group is proven redundant enough to remove. Begin consolidation with Settings/theme matrices and shared-style duplication, preserving explicitly named invariants. That review itself changed no tests; the subsequent authorized implementation is recorded above.

## Next review

1. Collect complete per-spec timings and backend JSON test durations before making removal recommendations; separate setup/shutdown, process overhead and test execution.
2. Compare Settings/theme/viewport workflows with the bounded smoke set. Keep administrator-only visibility and absence of management requests; avoid replacing these with literal tab counts.
3. Compare styling assertions with shared geometry helpers. Preserve overflow, readable money, 44px primary targets, navigation clearance, keyboard access and restored focus.
4. Map every proposed removal to a retained test/layer and list lost state/breakpoint/theme coverage explicitly.
5. Consider lazy empty-fixture startup only after inventorying which specs use the second/third services. Keep fresh synthetic financial/settings data across specs.

## Reproduction and artifacts

Run npm run test:e2e -- about.spec.ts. Results live in web/test-results/run-*/timings.json, per-spec Playwright JSON, failure artifacts and three synthetic server logs. Run-specific output directories permit concurrent runs; files inside a spec remain sequential. Output is ignored, contains synthetic fixtures only, and is retained in CI for seven days on success or failure. No database or credentials are uploaded. Direct Playwright commands still use their original shared output path and should run sequentially.

No MCP tools, schemas, output allowlists, consent, proposal previews/audits, application services or product behavior changed. This work affects verification infrastructure only.


## PR CI follow-up — #55

The first published PR run stopped at the desktop PWA identity case: the test read the manifest after observing a disabled save button, which also represents an in-flight save. The exact identity assertion is retained; the test now awaits and validates the successful PUT /api/pwa response before reading the manifest. Both phone/desktop workflows passed three repetitions each locally (six cases, run-htend9op, 16.627s total). The failed initial CI run is not a pass or a complete browser timing measurement. No application code changed.
