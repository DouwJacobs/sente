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

Transactions contains All transactions, Needs review and Import activity. Search stays visible with a short current account/period summary; Filters reveals account, period and remaining classification, acceptance/seen, date, amount, merchant/tag controls. Active summaries/counts expose collapsed restrictions. Collapsing filters preserves query and drafts; Clear filters is explicit. Import activity omits ledger-only period/seen/acceptance controls.

Entering Needs review starts a fresh queue across accessible accounts/periods, clears unrelated filters and selects Needs category with Seen and unseen. Filters remain editable within the queue. Opening or marking a transaction seen cannot remove an uncategorized entry from that default queue. All transactions retains its own seen/acceptance defaults.

Use the shared current-record editor from ledger, dashboard, accounts, categories, periods, imports, transfers and MCP references. Raw source rows without a ledger ID stay source details. Scoped navigation retains the original position across pages and excludes processed rows. Draft guards offer Save and continue, Discard changes or Stay here. Child saves refresh figures/invalidate previews while preserving parent drafts.

Editor details/classification use two desktop columns and stack on mobile. A single allocation uses the parent amount; split inputs/totals appear for splits or inconsistent allocations. Notes/merchant/tags, period assignment and rule offers use disclosures; existing metadata/manual assignment starts expanded. Transfer/source/history are secondary tools above the save footer. Save changes is primary, Save and next secondary; personal seen action sits with status/navigation. Opening alone does not mark seen.

Group and category selectors stay separate. Category search offers Most used (up to five distinct-transaction counts in accessible accounts) and All categories. Typing a new name exposes explicit Enter/Create with Expense/Income choice; typing never saves automatically. Optional account/direction rules show editable description text without changing references. Linked transfers require unlinking before a non-transfer group selection.

Bulk changes require inspectable before/after preview and one apply. Preserve paging, selection versions and request locks; failed writes retain drafts. Import handoff opens All transactions when fully categorized, otherwise Needs review.

## Dashboard and budgets

Dashboard leads with one continuous period-total surface emphasizing remaining budget. Compact period/account navigation contains the selected date-range hint once. Spending and Income each use full-width expandable groups with plain lightly inset category rows, soft tinted group headers and no trailing group border.

A category row opens a wide centered summary modal with group/period context and scoped transactions. Spending shows budget/spent/remaining; Income shows received amounts. Figure tracks use their own available width: below 480px, label/value pairs stack as full-width rows instead of squeezing three narrow columns. Keep signed amounts on one line. Exceptionally long overview totals use wider tracks; ordinary totals retain the compact continuous grid. Transaction rows open the shared editor and return after save. Selected period carries between Dashboard/Budgets, including periods outside the current page. Next steps and dated bank balances share an aligned desktop support row and stack on mobile.

Budgets has Budget periods and Trends & reports tabs. Edit budgets is primary; View spending quiet; Edit dates/Rebalance use ActionMenu. Builders add explicit groups/categories, preserve unloaded drafts and show This budget only/Upcoming choices. Group headers, totals and actions stack on phones; category amounts and schedule controls use full-width rows with separate removal actions. Long names wrap while monetary figures remain complete. Aggregate category comparisons sum independent group limits once. Rebalance previews show both limits before/after. Report year selection uses whole saved periods starting in that year.

## Accounts, categories and merchants

Accounts prioritizes dated balances and Get transactions/Refresh balances. Names open scoped transactions; setup/connection/edit/hide actions use menus. Detailed import health is collapsed and separates balance date, attempted fetch, successful import/check, schedule and coverage. Settings → Accounts reuses the same management workflow. New accounts default Private; hidden accounts can be explicitly shown without deleting history.

Categories uses horizontal tabs for Categories, Spending groups and Automatic rules. Category/group name rows navigate to scoped transactions with a quiet separate edit action. Categories create with name/type and edit name/archive; no ownership selector. SpendingGroupEditor is shared with global search. Rules expose description/category/optional group; direction/priority/account restriction live under More options. Global fallback/account scope and Pause/Resume/Delete remain clear.

