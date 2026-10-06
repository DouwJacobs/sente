# MCP efficiency and permission implementation plan

Requested 2026-10-05. This delivery sequence is implemented and verified; completion and release evidence are recorded below and in VERIFICATION.md. Existing accepted behavior in PLAN.md, MCP.md and the latest HANDOVER.md remains authoritative.

## First delivery status (2026-10-05)

Owner selected repair verification and step 2 as the first delivery. Existing repair retained, with category and transaction-batch upgrade/replay coverage. Step 2 adds typed/legacy-compatible ledger filters, the compact authenticated cursor queue and additive query indexes. Owner decisions: category_ids matches any selected category; queue selector unseen includes only unseen entries needing categories. UI expansion authorized in this chat: only Needs review starts with visible Needs category and Unseen filters; both can be changed/cleared to inspect other unseen entries. All transactions defaults remain broad. See MCP.md, HANDOVER.md and VERIFICATION.md for implementation and actual checks. Steps 3–6 are now implemented in source; final checks and production deployment are tracked below.

## 1. Repair proposal application first

Source already adds a missing mcp_proposals.result column during normal startup migration, including databases already at schema version 12. The existing synthetic upgrade test checks approved category creation and saved-result replay. Do not duplicate that migration or replace existing proposals.

The apply handler currently maps every SELECT/Scan failure to Proposal not found. Restrict that response to sql.ErrNoRows, preserving same-user/same-connection isolation. Return a sanitized database failure for other lookup errors; never expose SQL, financial payloads or credentials. Apply the same distinction to get_change_status.

Verify synthetic legacy-schema recovery, category creation and transaction batches, rollback, retained approvals, replay, expiry, cross-connection isolation and truly absent IDs. Confirm the serving process uses the intended source/build/database and has run startup migration. Inspect schema metadata only when needed; avoid reading financial records. Retry each existing proposal using its original connection only if still approved, unexpired and current. Category-dependent edits follow successful category creation. Expired or stale proposals need fresh proposals and approval; a new connection must not inherit old proposals. Do not claim the reported 13 proposals were recovered without runtime verification.

## 2. Typed, compact transaction reads

Extend shared authorized query services in transactions.go and transaction_filters.go rather than adding a parallel transaction model. Add date_from/date_to (inclusive local calendar dates), category_ids, explicit categorized/uncategorized/any, accepted/needs_category/any and signed min_amount_cents/max_amount_cents. Keep seen state separate. Validate bounds, contradictory inputs and limits. Preserve legacy list_transactions filters/offset/list_version clients.

Add a typed get_transaction_review_queue with needs_category or unseen selectors, bounded limit and opaque cursor. Cursor uses stable date/id ordering, binds filters and user, and detects stale lists. If a compatibility under_review alias is offered, document that it means needs_category. Return id, version, date, redacted description, amount_cents, currency, generic account label/id, minimal allocation/category details, acceptance and personal seen state. Transfers are category-exempt; missing allocation categories define uncategorized. Do not expose raw source, notes, account nicknames or audit data.

Use SQL predicates before paging/counting. Evaluate representative synthetic query plans before adding additive account/date/id, review or allocation/category indexes. Normalized substring matching may require scans; do not promise ordinary indexes solve it.

## 3. Category search and batch decision context

Reuse list_categories(q, page, page_size); optionally add a compact typed search_categories wrapper. Categories are flat, with separate spending groups; no parent hierarchy is introduced.

Add get_categorization_context(transaction_ids) with 1–100 distinct authorized IDs. Use existing description normalization and rule engine, not an invented merchant identity. Return bounded matching rule summaries, conflicts and historical allocation category counts. Define similarity explicitly and label it as description-based evidence. Count distinct parent transactions per category; splits must not double count. Apply current account authorization to every history aggregate. No private-account leakage, invented confidence or historical transaction dumps.

## 4. Rule impact previews

Add preview_categorization_rule over existing contains-description, direction, account scope, priority, enabled and category/group semantics. Return authorized match counts, uncategorized/eligible counts, bounded redacted samples and conflicts using the actual rule precedence engine. Distinguish future-import effects from existing-ledger eligibility. Reuse matching services; push deterministic matching/counts into SQL where equivalent, and bound any rule-engine work. A standalone saved rule still does not sweep existing transactions automatically.

## 5. Narrow writes and granular connection permissions

