# UI guide

Status: implemented MVP direction. This is the maintained source for all UI changes. Shared controls are in web/src/ui.tsx and tokens are in web/src/styles.css.

## Product character

A practical personal finance tool: clear figures, readable transactions, predictable controls. Lead with the current budget period, account scope, spending, and work awaiting review.

Avoid obvious AI-generated design conventions: decorative gradients, glowing borders, glass panels, oversized marketing headings, ornamental emoji, excessive rounded cards, and generic motivational copy. Avoid an AI/chat motif unless the user adds an actual AI feature.

Use neutral surfaces, restrained separators, one primary accent, and status colors for meaning. Group related content through spacing and hierarchy; cards only where a distinct section needs them. Choose a readable system font first. Use tabular numerals for money and align amounts consistently.

## Shared system

When implementation begins, define shared tokens for color, spacing, type, radii, focus, and responsive layout. Shared components cover buttons, fields, selects, notices, dialogs, transaction rows, account selector, period selector, category selector, and review badges.

Use established components and variants before adding another style. Record a new accepted convention here and implement it in the shared system in the same change. Do not let examples or isolated screen CSS become a competing style guide.

## Desktop and mobile

Desktop: compact side navigation, content header with period/account scope, data tables where comparisons benefit from columns.

Mobile: primary navigation for Dashboard, Transactions, Review, and More. Preserve each workflow with touch-friendly rows and full-height details/split editors. Move less essential columns into details rather than shrinking the table. Show date, description, amount, category, and review state without page-wide horizontal scrolling.

At 360px width, upload, mapping preview, review, account selection, period editing, and splitting must remain usable. CSV comparison grids may scroll inside a clearly bounded region. Controls should offer approximately 44px touch targets; don't depend on hover.

## Money and status

Always show selected period and account scope. Format money using configured locale and currency; preserve clear debit/credit signs. Never use color alone to convey income, expense, duplicate risk, or approval.

Distinguish pending review, approved, uncategorized, and possible duplicate with text. Show suggestion provenance without an AI sparkle badge. Dashboard totals explain pending inclusion and import coverage.

Split editor shows original amount, allocated total, and remaining amount; approval cannot succeed with an imbalance. Category totals must not display parent and allocations together.

## Interaction and copy

Prefer concrete labels: Upload CSVs, Review transactions, Split transaction, Approve, Save period.
Use plain explanatory copy for duplicate candidates and boundary changes. Empty states explain the next action; errors identify the file/row/field and how to resolve it.

Preserve filters and scroll position when returning from transaction details. Show loading, empty, error, and partial-import states intentionally. Make permission-limited and read-only states understandable.

## Accessibility and verification

Provide labeled controls, visible keyboard focus, semantic headings, accessible dialogs, sufficient text contrast, and status announcements. Charts also need text summaries. Validate real upload/review/split flows on desktop and mobile, keyboard navigation, long descriptions, large/signed amounts, and error states. Use synthetic finance data for screenshots.

## Change record

2026-10-02: initial guide created from user's requirements; visual choices remain proposals.

## Implemented conventions

Light/dark themes follow the device by default, with a local explicit override. Neutral surfaces, restrained forest-green accent, amber pending and red errors; 8px panel radius, 6px control radius; system font stack and tabular money. No gradients or decorative marketing visuals.

Desktop uses side navigation, a two-column dashboard, and compact transaction rows. Mobile uses Dashboard/Transactions/Review/More bottom navigation and full-height native dialogs. Progress bars show category limits with text equivalents; categories remain independent of spending groups. Native dialog focus management and Escape are preserved.

Shared Field uses explicit label/control IDs and separate descriptive hints. Buttons have consistent variants and minimum 44px height. Never add independent screen themes or hardcoded status colors. Use synthetic financial data for screenshots.

2026-10-02: implemented shared controls, responsive layouts, explicit pending states, light/dark themes, upload previews and period reassignment previews.

2026-10-02: First-run onboarding reuses the sign-in layout, Field and Button controls, and device theme. Show administrator username, new password and confirmation. Validate password equality before submission; announce errors inline without losing input. Successful setup opens the dashboard. Existing installations show sign-in, with no public account creation option.

