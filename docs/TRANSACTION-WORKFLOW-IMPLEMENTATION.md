# Transaction workflow implementation plan

Working baseline is d7f5f1b in /home/douw/finance-tracker. Implement the six changes below in order. Each section is independently verifiable. Recheck the named paths and existing tests against the current checkout before editing.

## Setup and constraints

- Work in the Ubuntu source checkout through WSL. Read AGENTS.md, docs/HANDOVER.md, docs/PLAN.md, docs/ARCHITECTURE.md, docs/UI.md and the repository UI skill first. Preserve unrelated work.
- Use synthetic accounts and disposable databases. Retain server-side authorization, exact-cent arithmetic, per-version seen state, automatic acceptance, audit records and explicit historical application. Use the existing Field, Form, modal, toast and request-lock patterns.
- The current request produces this plan only. Leave this document untracked and uncommitted, including during later implementation.

## 1. Protect drafts when linking or unlinking a transfer

**Entry points.** web/src/features/transactions/TransactionEditor.tsx owns the draft baseline and leave guard. TransactionExtras.tsx performs the transfer request and closes the editor directly. internal/app/transfers.go owns the financial mutation.

1. Route both mutation buttons through an editor-owned continuation. A clean draft follows the existing path. For a dirty draft, offer Save and continue, Discard changes, and Stay here before sending the transfer request. Opening a counterpart for inspection should continue to preserve the mounted parent.
2. Save and continue must validate fields, await the save and use its confirmed transaction version for linking. Keep the counterpart ID and version the user reviewed. If save also creates a rule, resume the continuation exactly once after the optional historical application dialog. Discard uses the saved row. Stay here retains the draft and candidate.
3. Keep pending actions locked across the handoff. If save fails, retain the draft and stop. If save succeeds but linking fails (for example, the amount changed), show the transfer error and keep the editor on the saved version so retry is possible. Unlink currently identifies the pair using the row ID. Ensure a changed counterpart cannot cause the browser to unlink a different pair than the one shown.

**Acceptance.** Exercise clean and dirty link/unlink, all three guard choices, invalid input, a changed counterpart and repeated clicks. Assert the stored note/category, both link endpoints, and the number of writes. Cover save success followed by transfer failure separately.

## 2. Make merchant defaults predictable during transaction editing

**Entry points.** Start with the onMerchant callback and allocation/group handlers in TransactionEditor.tsx. Inspect Allocations.tsx and the transaction merchant picker for explicit edits and clears.

1. For a single allocation, track category and group independently as untouched, merchant-supplied or user-chosen for this draft. Treat populated saved values as protected. An explicit user choice, including clearing a field, takes ownership until this editor session ends.
2. On merchant selection, fill each eligible field independently, including group-only defaults. When switching or clearing the merchant, replace or clear only the values supplied by the prior selection. Reset temporary ownership on transaction navigation. In split mode, preserve allocations and group overrides. Route any group change through the applicable transfer/link safeguards.
3. Keep these markers local to the draft. Persist the final assignments through the normal transaction save. Give a short indication of which values came from the merchant when it changes a visible field.

**Acceptance.** Use two merchants with different defaults. Test A → B, A → clear, category-only and group-only defaults, a saved category with a blank group, manual override and manual clear before switching, and selecting a merchant on a split. Reopen the saved record and verify assignments and acceptance.

## 3. Separate the end of a review pass from an empty queue

**Entry points.** web/src/TransactionAccess.tsx handles navigation and the end dialog. internal/app/workflows.go excludes processed IDs. Transactions.tsx supplies the original ledger filter query.

1. Keep processed IDs for navigation within a pass. When Save and next reaches the end of a Needs category pass, fetch the first page and total using the authorized ledger listing and the original filters. Remove navigation-only parameters and reset pagination. Count rows still matching, including previously visited rows.
2. For a positive total, show “End of this pass,” the remaining count, and Review remaining. That action starts a fresh pass in the same scope. For zero, use queue-complete copy only for the unrestricted Needs category queue. Use scope-specific empty copy for a filtered queue. Preserve ordinary ledger browsing.
3. If the count fails, offer Retry without resubmitting the save. Ignore late responses after close or a scope change. If access or matches change before restart, re-query the same authorized scope.

**Acceptance.** Save the final row once without a category and once with a category. Verify the count, restart target and completion copy. Repeat with an account filter and a pending row in another account. Simulate a failed count and verify retry performs no financial write.

