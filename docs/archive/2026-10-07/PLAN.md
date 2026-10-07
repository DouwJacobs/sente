> Historical record, archived 7 October 2026. This describes earlier development and includes superseded decisions. Use the [current documentation](../../README.md) for new work.

# Accepted MVP plan

Implementation authorized on 2026-10-02. This document supersedes the initial brainstorming proposals.

## Product

Self-hosted for a few invited users, initially the owner and spouse. Go + SQLite, React + TypeScript built with Vite, served through one Go HTTP process. Docker deployment on a home server behind an HTTPS proxy. ZAR and Africa/Johannesburg calendar dates.

Household accounts are explicitly selected and shared with all household members as editors. Private accounts use explicit viewer/editor grants and do not contribute to shared reporting. Administrators manage users, accounts, and grants; both household members manage limits and period dates.

Dashboard includes pending transactions, clearly labeled, category limits, spending, income, review counts, and dated bank-reported balances. Balances are imported snapshots and do not imply complete import coverage.

## Imports and review

Accept FNB ZIPs and standalone CSV/OFX, including the supplied CSV and OFX 1.02 SGML shapes. Adapters feed a normalized staging pipeline; account and currency checks precede commit. Limit uploads to 20 supported files, 25 MiB uploaded bytes, 100 MiB ZIP expansion, and 100,000 rows per file.

Preview row errors and duplicates. Valid rows can be imported after explicit confirmation despite rejected rows. Hashes block exact re-imports; account-scoped FITIDs block exact transaction duplicates. Conflicting IDs require inspection and skip. Date/amount/description matches require explicit keep/skip choices, including cross-format imports and within-batch candidates.

Discard original uploaded bytes after staging. Retain normalized source provenance, file hashes, row errors, and decisions. Never store personal exports in the repository.

Explicit account-scoped description rules offer categories. Complete category allocations are accepted automatically; missing categories require review. Offer a rule when a user changes a category; do not silently learn. Track seen/unseen separately per user, with individual and selected bulk actions. Explicit transfers do not require categories. Edits recompute acceptance and invalidate previous seen versions; saving marks the current version seen for the editing user.

## Budgets and ledger behavior

Categories are a flat list of expense/income categories. They do not belong to spending groups. Limits are per expense category, copied into new periods without rollover.

Spending groups are an independent, optional label on each parent transaction, separate from allocation categories. The same category can occur in different spending groups. Editable defaults: Day-to-day, Recurring, Invest-save-repay, Exceptions, Income, Transfer, Bank Fees, Communications, Debt, Utilities, and Insurance. Household members can add groups or edit names/colors; references use stable IDs and version checks. New imports and existing transactions remain unassigned until explicitly labeled. Changing a transaction's spending group is audited and invalidates prior seen versions; category completeness determines acceptance. Group names do not change ledger behavior: Transfer still requires explicit transfer designation, and categories determine income/spending and limits. All splits share their parent's spending group.

Transaction category selection supports search, account-authorized most-used categories, and explicit category creation. No new categories or labels are inferred from transaction descriptions.

Default periods start on the 20th; start days beyond a month's length clamp to the last day. Period dates are independently editable and inclusive. Latest start wins overlap; highest period ID breaks a same-start tie. Transactions in gaps await assignment. Explicit manual assignments survive date edits and are flagged if outside dates; explicit outside assignments remain outside budgets.

Preview reassignment before creating or editing periods. A preview token covers proposed dates and existing period/transaction versions; stale previews cannot be saved.

Money is integer cents. Signed same-sign allocations must total the parent transaction. Cash movement counts parent transactions once. Refunds reduce expense-category spending. Mark/link account transfers, excluding principal from income/spending; fees are separate expenses. Counterpart details obey account permissions.

## Authentication, deployment, and recovery

First-run browser onboarding creates the initial administrator only while no administrator exists. Setup atomically creates the administrator, household membership, and session; subsequent account creation requires an administrator. Disabled administrators do not reopen setup. No public registration or email dependency. Passwords are 12–72 bytes, hashed with bcrypt. Cookie sessions expire after seven days; mutations require CSRF tokens and allowed origins. Administrators recover passwords offline; users can change their own password.

