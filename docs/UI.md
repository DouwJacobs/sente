# UI guide

Use the existing Sente design and shared controls. This is the current guide; earlier design trials and screen decisions are in the [archive](archive/README.md). Product behavior is defined in [PLAN.md](PLAN.md).

## Shared design

Sente is a practical finance tool: readable transactions, clear figures and predictable controls. Use neutral surfaces, restrained teal accents and semantic financial colors. Avoid decorative gradients, glowing borders, glass panels, oversized marketing headings, ornamental emoji and generic motivational copy.

Use `web/src/ui.tsx` for shared Button/Field/Form/Modal/Loading controls, `styles.css` for structure and `finance-theme.css` for tokens. Extend an existing role before introducing a style. Light/dark themes follow the device unless explicitly overridden through the workspace toggle or Settings. Use system fonts, tabular money, 8px panels and 6px controls. Unselected clickable rows use `--interaction-hover` with `--text`; selected states use accent tokens.

Cards in a row share grid tracks, stretched edges and gaps. Stacked cards align left/right edges. Mobile content keeps its natural height; avoid arbitrary fixed heights. Use spacing and typography between sections. Do not add separators beneath cards or dropdown/disclosure cards, especially spending groups; internal transaction-row separators remain appropriate.

Product branding is Sente on onboarding, sign-in, sidebar, OAuth consent and browser metadata. Household names are separately configurable and private to authenticated views; signed-in titles use `<household> · Sente`. Long names wrap or truncate with their full text available.

## Copy and financial status

Use short action labels: Get transactions, Save changes, Edit budgets, Mark seen/unseen. Use Accepted and Needs category for transaction acceptance; Needs review is the category queue. Reserve approve/reject for agent proposals and connection consent. Uncategorized describes a missing category. Use category, spending group and budget period consistently; categories never belong to a group.

Explain what happened and the next useful action. Keep implementation terms such as ledger, provenance, allowlist, optimistic versions and atomic writes in technical docs rather than ordinary helper copy. Permission text must still identify what can be read/changed and when approval is required. Technical identifiers in diagnostics and exact agent previews remain available where they help inspection.

Empty states explain the next action using separate description/action elements. Errors identify the relevant field/file/row and recovery step. Do not repeat developer progress notes, future milestones or marketing claims in the product. Preserve financial warnings about incomplete import history, duplicates, transfers and private information.

Format money in the configured currency/locale with clear debit/credit signs and aligned amounts. Never rely on color alone for income, expense, duplicates, review or over-budget status. Budget guide amounts describe remaining budget, not account balances. Show allocation spending separately from parent totals. Split amounts must sum exactly; display original, allocated and remaining amounts when split controls are needed.

## Forms, notifications and accessibility

Use shared Field/Form for forms, dialogs and button-driven editors. Untouched fields stay quiet, including autofocus. First blur reveals errors; subsequent changes revalidate immediately. Revalidate dependent fields when their source changes. Submission reveals invalid fields and focuses the first; reveal a collapsed invalid section before focusing it.

Place identifiable server errors directly beneath their input, preserving values. Use `aria-invalid` and `aria-describedby`. Request-wide authentication/connectivity/permission/stale-write errors use overlay Toast. Avoid native validation popups and submit-only validation. Sign-in errors remain until dismissal/retry; other toasts can expire with hover/focus pause. Toasts appear above dialogs and do not shift layout.

Labels and controls align at the top; helper/error text must not shift adjacent inputs. Action buttons align with controls and have consistent spacing from helper text. Shared controls target 44px touch areas and visible keyboard focus. Provide labeled fields, semantic headings, accessible status announcements and text equivalents for charts. Do not depend on hover.

Shared Tabs support Left/Right/Home/End and labeled panels. Shared ActionMenu uses a Menu icon, left-aligned actions, outside-pointer/focus dismissal and Escape focus return. Native disclosure controls preserve optional drafts. Single-page lists hide navigation; empty lists omit pagination. Search and paging preserve selected values beyond the loaded page.

## Layout and dialogs

Desktop uses side navigation and compact lists/tables. Mobile uses Dashboard, Transactions, Review and More navigation; less essential columns move into details. All workflows must work at 360px without page-wide horizontal scrolling. CSV comparisons may scroll inside a bounded region. Intermediate widths use two-column summaries and stacked scope controls to keep money readable.

Modal uses shared compact/medium/wide sizes (480/640/900px), fixed headers and scrollable bodies with stable gutters. Transaction editors use content-sized desktop layouts; lookup dialogs retain stable viewport-bounded dimensions. Mobile overlays use full-width/full-height behavior. Headings wrap with the close control visible. Lock background scrolling and preserve focus/scroll on close. Explicit autofocus runs after `showModal`. Nested dialogs restore a connected opener in the parent; dismissed async requests cannot reopen an editor.

## Transactions and review

Transactions contains All transactions, Needs review and Import activity. Account/period/search controls stay visible; Filters reveals remaining classification, acceptance/seen, date, amount, merchant/tag controls. Active summaries/counts expose collapsed restrictions. Collapsing filters preserves query and drafts; Clear filters is explicit. Import activity omits ledger-only period/seen/acceptance controls.

