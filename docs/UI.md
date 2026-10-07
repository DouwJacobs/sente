# UI guide

Status: modern colored finance direction (2026-10-03), superseding the retro design trial. This is the maintained source for all UI changes. Shared controls are in web/src/ui.tsx, structural rules in web/src/styles.css, and active visual tokens in web/src/finance-theme.css.

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

The recorded computer-vision audit is in [VERIFICATION.md](VERIFICATION.md).

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


## Experimental retro design trial (2026-10-03)

Owner supplied an Antimetal-inspired design prompt and explicitly requested only its design, with the existing stack and app purpose retained. This trial supersedes the preceding green/neutral visual direction while keeping the existing financial, validation, permission and navigation contracts. Baseline checkpoint: 526260f.

Presentation overrides are centralized in web/src/retro.css after the shared structural stylesheet. Light page background is #d7d7d0, body text/CTA accent #1a1614, CTA foreground #f4f4e7; the remaining palette is #d8d8d8, #a8a8a8, #606060, #303030 and #909078. Muted small text uses #303030 on light surfaces for AA contrast rather than the insufficiently contrasted light greys. Existing dark appearance remains available using the supplied charcoal/cream palette. Group names, explicit signed amounts and textual statuses retain meaning in a monochrome treatment; pending uses a dashed badge, approved a solid badge and errors a heavier border plus an inline exclamation indicator.

Inter is the requested fallback for the unavailable Test Signifier font: self-hosted Latin WOFF2, with its OFL license and system fallbacks in web/public/fonts. Optional font display avoids a late font swap. Page headings use 54px desktop/34px mobile, regular weight; section headings 24px. Supporting page descriptions use 24px desktop with narrower adaptations. Ledger rows, inputs and operational copy remain 12–16px so dense financial workflows stay usable. Shared pill buttons are 14px/500 with 44px targets. Square panels and dialogs, thin borders, tint changes and 8px-based major spacing replace all elevation shadows. No new marketing page, fictitious metrics, customer endorsements or product features were added.

PixelScene is an original decorative low-resolution savings landscape in the Dashboard header, not a visualization of ledger values. It reacts subtly to pointer/scroll, stops requesting frames once settled, honors reduced-motion and cleans up events/observers on unmount. PixelMark supplies a simple original bar glyph. No copied logo, proprietary typeface, stock image, WebGL library or external runtime embed is used. Button hover scale is restrained and disabled under reduced motion. Existing keyboard focus, native dialogs/disclosures, first-blur feedback spacing, and mobile workflows remain shared.

This is the initial design trial only. Future changes should follow the owner's requested individual refinements; do not redesign unrelated sections. Current no-computer-use verification policy remains in force.


2026-10-03 theme interaction alignment: owner reported overly faint light-theme hover feedback. Shared interaction tokens now use #a8a8a8 hover fill/#606060 hover border in Light and #606060 fill/#a8a8a8 border in Dark; primary hover uses #303030/#d7d7d0 respectively. Explicit shared selectors override inherited quiet-button, navigation and row styles. Apply the same treatment to secondary/quiet/danger buttons, tabs, navigation, dashboard/transaction rows, disclosures, category choices/pickers, inputs, upload targets, mobile More and native file selectors. Selected navigation/tabs/rows retain their selected surface during hover; disabled buttons retain their resting surface and quiet controls their transparent border. Focus remains a separate visible 2px outline. No layout/type/palette redesign.


## Modern colored finance direction (2026-10-03)

Owner found the retro treatment dull and explicitly authorized color plus changes to layout, cards, inputs and buttons to suit a modern budget/tracker. This supersedes the monochrome palette, square surfaces, oversized headings and pill-button trial. The active global theme is finance-theme.css; retro.css has been removed rather than retaining competing overrides. styles.css supplies shared structural/responsive and validation rules. The locally hosted Inter font and small original pixel landscape remain; the latter now uses green/gold and a compact footprint.

Light uses neutral light-grey pages, white panels and dark ink; Dark uses layered neutral charcoal pages/panels with cool off-white text. Teal is the primary action/navigation accent. Income is green, spending terracotta, budget position blue and awaiting-review amber. Errors, negative monetary amounts and over-limit spending are red; textual labels/signs remain authoritative. Spending-group dots again display their saved colors. Shared hover/focus/selected/disabled roles are retained in both themes, using neutral grey hover fills and distinct borders, with teal reserved for primary/selected states. Danger actions use an error-tinted hover.

