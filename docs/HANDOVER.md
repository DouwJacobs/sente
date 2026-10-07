# Sente handover

Current source handover, updated 7 October 2026. Work directly in Ubuntu WSL at /home/douw/finance-tracker; the Windows project links to that source. Preserve existing uncommitted work. No production deployment was performed during the latest audit repair or module refactor.

## Working instructions

Read AGENTS.md before work, then [PLAN.md](PLAN.md) and [ARCHITECTURE.md](ARCHITECTURE.md). UI work also uses [UI.md](UI.md) and .agents/skills/finance-tracker-ui/SKILL.md. Every application change needs an MCP impact check: shared services, schemas, permissions, exact proposals, tests and documentation.

Run application commands through WSL from the source directory. Use make dev for Vite HMR and watched Go builds at http://127.0.0.1:5173, with a separate persistent development database at data/dev/finance.sqlite. Go is work/toolchain/go/bin/go; Linux Node is /home/douw/.nvm/versions/node/v22.21.1/bin. Do not copy the production database into development or point tests at owner data. The synthetic browser server is scripts/e2e-server.py.

Use focused builds/tests and structural checks. Follow the current owner inspection preference in AGENTS.md and the UI skill; record actual coverage and avoid claiming unperformed visual checks. Latest results and remaining limits are in [VERIFICATION.md](VERIFICATION.md).

## Accepted financial and classification behavior

Categories remain flat. A parent transaction's spending group is independent of allocation categories; a category can be used in multiple groups. Keep legacy category IDs/metadata without merging records or rewriting history.

Money uses signed integer minor units and exact split totals. Transfers use an explicit ledger flag and are excluded from spending/income. The transaction editor sets that flag when the owner selects the group named Transfer, and clears it when another group is selected; opening a record or naming a group in an import/rule/API does not itself rewrite transfer status. Linked transfers require unlinking before changing designation.

Complete categorized allocations accept transactions automatically; missing categories and conflicts stay in Needs review. Personal seen/unseen is independent, versioned and changes only through explicit actions or saved edits. Opening a transaction never marks it seen. Imports/rule-applied targets remain unseen. Private accounts stay outside shared budgeting, and authorization applies to every read, aggregate and mutation.

Budgets use independent period/group/category limits. Group totals sum their children; the same category can have limits in several groups. Explicit ungrouped scope differs from omitted legacy scope. Scoped replacement never removes other groups; whole-period removal is explicit. Empty groups, explicit zero entries and recurrence persist. Legacy aggregate-only limits are preserved under No spending group before totals rebuild, with audit evidence.

The builder defaults new categories to This budget only; upcoming recurrence fills missing future entries without overwriting saved amounts. Rebalance retains total limits and rechecks versions/spending. Reports/CSV use the same ledger semantics.

## Current workflows and UI

Transactions share one editor with scoped navigation and draft safeguards. Current-record links from accounts, categories, imports, transfers, budgets and MCP proposals recheck authorization. Nested editors preserve parent drafts; stale parent previews must be refreshed after child saves. Source rows without a ledger identity remain source details.

Dashboard emphasizes remaining budget, then full-width spending groups. Category rows open a centered summary/transaction modal. Group/category/account/period scope remains exact across paging and edits. Next steps and bank balances form the supporting desktop row and stack on mobile. Review entry clears stale constraints and starts with Needs category, including both seen/unseen.

Categories and search share SpendingGroupEditor and common Modal/Form/Field/Button controls. Categories have no ownership selector. Search has a labelled combobox/results, input-scoped keyboard navigation, announced loading/errors, retry and modal focus restoration. Shared theme tokens, first-blur/live validation and mobile hit targets remain authoritative.

Implemented core workflows include atomic bulk classification/labels, transaction notes, tags, merchants/global or account-specific naming rules, optional local logos, category archive/restore, import health/run summaries, saved-period navigation, budget sorting/daily guide/rebalance, trends and protected CSV snapshots. See architecture/UI guides for contracts; [FUTURE.md](FUTURE.md) contains unresolved work only.

