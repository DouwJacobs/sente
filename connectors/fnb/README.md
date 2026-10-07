# FNB connector

Sente uses the local `owner/refresh.mjs` browser worker for administrator-owned account discovery, dated ledger balances and recent posted transaction imports. The tracker sends credentials/targets over stdin and accepts bounded JSON reports; the worker has no public port. [Runtime setup](../../docs/FNB-RUNTIME.md) covers Node/browser configuration, encryption keys and deployment limits.

## Current behavior

Enter login details in Settings → Banking. Credentials are encrypted with an external owner-bound key; same-host encryption does not prevent privileged host access. Both manual and scheduled jobs default to headless. Saved debug mode shows Chrome for owner approvals without logging bank content. Runs serialize, validate approved origins and confirm logout before storing results. Attention failures pause scheduling.

Scheduling starts disabled. Scheduled runs refresh balances and import posted rows for mapped, visible editable accounts. Initial discovery without mapped targets remains account-only. New accounts default Private with an owner editor grant; existing account sharing/visibility is preserved. Manual Get transactions uses the same validated import service, while balance-only refresh remains available.

Supported ZAR products include Cheque, Savings (Money Maximizer/Maximiser), Credit, Easy and Home Loan. Identity must be unique and verified; Credit may use the exact matching masked summary/detail number without guessing digits. Changed masks are separate identities. Reward and foreign-currency balances are excluded. Home Loan history supports verified Effective Date/Description/Amount/Balance tables without status controls. Unknown identities/types/layouts/currency reject the run.

Money is parsed from decimal text without floating point. Nonnegative service fees produce a separate negative Service Fees entry alongside unchanged principal. Bank movement is Amount minus Service Fee. Pending authorizations are excluded. Only the first recent-history page is implemented, capped at 150 raw bank rows; coverage always warns that history may be incomplete.

Live repeat matching consumes source occurrences using account, immutable date/amount/description and available running balances/references. References/page positions are not FITIDs. Same-page identical purchases remain distinct; indistinguishable purchases outside the visible window may be missed. Clean imports apply current rules and automatically accept complete categories, starting unseen. Missing categories/conflicts require category review; invalid exports, bank-ID conflicts and ambiguous uploaded CSV/OFX candidates retain Import activity decisions.

## Source and tests

`internal/statements` owns FNB normalization and coverage; `internal/app` owns authorized staging/commit, connection storage, subprocess handling and scheduling. Worker extraction/navigation is in `owner/refresh.mjs` and `owner/transactions.mjs`. Browser, MCP and offline services retain shared financial invariants.

`upstream/` preserves bitshiftza/fnb-api at commit `3ddba8f104d2b356d491100755cd2562e02a6dd4` (2023-01-05), with GPL-3.0 notices and `UPSTREAM.json` attribution. Preserve those notices. The active worker uses modern puppeteer-core and local exact-money/lifecycle checks rather than executing the old API runtime. `DEPENDENCY-AUDIT.json` records a historical 2 October review of upstream dependencies; it is not a current dependency-health claim.

Run synthetic worker tests with `node --test` in `owner/`, and DOM checks with `node scripts/test-fnb-dom.mjs` from the root using the Linux Node runtime. Backend statement/import/connection tests use invented reports. These tests do not prove current live bank compatibility. The owner performs live login/MFA/account-coverage checks; agents must not inspect credentials, bank sessions, real exports, screenshots or profile contents. Diagnostics contain fixed codes and numeric counts only.

`owner/run.mjs` remains an optional manual metadata-discovery tool; see [owner instructions](owner/README.md). `service/` is a disconnected mock experiment, not the deployed connector. Its original separate-host proposal and development findings are preserved in the [archive](../../docs/archive/README.md).