Dashboard period totals are four independent, equally tracked summary cards with labeled decorative icon tiles and colored figures. The budget-position card has a subtle blue surface; negative remaining amounts retain the error color. Cards stretch to align row edges and use natural content height. Below, category spending and the next-step/balance cards retain consistent page/grid edges. A category exceeding its positive target shows a red progress indicator and an exact-money over-limit label; household target comparison is omitted in single-account scope as before. Totals, classification/review and ledger semantics remain unchanged.

Use 14px panel corners, 9px form/button corners, 36px desktop/30px mobile page headings, 18px section titles and compact readable operational text. Page descriptions and the pixel header art are smaller than the retro trial. White/raised dark inputs and solid teal primary actions give forms clear hierarchy. Tabs, disclosures, menus and account cards share the same vocabulary. Restrained panel/overlay shadows are permitted for this modern direction; no gradients or ornamental marketing sections. Preserve shared touched-field validation/feedback space, overlay toasts, focus and 44px controls. Do not apply colors/spacing independently per screen.

Automated responsive/theme, interaction, contrast and financial-workflow checks are recorded in VERIFICATION.md. Current no-computer-use/manual-inspection policy is unchanged.


2026-10-03 neutral surfaces/dark refinement: owner preferred neutral backgrounds with a dash of primary color, liked the Light layout, and requested improving Dark. Keep existing layout, sizing, card shapes and financial status colors. Light page is #f5f5f6 with white surfaces; Dark page #15171c, sidebar/topbar #191c22, cards #1e2128, inputs #242830, soft surfaces #272b33 and hover #333842. Primary teal is #157565 in Light / #61c6b3 in Dark. General hover, separators, secondary buttons and input borders are neutral rather than mint/green; selected navigation/tabs, primary actions and focus may use teal. Dark foreground/secondary text use cool off-white/grey. Shared chrome/input tokens explicitly maintain surface hierarchy. Error borders must retain their red state on hover. No unrelated Light layout changes.


2026-10-03 sign-in feedback: incorrect-credential/request errors use the shared overlay toast with automatic expiry disabled. Keep the message visible while editing credentials, until explicit dismissal, another submission or successful sign-in. Other shared notifications retain normal timed expiry and hover/focus pause. As of 2026-10-04, all sign-in feedback, including session expiry, uses one App-owned persistent toast; a new valid submission clears the previous message and duplicate in-flight submissions are ignored.


2026-10-03 navbar appearance control: signed-in desktop/mobile topbars include a shared quiet 44px icon button switching the resolved appearance between Light and Dark. Sun/Moon icons and accessible action labels indicate the next theme. It uses the same App state/local preference as Settings → Appearance, where Follow device remains available and responds to device changes. Future UI changes use the local finance-tracker-ui skill and latest accepted conventions as their baseline.


## Cohesive activity workflow (2026-10-03)

Transactions is one workspace with All transactions, Needs review and Import activity tabs. Counts distinguish ledger entries pending review from account imports awaiting confirmation. Review starts with all dates/accounts; continuing an import scopes the list to its new ledger entries and clears unrelated filters. Financial approval remains explicit. Fetching alone never creates ledger entries.

Imports use a connection-aware fetch area and a collapsed export uploader. Fetched previews use compact expandable account summaries, with the first account open initially. Ready previews can be added together in explicit batches of up to 100; duplicate decisions, rejected-row acknowledgement and stale-preview protections remain per import. The selected ready imports are committed sequentially; on failure, successful imports remain durable and outstanding ones stay available in activity. A commit never approves a transaction. Every live activity is grouped by original run, with account name/ending number/status prominent, local Johannesburg dates, coverage warning, continue/review/view actions and retained provenance. Activity paging keeps each run together; expanded account rows have their own bounded pagination under current permissions.

Accounts uses one full-width compact list: identity/sharing/access, dated bank balance and account action menu. Mobile puts the balance below the name. Discovery-file import and private-by-default manual creation are on Accounts. Settings → Accounts is a shortcut to the shared page; Banking remains credentials, schedule and diagnostics, with Discover accounts and refresh balances plus direct Accounts/Transactions actions. Schedules remain accounts/balances only.

Categories opens the Categories section. Desktop uses a compact left section menu for Categories, Spending groups and Automatic rules; mobile uses a scrollable strip. Shared tab semantics/focus remain, including arrow/Home/End keys. Settings and transaction tabs also scroll instead of wrapping. Main navigation resets scroll; same-page refreshes preserve it. Empty lists omit pagination, single-page lists show counts without disabled navigation. Shared field feedback space remains intact.


## UI audit follow-up (2026-10-03)

Context filters each own a wrapper containing their selector, loading state and search disclosure. Compact theme padding/gaps do not remove shared field feedback space. Budget period options use the short name; selected dates are an accessible hint beneath the control. Narrow workspace-name actions stack at full width while Settings tabs retain their scrollable strip.