One persistent SQLite database, foreign keys, WAL, short transactional writes, migrations, exclusive process lock, and optimistic versions. Consistent VACUUM INTO backups checked for integrity, at least once per 24 hours while serving, retaining 14 snapshots. Offline restore validates schema/integrity, preserves the previous database, and invalidates restored sessions.

## Acceptance

Backend tests cover duplicate/conflict and concurrent import handling, invalid rows, account mismatches, unauthorized reads/writes/revocation, exact money, split totals/review, refunds/transfers, pending totals, periods/assignment and preview staleness, authentication/CSRF, and backup restore/retention.

Browser checks cover account/user setup, limits, individual and bulk review, split editing, classification rules, upload, period preview, themes, narrow-screen overflow, and runtime errors using synthetic data.

User selected local-first deployment; retain Docker and PUBLIC_URL for a later reverse proxy. FNB FITID stability needs a second overlapping OFX export; the supplied matching CSV and OFX confirm equivalent rows but do not establish cross-download ID stability.

## Outside MVP

Other banks and custom mappings, external AI classification, FX conversion, investments, offline synchronization, and forecasting. Preserve the importer extension boundary for future adapters.

## Accepted future direction

Provider-agnostic agent integration through an MCP server was authorized on 2026-10-04 and is implemented as an expansion below. See [MCP.md](../../MCP.md) for the current authorized financial tools and exact approval workflow. ACP agent sessions and broader spending-based period setup remain future directions in [FUTURE.md](../../FUTURE.md).

## Implemented handover additions (2026-10-02)

Administrators can configure a workspace-wide display name, default Household, with separate optimistic versioning. All signed-in users see it; unauthenticated branding remains generic. This does not change member budget permissions. Budget limit editing can create a flat expense category inline while preserving draft limits; only explicit Save limits changes targets. Broader group budgets, forecasts, income targets and rebalance semantics still require product decisions.

Live FNB syncing is an authorized follow-up to this MVP, tracked in HANDOVER.md. The owner selected an independently deployed connector service outside agent host access. Current work provides mock normalization, source/dependency review and a read-only mock service only; live credentials, provider compatibility, mapping, staging integration and schedules are not implemented. The schedule default/frequency and fee/posting semantics still require explicit decisions.

Owner-operated FNB account discovery is the first live compatibility milestone: manual owner login in a temporary browser, metadata-only export and explicit administrator review/create in Accounts. No transactions, balances, credential storage or schedules are included. Same-host manual testing does not establish strict technical secrecy; the independent connector remains required for that guarantee.

## Accounts/balance connection update (2026-10-02)

Owner accepted same-host credential access limitations. FNB connection is per administrator, credentials encrypted with AES-GCM and owner-bound associated data, runtime key outside workspace/backups. Tracker can request automatic login, Accounts navigation and ledger-balance snapshots through a modern Node/Puppeteer subprocess. New full-number discoveries create private accounts with an owner editor grant; existing accounts require current editor permission, preserving sharing/grants. Explicit schedule intervals are off/6/12/24/168 hours, off initially. Schedule executes only while serving. Needs-attention failures suspend automatic attempts.

Hide/show persists by bank identity, excludes normal account/transaction/report views and connector balance updates, retains financial history and preserves permissions; future transaction adapters must use this exclusion before fetching. Disconnect deletes credentials and stops scheduling but keeps account history and hide preferences. No transaction fetching in this milestone. Owner live automatic login/balance compatibility is still unverified; no agent observation of banking.

Headless refresh is now the default for both manual/scheduled jobs. A saved connection debug setting shows Chrome for owner observation/approvals, with no extra sensitive logging. Missing balance fields are retried briefly, and failed layout/format extraction exposes only fixed numeric diagnostics. This supersedes the earlier always-visible manual-refresh behavior.

