# Product behavior

Sente is a self-hosted household finance tracker for a few invited users. It uses ZAR and Africa/Johannesburg calendar dates. This guide describes accepted behavior; unresolved directions belong in [FUTURE.md](FUTURE.md). Earlier decisions are retained in the [archive](archive/README.md).

## Accounts and access

Administrators manage users, accounts and grants. Household accounts are explicitly shared with household members as editors. Private accounts require viewer/editor grants, including for administrators, and stay outside shared budgeting. Every read, write, import, export and aggregate checks current access. Hidden accounts retain their history and grants but are excluded from normal views and syncing.

First-run setup creates the initial administrator, household membership and session together. Disabled administrators do not reopen setup. There is no public registration or email dependency. Users change their own passwords; administrators can recover them offline. Passwords are 12–72 UTF-8 bytes. Cookie sessions expire after seven days; writes require CSRF tokens and an allowed origin.

## Imports and FNB

Upload FNB CSV/OFX files or ZIPs containing them. Account identity and currency must match before commit. Limits: 20 supported files, 25 MiB uploaded bytes, 100 MiB ZIP expansion and 100,000 rows per file. Original uploaded bytes are discarded after staging; normalized source records, hashes, errors and decisions remain.

Clean statements and bank downloads import automatically through the same validated service. Hashes prevent exact re-imports; account-scoped FITIDs skip exact transaction duplicates. Conflicting IDs, invalid rows and ambiguous uploaded date/amount/description matches remain Import activity exceptions. Valid rows can be imported after acknowledging rejected rows; ambiguous uploaded candidates require explicit keep/skip decisions, including within-batch and cross-format matches.

FNB connections belong to administrators. Login details are encrypted on the same host with a separate runtime key; a privileged host operator can still access them. The owner enters credentials and verifies live login/MFA/layout compatibility. New discoveries create private accounts with owner editor access. Existing accounts require current editor access and keep their sharing settings. Hiding persists by bank identity. Disconnecting removes credentials and scheduling while retaining accounts/history/hide preferences.

Scheduling starts disabled, with off/6/12/24/168-hour choices. Runs execute while serving; failures requiring attention suspend automatic attempts. Scheduled runs refresh balances and import recent posted transactions for mapped, visible editable accounts. Initial discovery without mapped targets is account-only; new accounts join later transaction runs. Manual Get transactions uses the same import service; balance-only refresh remains available.

Live overlap matching consumes existing occurrences from immutable source date/amount/description and available balances/reference details. References and page positions are not FITIDs. Identical purchases on one page remain separate. Recent pages are capped at 150 bank rows and may miss older or indistinguishable purchases outside the visible window. Coverage warnings remain even after successful imports.

Supported ZAR types include Cheque, Savings (including Money Maximizer/Maximiser), Credit, Easy and Home Loan. Unique exact masked Credit identities are allowed when summary/detail match and Credit type/currency checks succeed. Do not guess digits or merge changed masks. Unknown types, currency, identity or layouts reject the run. Verified Home Loan history may use Effective Date/Description/Amount/Balance without status controls. Nonnegative service fees produce a separate negative Service Fees entry; principal remains unchanged, so bank movement equals Amount minus Service Fee. Logout must be confirmed before storing results.

## Classification and review

Categories are flat expense/income records. Spending groups are optional labels on parent transactions, independent of allocation categories. A category can appear in several groups; all splits share the parent group. Retain historical category IDs/metadata without merging records. Categories support rename/archive/restore; group names/colors are editable.

Complete categorized allocations accept transactions automatically on import, edit and transaction-created rule application. Explicit transfers are category-exempt. Missing categories, incomplete splits and rule conflicts remain in Needs review. Acceptance depends on category completeness, not manual approval or agent confidence.

Seen/unseen is personal and tied to the transaction version. Imports and rule-applied targets begin unseen. Saving marks the edited version seen for that user; later financial changes invalidate prior markers. Opening a dialog alone does not mark seen. Authorized 1–100 selection actions change personal seen state independently of acceptance and reporting.

Fresh installations contain no categories, spending groups, merchants or rules. Settings → Configuration supports optional starter configuration, multiple public HTTPS Git repositories and JSON files, explicit manual pulls, validated previews and snapshot export. Imports merge in explicitly confirmed application order, retain omitted entries and require confirmation for replacements. Existing installations retain their configuration. See [RULESETS.md](RULESETS.md).

