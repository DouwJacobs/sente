# Finance tracker handover

Updated 2026-10-03. Latest FNB runtime choice: owner accepted same-host encrypted storage; see Automatic accounts/balance refresh below. Implementation of outstanding handover tasks was explicitly authorized in the linked Windows project on this date. New feature requests beyond this authorization should still be recorded before implementation. Work from `/home/douw/finance-tracker` in Ubuntu WSL. Read `AGENTS.md`, `docs/PLAN.md`, `docs/ARCHITECTURE.md`, `docs/UI.md` and `.agents/skills/finance-tracker-ui/SKILL.md` before changes. The owner's latest instructions supersede initial MVP exclusions where explicitly expanded below.

## Completed before this linked-project handover

- Imports cards share page width; dashboard columns stretch to align their top and bottom edges. General card-alignment expectations are recorded in AGENTS/UI/skill.
- Eleven spending groups are seeded once: Day-to-day, Recurring, Invest-save-repay, Exceptions, Income, Transfer, Bank Fees, Communications, Debt, Utilities and Insurance. Household members can create groups and edit their names/colors. Stable IDs preserve transaction references after renaming; version checks reject concurrent overwrites.
- Each parent transaction has an optional spending group independent of its allocation categories. Categories are flat: Day-to-day + Eating Out and Exceptions + Eating Out are both valid. No category-group field is required. Existing legacy category-group metadata remains in SQLite without merging category IDs or rewriting limits.
- Transaction details offer spending-group selection and category search, Most used, selected checks and explicit category creation. Splits use separate category pickers and share the parent's spending group. Category popularity counts distinct transactions in currently accessible accounts, without exposing private-account history.
- Saving spending-group/category changes returns a transaction to pending review and records audit provenance. A group named Transfer never substitutes for explicit ledger transfer designation.
- Backend migration 3 preserves existing financial data and seeds defaults only once. At that original handover, there were no FNB connector endpoints, credential stores or display-name settings. See the implementation update below for the current status.

The latest verification record is `docs/VERIFICATION.md`. Screenshots use synthetic data. The display name is now implemented. FNB and the broader Vault22 reference remain incomplete; do not mistake a design proposal for a working connector.

## Implemented in the linked Windows project (2026-10-02)

- Windows project has a folder shortcut and editor workspace pointing to `\\wsl.localhost\Ubuntu\home\douw\finance-tracker`; root AGENTS.md routes subsequent work to the WSL source. Native directory symlink creation failed because Windows requires administrator privileges. No application source or owner database was copied.
- Migration 4 adds singleton workspace branding with default Household and a separate optimistic version. Authenticated GET /api/branding is available to all signed-in users; PUT is administrator-only. Ordinary budget-settings permissions remain unchanged.
- Admin Settings validates trimmed 2–60 Unicode characters using shared Field/Form controls. Stale edits preserve the draft and can reload the saved name. Sidebar, mobile header, page branding and title use the same setting. Sign-in/onboarding remain generic for privacy. Long navigation names truncate without shifting controls; full names remain available through the title and Settings.
- Category limits offers Add expense category using the shared flat category form. The new category begins at zero, gets input focus, and preserves draft limits and selected period. Creating a category does not save limits; explicit Save limits retains period-version checks. Existing expense categories already appear, including those without a target; income categories stay excluded.
- Applied the owner's reduced inspection policy consistently to AGENTS.md, UI.md and the UI skill: focused builds/tests for routine changes, targeted visuals when requested or layout/interaction risk warrants them. Actual coverage must be recorded.

## Display name: original acceptance requirements (implemented)

Owner request: an administrator can replace “Household” with a personal name in Settings. This is a workspace-wide presentation setting, separate from usernames, account names and permissions.

1. Add a migrated, defaulted workspace display name (default Household) and optimistic settings version. Add an authenticated read endpoint for branding available to all signed-in users, and an administrator-only update endpoint. Do not widen ordinary household-members' existing budget-settings permissions.
2. Add an Admin Settings field using shared Field/Form validation: trimmed nonempty text, bounded length (proposal: 2–60 characters), inline errors after first blur, server validation and stale-edit feedback. Update desktop sidebar, mobile brand, document title and relevant app branding from the same value. Decide explicitly whether the unauthenticated sign-in screen may show it; a generic title preserves privacy by default.
3. Escape text through ordinary React rendering. Test persistence/restart, non-admin direct API denial and concurrency. Visually inspect long names at desktop and 360px, both themes, without shifting the navigation or card edges.

## FNB prototype progress (2026-10-02)

Pinned original source under connectors/fnb/upstream with GPL notices and UPSTREAM.json, without installing or running it. Reviewed login/session handling, transaction/balance extraction and dependencies. Added an exact-money mock normalization contract and synthetic tests in internal/app/fnb_adapter.go and fnb_adapter_test.go. References are separate provenance, coverage always signals possible gaps, 150-row responses flag truncation, and identical purchases remain distinct. Added a separately packaged, authenticated read-only Node mock service, with fail-closed explicit mock mode and internal AES-256-GCM primitives tested on synthetic secrets. No live provider/credential endpoint/staging/commit/scheduler exists. See connectors/fnb/README.md for findings and deployment comparison. Owner accepted a separate isolated service. It must be deployed outside this agent's host permissions to meet strict secrecy; host details and owner-managed provisioning remain outstanding. A current-host container/process cannot meet that guarantee.

## Next delivery: FNB connection and scheduled imports

Owner request: connect an FNB profile with a username/password entered by the user, save securely, discover accounts and update accounts/transactions periodically; configure the schedule in Settings. Earlier authorization prefers vendoring maintained source over installing fnb-api itself from npm. Its ordinary dependencies may still require a runtime/package install.

### What has been verified from the reference