Configuration is now grouped under Settings subtabs: General, Banking, Accounts, Users & access, Backups, Security. Normal account views focus on authorized balances and link to account management. Settings organization does not expand account permissions or change bank/ledger behavior.

2026-10-02: dedicated-hostname reverse proxy deployment now includes explicit trusted proxy IPs/CIDRs for per-client login rate limiting, native bind-address configuration, Compose environment wiring and Nginx/Caddy templates. HTTPS cookie and Origin/CSRF behavior remains pinned to PUBLIC_URL. Subpath hosting remains unsupported.

2026-10-02: Administrators can configure public URL and trusted proxy addresses in Settings → Network. Save persists a workspace-wide override for the next restart; active request/session policies do not change until then. Use environment settings and an offline reset-network command recover environment defaults. NPM forwarding/SSL remains outside the tracker.


## Classification preparation (2026-10-03, simplified by owner)

Owner wants a simple rule editor before live FNB transaction fetching. Vault22 is a development reference only, not a transaction source. The earlier reference-upload/mapping/comparison workflow is retired.

Rules is the default Categories view. Normal controls are description contains, category and optional spending group. More options contains direction, priority and account restriction. New rules can apply to all current enabled editable accounts; identical definitions are grouped in the UI. Batch edits, pausing/resuming and deletion are atomic and version-checked without widening permissions. Adding future accounts does not silently grant an existing rule to them.

Rules support any/debit/credit direction and enabled state. Matching rules with different category/group outputs withhold suggestions for human classification. Existing committed records, category IDs, limits and transfer semantics stay authoritative. A Check an example action is optional. Live FNB transaction fetching follows rule setup.


2026-10-03 starter classification complete: once-only migration 9 includes general categories and editable/pauseable built-in rules; custom rules override defaults. Transaction category creation by typing and explicit Enter/Create is supported, as is atomic opt-in saving of account-specific description/category/group/direction rules during editing. Live FNB transaction fetching remains the subsequent milestone. No CSV transaction source is implemented.

## Live FNB transaction preview (2026-10-03)

First transaction delivery: explicit Fetch FNB transactions in Imports stages recent posted rows for the connection owner's previously discovered, visible editable accounts. Existing staging/rules/duplicates/confirmation/pending-review semantics apply. Immutable run/reference/coverage metadata persists; references are not assumed FITIDs. Identical whole snapshots reuse their durable import. Always show incomplete-history messaging and 150-row truncation signals, including pages containing excluded pending authorizations. Bank logout must confirm before staging. Unknown layouts/status/currency and nonzero embedded fees fail closed pending compatibility/fee representation review. Owner live verification is outstanding. Schedules remain accounts/balances only; automatic transaction policy, pagination, historical checkpoints and stable identifiers are future work.

2026-10-03 fee acceptance: owner confirms ledger movement equals bank Amount minus Service Fee. Live normalization preserves the principal amount and adds one separate negative Service Fees entry for a nonzero nonnegative fee, preserving linked source metadata. Both entries use normal rules/duplicates/mandatory review. Coverage counts bank rows separately from additional fee entries. Combined exports produce candidates for both components. This supersedes the initial nonzero-service-fee block; malformed/negative fees remain rejected. Fetch iterates all eligible accounts, not just the first.

## UI backlog completion (2026-10-03)

Owner authorized implementing all recorded UI tasks. Accounts and Settings → Accounts share editing, hide/show/restoration and scoped refresh controls; Banking focuses on connectors. Shared accessible loading and native disclosure controls apply across workflows. Large lists and lookup selectors use bounded server pages with permissions/search/order before paging, snapshots for changed lists, retained drafts/selections and full-dataset aggregates. Page-wide and single-account balance refresh honor current mapped editor scope, serialized jobs and hidden exclusions. Import confirmation and mandatory review remain explicit. Scheduled transaction retrieval and broader budget-reference product decisions remain outstanding.