Empty transaction and budget lists retain their ordinary shared panel boundaries. Rules must explain unmet account/category prerequisites and expose permission-appropriate next steps beside the unavailable action. Dashboard next steps prioritize account setup for users with no accessible accounts, import confirmation, and actual pending review; do not offer zero-count review as the initial step. Empty balance scopes have scope/refresh guidance instead of claiming existing accounts are absent. Stretched spending cards keep pagination/helper copy in one bottom footer, without fixed heights or stretched mobile content.


2026-10-03 disclosure border refinement: shared `.details` containers have no separate top divider; their summary card owns the border. Retain the shared 16px top margin between content/disclosures. This prevents an obsolete divider touching the card outline in import activity, rules, lookup searches and other shared disclosures. Banking groups already follow the same single-border convention.


2026-10-04 owner workflow/copy correction: clean bank downloads and uploaded statements are imported automatically with rule suggestions and mandatory pending review. Import activity records history and exceptions; possible duplicates, invalid rows, stale previews and rule-version changes still require explicit resolution. This supersedes the earlier separate confirmation requirement for clean imports. Review checks categories/spending groups before approval; importing never approves transactions. Existing staged imports are processed once per visit to Import activity using the same server commit validations. Plain language uses Get transactions, Transactions from FNB, categories and spending groups. Shared empty states separate actions from descriptions, single-page pagination is hidden, and native dialog dimming is neutral in both themes.


2026-10-04 account icons: Accounts uses accessible muted 20px type icons in its existing identity row: credit card, savings piggy bank, home-loan house and transactional bank. A wallet is the fallback for unknown/unavailable types. Use verified imported type metadata, not nickname guesses. The shared AccountIcon preserves current theme colors and desktop/mobile alignment; names, amounts and actions remain unchanged.


2026-10-04 transaction editor: primary Save and approve saves edits and explicitly approves them in one action; secondary Save changes retains pending review. Missing categories produce inline accessible picker errors and focus the first missing category. The proposed automatic rule shows editable description text before opt-in, conservatively removes long numeric reference tokens, and explains account scope and future review. It is unavailable for transfers, splits and zero amounts. Shared Modal uses explicit compact/medium/wide widths (480/640/900px), with stable viewport-bounded height for transaction editors and lookup selectors. All dialogs portal to document.body so selector descendants cannot resize their parent. Headers stay fixed, bodies scroll with a stable gutter; mobile dialogs remain full-screen. Picker dismissal restores trigger focus and the captured editor scroll position after native dialog cleanup; selecting/creating a category must not jump to the newly revealed rule proposal.


2026-10-04 shared transaction filters: all three Transactions tabs expose category (All/Uncategorized/specific), spending group (All/Not assigned/specific), money direction (both/in/out) and description search in a shared panel. Category/group lookup options retain server search/paging and exact selection hydration, with special filter options bypassing ID hydration. Selections persist between tabs; Clear filters resets category/group/direction/search and account/period/needs-period/import scopes. Accounts is also available on Import activity and to account viewers without budget membership. Budget period remains a ledger-only member control. Three classification tracks stack on narrow screens. Tab work counts stay global; list pagination/counts are filtered. Split matches appear once; explicit transfers are excluded from Uncategorized. Import activity's helper explains that activities contain matching transactions, while original details and exception decisions show every row. View/Review transaction actions carry classification/search filters into the import-scoped ledger.


2026-10-04 transaction rule offer now explains that saving an opted-in rule also categorizes matching unsplit uncategorized pending transactions in the same account, filling missing spending groups and preserving existing ones. These targets remain pending even when Save and approve approves the edited transaction. Existing categories/splits/transfers/approved rows remain unchanged; conflicting rule outputs require manual classification. Shared success toasts include the count of other matches and conflict skips when applicable. The local ledger approval selection is cleared after successful transaction editing so changed details are not selected using stale versions; paging/filter positions remain. Standalone Automatic rules management retains its existing future-import behavior.


2026-10-04 owner acceptance/seen policy: supersedes mandatory approval and save-only pending behavior. Complete categorized allocations automatically accept ledger entries on import, edit and transaction-created rule application; explicit transfers retain their category exemption. Missing categories (including incomplete splits and rule conflicts) stay in Needs review. Saving an edit marks its current version seen for the editing user. Imports/rule-applied targets begin unseen. Seen/unseen is personal per user and transaction version, independent of acceptance/reporting, with authorized atomic 1–100 selection actions and ledger seen filters. Later transaction changes invalidate old seen markers; opening a dialog alone does not mark seen. Existing explicit approved reviewers retain seen markers on migration; existing categorized pending rows are accepted without claiming human review, and acceptance changes are audited. Migration 10 is once-only and preserves money, allocations, groups, private account access and financial source data. Private accounts remain outside shared budgeting. UI uses Accepted/Needs category and Mark seen/unseen rather than manual approval; Save changes is the single editor save action.

