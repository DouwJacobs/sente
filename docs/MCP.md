# Agent access

Settings → MCP shows the public endpoint (`PUBLIC_URL/api/mcp`), connected agents and change proposals. The server runs inside the existing Go service; no second application or model API key is needed.

1. Share the MCP endpoint from Settings → MCP with your agent. Ordinary link reads return public text containing setup instructions, the same endpoint and OAuth discovery URLs. The agent can configure its supported remote MCP/OAuth integration without a separately copied prompt; clients that cannot add connections from chat still need their connection settings.
2. The client discovers OAuth metadata and registers its callback. It opens your browser to Sente sign-in and connection approval. Check the signed-in username, supplied agent name and callback origin; client names are not verified identities.
3. Choose Read-only (default), Categorisation proposals, Finance editing proposals, or custom permissions within the client’s requested scope. Denying creates no connection. No token needs to be copied or pasted. Server instructions arrive through MCP initialization after authentication.
4. Settings → MCP lists your own connected agents, capability and account-scope summaries, last use and expiry. Edit permissions requires explicit browser confirmation; a read-only connection must reconnect to obtain proposal scope. Revoke ends all credentials for that connection and removes its proposals. Separate users and separate approvals have independent connections; current tracker permissions determine accessible data.
5. For proposed changes, refresh proposals, inspect exact before/after values, then approve or reject. Approval itself does not change financial data. The agent applies the approved proposal once. Explicitly enabled automatic approval can approve matching change types; all other proposals need your review.

A remote agent needs a reachable HTTPS `PUBLIC_URL`. HTTP is allowed only on localhost/loopback for local development. Proxy `/api`, `/oauth` and `/.well-known` to the application, with `/mcp/authorize` served by the frontend. Vite proxies all three API paths in development. Browser consent uses the existing sign-in session and CSRF protection; the agent uses opaque bearer credentials managed by its OAuth client.

## OAuth contract

- Protected resource metadata: `/.well-known/oauth-protected-resource/api/mcp` (also root alias). Unauthenticated MCP protocol requests return 401 with `WWW-Authenticate` pointing here.
- Authorization server metadata: `/.well-known/oauth-authorization-server`.
- Dynamic public-client registration: `POST /oauth/register`, `token_endpoint_auth_method=none`; authorization code and refresh grants. HTTPS callbacks or HTTP localhost/127.0.0.1/::1 callbacks only, exact registered URL matching, no userinfo/fragments. No client metadata URL fetches or secret-bearing clients are supported.
- Authorization: `GET /oauth/authorize`; `response_type=code`, registered client/redirect, `resource=PUBLIC_URL/api/mcp`, scopes and S256 PKCE required. `finance:read` allows reads; `finance:propose` additionally allows proposals and separately approved writes. Read-only writes return a 403 insufficient-scope challenge for a new browser authorization.
- Consent request expires after 10 minutes and binds to the first viewing user's current browser session. Decisions are single-use. Callback echoes state and includes issuer (`iss`). Codes expire after 5 minutes, are single-use and bind client, redirect, S256 verifier, endpoint resource and connection.
- `POST /oauth/token` uses URL-encoded authorization_code or refresh_token grants with client_id/resource. Access tokens expire after one hour; refresh rotates on every use. Reusing a consumed refresh token revokes that entire connection. Scope cannot expand or change during refresh; request a new browser authorization instead.
- Connections and refresh credentials expire after 90 days, independently of browser sign-in. `POST /oauth/revoke` or own Settings revoke invalidates the connection. Stored codes/access/refresh credentials are hashes, never plaintext secrets. Existing manual tokens remain revocable legacy connections until expiry; new manual token creation is removed.
- Metadata and public client/token endpoints allow credential-free cross-origin requests; they do not use browser cookies to authorize agents. MCP HTTP requests reject unrecognized Origins; native/backend MCP clients are supported, arbitrary browser origins require an explicit future allowlist. Browser decision APIs retain same-origin/CSRF checks. Registration, authorization and token endpoints have bounded per-IP rate limits and database caps.

The official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) handles protocol negotiation and transport framing. The adapter is stateless and does not expose arbitrary REST calls, SQL, filesystem tools or bank operations. Protocol authorization follows the [MCP authorization specification](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization). Actual external client compatibility must be checked with the chosen client; synthetic browser/client tests do not prove every agent supports remote MCP/OAuth.