2026-10-03 home-loan compatibility: owner identifies the third account as a ZAR home loan. On-demand transaction normalization explicitly supports Home Loan alongside Cheque/Savings/Credit/Easy when the same verified posted table contract holds; preserve original signed amounts and ordinary classification/review. Unknown types and non-ZAR still reject atomic staging.


2026-10-03 product alias: owner confirms Money Maximizer is a ZAR savings account. Its Type label (including Maximiser spelling) maps to the existing Savings contract; product aliases do not change monetary, posting, review or currency requirements.


2026-10-03 home-loan table contract: support owner-observed Effective Date/Description/Amount/Balance loan history without status toggles, scoped to verified Home Loan type and exact named columns with no pending evidence. Preserve original signs and standalone fees; ordinary confirmation/review and incomplete-history coverage apply.


## Cohesive workspace redesign (2026-10-03)

Owner accepted the proposed import/review consolidation and authorized page redesign once the concurrent polish chat finished. Transactions now contains All transactions, Needs review and Import activity; separate Review/Imports navigation entries are retired. Keep fetching/staging, explicit ledger addition, and mandatory approval as separate financial states within one guided experience. Group FNB activity by immutable original run and show authorized account identity prominently. Ready account imports may be explicitly added in batches before continuing to review; failure leaves successful additions durable and remaining previews available. Import-specific review clears unrelated date/account filters.

Accounts owns discovery-file import, manual creation, names/sharing/hide/show, dated balances and refresh actions in a compact list. Banking owns credentials/scheduling/diagnostics with links to Accounts/Transactions. Settings → Accounts is a shortcut. Categories uses clear Categories, Spending groups and Automatic rules sections, starting with Categories. This supersedes the earlier Rules-default presentation only; classification, money, transfers, duplicates, permissions and approval semantics remain unchanged. Scheduled transaction fetching remains outstanding.


2026-10-03 masked credit-card discovery: resolve full identity from unique Credit detail fields, checking visible masked digits, ZAR and duplicate identities, before account discovery. Temporary navigation aliases never become persisted bank IDs; hidden-account, authorization, explicit confirmation and review requirements remain.


2026-10-04 owner workflow/copy correction: clean bank downloads and uploaded statements are imported automatically with rule suggestions and mandatory pending review. Import activity records history and exceptions; possible duplicates, invalid rows, stale previews and rule-version changes still require explicit resolution. This supersedes the earlier separate confirmation requirement for clean imports. Review checks categories/spending groups before approval; importing never approves transactions. Existing staged imports are processed once per visit to Import activity using the same server commit validations. Plain language uses Get transactions, Transactions from FNB, categories and spending groups. Shared empty states separate actions from descriptions, single-page pagination is hidden, and native dialog dimming is neutral in both themes.


2026-10-04 owner decision: exact unique masked summary numbers may identify Credit accounts when the detail page confirms the same mask and product. This supersedes the full-number requirement for masked-on-both-pages cards; preserve visibility/editor permissions, FX rejection and collision checks. Do not invent digits or merge changed masks.


2026-10-04 transaction review follow-up: owner requests Save and approve directly from editing, stable selectors with appropriately sized dialogs, and an editable proposed contains-text category rule. Save and approve is an explicit atomic edit/approval action; Save changes alone still returns the transaction to pending review. Rule suggestions are opt-in, account/direction scoped, and never approve future transactions.


2026-10-04 transaction filters: owner requests category and related filters across every Transactions tab, especially Uncategorized. Implement common category, spending-group, money-direction and description controls with authorized server filtering before pagination/counts. Category matches any split once; Uncategorized means any missing allocation category on a non-transfer. Money in/out uses signs, without inferring income or expenses. Import activity finds matching imports using current committed ledger classification or current staged rules; exception decisions and original source details retain full rows.


