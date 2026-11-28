# Sente UI and workflow implementation handover

11 November 2026 · Source checkout `/home/douw/finance-tracker` · `codex/split-spending-groups` at `d7f5f1b` plus existing working-tree changes.

The review prioritized **01, 02 and 03**. They affect a saved household preference, the financial impact shown before saving a period, and where an importer lands after partial success. The remaining tickets can be picked up independently, except that 05 should precede 06. Each ticket describes a source-traced defect and a synthetic regression for the implementing agent to demonstrate before and after editing. The priority order reflects user impact. It does not estimate frequency in production.

## Implementation — 12 November 2026

All twelve tickets are implemented in the existing checkout. The original ticket descriptions below remain available for regression work. Verification results are recorded in docs/VERIFICATION.md under the matching implementation date.

| Tickets | Implemented behavior |
| --- | --- |
| 01 | General preferences and backup history load and retry independently. Unsaved start-day edits survive background refreshes. Placeholder values cannot be submitted. |
| 02 | Period previews and their pages are bound to an editor generation. Changing or closing a draft invalidates outstanding responses. Saving locks dismissal and the period fields. |
| 03 | Partial bulk imports stay in Import activity with their remaining decisions. Successful files are retained for explicit navigation and are excluded from retry commits. Navigation waits for the remaining previews to finish. |
| 04 | API errors retain HTTP status internally. Session startup treats anonymous sessions and unavailable session checks separately, with retry for outages. |
| 05 | List navigation receives the actual requested page size. |
| 06 | Transaction period assignment and Rules account filters use paged lookup. Saved off-page periods hydrate by ID. Upload eligibility comes from an editor-scoped account request. |
| 07 | Category search exposes rows and creation only after the current query has settled successfully. |
| 08 | Background inbox updates preserve the page and filter, with clamping after records disappear. |
| 09 | Agent settings offer read retries. Failed refreshes clear stale proposal actions and selection. Permission saves close their editor before refreshing, and obsolete reads are ignored. |
| 10 | Dependency checks can retry inside the category editor while preserving its rename draft. Archiving waits for the check, and stale responses are ignored. |
| 11 | Review and import counts load independently. Unavailable counts use uncounted tab labels. Dashboard setup guidance waits for a verified import count. |
| 12 | An unknown bank connection has a neutral settings action and an in-place status retry. |

The new synthetic regression spec is web/e2e/ui-workflow-review.spec.ts. It exercises real browser entry points with controlled failures, delayed responses and large synthetic catalogues. Existing workflow and shared-control specs provide complementary financial, keyboard, theme and responsive coverage.

## Scope and baseline

Reviewed the signed-in workspace and startup paths, dashboard and spending flow, transactions and review, import preview/history, banking and accounts, budgets, categories, rule and merchant selection, global search, notifications, settings, agent permissions and PWA entry points. The existing nine-November transaction work is part of the baseline. Preserve its transfer guards, merchant defaults, historical rule previews, review-queue rechecks and import status changes. Check the current working diff before touching those files.

This is a structural review with a focused synthetic browser baseline. Layout, contrast and real-device behavior have not been manually inspected. Use the existing responsive tests for keyboard, focus, mobile actions and themes when implementing. Source paths below are relative to the application root. Reuse the current components, access checks, signed previews and transactional services.

## First assignments

### 01 · General settings can save a value the household never selected

**Where:** `web/src/features/settings/SettingsPage.tsx` and `GeneralSettings.tsx` in the same folder. Start day initializes to 20. A single Promise.all couples settings to users, backups and account management. The form can submit during loading or after any one of those reads fails. Backup history also renders its empty copy before a successful read.

**Reproduce:** Seed day 7, sign in as a household administrator, and fail only GET /backups. Open General. Save default can write 20 even though GET /settings succeeded. A delayed initial response can also overwrite typing.

**Deliver:** Give start day and backup history independent load/error/retry states. Enable editing after the saved day arrives. Preserve a dirty day during unrelated refreshes. Allow appearance controls to work independently. Show empty backup history only after a successful empty response.

**Accept:** In `settings.spec.ts`, delay settings and fail backups separately. Assert no PUT can originate from a placeholder, day 7 is shown after recovery, an edited day survives a background refresh, and one deliberate save persists it. Check an administrator without budget membership.

### 02 · Budget period preview can describe an earlier draft

**Where:** `web/src/features/budgets/Budgets.tsx`, period editor, preview pagination and modal dismissal. Editing clears the current preview, but an outstanding response unconditionally installs its result.

**Reproduce:** Delay Preview affected transactions for range A. Change the end date to range B before A returns. A’s count and rows appear under B and enable Save period. Closing and opening a different period before completion provides a second stale-response path.