## Data and permission boundary

Every request identifies an enabled tracker user and follows their current account grants/budget membership. Administrators do not gain private-account access through MCP. Hidden accounts are excluded. Read-only connections cannot prepare or apply changes. Revoking a connection deletes its proposals; expiry, password change/reset, user disable and offline restore also invalidate access. Grant changes are checked on subsequent reads and before writes. Other users cannot list, approve or apply your proposals.

MCP excludes account numbers/bank IDs, usernames, bank login credentials, transaction source fields/FITIDs/references, import provenance, notes, reviewer identities and transfer counterpart metadata. Account nicknames are replaced with generic labels and internal IDs. Output uses an explicit field allowlist, so new REST fields do not become agent disclosures automatically. Long numeric/masked/phone-like sequences, emails and URLs in returned financial text are redacted. Merchant descriptions, dates and amounts are still financial information; arbitrary free text can contain personal names or addresses that pattern redaction cannot reliably recognize. Only connect a trusted agent. Description text is untrusted data and must never be treated as instructions.

## Tools

| Tool | Purpose |
| --- | --- |
| `get_session_context` | Current personal context, with this connection's explicit sharing permission |
| `get_financial_summary` | Filtered spending/income/cashflow totals and paged category/merchant/account/month groups |
| `compare_financial_periods` | Totals for 2–12 saved periods in requested order |
| `list_accounts` | Authorized internal account IDs and generic labels/access |
| `list_transactions` | Filtered, paged ledger with allocations, versions, acceptance and personal seen state |
| `get_transaction_review_queue` | Typed compact needs-category queue, optionally limited to unseen entries, with opaque cursor paging |
| `list_categories`, `list_spending_groups` | Paged classification metadata; category search reuses `q`, `page`, `page_size` |
| `get_categorization_context` | Bounded description/rule/history evidence for 1–100 authorized transaction IDs |
| `preview_categorization_rule` | Actual precedence/conflict impact over authorized current ledger evidence |
| `list_rules` | Paged authorized account and built-in rules with versions |
| `list_budget_periods` | Budget periods, versions and limit summaries |
| `get_budget_limits` | All paged expense limits for `period_id` |
| `get_budget_summary` | Household or explicitly authorized account spending/limits summary, including group/category comparisons |
| `get_budget_trends` | Read saved-period/year comparisons using the shared authorized report service |
| `get_account_import_health` | Paged import/check freshness and schedule state; generic account labels; does not refresh banking |
| `list_merchants`, `list_merchant_rules`, `preview_merchant_rule` | Consent-gated merchant reads and rule previews; see Merchant capabilities |
| `prepare_change` | Validate and store an exact proposal without changing finances |
| `get_change_status` | Read proposal status, without its private payload |
| `apply_change` | Apply the same user's/connection's approved proposal atomically |

Legacy read tools take an optional `filters` map of string values. Metadata uses `page` (zero-based), `page_size` (1–100), `q`, `id`, `list_version`. Transactions use `offset` (100 per page), `id`, `account`, `period`, `pending=1`, `category=uncategorized` or ID, `spending_group=unassigned` or ID, `direction=in/out`, `q`, `seen=0/1`, `unassigned=1` and `list_version`. Summary supports `period`, `account`, `category_page`, `balance_page`. Reset paging when a list-version conflict occurs. Budget limits use `period_id` outside filters.

`prepare_change` supports these operations:

- `assign_categories`: 1–100 `categorizations`, each with `id`, current `version`, `category_id`, and explicit `allocation_id` for splits. Only fills missing categories; preserves allocation IDs, amounts, existing categories and notes. Several missing allocations in one transaction may be assigned atomically; the parent version increases once. Transfers reject.
- `set_seen`: `seen` boolean plus 1–100 distinct `seen_items` (`id`, current financial `version`). Requires the custom seen capability and account viewer access. Exact approval captures personal seen state; concurrent seen/financial changes reject. Does not change financial versions or other users’ markers.
- `edit_transactions`: 1–100 `edits`, each with `id` and current `version`. Partial fields include date, signed amount_cents, description, category_id for a single allocation, explicit allocations for splits (omitted notes retain the existing note at the same allocation index), spending_group_id, clear_spending_group, is_transfer, assignment and period_id. Omitted fields preserve original values, including hidden raw descriptions/notes. Amount-only edits keep a single allocation in balance; splits require explicit allocations. Bulk edits are atomic and duplicate IDs are rejected.
- `create_category`: `category` with name and kind (`expense`/`income`). Membership and duplicate-name checks apply.
- `save_rule`: `id=0` creates; existing IDs require the rule's current version. `rule` uses account_id, pattern, category_id, optional direction/priority/spending_group_id/enabled/version. Pausing uses enabled=false. Negative built-in IDs follow membership/version checks. Rules classify future imports; this operation does not sweep existing entries. Use explicit bulk transaction edits for existing matches.
- `delete_rule`: id and version.
- `update_budget`: `id` is the period ID; `budget` includes its current version and targets (category_id, nonnegative amount_cents). Group scopes and merge/replacement behavior follow the exact budget contract below. Period creation/date changes and default start-day edits are not exposed.

All money is integer ZAR cents. Split sums and signs must match the parent exactly. Source fields remain immutable. Transfer links retain existing constraints. Classification completeness determines Accepted; missing categories remain Needs review, and an agent cannot override this rule. Saving marks the editing user's new version seen; other users' old seen versions become stale.

Proposals expire after one hour and are capped at 20 active proposals per connection. Browser approval uses normal session/CSRF protection and authorizes the immutable stored payload only. Application rechecks current permissions and versions, and commits financial writes, agent audit, status and replay result together. Failed/stale application rolls back all financial changes; prepare a fresh proposal after reloading. Applied replays return the saved result without repeating writes. Connection identity and proposal ID supplement normal financial audits, without logging credentials or financial text.

The server supports edits, categorization and limits; it does not delete transactions, change account sharing, fetch banking credentials, trigger bank imports, manage users, restore backups or provide unrestricted commands. ACP and embedded agent sessions remain future work.


## Setup through the endpoint link (2026-10-05)

Settings retains the endpoint/copy action, connected agents and exact change proposals; the setup disclosure/prompt-copy button is removed. Opening `/api/mcp` as an ordinary web link returns HTTP 200 UTF-8 plain-text instructions with no user/session/financial records. This uses GET/HEAD without an Authorization, MCP-Protocol-Version or MCP-Session-Id header, and an Accept header without application/json or text/event-stream. HEAD omits the body. Public text includes the configured endpoint, metadata URLs, browser sign-in/approval, read/proposal scopes, privacy limits and exact change review workflow; request queries, cookies and Host are never reflected.

MCP GETs request SSE and retain authentication; all POSTs, protocol-marked GETs, JSON GETs and bearer-bearing requests retain OAuth challenges/validation. Foreign Origins still reject. Responses remain no-store and public setup varies on Accept. This preserves the [Streamable HTTP transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports): protocol messages use POST and streaming GETs request text/event-stream. Instructions are also delivered by MCP initialization after sign-in. Reading a link grants no connection or financial access and cannot install MCP support into a client that lacks it.


2026-10-05 OAuth client compatibility: an external client repeated the same resource parameter in its authorization URL. Authorization and URL-encoded token/revoke parsing now normalize repeated identical resource values, following RFC 8707's resource parameter exception. The MCP audience still must exactly match the configured endpoint. Different/empty resource values and all repeated client_id, redirect_uri, state, PKCE, scope, code and grant parameters reject before processing. Code, refresh, user/session and financial permission boundaries are unchanged. Regression fixtures are synthetic; no real client IDs, states or PKCE values are saved in source/docs.


## Typed transaction reads and review queue

`list_transactions` keeps the existing `filters`, `offset` (100 per page), and `list_version` contract. These additional fields can be supplied as typed top-level arguments or inside the legacy string map:

- `date_from`, `date_to`: inclusive local calendar dates in `YYYY-MM-DD` format, without UTC conversion. Either bound may be omitted.
- `category_ids`: a typed array of 1–100 distinct positive IDs; a comma-separated string in the legacy map. Matches **any** selected category, counting a split parent once. Cannot be combined with legacy `category`.
- `categorization`: `categorized`, `uncategorized`, or `any`. Uncategorized means a non-transfer with any missing allocation category (or no allocations). Explicit transfers are category-exempt and match categorized.
- `acceptance`: `accepted`, `needs_category`, or `any`, independent of personal `seen=0/1`. Legacy `pending=1` still means needs category.
- `min_amount_cents`, `max_amount_cents`: inclusive signed integer cents, including negative debits, positive credits and zero. Omitted bounds are unlimited. Dates, integer bounds, duplicate IDs and contradictory filters are validated; conflicting typed/map copies reject.