2026-10-04 pending rule application: owner authorizes transaction-created rules to categorize existing matching uncategorized transactions awaiting review. Saving an opted-in rule from the transaction editor applies it to all eligible matches in that same editable account, irrespective of the current filters/page. Preserve assigned categories, all splits, transfers, approved entries and existing spending groups. Fill a missing group when the rule supplies one. Conflicting rule outputs stay uncategorized for manual review. Rule creation/edit, source transaction save/optional approval and pending applications must succeed or roll back together; only the explicitly edited transaction may be approved. This scope does not add retrospective application to standalone rule management or run all older rules as a migration.


2026-10-04 owner acceptance/seen policy: supersedes mandatory approval and save-only pending behavior. Complete categorized allocations automatically accept ledger entries on import, edit and transaction-created rule application; explicit transfers retain their category exemption. Missing categories (including incomplete splits and rule conflicts) stay in Needs review. Saving an edit marks its current version seen for the editing user. Imports/rule-applied targets begin unseen. Seen/unseen is personal per user and transaction version, independent of acceptance/reporting, with authorized atomic 1–100 selection actions and ledger seen filters. Later transaction changes invalidate old seen markers; opening a dialog alone does not mark seen. Existing explicit approved reviewers retain seen markers on migration; existing categorized pending rows are accepted without claiming human review, and acceptance changes are audited. Migration 10 is once-only and preserves money, allocations, groups, private account access and financial source data. Private accounts remain outside shared budgeting. UI uses Accepted/Needs category and Mark seen/unseen rather than manual approval; Save changes is the single editor save action.

## MCP finance access (2026-10-04)

Owner explicitly expands scope to an MCP server, Settings connection/setup instructions, privacy-filtered transaction/category/budget reads and agent-assisted bulk categorization, accepted entries, rules/categories and transaction/budget editing. Implemented Streamable HTTP at /api/mcp with hashed, expiring, revocable per-user bearer tokens, read-only by default. Write-capable tokens prepare exact one-hour proposals; only the initiating user's cookie/CSRF browser approval enables atomic, version-checked application. Acceptance follows category completeness, never agent confidence. Bank identifiers/credentials/source references/provenance/notes are excluded; generic account labels and description number/contact redaction apply. See MCP.md for the boundary and client setup.


## Browser-authorized MCP connections (2026-10-04)

Owner authorized replacing manual MCP token setup with browser OAuth. Settings → MCP now shows the endpoint/copyable agent instructions, own connected agents (read-only or proposal-enabled, last use, expiry, revoke), and exact change proposals. Connection approval requires tracker sign-in, confirms the approving username and unverified client/callback origin, and defaults to read-only. The optional proposal checkbox grants only requested scope; exact financial changes still need separate proposal approval. Consent uses shared Form/Button controls, neutral login panels and persistent overlay feedback; no manual token form or secret display remains. Migration 12 adds OAuth metadata/registration, S256 PKCE authorization, one-use codes and endpoint-bound expiring opaque access/rotating refresh credentials; refresh reuse revokes the connection. Browser session binding, CSRF, current enabled-user/account permissions and proposal/version checks enforce isolation. Legacy manual credentials remain revocable until expiry, without a new generator. Password change/reset, disable and restore invalidate agent access. See MCP.md for the full endpoint/lifecycle contract and VERIFICATION.md for actual coverage. Pasting a URL requires a client with remote MCP/OAuth support and a reachable HTTPS public URL outside localhost; no universal chat installation or live external-client compatibility is claimed.

2026-10-05: Owner requested Network settings updates and browser-requested restart while exposing the development app for MCP. Administrators can save, then restart from Settings → Network. Restart applies the saved policy through a drained backend lifecycle; the development supervisor restarts Vite with the saved exact hostname allowlisted.


2026-10-05 MCP setup simplification: owner requested sharing only `/api/mcp`, with instructions available to the receiving agent. Removed the Settings setup disclosure/copy-prompt button; the shared endpoint field/copy button, connected agents and change proposals remain. Ordinary web GET/HEAD returns public plain-text setup and OAuth discovery instructions. MCP/SSE/JSON/bearer protocol requests retain authentication, browser connection consent, current user/account isolation and separate exact financial change approval. The link contains no private records and does not grant access or promise installation in clients without remote MCP/OAuth support. See MCP.md and VERIFICATION.md for negotiation and actual checks.