2026-10-02: Accepted onboarding copy: heading “Create admin account”; description “Set up your finance tracker.”; fields “Username”, “Password”, “Confirm password”; button “Create account”. No field hints, eyebrow, or card footer. Show password requirements only on validation failure, in plain language; keep byte limits internal.

## Form validation

Use shared Field and Form controls. An untouched field stays quiet, including on initial autofocus. First blur marks it touched and displays its current error. Subsequent typing revalidates immediately, clearing or updating the error; dependent fields (password confirmation, date ranges, allocations) revalidate when related values change. Submission marks invalid fields touched, blocks the request, and focuses the first invalid control.

Errors appear directly beneath their input, with an invalid border, aria-invalid, and aria-describedby pointing to the error text. Do not use native browser validation popups. Identifiable server errors (duplicate usernames/account numbers, incorrect current password) belong to their fields and clear when edited. Request-wide authentication, connectivity, permission, or stale-version failures use shared overlay toasts. Preserve values when validation fails. Button-driven editors use the same validation checks as forms.

2026-10-02: Accepted touched-field validation convention; shared controls implement it for onboarding, sign-in, account/category/rule/user forms, settings, limits and transaction/period editors.

## Computer vision review

Use focused builds, tests and structural checks for routine changes. Perform short, targeted computer-use visual checks when requested or when layout/interaction risk warrants them. Select affected viewports, themes and states; use synthetic data and preserve owner data/preferences. Record actual coverage and limitations; never claim screenshots or visual verification that were not performed.

## Responsive form and dialog alignment

Align adjacent field labels and inputs at the top; helper text and errors must not shift neighboring inputs. Align direct form-grid action buttons with input controls. Keep allocation controls aligned when errors appear, and give signed amounts enough width on mobile. Stretch grouped dialog actions to a consistent height when labels wrap.

Dialog headings must wrap within available space while the close button remains visible. Lock background scrolling while a dialog is open, preserving dialog scrolling and the page's scrollbar gutter. Stack user names above role controls on mobile. Between 761px and 1000px, use two summary columns and stacked period/account filters so currency decimals remain intact.

The recorded computer-vision audit is in [UI-VALIDATION.md](UI-VALIDATION.md).

2026-10-02: Dropdown text and arrow use matching 12px outside insets. Reserve additional space between text and the arrow, keep the native select interaction, use a theme-aware chevron, and retain the native arrow in forced-colors mode.

## Card alignment

Cards should align horizontally and vertically wherever the layout allows: stacked cards share the same outer width and left/right edges; cards in a grid share row top/bottom edges, consistent gaps, and column boundaries. Use shared grid tracks and stretching rather than independent maximum widths or arbitrary fixed heights. Keep mobile stacks at natural content height, and visually verify both empty and populated states.

2026-10-02: Imports upload, previews, and history share the full content width. The spending card spans the combined height of the adjacent dashboard cards, with centered empty content. Account grids use one consistent gap without extra card margins.

## Spending groups and category selection

Spending group and Category are separate controls in transaction details. Spending groups use a named label and a small theme-aware colored dot; selection also shows a check, never color alone. Categories are a flat list with an expense/income type; creating a category asks for a name and type only. Category search shows Most used (up to five, based on distinct transactions in accounts the signed-in user can access) and All categories, with explicit creation inside the picker. Picker search, rows and creation controls support keyboard navigation and native dialog Escape/focus restoration. New category forms use ordinary touched-field validation. On mobile, category selection spans the allocation width, with amount and note aligned below it.

The Categories page manages Spending groups separately from a flat Categories list. Shared selector controls, rows and dots live in Choices.tsx; use existing theme surfaces and focus styles. Group names never replace the explicit transfer designation.

2026-10-02: Accepted independent spending groups with eleven editable defaults, searchable category selection and Most used. Existing transactions are preserved; no silent classification or financial behavior inferred from group names.

2026-10-02: Clarified that categories never belong to spending groups. Removed category-group inputs, category-group labels and the old category-group totals from the UI. A transaction may select Day-to-day + Eating Out or Exceptions + Eating Out. Category creation no longer requires a group.