Example:

```json
{"date_from":"2026-10-01","date_to":"2026-10-31","category_ids":[1,2],"acceptance":"accepted","min_amount_cents":-50000,"max_amount_cents":-1,"filters":{"seen":"0"}}
```

`get_transaction_review_queue` requires `selector`: `needs_category` includes seen and unseen entries needing categories; `unseen` includes **only unseen entries needing categories**, as explicitly selected by the owner. Accepted unseen entries and transfers can be read through `list_transactions` with `seen=0`. No `under_review` alias is introduced.

The queue accepts the typed fields above, `account_id`, `period_id` (membership required), `q` (normalized literal substring), `limit` (1–100, default 50), and `cursor`. Accepted/categorized-only queue filters reject. Rows sort by descending date, then descending ID. The result contains `items`, full filtered `total`, `more`, `list_version`, and `next_cursor` (empty at the end). Pass the returned cursor with exactly the same selector, filters and limit:

```json
{"selector":"unseen","account_id":1,"date_from":"2026-10-01","limit":50}
```

Queue entries contain internal transaction/allocation/category IDs, version, date, redacted description, exact signed cents, `currency=ZAR`, generic account label/ID, minimal allocations, acceptance (`review_state`) and the current user's seen state. No notes, banking identifiers, source/provenance, nicknames or audit details are selected. The shared privacy allowlist/redactor remains in use.

Cursors are encrypted and authenticated with an ephemeral process key and bind to the user, connection, filters, limit and list snapshot. Authorization is checked again on every page. Financial-version, queue membership or personal-seen changes produce a stale-list error; altered/cross-user/cross-connection cursors reject. Restart paging without a cursor after these errors or a server restart. Cursors grant no access themselves. Counts, snapshots, page rows and allocations share one SQLite read transaction; predicates apply before counts and paging.

Indexes `transaction_account_date_id` and `allocation_category_transaction` support account/date/ID ordering and category filtering. Normalized substring searches and full-list snapshots can still scan; these indexes do not make all searches constant-time.

## Category context and rule impact

Use `list_categories` with `{"filters":{"q":"grocery","page":"0","page_size":"20"}}` to search existing flat categories before proposing a new one. The adapter uses the ordinary API paging path, including category-specific snapshot signatures and filters.

`get_categorization_context` example: `{"transaction_ids":[42,43]}`. IDs must be distinct and all currently accessible within the connection scope; the batch fails as a whole for an unavailable ID. Similarity is **same normalized description**, using the existing Unicode/whitespace normalization. Historical evidence excludes every ID in the requested batch. It counts distinct parent transactions per category, so a split parent contributes once to each applicable category. Includes all authorized matching history, with category counts for assigned allocations; no invented merchant identity, confidence, or historical transaction dump. Each target returns actual rule suggestions/conflicts, at most 10 matching rule summaries and 20 history categories, full counts and truncation flags. Active rules are cached per target account; more than 2,000 rules in an account rejects bounded evaluation.

`preview_categorization_rule` example:

```json
{"rule":{"account_id":1,"pattern":"market","category_id":2,"direction":"debit","priority":10,"enabled":true},"account_ids":[1],"sample_limit":10}
```

The explicit accounts must all be authorized; omitted account_ids defaults to rule.account_id. Optional `replace_id` evaluates replacement of an existing positive account rule in its own account. `sample_limit` is 1–50, default 10. Counts use SQL normalized literal contains/direction predicates before evaluation, then the actual precedence engine. A total above 10,000 matching ledger parents or more than 2,000 rules in an account rejects with a narrowing request. `description_match_count` is the broad pattern count; `future_rule_match_count` means the draft participates in current-ledger examples under actual precedence, and `future_classified_count` additionally excludes conflicts. These are evidence, not forecasts. `uncategorized_count`, `existing_eligible_count` and `conflict_count` are separate: eligibility follows the existing pending, non-transfer, single-null-allocation/equal-money rule service and current editor access. Bounded samples are redacted and contain no notes/source fields. Saving a standalone rule never sweeps existing transactions.