Import handoff opens All transactions when the imported entries are fully categorized, so accepted unseen entries are immediately visible. Mixed/uncategorized imports open Needs review. Seen filters apply to ledger tabs; Import activity continues filtering its financial classification only. Opening an editor keeps the seen marker unchanged; explicit Mark seen/unseen is available to viewers as a personal action, while financial saving still requires editor access.


2026-10-04 category picker spacing: compact selector lists retain rounded-border clipping while the dialog body owns scrolling. The New category type field has a 24px gap above it. Row keyboard focus uses an inset outline offset so list clipping preserves the focus indicator. Stable picker/editor dimensions, existing selection/create flows and shared theme tokens remain in use.

2026-10-04 MCP settings: all signed-in users have a Settings → MCP tab. Use shared panels, Field/Form validation, toasts and aligned form-grid tracks. Display the configured endpoint, copyable agent instructions and generic HTTP configuration; tokens are masked, shown only at creation, cleared on tab navigation and individually revocable. Read-only is default; enabling write proposals explains exact browser approval. Pending proposals show operation/token identity/expiry and inspectable before/after transaction values (integer cents); approval/rejection uses explicit buttons. Code/previews wrap within their panels and scroll at bounded height, with natural mobile stacks. Financial descriptions retain a clearly stated privacy limitation; never claim arbitrary personal free text is fully anonymized.


## Browser-authorized MCP connections (2026-10-04)

Owner authorized replacing manual MCP token setup with browser OAuth. Settings → MCP now shows the endpoint/copyable agent instructions, own connected agents (read-only or proposal-enabled, last use, expiry, revoke), and exact change proposals. Connection approval requires tracker sign-in, confirms the approving username and unverified client/callback origin, and defaults to read-only. The optional proposal checkbox grants only requested scope; exact financial changes still need separate proposal approval. Consent uses shared Form/Button controls, neutral login panels and persistent overlay feedback; no manual token form or secret display remains. Migration 12 adds OAuth metadata/registration, S256 PKCE authorization, one-use codes and endpoint-bound expiring opaque access/rotating refresh credentials; refresh reuse revokes the connection. Browser session binding, CSRF, current enabled-user/account permissions and proposal/version checks enforce isolation. Legacy manual credentials remain revocable until expiry, without a new generator. Password change/reset, disable and restore invalidate agent access. See MCP.md for the full endpoint/lifecycle contract and VERIFICATION.md for actual coverage. Pasting a URL requires a client with remote MCP/OAuth support and a reachable HTTPS public URL outside localhost; no universal chat installation or live external-client compatibility is claimed.

2026-10-05 Network: keep Save network settings and Restart tracker as separate actions. Restart is disabled with unsaved drafts, during requests or when unsupported; show a persistent restarting state with a link to the configured URL. Explain brief disconnection and retain inline URL/proxy validation and overlay request feedback.


2026-10-05 MCP setup simplification: owner requested sharing only `/api/mcp`, with instructions available to the receiving agent. Removed the Settings setup disclosure/copy-prompt button; the shared endpoint field/copy button, connected agents and change proposals remain. Ordinary web GET/HEAD returns public plain-text setup and OAuth discovery instructions. MCP/SSE/JSON/bearer protocol requests retain authentication, browser connection consent, current user/account isolation and separate exact financial change approval. The link contains no private records and does not grant access or promise installation in clients without remote MCP/OAuth support. See MCP.md and VERIFICATION.md for negotiation and actual checks.


2026-10-05 owner-selected review defaults: Needs review initially selects Acceptance → Needs category and Seen by you → Unseen. Both controls remain visible and editable; selecting Accepted or all acceptance states can show other unseen transactions in this tab. Do not add an invisible mandatory pending filter. All transactions retains broad seen/acceptance defaults. Preserve separate seen/acceptance drafts for the two ledger tabs while sharing category/group/direction/search filters; Clear filters removes the current restrictions and scopes. Import activity hides and ignores seen/acceptance. These defaults change presentation only; acceptance, personal-seen and authorization semantics remain independent.


