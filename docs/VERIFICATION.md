# Verification

## Issues #4 and #5 — 7 October 2026

Scope: plain product copy and current documentation. UI edits change text only; no financial behavior, runtime configuration, schema, API fields or permission logic changed. Core guides and FNB notes now separate current contracts from archived development history.

Completed with synthetic fixtures:

- All 8 frontend unit tests and the production TypeScript/Vite build passed.
- 16 distinct Playwright workflows passed across focused runs: transaction navigation/drafts/metadata/filters, reports/rebalance/account health, bulk edits/category archive/merchant preview, daily guide/export, FNB connection controls, Income/merchant permissions, and MCP consent/proposal/manual/automatic approval/revocation. Coverage includes 1440px/360px, with MCP light/dark variants.
- All 118 local Markdown links resolve. All nine archived originals retain their complete text, apart from historical notices and relocated links.
- Syntax-tree comparison confirms text/comment-only source edits in six frontend files. Whitespace checks passed.

The first browser run passed 9 workflows; 7 stopped at outdated Permission/Account labels. After those selectors were corrected, the bulk workflow passed and MCP reached a hidden automatic-approval checkbox. That rerun was stopped after the first timeout (one further test interrupted). The test now opens the existing disclosure; all 6 MCP workflows passed on a fresh synthetic service. Tests were aligned with current controls without changing application behavior. Temporary configurations were removed.

The full backend and historical browser suites were not rerun for text/docs-only changes. Browser tests built the unchanged backend and exercised the affected integration workflows.

MCP impact: connection/privacy/approval copy is clearer, but tools, schemas, output fields, grants, proposal evidence/audits, consent and shared write services remain unchanged. No new MCP capability or browser financial operation is introduced.

## Previous evidence

The [archived verification record](archive/2026-10-07/VERIFICATION.md) retains the detailed audit/refactor/demo results, including intermediate failures and reruns. The refactor covered 168 distinct backend tests with race detection across two runs and 55 distinct synthetic browser workflows across main/focused runs. The combined branding/demo branch passed 8 frontend tests, a production build, demo invariants, 3 focused MCP/OAuth tests, Go vet and 2 handover workflows. These are previous results, not a full-suite rerun for this change.

## External limits

- Live FNB login/MFA/layout compatibility and complete history require owner-run checks; recent pages may be incomplete.
- OFX identifier stability across separate overlapping downloads needs another external sample.
- External MCP clients require their own connection/consent checks.
- Docker is unavailable in the current WSL distro; image build/publishing and production deployment have not been verified here.
- Current owner policy excludes computer-use UI inspection. Automated workflow/DOM/geometry checks do not provide manual visual sign-off.

No production data, credentials or live banking session was inspected. No production deployment is part of this change.


## Issues #15–#17 user security — 2026-10-07

Branch/worktree: `codex/user-security`, `/home/douw/sente-user-security`, based on main. Schema 20 is an additive nullable deletion marker. Synthetic migration checks preserve users/sessions; deletion preserves ledger/source/rule/audit/review foreign keys and reserves historical usernames while removing active access and credentials. Current-session retention for self-service is explicit; administrator resets revoke every target session. Every password path revokes MCP connections and user-bound pending OAuth consent, with no MCP tool/schema/allowlist/capability expansion. Browser-only account administration uses shared validation/dialog/toast controls.

Passed: 18 distinct focused backend tests with race detection across the main security/OAuth/authentication/setup run (16 tests) and migration/disabled-reset follow-up (2 tests); all four financial domain packages; `go vet ./...`; final frontend production build; 8 frontend unit tests; 7 synthetic Playwright workflows (4 new security workflows at 1440/360px plus 3 settings regressions); `git diff --check`. Security coverage includes current-password checks, byte bounds, administrator permissions, CSRF, stale versions/concurrent resets, session/agent/OAuth revocation, retained disabled status, last-administrator/self-deletion protection, history/foreign-key preservation and atomic rollback when audit fails.

The initial migration fixture incorrectly removed its schema marker and was corrected to represent version 19 before passing. The initial standard browser runner stalled at localhost readiness despite a healthy synthetic server; it was stopped. The successful run used a temporary one-server config and local proxy bypass on port 18580; the temporary config and test servers were removed/stopped. Tests used disposable synthetic databases only. Full historical backend/browser suites were not repeated. No manual/computer-use visual inspection, real financial data, live banking or production deployment.


Password visibility/user-menu follow-up (2026-10-07): final frontend build and 8 unit tests passed. Nine synthetic browser workflows passed across the main/follow-up runs: password toggles and menu editing at 1440/360px, four existing security workflows, and three onboarding/budget workflows. Coverage checks all ten password inputs across setup, sign-in, self-service, add/reset users and FNB credentials; value preservation, re-masking on clear, keyboard toggling, labelled input linkage, 44px targets, transparent hover background/border, validation and menu focus restoration. Initial onboarding failures came from a partial selector matching the new eye button; exact field selectors and keyboard expectations were corrected, and all three onboarding workflows then passed on a fresh disposable server. Temporary runner configs/test servers were removed/stopped; the worktree dev server on 5175 remains running. No manual visual inspection, banking interaction or backend/MCP contract changes.

Integration before merge (2026-10-07): incorporated main’s PR #43 documentation/copy cleanup, retained its active/archive organization and this branch’s security notes, and repeated the final production frontend build, 8 unit tests and six desktop/mobile password/menu/security browser workflows successfully. Backend implementation is unchanged by that integration. Temporary integration server/config removed; dev on port 5175 remains available.