2026-10-02: Workspace name is administrator-managed, 2–60 trimmed characters, authenticated branding only. Navigation truncates long names with full title text; page branding wraps. Generic sign-in title preserves privacy. Budget limits reuse shared category creation constrained to expense type, preserve draft amounts, and focus the newly created zero-limit field. Reduced visual inspection policy applied from the recorded owner request.

Current owner instruction (2026-10-02): do not use computer use to inspect the UI for now. Use automated tests/builds and structural checks; resume visual inspection only when requested.

Shared inline category creation validates a trimmed nonempty name and the existing backend UTF-8 length bound before submission. Budget creation is expense-only and retains existing duplicate-name server feedback beneath the name input.

2026-10-02: Accounts supports Import discovered accounts for owner-run account-only JSON. Reuse shared Modal/Field/Form controls, inline file/number errors, draft name/number edits and explicit per-account creation. Private is the default; household sharing is opt-in. Explain existing accounts and masked-number correction; no financial rows or balances are imported. Follow the current no-computer-use inspection instruction.

2026-10-02: Accounts includes an FNB connection section using shared Field/Form/Button controls. Username/password are write-only, cleared after successful save, never localStorage. Refresh now is explicit; schedule is off until saved. Show dated ledger balances, last attempt/success, next due and sanitized needs-attention state. New accounts default Private. Hidden discoveries require an explicit Show hidden discovered accounts toggle and a Show account action; explanatory copy states that hidden history remains stored but is excluded from current views/totals. Partial discovery reports skipped count. No computer-use inspection.

2026-10-02: FNB refresh is headless by default. Add a saved Debug mode — show Chrome during refresh checkbox and Save browser mode button using existing form controls. Explain phone/browser approvals and show count-only layout diagnostics in a disclosure, with a safe-to-share BALANCE_COUNTS line. Debug affects manual and scheduled runs and adds no screenshots/page dumps/logs. No computer-use inspection.

2026-10-02: Keep configuration under Settings. Settings uses General, Banking, Accounts, Users & access, Backups, Network and Security subtabs; admin-only tabs respect existing permissions, ordinary users get General/Security. Reuse shared Tabs/Button styles, roving keyboard focus with Left/Right/Home/End, labeled tabpanels, wrapping controls at 360px and natural full-width panels. General drafts persist across subtab changes. Banking unmounts when left, discarding transient credential drafts. Account setup/sharing, discovery-file import and hide/show live in Settings. Accounts shows authorized balances and a Manage accounts shortcut. Appearance is configured in General, with the previous topbar theme shortcut removed. Budget/category/transaction workflows remain their own product pages.

2026-10-02: Transient success/errors use shared fixed overlay Toast, including above dialogs; no flow-inserted page/modal notification cards. Accessible status/alert, dismiss action, expiry with hover/focus pause. Inline field errors remain. Banking groups have 24px form spacing, helper text/button separation and stable diagnostic disclosure; persistent failure details live inside diagnostics while request errors toast. Native manual popover places the toast in the top layer without stealing focus; portal it into the active dialog to remain interactive under modal inertness.

2026-10-02: Administrator-only Settings → Network uses shared fields/forms and overlay toasts. Show active URL/proxies/source and restart status; edit/save validated values without interrupting the current session, reload stale drafts explicitly, and offer Use environment settings. Save applies after service restart. NPM forwarding/certificates remain managed in NPM. Keep spacing, wrapping and full-width shared panels at desktop/360px; no computer-use inspection.

2026-10-02 balance currency follow-up: owner-provided screenshot identifies numeric-identity eBucks and foreign-currency ledger rows as unsupported by the ZAR-only tracker. Worker now skips recognized reward/foreign currency units, reports fixed reward_entries/non_zar_entries counters, and continues the supported ZAR snapshot. Masked identifiers remain skipped. No conversions or unit stripping into ZAR; unfamiliar text still fails closed. Previously stored accounts/history remain intact. Banking skipped-entry copy explains number/unit exclusions. Screenshot financial values/identities are not copied into fixtures; all tests use synthetic values. Owner diagnostics confirm logout; next live balance refresh remains owner-run.