2026-10-05 Transactions filter cleanup: all three tabs share one Filters panel, including account scope and the ledger-only budget period selector. The header contains Clear filters. Desktop ledger controls use four equal tracks; Import activity uses three, with description search spanning two. Intermediate widths use two tracks and narrow mobile uses a natural single-column stack. Label/input spacing is consistent across shared Field and PagedSelect controls, with lookup disclosures and period hints contained beneath their own input. Import activity continues to omit period/acceptance/seen controls. Review defaults, per-tab seen/acceptance drafts and common classification/search persistence stay as accepted.

2026-10-05 owner clarified unassigned-period filter: remove the standalone Needs a period checkbox. Budget period includes “Not assigned to a budget period”; choosing it clears a named period and shows authorized household entries with no assignment, excluding explicit outside-budget entries. Choosing All periods or a named period disables the unassigned filter. Show a short scope explanation only when this option is selected; dashboard assignment shortcuts use the same visible selector state.

## MCP capability consent (2026-10-05)

OAuthConsent and MCPSettings share MCPPermissionFields/PermissionSummary with Review only, Categorisation proposals, Finance editing proposals and custom capability controls. Explain assignments, recategorization, financial edits, category/rule operations, shared budgets and custom independent seen separately; exact proposal approval is still required. Selected accounts limit reads and proposals and are explicitly selected rather than implicitly granting all on empty selection. Shared Field/Form validation makes empty scope/conflicting constraints inline and accessible; drafts survive failures. Settings permission edits use the existing modal and require explicit confirmation of the displayed permissions, reset after any draft change. Read-scoped connections explain fresh agent authorization for proposal access. Connected agent summaries remain persistent workflow content; request feedback uses existing overlay toasts. Existing proposal previews show personal seen state when relevant. Preserve theme tokens and natural mobile stacks; no unrelated Settings redesign.

2026-10-05 MCP approval follow-up: add per-proposal pending checkboxes and a shared wrapping editor-actions toolbar for Select all pending, selected count, Approve selected and Approve all shown. All-shown captures the displayed pending IDs and excludes later arrivals. Existing exact previews/single approve/reject remain. Connected agent permissions and OAuth consent share opt-in automatic-approval controls for each granted change type, explain mixed-type requirements and continuing access/version checks, and show permission summaries and Automatically approved badges. Defaults remain manual. Changes to the permission draft reset explicit Settings confirmation; preset changes clear automatic options, and removing a capability clears its automatic option. Use existing panels/form/field/button/toast styling and accessible checkbox labels on mobile/desktop.


2026-10-05 connected-agent card cleanup: keep agent name and Edit permissions/Revoke in one wrapping header. Show three aligned summary tracks for access preset, account scope and manual/automatic approval; automatic approval remains visible even with details collapsed. Put full granted capabilities, account names and constraints behind a keyboard-accessible View permission details disclosure, collapsed initially. Last use/expiry remain a concise activity line, with an explicit Expired badge. Mobile uses natural single-column summary tracks; long names and account labels wrap. Consent and permission editor retain their complete summaries.


2026-10-05 Banking failure copy: generic refresh/startup/timeout failures must not imply invalid login details. Explain startup/timeouts specifically and direct owner retries to Debug mode and count-only Troubleshooting. Preserve connection/credential fields and existing error presentation.


## Dashboard group/category budgets (2026-10-05)

Owner clarified that limits belong to a period/group/category combination: Eating out + Day-to-day can have R1,000, while Eating out + Exceptions has R300. A group's budget is exactly the sum of its category limits; there is no separate group allowance. Dashboard uses plain, separated native disclosure rows, with aligned Budget, Spent and Left figures and a progress bar at group and child levels. Over-budget rows show the excess in semantic negative color and text. No-budget rows say No budget set; never invent a remaining allowance. Private/account scopes show actuals only. Groups with a budget and no spending remain visible, as do missing groups/categories. Category totals across all groups remains a collapsed aggregate comparison. Edit budgets reuses the selective budget builder for the displayed period, retaining drafts when changing groups. Budget cards show group budget totals, the selected period and View spending. Use shared group dots, theme surfaces, validation, pagination and modals. Mobile stacks comparisons below names; desktop aligns the three figures. Native disclosure indicators keep reserved space.


2026-10-05 budget builder refinement: show only groups/categories explicitly included in this period. Add group selects a spending group; each group has Add category, editable amounts, a live inherited total and removable entries. Add category supports existing expense categories and explicit creation, and offers This budget only (default) or This and upcoming budgets with the amount. The latter fills missing future entries without replacing existing amounts; only upcoming entries carry to newly created periods. Empty groups and explicit zero amounts are retained. Save budget atomically applies staged additions/edits/removals; cancellation leaves limits unchanged. New global category creation is its own explicit action, preserving budget drafts. Use shared validation and nested native modals; drafts survive paging and category creation.