Description rules choose categories and optional groups. Custom rules take precedence over built-ins; conflicting outputs withhold classification. Direction, priority, enabled state and account scope are explicit. New all-current rules apply only to current enabled editable accounts; future accounts do not silently join. Grouped edits/pause/delete are all-or-nothing and version-checked.

The transaction editor can explicitly save an account/direction rule for a single-category, non-transfer, nonzero transaction. In the same write it categorizes eligible existing unsplit uncategorized matches in that account, fills only missing groups and preserves existing categories/splits/transfers. Conflicts remain uncategorized. Standalone rule management applies to future imports, without retrospective application. Rules are never silently learned.

## Money, transfers and periods

Store signed integer minor units with currency. Same-sign split allocations must exactly total the parent. Spending/income totals use allocations; cash movement counts each parent once. Refunds reduce expense spending. Explicit transfer principal is excluded from income/spending; fees are separate expenses and linked counterpart details follow account access.

In the browser editor, selecting the group named Transfer sets the explicit transfer flag on save; selecting another group or Not set clears it. Existing explicit transfers keep their flag until the group is changed. Linked transfers require unlinking first. Opening a record, renaming a group or assigning that group through import/rule/API does not itself rewrite transfer status.

Periods default to the 20th, clamping beyond-month start days to the last day. Dates are inclusive and independently editable. Latest start wins overlaps; highest ID breaks a same-start tie. Gaps leave transactions unassigned. Manual period assignments survive date changes and flag out-of-range entries; explicit outside-budget assignments remain outside budgets. Creating/editing dates requires a reassignment preview covering period and transaction versions; stale previews cannot save.

## Budgets and reports

Limits are independent per period, spending group and expense category. Group and overall totals sum their entries once. Explicit No spending group differs from omitted legacy scope. Scoped replacement never removes other groups; whole-period removal is explicit. Preserve legacy aggregate-only limits under No spending group before rebuilding totals, with audit evidence.

Build a budget in the on-page spending-group/category view, using each group's plus to add and each category's pencil to edit. Dashboard drill-down and Budgets share a focused category modal with amount, recurrence and personal alerts saved together. Add group preserves an empty group immediately; removing a group requires confirmation. New entries default to This budget only. Upcoming entries fill missing entries in later-starting saved periods without overwriting included amounts, including zero; new periods copy recurring entries only. Empty groups, explicit zero entries and stopped recurrence persist. Removing budget membership does not delete category/transaction history.

Rebalance moves unspent budget between existing entries, preserves total limits and rechecks current versions/spending. Reports use saved periods, or whole periods starting in a selected year; private account scope uses period dates. CSV exports share ledger calculations and escape formula-like text.

## Metadata and agent access

Transactions support notes, tags, merchants, optional local logos and account-specific or global merchant naming rules. Import health separates dated balances, fetch/check attempts, successful imports and incomplete coverage. Viewing does not refresh bank data.

MCP uses identified users, current account grants, explicit connection consent and exact change proposals. Read-only is the default. Automatic approval is opt-in per connection/change type; combined effects require every grant. No silent permission expansion or direct write bypass. Private notes remain outside agent read fields. See [MCP.md](MCP.md) for tools, redaction, supported writes and browser-only workflows; [RULESETS.md](RULESETS.md) covers offline configuration import/export.

## Operations and scope

One Go service serves React and SQLite. Docker deployment uses an HTTPS proxy for remote access. Network overrides take effect after restart; offline reset restores environment defaults. Backups use integrity-checked snapshots at startup and at least every 24 hours, retaining 14. Offline restore preserves the prior database and invalidates restored browser/agent sessions. Connector keys need a separate private backup.

Verification uses synthetic fixtures for financial invariants, authorization, import idempotency, stale writes, rollback and browser workflows. Live FNB reliability, OFX ID stability across downloads and external MCP clients require separate checks. See [VERIFICATION.md](VERIFICATION.md).

Other banks, custom mappings, external AI classification, FX conversion, investments, offline synchronization and forecasting remain outside current scope.

## Application information

The sidebar and Settings → About show the backend's application build version and available revision details. Every signed-in user can access About, repository/documentation/issue links and a Report an issue link. Report prefills contain only public application build metadata, with user review and submission on GitHub. No financial, account or personal details are prefilled.


## Personal MCP context and aggregate reads

Each user can save personal context in Settings → MCP and explicitly allow individual connections to read it. Context is shared as written during authenticated initialization and through a refresh tool for later conversations. Existing connections start with sharing off. It grants neither financial access nor approval. Users can edit/clear it; concurrent edits reject without losing browser drafts.

