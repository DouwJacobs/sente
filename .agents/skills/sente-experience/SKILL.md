---
name: sente-experience
description: Build or refine the Sente finance-tracker UI with reproducible empty states, responsive actions, motion and interaction feedback. Use only for this application and its shared React controls.
---

# Sente experience

Work in the user's chosen Sente checkout; the application is in Ubuntu WSL, not the Windows link folder. Read that checkout's docs/UI.md for existing financial and workflow semantics. Follow the current owner inspection policy in AGENTS.md; use synthetic automated checks when computer-use inspection is restricted. Preserve permission boundaries, drafts, exact monetary values, request locks and review defaults when improving presentation.

## Owning components

- `web/src/ui.tsx`: Button, Empty, Field, Form, Modal, Tabs, ActionMenu, Reveal and useExitPresence.
- `web/src/Choices.tsx`: searchable bounded ChoiceField selectors; retain the selected row before returning focus.
- `web/src/finance-theme.css`: active theme and interaction tokens, motion and empty-state presentation.
- `web/src/styles.css`: structural layout, responsive tracks and existing workflow geometry.
- Feature components own copy, allowed actions and callbacks. Extend these shared components; do not replace a financial workflow with a display-only mockup.

## Visual decisions

Use the existing Inter/system font, neutral surfaces and restrained teal accent. Money stays tabular, signed and untruncated. Read actual shared tokens instead of inventing screen palettes. Primary means the next useful action; secondary is an alternative; quiet means supporting navigation. Keep the current theme's financial status colors. Use rounded monochrome Lucide icons for empty-state visuals, not raster art or promotional decoration.

The shared Empty component is a labelled section: decorative icon → h3 → description → action row. Use its `kind` deliberately: `start` Inbox, `filtered` Search, `complete` CheckCheck, `error` CircleAlert, `accounts` Wallet, `budget` Target, `categories` Tags, `connection` Plug. An icon may be explicitly overridden; never infer state from title text. Complete uses income tokens, error uses danger tokens, all other icons use muted text on a soft surface. The icon tile is 56px with an 18px radius, icon 26px / stroke 1.6. Title is 16px / line-height 1.4 / weight 600; description is 13px / line-height 1.6 with a 440px maximum. Use 12px vertical gaps and 36px × 20px desktop padding. Do not make empty content look like an input or error unless it is a failure.

At 760px and below, use 28px × 12px padding and stack action buttons at full width. All actions remain at least 44px high. Let long copy wrap; contain search text and financial details within the viewport. Use natural height, not a page-filling blank illustration.

Pass direct Button children to Empty so actions live in the action row. Wrap explanatory prose in paragraphs. Reuse actual feature callbacks and existing access checks. A secondary action is optional: add it only when a distinct useful alternative exists. Do not invent an action just to satisfy a two-button layout.

## Exact state decisions and copy

| Context | Title / description | Primary action | Supporting action |
| --- | --- | --- | --- |
| Transactions, no restrictions | Your transactions start here / Import a bank statement to see your income and spending in one place. | Import transactions | None |
| Transactions, restricted | No transactions found / No transactions match this view. Clear filters to widen your search. | Clear filters | Import transactions |
| Default Needs review queue | You're all caught up / Every transaction in your review queue has a category. New transactions that need a category will appear here. | View all transactions | Import transactions |
| Restricted review | No transactions found / No transactions need a category in this view. Clear filters to check the full queue. | Clear filters | Import transactions |
| Staged imports | Your imports need attention / [count] import(s) need attention. Resolve possible duplicates or invalid data in Import activity. | Resolve import issues | Clear filters, when restricted |
| Dashboard spending | No spending yet / Spending for this period will appear here once transactions are imported and categorized. | Import transactions; Add an account if none exists | Set up a budget |
| Dashboard category totals | No category spending yet / Categorized expenses for the selected accounts and period will appear here. | Review categories | None |
| Dashboard failure | Dashboard unavailable / Actual request error | Retry dashboard | None |
| Accounts | No accessible accounts / Connect FNB to discover accounts, add one manually, or import a discovery file. | Connect FNB | Add account manually |
| Accounts without admin rights | No accessible accounts / Ask an administrator to grant account access. | None | None |
| Categories | Create your first categories / Give income and expenses a clear home, such as Salary, Groceries or Transport. | Add category for budget members | None |
| Spending groups | Organize spending your way / Use groups such as Essentials or Leisure to organize transactions without changing their categories. | Add spending group for budget members | None |
| Budget periods | No budget periods / Choose your dates, then set spending limits. Your first period can follow a calendar month or your payday. | New period | None |
| Period groups | Start building your budget / Add a spending group, then choose the categories and limits for this period. | Add group | None |
| Rules | No rules yet / Match a bank description to a category so future imports need less manual review. | Add rule; route to the missing account/category prerequisite otherwise | None |
| Merchant rules | Recognize your regular merchants / Turn bank descriptions into familiar merchant names. You can also set a default category for future imports. | Add merchant rule | None |
| Global search, untouched | Find something in your finances / Search categories, groups, transactions and rules. Try a merchant name or a bank description. | Focus remains in search | None |
| Global search, no matches | No search results / No results for "[query]". Try a shorter name or a different description. | Clear search; restore input focus | None |
| Global search failure | Search could not be completed / Your search is still here. Try again when your connection is available. | Retry search | None |
| Import activity, first use | No import activity yet / Completed imports will appear here, with their accounts and any issues that need attention. | None in history; Connect FNB or Get transactions lives in the connection panel | Upload a bank export disclosure lives above history |
| Notifications, unread empty | No notifications / You're up to date. New unread messages will appear here. | View all notifications | Manage preferences |
| Notifications, all empty | No notifications / Budget alerts and account updates will appear here when there is something to know. | None | Manage preferences |