2026-10-05 shared transaction editing entry points: owner requested that transaction lines throughout the program open the existing edit modal. TransactionAccess now hosts the existing TransactionEditor for all routes and resolves current records through the authorized `/transactions?id=` read rather than treating source/import/proposal snapshots as editable ledger data. Nested references keep the underlying editor draft mounted; closing the child returns to the parent. Keyboard-accessible transaction-link buttons use shared styles and the existing native dialog/focus/toast controls. Opening alone does not mark seen; save/acceptance/permissions/version behavior is unchanged.

Accounts, Categories, spending groups and budget periods offer View transactions with an explicit fresh scope, clearing unrelated ledger filters. Dashboard category/group/account references also link to scoped ledger lists. Individual ledger rows, budget-period affected-transaction previews, import-history ledger rows/known duplicate references, transfer counterparts/candidates and MCP proposal transaction references open the same current-record editor. A nested saved edit invalidates its parent budget-period preview while retaining the period draft. Raw unimported/rejected rows without a ledger identity remain source details; ambiguous row numbers are never treated as transaction IDs.

Committed import-detail reads attach transaction_id by immutable saved provenance row/import identity under current account access. This is response-only metadata, with no financial migration/source rewrite. Skipped source rows may link to one known saved candidate; multiple candidates remain separate comparisons. Imported source descriptions remain original, while the modal reads current edited details. Account authorization is enforced on both source-detail and current-ledger reads and all saves. No real financial data/credentials or live bank inspection, commit or production deployment.

Shared transaction editor focus: async-loading dialogs preserve the original opener separately, so replacing the loading dialog does not lose keyboard return focus. Closing nested editors returns to a connected opener within the current top dialog; dismissed pending requests cannot reopen an editor.


2026-10-05 classification navigation design correction: owner rejected repeated View transactions buttons shown in Categories/Spending groups. Category rows now act as full-width, keyboard-accessible transaction links, with a restrained chevron beside the type badge. Spending groups use one aligned row per grid cell: colored dot/name/chevron navigate to transactions; a separately labelled quiet pencil edits the group, preserving membership permissions. Removed repeated boxed navigation buttons. Account and budget-period names also open their scoped transactions without extra View transactions controls. The shared editor and transaction scope semantics are unchanged. This supersedes the previous explicit-button presentation convention.


2026-10-05 dashboard hierarchy: Spending by group occupies the full content width beneath period totals, with expandable category comparisons as the primary content. Next steps and Bank-reported balances follow in an aligned two-column support row on desktop and stack naturally on mobile, including empty states. Existing scoped transaction navigation remains available; inline transaction drill-down within each group/category is an accepted future direction and should reuse the shared TransactionAccess editor with exact period/account/group/category scope. No financial behavior changes.


2026-10-05 dashboard budget connections completed: selected period carries between Dashboard/Budgets, and Budgets keeps its selected card visible even outside the current list page. Dashboard Edit budgets opens the displayed period directly. Expanded category rows show all-groups spending and the combined budget (sum of independent group/category limits, not a new shared allowance). Native group/category transaction disclosures read bounded spending pages in-place, reuse the shared transaction editor, refresh figures after saves and retain open group/category disclosures. Aggregate category totals also support in-place transaction disclosure. Stable group/category IDs and exact period/account scope preserve lineage; missing classifications remain inspectable. No production deployment.


2026-10-05 transaction transfer control: owner removed the separate transfer checkbox. In the shared transaction editor, selecting the group named Transfer (case-insensitive) sets the existing explicit transfer flag on save; choosing another group or Not set clears it. Current Transfer-group entries gain the flag when saved. Existing explicit transfers retain their status until the owner changes the group; opening alone and group renames do not rewrite ledger records. Linked transfers require unlinking before selecting a non-transfer group, including inaccessible counterparts. Category amounts/splits remain exact; transfers remain category-exempt and excluded from income/spending. Picker selection carries the chosen row metadata across server pages. This supersedes the earlier editor requirement for a separate explicit transfer checkbox; import/rule/API classification is unchanged. No financial migration or production deployment.


## Core finance flows (2026-10-05)

Ledger filters keep one aligned panel, including dates, signed amount range and account-scoped merchant/tag choices. Date/amount/label filters apply to the ledger; staged import filters retain their existing classification semantics. Ledger rows show missing-category and personal seen state without repeating Accepted. The shared editor provides context navigation only from a ledger or dashboard spending scope. Draft navigation/close asks Save and continue, Discard changes or Stay here. Save and next follows the original position across pages and excludes processed rows. Transaction notes are separate from split notes; merchant/tag choices belong to the transaction account. Bulk edits require an explicit before/after preview and one atomic apply.