2026-10-02: Banking uses progressive disclosure: connected summary/primary Refresh now and latest/next sync dates remain visible; Automatic refresh, Account visibility, Troubleshooting and Connection settings start collapsed. Native summary controls provide keyboard/touch interaction with short saved-state/count summaries. Diagnostics and debug share Troubleshooting, without nested diagnostic disclosure; credentials/disconnect stay in Connection settings. New connections show the credential form directly. Retained account visibility remains accessible when disconnected. Shared buttons/forms/toasts and spacing apply; no computer-use inspection.

2026-10-02: Settings → Accounts rows place the account name on its own line, with bank/ending-number/sharing metadata beneath it and a 6px gap. Long names wrap while the edit button retains its width.


2026-10-03 owner simplification: Categories starts on Rules, with shared tabs for Categories and Spending groups. The normal rule form is description contains, category and spending group. New rules default to all current enabled editable accounts; account restriction, payment direction and priority sit under More options. Identical definitions across accounts appear as one row with account scope, Edit, Pause/Resume and Delete. Create category is available inside the editor via the shared category form. Check an example is optional. No Vault22 upload/mapping/comparison screen or transaction import is present in this flow. Product copy identifies the FNB connection as the intended transaction source. Shared form validation/toasts/accessibility and mobile layout remain authoritative.


2026-10-03 starter classification: Rules identifies built-ins and their all-enabled-account scope. Edit, pause/resume and delete use existing controls; custom rules take precedence. The transaction category picker says to choose an existing category or type a new name and press Enter. Unknown names expose a Create action and simple Expense/Income choice, default Expense; no second form or automatic save on typing. Request failures toast; category-name errors remain inline. Transaction editor offers an explicit similar-transactions checkbox for single-category non-transfer nonzero transactions, plus a description pattern field. Transaction and rule are saved together.

2026-10-03: Imports includes an administrator-only Live FNB transactions section with Fetch FNB transactions, explicit preview/confirmation/review copy and first-page warning. Live previews use existing shared import rows/category suggestions/duplicate choices and show returned coverage, excluded pending authorizations and page-limit gaps. Empty snapshots cannot be confirmed. Durable previews resume through Import history. Banking explains that schedules remain account/balance refreshes and transaction fetch is in Imports. Shared Button/Field/toasts, widths and pagination remain authoritative; no computer-use inspection.

2026-10-03: Live preview coverage distinguishes bank rows from additional service-fee entries. Fee entries display their bank-row number and originating description, preserving context while using the standard classification/duplicate/review workflow. Fetch success reports the number of account previews returned. Banking transaction failure guidance points to Fetch FNB transactions in Imports; credential replacement guidance is no longer shown for transaction extraction errors.

## Accepted UI backlog conventions (2026-10-03)

Account management is shared between Accounts and Settings → Accounts. Visible accounts offer a compact native disclosure menu for Edit, Refresh and Hide; hidden accounts have a separate paged restoration disclosure. Banking contains connector credentials, schedule, debug and troubleshooting. Account update status stays visible outside the menu. Page-wide refresh updates eligible visible accounts; a card refresh names only that account. Current permissions still govern every action.

Native details/summary controls share theme tokens, subtle borders/backgrounds, a disclosure indicator, visible keyboard focus and mobile touch spacing. The shared Spinner/Loading and Button loading prop announce pending work, disable duplicate action submissions and respect reduced motion. Button labels and dimensions stay stable; useful loaded content remains visible during refresh. Data-load errors expose retry and request failures use overlay toasts.

Large lists use shared page controls and bounded server requests. Search/filter/order happen before paging. Lookup controls preserve selected values outside the current page and provide server search/navigation; category/group choices retain Most used and explicit create. Grouped rules page by definition and retain all account members for atomic editing. Budget limit drafts, import duplicate decisions and transaction selections persist across pages. Transaction review has an explicit selected-row scope and a 100-row limit. Import-wide candidate skip is an explicit action. Totals cover the full authorized dataset; page navigation never submits a form or saves edits.

