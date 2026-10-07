# Project instructions

Implementation is authorized. Use the accepted MVP behavior in docs/PLAN.md. Do not treat superseded planning proposals as current requirements.

For every application change, check whether MCP tools, input schemas, output allowlists, permissions, proposal previews/audits, shared services, tests and docs/MCP.md also need updating. New fields must not become accidental disclosures or silently expand existing connection consent. Record either the corresponding MCP update or the reason a feature remains browser-only in the handover/verification notes.

Read docs/PLAN.md and docs/ARCHITECTURE.md before implementation. For UI changes, read docs/UI.md and apply .agents/skills/finance-tracker-ui/SKILL.md. These are the shared source for consistent UI development.

Forms must use the shared Field and Form components for validation. Keep untouched fields quiet; validate on first blur, then revalidate on every change. Show errors directly beneath the relevant input, including identifiable server field errors, with aria-invalid and aria-describedby. Submission reveals all invalid fields and focuses the first one. Revalidate dependent fields when their source changes. Use shared overlay toasts for request-wide failures such as authentication, connectivity, or concurrent edits; never use submit-only validation or browser error popups as the primary feedback. Apply the same behavior to dialogs and button-driven editors.

Current owner instruction: do not use computer use to inspect the UI for now. Use focused builds, tests and structural checks. Resume targeted visual inspection only when the owner requests it.

Use focused builds, tests and structural checks for routine UI changes. Perform short, targeted computer-use visual checks when explicitly requested or when layout/interaction risk warrants them. Use synthetic data, preserve user data/preferences and record coverage and unverified states. Never claim visual verification that was not performed.

Cards should align horizontally and vertically wherever the layout allows: stacked cards share the same outer width and left/right edges; cards in a grid share row top/bottom edges, consistent gaps, and column boundaries. Use shared grid tracks and stretching rather than independent maximum widths or arbitrary fixed heights. Keep mobile stacks at natural content height, and verify both empty and populated states with checks permitted by the current inspection policy.

Maintain docs/UI.md when an accepted component or design convention changes. Once implemented, shared tokens and components enforce the guide in code; avoid copying styles independently into screens. Explicit user instructions take precedence; record their accepted changes in the guide.

Keep account authorization on the server for every read, write, import, export, and aggregate. Categorized transactions are accepted automatically; missing categories require review. Seen/unseen is personal and independent of acceptance. Store monetary values as integer minor units with currency, never floating point. Split allocations must equal their parent transaction, and totals must never count both parent and allocations.

Do not commit real financial data, uploaded CSVs, credentials, database files, or backups. Use synthetic fixtures. Prefer tests of financial invariants, permission boundaries, and import idempotency over tests that merely reproduce implementation.

For local iteration, use make dev and http://127.0.0.1:5173: Vite hot reload and the watched Go backend avoid Docker rebuilds. Development has its own persistent database; do not copy production financial data into it implicitly. Use Docker builds for production verification/deployment, not routine local UI edits.

Keep scope aligned with the MVP. Record open product questions instead of silently choosing financial behavior.

Use shared overlay toasts for transient success/error feedback. Never insert notification cards/banners into the page or modal flow that shift layout. Toasts must appear above dialogs, announce status/errors accessibly, support dismissal and pause expiry during hover/focus. Keep field-validation errors inline below their fields. Persistent connection status and review/preview content remain part of their settings/workflows. Give buttons consistent space from helper text above/below; use shared spacing conventions.


2026-10-04 owner acceptance/seen policy: supersedes mandatory approval and save-only pending behavior. Complete categorized allocations automatically accept ledger entries on import, edit and transaction-created rule application; explicit transfers retain their category exemption. Missing categories (including incomplete splits and rule conflicts) stay in Needs review. Saving an edit marks its current version seen for the editing user. Imports/rule-applied targets begin unseen. Seen/unseen is personal per user and transaction version, independent of acceptance/reporting, with authorized atomic 1–100 selection actions and ledger seen filters. Later transaction changes invalidate old seen markers; opening a dialog alone does not mark seen. Existing explicit approved reviewers retain seen markers on migration; existing categorized pending rows are accepted without claiming human review, and acceptance changes are audited. Migration 10 is once-only and preserves money, allocations, groups, private account access and financial source data. Private accounts remain outside shared budgeting. UI uses Accepted/Needs category and Mark seen/unseen rather than manual approval; Save changes is the single editor save action.


## UI separators

Do not add separators directly beneath content blocks, cards or dropdown/disclosure cards. Use spacing and shared surface/typography hierarchy to distinguish adjacent content. In particular, spending-group dropdown cards must not have a trailing separator or border-bottom underneath them. Apply this rule when creating, editing or reviewing UI layouts.


## Maintainable module boundaries

Follow the module map in docs/REFACTORING.md for future changes. Keep files focused on one domain or workflow; extend the owning module rather than adding unrelated code to an entry point or a generic management file. When a change makes a file difficult to navigate, extract a cohesive component, hook, adapter or service in the same change. Treat a few hundred substantive lines or multiple independent workflows as a review signal, not a hard size limit; do not hide complexity in compressed one-line JSX or split files arbitrarily.

Backend: internal/app composes routes, authorization and shared transactional services. Put reusable money, classification, statement-adapter and allocation invariants in their existing domain packages. Domain packages must not import internal/app or depend on HTTP/MCP transport. Browser handlers, MCP proposals and offline commands must call the same authorized write services; preserve the single serialized SQL transaction for access/version checks, mutations and audit. Introduce a new package only for a real dependency boundary, not merely to meet a line count. Preserve API/error/JSON contracts, exact-money arithmetic, source provenance and migrations when moving code. Keep the application facade aliases thin; do not duplicate implementations or bypass consent.

Frontend: route screens and workflow components belong under web/src/features/<feature>. App.tsx owns session/workspace composition and shared navigation state; reusable contracts and request-task hooks belong in web/src/shared, never in App.tsx. Feature components must not import the App entry point. Keep transient state with its owner and lift it only where workflows share it; preserve mounted drafts, request locks, async cleanup, validation, focus restoration and selected scopes during extraction. Reuse existing UI primitives, API client and theme tokens. Avoid dependency cycles, broad barrel exports, generic dumping-ground files, duplicated logic and new state frameworks without a concrete need.

Keep public types and imports explicit, remove unused dependencies, and format new code so control flow and JSX remain readable. Separate structural refactors from product/financial behavior changes. Keep meaningful invariant tests with their domain and permission/atomicity tests at the integration boundary. Run affected domain tests, backend tests/vet, frontend tests/build and synthetic workflow checks as appropriate; record actual coverage and MCP impact in docs/VERIFICATION.md and docs/HANDOVER.md. Update docs/ARCHITECTURE.md and the module map when boundaries change.
