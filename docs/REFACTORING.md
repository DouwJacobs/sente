# Module ownership

Use these boundaries when extending the application. The application remains one Go service with a React/Vite frontend.

## Backend

| Module | Responsibility |
| --- | --- |
| `internal/money` | Exact signed decimal parsing into integer minor units. |
| `internal/classification` | Description normalization, category-rule precedence/direction/conflicts and merchant pattern matching. No database or transport. |
| `internal/statements` | Normalized import models, bounded ZIP expansion, CSV/OFX parsers, FNB report normalization, immutable source fingerprints and coverage. |
| `internal/ledger` | Allocation count, sign, exact totals, note bounds and category-completeness validation. The application supplies a category lookup using the current SQL transaction. |
| `internal/problem` | Safe error status/message values shared with transport adapters. |
| `internal/app` | Runtime composition, permissions, database transactions, audit, HTTP/MCP adapters and shared authorized write services. |

Dependencies point from app into domain packages. Statements uses money/classification; ledger and statements use problem. None imports app. The thin app aliases preserve its existing normalized-model and money/FNB facade; domain implementations exist once.

Persistence remains intentionally in app: moving a method to a separate package solely to reduce file size would force a large exported database/authorization interface and risk fragmented transactions. Focused workflow files make the existing shared services navigable:

- `migrate.go` owns the sequential schema registry, connection settings, atomic upgrade transaction and version records. `migrate_legacy.go` owns the frozen pre-23 schema compatibility bridge; `migrate_legacy_data.go` owns its named historical data conversions. `merchant_migration.go` rebuilds the legacy catalogue inside the runner-owned transaction.
- `browser_write.go` owns transaction-scoped browser actor/session refresh; `auth_maintenance.go` owns bounded login/OAuth buckets and serving credential retention.
- `users.go` owns user administration; `user_security.go` owns browser security mutations and shared atomic password/agent/session revocation used by offline recovery.
- `periods.go`, `budget_targets.go`, `budget_target_queries.go`, `budget_settings.go`, `dashboard.go` separate period lifecycle, atomic targets, target reads, preferences and aggregates.
- `transaction_queries.go`, `transactions.go`, `transaction_review.go`, `transfers.go`, `transaction_audit.go`, `transaction_metadata.go` separate authorized ledger queries, atomic editing, review, linked transfers, audit and metadata.
- `configuration.go`, `configuration_state.go`, `configuration_preview.go`, `configuration_apply.go`, `configuration_repository.go` separate source contracts, configuration snapshots/diffs, validation previews, atomic application and public Git fetching. `ruleset.go` owns the shared portable import/export writers.
- `labels.go`, `categories.go`, `merchant_rules.go`, `merchant_matching.go` separate catalogue workflows and merchant matching/persistence.
- `fnb_credentials.go`, `fnb_connection.go`, `fnb_runner.go`, `fnb_snapshots.go`, `fnb_provider.go`, `fnb_scheduler.go`, `fnb_diagnostics.go`, `fnb_authorization.go`, `fnb_process.go` separate encrypted storage, settings endpoints, serialized jobs, snapshot application, bounded subprocess IO, scheduling and allowlisted diagnostics.

Authorization, versions, source identities, exact proposal effects and audit remain checked inside the existing serialized writes. MCP calls these same services. No input schema, output allowlist, capability, consent or endpoint changes accompany this refactor.

## Frontend

| Folder | Responsibility |
| --- | --- |
| `features/auth` | Session lifecycle, startup/sign-in gates and first-run/sign-in forms. |
| `features/workspace` | Controlled navigation/chrome and cancellable workspace data/count loading. |
| `features/dashboard` | Dashboard rendering and summary figures. |
| `features/accounts` | Account screen using shared account management. |
| `features/categories` | Category/group/rule screen. |
| `features/budgets` | Budget-period screen, paged BudgetGroups hierarchy, shared CategoryBudgetModal and report integration. |
| `features/settings` | Settings orchestration, general/security forms, portable configuration source forms/preview/history, access list/dialogs and persistent drafts. |
| `features/imports` | Import orchestration, activity expansion, exception previews, retained source rows and rule suggestions. |
| `features/transactions` | Ledger selection/paging, editor orchestration, split controls, transfer/provenance extras and audit history. |
| `shared/types.ts` | Row, workspace data and page contracts. |
| `shared/useTask.ts` | Shared busy state, duplicate-request lock and request notifications. |
| `shared/useAppearance.ts` | Theme preference and device-theme subscription. |

App composes session, theme, navigation, filters and shared workspace data. Feature modules import shared contracts/hooks rather than App, avoiding the old entry-point cycles. Existing API, UI primitives, selectors, transaction access and theme files remain shared. Editor financial drafts remain in the editor; independent transfer lookup/history state stays in its extras component. The parent supplies the request lock so saves, seen changes and transfer operations remain serialized.