Merchants use account-specific or global naming rules with optional local PNG/JPEG logos and initials fallback. Keep original bank descriptions beneath merchant names. Names/subtitles stack with a small gap. Square logos fill the round avatar; wide/tall wordmarks use contain with inset padding on white. Logo removal is explicit; account rules never silently become global.

## Settings, banking and MCP

Settings uses shared tabs with access-based visibility. When the strip overflows, persistent 44px left/right controls expose off-screen sections; the active tab stays within the visible strip on section changes and resizing. Keep the existing arrow/Home/End keyboard semantics and labelled panels. Scrolling tabs must not remount draft panels. Configuration is available to administrators with budget membership: repository and file forms sit in aligned panels with the same vertical spacing above and below their row, previews show named before/after changes with explicit replacement confirmation, and source history offers manual pulls and Forget source. Export downloads a portable JSON snapshot. Configuration drafts remain mounted across Settings tabs. General drafts remain mounted; Banking credential drafts clear on leave/cancel/success. Workspace name is admin-managed, 2–60 trimmed characters. General panels align in two desktop columns and stack naturally on mobile. Onboarding says Create admin account / Set up your finance tracker with Username, Password, Confirm password and Create account; show requirements only on validation failure.

Banking shows connection state, primary refresh and last/next dates. Automatic refresh, visibility, troubleshooting and connection controls use disclosures; new connections show the credential form. Schedules start off. Debug shows Chrome without sensitive logging; fixed error codes/numeric diagnostics remain inspectable. Credentials are write-only. Disconnect copy explains retained history. Network Save and Restart are separate; restart is disabled for unsaved drafts/requests/unsupported runtimes and shows reconnection guidance.

MCP shows the endpoint/copy action, own connected agents and change proposals. OAuth sign-in/consent identifies the user and unverified client/callback origin; read-only is default. No manual token display/setup form. Permission editor order: Permission level, Read access/account scope, Allowed changes, Restrictions, Automatic approval, explicit consent. Merchant reads and each change grant remain separate. Previews show exact before/after effects, including grouped budgets/recurrence; JSON wraps/scrolls inside bounded panels. Amounts/dates/descriptions remain sensitive even after redaction; do not claim complete anonymization.

## Verification policy

Current owner instruction: do not inspect UI through computer use until requested. Use focused builds, unit tests, synthetic browser workflows and structural/geometry checks. Cover affected mobile/desktop, themes, populated/empty, long values, errors and keyboard states as appropriate. Preserve owner data/preferences. Record actual coverage and unverified states in [VERIFICATION.md](VERIFICATION.md); automated geometry checks do not constitute manual visual sign-off.


2026-10-07 user security: Users & access provides per-user Reset password/Delete user actions for other users. Reset opens a shared dialog with new password and dependent confirmation and explains session/agent revocation. Delete explains permanent access removal and retained financial history/attribution, and requires the exact saved username in a validated field. Cancel is non-destructive; closing during a request is blocked. Own password changes remain in Security with current/new/confirmation fields and explicit copy that the current session stays active while other sessions and agent connections are revoked. Errors use shared inline fields or overlay toasts. Deleted usernames stay reserved for historical attribution; deleting oneself requires a different administrator.


2026-10-07 owner follow-up: every password field uses the shared eye Show/Hide control, including onboarding/sign-in, user creation/reset, self-service changes and FNB credentials. Passwords begin hidden, return to hidden when cleared and keep their value/validation/autocomplete semantics when toggled. The labelled non-submit button controls its input, exposes pressed state, supports keyboard activation and has a 44px target. Pointer activation preserves input focus. User rows have one shared hamburger ActionMenu with Edit; other-user reset/delete actions remain in that same menu, with own password changes still in Security.

