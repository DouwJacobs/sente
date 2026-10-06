# Core finance improvements

Owner-authorized implementation, 2026-10-05. Vault22 provided the reference for everyday finance flows. The owner's focus is the core application; crypto, investments and new connectors remain outside this delivery. Existing project work and financial policies are preserved. Production deployment is separate.

## 1. Transaction review

- Reuse the shared editor with Previous/Next and Save and next in the current authorized, filtered ledger order (date descending, ID descending). Navigation spans list pages. Save uses the original anchor, excludes processed entries and reports completion when the remaining scope is empty. Standalone references retain their normal editor.
- Protect drafts with Save and continue, Discard changes and Stay here. Opening alone never marks a transaction seen. Manual saved edits mark their new version seen by the editing user; automatic/import changes stay unseen.
- Atomic signed previews for 1–100 selected records: fill missing categories while preserving splits, replace category on unsplit entries, set/clear group, merchant or tags. Transfers are excluded from category actions; linked-transfer restrictions match the editor. Recheck versions, permissions and preview semantics before applying.
- Show date and signed amount filters in the shared aligned panel, plus authorized merchant/tag filters. Remove the redundant Accepted badge from ledger rows.

## 2. Import health

- Separate bank-reported balance date, transaction fetch/check time and successful committed import/check time. Zero-new successful connector checks update freshness.
- Show never imported, ready, refreshing, overdue and needs attention, next due, separate incomplete-coverage information and import exceptions. Reading a screen does not trigger a refresh.
- Import activity summarizes the complete authorized run: new, already present, needing categories and exceptions. Banking and upload/import activity provide recovery paths.

## 3. Budget use

- Shared Previous/Current/Next period navigation preserves account scope and uses saved period order by start date then ID.
- Sort whole dashboard groups/categories before pagination: alphabetical, spending descending or positive-target remainder ascending, with no targets last.
- Active-period whole-budget daily guide divides positive remaining budget by Johannesburg days left, including today, rounding down to cents. Hide for future/ended periods, no targets or account-only actuals; negative remainder is Over budget. Never describe this as available bank money.
- Rebalance between two existing expense budget entries in one saved period. Source availability is bounded by both its limit and positive unspent remainder. Signed preview rechecks period version and spending. Total limits, carry-forward and future periods are preserved.

## 4. Reports and export

- Budgets Trends compares the selected/previous period and 6/12 saved periods, with group/category filters. A budget year includes complete periods starting in that year; there is no calendar-year apportionment.
- Use the same allocation/refund/uncategorized expense and explicit transfer semantics as Dashboard. Private/account scope shows actuals without household targets. Averages are informational.
- Link to authorized transactions unassigned/outside budgets in the report dates.
- Export the complete filtered authorized ledger snapshot as transactions.csv plus allocations.csv in one ZIP, with parent IDs joining the files. Money is represented in integer cents. Include current notes, merchant and tags; omit bank identifiers, credentials and raw provenance. Escape spreadsheet formula text. Budget reports export group/category budget, spent and remaining cents for periods/year.

## 5. Organisation

- Transaction note up to 2000 Unicode characters, distinct from allocation notes.
- Category rename/archive/restore retains stable IDs, type and history, with member permission and optimistic version. Archive is blocked by enabled rules and included carry-forward budget entries; pause/replace rules and stop carry-forward first. Restoring never enables rules. Archived categories cannot receive new assignments/rules/budgets.
- Account-scoped merchants (2–100 characters) and tags (1–40), case-insensitive unique names and up to 20 distinct tags per transaction. These metadata changes never change money, category, group or transfer designation.
- Separate account merchant rules use normalized contains, direction and priority. Highest priority wins; conflicting highest-priority names leave the merchant unset. Future imports apply automatically. Existing unnamed entries require explicit selected preview/apply; existing names are preserved.
- No MCP permission expansion or new metadata read/write surface. Legacy edits omitting metadata preserve it.

Implementation is in the WSL source. See HANDOVER.md, ARCHITECTURE.md, UI.md and VERIFICATION.md for current contracts and actual verification. No production financial data or credentials are used in automated checks.