Narrow assignment example:

```json
{"operation":"assign_categories","categorizations":[{"id":42,"version":3,"category_id":2,"allocation_id":87}]}
```

Prepare, obtain approval for the exact proposal, then apply its proposal_id. Existing categorized allocations and transfers cannot be overwritten using this operation. Generic financial edits remain available only with the corresponding capabilities.

## Versioned connection permissions

`mcp_tokens.permissions` stores versioned JSON (`schema_version=1`); `permission_version` guards browser changes with optimistic versions. Migration maps legacy read-only/proposal-enabled connections to exactly their existing capabilities. Legacy finance grants and the Finance preset exclude independent seen changes and merchant capabilities; only custom consent can enable them. No usage quotas were added by owner decision. Existing expiry, rate limits, registration/connection caps and 20-active-proposal cap remain safeguards, not daily usage quotas.

- **Read-only:** authorized reads; no change proposals.
- **Categorisation proposals:** reads, missing-category assignments and constrained contains-rule creation. Requires explicit selected accounts; cannot recategorize or edit finances, existing rules or shared budgets.
- **Finance editing proposals:** existing assignment/recategorization, financial edits, category creation, rule create/edit/delete and budget-limit proposals, within current user permissions. Seen changes remain custom.
- **Custom:** independently select `assign_missing`, `recategorize`, `financial_edit`, `create_category`, `create_rule`, `update_rule`, `delete_rule`, `update_budget`, `change_seen`. Only supported category creation is exposed; category administration/deletion is absent.

Constraints are explicit `account_ids` (1–100 distinct currently accessible IDs), `selected_accounts`, `missing_categories_only` and `constrained_rules`. No selected IDs means all currently accessible accounts only when selected_accounts=false. Empty selected scopes reject. Account limits apply to **both reads and proposals**, including category popularity/history, scoped rules, review queues, balances and ledger budget aggregates. Shared category/group/period/limit definitions remain household metadata under normal membership checks. Shared budget writes and global built-in rule changes cannot fit a selected-account scope; budget editing requires all accessible accounts. Constrained rules permit creation only in explicit selected accounts using the existing nonempty 2–200-character contains pattern and normal category/group/direction/priority fields. Missing-only constraints reject financial edits, transfers and recategorization.

Every entry point rechecks grants/capabilities, including legacy `prepare_change` generic edits, exact browser approval, apply and stored-result replay. Generic no-op edits require financial-edit permission, preventing a category-only grant from being used solely to mark seen while preserving existing finance-edit behavior. Removed categorized allocations require recategorization permission. Automatic seen marking following a legitimate financial/category edit retains normal edit semantics; the custom capability governs independent manual seen actions.

OAuth consent defaults to Read-only and never silently grants proposal scope. Existing write-scoped connections can narrow or expand individual capabilities through signed-in, CSRF-protected browser consent in Settings; the original OAuth coarse scope remains the maximum ceiling. A read-scoped connection cannot upgrade there: start fresh authorization from the agent. Refresh cannot expand scope. Browser permission changes require exact displayed confirmation and optimistic permission_version; failures preserve the draft. Writes always use approved proposals; automatic approval requires separate explicit grants.

All read tool annotations are read-only and closed-world. Preparation is a write to proposal storage, non-destructive and not idempotent; application is potentially destructive and idempotent through stored replay. Annotations never authorize changes. Atomic `mcp.applied` audit records user, connection, proposal, time and durable before/actual-after evidence for every affected transaction, category, rule (including deletion), budget or personal seen marker. Audits remain inside browser/storage boundaries and are not agent read results or logs. Legacy deleted-rule replay resolves its original account from retained deletion audit when older snapshots omit it; current access still applies.

## Proposal recovery

Existing proposals retain their connection identity, approval, version and expiry checks. Use the original connection and an approved, unexpired, current proposal; otherwise prepare a fresh proposal. [VERIFICATION.md](VERIFICATION.md) records source checks and external limits, without claiming a production deployment or recovery of real proposals.

## Selected and automatic approval

