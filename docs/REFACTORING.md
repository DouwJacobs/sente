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

- `users.go` owns user administration; `user_security.go` owns browser security mutations and shared atomic password/agent/session revocation used by offline recovery.
- `periods.go`, `budget_targets.go`, `budget_target_queries.go`, `budget_settings.go`, `dashboard.go` separate period lifecycle, atomic targets, target reads, preferences and aggregates.
- `transaction_queries.go`, `transactions.go`, `transaction_review.go`, `transfers.go`, `transaction_audit.go`, `transaction_metadata.go` separate authorized ledger queries, atomic editing, review, linked transfers, audit and metadata.
- `labels.go`, `categories.go`, `merchant_rules.go`, `merchant_matching.go` separate catalogue workflows and merchant matching/persistence.
- `fnb_credentials.go`, `fnb_connection.go`, `fnb_runner.go`, `fnb_snapshots.go`, `fnb_provider.go`, `fnb_scheduler.go`, `fnb_diagnostics.go` separate encrypted storage, settings endpoints, serialized jobs, snapshot application, bounded subprocess IO, scheduling and allowlisted diagnostics.

Authorization, versions, source identities, exact proposal effects and audit remain checked inside the existing serialized writes. MCP calls these same services. No input schema, output allowlist, capability, consent or endpoint changes accompany this refactor.

## Frontend

| Folder | Responsibility |
| --- | --- |
| `features/auth` | Session lifecycle, startup/sign-in gates and first-run/sign-in forms. |
| `features/workspace` | Controlled navigation/chrome and cancellable workspace data/count loading. |
| `features/dashboard` | Dashboard rendering and summary figures. |
| `features/accounts` | Account screen using shared account management. |
| `features/categories` | Category/group/rule screen. |
| `features/budgets` | Budget-period screen and existing builder/report integration. |
| `features/settings` | Settings orchestration, general/security forms, access list/dialogs and persistent access drafts. |
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