2026-10-05 owner requested cohesive Dashboard/Budgets presentation and clarified group/category budgets. Limits are per period, spending group and expense category; a group inherits the sum of its category limits. The same category can have independent limits in several groups (for example Eating out: Day-to-day R1,000 and Exceptions R300). Overall category/period budgets sum these limits once. Existing limits migrate to No spending group without guessed distribution. New periods copy only category entries marked for upcoming budgets. Dashboard exposes group and category budget/actual/remaining/progress comparisons, with visible over-budget/no-budget states, bounded disclosures and missing classifications. Budget editing selects the group; cards link to that period's household spending. Categories remain flat and transaction-group lineage, money, transfers, refunds, permissions and seen semantics stay authoritative.


2026-10-05 owner automatic FNB policy: scheduled runs for mapped visible editable accounts now refresh balances and automatically import overlapping posted history using account-scoped source occurrence matching; manual Get transactions uses the same server commit. Rules classify automatically; complete categories accept unseen, missing categories alone enter category review. Import validation/ID conflicts remain separate Import activity exceptions. Reference alone is not an identifier; incomplete bank page coverage remains explicit. See ARCHITECTURE/HANDOVER for fallback and new-discovery limits.


2026-10-05 owner requested building each period's budget by explicitly adding groups and then categories, rather than listing every category in every group. One-time category entries default to the current period only. Upcoming entries use the chosen amount in existing later-starting budgets only where the same group/category is not included, preserve existing amounts, and carry forward when new periods are created. Removing entries changes limits/membership only, preserving category and transaction history.


2026-10-05 dashboard budget connections completed: selected period carries between Dashboard/Budgets, and Budgets keeps its selected card visible even outside the current list page. Dashboard Edit budgets opens the displayed period directly. Expanded category rows show all-groups spending and the combined budget (sum of independent group/category limits, not a new shared allowance). Native group/category transaction disclosures read bounded spending pages in-place, reuse the shared transaction editor, refresh figures after saves and retain open group/category disclosures. Aggregate category totals also support in-place transaction disclosure. Stable group/category IDs and exact period/account scope preserve lineage; missing classifications remain inspectable. No production deployment.


2026-10-05 transaction transfer control: owner removed the separate transfer checkbox. In the shared transaction editor, selecting the group named Transfer (case-insensitive) sets the existing explicit transfer flag on save; choosing another group or Not set clears it. Current Transfer-group entries gain the flag when saved. Existing explicit transfers retain their status until the owner changes the group; opening alone and group renames do not rewrite ledger records. Linked transfers require unlinking before selecting a non-transfer group, including inaccessible counterparts. Category amounts/splits remain exact; transfers remain category-exempt and excluded from income/spending. Picker selection carries the chosen row metadata across server pages. This supersedes the earlier editor requirement for a separate explicit transfer checkbox; import/rule/API classification is unchanged. No financial migration or production deployment.


## Core improvements authorised 2026-10-05

The owner authorised implementing the five-phase Vault22 core finance plan after reference review. The accepted scope and constraints are in [ARCHITECTURE.md](ARCHITECTURE.md). Source now includes filtered review navigation with draft protection, atomic bulk classification/labels, transaction notes and account-scoped merchants/tags, import-health timestamps and run summaries, shared period navigation, budget sorting/daily guide/rebalance, period/year trends and protected CSV exports, category archive/restore and independent merchant rules. Schema 16 is additive. MCP grants remain unchanged and omitted metadata is preserved. Production deployment remains separate; verification is recorded in VERIFICATION.md.


2026-10-07 audit repair scope authorized by owner: fix all nine findings summarized in VERIFICATION.md. Categories remain flat; MCP uses independent group/category transaction, rule and budget scopes with exact approvals. Repair Search through shared UI conventions, consolidate bounded audited ruleset import/export and update current regression contracts. Production deployment is separate.
