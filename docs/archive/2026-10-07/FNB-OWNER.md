> Historical record, archived 7 October 2026. This describes earlier development and includes superseded decisions. Use the [current documentation](../../README.md) for new work.

# Owner-run FNB account discovery

This is an owner-operated compatibility test. The agent does not run live banking, enter credentials, observe the window, read the exported account file, or inspect browser memory/session files. Same-host manual operation is an agreed workflow, not a technical secrecy guarantee. The separately isolated service remains the long-term option for strict secrecy and scheduled syncing.

The script opens a temporary incognito Chrome/Edge window. Sign in directly on FNB and complete MFA/device approvals yourself. Open Accounts, then press Enter in the terminal. The script reads only the account nickname/number text using selectors adapted from the pinned GPL-3.0 reference. It does not automate login, read credential fields, accept credential environment variables, access cookies, capture screenshots/HTML/debug logs, navigate transaction tabs, read balances, or save credentials.

Account-layout compatibility remains unverified until the owner runs it. Hidden elements and input controls are excluded; empty/mismatched visible lists fail closed. Masked account numbers may appear in the exported report; the tracker requires the owner to enter the full number before creating that account.

## Setup

Use Node 22.12+ and an installed Chrome/Edge. Install only the pinned modern puppeteer-core dependency with `npm ci --ignore-scripts` in this directory. This does not install or run the old fnb-api package or download a browser. The agent may install/test dependencies using synthetic data; the owner alone runs the live command.

On Windows, run the workspace's Test-FnbAccounts.ps1 launcher in your own PowerShell terminal. It starts this script with Windows Node and your installed browser. It never reads or sets bank credentials. Alternatively run `node run.mjs --browser /path/to/chrome` from your terminal on a supported desktop.

The default account file is saved in your Downloads folder with a unique fnb-accounts filename, outside the source tree. It contains account names/numbers, not credentials, balances or transactions. Keep it private; import it through Settings → Accounts → Import discovered accounts, review/edit each item and explicitly create selected accounts as Private or Household. Existing accounts are not changed. Do not paste the report into chat; share only fixed error codes/counts.

For a synthetic test, run `node run.mjs --mock --output /outside/workspace/synthetic-accounts.json`. Tests never launch a browser or contact FNB. `node --test` checks metadata filtering, masking, duplicates, invalid layouts and bank-domain checks.

The browser profile is created in the OS temporary directory outside the workspace and removed after normal completion. Force-killing the process or machine failure may require manual cleanup. Do not use an existing personal browser profile, no-sandbox flags, remote debugging exposed to the network or persistent login caches. Manual browser login can still process secrets in Chrome memory: local masking/environment variables cannot provide an isolation boundary.

Attribution: selectors derived from bitshiftza/fnb-api commit 3ddba8f104d2b356d491100755cd2562e02a6dd4; original license at ../upstream/LICENSE. New adapter and owner runner are GPL-3.0. Local adaptation replaces automatic credential handling with owner login, drops all balances/transactions and uses modern puppeteer-core.

## Layout failures

The owner reported ACCOUNT_LAYOUT_CHANGED after manual login. This is not yet proof of current bank compatibility. Retry with `--diagnose` to print LAYOUT_COUNTS on extraction failure: only fixed integer counters for named/visible fields and invalid/duplicate metadata. No account text, arbitrary attributes, HTML, URLs, screenshots or credentials are printed. Share only the fixed error and that counts line. The agent must not run this mode on a live bank session. Diagnostics help narrow the mismatch without broadening extraction to balance/transaction content.

If PowerShell blocks the launcher, run Windows Node directly: `node "\\wsl.localhost\Ubuntu\home\douw\finance-tracker\connectors\fnb\owner\run.mjs" --diagnose`. No execution-policy change is needed.

For an otherwise valid list with an unsupported account-number field, `--skip-unsupported` explicitly allows a partial export. It skips only invalid number formats, prints PARTIAL_DISCOVERY with count and one-based positions in extracted summary order (no account text), and keeps the same strict metadata-only report. Review the exported items before adding them; add omitted accounts manually if needed. Invalid names, duplicate valid IDs, oversized/mismatched lists and zero valid entries still fail. The default remains all-or-nothing.

