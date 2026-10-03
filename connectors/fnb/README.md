# FNB compatibility prototype

Status: mock normalization only. No live provider, credential input, credential storage, account creation, scheduling, bank request or import commit is enabled.

## Pinned reference

`upstream/` contains the original TypeScript source, README, GPL-3.0 license, package manifest/lock and TypeScript configuration from https://github.com/bitshiftza/fnb-api at commit `3ddba8f104d2b356d491100755cd2562e02a6dd4` (2023-01-05). Attribution, pin and local changes are recorded in UPSTREAM.json. No upstream source changes; no npm installation or execution was performed. The demonstration test program was deliberately excluded. Upstream licensing remains attached to the copied source; preserve its notices and review distribution requirements before shipping a derivative connector.

## Source review findings

- Upstream package 1.0.11 declares Puppeteer ^19.4.1 and Moment ^2.24.0; the lockfile root still says 1.0.10. There is no engines declaration. Current Node/Chromium compatibility remains unverified. A lockfile-only npm audit on 2026-10-02 found 9 production dependency advisories (8 high, 1 moderate), recorded in DEPENDENCY-AUDIT.json. No dependencies were installed or executed. Update/review dependencies before live execution.
- Login is browser DOM automation against fnb.co.za selectors and relies on page-global jQuery. There is no explicit MFA/approval state machine or pagination implementation. Do not advertise supported current login based on this source.
- The README limits successful transactions to the first page (150 entries). A shorter page is not proof of full history either. Credit entries lack the cheque/savings reference field. References must not be assumed to equal CSV/OFX identities.
- Monetary parsing repeatedly uses parseFloat, multiplication and rounding. The prototype contract requires decimal text from the bank before this conversion; copying upstream numeric results cannot establish exact-money correctness.
- `_isLoggedIn` returns true for a closed page; `_login` calls close without awaiting it; Api.close also drops the close promise. Session validity is inferred from a 100-second age instead of verifying bank session state. These need correction in a future working adapter.
- Credentials remain in the Api options object, passed into browser fields. The cache is process memory. The demo prints financial results, and must not be integrated. No raw banking HTML, browser profiles, credentials, screenshots or result logging is permitted in the future worker.
- Fee representation is not established. The mock normalizer rejects a nonzero separate service fee rather than inventing an additional ledger expense or double-counting fees.

## Implemented mock contract

`internal/app/fnb_adapter.go` accepts explicitly mapped bank identity, ZAR, supported provisional account types, ISO bank calendar dates, signed decimal amount strings, optional dated balances, source references and a run ID. It normalizes into existing SourceRow/ParsedFile shapes using the exact Cents parser. Unknown currencies/types, invalid dates/amounts/statuses and account mismatches fail with sanitized errors. All reported transactions must be verified posted entries; pending-authorisation semantics remain unresolved.

Reference identifiers remain separate provenance and never become FITIDs. Identical purchases remain separate rows. Coverage reports the dates actually returned, always signals possible gaps, and marks the 150-row limit. Empty results imply no coverage. Normalization has no authority to grant account access, apply categories, approve rows, deduplicate candidates, create accounts or write the ledger. Tests use synthetic reports only.

This function is not an import service: it does not produce immutable persisted provenance/hash/run records, enforce a connection's current permissions, or stage/commit. Those are required at the later shared staging boundary. Never route normalized output directly into a trusted ledger insert.

## Deployment decision required before live credentials

| Option | Operational impact | Meets strict agent secrecy? |
| --- | --- | --- |
| Worker process in current container | Node/Chromium and secret provisioning added to Go image; shared host/service identity | No: unrestricted host access can reach key/process memory |
| Separate container on this host | Better process separation and resource limits; shared host still controls it | No under this agent's unrestricted host access |
| Independently operated service outside agent host access | Separate deployment/secret lifecycle; Go consumes authorized sanitized reports | Can provide the necessary boundary if host access, logs, secrets and browser sessions are isolated |

Proposed direction for the strict requirement: independently deployed connector with a service-owned secret store, authenticated encrypted transport and least-privilege report delivery. No public port or deployment change is made by this prototype. The owner's secret remains write-only to that service; Go receives status/account discoveries/reports only. Even administrators cannot retrieve credentials through an API. A live compatibility UI must be hosted across this boundary and used by the owner without agent observation.

Next steps: resolve hosting/isolation; review dependencies and supported runtime; owner-managed login/MFA/account coverage test; patch exact text parsing and lifecycle; define fees/posting/identity/coverage; then connection mapping, normalized staging, mandatory human review and a disabled-by-default scheduler with execution-time permission checks and recoverable checkpoints. Existing CSV/OFX imports remain available.

## Accepted isolation direction and scaffold

The owner selected a separately isolated service on 2026-10-02. service/ contains a dependency-free Node mock HTTP service and AES-GCM primitives, with five synthetic tests covering authenticated read-only routes, fail-closed startup/mode, nonce freshness, identity binding, tampering, missing keys and rotation. The encryption primitives are disconnected from storage and HTTP. No live credentials are accepted. External hosting details/provisioning are still required; this local prototype does not establish the eventual isolation boundary.

## On-demand live transaction preview (2026-10-03)