## 4. Create a merchant and its first rule atomically

**Entry points.** web/src/features/rules/UnifiedRuleEditor.tsx currently posts to /labels before /unified-rules. internal/app/unified_rules.go contains writeUnifiedRulesTx and testUnifiedRule. The example tester already evaluates the write path inside a rolled-back savepoint.

1. Add an optional draft merchant name to the browser unified-rule request for new merchant rules. Require exactly one existing merchant ID or new name for this case. Preserve existing-ID callers and current name normalization, duplicate-name behavior, account/global scope, grants and logo validation.
2. Resolve/create the merchant with the existing authorized label-write logic inside the same SQL transaction as rule creation. If necessary, extract the transaction-taking part of the label service. Grouped account rules must refer to one merchant of the intended scope. Roll back all merchant, rule, logo and audit writes if any step fails.
3. Send one request from the form. Enable Check rule for a valid draft name and evaluate it through the same write path inside the existing savepoint. Keep the name and rule fields on errors. On successful save, use the returned rule refs for the existing optional backfill.

**Acceptance.** In internal/app/unified_rules_test.go, verify a successful grouped save, duplicate-name behavior, rejected scope, failure after merchant creation, and preview followed by cancellation. Assert database and audit counts before/after preview and failure. Browser tests should verify the example shows the draft name, then saving creates one merchant and offers backfill once.

## 5. Add a date-range scope for historical rule application

**Entry points.** web/src/features/rules/RuleBackfill.tsx and internal/app/rule_backfill.go. The existing saved-period scope selects household transactions. The service caps scans at 100,000 transactions and matches at 10,000.

1. Add Date range alongside All previous transactions and the existing budget-period option. Offer it for private accounts and non-budget editors. Require both inclusive transaction dates. Validate real calendar dates and start ≤ end in both form and service. Keep existing all/period payloads valid and reject mixed scopes.
2. Filter dates in SQL before the scan limit. Intersect the range with rule scope and current editable accounts. Include the bounds in the signed preview input and use the identical query on apply. Changing either bound invalidates the displayed preview. Update cap errors to offer a narrower date range.
3. Keep missing-only assignments, exclusions, conflict handling, per-version seen invalidation, replay protection and stale/access checks. Reuse the existing before/after preview and explicit Apply action.

**Acceptance.** Cover both boundaries, same-day ranges, invalid/reversed dates, private and household matches, inaccessible accounts, oversized ranges, and a narrow range on a large history. Test a changed matching row or revoked grant between preview and apply. Verify all/period callers and signed preview replay protections still work.

## 6. Give import previews one status

**Entry point.** web/src/features/imports/ImportPreview.tsx derives the summary badge but renders a fixed “Needs attention” badge inside the expanded panel.

1. Remove the duplicate inner badge. Keep the summary status visible when expanded. Preserve rejected-row acknowledgement, duplicate decisions, and commit gating.
2. Exercise an ambiguous duplicate, resolve it and verify the expanded panel switches to “Ready to add.” Check already-imported and empty previews, plus a stale preview requiring another check.

**Acceptance.** Only one account status is presented in either disclosure state. Its readiness agrees with the existing commit eligibility once requests settle.

## Verification and handoff

From the Ubuntu repository root, activate the project tools and run the affected suites. Select additional existing specs covering the changed workflows. Add focused regression cases where coverage is missing.

~~~sh
export PATH="/home/douw/.nvm/versions/node/v22.21.1/bin:$PWD/work/toolchain/go/bin:$PATH"
go test ./internal/app ./internal/classification
go vet ./...
(cd web && npm test && npm run build)
python3 scripts/test-browser.py transaction-editor.spec.ts core-improvements.spec.ts review-defaults.spec.ts
python3 scripts/test-browser.py rules.spec.ts pending-rules.spec.ts merchant-global.spec.ts
~~~

Use the disposable browser runner for the new import-status and backfill scenarios as well. Record actual checks and outstanding limits in docs/VERIFICATION.md and docs/HANDOVER.md when implementing. Update docs/PLAN.md and docs/UI.md for accepted behavior. Check MCP schemas, permission boundaries and shared write services for each backend change. Keep draft-merchant entry and date-range backfill browser-scoped unless separately authorized, and record the MCP impact. Check git status at handoff and leave this planning document outside the index.