Settings → MCP supports proposal checkboxes, Select all pending, Approve selected and Approve all shown. These actions submit explicit displayed proposal IDs, never proposals arriving later. Server batches allow 1–500 distinct own IDs and approve atomically: an expired, changed, revoked or unauthorized item rolls back every approval and audit. Approval itself applies no financial effects. The displayed list is bounded at 500 with deterministic ordering; bulk-all explicitly means shown pending proposals.

Under each agent’s Edit permissions (and browser connection consent), Automatic proposal approval offers only its granted change types. All existing connections and presets default to no automatic approval. The new optional automatic_approval capability map remains in schema-version-1 permission JSON, so no database version change is needed. It cannot exceed the corresponding grants, OAuth scope or account/operation constraints. Switching presets clears automatic choices; disabling a capability clears its automatic choice. Settings confirmation is reset after a draft change.

prepare_change returns approved for a new proposal only when **every** capability required by its exact effects is enabled for automatic approval. Generic edits cannot disguise financial/notes/group/period/transfer/split changes or recategorization as a category-only operation; mixed changes otherwise remain pending. New and edited rules are separate types. No direct-write tool or automatic application was introduced: the agent still calls apply_change. Existing pending proposals are not retroactively approved when automatic approval is enabled. Exact payload retains its automatic-approval basis, and approval attribution is audited atomically; application retains complete entity before/after evidence and once-only replay.

Removing an automatic type blocks any still-unapplied proposal relying on it; the agent prepares a fresh proposal for manual approval. Current grant/account/version/expiry checks still run. Already-applied replays return their original result after ordinary current-capability checks, without repeating effects. MCP initialization/tool/public setup instructions explain pending versus approved, so agents need not ask for manual approval again when the proposal is already automatically approved. Manual approval remains required for types without automatic approval.

## Budget contract

Categories are flat and do not own spending groups. Transactions and categorization/merchant rules retain independent category and spending-group fields. Aggregate category-budget reads sum independent period/group/category limits.

`update_budget` proposals accept an explicit root `group_id`/`spending_group_id`, per-target group IDs or `spending_group_name`, or `groups` with explicit group scopes and child targets. Group 0 explicitly means No spending group; omitted scope is only a compatibility path for legacy ungrouped limits. Names resolve to stable IDs during preparation; conflicting IDs/names and unknown names reject. An omitted target group rejects when that category already has a named-group budget. An omitted-scope whole-period replacement rejects once named groups exist. No category metadata, rule defaults or Day-to-day inference chooses a budget's group.

`merge=true` changes specified entries and retains other independent scopes. `merge=false` with an explicit root replaces only that group; explicit grouped/per-target whole-period replacement previews every removal. Empty explicitly selected groups persist. Retained entries preserve stopped/active carry-forward; new entries follow the shared compatibility writer. The browser's selective This budget only/This and upcoming builder and rebalance workflow remain browser-only; these are not additional MCP operations.

Preparation stores canonical IDs plus exact before/after group membership, group versions, category IDs/names, cents, inclusion and carry-forward. Undistributed legacy aggregate limits without canonical children appear as legacy_targets in the before-state and are preserved under No spending group before synchronization. Named/inactive canonical entries are never replaced by that compatibility path; partial writes retain unloaded limits. Settings → MCP displays both states before approval. Apply checks the current before state and actual after effects atomically, alongside current period version, consent, account constraints and automatic-approval rules. A stale context rejects without financial effects. Old unapplied budget proposals lacking exact grouped effects must be prepared again; already-applied results retain guarded once-only replay. This repair does not expand existing connection grants or automatic-approval types.


## Feature coverage

The following read fields and workflows are supported. New browser fields never enter MCP responses automatically; write capabilities remain separately consented.

- `list_categories` now retains `archived` and `version`; `filters.active=1` returns assignable categories. The shared writer rejects new assignments to archived categories.
- `get_budget_limits` accepts `filters.group` (0 = No spending group) and `filters.budget_only=1`, returns `carry_forward`, and retains exact group/category amounts. The default aggregate read remains compatible.
- `list_budget_periods` retains `group_budgets`. `get_budget_summary` retains `spending_groups`, their nested category comparisons and `group_total`; accepts `group_page` and `sort`.
- `get_budget_trends` supports filters `period`, `account`, `count` (1, 2, 6, 12), `year`, `category`, `spending_group`; returns periods, entries, totals and excluded_count. It uses ordinary budget membership, current account grants and selected connection scope.
- `get_account_import_health` supports page/page_size; returns generic account labels, balance date, last_imported/last_fetched, state, next_due, possible_gap and import_issues. No raw import provenance, bank IDs, worker diagnostics or credentials are exposed. Coverage stays conservative.

