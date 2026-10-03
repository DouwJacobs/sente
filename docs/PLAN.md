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

Explicit account-scoped description rules offer categories. Review remains mandatory. Offer a rule when a reviewer changes a category; do not silently learn. Support individual and selected bulk approvals. All allocation categories must be present unless designated as a transfer. Financial edits return approved entries to pending.

## Budgets and ledger behavior

Categories are a flat list of expense/income categories. They do not belong to spending groups. Limits are per expense category, copied into new periods without rollover.

Spending groups are an independent, optional label on each parent transaction, separate from allocation categories. The same category can occur in different spending groups. Editable defaults: Day-to-day, Recurring, Invest-save-repay, Exceptions, Income, Transfer, Bank Fees, Communications, Debt, Utilities, and Insurance. Household members can add groups or edit names/colors; references use stable IDs and version checks. New imports and existing transactions remain unassigned until explicitly labeled. Changing a transaction's spending group returns it to pending review and is audited. Group names do not change ledger behavior: Transfer still requires explicit transfer designation, and categories determine income/spending and limits. All splits share their parent's spending group.

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

A provider-agnostic agent integration through an MCP server, with ACP considered for a future agent-session UI, is documented in [FUTURE.md](FUTURE.md). It should support transaction updates, financial overviews, and spending-based budget setup through the same authorized backend services as the UI. This is future work and does not expand the current MVP implementation.

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
