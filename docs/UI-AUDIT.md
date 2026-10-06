# Visual UI review — 2026-10-03

Requested by the owner using computer use. Inspection used the Codex browser and two isolated synthetic services; no owner financial data, bank pages, credentials or preferences were inspected. This is a findings report, not a redesign or implementation of the recommendations.

## Coverage

Desktop (1440px): populated Dashboard, Transactions, Review, Accounts, Budgets, Imports, Categories/Rules and Settings/General in Dark; empty Dashboard, Transactions, Accounts, Categories/Rules, Imports and a period with no limits in Light. Initial tablet-sized populated Dashboard was also inspected.

Mobile (360px): populated Dashboard in Light, Transactions and Settings/General/Banking in Dark; empty Imports in Light and Review in both themes. Scrolled populated transaction rows, mobile More navigation, and Settings action controls were inspected. Screenshots were visually reviewed through computer use, supplemented by read-only viewport/geometry checks. One full-page capture produced a distorted capture; a fresh normal screenshot and DOM geometry confirmed the actual Budget layout was intact.

Not covered: connected Banking and live refresh; populated import previews/history; every Settings subtab; no-period Budgets; all pages in every theme/viewport; edit dialogs; long real account names; browser/device combinations beyond this browser. No bank refresh or imports were triggered. Localhost services on different ports share browser cookies; changing fixture sessions caused an expected fixture-only session expiry, not a reproduced product defect.

## Prioritized findings

1. **Reset scroll when changing main pages.** After scrolling the mobile transaction list, opening Settings through More retained the scroll offset and displayed the middle of General, with its heading/tabs off-screen. Navigate to the page top and consider heading focus for keyboard navigation; avoid resetting scroll during same-page data refreshes or draft editing.
2. **Compact filter spacing.** Desktop period/account filters have a large unused strip beneath the controls. On mobile, labels/inputs are separated by substantial unused vertical space; Transactions/Review results begin beneath the first screen. Tighten container/field margins rather than removing the reserved validation feedback line, which prevents mobile button taps being swallowed by first-blur layout shifts. Period option text is also clipped at 360px: show a shorter period label and put the date range in readable supporting text.
3. **Standardize empty list composition.** Accounts puts zero-item pagination before the empty message; Transactions/Rules/Imports place it after. Use consistent content order, panel boundaries and guidance. Omit disabled Previous/Next when there is no navigable page; retain a meaningful result count where useful. Hide selection/approval instructions when there are no rows. Distinguish first-use emptiness from a filter with no matches.
4. **Make empty guidance actionable and prerequisite-aware.** Empty Accounts tells an administrator to ask an administrator; Rules tells users to add a rule while Add rule is disabled with no accounts/categories; Imports offers Fetch despite no configured connection. Give the current user a clear next step using existing permission-aware navigation (Manage accounts, add category, set up Banking), and explain prerequisites beside unavailable actions. Do not relax server permissions or trigger financial/import work automatically.
5. **Reduce the height of mobile Settings navigation/actions.** Seven tabs wrap over three rows at 360px, pushing the first card down. A compact horizontally scrollable tab strip with a visible selected state could preserve the existing tab semantics. Save workspace name wraps to three lines next to Reload saved name; stack full-width actions at narrow widths or use concise labels so each retains a comfortable height.
6. **Use stretched dashboard cards more deliberately.** Summary cards and account grid tracks align well. The desktop spending card stretches to match the combined right column but leaves a large blank area below its footer. Keep row/bottom alignment; anchor pagination/helper content to the bottom and let the main content occupy the remaining track, rather than giving cards unrelated fixed heights.
7. **Small copy and hierarchy cleanup.** “1 transactions” and “1 items” need singular forms. On a new workspace, prioritize Add account/set up imports over Review transactions with a zero count. Keep generous page spacing, but separate meaningful whitespace from unused control padding.

## What already works

Neutral surfaces with restrained teal and financial colors read consistently. Populated account cards share their widths, row edges, amount baselines and action positions. Dashboard metric cards align. Long mobile transaction descriptions wrap without visibly clipping amounts; negative and positive values remain signed and color-coded. Navbar appearance/sign-out controls remain usable at 360px. Light empty panels and dark inputs have clear surface separation.

## Recommended next pass

Start with scroll restoration, compact filter layout, and one consistent empty-state/pagination treatment. Then refine mobile Settings and dashboard footer placement. Re-check empty/populated states in both themes and desktop/mobile with synthetic data. Extend shared components/tokens under the finance-tracker-ui baseline; update UI.md only for implemented/accepted conventions.


## Resolution after cohesive redesign — 2026-10-03

Owner requested implementing findings still relevant after the screen consolidation. The original report and visual coverage above are retained as historical evidence. Follow-up validation uses synthetic automated browser checks under the current no-computer-use instruction; it is not a new visual review.

| Finding | Current resolution |
| --- | --- |
| 1. Main-page scroll | Resolved in the preceding redesign: main navigation scrolls to the top, while refreshes/drafts retain scroll. Native navigation/tab focus remains; no automatic heading focus was added. |
| 2. Filter whitespace/dates | Completed in this follow-up: wrapped selectors keep loading/disclosures within their own tracks; the active theme uses 12px padding and compact gaps. Short period options retain an accessible date-range hint beneath the selected control. Shared validation feedback space remains. |
| 3. Empty lists/paging | Previous redesign removed zero-item paging and single-page disabled navigation, and hid empty selection instructions. This follow-up puts empty Transactions and Budgets inside their normal shared panels. |
| 4. Prerequisite guidance | Accounts and Imports were handled by the redesign. Rules now explains missing editor accounts or categories, provides Open Accounts / Create category where permitted, and disables Add rule until prerequisites exist. Dashboard distinguishes absent accounts from an empty balance scope and provides an Accounts action. |
| 5. Mobile Settings | Previous redesign made tabs scrollable. Workspace-name save/reload actions now stack at full width on narrow screens. |
| 6. Dashboard footer | Category pagination and uncategorized guidance share a bottom-anchored footer inside the stretched spending card; mobile cards retain natural height. |
| 7. Copy/first-use hierarchy | Previous redesign fixed singular pagination/transaction counts. Dashboard now prioritizes account setup, then imports awaiting confirmation, then pending review; no zero-count review action is shown. Removed the redundant first-use notice. |

Connected live banking and fresh visual inspection remain outside this follow-up's verification; financial/import permissions and approval semantics are unchanged. See VERIFICATION.md for the actual test outcomes.


2026-10-05 cleanup implemented: compact default transaction search with optional filters; secondary actions consolidated in shared menus; horizontal classification and budget/report tabs; account health disclosed on demand; quieter editor optional sections; aligned Settings tracks. Dashboard now has a unified summary surface, one date range, actual-control toolbar alignment and plain group/category rows. Category detail is a centered summary/transaction modal, with no side lines or nested transaction cards. Needs review restores the counted category queue including seen entries. See VERIFICATION.md for direct computer-use and synthetic checks and their limits.