Dashboard sorts budgets before pagination and can show an active-period daily budget guide. The guide describes remaining budget, never an account balance. Period navigation preserves account scope. Budgets adds Trends, report export and explicit rebalancing between existing entries. Compare saved periods or whole periods starting in the selected year. Rebalance preview shows both limits before/after and preserves total budget. Existing budget entries expose their copy-to-new-period schedule so carry-forward can be stopped before archiving a category. Category rows retain transaction navigation and add a separate quiet edit action.

Account import health keeps balance date, successful transaction import/check, fetch/check time, schedule status and incomplete coverage separate. Banking and Import activity actions are explicit; viewing does not refresh bank data. New catalogue/rule forms and draft safeguards use shared Field/Form/Button/Modal and overlay toasts in both themes. No new independent palette or decorative visual system.


## Focused workspace hierarchy (2026-10-05)

Owner requested a full UI review and a cleaner modern layout after the core features landed. This supersedes the earlier requirement to show every filter at once. Transactions keeps Accounts, Budget period and Search visible; one Filters action opens the remaining classification/review/date/amount/merchant/tag fields. Visible active-filter summaries and counts keep collapsed restrictions discoverable, including Needs category/Unseen defaults. Collapse never clears drafts or changes the query. Clear filters remains explicit. Mobile account/period selectors share a row with full-width search. Import and export sit with ledger actions; personal Seen/Unseen uses quiet text, reserving badges for actionable financial state.

The transaction editor prioritizes its existing description, date/amount and category/group controls. Notes/merchant/tags, budget assignment and optional rule creation use native disclosures; existing metadata/manual assignment is initially expanded. Validation reveals collapsed invalid sections before focusing a field. Draft guards, exact-money/splits and save/seen semantics remain authoritative. Save and next uses the secondary button variant, keeping one primary Save changes action.

Budgets uses Budget periods and Trends & reports tabs; report settings do not compete with budget editing. Edit budgets is primary, View spending quiet, and Edit dates/Rebalance use shared ActionMenu. Accounts prioritizes balances and Get transactions/Refresh balances; setup/connection actions share a menu and detailed import health is a native disclosure. Account-health and report rows stack their supporting text clearly. Categories uses horizontal section tabs across desktop/mobile, preserving a full-width list. Merchant-rule rows use quiet action menus, and their editor groups direction/priority/active state under More options.

ActionMenu reuses the existing account menu surfaces, 44px trigger, native disclosure semantics, outside-pointer/focus dismissal and Escape focus restoration. General Settings shares two aligned grid columns on desktop and natural stacks on mobile. Dashboard period navigation shares its scope strip; sorting sits beside spending actions. Maintain shared themes/tokens, touch targets and visible keyboard focus. Review screenshots use synthetic data only; coverage is recorded in VERIFICATION.md.


2026-10-05 Dashboard refinement after direct Vault22 reference inspection: use one continuous period-total surface with budget remaining emphasized, restrained neutral figures and no decorative header scene. Show the date range once below the compact period/account navigation. Align period navigation and Edit budgets against actual controls, not labels. Groups expand into plain, full-width category rows without side bars or separate transaction disclosure cards. A category row opens the shared wide centered modal, with its group/period context, budget/spent/remaining comparison and scoped transaction list; transaction rows open the existing editor and return to the modal after save. On narrow screens use the existing full-width overlay behavior. Keep split allocation amounts distinct from parent transaction totals.

Needs review entry starts a fresh queue across accessible accounts/periods, clears unrelated search/classification/date/import filters and selects Needs category with Seen and unseen. Seen is not acceptance; opening or marking a transaction seen must not remove an uncategorized transaction from the default review queue. Filters remain editable within the queue. Re-entering the tab or Dashboard review action restores the counted default queue. This supersedes the historical unseen default and persisted review filters on re-entry.

2026-10-05 hierarchy refinement: group headers use a soft tinted surface (teal when expanded) and stronger type; categories are plain lightly inset rows without rails. The centered category modal separates a labelled Category summary surface from the Transactions heading/list below, with shared theme tokens in both themes.

Owner refinement: remove the standalone period/pending text row below the dropdown card. Show a selected date range only as the period selector hint inside that card; pending amounts remain in the Spending metric.

Owner clarified the rejected line as the separator beneath each spending-group dropdown card; remove the group border-bottom and separate groups by spacing. Category transaction rows retain internal horizontal separators.

### Merchant identities
Merchant naming rules default to All accounts, with optional account scope. Logos are optional local PNG/JPEG uploads with initials fallback; render them consistently beside merchant names in rules, transactions and category transaction modals. Preserve the original bank description beneath the merchant name. Logo removal is explicit; existing account rules are not silently converted to global rules.