| Core feature | MCP coverage / deliberate limitation |
| --- | --- |
| Review navigation/draft protection | Browser interaction; existing queue, category assignments, seen proposals and typed filters support agent review. |
| Bulk categories/groups | Existing atomic exact transaction proposals; browser bulk preview tokens remain browser-owned. |
| Merchants / merchant naming rules / logos | Implemented through opt-in merchant reads and separate proposal capabilities; see Merchant capabilities below. Existing grants are preserved. |
| Tags | Gap: no tag read/proposal tools or MCP tag filter. Ordinary financial edits preserve tags. |
| Transaction notes | Intentionally excluded by the accepted privacy boundary; ordinary edits preserve them. |
| Import health | Read tool; banking/import execution remains owner/browser-only. |
| Group/category budgets | Explicit grouped limit proposals and exact effect previews supported; see the budget contract above. Selective one-time/upcoming builder and rebalancing remain browser workflows. |
| Period/year reports | Read tool. Browser CSV/ZIP downloads remain browser workflows; MCP can return the authorized report data. |
| Category rename/archive/restore | Archived state visible. **Write gap:** existing create_category consent does not authorize category administration; requires a distinct opt-in capability before adding writes. |
| UI layout/theme changes | No additional MCP operation needed; finance services and permissions are unchanged. |

No existing connection gains a new write capability or automatic-approval type. Synthetic tests cover selected-account/hidden-account boundaries, generic labels, archived/group budget reads and preservation/privacy of schema-16 transaction metadata. No external-client or production release verification is implied.

## Merchant capabilities
- Read tools: `list_merchants`, `list_merchant_rules`, `preview_merchant_rule`. Merchant reads require explicit `read_merchants` consent; logos are omitted unless `include_logo=true`. Items include associated `category_id`, `category_name`, `spending_group_id`, `spending_group_name`, and `spending_group_color` when set. Ledger reads include merchant ID/redacted name and logo presence, with a consent-gated `merchant` filter. Notes/tags/source data remain excluded.
- Proposal operations: `save_merchant` (create/rename/versioned logo update/removal, category/spending group assignment), `save_merchant_rule` (create/edit/pause/global or account scope, regex/alternation patterns), `delete_merchant_rule` (permanently delete by id/version), `assign_merchants` (1–100 versioned transaction assignments/clear). Use existing prepare/approve/apply workflow; reads never write. Saved-rule previews leave named entries untouched; assignments are explicit and preserve money/categories/splits.
- Separate change capabilities: `manage_merchants`, `manage_merchant_rules`, `delete_merchant_rule`, `assign_merchants`; automatic approval is opt-in per type. Combined rule/logo changes require both grants. No new grants are added to legacy or finance presets. Current connection consent, account grants, versions and automatic approval are rechecked at apply and replay. Global merchant/catalogue writes require unrestricted connection account scope and budget membership; restricted connections can propose permitted account-level changes only.
- Permission modal groups read/scope, allowed change types and optional automatic approval; logo proposals show the actual image with concise JSON rather than the full encoded data. Existing connections must explicitly enable merchant access in Settings → MCP; read-only OAuth connections must reconnect with proposal access for writes.
- Dashboard summary additionally exposes bounded `income_categories`, `income_category_total` and accepts `income_page`; no new permissions are needed for these existing budget-scope reads.

## Portable configuration boundary

Browser configuration import/export and public repository pulls require administrator access with budget membership. They share the existing authorized catalogue/rule writers and audits with offline imports and MCP entity writes. Bulk source import/export is browser/host only; no agent tool, input field, output allowlist, permission or automatic approval type is added. Existing connection consent does not authorize bulk configuration imports.

