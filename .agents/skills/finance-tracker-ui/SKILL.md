---
name: finance-tracker-ui
description: Create, change, or review the finance-tracker interface using its maintained UI guide, responsive workflows, money display, and transaction review conventions. Applies only to this project.
---

# Sente UI

Read the repository's docs/UI.md before UI work. Read relevant requirements in docs/PLAN.md for import, review, splitting, periods, and permissions.

Use this skill and the current conventions in docs/UI.md as the baseline for every Sente UI change. The latest accepted modern finance direction takes precedence over historical retro/theme trials in the guide. Preserve the existing design unless the owner requests a specific change. Once shared tokens and components exist, extend them instead of introducing independent screen styles. Keep proposed choices distinct from accepted conventions.

Start from the existing shared components and tokens: neutral page/chrome/card/input surfaces in both themes, restrained teal primary/selected/focus accents, and semantic financial colors. Keep theme changes synchronized through App state and the saved finance-theme preference; retain the navbar theme toggle and Settings Light/Dark/Follow device choices. Extend shared roles rather than introducing page-specific palettes or parallel control styles.

Preserve complete desktop and mobile workflows, explicit review state, account/period scope, exact split totals, and accessible controls. Avoid the decorative AI design conventions named in the guide.

When the user changes the design direction or a new shared convention is accepted, update docs/UI.md with the implementation. Keep one maintained source rather than duplicating detailed rules here.

Use focused builds, tests and structural checks for routine changes. Use short, targeted computer-use visual checks when requested or when layout/interaction risk warrants them. Record actual coverage; never claim visual verification based only on assertions.

Verify changed workflows at narrow mobile and desktop widths, including realistic long descriptions, pending states, validation errors, and keyboard use. Use synthetic data.

Use shared overlay toasts for request failures without shifting forms or dialogs. Sign-in errors stay visible until dismissal or retry; other notifications retain timed expiry with hover/focus pause.

Follow the form validation convention in docs/UI.md: shared fields and forms, first-blur validation followed by live validation, inline accessible field errors, and dependent-field checks.

Use web/src/styles.css for shared structural rules, web/src/finance-theme.css for the active shared visual tokens, and web/src/ui.tsx for shared controls. Run the production build and browser workflow checks for changes to responsive UI behavior.

Keep card edges and grid rows aligned as described in docs/UI.md. Check both empty and populated layouts within the current inspection policy; avoid independent card width limits and doubled grid gaps.

Current owner instruction (2026-10-02): do not use computer use to inspect the UI for now. Use automated tests/builds and structural checks; resume visual inspection only when requested.

Use the shared --interaction-hover background and --text foreground for unselected clickable rows, including category/group rows. Reserve --accent-soft/--accent for selected states. Never introduce a page-specific hover palette; keep light/dark hover and focus behavior consistent with shared tokens.
