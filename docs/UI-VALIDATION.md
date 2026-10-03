# UI validation — 2 October 2026

## Result

The interface is visually consistent across the eight main pages. Its restrained green accent, shared surfaces, amount formatting, status labels, and navigation form a coherent visual system. Six layout issues were found and corrected, then inspected again in the running browser.

## Coverage

Computer use and computer vision were used on the actual running application through the Codex in-app browser. The user's local development instance was inspected read-only. Populated workflows used an isolated temporary database and synthetic accounts, users, categories, transactions, and CSV rows; the user's data and appearance preference were preserved. Temporary viewport overrides were reset afterwards.

| Screen | Desktop 1280×900 | Mobile 360×800 | Themes |
| --- | --- | --- | --- |
| Dashboard | Inspected | Inspected | Light and dark |
| Transactions | Inspected | Inspected | Light and dark |
| Review | Inspected | Inspected | Light and dark |
| Imports | Inspected | Inspected | Light and dark |
| Accounts | Inspected | Inspected | Light and dark |
| Budgets | Inspected | Inspected | Light and dark |
| Categories | Inspected | Inspected | Light and dark |
| Settings | Inspected | Inspected | Light and dark |

Additional dashboard checks at 1024px and 768px inspected the desktop/sidebar breakpoint. The 768px correction was rechecked in both themes.

Inspected states included populated and empty dashboards/review/import history, long transaction descriptions, pending and approved badges, mobile More navigation, account/category/import-rule forms, budget dates and reassignment preview, category limits, add/edit user dialogs, account-access settings, backup status, transaction allocations, inline errors, keyboard focus, Escape dismissal, transfer controls, normalized provenance, and audit history. First-run onboarding was inspected at desktop and 360px in light theme using a separate empty database; no administrator was created.

## Corrections

1. Adjacent date and amount fields now align at their top edges even when one has helper text. Direct action buttons in form grids align with the input rather than its label.
2. Split category and amount controls remain aligned when an inline error appears; mobile amounts have more room and remove controls align with the inputs.
3. Dialog action buttons stretch to a consistent height when labels wrap.
4. Long dialog titles can shrink/wrap and cannot push the close button out of view.
5. Open dialogs lock background page scrolling while retaining dialog scrolling and a stable scrollbar gutter.
6. Mobile user rows stack their names and role controls cleanly; tablet dashboard summaries use two columns and stacked filters below 1000px, preventing decimals from wrapping mid-amount.

## Workflow verification

A synthetic CSV with an existing-transaction candidate, a valid new row, and an invalid amount was uploaded using the browser file chooser. The comparison was inspected, the candidate explicitly skipped, and valid-row import explicitly confirmed. Result: one imported transaction and two skipped rows, with the rejected row retained in history.

A pending transaction was split into two valid signed allocations, saved, and individually approved. Remaining pending entries were inspected and approved through the explicit bulk action; the empty review state appeared. Invalid same-sign allocations displayed field errors beneath the amount input before saving. Account submission with missing fields revealed inline errors and focused the first input. First-blur username validation and keyboard Escape dismissal were also exercised.

Read-only DOM measurements supplemented visual inspection: the mobile category-limits dialog and page had no horizontal overflow (345px content within a 360px viewport); the tablet page stayed within its viewport. Production TypeScript/Vite build passed after the final corrections. Three existing frontend money tests passed. Prior automated browser results remain separate from this computer-vision audit.

## Limits

This audit covers the inspected states in the desktop in-app Chromium browser with resized viewports. It does not establish native Safari/iOS keyboard or touch behavior, screen-reader usability, formal contrast conformance, every permission-role view, every server failure/concurrent-edit state, or the full OFX/ZIP import matrix. Onboarding dark theme and administrator creation were not exercised in this pass. Actual transfer linking was not performed; its controls and layout were inspected.

## Evidence

Representative screenshots were saved as `ui-audit-desktop-light.jpg`, `ui-audit-split-mobile.jpg`, `ui-audit-limits-mobile.jpg`, and `ui-audit-tablet-dark.jpg` in the chat's outputs directory. All screenshot financial data is synthetic.

AGENTS.md, docs/UI.md, and the project UI skill now require computer-use/computer-vision review after UI changes, covering desktop, 360px mobile, both themes, relevant interactive states, and an honest record of remaining limits.
