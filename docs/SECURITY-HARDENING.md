# Security hardening — issues #60, #61, #64 and #65

## Browser mutation boundary

`browser_write.go` owns the browser transaction boundary. Each serialized write checks the initiating session hash, expiry, CSRF and enabled/non-deleted user, then supplies the current username, administrator role and household membership to the callback. Permission checks, shared service calls and audits use that actor. Pre-handler checks remain useful early rejection, but never authorize a later write by themselves.

The inventory covers accounts and grants; user administration and passwords; categories, groups, rules and rule previews; merchant/tag metadata and naming rules; periods, budget settings/builders/targets/rebalance; transaction edits, transfers, seen/review and bulk changes; uploaded import staging/commit; configuration previews/application/removal; branding/network/restart; MCP consent, connection permissions/revocation, proposal decisions and personal context; notification state/preferences; and logout. Manual backups recheck the current browser credential/administrator under the same write mutex as snapshot creation: SQLite forbids `VACUUM INTO` inside a SQL transaction, so this operational action uses a serialized critical section instead. The GET OAuth-consent binding is also guarded, without requiring a CSRF header for GET.

First-run setup and password login intentionally do not require an existing session. Their existing transaction checks protect closed setup and password/identity changes during hashing. Offline administrator creation/recovery and ruleset commands keep their process-lock/current-actor contracts. MCP proposal prepare/apply/replay retain token, role equality, current account grants and current consent checks; soft-deleted users reject explicitly. No browser credential is treated as an MCP credential.

Manual FNB runs retain the initiating browser session/CSRF across network work and recheck it together with current owner roles and account grants in each financial persistence transaction. Scheduled runs instead require a current enabled, non-deleted administrator and current target access. Failed/cancelled runs update operational recovery state without writing financial snapshots.

## Read review

Protected browser reads authenticate on admission. Paged ledger/import/metadata reads and aggregate/report paths use database snapshots for their multi-query count/data consistency and current grant queries. Several simple reads use individual queries; administrator/member flags are still captured at middleware admission. This batch does not promise cancellation of an already admitted read after revocation, or a single authorization snapshot for every browser read. These read guarantees are separate from the new transactional mutation guarantee. A future read-boundary change must refresh actor/session in the owning snapshot without holding a write mutex over response/network IO. Private-account access continues to require explicit grants.

## Atomic user creation

Browser user creation validates/trims the username and hashes the password before opening the serialized write. The refreshed administrator/session then creates the identity and its creation audit in one transaction. Audit failure removes the identity as well, so a retry can succeed. Passwords/digests are excluded from audit details. The first-run and offline creation contracts remain compatible.

## Bounded authentication and retention

Login and OAuth share independent per-peer/path buckets with a 15-minute sliding window and a shared maximum of 4096 live keys. Every admission prunes expired entries, including password-only deployments. At capacity, new peers are rejected instead of evicting live buckets. Existing peer limits and trusted-proxy identification remain unchanged.

Serving starts credential cleanup immediately and then hourly; shutdown waits for cleanup before closing SQLite. Expired browser sessions, pending OAuth requests, authorization codes, access tokens and refresh hashes are removed at expiry. Expired MCP connections are removed with their existing cascaded credentials/proposals. Expired OAuth clients are removed only when no connection references them. Live used refresh hashes remain until their original expiry, preserving reuse detection and connection revocation. Financial audit history is retained. Maintenance failures log a fixed message without credential/provider details. No schema migration is needed.

## Connector cancellation

The owner worker accepts a bounded credential JSON line, followed by cancellation-only JSON frames on the same stdin pipe. Go also sends cooperative SIGTERM to native Linux workers; stdin cancellation works with Windows Node through WSL. Unexpected loss of the parent pipe aborts the worker. Signals/control stay installed through logout/browser/profile cleanup. Cancelled output cannot be persisted even if the child exits successfully or a provider returns a partial snapshot.

Go allows 30 seconds for cooperative cleanup after its four-minute timeout/service-stop cancellation. It then terminates the Linux process group/descendants and reported detached browser group, or the Windows worker tree with a bounded five-second `taskkill /T /F` call. A bounded wait also prevents inherited output pipes from hanging shutdown. The IPC stderr parser accepts only numeric worker/browser PIDs; arbitrary stderr is discarded and no process metadata is exposed in financial/MCP output.

Go creates each profile directory and removes that exact directory after termination, including forced cancellation. Windows Node/Chrome use a generated local Windows temporary directory mapped into WSL. Cleanup failure rejects the result with a fixed diagnostic. The ordinary production image still excludes the connector runtime/browser.

Synthetic tests cover middleware-admitted revocations, audit rollback/retry, current membership versus private grants, transaction/import rejection, manual connector session revocation, rate-map flooding/expiry, live reuse evidence, and cooperative/startup/hung worker cleanup on Linux and WSL/Windows Node, including real headless Chromium/Chrome on blank pages with disposable profiles. No real bank page or credential is required.

Live MFA/layout/logout compatibility and overlapping history remain owner-run checks. Another overlapping OFX export is still needed to establish cross-download identifier stability. This hardening does not establish production support for live FNB or add PWA/versioning/release publication.

## MCP impact

No tools, input schemas, financial output allowlists, capability/consent grants or proposal contracts expand. Browser user/password administration and connector lifecycle controls remain browser/offline workflows. MCP uses the existing shared financial services and credential-reuse detection; expiry maintenance removes only invalid credentials and expired connections. The internal worker transport change is not a public browser API or MCP protocol change.
