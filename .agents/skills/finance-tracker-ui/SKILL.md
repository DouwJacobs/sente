---
name: finance-tracker-ui
description: Create, change, or review the finance-tracker interface using its maintained UI guide, responsive workflows, money display, and transaction review conventions. Applies only to this project.
---

# Sente UI

Read the repository's docs/UI.md before UI work. Read relevant requirements in docs/PLAN.md for import, review, splitting, periods, and permissions.

Use this skill and the current conventions in docs/UI.md as the baseline for every Sente UI change. Historical design trials are archived and do not set current requirements. Preserve the existing design unless the owner requests a specific change. Once shared tokens and components exist, extend them instead of introducing independent screen styles. Keep proposed choices distinct from accepted conventions.

Start from the existing shared components and tokens: neutral page/chrome/card/input surfaces in both themes, restrained teal primary/selected/focus accents, and semantic financial colors. Keep theme changes synchronized through App state and the saved finance-theme preference; retain the navbar theme toggle and Settings Light/Dark/Follow device choices. Extend shared roles rather than introducing page-specific palettes or parallel control styles.

Preserve complete desktop and mobile workflows, explicit review state, account/period scope, exact split totals, and accessible controls. Avoid the decorative AI design conventions named in the guide.

When the user changes the design direction or a new shared convention is accepted, update docs/UI.md with the implementation. Keep one maintained source rather than duplicating detailed rules here.

Use focused builds, tests and structural checks for routine changes. Use short, targeted computer-use visual checks when requested or when layout/interaction risk warrants them. Record actual coverage; never claim visual verification based only on assertions.

Verify changed workflows at narrow mobile and desktop widths, including realistic long descriptions, pending states, validation errors, and keyboard use. Use synthetic data.

Use shared overlay toasts for request failures without shifting forms or dialogs. Sign-in errors stay visible until dismissal or retry; other notifications retain timed expiry with hover/focus pause.

Follow the form validation convention in docs/UI.md: shared fields and forms, first-blur validation followed by live validation, inline accessible field errors, and dependent-field checks.

Use web/src/styles.css for shared structural rules, web/src/finance-theme.css for the active shared visual tokens, and web/src/ui.tsx for shared controls. Run the production build and browser workflow checks for changes to responsive UI behavior.

Keep card edges and grid rows aligned as described in docs/UI.md. Check both empty and populated layouts within the current inspection policy; avoid independent card width limits and doubled grid gaps.

Follow the current inspection policy in AGENTS.md. Explicit owner requests authorize scoped computer-use layout checks with synthetic fixtures. Record the inspected views separately from automated geometry coverage; browser simulation does not verify real-device keyboards or safe areas.

Use the shared --interaction-hover background and --text foreground for unselected clickable rows, including category/group rows. Reserve --accent-soft/--accent for selected states. Never introduce a page-specific hover palette; keep light/dark hover and focus behavior consistent with shared tokens.

## Mobile core conventions

Use [the mobile core standards in the UI guide](../../../docs/UI.md#mobile-core-workflows) for navigation, transaction lists and editing. Extend the owning feature and shared controls rather than adding independent phone implementations.

- Keep Dashboard, Transactions, Review and More as the primary phone navigation, with account/member visibility respected. Review starts the existing fresh Needs review queue; Accounts belongs in More. Reflect the active queue and secondary page accessibly.
- Measure bottom navigation height, including safe-area padding, and share that measurement with page clearance and More positioning. Make the final action scrollable above navigation. More dismisses on outside interaction and Escape; Escape restores trigger focus.
- Reserve readable header space for household branding and search/theme/sign-out actions. Keep long names available through full-text titles. Phone navigation and icon actions have at least 44px touch targets. Phone transaction selection uses long press and full-row taps, with keyboard/menu alternatives; desktop checkbox selection must not open an editor.
- Give phone transaction rows a complete signed amount alongside the merchant, followed by date/account and category. Use labelled check/question and eye/crossed-eye status icons inline with category text; preserve accessible status names and full merchant details. Centre a 36px merchant logo beside the row and overlay the selection checkmark. Long press starts selection; taps toggle until the last row is deselected. Keep mobile selected actions in the existing menu so selection does not shift the list. Retain text for transfer/private/split status. Secondary original merchant descriptions remain in the editor. Stack narrow scope controls instead of truncating their selected values unnecessarily.
- Stack editor fields and split category/amount controls on phones. Keep split removal separate and reachable, preserve shared first-blur/live validation, and leave save/close actions reachable in short viewports. Fit shared dialogs to the visual viewport, lock background scrolling, and preserve nested drafts and focus return.
- Verify 360/390/430px phones, short landscape, both sides of affected breakpoints and desktop. Include both themes, long names, large signed values, selection, validation and nested selectors. Use the existing disposable-fixture runner; run browser suites sequentially unless output directories are isolated. Failure screenshots/geometry checks complement, rather than replace, authorized visual review.

Keep money, acceptance/seen semantics, permission scopes and request locks intact. Assess and record MCP impact; presentation work does not justify expanding tools, fields or consent.

Apply the owner mobile refinements in UI.md: compact dashboard typography, balanced period/account columns, insets around all category/group figure tracks, quiet navbar search and compact transaction status icons. Preserve full signed amounts and readable exceptional warning text; inspect long values and both themes after layout changes.

Categories and Spending groups share the same grid and choice-row presentation, with type/archived information and separate editing preserved. Use owner-supplied Vault22 references for density and hierarchy within the existing Sente tokens.

Prefer shared StatusIcon for routine status/type markers instead of pill badges, following UI.md's compact status conventions. Preserve visible explanatory warnings and permission wording using icon-plus-text. Automatic/merchant rule rows use one shared actions menu. Keep logos square in dimensions and circular in shape, including their flex basis; avoid decorative arrows that imply unavailable navigation.

Apply [compact workspace controls](../../../docs/UI.md#compact-workspace-controls) on Dashboard and Transactions: unboxed balanced dashboard scope, compact headings/tabs, visible search and account/period summary, and detailed transaction scope behind one accessible Filters disclosure. Preserve values while collapsed and across tabs, review defaults, Escape focus return and 44px icon targets. Use geometry and workflow checks to verify content is visible early in narrow and desktop viewports.