2026-10-05 Transaction editor redesign: use a content-sized modal with transaction details and classification in two desktop columns, naturally stacked on mobile. A single allocation uses the transaction amount; allocation inputs/totals appear for splits or an inconsistent existing allocation. Optional notes are quiet disclosures, and transfer/source/history tools form a secondary group above a dedicated save footer. Personal seen action belongs with status/navigation. Preserve drafts, validation, scoped navigation and exact-money semantics.

2026-10-06 Hover consistency: unselected category/group/action rows use --interaction-hover with --text; selected states retain accent tokens. Category rows and dashboard category rows share this rule in both themes.

2026-10-06 Dashboard Income uses the same expandable group/category hierarchy and centered summary/transaction modal as spending, with Received totals rather than expense budget metrics. Include income allocations and positive uncategorized amounts; negative income reversals reduce received totals. Exclude explicit transfers and expense-category refunds. Share current account/period authorization and bounded paging.

MCP permission editor groups read/account scope, categorized change grants and automatic approval, with a final explicit consent control. Merchant read consent and merchant/catalogue/rule/assignment grants stay separate from category rules. Use responsive two-column groups with shared surfaces and natural mobile stacks.

2026-10-06 ActionMenu and list spacing consistency:
- ActionMenu standardizes on the Menu icon (hamburger trigger) and left-aligned menu action buttons (`justify-content: flex-start; text-align: left; width: 100%`) so actions read consistently across account, transaction, and rule tables.
- Merchant identities and rules stack name and secondary metadata vertically (`.merchant-identity > div:not(.transaction-description) { display: flex; flex-direction: column; gap: 2px }`) with explicit `display: block` on subtitle elements to prevent text run-together.
- Merchant rules display associated category and spending group dot in the subtitle.
- MCP permission modal reorders sections into: Permission level, Read access & Account scope, Custom allowed changes, Restrictions (account/operation constraints with clear inline error feedback), and Auto-approval.
- Merchant logo display: smart aspect-ratio fitting. Square logos (~1:1) fill the round avatar edge-to-edge (`object-fit: cover`) without inner padding or white backgrounds. Wide wordmark logos (e.g. Amazon, Makro, Checkers, aspect ratio >= 1.25 or <= 0.8) automatically use `object-fit: contain` with inset padding on a clean white badge background (`.merchant-avatar-contain`), ensuring full wordmark visibility, legibility, and no side clipping.


## Audit correction — 2026-10-07

Categories stay flat: create with name/type, edit name/archive, and present no category-owned spending group. Historical category metadata stays in storage without being promoted to current UI ownership. Transactions, classification rules, merchant defaults and chosen budget entries retain independent category/group selections.

Global search uses the shared Modal, accessible Loading, Button and theme roles. Its input is a labelled combobox with a named results list and a valid active descendant only for the current loaded results. Navigation keys apply to the input; Clear, Retry and Close keep ordinary button keyboard behavior. Failed requests show a shared toast and an explicit retry state, separate from empty results. Closing/resetting or changing a query discards old asynchronous results. Search controls and results keep 44px targets at mobile widths.

SpendingGroupEditor is shared by Categories and search and uses Modal/Form/Field/Button, first-blur/live inline validation, trimmed Unicode name bounds, focus management and toast request failures. Selected search rows use accent-soft; unselected hover uses interaction-hover. Search uses shared modal layout/scroll behavior and has no independent dialog palette or decorative separator.

Automated synthetic responsive and keyboard checks are the verification method under the current owner instruction. No manual/computer-use visual sign-off is implied.

Shared modal focus: Field marks an explicitly autofocus control; Modal focuses it after showModal opens the dialog. Direct controls such as search use the same data-autofocus marker. Captured dialog cleanup closes the native overlay and preserves focus restoration.

2026-10-07: Product branding is Sente on onboarding, sign-in, sidebar, OAuth consent and browser metadata. Configurable household names remain separate; signed-in titles use `<household> · Sente`. Existing theme/controls remain unchanged.


2026-10-07 user security: Users & access provides per-user Reset password/Delete user actions for other users. Reset opens a shared dialog with new password and dependent confirmation and explains session/agent revocation. Delete explains permanent access removal and retained financial history/attribution, and requires the exact saved username in a validated field. Cancel is non-destructive; closing during a request is blocked. Own password changes remain in Security with current/new/confirmation fields and explicit copy that the current session stays active while other sessions and agent connections are revoked. Errors use shared inline fields or overlay toasts. Deleted usernames stay reserved for historical attribution; deleting oneself requires a different administrator.