Never label a failed or still-loading response as empty. Only claim a review queue is complete when `acceptance=needs_category` and the only nonempty restriction is that default acceptance filter. Account, period, import, seen, search or other restrictions invalidate that claim. Upload a bank export uses the existing native disclosure; Choose FNB export files explicitly opens the picker. Returning to all transactions clears review scope through the existing `viewTransactions({})` callback. Clear filters is explicit and preserves existing semantics.

## Density, hierarchy and action ownership

Use the shared compact workspace header on every page: 28px desktop / 24px phone title, modest gaps and accessible descriptions without repeated visible eyebrow text. Period navigation renders only after loading and when a previous/next period or useful current-period reset exists. Do not show a strip containing only disabled or ineffective controls.

Desktop transaction detail cells use 9px vertical padding and secondary descriptions a 3px top gap. Ordinary synthetic rows are 64–92px high; height remains natural for long names and warnings. Preserve complete signed figures, separate selection controls and 44px action targets.

Transactions keeps search and the account/period scope summary immediately visible. Its expanded card has a Filters heading and a 44px reset-arrow icon in the top-right, labelled/titled Clear filters and disabled when nothing can be cleared. Show the equivalent search-bar reset only while collapsed, so there is one visible reset action. Group common controls under Account & period, Classification, and Direction & review. Align scope/classification in two equal columns; status has three desktop columns and phone direction spans above two review columns. Use 18px desktop / 14px × 12px phone card padding, 12px grid gaps, 12px muted group headings and 18–20px section gaps. Keep active restriction summaries at the card's footer, outside field tracks. Use compact default option copy All accounts, All groups, All statuses and All states beneath their explicit labels so phone pairs remain readable; server access scope still applies.

Advanced filters groups Date range, Amount range and Merchant & tag. Each pair shares one row; desktop date/amount groups share two tracks and merchant/tag spans below. At 1000px and below the groups stack while pairs remain side by side. Place Use negative amounts for expenses and positive amounts for income. beneath the entire amount pair, referenced by both inputs; no hint belongs between minimum and maximum. Merchant/Tag uses one shared ChoiceField control that opens search, all bounded results, paging and All merchants/All tags reset in a dialog. Preserve account scope and account-name disambiguation. Do not add a separate Find more merchant options control. Search/paging must leave the background grid geometry unchanged. Escape from the picker closes only that picker; outer-filter Escape closes the card and restores its trigger.

The ledger toolbar places Import transactions inside Transaction actions before selection/export actions. Keep contextual import actions in empty states. Reuse the real import callback and menu keyboard/outside dismissal.

 Filters contains accounts, period, category, spending group, direction, acceptance and seen. Initially closed More filters contains dates, signed amount bounds, merchant and tag. Its label becomes More filters ([count] active) when values are present. Closing either disclosure preserves values and active restrictions; keep the outer filter count, expanded active summaries and explicit Clear filters. Import activity omits unsupported advanced filters. Keep date/amount validation and existing review/tab semantics.

The editor opens with a draft-aware transaction summary (description, date, account and complete signed amount; Check amount below while invalid), followed by Classification, then Transaction details. Keep this DOM order for keyboard and phone use; desktop uses two columns. Category has one field label; only splits have the Split categories subheading. Keep Add split available without a duplicate Category heading. Editor fields use 6px gaps and 12px bottom spacing with no reserved blank error slot; actual errors/hints still render. Keep draft guards, rule proposal, allocation checks, metadata, review navigation and submit validation.