### Account security lifecycle (2026-10-07)
User deletion and administrator password reset are browser administration workflows; MCP has no user/password tools or new grants. Self-service password change retains its initiating browser session and revokes every other session and MCP connection. Administrator reset revokes all target sessions/connections, preserving disabled status. User deletion retires the historical identity, removes grants/sessions/personal seen state/FNB credentials, and retains financial history and audit attribution. Every path removes user-bound pending OAuth consent as well as connections; existing cascades remove proposals, codes and access/refresh credentials. A client must reconnect with fresh browser consent after a password change/reset. Offline password recovery uses the same revocation service. No password material enters proposals, audit details or response allowlists.

## Application version metadata

MCP initialization advertises the same application version as the backend/browser through `internal/buildinfo`, replacing the independent hardcoded server version. Server identity and protocol negotiation are unchanged. No tools, input schemas, financial output allowlists, permissions, consent, proposals or audits change. The detailed build endpoint and About/report links are browser-only support features; they do not need a new agent capability.


## Personal context

Settings → MCP has one personal context field per user, up to 6,000 Unicode characters. Browser GET/PUT `/api/mcp/context` uses the current signed-in user, CSRF, optimistic versions and a serialized audited write; no arbitrary user ID is accepted. Schema 22 adds `mcp_user_context`. Database backup/restore retains context; user deletion removes it. Configuration exports exclude it. Audits contain only action, version and character count, never the prose.

Each connection needs explicit `read_context` consent in OAuth or Settings. This independent read flag defaults false for existing connections and new presets; changing proposal presets retains explicitly chosen read permissions. It grants no changes or automatic approval. Disabling sharing takes effect on subsequent reads/initializations.

Authenticated initialization includes current shared context in a per-request copy of server instructions. Shared server options and public setup text never contain context. `get_session_context` returns `{shared, context, version}`; sharing off returns false, empty text and version 0 without reading the stored prose. Instructions ask clients to refresh context at the start of every new conversation, including reused connections. Sente delivers it on initialization; client obedience/use cannot be enforced.

Context is deliberately shared as written, unlike redacted financial descriptions. It is user-provided guidance and cannot override account access, financial invariants or proposal approval. Include preferences and useful background rather than credentials.

## Financial aggregates

`get_financial_summary` uses typed `account_id`, `period_id`, inclusive `date_from`/`date_to`, `category_id`, `merchant_id`, normalized description search `q`, `group_by` (none/category/merchant/account/month), and zero-based `page`/`page_size` (1–100, default 50). Invalid IDs/dates/groupings reject. Merchant grouping/filtering needs `read_merchants`. Current grants, connection account scope and hidden-account exclusion precede all queries in one SQLite snapshot.

Results expose only currency, typed overall totals, bounded group items and paging metadata. Totals cover the complete matched scope regardless of paging. Allocation category kinds determine income/spending; transfers contribute neither, and expense refunds reduce spending. Category filters sum only matching allocations. Cash in/out/net count each matching parent once, including transfers; category filtering selects matching parents but retains their whole cash movement. Distinct-parent group counts must not be summed across categories when a split spans groups. Month IDs/labels stay exact, account labels are generic, and category/merchant names are redacted. No raw transactions, notes, logos, banking identity or source data are returned. Sum/net overflow rejects rather than rounding.

A saved period without an account uses household assigned-period scope, excluding private accounts. An explicit private account uses period dates; date filters can narrow either scope. With no period, all authorized enabled accounts or the selected account use the supplied calendar dates.

`compare_financial_periods` accepts 2–12 distinct positive `period_ids` and optional account/category/merchant/query filters, preserves requested order and uses the same arithmetic in one snapshot. Unknown periods or access errors reject the whole result. Existing `get_budget_summary`, `get_budget_limits` and `get_budget_trends` remain authoritative for budget status and limits. New aggregate reads add no write permissions or automatic approval types.

Review queue allocation hydration uses one query for the already authorized returned page, none for an empty page. Allocation-ID order, cursor/version, privacy and output fields remain unchanged; lookahead and unrelated entries are not hydrated.

## Browser-only notification infrastructure

Notification inbox/preference APIs are authenticated browser workflows. Notification messages, source references, preferences, receipts and account dependencies are not MCP tool output or saved session context. Existing connection consent does not expand to personal notifications. Future MCP exposure needs explicit consent and current recipient/account/source checks; the shared financial writer may eventually emit via notifyTx inside its existing SQL transaction without introducing a separate MCP mutation path. This foundation adds no tools, schemas, allowlist fields, capabilities, proposal previews or financial/audit contract changes.
