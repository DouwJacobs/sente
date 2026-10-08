# Architecture

## Runtime and modules

React 19, TypeScript and Vite provide the frontend. Go 1.27.1 serves the JSON API at `/api` and built static files through `net/http`. The pure-Go modernc SQLite driver avoids CGO in the final container; `x/crypto` provides bcrypt and `x/text` provides import decoding.

Production runs one non-root Go service in a read-only Docker container, with named database/backup volumes. `PUBLIC_URL` sets the browser origin and HTTPS cookie policy; a reverse proxy handles TLS. See [REVERSE-PROXY.md](REVERSE-PROXY.md).

[REFACTORING.md](REFACTORING.md) maps module ownership. `internal/app` composes routes, permissions, SQL transactions and audited services shared by browser, MCP and offline commands. Domain packages `money`, `classification`, `statements` and `ledger` own reusable invariants; `problem` owns safe errors. Domain packages never import app or HTTP/MCP transports. App facade aliases stay thin.

Frontend workflows live in `web/src/features/<feature>`. App composes session, navigation, theme, filters and shared data. Contracts/request hooks live in `web/src/shared`; features do not import App. Shared UI, choices, transaction access and API client remain reusable. Keep editor drafts and request locks with their workflow owner.

`make dev` runs Vite at 127.0.0.1:5173 and a watched Go backend at 8081, proxying API requests under the frontend origin. Development uses its own persistent database; production serves `web/dist`. Playwright uses isolated synthetic services. Runtime paths and commands are in [DEVELOPMENT.md](DEVELOPMENT.md).

## Persistence and authorization

The ordered registry in `internal/app/migrate.go` initializes and upgrades SQLite. The current schema is 24. Its one-time compatibility bridge uses the frozen version-22 `schema.sql` snapshot and preserves sparse legacy version history; subsequent upgrades append named, consecutive migrations. Schema/data changes and version records commit in one transaction, with a pinned connection and foreign-key integrity checking for table rebuilds. Current-version startup does not reapply schema or backfills. Foreign keys, WAL, one pooled connection and short serialized writes protect consistency. All mutation services check permissions, expected versions, dependencies and audit within one SQL transaction. Browser handlers, MCP application and offline rulesets call those services rather than separate writers.

Budget membership grants editor access to household accounts. Private accounts require explicit grants, including for administrators, and do not participate in shared budgets. Administrative account-management metadata excludes financial records. Hidden accounts are excluded before ordinary reads/aggregates and connector work. Access checks precede search, count, paging, history and aggregates.

Immutable transaction source date/amount/description, FITID, import linkage and provenance remain separate from editable fields/allocations. Monetary arithmetic uses signed integer cents. Allocation totals/sign/category completeness are checked by `internal/ledger`; category lookup uses the active SQL transaction. Parent cash movement and allocation spending are never counted together.

Acceptance follows complete categories, with an explicit-transfer exemption. Compatibility JSON retains `review_state=approved` for Accepted and `pending_review` for Needs category. Acceptance audits remain distinct from historical explicit approval metadata. `transaction_seen` stores a user's seen version/timestamp without changing the financial version. Imports/rule targets begin unseen; saves mark the edited version seen. List snapshots include the user's current seen state.

Categories remain flat. Legacy `group_name` storage preserves history without creating current ownership. Group labels on transactions/rules and budget scopes are independent. Duplicate category names reject case-insensitively. Archive checks preserve recurring budget dependencies. Transactions also have notes, merchant IDs and tags; omitted edit fields retain existing metadata. Merchants and naming rules support global or account scopes, local logos and independent category/group defaults.

## Imports, rules and bounded reads

`internal/statements` owns normalized `ParsedFile`/`SourceRow`, CSV/OFX parsing, bounded ZIP expansion and FNB normalization. App staging and commit retain source hashes/run/coverage/provenance, authorize the account and recheck current classification. Uploaded bytes are discarded. Clean imports commit through the same audited writer; invalid/ambiguous/conflicting candidates retain exception decisions. Live overlap policy and exact fee/identity rules are in [PLAN.md](PLAN.md).

Classification normalizes descriptions, honors direction and custom-over-built-in precedence, and withholds conflicting outputs. Rules have explicit scope, priority, enabled state and version. Batch changes include full grouped account membership and commit atomically. Transaction-created rules apply within the source edit transaction to eligible uncategorized unsplit matches; failures roll back edit, rule, applications and audit. Merchant matching is independent of category rules.

Metadata pages use deterministic order, server search and list snapshots (maximum page size 100). Transactions use versioned offset pages; typed MCP review uses opaque cursors. Import summaries avoid whole row blobs: paged previews return at most 50 rows, activity pages retain original run grouping and bounded account summaries. Annotation/decisions still evaluate the complete stored import. Exact-ID hydration retains selections outside the current page; unloaded pages cannot erase drafts or targets.

Transaction filters combine authorized account/period/import scope with allocation-aware category sets, missing categories, acceptance, seen, dates, amounts, merchant/tag and search. Split matches count a parent once. Staged import filtering uses its own classification semantics. Reads use one SQLite snapshot for count, rows, allocations and version; aggregate totals cover the full authorized scope.