General preferences uses one column bounded at 640px. Save default lives in a wrapping action row: intrinsic desktop width, full width at 760px and below, at least 44px high. Financial metadata stays aligned and wraps as needed.

Give a creation action one owner. For a settled empty category/group/period/rule/merchant list, hide the equivalent header action and use the empty state's authorized action. Retain header creation while loading, after failure and for populated lists. Preserve permission and prerequisite checks. Dashboard Edit budgets appears only when populated targets make it useful. Import history reports history; connection setup/fetch and export upload have their own panels. Do not repeat those actions in history. Show Get transactions when loading/error/connected, and promote Connect FNB when no connection exists.

Accounts groups Transaction import health and the administrator-only Import an account discovery file under one Account tools panel. Use 16px × 20px desktop / 12px × 16px phone padding, a 14px section heading and compact 44px disclosure rows. Account tools and More filters summaries use 12px left text padding, 36px reserved on the right and 10px vertical padding; the entire row owns hover/focus treatment so text never touches its edge. Expanded health has no nested panel border, shadow or padding. Keep hidden-account administration separate and preserve all callbacks/permissions.


## Recent feature layout conventions

Budget period cards keep the selected period’s spending groups visible by default. Omit the redundant Edit budgets action on Budgets (the dashboard navigation action remains useful). View spending belongs first in the period’s Actions for [name] menu, followed by Show spending groups only for collapsed periods, Edit dates and Rebalance. Keep other periods reachable without mounting every period’s groups at once. The period header uses minmax(0,1fr) / auto grid tracks, a 12px gap and top alignment so its 44px menu stays at the top right even with long names. At 760px and below, hide the Selected period icon/text together; retain the period name, dates and total. Preserve selected-period synchronization and group/category editing, menu dismissal and Reveal motion.


The transaction Classification card places its heading and sole 44px Add split action in a minmax(0,1fr) / auto header row with a 12px gap and 12px bottom spacing. Spending group is followed directly by Category; no action row interrupts those fields. Keep Split categories only when multiple allocations exist. At 700px and below, every category selector spans the card’s full content width. A single allocation uses one grid track without a reserved remove-button column; splits place amount and its 44px remove action together beneath the full-width category. Preserve signed amounts, allocation totals, 100-split cap, view-only/pending locks, notes, validation and draft behavior.


General preferences alone uses general-preferences-grid for its single bounded 640px track. Keep the shared general-settings-grid at two equal minmax(0,1fr) desktop columns with a 24px gap and align-items:stretch: Configuration source forms and PWA installation/identity cards share equal top/bottom edges at natural row height. At 760px and below the shared grid uses one full-width track and 16px gaps with natural card heights. Never apply the General preferences width restriction to all settings grids; preserve mounted drafts and access-specific cards.


The topbar search is an icon-only shared Button with a 44px target, transparent resting background and var(--text) icon color against var(--chrome). Preserve its visible button-content wrapper on every viewport; do not hide all descendant spans to remove an obsolete text label. Scope icon geometry through .button-content>svg. Verify the actual SVG is visible and its contrast against chrome is at least 3:1 in phone/desktop light/dark, plus search opening and focus restoration.

## Notification inbox

Keep the Notifications workspace heading and a compact Your notifications inbox heading (16px), with the unread/current-view message count beneath at 12px. Header actions are a 44px Refresh notifications icon and Notification actions menu containing Mark all as read and Notification preferences. Loading/failure copy is Checking your inbox… / Inbox unavailable; do not fabricate zero counts on failure. Preserve request locks and the existing privacy refresh policy.

Scope presentation rules to .notification-centre so browser-push device lists retain their own layout. Inbox padding is 16px desktop / 14px × 12px phone. Message cards use 8px gaps, 14px × 16px desktop / 12px phone padding, 8px radius and a quiet border. Unread cards use a soft surface and an accent dot beside explicit Unread text; read cards use the normal surface and explicit Read. Titles are 14px, messages 13px / 1.6 with an 80ch maximum, preserving full content and line breaks. Type, title-case severity and short browser-local timestamps use wrapping 11px metadata; timestamps retain ISO dateTime and full local-time titles. Warning/critical/error colors supplement severity text.

Each card keeps View budget/account/transaction as the explicit source action. Mark as read is a labelled/titled 44px CheckCheck icon; Dismiss belongs in the card's Notification actions for [title] menu only if dismissible. Place source left, secondary controls right in one wrapping row. Opening a notification/source never silently marks it read. Keep authorized source lookup, unavailable-target feedback, transaction opener restoration and explicit read/dismiss callbacks intact. Pagination uses 44px chevrons labelled Previous notifications / Next notifications with a centred Page [n] of [total] count. Empty/error states retain the existing contextual copy, allowed actions and retry routing.