Owner refinement: password eye buttons stay borderless with a transparent background at rest and on hover; preserve the shared visible keyboard focus outline.

## Application information

Settings → About is available to every signed-in user. Show the backend application version, commit and revision date, with honest loading/unavailable states and retry. Keep repository, documentation, issue tracker and Report an issue links together. Report links prefill public build fields only and leave final submission to the user on GitHub. The sidebar footer shows a compact version-only text control with zero margin/padding and no hover surface (Development for dev builds), without commit/modified details; it opens About. About uses the existing Sente mark, grouped project links, a primary Report an issue action and an inset installation-details surface, stacking naturally on mobile. Mobile users reach About through Settings. Licence information reflects the repository declaration.


MCP setup uses three short steps and a privacy disclosure. Personal context has its own shared Field/Form section, character limit, Save context and explicit Reload saved context; drafts survive Settings tab changes. Agent cards show sharing status. Consent separates default finance reads, optional merchant/context reads, account scope, proposed changes and optional automatic approval. Summaries use short lists and are collapsed in the editor to avoid duplicating choices. Draft changes clear confirmation; invalid combinations block submission with inline validation.

## Mobile core workflows

Primary phone navigation is Dashboard, Transactions, Review and More. Respect budget-member visibility; Accounts, Budgets, Categories and Settings are reached through More. Review opens the fresh Needs review queue with existing category/seen defaults. Show the active queue separately from All transactions and identify secondary-page selection through More. Measure the rendered bottom-navigation height, including safe-area padding, for both page clearance and More positioning; do not use unrelated fixed offsets. More dismisses on outside interaction or Escape, with Escape returning focus to its trigger.

Keep the household name within the available header width, with its full value in a title. Search, theme and sign-out retain 44px targets; the desktop username may be omitted in the narrow header. Dashboard scope uses two balanced columns on phones; expanded transaction scope controls stack to keep account/period values readable. Desktop selection labels remain separate from opening rows. On phones, hide the checkbox column and use the complete transaction row as a touch target.

Phone transaction rows put the readable merchant/description and complete signed amount on the first line, followed by date/account and concise classification/status. Limit the mobile merchant preview to two lines; retain full accessible text/title and editor details. Use distinct labelled check/question icons for Accepted/Needs review and eye/crossed-eye icons for Seen/Unseen, with hover descriptions; do not rely on color alone. Keep the indicators inline with category text and preserve private/transfer/split text. Original bank descriptions beneath merchant names are secondary on phones and remain available in the editor. On phones, a 500ms long press selects the first row; subsequent taps toggle selection. Moving more than 10px, pointer cancellation or leaving the row cancels the pending hold. Normal taps open the editor outside selection mode. A 36px merchant logo is centred vertically and displays a checkmark overlay when selected. Keep keyboard Space selection and an explicit Select transactions menu action. Deselecting the last row exits selection mode. The existing toolbar count changes to the selected count; edit/seen/cancel actions stay inside its shared menu without inserting buttons or helper blocks into the page. Desktop checkbox selection remains separate from opening the transaction. Existing filter drafts, 100-row selection limits, paging and scoped navigation remain authoritative.

Phone editor fields stack vertically. Split category and amount inputs use separate rows, with a dedicated 44px remove control next to the category. Keep disclosure targets touchable and save/next/close controls full-width on narrow screens. Shared dialogs fit the visual viewport when its height/offset changes, retain fixed headings and a scrollable body, lock background scrolling and restore nested opener focus. Preserve shared first-blur/live validation and reveal/focus invalid fields on submission.

Cover 360/390/430px phones, short landscape, both sides of relevant editor/navigation breakpoints and desktop, including light/dark, long names, large signed values and nested drafts. Run suites sequentially with the existing disposable-fixture runner unless trace/output directories are isolated. Real-device keyboards, safe areas and browser-specific behavior require separate evidence. The owner authorized a targeted computer-use check for this batch on 7 October 2026; this does not replace the standing inspection policy for future work.