Add category-only single/batch proposal operations that accept transaction id/version and category id. For splits, require an explicit allocation target and preserve existing allocations/categories/amounts; initial unsplit-only support is acceptable if documented and rejected clearly. Restricted categorisation must only fill missing categories and reject transfers or recategorisation. Check constraints atomically both when preparing and applying, alongside versions/current account editor access.

Extend existing OAuth connection metadata with versioned capabilities and constraints. Enforce permissions for every entry point, including legacy prepare_change and approved proposal replays, so generic edits cannot bypass restrictions. Financial fields, recategorisation, category creation, rule create/update/delete and seen changes are separate capabilities. Only expose category operations actually supported by the app; do not introduce category deletion/administration just to populate a permission matrix.

Presets: Review only; Categorisation proposals (reads, missing-category assignments and constrained contains-rule creation); Finance editing proposals (existing legitimate edit/category/rule/budget operations, within current user/account permissions). All writes retain exact browser proposal approval. Constrained rules use existing fields/operator, explicit authorized accounts and bounded nonempty patterns. Any per-connection quotas must be persistent, atomic, have defined reset periods and count successful effects, not retries. Do not confuse stateless MCP transport with an agent run.

Migrate legacy connections to their existing read/proposal behavior without silently broadening or narrowing grants. Scope expansion requires browser consent. Reuse OAuth expiry/revocation; no new authentication system. Direct autonomous writes and a default autonomous preset remain a separate owner decision.

## 6. Settings, annotations and audit

Extend MCPSettings and OAuthConsent with read/proposal capability summaries, presets and appropriate custom controls using shared Field/Form/Button/toast primitives and the finance UI skill. Clearly distinguish category assignment, recategorisation, rule creation/deletion and financial edits. Preserve drafts, accessibility and current approval previews; do not redesign unrelated screens. Update UI.md for accepted conventions.

Check installed SDK annotations for reads, proposal preparation and application, accurately reflecting replay/idempotency and destructive behavior. Annotations never authorize writes.

Reuse audit and proposal storage. Verify durable before/after evidence for each affected entity, including deleted rules, with user/connection/proposal/time attribution committed atomically. Do not place audit details in agent read responses or logs.

## 7. Validation and release

Use synthetic fixtures for SQL filtering before pagination, splits/transfers, combined filters, cursor boundaries/staleness, schemas and privacy. Test aggregate account isolation, rule precedence/conflicts, batch rollback, uncategorized preconditions, all permission denials including legacy tools, OAuth expansion/expiry/revocation, quotas if included, audit attribution and replay after migration.

Run relevant Go tests/race checks, go vet, frontend unit tests and TypeScript/production build. Add focused synthetic browser checks for changed settings/consent forms; no computer-use inspection under current owner policy. Run Docker production verification when preparing deployment. Record actual coverage/limitations in VERIFICATION.md.

Publish tool parameter examples and compatibility/migration instructions in MCP.md, update architecture and handover, and list changed files/additive schema changes. Deploy/restart only the intended runtime, preserve databases/credentials and verify health. Never retry financial writes without the existing valid approval and original connection.


## Full-plan implementation status (2026-10-05)

Owner explicitly requested a goal to implement the whole plan. Decisions: same normalized description for history; no usage quotas; selected accounts limit both reads and proposals; Finance excludes independent seen changes (custom only); deploy the verified image to the existing production runtime.

Steps 3–6 implemented: reused category search; bounded batched context and actual-engine previews; narrow split-preserving assignments; schema-13 versioned capabilities/constraints preserving legacy grants; browser consent/presets/custom controls and Settings permission editing; custom exact personal-seen proposals; accurate annotations and atomic entity before/after audit including deleted rules. Every financial/seen effect retains separate exact proposal approval. No autonomous writes were introduced.

Step 7 complete: full 118-test app race binary and go vet passed, frontend build/8 unit tests passed, 6 focused MCP browser checks passed at desktop/mobile in both themes, production Docker image built/smoke-tested, old runtime backed up and the verified image deployed to the existing production service. Production is healthy at schema 13 with unchanged configuration/volumes. Full release identity/backup/schema/health evidence and limitations are recorded in VERIFICATION.md. Original reported proposals have not been retried and external-client compatibility remains owner-operated.

2026-10-05 follow-up beyond the original plan: owner authorized selected/all manual proposal approval and per-capability automatic approval within the existing proposal workflow. Implemented, synthetically verified and deployed; defaults remain manual and direct writes remain absent. See MCP.md/VERIFICATION.md for the superseding consent/approval contract and actual evidence.