## How to extend

Add work beside its domain owner; extract by responsibility when a module accumulates independent workflows. Prefer explicit imports and small public surfaces. Keep SQL writes atomic and UI draft ownership clear. A shared abstraction must serve an actual caller; avoid frameworks, microservices or generic repository layers merely for file size. Keep behavior changes separate from moves and update this map when an accepted boundary changes.

Verify with the documented WSL Go/Node runtimes: `go test -race ./... -timeout=30m`, `go vet ./...`, frontend `npm test` and `npm run build`, and isolated synthetic Playwright workflows. Domain financial tests live beside their implementation; browser/MCP authorization, stale-write and rollback tests remain in app. Do not inspect production data or imply visual/live-bank verification from synthetic results.

Build information belongs to `internal/buildinfo`; the browser adapter is `internal/app/build_info.go`. The frontend contract/hook and report-link builder live in `shared/buildInfo.ts`, with About owned by `features/settings/AboutSettings.tsx`.

MCP `mcp_aggregates.go` owns filtered aggregate read adapters; `mcp_context.go` owns context storage/browser writes/session delivery; queue hydration remains in `mcp_transactions.go`. Frontend `features/mcp` owns the personal-context form/draft; shared consent fields remain usable by OAuth and Settings.

MCP Settings composes `features/mcp/MCPConnectionCard.tsx` and `MCPProposalCard.tsx` for connection summaries and exact change presentation; parent request locks, selection and permission drafts remain in MCPSettings.

Notification infrastructure belongs in `internal/app/notifications.go` (event/service/adapter), `notification_schema.go` (immutable migration 24), `notification_api.go` (browser inbox/state), `notification_preferences.go` (personal settings) and `notification_maintenance.go` (serving lifecycle). Producers use the shared authorized transaction service rather than writing inbox rows or receipts independently. The browser feature lives in `web/src/features/notifications`: contracts, header count polling, persistent inbox and personal preference forms. App owns page/source navigation; transaction links use TransactionAccess. Settings preserves mounted preference drafts. See NOTIFICATIONS.md for privacy and retry contracts.

Notification producers: `notification_budget_producers.go` uses `budget_notification_totals.go`, also called by the dashboard. `notification_spending_producers.go` owns transparent amount/recurrence detection on authorized account-scoped allocation observations. `notification_conditions.go` owns persistent cycles, cooldowns, evaluation reconciliation and bounded diagnostic recording; `notification_diagnostics.go` owns the redacted admin API. `notification_push_api.go` owns browser opt-in/devices; `notification_push_delivery.go` owns queueing, safe Web Push transport, durable claims and retries. Migrations 25/26 live in separate immutable schema files. Frontend `PushDevices`, `NotificationDiagnostics` and `NotificationItem` stay with the notification feature.

`features/notifications/NotificationSettings.tsx` owns the Notifications Settings subtab composition, preserving preference/device panels while exposing compact diagnostics only to administrators.

`notification_schedule_schema.go` owns migration 27 source-revision triggers; `notification_maintenance.go` separates source/date evaluation from persisted push delivery deadlines.

`internal/app/pwa.go` owns bounded PNG normalization/rendering, public manifest/icons and authorized PWA settings; `pwa_schema.go` owns immutable migration 28. `branding.go` retains workspace-name/logo saves. `features/pwa/PWAProvider.tsx` owns installation/update lifecycle; `features/settings/BrandingSettings`, `PWASettings` and `IconField` own draft-preserving forms. Vite generates the root push/PWA worker's exact asset allowlist and revision. See PWA.md for cache/authentication boundaries.

`features/pwa/PWAInstallInvitation.tsx` owns signed-in installation discovery. `installInvitationPreference.ts` shares the browser-local suppression flag with native install actions in PWAProvider; this preference contains no financial/user data and grants no permissions.


Issue #69: `notification_budget_preferences.go` owns personal category/group setting contracts, shared atomic save helpers and silent baselines; `notification_budget_preference_schema.go` owns immutable migrations 29/30. `CategoryBudgetAlert.tsx` owns the personal fields used by `features/budgets/CategoryBudgetModal.tsx` for dashboard and Budgets add/edit workflows; NotificationPreferences owns global type/channel choices. Producers continue using shared allocation arithmetic; internal delivery scope metadata permits push preference rechecks without adding inbox/MCP output fields.


`mcp_budget_alerts.go` adapts explicit personal category/group alert reads and proposals to the shared `notification_budget_preferences.go` save/baseline services. Consent stays in `mcp_permissions.go`; exact preference evidence follows `mcp_changes.go` proposal/audit transactions. MCP Settings/OAuth reuse shared permission fields; `MCPProposalCard` owns before/after presentation.