## Automatic tracker connection

The tracker now also uses refresh.mjs through Go stdin/stdout for account/ledger-balance refresh. The owner accepted same-machine storage limitations. Enter credentials in the tracker's Settings → Banking → FNB connection form; credentials are encrypted in SQLite, with an external runtime key generated by Go. Refresh now opens a visible temporary browser for owner approvals; schedules use a headless browser. Never invoke refresh.mjs live from agent tools or put bank credentials in arguments, fixtures, environment variables or debug logs.

The worker types into same-origin FNB login fields only after checking their form origin, submits once, then opens Accounts. These legacy login selectors and the ledger balance layout are not yet live verified. Missing approval results in APPROVAL_REQUIRED and pauses automatic attempts. Fixed errors only; do not share real credential input or banking output. Share the fixed failure code. Hidden IDs suppress balance text reads and no transaction tabs/routes are implemented. Unsupported number formats such as rewards are skipped with a count. Balance parsing accepts exact ZAR amounts with two decimals; ambiguous/foreign amounts fail.

Forced process/machine termination can leave an OS temporary finance-fnb-refresh-* browser profile; close the temporary bank window and remove that profile yourself if needed. It must never be copied into the workspace or inspected by the agent.

Browser mode update: both manual and scheduled refreshes are headless by default; the saved debug setting controls visible Chrome and fronts its page. Debug adds no sensitive logs/screenshots/HTML. Missing ledger fields retry briefly for asynchronous rendering. Explicit unavailable-value placeholders keep balances unknown, not zero; unexpected formats fail with safe numeric counters. Live BALANCE_LAYOUT_CHANGED was owner reported and is not yet confirmed fixed.

Refresh cleanup now attempts explicit FNB logout (exact Log out/Logout/Sign out control), confirms visible login fields, then destroys its temporary browser. This also applies after extraction errors. If logout is missing/unconfirmed for an authenticated session, LOGOUT_REQUIRED pauses the connection and discards the run's snapshot; sign out manually before retrying. Current FNB logout compatibility remains owner-test dependent. Diagnostics now include count-only fallback data when DOM evaluation fails.

Automatic refresh recognizes Log Off, Log out and Sign out controls and confirms signed-out login fields before browser cleanup. A visible session-termination/logged-out-shortly warning ends account waiting promptly with SESSION_CONFLICT; let the prior session end/sign out before manually retrying. Both conflict and unconfirmed logout pause scheduling and discard the snapshot. Balance extraction selects ledgerBalance only, never Available balance; ZAR amounts with zero to two decimal places, plus or Unicode minus normalize exactly to two decimal places. Unknown formats remain errors with fixed numeric shape counters; no amounts/page text are returned in diagnostics.

2026-10-02 logout confirmation follow-up: owner observed actual logout with the message “You have successfully logged out of banking,” despite LOGOUT_REQUIRED. Worker now accepts explicit completed logout text with no visible authenticated markers, or visible login fields, on approved bank pages/frames; retries navigation races for up to 15 seconds. Impending logout/session-termination text does not confirm completion. Fixed numeric logout_clicked/logout_confirmed/logout_unconfirmed counters distinguish cleanup from balance problems; balance_failure is retained if cleanup also fails. Confirmed cleanup preserves the original balance error. Ledger extraction uses rendered innerText rather than hidden-descendant textContent; unknown text remains a format error with fixed balance_label/unavailable_text/loading_text counters. No bank text/values in diagnostics and no agent live session access. Live amount format remains pending owner retry.

2026-10-02 balance currency follow-up: owner-provided screenshot identifies numeric-identity eBucks and foreign-currency ledger rows as unsupported by the ZAR-only tracker. Worker now skips recognized reward/foreign currency units, reports fixed reward_entries/non_zar_entries counters, and continues the supported ZAR snapshot. Masked identifiers remain skipped. No conversions or unit stripping into ZAR; unfamiliar text still fails closed. Previously stored accounts/history remain intact. Banking skipped-entry copy explains number/unit exclusions. Screenshot financial values/identities are not copied into fixtures; all tests use synthetic values. Owner diagnostics confirm logout; next live balance refresh remains owner-run.