## FNB connection and accepted limits

The owner accepted same-host encrypted credential storage, superseding the original separate-host requirement. Encryption does not guarantee secrecy from a privileged host agent. Never request or inspect live credentials, bank sessions, raw banking screenshots or financial database contents. The owner enters credentials and verifies live compatibility.

The connector supports owner-only connection state, account discovery/balances, persistent hide/show, manual transaction fetching and optional scheduled importing. Schedules start disabled; MFA/layout failures require owner action rather than repeated authentication. Current runtime, key recovery and deployment requirements remain in README.md, architecture and connectors/fnb/README.md.

Scheduled runs fetch transactions for mapped, visible editable accounts and refresh balances in the same provider session. Initial discovery without mapped targets remains account-only. Clean imports commit through authorized audited services; ambiguous uploaded CSV/OFX candidates and conflicting bank IDs retain exception workflows.

Overlap matching consumes existing occurrences using immutable date/amount/description and available balances/reference details. Page positions are not identities and live references are not promoted to FITID. Identical same-page purchases remain distinct. Capped pages or indistinguishable purchases outside the visible window cannot establish complete history; coverage flags remain mandatory.

The owner accepted exact unique masked credit-card identities when both summary/detail are masked, with verified Credit type and ZAR checks. Do not invent missing digits, merge changed masks or accept duplicate/ambiguous/foreign-currency identities. Preserve home-loan and service-fee compatibility constraints. Live reliability remains owner-verified; synthetic fixtures do not establish it.

## MCP and offline rulesets

[MCP.md](MCP.md) documents the current tool surface and consent. Agent connections act for an identified tracker user under current permissions. Group/category fields remain independent. Budget names resolve to stable IDs at preparation, and exact grouped membership/target/recurrence before/after state is shown and checked at apply. Unresolved older proposals require fresh preparation.

Automatic approval is opt-in per connection/capability; mixed effects need every grant. No silent capability expansion, direct bypass of proposal checks or retroactive approval. Merchant reads/logos require explicit consent; catalogue/rule/assignment mutations use exact proposals. Global merchant writes are blocked for account-restricted connections.

Category administration, selective budget recurrence and rebalance remain browser workflows. Private notes stay outside agent read allowlists. Offline ruleset import/export adds no MCP tool; [RULESETS.md](RULESETS.md) defines the strict version-2 format, explicit operator/database/account mapping, atomic shared writes, versions/audit and export protections.

## Latest repair and verification

All nine 7 October repository/UI findings are resolved in source. The final build, 8 frontend unit tests, 25 synthetic browser workflows, four strict ruleset tests, CLI wrappers, vet and whitespace checks passed.

The full backend run had 164 passes, one expected external-sample skip and one legacy-limit preservation failure. That failure was repaired; all seven affected regressions then passed. At the end of that audit, the entire backend suite had not been repeated after its final correction; the module refactor below subsequently covered the complete backend test list across race-enabled runs. [VERIFICATION.md](VERIFICATION.md) preserves exact coverage and limitations. There was no production deployment, commit, live banking test or manual visual sign-off.


## Issues #1 and #2 module refactor — 7 October 2026

Work is on `codex/domain-feature-refactor`. Backend domain packages now own money, classification, statement adapters and allocation invariants; focused app files keep authorization and atomic SQL services shared by HTTP, MCP and offline commands. Frontend screens/import previews/transaction components live under `web/src/features`, with contracts/request tasks under `web/src/shared`. App no longer exports shared contracts or hooks. [REFACTORING.md](REFACTORING.md) maps ownership; AGENTS.md requires future changes to follow these boundaries and preserve behavior, draft ownership and shared UI controls.

MCP impact: the existing shared services reach the extracted domain logic; routes, input schemas, JSON output allowlists, grants, proposal previews/evidence and consent are unchanged. No migration, live banking, production-data inspection or deployment. Current validation results are recorded in VERIFICATION.md.