Owner mobile refinements (7 October 2026): use compact headings, overview figures and panel spacing. Dashboard period/account selectors share two equal columns with period navigation below; expanded transaction filters still stack. Pad every group/category track, including figures and progress bars, from both card edges. Keep phone merchant logos centred beside the row text and full rows touchable; desktop checkbox selection remains available. Search uses the same quiet borderless icon treatment as theme. Normal mobile transaction rows use three compact lines without acceptance/seen pill badges; exceptional assignment warnings retain text. Avoid duplicate Transfer labels and numeric false-value artifacts on private rows.

Categories uses the same spending-group grid container, choice-row spacing, borders and responsive columns as Spending groups. Preserve the type/archived labels, category-to-ledger navigation and separate quiet edit action. Vault22 screenshots supplied by the owner are inspiration for compact row density and clear hierarchy; retain Sente theme tokens and financial semantics.

## Compact status and rules conventions

Use labelled StatusIcon indicators for routine list states and types, rather than pill badges. Icons are 14–16px with accessible names and hover descriptions. They do not act as buttons; keep their cursor neutral. Category Expense/Income uses minus/plus circle symbols, not upward/right arrows that suggest navigation. Reserve navigation arrows for actual links or controls. Explanatory warning, consent, permission and unusual-state wording remains visible beside a compact icon, with no pill border/background/padding; the legacy Badge API now renders this shared presentation. Currency is plain text.

Automatic rule rows show the match without a Contains prefix, classification and scope. Merchant rule rows show only merchant name, category and spending group; match, scope, direction and priority remain in Edit. Both use inline state icons and one shared actions menu. Keep grouped account mutations, edit/pause/resume/delete, merchant previews, paging, validation and request locks intact. Merchant rows centre an equal-width/height 36px circular avatar with matching fixed flex basis and overflow clipping; preserve wide/tall wordmark containment. Priority remains in the editor's More options. Avoid separate full-width action rows on phones.

## Compact workspace controls

Dashboard and Transactions use compact headings and smaller vertical gaps so figures and rows lead the page. Avoid repeated eyebrow/helper copy above familiar workflows; preserve accessible page descriptions. Dashboard period/account selectors form an unboxed scope strip with icon period navigation and readable dates. Phone scope uses two columns with navigation beneath. Keep icon controls at least 44px.

Transaction tabs use a compact single row. Search remains immediately available, beside one quiet Filters control and a clear action when filters are active. A short summary always identifies the selected account and period and additional filter count. Accounts, periods and detailed filters live in an initially closed disclosure. Preserve filter state across transaction tabs, review defaults and imported-transaction scope. Escape closes the disclosure and returns focus to Filters; hiding controls must not reset their values. Use shared fields and theme tokens, retaining inline validation. Do not reserve blank validation space for non-validating search/scope controls. Keep toolbar and table-header gaps modest, without shrinking touch targets or adding a large filter card.

## Persistent notifications

The signed-in header bell opens Notifications and exposes an accessible unread count. Persistent messages live in a paged All/Unread/Read list with text severity/state, timestamp, explicit read/dismiss actions and authorized source navigation. Opening the centre or a source does not silently mark messages read. Keep request feedback in overlay toasts. Centre entry and read/dismiss refresh return focus to main content; transaction editors restore the connected source button. On phones the header retains 44px icon targets and messages/actions wrap within the page. Settings → Notifications provides in-app type preferences in one shared form with a single Save changes action. Save every changed value atomically; disable submission when nothing changed. Preserve unsaved drafts across Settings sections; failed/stale saves keep drafts, while explicit Reload saved preferences replaces them.

Owner-requested density refinement: notification preference labels and enabled/disabled selects share compact rows, in two columns on desktop and one on phones. Explain in-app delivery once above the grid rather than repeating a hint per type. Keep 44px controls, visible field errors and one compact Save changes/Reload saved preferences action row; use spacing without trailing separators.