## Budgets, reports and previews

`group_targets` stores unique period/category/nullable-group entries; null means No spending group. `budget_groups` preserves included empty groups. Entries retain inclusion and carry-forward flags, including explicit zero amounts. Budget writes preserve undistributed legacy aggregate targets as ungrouped entries before rebuilding compatibility totals. Named scope is never inferred from category metadata.

Builder changes are sparse and version/member checked. Scoped replacement affects only its named group. Upcoming recurrence fills missing later entries without overwriting included ones, updates each affected period once and audits in the same write. New periods copy recurring entries. Removing membership preserves financial/category history.

Period-date previews fingerprint affected transaction/period state. Rebalance previews bind current limits/spending and preserve total budget. Browser bulk-edit previews bind user, semantic changes and transaction versions for 15 minutes; application recomputes and rechecks under serialized write. New label names resolve on apply. Bulk operations affect 1–100 selected transactions atomically.

Dashboard and reports aggregate allocations under shared authorization, refunds and explicit-transfer rules. Household scope uses assigned periods; private account selection uses date scope and omits shared limit comparisons. Dashboard spending/income pages use exact period/account/group/category scope and bounded snapshot paging. `matched_spend_cents` describes matching allocations without replacing the parent amount. CSV text is formula-escaped; cents remain numeric.

## Shared browser behavior

The API client centralizes credentials, CSRF, session expiry and JSON errors. Shared Field/Form implements first-blur then live inline validation, dependent revalidation and first-invalid focus on submission. Toast provides dismissible status/errors above dialogs with hover/focus expiry pause. Theme preference is local; credentials are never stored in browser localStorage.

`TransactionAccess` resolves current authorized ledger records from every entry point. Nested editors keep parent drafts mounted and restore focus; child saves invalidate parent period previews. Source rows lacking a ledger identity remain source details. Scoped navigation follows the original list position and protects drafts. Search discards stale asynchronous results and distinguishes errors from empty results. [UI.md](UI.md) defines the active presentation conventions.

## FNB worker

Owner-admin connection routes manage encrypted credentials, schedule/debug state, discoveries and visibility. AES-256-GCM uses owner-bound associated data and an external 0600 key; existing ciphertext without its key fails without generating a replacement. Credentials have no read endpoint or audit payload. See [FNB-RUNTIME.md](FNB-RUNTIME.md).

Go launches `connectors/fnb/owner/refresh.mjs` with credentials and target identities on stdin. Strict bounded JSON stdout is accepted; stderr is discarded. WSL uses Windows Node/Chrome with a probed live interop socket; native Linux/Windows runtimes remain configurable. No credentials enter arguments/environment. Runs share one connector lock and recheck owner/account/visibility at persistence. JSON is bounded to 16 MiB; transaction work has a four-minute Go timeout and bounded worker abort/cleanup.

Temporary browser profiles live in OS temp and are removed on normal completion. Manual and scheduled jobs default to headless; saved debug mode shows Chrome for owner approvals. Approved-origin/form checks precede credential entry. Successful cleanup requires confirmed logout; failures pause scheduling. Numeric diagnostics use a fixed allowlist, including startup phase/timeouts and account/transaction/layout counts; no raw errors, page text or credentials are exposed.

Scheduled jobs fetch mapped visible editable targets and refresh balances in the same provider session; initial discovery without targets is account-only. Clean imports are committed server-side. Coverage always admits incomplete history. Supported account/fee/masked-credit rules are in PLAN. Live compatibility remains owner-tested. The standard image excludes the browser/runtime. `connectors/fnb/service` is a disconnected mock experiment, not the active deployment.

## MCP and offline configuration

The official Go MCP SDK provides stateless Streamable HTTP at `/api/mcp`. Ordinary GET/HEAD exposes public setup text; protocol requests require agent credentials, not browser cookies. OAuth uses browser sign-in/CSRF consent, S256 PKCE, one-use codes, endpoint-bound expiring tokens and rotating refresh credentials. Refresh reuse revokes the connection. Password change/reset, user disable and restore invalidate agent access.

Connection permission JSON and account constraints are checked at prepare, approval, apply and replay. Explicit output fields exclude private notes/source identity; merchant reads require consent. Exact one-hour proposals capture before/after financial state and versions. Grouped budget proposals resolve stable IDs and retain membership, target, inclusion and recurrence evidence. Application commits financial effects, result and audit together for idempotent replay. Automatic approval requires every effect's explicit grant and is rechecked before apply; it cannot approve older pending proposals retroactively. See [MCP.md](MCP.md) for the full tool, schema, permission and redaction contract.

Portable configuration version 2 uses consistent snapshot export and one atomic authorized import service shared by browser previews/application and offline CLI/Python wrappers. Schema 21 adds source history and expiring user-bound configuration previews. Preview simulation rolls back SQL savepoint changes; apply rechecks configuration/access fingerprints and explicit replacement consent. Public HTTPS Git sources use isolated, bounded manual fetches and record the applied commit. No background sync or MCP capability is added. Fresh installations skip historical classification seeds; existing records and upgrade migrations remain. See [RULESETS.md](RULESETS.md).