Verify notifications, notification-layout and notification-delivery specs for these changes, including actual mutation/privacy workflows, push-only links, 44px controls, long messages, empty/failure states, focus and both themes at phone/intermediate/desktop widths. Keep browser-push permission explicit and preference drafts mounted.

## Interaction and motion

Keep loading buttons' resting dimensions and accessible names: Button wraps children in `.button-content`, sets aria-busy and disabled, hides content with opacity (not visibility), and centres an absolute spinner. Keep the caller's action label stable during loading; use loading/aria-busy instead of swapping in a longer phrase. Retain left-aligned action-menu content. Do not remove icon-only button labels. Pending requests must keep their existing duplicate-submission protection.

Color/border feedback uses 120ms ease. Dialogs and their backdrops fade in over 180ms; their geometry stays fixed from the first frame. Toasts and phone More enter in 180ms with cubic-bezier(.2,.8,.2,1), opacity and at most 6px downward offset. Action menus use the 180ms entrance fallback only when native disclosure transitions are unsupported. Animate height only for user-controlled expandable sections; keep money, transaction rows and scope changes stable. Filter and Reveal wrappers use grid-template-rows 0fr/1fr over 180ms, opacity over 120ms and immediate inert/aria-hidden on collapse. Reveal lazily mounts on first use and retains content afterwards. Native details uses interpolate-size and ::details-content block-size/opacity/content-visibility transitions behind @supports; unsupported browsers retain native immediate disclosure. Where native disclosure transitions are supported, action menus fade in/out over 120ms without changing their positioning, replacing the entrance fallback. Phone More uses retained exit presence for a 120ms opacity/6px exit, immediately inert/aria-hidden on close; reversal cancels pending removal.

Submission validation temporarily bypasses disclosure transitions, flushes the open geometry and focuses the invalid field immediately; ordinary toggling resumes animation. Dialog dismissal and focus restoration stay immediate, including dirty guards. Animate an actually unmounted dialog with a 120ms visual-only inert/aria-hidden exit snapshot and backdrop; preserve its fixed bounds and internal scroll positions, remove IDs and open, and remove the snapshot when animation finishes. A connected dialog (including Strict Mode rehearsal or a dirty editor retained beneath a guard) does not produce a snapshot. Never delay onClose, submissions or financial callbacks for animation. Under prefers-reduced-motion:reduce, remove these transitions and entry/exit animations, skip exit snapshots; stop spinner rotation while retaining its status label. Preserve static transforms used for disclosure arrows: closed/open chevrons retain their distinct angles under reduced motion; remove animation, not state geometry. Pressed buttons use a subtle inset overlay without shifting their position; disabled controls do not acquire active styling.

Preserve keyboard focus rings independently of hover. Dialog Escape restores the connected opener; nested draft guards remain authoritative. Keep native disclosure interaction and existing tab Arrow/Home/End semantics. Search empty/error content is not a listbox: expose the listbox only while actual options exist. Do not announce the entire empty section as an alert; retain a local status for asynchronously updated search messages and existing toasts for request failures.

## Verification

Use synthetic data and the repository's disposable `scripts/test-browser.py` runner. Run production build and unit tests, then experience, polish, theme-states, global-search, budget-group-interaction, transaction-filters, mobile-core and toasts browser specs for shared-control changes. For filter/motion work also run filter-layout-motion. For density refinements select affected specs from density-refinements, editor-layout, core-improvements, ui-hierarchy, workspace-controls, settings, onboarding, rules and budget-category-editor. Local feature-only changes use focused affected checks; the full shared-control set applies when shared primitives or tokens change. Behavior checks must exercise the affected real callbacks and relevant invariants: filter clearing/review completion for those flows, error recovery, focus restoration, reduced motion for motion changes, and 44px mobile actions. Check both themes at 360/390px, an intermediate width and desktop. Keep full signed amounts and long names readable.

Record actual passed checks and any limits in docs/VERIFICATION.md. Automated geometry does not constitute manual visual or physical-device verification. Keep docs/UI.md synchronized and update this skill when implemented decisions change. Presentation work does not expand MCP tools or financial access.

## Dashboard spending menu and Sankey page

Spending by group keeps Sort budgets visible and places Edit budgets and Sankey graph
inside one shared ActionMenu labelled Spending actions. Use quiet borderless menu items,
including hover; keep shared focus rings, neutral hover and 44px targets. Preserve the
existing Edit budgets scope/availability. The chart belongs on its dedicated Sankey graph
page, retaining account/period selectors and Back to dashboard without resetting scope.
The Spending breakdown disclosure keeps 12px left padding and 36px right space.
See docs/UI.md for node labels, proportional sizing and responsive/table behavior.