**Deliver:** Bind each preview/page response to the originating editor instance and draft. Ignore it after a change, close or replacement. Apply the same rule to pagination. Keep the server preview token and version checks. Lock dismissal during the actual save so an old completion cannot close a newly opened editor.

**Accept:** Delay the initial preview and a later page in separate browser cases. Only a preview obtained for B enables B’s save. Closing and reopening a different period leaves its preview empty. Verify one successful preview/save round trip with the existing budget fixtures.

### 03 · Bulk import leaves the unresolved file behind on partial success

**Where:** `web/src/features/imports/Imports.tsx`, addReady and continueImported, and `ImportPreview.tsx`. The bulk loop stops at the first failed commit, then navigates if any earlier commit succeeded. The ordinary multi-file preview path already stays put when work remains.

**Reproduce:** Register three ready files. Let the first commit succeed and fail the second. The app can leave Import activity for the first file’s transactions before the operator can resolve the second.

**Deliver:** Stay in Import activity after partial success. Identify the successfully added files, the failed file and those still waiting. Keep decision drafts for pending files where their preview versions remain valid. Make viewing the successful subset an explicit action; continue automatically after complete success as today.

**Accept:** Extend `fnb-transactions.spec.ts` or `cohesive-flow.spec.ts`. Assert first/second/third request counts of 1/1/0 before retry. Recover, finish the remaining files, verify no duplicate commit of file one, and verify the correct review versus all-transactions destination.

## Shared navigation and data selection

### 04 · A session-check outage looks like a sign-out

**Where:** `web/src/features/auth/useSession.ts` swallows every error from the startup /me read. `web/src/api.ts` drops the HTTP status when it throws.

**Reproduce:** With a valid signed-in fixture, let /setup succeed and return 503 for /me. The app falls through to sign-in. **Deliver:** Distinguish an anonymous 401 from an unavailable session check, using the existing startup retry path. Preserve the shared session-expiry event and CSRF handling. **Accept:** Cover 401, 503 and interrupted fetch in onboarding/login tests. A successful retry restores the signed-in workspace without asking for credentials.

### 05 · Shared selectors advertise pages that do not exist

**Where:** `web/src/PagedList.tsx`. PagedSelect requests 50 rows per page, ListNavigation omits page size, and Pagination in `web/src/ui.tsx` defaults to 20.

**Reproduce:** With 51 matching options, open Find more options. Two API pages appear as three in the control. Next from the second page requests a nonexistent page before the hook corrects it. **Deliver:** Pass the actual hook page size through list navigation. **Accept:** Cover totals 50, 51 and 100 at size 50 and retain a size-20 caller. At 51, the second page contains the final option, reads Page 2 of 2 and disables Next. Exercise selection and focus from the real form.

### 06 · Long histories hide eligible accounts and periods

**Where:** `features/workspace/useWorkspaceData.ts` under `web/src` seeds the first 100 rows. `features/transactions/TransactionEditor.tsx` maps that period array into Assigned period. `features/rules/RulesPage.tsx` similarly maps accounts into its filter. `features/imports/Imports.tsx` only mounts the paged upload selector if the initial account slice contains an editor.

**Reproduce:** Pick a period outside the first 100 that is also outside the current workspace selection. Try to assign a transaction to it. Separately place a user’s sole editable account after 100 viewer accounts and open Upload a bank export.

**Deliver:** Page/search the authorized source at each entry point. Hydrate the existing assigned period by ID, even when the workspace period differs. Derive upload eligibility from the editor-scoped source. Preserve the distinction between account viewers, editors and household members. Split ownership by screen if needed after 05 lands.

**Accept:** Exercise old and new manual assignments, an off-page Rules account filter and the late editor account. Verify correct IDs in requests, retained labels after reopening, and the viewer-only empty state. Never broaden the server scope to populate a selector.

### 07 · Choice dialogs offer creation before search settles

**Where:** `web/src/Choices.tsx`, ChoiceField. ListStatus, retained rows, “No matches” and the create controls render independently. Enter shares the quick-create path while a server search is pending.

**Reproduce:** Search for an existing category outside the preloaded options, delay its search response, then press Enter. The dialog can issue a create request before learning the category exists. With a failed first read it can show Retry alongside “No matches”.

**Deliver:** Tie results and create eligibility to the settled current query. Suppress stale actionable rows and the no-match message during pending/failed reads. Keep explicit retry, keyboard selection, concurrent-create handling and the existing local-only picker behavior.

**Accept:** In a real transaction editor, delay search A, enter search B, then fail and retry B. Enter must send zero create requests while unresolved. A successful exact match selects the existing ID; a settled, genuinely new name can still be created once. Verify Escape returns focus to the field.