Reference: [bitshiftza/fnb-api](https://github.com/bitshiftza/fnb-api). Its [README](https://raw.githubusercontent.com/bitshiftza/fnb-api/master/README.md) describes balances and transactions scraped through online banking, cheque/credit/savings transaction support, and **only the first page, up to 150 successful transactions**. The [package manifest](https://raw.githubusercontent.com/bitshiftza/fnb-api/master/package.json) declares Puppeteer and Moment dependencies and GPL-3.0 licensing. This is a browser scraper wrapper, not proof of a currently supported official bank integration.

Current FNB login compatibility, MFA/device approval, complete pagination/history, statement-to-scraper identifier stability, current read-only-profile availability and bank permission to automate remain unverified. Do not advertise automatic syncing as reliable until these are checked. Preserve attribution/license if source is vendored, pin the upstream commit, document local changes, and review source/dependencies before running it. Do not import its demonstration console logging into production.

### Credential requirement and limits

The owner specifically says the development agent should not be able to access the username/password. Never request bank credentials in chat, fixtures, logs, screenshots or command arguments. The owner enters them manually in the tracker when the secure flow is ready; any live bank login/approval test is handed to the owner. All development and automated tests use synthetic credentials and mocked provider responses.

Scheduled password login needs the worker to recover plaintext briefly. Password hashing alone cannot support this. Proposed storage: authenticated encryption (AES-256-GCM or equivalent), fresh nonce per record, connection/user identity bound as associated data, ciphertext and key version in SQLite, key supplied separately at runtime. Do not store a plaintext key beside the encrypted database, in the repo or in ordinary backup snapshots. Use an external secret service or an owner-provisioned service-scoped runtime secret; document key backup, rotation, loss and recovery. The encrypted username is also private. Return only connection state from APIs; never provide a decrypt/read-credential route or include credentials in MCP resources.

Encryption at rest and a hidden password field cannot guarantee secrecy from an agent with unrestricted host access, the service identity, its memory or its decryption key. A strict “agent cannot access it” guarantee requires a real isolation boundary: for example, an independently deployed connector/secret service outside the agent's host permissions. Resolve and implement that boundary with the owner before accepting live credentials. Do not claim a guarantee based on UI masking alone.

Treat bank cookies/session caches as credentials too: encrypted or ephemeral, no browser profile in the workspace, redacted errors, no raw HTML/screenshots/debug dumps of live banking. Credentials must be user-owned; other users/admins cannot retrieve them. An admin may disable a failing connection without learning secrets. Respect current account permissions on every sync.

### Implementation sequence

1. **Compatibility prototype, no credentials stored.** Inspect and vendor a pinned source snapshot into a clearly separated connector directory. Check license notices and supported runtime. Build a mock adapter contract for account identity, signed integer cents, bank dates, balances and available transaction IDs. Let the owner perform a live read-only login test through a prepared UI without agent observation; establish MFA/device approval, supported account types, pagination and date coverage. Never retry authentication in a tight loop.
2. **Resolve deployment and secrecy.** Keep Go + static frontend as the main app. Puppeteer implies Node/Chromium, greater image size and runtime needs; compare a restricted worker process in the existing container against a separately isolated connector service. The strict credential requirement may justify revisiting the one-container default. Avoid exposing a new public port or using a privileged browser sandbox shortcut without a concrete reviewed deployment decision.
3. **Connection setup.** Introduce per-user connection metadata, encrypted secrets, connection version/status, and explicit mapping from discovered bank accounts to tracker accounts. Preview account discovery. No silent household grants or account creation; account-management rights remain administrative and private accounts stay private. Editing/replacing credentials is write-only. Implement pause, disconnect/secret removal and reauthentication states.
4. **Reuse staging.** Refactor file-owned commit logic into shared normalized staging/import services before attaching the adapter. Sync entries retain immutable source metadata/run IDs and go through account/currency validation, account-scoped IDs, cross-format candidate duplicates, description rules and mandatory human review. No silent overwrite of conflicting IDs or automatic acceptance of uncertain duplicate candidates. Existing valid transactions can be staged/committed only under an explicit opt-in policy agreed for the connection; candidate decisions remain in the UI.
5. **Reconcile historical coverage.** Poll overlapping date windows and keep a checkpoint only after successful durable import. Compare scraper references with CSV/OFX identifiers; do not assume equality. Define how pending authorizations become posted entries and how refunds/reversals/fees are represented. A 150-row page is an incomplete-history signal, not a complete account ledger. Report returned coverage, latest balance date and possible gaps.
6. **Settings and scheduler.** Owner-visible schedule controls, enable/pause and Sync now; persist last attempt/success, next due run and sanitized failure state. Proposed initial default: disabled until explicitly enabled, then one daily run (not yet an accepted frequency). Use serialized per-connection runs, bounded retries/backoff, no overlapping manual/scheduled jobs, cancellation and a recoverable crash checkpoint. Authentication/MFA failures stop retry loops and require owner action. Check permissions again at execution, not just scheduling.
7. **Acceptance and documentation.** Mock expired sessions, MFA, revoked access, changed bank layouts, pagination, exact money, account mismatches, repeated/concurrent runs, CSV/OFX overlap, legitimate identical purchases and ID conflicts. Check secret redaction in API errors, audit, logs, backups, worker messages and agent tools; test rotation and missing-key behavior. Verify schedule persistence, crash recovery and owner-managed live tests. Visually check setup/status/errors on desktop and 360px in both themes. Document deployment, secret provisioning and recovery. Keep manual CSV/OFX imports available.

## Vault22 as a reference: backlog, not automatic scope

The owner asks to record useful missing features for later. Use the supplied screenshots as interaction references, while preserving our shared UI guide and finance semantics. Implemented now: spending-group/category selectors, searchable categories, Most used, transaction review and splits.

Candidates observed in the screenshots, to validate and prioritize later:

- Merchant field separate from raw imported description, with explicit normalization and rules.
- Transaction tags: reusable labels, filters and user-defined tags.
- Transaction-level notes independent of split allocation notes.
- Meaningful transaction markers (the reference displays “Safe”); clarify their meaning before creating another status alongside pending/approved.
- Category rename/archive and spending-group archive, preserving historical IDs, budgets and rules.
- Independent spending-group filters and reports; category budgets remain category-based. Decide whether group limits are wanted before adding them.
- Most-used recency/account-filter behavior, richer transaction detail presentation and category-management shortcuts.

Existing review, transfer, period assignment, audit and account-permission behavior must remain authoritative. Do not silently copy unseen Vault22 behavior or add external AI classification. Agent access remains a separate future direction in `docs/FUTURE.md`.

## Working and verification instructions

Use `make dev` for frontend HMR and watched Go builds; the browser-facing URL is `http://127.0.0.1:5173`. Avoid Docker rebuilds for UI iteration. Go executable is `work/toolchain/go/bin/go`; Linux Node is `/home/douw/.nvm/versions/node/v22.21.1/bin`. Build frontend with that Node on PATH, then run Go tests. `scripts/e2e-server.py` provides a separate synthetic database and optional `E2E_EMPTY_ACCOUNTS=1`. Never point tests at the owner's database.

UI changes use focused build/tests and structural checks. Use targeted computer-use visual checks when requested or layout/interaction risk warrants them, recording actual coverage. Preserve user preferences and data. Update AGENTS/UI/skill for accepted conventions, and PLAN/ARCHITECTURE when product behavior changes.

Recommended next order: provision the accepted separate isolated service and fix/test FNB compatibility → secure owner-only connection UI → shared staging integration → scheduling and recovery → prioritized Vault22 backlog. Display name, inline budget categories and the mock service/prototype are implemented. Do not start live credential storage before the isolation decision is resolved.

## Additional requests — current implementation status

These requests were originally recorded without implementation. The latest explicit authorization starts implementation: the inspection policy and inline budget categories are now implemented. The broader budget-management behavior still requires product decisions below.

### Reduce inspection token usage

The owner requested removing the mandatory computer-use inspection requirement because it uses too many tokens. Proposed future policy: use focused build/tests and structural checks for routine changes; use short, targeted visual checks only when explicitly requested or when layout/interaction risk warrants them. Avoid a full desktop/mobile/theme sweep after every small UI edit. Do not claim screenshots or visual verification that were not performed. Update AGENTS.md, docs/UI.md and the project UI skill consistently when this policy change is applied; the reduced policy has now been applied consistently under the latest implementation authorization.

### Vault22 budget-management reference

The owner supplied a Budget Management screenshot, showing:

- Previous/next budget-period navigation and Edit period.
- A period summary ring, spent/budget/remaining amounts, days remaining and a per-day allowance.
- Expected end-of-period status and pace labels, plus Month/Year controls.
- Income actual versus expected, expandable spending-group rows, and progress indicators.
- Add category, Rebalance, alphabetical/amount sorting and Export CSV.

Treat this as an interaction/product reference, not an instruction to copy its colors or every behavior. Preserve the tracker's existing tokens, responsive card alignment and exact money semantics. No financial details from the screenshot need to become fixtures or seed data.

Next planning steps: define the budget's primary unit (category limits, spending-group limits, or group+category combinations); define expected income, rebalance behavior, period/year grouping and CSV scope. Categories remain independent of spending groups. If both types of limits are shown, they must not be added together into one total or count the same allocation twice. Spending-group breakdowns use the parent's group; category breakdowns use its allocations. Explain pending inclusion, refunds, transfers and unassigned spending consistently. Determine whether allocation-level group overrides are actually needed before changing the current parent-group model.

Per-day allowance and pacing must use the saved custom period dates and South African calendar dates, handle zero/ended/future periods and overspending, and avoid suggesting certainty from incomplete imports. Forecasts need an explicit formula and sufficient-history/coverage messaging. Year summaries must define how custom periods, gaps and overlaps are handled. Income targets and group limits are new financial behavior, not styling-only changes. Retain account-permission filtering for every aggregate/export.

### Add categories directly to budgets

Owner report: “I can't add categories to budgets.” Current implementation lists every existing expense category in a period's Category limits dialog. Categories are created on the separate Categories page; there was no inline Add category action in Budgets; this gap is now fixed. Income categories are intentionally not expense-limit fields. This is a workflow gap; it is not evidence that existing category limits fail to save.

Implemented improvement: add a clear Add category action inside the budget/limits workflow, reuse the flat category-creation form, then include the new expense category with an initial zero limit ready to edit. Preserve unsaved limit values and the selected period; creating a category must not silently save all limits or modify historical periods. Give a route back to an existing category, prevent duplicate names and maintain member/editor permissions. If the future budget model assigns category targets within a spending group, let the same category be budgeted in different groups without making that group part of the category definition.

Acceptance: first category with an empty budget; new category in an existing period; an existing category with no target; duplicate/category-type errors; permission denial; preserving unsaved limits; concurrent period edits; and explicit save/refresh confirming the chosen amount. Confirm the revised budget model before implementing group-specific limits or rebalance.

Latest steering: do not use computer use to inspect the UI for now. Continue with automated tests/builds and structural checks; do not conduct further visual inspection until requested.

## Owner-run accounts-only test (2026-10-02)

The owner proposed manually running live compatibility tests and reporting only output/errors, without agent observation of credential entry. Current short-term implementation: an owner-run script opens a temporary incognito Chrome/Edge window; the owner signs into FNB directly and opens Accounts. After terminal confirmation, the script reads only nickname/account-number text and exports a metadata-only JSON file outside the source tree. No login automation, credential prompts/environment-variable ingestion, credential storage, balance/transaction extraction, recurring calls or live agent execution. Same-host operation is a working agreement, not a technical secrecy guarantee; the separately isolated service remains the long-term design. Environment variables/.env files do not create that boundary.

`connectors/fnb/owner/` uses pinned modern puppeteer-core 25.12.0, not the vulnerable old upstream runtime. Source-derived selectors preserve GPL attribution. The installed owner dependency audit found zero known vulnerabilities on this date. Windows project launcher: Test-FnbAccounts.ps1. The script defaults to installed Chrome/Edge, pipe transport, a temporary profile outside source and a new Downloads export file. Fixed error codes hide raw browser errors/HTML. Live DOM compatibility is owner-verified only and remains unconfirmed.

Accounts → Import discovered accounts validates a strict account-only JSON shape in the browser; extra secrets/balances/transactions are rejected. Administrators review/edit every account and explicitly add it using the existing authorized/audited account API. Private is the default; household sharing is explicit. Existing accessible accounts are detected without modification, and server duplicates remain inline errors. Masked numbers require full-number correction. No balances, ledger entries or import records are created by this workflow. Do not ask the owner to paste the real exported report or credentials into chat.

Owner live-test result: manual login completed, then ACCOUNT_LAYOUT_CHANGED; discovery remains unverified. The extractor now excludes hidden/input nodes and supports owner-run --diagnose with allowlisted numeric LAYOUT_COUNTS only. Await the owner's next result; never inspect the live browser or read the report.

Owner diagnostics found 13 visible names/numbers, one invalid number format, no invalid names or duplicates. Added explicit owner-run --skip-unsupported for a partial metadata export; prints skipped count/summary positions, defaults to strict failure, never guesses or repairs bank IDs. Live partial export still awaits owner confirmation; omitted accounts can be created manually.

## Accepted automatic accounts/balance refresh (2026-10-02)

Owner now accepts same-machine encrypted credential storage and its agent-access limitation, superseding the separate-host requirement for this implementation. Build tracker credential entry, automatic FNB login/Accounts navigation, account name/number/current ledger balance discovery, on-demand and optional scheduled refresh. Hidden discoveries persist across runs and are excluded from tracker views and sync; no transaction extraction in this milestone. Keep original transactions stored for restoration. No live credentials/tests by the agent; owner verifies compatibility. Schedule starts disabled with a configurable interval. MFA/layout failures suspend automatic retries until owner action.

Implementation update: migration 5, owner-only encrypted connection routes, local Node stdin/stdout bridge, current ledger-balance snapshots, private account creation/update, persistent hide/show and configurable scheduler are now implemented. User credentials and live browser remain untouched by the agent. Manual account discovery was owner-confirmed (12 usable entries, one unsupported); automatic login/approval/ledger extraction still awaits owner verification. See README/ARCHITECTURE for runtime/key recovery and Docker limitations. Transaction syncing remains outstanding.

## Headless/debug and balance failure follow-up

Owner reports BALANCE_LAYOUT_CHANGED after automatic login, and requests headless refresh unless debug is enabled. Migration 6 adds per-connection debug_browser (off for existing/new connections) and safe count-only last_diagnostics. Accounts offers Debug mode — show Chrome during refresh plus Save browser mode; both manual/scheduled runs honor the saved value. Debug opens/fronts the temporary browser without screenshots, page dumps or credential logging.

Account names may appear before ledger fields; the worker now waits briefly for balance fields. Explicit missing-bank-value placeholders preserve old balances, never invent zero. Structure/format failures remain suspended and show BALANCE_COUNTS through Layout diagnostics; counters identify named field/row mismatches or amount format shapes without account details. Actual failure cause/compatibility still awaits owner retry. Do not inspect the live browser, real reports, stored credentials or key.

## Settings organization (2026-10-02)

Owner requested a clean UI with all settings-related controls under Settings and subtabs as needed. Implemented General (branding/preferences), Banking (FNB connection, refresh, schedule/debug/diagnostics/hide-show), Accounts (setup/names/numbers/sharing and discovered-file import), Users & access, Backups and Security. Accounts is a balance overview with a Manage accounts shortcut. Appearance shortcut removed from topbar. Permission checks/API behavior are unchanged; non-admin UI has General/Security only. General form drafts survive tab changes; Banking credential drafts are discarded when leaving its tab. Updated UI guide and relocated workflow selectors; no computer-use/live-bank inspection.

## Toasts, Banking spacing and logout follow-up

Owner prefers overlay toasts rather than notification cards that shift layout; recorded in AGENTS and UI guide. Shared Toast replaces main/login/modal notification banners, preserving inline field validation. Banking controls/helper text now have consistent separation. Layout diagnostics is always available for connected accounts, showing an explicit empty state and fallback fixed numeric counts for evaluation errors. The live BALANCE_LAYOUT_CHANGED cause is still unknown; owner could not locate the previous conditional disclosure.

Owner requires explicit bank logout after every refresh because browser closure alone can leave an active server session. Worker cleanup attempts an exact logout/sign-out control on approved FNB pages on success/failure, waits for visible signed-out login fields, then closes context/browser/profile. If an authenticated session cannot be signed out/confirmed, emit LOGOUT_REQUIRED, suspend scheduling and do not commit its snapshot; owner may need to sign out manually. No raw logs, credentials or live session inspection. Actual bank logout compatibility still requires owner verification.

Latest owner counts show 13 names/numbers/ledger fields, 11 mapped supported rows, two unsupported identities and one unsupported balance value. Ledger-only selection excludes Available balance. Expanded exact normalization accepts optional decimal zeros, explicit plus and Unicode minus; unknown values remain atomic failures with format-only counters. Added exact Log Off matching and immediate SESSION_CONFLICT detection for visible session-termination/logged-out-shortly warnings during account polling. Conflict suspends scheduling without writing accounts; owner must let the previous session end/sign out and retry manually. Live compatibility remains owner-verified only.

Follow-up verification completed: frontend build/five unit tests, all 18 synthetic owner tests, focused Go Headless/Logout/PreviousSession tests and vet, and four Playwright Banking/toast workflows at 1440/360px passed. Toasts are interactive above native dialogs via the active-dialog portal. Dev health returned 200; no live banking or computer-use inspection. Owner should sign out any existing FNB session before another Settings → Banking → Refresh now; if balance parsing still fails, share only the updated BALANCE_COUNTS line.

2026-10-02 logout confirmation follow-up: owner observed actual logout with the message “You have successfully logged out of banking,” despite LOGOUT_REQUIRED. Worker now accepts explicit completed logout text with no visible authenticated markers, or visible login fields, on approved bank pages/frames; retries navigation races for up to 15 seconds. Impending logout/session-termination text does not confirm completion. Fixed numeric logout_clicked/logout_confirmed/logout_unconfirmed counters distinguish cleanup from balance problems; balance_failure is retained if cleanup also fails. Confirmed cleanup preserves the original balance error. Ledger extraction uses rendered innerText rather than hidden-descendant textContent; unknown text remains a format error with fixed balance_label/unavailable_text/loading_text counters. No bank text/values in diagnostics and no agent live session access. Live amount format remains pending owner retry.

2026-10-02 reverse proxy support: explicit TRUSTED_PROXIES IP/CIDR configuration permits bounded, right-to-left X-Forwarded-For client identity for login rate limiting only. Untrusted/malformed forwarding falls back to the socket peer; IPv6/mapped IPv4 normalize correctly. PUBLIC_URL remains authoritative for HTTPS cookies and allowed origins. Native LISTEN_ADDRESS controls binding; Compose passes trusted proxies and preserves its loopback host-port default. Added deploy Nginx/Caddy examples and docs/REVERSE-PROXY.md for dedicated-hostname TLS, multipart limits, 300-second FNB timeouts and Docker/network configuration. No live proxy/domain deployment changed.

2026-10-02: Owner authorized Settings → Network. Implemented administrator-only public URL/trusted proxy editing, validation, active-versus-saved status, save-for-restart and Use environment settings. Migration 7/API use optimistic versions and audit; saves do not change running origin/cookie policy. Offline finance reset-network recovers access. NPM certificates/forwarding and listener bindings remain deployment configuration. Build, automated UI and focused backend verification recorded in docs/VERIFICATION.md; no live network deployment changed.

Settings → Network verification complete: production build/seven unit tests, five desktop/mobile Playwright settings workflows, focused Go proxy/network/restart/migration/backup recovery tests and vet/build passed. Dev health returned 200. Saved overrides apply only on service restart; environment fallback and offline reset-network recovery tested. No computer-use/live banking inspection or actual NPM deployment changes.

2026-10-02 balance currency follow-up: owner-provided screenshot identifies numeric-identity eBucks and foreign-currency ledger rows as unsupported by the ZAR-only tracker. Worker now skips recognized reward/foreign currency units, reports fixed reward_entries/non_zar_entries counters, and continues the supported ZAR snapshot. Masked identifiers remain skipped. No conversions or unit stripping into ZAR; unfamiliar text still fails closed. Previously stored accounts/history remain intact. Banking skipped-entry copy explains number/unit exclusions. Screenshot financial values/identities are not copied into fixtures; all tests use synthetic values. Owner diagnostics confirm logout; next live balance refresh remains owner-run.

2026-10-02: Owner requested less clutter in Settings → Banking. Reorganized into a concise connection/refresh summary and four collapsed sections for schedule, account visibility, troubleshooting/debug/diagnostics and credentials/disconnect. Preserved account hide/show after disconnect, credential clearing/cancel, APIs/permissions and all existing controls. Transient feedback stays in toasts; persistent detail is in disclosures. Focused desktop/mobile verification is recorded below when complete.

Banking cleanup verification: frontend build/seven unit tests and five desktop/mobile Banking/Settings Playwright workflows passed. Default view shows Refresh now and sync status/dates; schedule, visibility, troubleshooting and connection controls are collapsed. No backend/financial behavior changes or computer-use inspection. Dev health returned 200.

2026-10-02: Fixed Settings → Accounts row metadata appearing directly beside the name. Name is now above muted bank/ending-number/sharing details, with spacing and wrapping. Build passed; no backend or financial changes.

## Classification preparation authorized (2026-10-03)

Owner confirms live FNB account pulling looks correct and requests transaction fetching for enabled accounts, with classification rules prepared first using a local Vault22 CSV as reference. The owner accepted continuation of the classification proposal. Implement direction-aware rules, optional spending groups, editable/paused rules and a reference preview/setup workflow before the transaction connector. Starting assumptions: combine grocery labels for new proposed rules and preserve useful personal/project labels; expose those choices in preview. Never merge existing category IDs/limits, import historical transactions from this reference implicitly, or infer ledger transfers from group names. Keep real reference data/drafts out of source control. Live bank browser/credentials remain owner-operated. Transaction syncing is the subsequent milestone.

Implementation: migration 8, direction/group/enabled/version rule CRUD, audited atomic definition setup, saved/draft rule tests, shared conflicting-output withholding, group persistence and classification-version import protection are implemented. Categories contains browser-only Vault22 reference parsing/proposals, explicit account mapping, selection and historical overlap/disagreement preview. Default combines grocery labels for proposals; personal/project labels are retained where consistent. No existing categories/limits/ledger records are rewritten. Owner must map the masked reference accounts to enabled tracker accounts and install their chosen rules in Categories. FNB transaction scraping is not implemented in this milestone.

## Owner simplification (2026-10-03)

Owner says setup is unintuitive/overcomplicated. Remove the Vault22 file-upload, account-mapping and historical comparison workflow from the product. Vault22 remains a development reference only. Transactions must come from the live FNB service after rule setup; never use the reference CSV as their source. Make Rules the default Categories view, with plain description/category/group controls, optional direction/account restrictions and a way to apply one rule to current enabled editable accounts without repetitive account-by-account entry. Preserve backend permissions, versions and review/transfer semantics. This owner steering supersedes the earlier reference-setup UI requirements; live transaction fetching remains the next step after rule setup.

Simplification implemented: Rules is the default Categories tab, with separate Categories/Spending groups tabs. Three main rule fields, More options for scope/direction/priority, inline category creation, optional example check, grouped account definitions and direct Pause/Resume. New rule can save atomically to all current enabled editable accounts; existing grouped edits preserve per-account permissions/versions. Vault22 parser/proposals/upload/mapping/setup route were removed. Existing definitions/categories/ledger data are preserved. Current live connector is still accounts/balances only; FNB transaction fetching remains outstanding.

## Built-in classification and transaction shortcuts authorized (2026-10-03)

Owner requests basic categories/rules shipped with the program, plus custom categories/rules and category/rule creation while editing transactions. Seed general starter labels once without rewriting existing IDs/kinds/limits; built-in rules are shared fallback suggestions for enabled accounts, editable/paused by household budget members, and custom account rules take precedence. No private/project reference data is shipped. Category picker should create and select a typed unknown name in one explicit action, without another form for ordinary expense creation. Transaction rule saving must retain category, parent group and payment direction, stay account-scoped, reject splits/transfers and commit atomically with the edit. Preserve current exact-money/review/account permissions.

## Todo: consolidate account management (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md. This delivery supersedes the earlier placement of account visibility controls in Settings → Banking.

- [x] Move account hide/show controls from Settings → Banking to Settings → Accounts, alongside account editing, using a simple, elegant layout with clear visibility state and easy access to hidden accounts for restoration.
- [x] Make account editing and hide/show available directly from the main Accounts tab as well. Reuse the same account-management controls and saved state so account-related actions are kept together and both entry points stay consistent.
- [x] Keep Settings → Banking focused on setting up and managing connectors for account retrieval, including connection credentials, refresh/scheduling and connector troubleshooting. Remove account-management/visibility controls from that tab.
- [x] Preserve existing account permissions and persistent visibility behavior across refreshes and disconnects. Hiding must retain stored accounts and transaction history so showing an account restores it; do not change financial or sync semantics as part of this UI reorganization.

## Todo: paginate classification rules (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md.

- [x] Add pagination to the Rules tab on the main Categories page so a long rule list stays manageable.
- [x] Keep the controls simple and consistent with the shared UI: clear current page, previous/next navigation and an indication of the total rules/pages. Preserve existing rule ordering, account scope and edit/pause/resume actions across pages; keep any filtering/search consistent with pagination.

## Todo: refine expandable card styling (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md.

- [x] Restyle expandable/collapsible cards and disclosure controls across the application to fit the surrounding UI, including Settings → Banking sections such as Automatic refresh and Account visibility (while visibility remains there before the planned account-management move).
- [x] Replace the current stark white dropdown presentation with a subtler disclosure button/header using shared theme colors, spacing and borders. Keep the treatment simple, elegant and consistent in light and dark themes.
- [x] Preserve clear expanded/collapsed indicators, keyboard operation, accessible focus states and comfortable mobile touch targets. Apply the accepted treatment through shared components/styles rather than separate screen-specific designs.

## Todo: paginate all large lists end to end (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md. This expands the Categories → Rules pagination request above.

- [x] Audit potentially large lists across the application and add backend pagination to their list endpoints. Frontend-only slicing or loading the entire dataset before displaying pages does not satisfy this requirement.
- [x] Use frontend page navigation or infinite scrolling as appropriate for each workflow, fetching bounded pages from the backend. Keep navigation/loading states simple and consistent with the shared UI.
- [x] Apply permissions, search, filters and sorting on the server before pagination. Use deterministic ordering and a bounded page size; choose cursor or offset pagination to suit the list and handle changes between requests without silently duplicating or skipping results.
- [x] Preserve editing, selection and list actions across pages. Totals and financial aggregates must cover the full authorized filtered dataset rather than only the loaded page; exports and bulk actions must have explicit scope.
- [x] Verify bounded backend responses, permission boundaries and correct filtering/ordering across multiple pages, plus frontend loading, empty/end states, errors and navigation or scroll continuation.


## Starter classification implemented (2026-10-03)

Migration 9 seeds 21 general categories and 24 general rules once. No personal reference data is shipped. Separate builtin_rules are visible, editable, pausable and deletable by budget members; negative API IDs distinguish them from custom account rules. Enabled built-ins provide fallback for all enabled accounts, including future discoveries. Any matching enabled custom account rule suppresses built-in suggestions; conflicting custom outputs remain unclassified. Existing category IDs/kinds, targets, ledger records and review state are preserved. Legacy ambiguous category names or conflicting kinds skip the related defaults. Deleted/paused defaults do not return after restart.

Transaction category picker supports explicit type-and-Enter/Create, selecting the result immediately without a second form. Expense is the default; Income remains available. Typing alone never persists a category. Exact existing names select the existing category. Category creation retains budget-member permissions and shared validation/toasts.

Transaction PUT optionally carries a rule pattern. The server saves category/group/account/signed direction in the same database transaction; errors roll back the entire edit. A unique normalized existing account/pattern/direction rule is updated rather than duplicated. Multiple such existing rules require resolution; splits, transfers and zero amounts cannot save a rule. Explicit user opt-in remains required. Rules never approve transactions or designate transfers. FNB transaction fetching remains outstanding.

## FNB live transaction importer: first delivery (2026-10-03)

Owner requested implementation of transaction fetching after classification setup. Imports now offers Fetch FNB transactions using the saved owner connection and debug mode. The server sends only visible, previously mapped accounts under current editor access, serializes with account/scheduled refreshes, and rechecks active owner, identities, visibility and editor access before atomic staging. The worker uses pinned-source-derived navigation and strict named columns/full detail identity; supported Cheque/Savings/Credit/Easy ZAR history only. Pending status rows are excluded. Nonzero embedded service fees stop with TRANSACTION_FEE_REVIEW_REQUIRED until representation is owner-verified. Unknown layout/date/sign/currency/status fails closed. Logout confirmation is required before any preview is stored; failure suspends automatic refreshes.

Normalized snapshots enter the ordinary durable import preview, rules/classification-version check and explicit Confirm import. Cross-format and overlapping candidates require keep/skip; bank references remain provenance, not FITIDs. Repeated identical snapshots reuse their original staged/committed import and run provenance. Original run IDs, row references, returned dates/counts and possible-gap/150-row warnings survive commit/history. All imported entries require review; no automatic ledger commit or approval. Transaction fetches do not replace account balances or their successful-refresh timestamp.

This first delivery is on-demand only. Existing schedules still update accounts/balances. First-page coverage is always potentially incomplete; pagination, historical checkpoints, stable bank IDs and fee semantics remain outstanding. Live navigation/table/posting compatibility is unverified and must be tested by the owner from Imports, using saved Debug mode if needed. Share only fixed error codes and count-only diagnostics from Settings → Banking → Troubleshooting; never credentials, real rows, banking HTML or screenshots. The agent did not run a live login or read credentials/key/data.

## Todo: consistent loading indicators (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md.

- [x] Audit asynchronous actions and data loads across the application and show a loading spinner while work is pending, including button-triggered requests, account/balance loading and refreshes, saves, imports and paginated/infinite-scroll fetches.
- [x] Use a shared, subtle spinner treatment that fits the UI. Show it within the triggering button for actions, or beside/in the affected content for data loads; keep labels clear and avoid shifting layout or replacing useful existing data unnecessarily.
- [x] Prevent duplicate submissions while the same action is pending, retain appropriate navigation and cancel controls, and clear loading state on success, failure or cancellation. Preserve inline field errors and overlay toasts for their existing purposes.
- [x] Make pending state accessible through appropriate busy/status announcements and respect reduced-motion preferences. Verify initial loads, refreshes and failure/retry paths so no action appears idle while waiting or remains stuck loading.

## Todo: verify and complete scheduled banking workflow (2026-10-03)

Authorized outstanding backend milestone. This is the desired integrated workflow to audit and complete, not a claim that scheduled transaction retrieval is working. Current schedules update accounts/balances; on-demand transaction staging is implemented, while scheduled transaction retrieval remains outstanding.

- [ ] Audit banking setup and automation end to end. Once the user has configured the connector and enabled scheduled refresh, each scheduled run should discover newly available accounts and refresh account/balance information while preserving saved hidden states and existing account permissions.
- [ ] Fetch new transactions for non-hidden accounts, including newly discovered eligible accounts, through the authorized connector/import workflow. Hidden accounts must remain excluded from transaction syncing; discovery must not silently unhide them or broaden sharing/access.
- [ ] Apply the active classification rules to retrieved transactions using the same precedence, account scope, category/group and direction behavior as ordinary imports. Preserve existing conflict handling and explicit transfer semantics.
- [ ] Leave imported/classified transactions pending user review. Rule matches are suggestions/classification, never automatic review approval. Make the outcome and any partial failure clear so the user can find and review new transactions.
- [ ] Verify first setup, manual and scheduled runs, newly discovered accounts, hide/show persistence, repeated/overlapping fetches without duplicate transactions, rule matches/conflicts and unmatched transactions, permission changes, restart/schedule persistence and failure recovery. Retain exact-money, provenance and human-review requirements.
- [ ] Use synthetic provider responses and automated checks for development; live bank login, credentials and compatibility verification remain owner-operated. Record what is implemented, missing and verified before declaring the complete automation functional.

## Scraper layout follow-up (2026-10-03)

Owner reports OFX exports contain the same transactions as the displayed history and chooses to retain scraping. Owner-run diagnostics confirmed logout and account identity/type, but found 60 row elements and zero old-style header elements. Updated header discovery supports semantic headers, a named header row and unique named labels above the rows within their shared container; no amount-based or positional column guessing. Successful/Pending controls are explicitly recognized; the worker selects Successful and requires selected/checked/ARIA evidence before treating a toggle table as posted history. Merely displaying Successful is insufficient, and selected Pending/ambiguous tables fail closed. Header whitespace is normalized. Additional fixed counters report Successful/Pending control counts and selected Successful state.

Synthetic browser-shaped pages verify this compatibility change without inspecting real banking. Actual bank DOM compatibility still requires owner retry via Imports → Fetch FNB transactions. The 150-row/incomplete-history warning and original review, duplicate, fee and logout checks remain. No OFX download automation was added.

## Accepted service-fee treatment and all-account fetch (2026-10-03)

Owner confirms the bank balance changes by Amount minus Service Fee. This supersedes the initial nonzero-fee compatibility block. The browser preserves exact nonnegative service_fee_decimal alongside the unchanged bank amount. Go normalization creates the principal entry plus one negative Service Fees entry for each nonzero fee; their total equals the accepted bank movement. Credits retain their positive principal with a separate negative fee. Fees participate in the ordinary account-scoped/built-in classification rules and mandatory review, and never receive an assumed FITID. Run ID, original bank row, component, bank description, reference and fee cents link both entries in immutable source provenance. Reported coverage remains in bank rows, with additional fee-entry counts separate; 150 bank rows can produce up to 300 normalized entries.

The prior fee exception aborted before the first account completed; transaction_accounts=0 meant no account previews had completed, not a one-account design. Synthetic tests now verify a fee-bearing first account followed by a second eligible account and both previews/commits. Requested/completed account and fee-row counters are allowlisted. Unknown layouts, account identity, malformed/negative fees, permission changes and unconfirmed logout still reject atomic staging.

Combined Amount-minus-Fee export records are flagged as possible duplicates on both new components, requiring explicit keep/skip choices. Repeated snapshots and repeated commits do not add another fee. Banking transaction errors now direct users to Fetch FNB transactions in Imports rather than suggesting credential replacement. Actual all-account live navigation and fees remain owner-run verification; no bank/credential/key observation by the agent.

## Todo: account refresh actions (2026-10-03)

Implementation authorized by the owner and completed on 2026-10-03. See UI backlog delivery and VERIFICATION.md. Include this in the account-management refactoring above and use the shared loading-indicator treatment.

- [x] Add a Refresh button to the main Accounts page to refresh all non-hidden accounts, using the authorized banking refresh workflow and keeping hidden accounts excluded.
- [x] Add an individual refresh action to each account card that updates only that account. Show the same shared loading spinner while that account is being updated, whether triggered individually, by the page-wide refresh or by a scheduled run.
- [x] Show pending state on the page-wide refresh control while its request is running and prevent duplicate/overlapping refresh requests. Keep each card's busy state accurate and clear it on completion or failure, with existing toast/error conventions.
- [x] If edit, refresh, hide/show and other card actions make the layout crowded, group them in a compact hamburger dropdown menu on each card. Keep actions easy to find, keyboard accessible and consistent with the planned subtle disclosure styling; keep account update status visible even when the menu is closed.
- [x] Preserve permissions, hidden-state persistence and stored history. Verify page-wide versus single-account scope, concurrent/scheduled updates and loading/error states using synthetic data.

## UI backlog delivery authorized (2026-10-03)

The owner explicitly requested implementing the outstanding handover items and created scope of at least all UI tasks. This supersedes the recorded-only authorization notes for account consolidation, rules/list pagination, disclosures, loading indicators and account refresh actions.

Implemented shared account edit/hide/show controls in Accounts and Settings → Accounts, paged hidden restoration, connector-only Banking, all-visible and single-account refresh with accurate manual/scheduled busy status, shared subtle native disclosures and accessible loading controls. Large list endpoints and frontend workflows now page end to end: rules, accounts, categories/groups, periods/limits/affected rows, users/access, imports/history/rows/suggestions, transaction/audit lists, dashboard display rows and lookup selectors. Server search/permissions/order/snapshots, full aggregates, all-current rule scope, draft/selection persistence and explicit import/review actions are retained. See ARCHITECTURE/UI for the accepted contracts and VERIFICATION for actual coverage.

Scheduled banking workflow remains outstanding as an integrated backend milestone. Existing schedules discover/update accounts and balances; transaction retrieval is on-demand. This delivery does not introduce scheduled transaction staging, automatic ledger commits, historical checkpoints or complete bank pagination. Vault22 budget-management candidates still need the recorded product decisions. Live bank compatibility remains owner-operated.

2026-10-03 disclosure refinement: owner rejected the small native white arrow. Replaced it across shared disclosure headers with a muted outlined right-aligned chevron that rotates on expansion. Native interaction/focus, theme colors and reduced-motion behavior remain; account hamburger menus keep their existing icon. Production build and four synthetic desktop/mobile/theme structural checks passed; see VERIFICATION.md.


## Home-loan transaction compatibility (2026-10-03)

Owner retry completed two of seven accounts before TRANSACTION_ACCOUNT_UNSUPPORTED; the owner identifies the third account as a ZAR home loan. Home Loan product labels now map explicitly to the Home Loan report type, accepted by both worker and Go normalization. The existing full-identity, named-column, verified-posted, exact signed amount/fee, currency, logout and atomic-staging checks apply unchanged. Bank signs are preserved without inferring repayment, transfer, interest or principal semantics. Unknown product types and non-ZAR still reject; no silent skips or partial previews. Fixed counters distinguish unsupported type/currency and identify only the failed account's one-based request position.

Synthetic worker navigation and backend staging tests exercise seven accounts with a home loan third and retain all subsequent accounts, exact debit/credit signs, source provenance, incomplete-history coverage and explicit confirmation. Owner live retry remains necessary; the agent did not inspect bank pages, credentials, keys or financial data.

2026-10-03 account menu follow-up: centered the hamburger icon in a 44px square shared trigger. Account menus dismiss on outside pointer interaction, focus leaving the menu, opening another account menu, or Escape. Event listeners are removed on unmount; outside interaction retains normal target behavior and Escape restores trigger focus. Applies to Accounts and Settings → Accounts through shared AccountActions. Verification is recorded in VERIFICATION.md.

2026-10-03 file chooser refinement: native file-selector buttons now match shared secondary buttons, including theme surfaces/text/borders, 6px radius, typography, hover and 44px height. Removed the dark native-input background strip; filename text uses the muted token. Applies to export uploads and discovery-file inputs; native selection behavior remains. Verification is in VERIFICATION.md.

2026-10-03 loading presentation follow-up: button spinners now align to the vertical center instead of the top corner, including wrapped labels, without changing button dimensions. Removed trailing ellipses from all spinner-backed loading/action labels; search placeholder punctuation remains. Updated matching browser assertions. Verification is in VERIFICATION.md.


2026-10-03 Money Maximizer compatibility: owner debug observation identifies the failing product as a ZAR Money Maximizer savings account, superseding the earlier assumption that request position three was the home loan. Request positions follow stored account IDs, not bank display order. The transaction Type matcher now recognizes Money Maximizer and Money Maximiser as Savings, using product labels rather than account nicknames. All existing identity, exact signed amount/fee, posted history, ZAR, logout, duplicate and review checks remain. Global/eBucks labels are not mapped to Savings; balance discovery retains its non-ZAR/reward unit exclusions. Live compatibility still requires owner retry. No real nicknames/transactions were added to fixtures.


2026-10-03 home-loan table layout: owner retry completes six of seven accounts; last home-loan page exposes Effective Date, Description, Amount, Balance with no Successful/Pending controls. Effective Date is now a named date alias in normalization and all header-discovery paths. A verified Home Loan detail identity plus the exact four-column effective-date/running-balance contract supplies loan-history evidence when no status controls are present; any pending heading or displayed unselected/pending controls still blocks it. Other account types do not receive this fallback. Signed repayments/interest, standalone charge rows and zero adjustments remain unchanged; no extra fee is generated without a Service Fee column. Unknown layout, currency, identity and logout checks still reject atomic staging. Coverage remains potentially incomplete. No screenshot financial values were copied to fixtures; live compatibility remains owner retry.

2026-10-03 export drop zone: removed the duplicated native chooser row. The centered upload box is a full-width button opening a hidden native multiple-file input, with file drag/drop using the same selection state. Selected names appear once; another selection/drop replaces them. Drag feedback, Enter/Space activation and busy guards are supported. Selection never automatically previews or imports; existing server validation and explicit confirmation remain. Discovery-file chooser is unchanged. Build and four synthetic compiled-app browser checks passed; see VERIFICATION.md.

2026-10-03 button spinner spacing: replaced absolute spinner placement inside padding with the shared inline flex layout. Standard 14px spinner and 8px gap keep text separate and vertically aligned, including wrapped labels. Updated the account browser assertion to check real text separation instead of the superseded fixed-width busy-button convention. Build, seven unit tests and eight synthetic browser geometry combinations passed; see VERIFICATION.md.


## Whole-program standardization (2026-10-03)

Owner requested premium, consistent polish throughout the program. Implemented shared design tokens and readable stylesheet, neutral themes, more readable supporting text, consistent panels/actions/dialogs/menus, selected navigation/tabs, common PageHeader with compact refresh status, keyboard skip navigation, common transaction pagination and aligned transaction heading tracks. Shared Field reserves a feedback line to prevent first-blur validation shifting a mobile action during its tap. Updated UI.md records conventions. Financial behavior, permission scope, banking automation and stored data are unchanged. See VERIFICATION.md for actual automated coverage; no computer-use visual review was performed.