Refactor verification: 168 distinct backend tests passed with race detection across the main run and its one remaining-test follow-up; the external FNB sample test skipped as expected. All final packages compiled and vet passed. Frontend build and 8 unit tests passed; 55 distinct synthetic browser workflows passed across main/focused runs, including settings draft retention, session feedback and nested editing. Historical browser expectations were repaired without changing the application UI. The main backend run hit its 30-minute process limit, and the complete 140-test historical browser suite was not rerun; VERIFICATION.md records exact coverage. No production deployment or live banking check.

## Sente README and demo branch — 2026-10-07

Issues #3 and #18 are paired on `codex/sente-readme-demo` in `/home/douw/sente-readme-demo`, based on main. The issues #1/#2 refactor was merged into main as PR #42 before this branch was integrated; branding now follows the extracted auth/workspace components. README is user-focused; runtime/development notes are in dedicated docs. Product names use Sente; existing binary, key path, module and volume identifiers remain compatible. Docker destination is douwjacobs/sente with latest/development tags. Synthetic SQL and exclusive new-database tooling support shared local demo data. MCP impact: public product/resource names change only; offline demo tooling introduces no MCP endpoint, financial schema/allowlist, capability or consent change. Screenshots and live FNB compatibility are not claimed.

Integrated with main after PR #42; current production build, unit/demo/MCP/OAuth checks, vet and both synthetic handover browser workflows passed. See VERIFICATION.md for the unavailable Docker repeat check and publishing credential requirement.


## User security — issues #15, #16 and #17 (2026-10-07)

`codex/user-security` in `/home/douw/sente-user-security` adds administrator password resets and explicit user deletion, and hardens the existing self-service change. Schema 20 adds nullable `users.deleted_at` without rewriting financial history. Deletion is permanent access removal: retain the username/ID as a reserved historical identity for imports, rules, audit and review attribution; clear its password, roles and membership, hide it from user lists, remove grants/personal seen markers/sessions/FNB credentials and discoveries, and revoke MCP connections plus pending OAuth consent. Accounts, source provenance, allocations, classification rules and financial history remain unchanged. Deleted identities cannot be restored through update/reset/grants. Keep an enabled administrator; self-deletion requires another administrator.

Administrator resets target another user with optimistic version checks, leave disabled status unchanged, and revoke all target sessions and MCP credentials. Self-service requires the current password, deliberately retains the initiating session and revokes other sessions/all MCP connections. Both use the existing 12–72-byte rule and audit without password material. Offline recovery shares the credential-revocation service and records its recovery method. Login rechecks the verified password and active user atomically with session creation; security mutations recheck the actor session/administrator role within the write transaction. Audit failures roll back all mutation/revocation effects.

MCP impact: account administration/passwords remain browser-only (offline recovery also remains available). No tool, input schema, output allowlist, capability or consent expansion. Connection deletion uses existing cascades for proposals, OAuth codes, access and refresh tokens, and also removes pending user-bound authorization requests. Financial audit evidence remains. Frontend uses shared forms/fields, first-blur/live errors, dependent password confirmation, native dialogs, explicit typed-username deletion and overlay toasts. No production database, banking credentials, deployment or manual visual inspection.

Verification for #15–#17: 18 focused backend tests passed with race detection, all four domain packages and vet passed, and the final frontend build/8 unit tests/7 desktop-mobile browser workflows passed. Full historical suites and manual visual inspection were not repeated. See VERIFICATION.md for exact coverage and the isolated browser-runner workaround.


User security UI follow-up: shared Field adds password eye controls across all browser password inputs; user rows now have a single hamburger menu with Edit rather than a separate pencil. This changes browser presentation only: MCP tools, schemas, output allowlists, permissions, proposal previews/audits and shared financial write services are unchanged. The dev server remains on port 5175 for this worktree; verification is recorded below/in VERIFICATION.md.

Password/menu verification: final frontend build, 8 unit tests and 9 synthetic desktop/mobile password/menu/security/onboarding workflows passed. Eye hover is transparent/borderless; keyboard focus remains visible. See VERIFICATION.md for exact coverage and corrected test selectors.