### 08 · Background inbox refresh takes readers back to page one

**Where:** `web/src/features/notifications/NotificationCentre.tsx`. The 30-second interval, focus and visibility handlers all invoke a callback that resets page to zero.

**Reproduce:** Seed 41 messages, go to page two, then refocus the browser. **Deliver:** Revalidate the current page and filter on background events. Clamp to the last accessible page when the result shrinks. Preserve the current privacy behavior that clears old content while access is rechecked. Keep explicit filter changes starting on page one.

**Accept:** In `notifications.spec.ts`, verify timer, focus and visibility refresh on page two. Remove access to enough items to invalidate the page, confirm removed messages disappear and the last valid page becomes reachable. Reading or refreshing must never mark messages read.

## Recovery and feedback

### 09 · Failed agent settings load leaves a permanent spinner

**Where:** `web/src/MCPSettings.tsx`. The initial read reports an error toast but leaves state null. The endpoint panel continues to say “Loading MCP settings”; revisiting its retained tab does not rerun the mount effect.

**Deliver and test:** Fail GET /mcp/settings once, then restore it. Expose a failed state with Retry in the same tab. After retry, render the real connections and proposals. Also test a successful proposal decision followed by a failed refresh: offer a read retry that never resubmits the decision. Keep selected proposal IDs reconciled with pending proposals and preserve explicit permission consent. Extend `mcp.spec.ts`.

### 10 · Category dependency failure strands a rename draft

**Where:** `web/src/Organisation.tsx`, CategoryEditor, opened from Categories and global search. A failed dependencies request leaves deps null, Save disabled and “Checking dependencies” visible indefinitely. Archive can be toggled before the dependency check finishes.

**Deliver and test:** Type a new name, fail the dependencies read, then retry in the dialog. Preserve the draft, show the actual failed state and wait for dependency results before enabling archive. A late response for a closed or replaced editor must have no effect. Verify a blocked archive still explains the dependency and a permitted rename saves once through either entry point.

### 11 · Workspace badges report zero when counts are unknown

**Where:** `web/src/features/workspace/useWorkspaceData.ts` initializes review/import totals to zero and silently ignores a rejected combined request. `web/src/App.tsx` interpolates both totals into transaction tabs and passes stagedCount to downstream views.

**Deliver and test:** Fail only GET /imports?page=0 while the pending transactions query returns a positive total. Show the verified review count; keep the import tab usable with an uncounted label until its read succeeds. On an initial pending request, show neither a fabricated zero nor completion copy based on it. After successful empty responses, zero is appropriate. Audit consumers of stagedCount for the same distinction. Extend `workspace-controls.spec.ts` with delayed, failed and recovered reads.

### 12 · Connection status failure presents “Connect FNB” to existing customers

**Where:** `web/src/features/imports/Imports.tsx`, FNB summary effect and button row. When the first summary read fails, connection is null; the screen displays an unavailable status, a disabled fetch button and a primary Connect FNB button.

**Deliver and test:** Make Retry connection status available in Import activity. During unknown status label the settings destination neutrally. Promote Connect FNB only after a successful disconnected response. Fail the first summary GET for a connected administrator, recover in place and verify Get transactions invokes its normal explicit callback. Cover successful disconnected, refreshing and non-admin cases in `fnb-transactions.spec.ts`. Use synthetic responses throughout.

## Integration and verification

Run implementation work in the WSL checkout. Follow AGENTS.md, docs/UI.md and docs/PLAN.md. For each ticket, reproduce with a disposable fixture, add the smallest regression that exercises the user action, make the scoped change and record the exact commands/results in docs/VERIFICATION.md and docs/HANDOVER.md. Use the spec filenames above with `python3 scripts/test-browser.py <spec>` from the repository root. Run `npm --prefix web test` and `npm --prefix web run build`. For shared controls, include the project’s mobile, theme, keyboard and focus checks. For edits to the transaction/import files, rerun the November transaction-workflow regressions.

All twelve assignments concern browser behavior. Existing authorized reads, writes, versions, import semantics and consent remain authoritative. Ticket 09 changes the browser administration experience for MCP. None of these assignments requires an agent tool, output field, migration or permission expansion.

**Review verification:** All 30 frontend unit tests passed. The production build passed with the existing bundle-size advisory. The browser baseline below uses fresh synthetic services. The proposed failure and scale cases above are acceptance work for the implementing agents; they were traced in source during this review. Live bank access, physical-device interactions and a complete browser-suite rerun are outside this review’s evidence.

All 10 cases in cohesive-flow.spec.ts passed, including 360px and 1440px in both themes, import continuation, staged rows and automated empty-state/modal checks.