## Recovery and network lifecycle

Backups use `VACUUM INTO`, integrity checks and 14-snapshot retention; automatic work runs at startup and at least every 24 hours. Offline restore checks integrity/schema under the exclusive process lock, preserves the previous database and clears browser/agent sessions. Runtime keys require separate recovery.

Trusted proxy IPs/CIDRs affect client identity for login throttling; malformed/untrusted forwarding falls back to the socket peer. `PUBLIC_URL` remains the cookie/origin authority. Saved network overrides apply only at startup; offline reset recovers environment defaults. Admin restart acknowledges before graceful shutdown drains jobs/database. Development exits with code 75 for supervisor restart and exact Vite hostname configuration.

Historical migration narratives and intermediate findings are in the [archive](archive/README.md); [VERIFICATION.md](VERIFICATION.md) records actual test coverage and external limits.


## User security — issues #15, #16 and #17 (2026-10-07)

`codex/user-security` in `/home/douw/sente-user-security` adds administrator password resets and explicit user deletion, and hardens the existing self-service change. Schema 20 adds nullable `users.deleted_at` without rewriting financial history. Deletion is permanent access removal: retain the username/ID as a reserved historical identity for imports, rules, audit and review attribution; clear its password, roles and membership, hide it from user lists, remove grants/personal seen markers/sessions/FNB credentials and discoveries, and revoke MCP connections plus pending OAuth consent. Accounts, source provenance, allocations, classification rules and financial history remain unchanged. Deleted identities cannot be restored through update/reset/grants. Keep an enabled administrator; self-deletion requires another administrator.

Administrator resets target another user with optimistic version checks, leave disabled status unchanged, and revoke all target sessions and MCP credentials. Self-service requires the current password, deliberately retains the initiating session and revokes other sessions/all MCP connections. Both use the existing 12–72-byte rule and audit without password material. Offline recovery shares the credential-revocation service and records its recovery method. Login rechecks the verified password and active user atomically with session creation; security mutations recheck the actor session/administrator role within the write transaction. Audit failures roll back all mutation/revocation effects.

MCP impact: account administration/passwords remain browser-only (offline recovery also remains available). No tool, input schema, output allowlist, capability or consent expansion. Connection deletion uses existing cascades for proposals, OAuth codes, access and refresh tokens, and also removes pending user-bound authorization requests. Financial audit evidence remains. Frontend uses shared forms/fields, first-blur/live errors, dependent password confirmation, native dialogs, explicit typed-username deletion and overlay toasts. No production database, banking credentials, deployment or manual visual inspection.

## Build metadata

`internal/buildinfo` owns public version/revision metadata, using release linker values with Go VCS fallback. `internal/app/build_info.go` exposes it through the authenticated, non-cached browser API. `scripts/build-metadata.sh` and Docker workflow arguments derive release information from Git. `web/src/shared/buildInfo.ts` owns the typed browser contract and safe issue-report URL; Settings owns the About view. This metadata does not read the database or alter MCP consent.


## MCP summaries and personal context

`mcp_aggregates.go` owns bounded SQL summaries and period comparisons over the shared authorized ledger scope, with allocation-based income/spending and separate parent cashflow. Merchant dimensions retain explicit consent. `mcp_transactions.go` hydrates queue allocations in one page query inside the existing snapshot.

`mcp_context.go` owns private context storage, versioned browser writes and consent-gated MCP reads/initialization. Schema 22 adds `mcp_user_context`; existing connections retain sharing off. Middleware copies each request's initialization result rather than modifying shared instructions. Audit records contain only version/length. `features/mcp/SavedMCPContext.tsx` owns the draft; Settings keeps MCP mounted after first visit to preserve drafts across tabs.

## Notification infrastructure

Migration 24 adds recipient-scoped notification payloads, durable retry receipts, immutable account dependencies and personal in-app preferences. `notifications.go` owns the validated event contract, current source/recipient authorization, shared atomic emission service and in-app adapter; `notification_api.go` owns snapshot list/count and personal state endpoints; `notification_preferences.go` owns versioned preference reads/writes; `notification_maintenance.go` owns startup/daily cleanup and coordinated shutdown. The serve command starts maintenance explicitly; offline commands do not. Payload expiry/caps and longer receipt retention preserve bounded storage and prevent retries from resurrecting read/dismissed/evicted records. See [notification contracts](NOTIFICATIONS.md) for exact retry/privacy/retention behavior. Financial producers, UI and external delivery remain follow-ups; MCP consent is unchanged.

The browser notification feature belongs in `web/src/features/notifications`: typed inbox/preference contracts, header count polling, the persistent centre and personal preference forms. App composes authorized budget/account navigation and the existing TransactionAccess loader handles transaction links. Settings keeps the preference feature mounted after first visit. Notification fixtures are opt-in for the dedicated browser spec and use only disposable synthetic databases.
