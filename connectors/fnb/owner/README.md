# Owner-run FNB account discovery

This is an owner-operated compatibility test. The agent does not run live banking, enter credentials, observe the window, read the exported account file, or inspect browser memory/session files. Same-host operation cannot prevent a privileged host operator from accessing secrets. The active tracker connection uses the local worker described below.

The script opens a temporary incognito Chrome/Edge window. Sign in directly on FNB and complete MFA/device approvals yourself. Open Accounts, then press Enter in the terminal. The script reads only the account nickname/number text using selectors adapted from the pinned GPL-3.0 reference. It does not automate login, read credential fields, accept credential environment variables, access cookies, capture screenshots/HTML/debug logs, navigate transaction tabs, read balances, or save credentials.

Account-layout compatibility remains unverified until the owner runs it. Hidden elements and input controls are excluded; empty/mismatched visible lists fail closed. Masked account numbers may appear in the metadata report; review identity/type before creating accounts. The automatic worker separately verifies exact masked Credit summary/detail matches.

## Setup

Use Node 22.12+ and an installed Chrome/Edge. Install only the pinned modern puppeteer-core dependency with `npm ci --ignore-scripts` in this directory. This does not install or run the old fnb-api package or download a browser. The agent may install/test dependencies using synthetic data; the owner alone runs the live command.

On Windows, run the workspace's Test-FnbAccounts.ps1 launcher in your own PowerShell terminal. It starts this script with Windows Node and your installed browser. It never reads or sets bank credentials. Alternatively run `node run.mjs --browser /path/to/chrome` from your terminal on a supported desktop.

The default account file is saved in your Downloads folder with a unique fnb-accounts filename, outside the source tree. It contains account names/numbers, not credentials, balances or transactions. Keep it private; import it through Settings → Accounts → Import discovered accounts, review/edit each item and explicitly create selected accounts as Private or Household. Existing accounts are not changed. Do not paste the report into chat; share only fixed error codes/counts.

For a synthetic test, run `node run.mjs --mock --output /outside/workspace/synthetic-accounts.json`. Tests never launch a browser or contact FNB. `node --test` checks metadata filtering, masking, duplicates, invalid layouts and bank-domain checks.

The browser profile is created in the OS temporary directory outside the workspace and removed after normal completion. Force-killing the process or machine failure may require manual cleanup. Do not use an existing personal browser profile, no-sandbox flags, remote debugging exposed to the network or persistent login caches. Manual browser login can still process secrets in Chrome memory: local masking/environment variables cannot provide an isolation boundary.

Attribution: selectors derived from bitshiftza/fnb-api commit 3ddba8f104d2b356d491100755cd2562e02a6dd4; original license at ../upstream/LICENSE. New adapter and owner runner are GPL-3.0. Local adaptation replaces automatic credential handling with owner login, drops all balances/transactions and uses modern puppeteer-core.

## Layout failures

For ACCOUNT_LAYOUT_CHANGED, retry with `--diagnose` to print LAYOUT_COUNTS on extraction failure: only fixed integer counters for named/visible fields and invalid/duplicate metadata. No account text, arbitrary attributes, HTML, URLs, screenshots or credentials are printed. Share only the fixed error and that counts line. The agent must not run this mode on a live bank session. Diagnostics help narrow the mismatch without broadening extraction to balance/transaction content.

If PowerShell blocks the launcher, run Windows Node directly: `node "\\wsl.localhost\Ubuntu\home\douw\finance-tracker\connectors\fnb\owner\run.mjs" --diagnose`. No execution-policy change is needed.

For an otherwise valid list with an unsupported account-number field, `--skip-unsupported` explicitly allows a partial export. It skips only invalid number formats, prints PARTIAL_DISCOVERY with count and one-based positions in extracted summary order (no account text), and keeps the same strict metadata-only report. Review the exported items before adding them; add omitted accounts manually if needed. Invalid names, duplicate valid IDs, oversized/mismatched lists and zero valid entries still fail. The default remains all-or-nothing.


## Automatic tracker connection

Go launches `refresh.mjs` for account discovery, ledger balances and recent transaction fetching. Enter credentials only in Settings → Banking; encrypted SQLite storage uses an external runtime key. Manual and scheduled refreshes default to headless; the saved Show Chrome setting enables owner observation/approvals. Scheduling starts disabled and includes posted transactions for mapped, visible editable accounts.

The worker checks FNB login form/origin, submits once and validates account identity, ZAR money and named history columns. Unsupported rewards/foreign currency are excluded. Exact masked Credit identities and supported loan/savings layouts follow the [current connector contract](../README.md). Missing approval, uncertain layout, identity mismatch and unconfirmed logout stop the run and pause automatic attempts.

Cleanup requires visible signed-out fields or explicit completed logout without authenticated markers. Session-conflict warnings require the owner to let the previous session end or sign out before retrying. Balance extraction uses ledger balance, never available balance; unavailable placeholders remain unknown rather than zero. Diagnostics expose fixed codes/numeric counts, including startup phase and logout confirmation, without page text or financial values.

Never invoke the worker live from agent tools or place credentials in arguments, environment, fixtures or logs. Share only the fixed failure code and counts. Forced termination can leave an OS temporary `finance-fnb-refresh-*` profile; close the temporary window and remove it yourself. Do not copy profiles into source or ask agents to inspect them.

See [runtime setup](../../../docs/FNB-RUNTIME.md) for executable/key configuration and [archived owner notes](../../../docs/archive/2026-10-07/FNB-OWNER.md) for earlier findings. Live reliability remains owner-verified.
