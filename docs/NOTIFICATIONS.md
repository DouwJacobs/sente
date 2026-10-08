# Notification foundation

Issues #29 and #30 introduce backend infrastructure; this branch also supplies the backend preferences needed by #31. The browser notification centre and personal preference controls build on this foundation. Financial alert producers, browser permission prompts, service workers and external delivery remain follow-ups. The existing Toast component remains transient request feedback. User-visible notifications begin when the separately specified producers and centre are implemented.

## Producer and delivery contract

`notificationEvent` and the shared `App.notify` / `notifyTx` service live in `internal/app/notifications.go`. A producer supplies a recipient, stable type, severity, bounded title/message, source kind/ID, all contributing account IDs, a logical dedupe key, an immutable occurrence time and explicit dismissal policy. Producers calculate financial conditions separately using authoritative financial services. There is no browser/MCP API for arbitrary event creation.

Standalone evaluations call `notify`; producers inside an existing authorized financial write call `notifyTx` with that SQL transaction and the evaluation time. Never call `notify` from inside a write: it would acquire the same lock again. Event persistence, dependency records and delivery receipt commit together with the triggering financial change. The `notificationAdapter` boundary writes channel work within that transaction. The installed adapter is in-app only. Future push adapters must persist an outbox job rather than make a network call while holding the SQL write, and need their own per-channel attempt/receipt state and dispatcher under #39. External provider uncertainty is outside this in-app implementation.

Types are `system`, `budget_threshold`, `budget_projection`, `budget_overspend`, `unusual_spending` and `recurring_payment`; severities are `info`, `warning`, `critical`. Titles are 1–120 Unicode characters, messages 1–1000 and keys 1–200; invalid UTF-8, blank values, NULs, unknown types/sources and future/zero occurrence times reject. Up to 1000 account dependencies are canonicalized and deduplicated before fingerprinting. System events must use source `system`, ID 0, and no financial dependencies. Internal producers must not place financial content in system messages. Budget alert types require a budget-period source; unusual-spending and recurring-payment types require an account or transaction source with explicit account dependencies; an account/transaction source must be covered by that scope.

Each recipient must currently be active. Every dependency must be visible and readable by that recipient, even if the recipient is an administrator. Budget events require household membership, an existing period and only household accounts. Private accounts are excluded from budget event scope. Producers must include every account that contributed to their message; the service does not calculate their financial result or infer omitted inputs.

## Retry and reset semantics

A receipt uniquely identifies recipient + type + SHA-256 of the logical key. Its event fingerprint binds all event fields and canonical account dependencies. Exact retries return the existing record (or ID 0 if its payload was retained only as a receipt). A different payload/occurrence/scope with the same key returns conflict; read/dismiss/eviction never resets the receipt. Keys are scoped per recipient/type; they must not combine distinct privacy scopes. No raw keys or payloads enter logs or public receipt responses.

Producers must keep the original occurrence time and payload for retries. A genuine new condition/crossing needs a new key, including its explicit reset/cycle identity; reusing a key to update an alert is invalid. Threshold, overspend, projection, unusual-spending and recurring-payment detection/reset/cooldown/coalescing semantics remain to be defined and tested with #33–#38. This foundation guarantees exact event idempotency, not producer-level anti-spam heuristics.

Validation/authorization run before dedupe, so an exact retry cannot bypass revoked access. In-app failures roll back the payload, dependencies and receipt and can be retried. Preference-disabled events return `disabled` without a receipt; a later explicitly enabled evaluation can therefore become deliverable. Already delivered records remain in the inbox when preferences are disabled. Both financial and system types default to enabled for the available in-app channel; critical severity does not bypass the user's preference. No push consent or subscription is implied. New types must be registered explicitly and default to in-app enabled; introduce different mandatory-system behavior as an explicit product change.

## Browser APIs

All endpoints use existing cookie authentication, origin checks, CSRF for writes, JSON error conventions and `Cache-Control: no-store`. Inbox responses never expose recipients, account dependencies, dedupe/event hashes or delivery receipts. Source kind/ID are structured references, not arbitrary URLs, and are returned only after current permission checks. The notification centre navigates through existing authorized loaders and handles deletion or permission changes safely.