List snapshots reject changed offset sequences and restart navigation with a toast, rather than silently skipping or duplicating rows. Stale import previews require reload before confirmation. Empty/loading/error/end states use the same shared controls. Verification follows the current no-computer-use owner instruction; automated DOM/overflow assertions are recorded separately from visual inspection.

2026-10-03: Owner rejected the basic white disclosure triangle. Shared disclosure headers now use a muted outlined chevron at the right, rotating upward when expanded, with theme-aware hover color and reduced-motion support. Native details/summary keyboard semantics and focus remain. Banking helper text aligns with the heading; compact account hamburger menus retain their existing icon without an extra chevron.

2026-10-03: Account hamburger triggers are 44px squares with the icon centered on both axes. Account action menus dismiss on outside pointer interaction (including touch), focus moving outside, opening another account menu, or Escape. Escape restores trigger focus; outside interaction keeps the destination's normal focus/action behavior. Native disclosure keyboard activation remains.

2026-10-03: Native file-selector buttons use the shared secondary-button surface, border, radius, typography and 44px target instead of browser-default chrome. Filename text uses the muted token and transparent background. Keep native keyboard/file-picker behavior, file type/multiple selection and disabled state; apply this consistently to export uploads and discovery files.

2026-10-03: Button spinners sit on the vertical centerline, including multiline buttons. The later inline-spinner convention below supersedes the original absolute inset. Labels accompanied by a spinner have no trailing ellipsis; the spinner communicates ongoing work. Search placeholders keep their existing punctuation.

2026-10-03: Export upload uses one full-width clickable drop zone with centered icon, selection text and helper copy. The duplicate native chooser row is hidden. A native button preserves Enter/Space selection; a hidden multiple-file input opens the OS picker, while drag/drop uses the same selection state and explicit Preview import. Show selected names once, support replacement, highlight file drags and disable selection during work. Existing server file limits/validation and confirmation remain authoritative. Discovery-file inputs retain their native styled chooser.

2026-10-03: Button spinners now participate in the shared flex layout with an 8px gap before the label, using the standard 14px indicator. This supersedes absolute positioning inside the button padding, which crowded the text. Keep spinner and label vertically centered; allocate room for the indicator instead of overlaying the label.


## Whole-program polish (2026-10-03)

Owner requested a premium, consistent application across all screens. The shared system now defines neutral light/dark surfaces, control radius and height, spacing steps, 12/13/14px text sizes, accent foregrounds and restrained panel/overlay shadows in styles.css. Secondary labels, badges and footnotes use at least 12px text; financial amounts retain tabular numerals. System fonts require no external font service. The stylesheet is formatted as readable rules for maintenance.

All signed-in pages use PageHeader, with the same workspace label, heading, description and currency metadata. Workspace refresh status sits beside header metadata without a permanent empty loading row. Navigation has a restrained selected indicator, tabs use a shared selection/underline treatment, and mobile navigation highlights both selected pages and expanded More. Skip to content provides keyboard access to the main region. Focus styles include textarea, links and native disclosures; forced-colors selections remain visible.

Panels, actions, dialogs, menus, search controls and pagination use the shared visual vocabulary. Dialog/editor actions have a consistent separator and spacing; native focus, Escape, disclosure and menu behavior are retained. Transaction pagination now reuses Pagination with a transaction-range label; its selection and description headings share row tracks. Selected rows have a theme-aware surface, with checkboxes retaining explicit selection semantics. Long account/category names and large balances wrap without page overflow.

Field reserves one empty feedback line beneath each control (after a hint, if present). Untouched fields still show no error. This prevents a one-line first-blur error from moving a button between pointer-down and pointer-up, which previously swallowed mobile submissions. Longer errors may grow naturally. Keep first-blur/live validation, inline error descriptions and first-invalid-field focus. Do not replace this space with a hidden element that collapses its geometry.

Verification uses isolated synthetic data and automated browser DOM/geometry/keyboard checks under the owner's current no-computer-use policy. This work is presentation and interaction polish; backend financial and connector behavior has not changed.
