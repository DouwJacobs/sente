---
name: sente-experience
description: Build or refine the Sente finance-tracker UI with reproducible empty states, responsive actions, motion and interaction feedback. Use only for this application and its shared React controls.
---

# Sente experience

Work in the user's chosen Sente checkout; the application is in Ubuntu WSL, not the Windows link folder. Read that checkout's docs/UI.md for existing financial and workflow semantics. Preserve permission boundaries, drafts, exact monetary values, request locks and review defaults when improving presentation.

## Owning components

- `web/src/ui.tsx`: Button, Empty, Field, Form, Modal, Tabs and ActionMenu.
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
| Spending groups | Organize spending your way / Use groups such as Essentials or Leisure to organize transactions without changing their categories. | Create spending group for budget members | None |
| Budget periods | No budget periods / Choose your dates, then set spending limits. Your first period can follow a calendar month or your payday. | Create first period | None |
| Period groups | Start building your budget / Add a spending group, then choose the categories and limits for this period. | Add first group | None |
| Rules | No rules yet / Match a bank description to a category so future imports need less manual review. | Create first rule; route to the missing account/category prerequisite otherwise | None |
| Merchant rules | Recognize your regular merchants / Turn bank descriptions into familiar merchant names. You can also set a default category for future imports. | Create merchant rule | None |
| Global search, untouched | Find something in your finances / Search categories, groups, transactions and rules. Try a merchant name or a bank description. | Focus remains in search | None |
| Global search, no matches | No search results / No results for "[query]". Try a shorter name or a different description. | Clear search; restore input focus | None |
| Global search failure | Search could not be completed / Your search is still here. Try again when your connection is available. | Retry search | None |
| Import activity, first use | No import activity yet / Get transactions or upload an export to begin. | Upload a statement when editor accounts exist | Manage bank connection for administrators |
| Notifications, unread empty | No notifications / You're up to date. New unread messages will appear here. | View all notifications | Manage preferences |
| Notifications, all empty | No notifications / Budget alerts and account updates will appear here when there is something to know. | None | Manage preferences |

Never label a failed or still-loading response as empty. Only claim a review queue is complete when `acceptance=needs_category` and the only nonempty restriction is that default acceptance filter. Account, period, import, seen, search or other restrictions invalidate that claim. Upload a statement expands the existing upload disclosure and focuses Choose FNB export files; it never opens a native file picker automatically. Returning to all transactions clears review scope through the existing `viewTransactions({})` callback. Clear filters is explicit and preserves existing semantics.

## Interaction and motion

Keep loading buttons' resting dimensions and accessible names: Button wraps children in `.button-content`, sets aria-busy and disabled, hides content with opacity (not visibility), and centres an absolute spinner. Keep the caller's action label stable during loading; use loading/aria-busy instead of swapping in a longer phrase. Retain left-aligned action-menu content. Do not remove icon-only button labels. Pending requests must keep their existing duplicate-submission protection.

Color/border feedback uses 120ms ease. Dialogs and their backdrops fade in over 180ms; their geometry stays fixed from the first frame. Menus and toasts enter in 180ms with cubic-bezier(.2,.8,.2,1), opacity and at most 6px downward offset. Do not animate page heights, money, table row positions or scope changes. Closing stays immediate so Escape and draft guards remain predictable. Under prefers-reduced-motion:reduce, remove these transitions and entry animations; stop spinner rotation while retaining its status label. Pressed buttons use a subtle inset overlay without shifting their position; disabled controls do not acquire active styling.

Preserve keyboard focus rings independently of hover. Dialog Escape restores the connected opener; nested draft guards remain authoritative. Keep native disclosure interaction and existing tab Arrow/Home/End semantics. Search empty/error content is not a listbox: expose the listbox only while actual options exist. Do not announce the entire empty section as an alert; retain a local status for asynchronously updated search messages and existing toasts for request failures.

## Verification

Use synthetic data and the repository's disposable `scripts/test-browser.py` runner. Run production build and unit tests, then experience, polish, theme-states, global-search, budget-group-interaction, transaction-filters, mobile-core and toasts browser specs for shared-control changes. New behavior checks must exercise real callbacks, filter clearing, review completion, error recovery, focus restoration, reduced motion and 44px mobile actions. Check both themes at 360/390px, an intermediate width and desktop. Keep full signed amounts and long names readable.

Record actual passed checks and any limits in docs/VERIFICATION.md. Automated geometry does not constitute manual visual or physical-device verification. Keep docs/UI.md synchronized and update this skill when implemented decisions change. Presentation work does not expand MCP tools or financial access.