| Endpoint | Contract |
| --- | --- |
| `GET /api/notifications?page=0&page_size=20&state=unread` | Newest-first paged `items`, filtered `total`, `inbox_total`, `unread_count`, `page`, `page_size`. Optional state is `read` or `unread`; page size is 1–100. |
| `GET /api/notifications/unread-count` | Current accessible, undismissed, unexpired unread count. |
| `POST /api/notifications/{id}/read` | Idempotently marks one accessible inbox record read; returns `ok` and unread count. |
| `POST /api/notifications/read-all` | Marks only currently accessible inbox records read in one serialized write; returns unread count. |
| `POST /api/notifications/{id}/dismiss` | Hides a currently accessible record if its producer allowed dismissal. Unavailable, dismissed and non-dismissible records return 404. |
| `GET /api/notifications/preferences` | Registered type/channel preferences, defaults and optimistic versions (0 for an unsaved default). |
| `PUT /api/notifications/preferences` | Exactly one `type`, `channel: in_app`, explicit boolean `enabled`, and integer `version`; rejects stale versions, unknown fields/types/channels and omitted values. Returns current preferences. |

List/count share a SQL snapshot, and writes recheck the actor's session under the write lock. Each query applies recipient and privacy predicates before count/paging. Old messages disappear if a dependency becomes hidden/inaccessible/private (for budgets), the user loses household membership, or their source disappears or moves to an uncaptured account. Dependency account IDs deliberately retain tombstones instead of cascading away when an account disappears. Administrators cannot inspect another user's inbox. Permission restoration can make a still-retained record visible again. Read-all does not silently mark inaccessible messages read.

Offset paging is newest-first; additions can shift later pages, so the centre refreshes from page zero after mutations and periodic/focus refreshes. Counts reflect current permission and retention state, not the number of stored private/inaccessible payloads.

## Persistence, retention and lifecycle

Migration 24 appends notification receipts, payloads, account dependencies and preferences after baseline 23. It does not modify the frozen schema snapshot or existing financial/configuration/access rows. Fresh databases run 23 then 24; older fixtures upgrade through the same bridge. Failed schema or version-record writes roll back the whole pending batch.

Payloads expire from reads after 90 days, with a maximum of 500 stored payloads per recipient (including dismissed records). Eviction retains receipts. Receipts expire after 180 days; up to 5000 unexpired receipts per recipient are accepted, then new events return capacity error until cleanup releases space. This deliberately refuses further history rather than evicting recent idempotency evidence. Events whose occurrence is already at least 90 days old return `expired`, so a correctly retried old event cannot resurrect after its receipt expires. Clock time is UTC Unix seconds; producers need stable occurrence times.

Serving runs cleanup at startup and every 24 hours; delivery/state writes also clean expired records. Logical expiry is checked at read time, independently of physical cleanup. Cleanup rolls back on failure and logs only a fixed diagnostic message; it retries on the next serving interval/activity. Shutdown waits for cleanup before closing SQLite. Offline commands do not start a scheduler. Soft user deletion immediately removes preferences, receipts and their dependent payload/scope rows while preserving financial history. Normal backups include notification state/preferences; restored sessions are invalidated through the existing restore behavior. Restored notification content still rechecks current permissions.

## MCP impact and remaining work

Notifications and preferences are browser-only personal communication workflows for now. Existing MCP connections have not consented to reading these potentially financial messages. No new tools, schema fields, allowlists, permissions, consent, proposal previews/audits or financial write semantics are added; no notifications are appended to MCP session context. A future MCP notification capability must require explicit consent and the same authorized service boundaries.

Implemented browser workflows (#31–#32): header bell with current unread count, paged All/Unread/Read inbox, explicit read/read-all/dismiss actions, authorized source navigation, and Settings notification preferences. Counts refresh every 30 seconds while visible, on focus and after personal state writes. Inbox snapshots are discarded while reloading; failed writes reload current accessible records. Preferences are saved individually using optimistic versions; failed saves retain the draft and explicit Reload saved preferences discards drafts. Drafts survive Settings section changes. Only the currently available in-app channel is shown; no push opt-in is inferred.

Follow-ups: #33–#37 deterministic financial producers; #38 producer reset/coalescing/cooldowns; #39 opt-in browser push/outbox/PWA integration; #40 bounded redacted administrator diagnostics. No frontend or financial policy has been silently introduced here.