The owner worker now accepts transaction_accounts and a server run_id over stdin, alongside existing encrypted-connection credentials supplied by Go. Imports → Fetch FNB transactions requests visible, previously mapped editable accounts only. Login first establishes the supported ZAR account summary; navigation matches full bank identity (never nickname), then the transaction tab/detail identity and named table headers are validated. selectors/navigation are derived from the unchanged pinned GPL reference; local extraction uses no jQuery/Moment/float money parser and adds no dependencies. A posted-history heading or explicit posted/successful/completed status is required; explicit Pending rows are omitted. Exact dates/decimal amounts/references/running balances normalize into the ordinary staging contract. No bank reference is promoted to FITID.

Only the first page (maximum 150 raw rows) is implemented. All coverage is potentially incomplete; pending exclusions cannot suppress the 150-row warning. Nonzero embedded service fees produce TRANSACTION_FEE_REVIEW_REQUIRED rather than a guessed ledger charge. Unknown headers, selected identity, currency, status or amount format stop atomically. Strict selectors/posted evidence are intentionally compatibility-dependent; live owner verification remains required. Logout must confirm before Go stores previews. No live login was executed by the agent.

From Imports fetch, inspect suggestions and duplicate candidates, Confirm import, then review accepted transactions. Settings → Banking → Troubleshooting reports fixed error codes and numeric transaction counters in the existing BALANCE_COUNTS line. TRANSACTION_LAYOUT_CHANGED means the current bank layout could not be established; it does not establish that the account is empty. Do not share real reports/page text/HTML/screenshots or credentials with the agent. Existing schedule updates accounts/balances only. Pagination, stable per-entry IDs, fee semantics, full-history reconciliation and scheduled transaction policy remain future work.

2026-10-03: owner reports OFX and displayed history have identical scope and prefers scraping. Header extraction now supports unique named labels above data rows and semantic/header-row layouts without depending solely on old tableHeader classes. Worker selects Successful and checks selected/checked/ARIA state; visible Pending controls alone no longer invalidate a selected Successful table, while selected Pending remains rejected. Numeric diagnostics include transaction_successful_controls, transaction_pending_controls and transaction_selected_successful. No download automation or expanded-history claim. Run synthetic browser DOM checks with the installed dev Playwright via `node scripts/test-fnb-dom.mjs` from the project root.

2026-10-03 fee follow-up: owner confirms Amount minus Service Fee is the bank movement. Exact nonnegative fees are retained in reports rather than aborting the first account. Go staging expands principal plus negative Service Fees entry, links source bank row/run/reference/fee metadata, and applies normal rules and review. Combined net exports are possible duplicate candidates on both entries. Fee expansion does not change the raw 150-bank-row limit. The worker continues to all requested eligible accounts; requested/completed/fee-row numeric diagnostics distinguish progress. Negative or malformed fees remain rejected. Actual multi-account bank navigation is still owner-verified only.


## Home-loan transaction compatibility (2026-10-03)

Owner retry completed two of seven accounts before TRANSACTION_ACCOUNT_UNSUPPORTED; the owner identifies the third account as a ZAR home loan. Home Loan product labels now map explicitly to the Home Loan report type, accepted by both worker and Go normalization. The existing full-identity, named-column, verified-posted, exact signed amount/fee, currency, logout and atomic-staging checks apply unchanged. Bank signs are preserved without inferring repayment, transfer, interest or principal semantics. Unknown product types and non-ZAR still reject; no silent skips or partial previews. Fixed counters distinguish unsupported type/currency and identify only the failed account's one-based request position.

Synthetic worker navigation and backend staging tests exercise seven accounts with a home loan third and retain all subsequent accounts, exact debit/credit signs, source provenance, incomplete-history coverage and explicit confirmation. Owner live retry remains necessary; the agent did not inspect bank pages, credentials, keys or financial data.


2026-10-03 Money Maximizer compatibility: owner debug observation identifies the failing product as a ZAR Money Maximizer savings account, superseding the earlier assumption that request position three was the home loan. Request positions follow stored account IDs, not bank display order. The transaction Type matcher now recognizes Money Maximizer and Money Maximiser as Savings, using product labels rather than account nicknames. All existing identity, exact signed amount/fee, posted history, ZAR, logout, duplicate and review checks remain. Global/eBucks labels are not mapped to Savings; balance discovery retains its non-ZAR/reward unit exclusions. Live compatibility still requires owner retry. No real nicknames/transactions were added to fixtures.


2026-10-03 home-loan table layout: owner retry completes six of seven accounts; last home-loan page exposes Effective Date, Description, Amount, Balance with no Successful/Pending controls. Effective Date is now a named date alias in normalization and all header-discovery paths. A verified Home Loan detail identity plus the exact four-column effective-date/running-balance contract supplies loan-history evidence when no status controls are present; any pending heading or displayed unselected/pending controls still blocks it. Other account types do not receive this fallback. Signed repayments/interest, standalone charge rows and zero adjustments remain unchanged; no extra fee is generated without a Service Fee column. Unknown layout, currency, identity and logout checks still reject atomic staging. Coverage remains potentially incomplete. No screenshot financial values were copied to fixtures; live compatibility remains owner retry.