Entering Needs review starts a fresh queue across accessible accounts/periods, clears unrelated filters and selects Needs category with Seen and unseen. Filters remain editable within the queue. Opening or marking a transaction seen cannot remove an uncategorized entry from that default queue. All transactions retains its own seen/acceptance defaults.

Use the shared current-record editor from ledger, dashboard, accounts, categories, periods, imports, transfers and MCP references. Raw source rows without a ledger ID stay source details. Scoped navigation retains the original position across pages and excludes processed rows. Draft guards offer Save and continue, Discard changes or Stay here. Child saves refresh figures/invalidate previews while preserving parent drafts.

Editor details/classification use two desktop columns and stack on mobile. A single allocation uses the parent amount; split inputs/totals appear for splits or inconsistent allocations. Notes/merchant/tags, period assignment and rule offers use disclosures; existing metadata/manual assignment starts expanded. Transfer/source/history are secondary tools above the save footer. Save changes is primary, Save and next secondary; personal seen action sits with status/navigation. Opening alone does not mark seen.

Group and category selectors stay separate. Category search offers Most used (up to five distinct-transaction counts in accessible accounts) and All categories. Typing a new name exposes explicit Enter/Create with Expense/Income choice; typing never saves automatically. Optional account/direction rules show editable description text without changing references. Linked transfers require unlinking before a non-transfer group selection.

Bulk changes require inspectable before/after preview and one apply. Preserve paging, selection versions and request locks; failed writes retain drafts. Import handoff opens All transactions when fully categorized, otherwise Needs review.

## Dashboard and budgets

Dashboard leads with one continuous period-total surface emphasizing remaining budget. Compact period/account navigation contains the selected date-range hint once. Spending and Income each use full-width expandable groups with plain lightly inset category rows, soft tinted group headers and no trailing group border.

A category row opens a wide centered summary modal with group/period context and scoped transactions. Spending shows budget/spent/remaining; Income shows received amounts. Transaction rows open the shared editor and return after save. Selected period carries between Dashboard/Budgets, including periods outside the current page. Next steps and dated bank balances share an aligned desktop support row and stack on mobile.

Budgets has Budget periods and Trends & reports tabs. Edit budgets is primary; View spending quiet; Edit dates/Rebalance use ActionMenu. Builders add explicit groups/categories, preserve unloaded drafts and show This budget only/Upcoming choices. Aggregate category comparisons sum independent group limits once. Rebalance previews show both limits before/after. Report year selection uses whole saved periods starting in that year.

## Accounts, categories and merchants

Accounts prioritizes dated balances and Get transactions/Refresh balances. Names open scoped transactions; setup/connection/edit/hide actions use menus. Detailed import health is collapsed and separates balance date, attempted fetch, successful import/check, schedule and coverage. Settings → Accounts reuses the same management workflow. New accounts default Private; hidden accounts can be explicitly shown without deleting history.

Categories uses horizontal tabs for Categories, Spending groups and Automatic rules. Category/group name rows navigate to scoped transactions with a quiet separate edit action. Categories create with name/type and edit name/archive; no ownership selector. SpendingGroupEditor is shared with global search. Rules expose description/category/optional group; direction/priority/account restriction live under More options. Built-in/custom scope and Pause/Resume/Delete remain clear.

Merchants use account-specific or global naming rules with optional local PNG/JPEG logos and initials fallback. Keep original bank descriptions beneath merchant names. Names/subtitles stack with a small gap. Square logos fill the round avatar; wide/tall wordmarks use contain with inset padding on white. Logo removal is explicit; account rules never silently become global.

## Settings, banking and MCP

Settings uses shared tabs with access-based visibility. General drafts remain mounted; Banking credential drafts clear on leave/cancel/success. Workspace name is admin-managed, 2–60 trimmed characters. General panels align in two desktop columns and stack naturally on mobile. Onboarding says Create admin account / Set up your finance tracker with Username, Password, Confirm password and Create account; show requirements only on validation failure.

Banking shows connection state, primary refresh and last/next dates. Automatic refresh, visibility, troubleshooting and connection controls use disclosures; new connections show the credential form. Schedules start off. Debug shows Chrome without sensitive logging; fixed error codes/numeric diagnostics remain inspectable. Credentials are write-only. Disconnect copy explains retained history. Network Save and Restart are separate; restart is disabled for unsaved drafts/requests/unsupported runtimes and shows reconnection guidance.

MCP shows the endpoint/copy action, own connected agents and change proposals. OAuth sign-in/consent identifies the user and unverified client/callback origin; read-only is default. No manual token display/setup form. Permission editor order: Permission level, Read access/account scope, Allowed changes, Restrictions, Automatic approval, explicit consent. Merchant reads and each change grant remain separate. Previews show exact before/after effects, including grouped budgets/recurrence; JSON wraps/scrolls inside bounded panels. Amounts/dates/descriptions remain sensitive even after redaction; do not claim complete anonymization.

## Verification policy

Current owner instruction: do not inspect UI through computer use until requested. Use focused builds, unit tests, synthetic browser workflows and structural/geometry checks. Cover affected mobile/desktop, themes, populated/empty, long values, errors and keyboard states as appropriate. Preserve owner data/preferences. Record actual coverage and unverified states in [VERIFICATION.md](VERIFICATION.md); automated geometry checks do not constitute manual visual sign-off.