Agents can request server-side spending/income/cashflow summaries and saved-period comparisons using authorized account/date/category/merchant filters. Spending uses allocations and refunds; cashflow counts parent movement once. Shared period summaries exclude private accounts unless an accessible private account is explicitly selected. Existing budget tools remain the source for budget status.

## Personal notifications

Persistent notifications belong to a specific user and are separate from transient Toast feedback. The header centre supports paged messages and explicit read/dismiss/source actions, with current account visibility, grants and household membership rechecked. Financial alerts use authoritative category/spending-group budget arithmetic and conservative, explainable unusual/recurring patterns. Approved defaults are 75/90/100% budget thresholds, projections after seven days/three spending days with at least 10% and R100 overrun, five matching historical observations for unusual spending, and four stable payments for recurrence. Shared durable episodes, resets, coalescing and cooldowns suppress repeated evaluations. Private accounts never contribute to shared budget alerts.

All registered in-app types default enabled; browser push defaults disabled and requires explicit device opt-in. Preferences for both channels save atomically through one action. Push lock-screen text is generic and its direct link rechecks the signed-in recipient and source access. Unsupported browsers retain in-app workflows. The shared push/PWA worker caches only build-listed static assets and a generic reconnect page; financial/authentication responses and application HTML remain network-only. Administrator diagnostics expose bounded redacted lifecycle metadata without financial messages or subscription secrets. Payloads expire after 90 days (500 per recipient); longer receipts preserve retries, and diagnostics expire after 14 days (10,000 global rows). [NOTIFICATIONS.md](NOTIFICATIONS.md) defines exact detector rules, reset/retention semantics, browser APIs and remaining operational checks.

## Branding and PWA

Administrators manage shared workspace branding and a separate public installed-app identity in Settings → Branding and PWA. PWA name/icon can independently follow branding or use custom values. Existing workspace names stay private unless inheritance is explicitly selected; public logos and PWA metadata are identified before save. PNG logos supply consistent sign-in, navigation, browser and installed icons. Installation and explicit update/reload controls are available to every user. Offline navigation shows only a generic reconnect screen, with no financial API caching or offline editing. Update reloads are user-triggered so other tabs retain drafts. See [PWA](PWA.md) for exact cache, identity and operational behavior.

PWA discovery: signed-in users receive a one-time, dismissible installation invitation when their browser offers installation (or Safari home-screen guidance on iOS). The browser remembers that it was offered or installation attempted; installed apps and pending update notices suppress it. Manual installation remains available in Settings → PWA. No native install or notification-permission prompt opens automatically.


Personal budget alerts are configurable per expense-category/spending-group combination, including No spending group, and apply across saved periods. Each combination has an on/off switch covering threshold, projected overspend and actual overspend types. Its optional single whole percentage (1–100) replaces the default 75/90/100 threshold sequence. Reset to defaults enables the combination and restores that sequence. Global type/channel preferences still gate every delivery. Alerts use that combination's explicit saved budget and transaction-group allocation spending; no aggregate category or group-total duplicate is emitted. Saving settings silently consumes currently active conditions, preserving future refund/reset/re-cross behavior and projection cooldowns rather than sending historical alerts.

## Releases and distribution

Development pushes produce verified beta versions; main pushes produce verified stable versions. Mutable beta/development and main/latest image channels are backed by immutable SemVer/SHA tags and recorded digests. Release binaries, browser About, MCP initialization and OCI labels share metadata. Standard GPL-3.0 applies to original project code, preserving upstream/dependency licenses; forks and sales are permitted under its terms. See [release scope, compatibility and recovery](RELEASES.md).


Personal scoped budget alerts also support explicitly consented MCP reads and exact prepare/approve/apply proposals for the connection owner. Separate read/change grants default off; budget editing never grants alert access. Household membership and unrestricted connection account scope apply. Shared preference services preserve silent baselines and global notification precedence; inbox, global channels and push devices remain browser-only. See [MCP.md](MCP.md).

Application release awareness in Settings → About compares the installed published stable or beta version with its own channel. Local/dev/modified/unknown builds are explicitly unsupported. A quiet version/Settings indicator appears only for a confirmed newer release. Checks are credential-free, server-side and cached; failures remain unavailable, with prior results labelled outdated. Release notes and backup/migration guidance precede a manual administrator-controlled Docker upgrade. This is separate from browser-worker refresh and performs no installation or restart.
