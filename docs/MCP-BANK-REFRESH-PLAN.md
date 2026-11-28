# MCP bank refresh: specification and implementation plan

Date: 20 November 2026. Status: implemented; see docs/MCP.md and docs/VERIFICATION.md for the current contract and actual checks.

## Outcome and scope

An authorized agent can refresh balances for existing connected accounts, fetch
recent posted transactions, or do both in one FNB session. The owner controls
which connection may request each action, which accounts it can affect, and
whether requests need individual approval. Credentials remain on the server.

Version one supports the authenticated administrator's own existing FNB profile
and already mapped, visible accounts with current editor access. It does not use
another administrator's bank connection. Account discovery/creation, account
sharing, credentials, schedules, debug/browser settings, hiding/unhiding accounts
and resolving import exceptions remain browser-only. Here, “refresh accounts”
means refreshing existing accounts' balances; discovering new accounts is a
separate future permission, not an implicit effect of this feature.

This is an on-demand refresh, not a new recurring scheduler or full-history fetch.
The current 150-row bank-page limit and incomplete-history warnings still apply.

## Baseline assessed before implementation

Source evidence in this checkout:

| Source | Current behavior / implication |
| --- | --- |
| `internal/app/mcp.go` | Offers `get_account_import_health`, but no bank actions. Uses an explicit read allowlist and read/propose OAuth scopes. |
| `internal/app/mcp_changes.go` | Prepares changes by executing SQL writes then rolling back; applies proposals in one SQL transaction with a stored replay result. Network work cannot run inside this mechanism. |
| `internal/app/mcp_permissions.go`, `mcp_yolo.go` | Separate capabilities/automatic approvals, account constraints, and explicit future-YOLO consent. New bool fields default false in saved JSON. |
| `internal/app/fnb_runner.go` | Serializes connector calls globally, allows four minutes for provider work, and binds encrypted credentials. Its `manual` boolean also controls snapshot/import/schedule behavior. |
| `internal/app/fnb_transactions.go` | `fnbTargets` resolves the owner's mapped editable accounts; there is no explicit multi-account MCP selection. |
| `internal/app/fnb_snapshots.go` | `applyFNBSnapshot` can create discoveries/accounts/grants and update names. Calling it unfiltered would exceed refresh consent. |
| `internal/app/fnb_authorization.go` | `fnbActorTx` rechecks owner/admin, credential fingerprint and optional browser session; it does not retain/recheck MCP connection identity. |
| `internal/app/import_live.go` | `autoCommitFNB` commits clean imports atomically and leaves blocked files staged. |
| `internal/app/fnb_scheduler.go` | Runs scheduled work and marks interrupted connector state at startup. |
| `web/src/MCPPermissions.tsx`, `features/mcp/MCPProposalCard.tsx` | Shared Settings/OAuth consent controls and proposal presentation need refresh-specific copy. |

Do not call `runFNB(owner, true, ...)` directly from an MCP handler: it would lose
the connection's account constraints and ongoing consent checks. Do not use
`manual=false` to obtain combined behavior: that selects scheduled-run semantics.

## Permissions and consent

Add these names to the existing permission JSON format, keeping schema_version=1
unless implementation finds a genuine incompatible format change:

| Permission | Owner-facing label | Effect |
| --- | --- | --- |
| `read_bank_refresh` | Read bank refresh availability and results | Safe availability/eligible-account reads and this connection's job results. No credentials or raw diagnostics. |
| `capabilities.refresh_accounts` | Refresh balances for connected accounts | Request a bank session and update selected existing balances/dates only. |
| `capabilities.refresh_transactions` | Fetch recent transactions for connected accounts | Fetch/stage recent posted transactions and automatically import clean results under the existing import policy. |
| `automatic_approval.refresh_accounts` | Automatically approve account balance refreshes | Allow the corresponding action without individual browser approval. |
| `automatic_approval.refresh_transactions` | Automatically approve transaction refreshes | Allow the corresponding action without individual browser approval. |

Rules:

1. Either change capability requires `read_bank_refresh`. Automatic approval
   requires its matching capability. Combined refresh requires both capabilities;
   automatic combined approval requires both automatic flags.
2. Read-only, categorisation, finance-editing and legacy presets gain no refresh
   access. Custom grants start disabled. A read-only OAuth connection can receive
   the read flag, but must reconnect requesting `finance:propose` to enable actions.
   Keep existing OAuth scopes; new actions use `finance:read finance:propose`.
3. Explicit new/updated YOLO selection includes the new read, capabilities and
   automatic flags. Update the explicit YOLO read assignments in validation as
   well as the frontend preset builder; reflection covers capability fields and
   future-consented read flags, but not every existing preset code path.
4. Existing YOLO without `include_future_permissions` retains its saved snapshot.
   Existing YOLO with that explicit flag resolves the new grants automatically,
   consistent with the owner's existing policy. Runtime admin/ownership/account
   checks always apply, even to YOLO. Never rewrite stored grants on deployment.
5. Refresh status requires an enabled administrator and their own saved profile.
   It may explain `not_connected` without revealing any other owner's connection.
   Action admission additionally requires a ready, usable connection and editor
   access to every requested account. Non-admins cannot use admin bank operations.
6. Selected-account constraints apply to eligibility, proposals, network targets,
   every persistence step, status and replay. Resolve IDs from server mappings;
   never accept bank IDs or credentials. A bad batch rejects as a whole.
7. Missing-category and constrained-rule restrictions continue to constrain those
   operations; they do not grant or suppress independent refresh capabilities.
   Imported classification follows the existing rules, not agent edit privileges.

Settings and OAuth must expose the new read switch, a Banking capability group,
matching automatic approval switches, dependencies, and a plain-language summary.
Explain that fetching transactions may change totals and trigger existing alerts;
bank login can require phone approval. Selecting permissions or reading status
never starts a bank session. Preserve explicit permission-save confirmation.

## MCP contract

Reuse proposals rather than introducing a direct-write refresh tool.

### Read availability / result

Add `get_bank_refresh_status` with a strict typed input:

```json
{}
```

Without `job_id`, return only the caller's safe availability:
`available`, `reason_code`, up to 100 eligible internal account IDs/generic
labels, `retry_after_seconds`, and a bounded `poll_after_seconds` recommendation.
Eligibility is the intersection of the owner's mapped visible FNB accounts,
current editor grants and connection account constraints. Do not return hidden
accounts, bank identifiers, account nicknames, other owners, or global run details.
Reasons include `not_connected`, `attention_required`, `runtime_unavailable`,
`no_eligible_accounts`, `busy`, and `rate_limited`. Busy reveals no job owner.

With `{"job_id":"opaque-id"}`, return this user AND this MCP connection's job.
An unknown or foreign job returns the same generic unavailable error. Recheck
read consent, admin/profile ownership, and access to all frozen targets before
returning counts or target IDs. If a target becomes inaccessible, return a generic
unavailable result; do not partially disclose a formerly authorized job.

### Prepare refresh

Extend `prepare_change` with one operation and a dedicated typed input:

```json
{
  "operation": "refresh_bank",
  "refresh": {
    "mode": "balances_and_transactions",
    "account_ids": [1, 2]
  }
}
```

`mode` is required: `balances`, `transactions`, or `balances_and_transactions`.
`account_ids` is required: 1–100 distinct positive internal IDs. No omitted/all
wildcard; an agent wanting all eligible accounts must submit the explicit list.
Reject unknown modes, duplicate IDs, unrelated change payloads, and targets not
mapped to this owner's current profile. No arbitrary date/history-depth controls.

Preparation performs database validation only. It captures an immutable refresh
intent: exact account IDs, mode, owner/connection, mapping/visibility evidence,
credential fingerprint and permission version. Reuse the one-hour proposal
expiry and 20-active-proposal cap. It does not simulate provider work in the
rollback preview. Browser approval displays the exact scope and states:

> Fetch current bank data for these accounts. Clean transactions may be imported
> using your existing rules. New balances, transaction amounts and counts are
> unknown until the bank responds. Import exceptions require review in Sente.

Approval binds the action and its import policy, not invented before/after bank
values. Existing exact financial-edit proposal guarantees remain unchanged.

### Apply and poll

`apply_change({"proposal_id":"..."})` authorizes and durably accepts a refresh job
in a short SQL transaction. It returns immediately:

```json
{
  "operation": "refresh_bank",
  "job_id": "opaque-id",
  "status": "queued",
  "poll_after_seconds": 5
}
```

For this operation, proposal `applied` means the job was accepted, not that banking
finished. Replays return the same job reference and never start a second run.
Keep `get_change_status` proposal-only; use the new status tool for execution.
Replays must recheck current consent/access before exposing the saved reference.

Final job output contains only: job ID, mode, generic target IDs/labels, lifecycle
state, created/started/finished timestamps, structured reason/recovery codes,
poll/retry guidance, per-account and total counts of balances updated, inserted
transactions, recognized duplicates and staged exceptions, `possible_gap`, and
separate balance/transaction completion flags. No balance amounts or transaction
rows are needed here; the agent can use existing authorized financial reads.
Zero inserted rows may be a successful fresh check. Duplicate counts mean
recognized/skipped rows, not a claim that every source row was checked forever.

Construct dedicated output DTOs with explicit fields. Do not pass through browser
FNB status, discoveries, ParsedFile, raw errors or provider diagnostics. Keep
credentials, credential fingerprints, bank IDs, source references, names, notes,
debug settings and technical logs out of schemas/results/audits visible to agents.
If the generic `mcpSafe` path is used, add only required output keys and regression
tests; prefer the dedicated DTO to broadening unrelated tool outputs.

Status reads are read-only/closed-world. Refresh execution is externally connected:
give the refresh-bearing apply call `OpenWorldHint=true`. Since apply_change is a
mixed tool, conservatively mark it open-world as a whole and explain why. Keep
idempotent replay semantics. Tool annotations never replace server authorization.

## Shared execution service and jobs

### Scope and actor

Introduce explicit run options separating trigger (`browser`, `scheduler`, `mcp`),
balance updates, transaction fetches, discovery allowance, and frozen target IDs.
Preserve browser/scheduler defaults with adapters and characterization tests.
Keep provider credentials/process cleanup and import services shared.

The MCP actor retains owner ID, connection ID, permission version, required grants,
exact targets/mapping evidence and credential fingerprint across the network call.
Extend the shared actor/persistence authorization rather than relying on the
initial request's copied User. Re-resolve current user, current MCP connection
expiry/consent, automatic approval if used, selected scope, editor grants,
visibility, mapping and credential identity at admission, worker start and every
financial persistence transaction. A changed permission version invalidates an
unstarted intent; an in-flight change fails closed before further persistence.
Normal access-token rotation/expiry is not itself connection revocation: jobs
are bound to the approved connection, not to the one-hour bearer token string.

For MCP balance application, update only selected mapped balances and dates.
Do not rename accounts, insert discoveries/accounts, grant access or store results
for out-of-scope accounts. Use a focused balance writer with the same validated
money/identity rules. Provider account listing may necessarily observe additional
accounts inside the bank session; do not persist, log or disclose those results.
Restrict transaction requests themselves to the frozen mapped targets. Never
silently drop a requested account or add newly discovered accounts mid-run.

### Admission, concurrency and lifecycle

Add a durable `bank_refresh_jobs` table through the next consecutive migration
(37 in this checkout; recheck before implementation), owned by a focused schema
file. Store intent, owner/connection association, unique originating proposal ID,
state/timestamps, internal authorization evidence, bounded counters/warnings and
terminal reason. Do not store decrypted secrets or raw provider output in jobs.
Use a child target table if that makes per-account outcomes and constraints safer.

Extend the existing global connector admission gate to reserve an MCP job before
acceptance. Browser, scheduler and MCP must share one lock/order; reject `busy`
rather than accumulating bank logins. Document lock ordering and release on
rollback/cancellation to avoid deadlocks with the existing serialized SQL writer.
Reserve first, then atomically create the unique job, record audit, mark proposal
applied and save its job reference. Audit/write failure rolls back admission and
starts no provider. Wake a serving-owned worker only after commit, outside SQL.
The worker participates in the existing shutdown wait/cancellation lifecycle.

Use durable per-owner MCP admission cooldown: one accepted MCP refresh per
60 seconds across all that owner's agent connections/modes. Persist the admission
time, so another connection or server restart cannot bypass it. Poll no faster
than every five seconds and use existing request limits. Busy/cooldown rejection
does not consume an approved proposal. Never disclose another owner's run.

Lifecycle: `queued -> running -> succeeded | needs_review | partial | failed |
cancelled | interrupted`. A provider attention failure uses `failed` plus a safe
reason/recovery code. `needs_review` means fetch/staging finished but import
exceptions remain. `partial` explicitly means some financial stage committed
before another failed; retain separate committed-stage counters.

Keep the four-minute provider timeout and bounded logout/process cleanup. Do not
hold a database transaction while waiting for FNB. Network effects cannot be
rolled back. Validate logout before any financial results persist. Balances,
staging and clean auto-imports can commit in separate existing service transactions;
do not claim whole-job atomicity. Commit each stage's job receipt/counters with
its data/audit so a crash cannot hide completed effects. Financial authorization
or audit failure rolls back that entire stage.

At startup mark unfinished jobs interrupted, retaining committed-stage receipts;
never automatically replay an accepted bank session. A stale response to
apply_change can be replayed safely to retrieve its job. An interrupted/failed
job needs a fresh explicit proposal for another bank attempt. Existing import
deduplication and archival suppression still protect subsequent runs.

Revocation, user disable/deletion, password recovery or restore, consent removal,
lost target access/visibility and credential replacement block subsequent writes.
Cancel matching active work promptly where lifecycle hooks exist; persistence
reauthorization is mandatory even when immediate cancellation is unavailable.
Do not wait on the provider lock inside a revocation SQL transaction. Disconnect
and shutdown preserve confirmed logout/cleanup behavior. No new public cancel
tool is needed for version one.

Retain terminal job details for 90 days with a bounded per-owner cap (500).
Preserve a minimal proposal-to-job replay receipt while its proposal remains
replayable; after detail expiry return `result_expired`, never repeat execution.
Do not make job/audit evidence cascade-delete when a token or proposal is revoked;
nullable foreign keys plus immutable internal origin identifiers can retain
receipts while making revoked connections unable to query them. Startup/restore
must not resurrect runnable jobs. Cleanup must never evict active work.

### Import and schedule semantics

Use the existing source matching, fee handling, exact integer money, duplicate
recognition, removal suppression, classification and acceptance/seen services.
Clean files commit; blocked files remain in Import activity. Return review counts
and a fixed browser destination, without exposing import source payloads or
silently resolving conflicts. Counts and possible_gap derive from actual import
results. Do not equate success with complete history or a change in row count.

Transactions-only retains current behavior: it does not overwrite Accounts
balances or claim a successful balance refresh. Combined mode applies scoped
balances and transactions in one provider session. Account-only changes no ledger
rows. Refresh approval covers normal rule classification and resulting existing
notifications; it does not require unrelated edit/delete/category capabilities.

Preserve existing calendar/interval next-due calculation after successful manual
refreshes; do not change schedule settings. Recheck the current schedule when
finishing, rather than overwriting concurrently saved settings with captured data.
Reject new MCP starts while connection state needs attention; guide the owner to
Accounts & banking for recovery instead of automated repeated login attempts.
Provider/login/logout failures retain existing pause policy. MCP admission/auth
errors alone must not pause an otherwise healthy connection. Preserve truthful
last-attempt, balance-success and transaction-check timestamps for each mode.

## Implementation sequence

1. **Scoped shared refresh service.** Add run-options/actor types and a mapped-only
   balance writer beside the existing FNB modules. Adapt browser/scheduler callers
   without changing behavior. Test selection, credential/session/actor checks,
   account visibility, confirmed logout, combined mode and transaction-only dates.
2. **Durable admission and execution.** Add schema migration, job/target storage,
   shared global admission, worker/shutdown/startup reconciliation, atomic stage
   receipts, cooldown and retention. Define lock order before integration. Test
   rollback, races, crashes and interruption without any live bank calls.
3. **Consent and proposal integration.** Add read/capability/automatic fields and
   validation/required-capability mapping in mcp_permissions.go. Update YOLO paths.
   Add focused mcp_bank_refresh.go for intent preparation, approval rechecks,
   apply admission and replay. Branch refresh preparation before rollback write
   simulation; existing exact-change operations keep their current path.
4. **MCP reads and discovery.** Register get_bank_refresh_status and extend typed
   prepare_change schema/descriptions, apply description/annotations, initialization
   instructions and public setup copy. Add privacy DTOs and catalogue/schema tests.
   Agent flow: status -> prepare -> owner/automatic approval -> apply -> poll ->
   existing financial reads. Never say “refreshed” on an acceptance-only response.
5. **Owner consent/review UI.** Read docs/UI.md and the repository UI skill before
   implementation. Extend shared MCPPermissions, OAuth consent, connection summary
   and MCPProposalCard with explicit fetch/import effects and automatic controls.
   Show bounded job progress/result in the MCP feature using an owner-only browser
   read adapter to the same authorized job service; recheck scope before rendering.
   Preserve drafts/request locks and existing Accounts & banking recovery actions.
6. **Verification and documentation.** Execute the gates below. Update docs/MCP.md,
   PLAN.md, ARCHITECTURE.md, REFACTORING.md, relevant FNB runtime guidance, HANDOVER
   and VERIFICATION with actual behavior/results. Remove obsolete blanket “no bank
   operations/import triggers” wording, preserving exclusions for credentials,
   discovery and bank administration. No production bank run is part of CI.

Steps 1–2 establish the shared boundary; steps 3–5 depend on it. Do not expose a
registered action until all authorization and execution gates exist. Preserve
unrelated local changes. No second checkout/application is needed for this plan.

## Verification and acceptance criteria

Use only synthetic provider snapshots and disposable databases.

| Area | Required evidence |
| --- | --- |
| Consent | Old/missing JSON fields stay false; legacy/finance/read-only cannot refresh; custom dependencies and each automatic flag work; new YOLO and both saved future-consent variants behave correctly. |
| Ownership/scope | Non-admin, foreign profile, private-without-editor, hidden, unmapped, out-of-consent, duplicate and mixed-validity batches fail with no provider call or writes. |
| Revocation | Revoke/expire connection, remove consent/automatic approval, disable/delete owner, reset password, replace credentials, change mappings/grants/visibility between prepare/approve/apply/start and each persistence stage. No newly unauthorized results persist or leak through status/replay. |
| Proposal safety | Preparing and approving make zero bank calls. Exact modes/targets bind approval. Expiry and permission changes require fresh preparation. Audit failure prevents job admission. Existing financial proposals still compare exact effects. |
| Concurrency/replay | Concurrent apply/retry produces one job/provider call. Browser/scheduler/MCP serialize. Busy/cooldown preserve approved intent. Cross-connection polling is denied. Restart interrupts without replay; expired details never re-execute. |
| Financial results | Balance-only has no ledger writes/names/discovery; transaction-only preserves balance dates; combined updates only requested accounts. No-scope snapshot results leave no account/discovery side effects. Fees, split groups, duplicates, removed-source suppression, rules, category completeness and seen policy remain correct. |
| Failure/partial | MFA/layout/key/runtime/timeout/logout failures return bounded recovery and correct schedule pause state. Mixed clean/blocked imports, balance success plus import failure, crash after a committed stage, zero new rows and incomplete coverage all report accurately. |
| Privacy | Tool schema/output, proposal result, status, replay and browser job adapter exclude secrets/bank IDs/private text and foreign job counts. New browser fields cannot silently enter the generic MCP allowlist. |
| Migration/lifecycle | Fresh/36/representative older upgrades, existing data/grant/consent preservation, injected rollback/retry and once-only restart; token/proposal deletion and restore retain safe receipts without runnable orphan jobs. |
| UI | Shared OAuth/Settings dependency validation, default-off grants, combined approval copy, pending/accepted/completed distinction, revocation recovery and stable drafts; synthetic phone/light and desktop/dark workflows. No computer-use inspection unless the owner requests it. |

Run affected backend/domain race tests and Go vet, then the required complete
backend gates, frontend unit tests/build and targeted synthetic MCP/FNB/import/
scheduling/browser workflows. Follow the documented WSL toolchain and batching.
Record actual tests and limits; external MCP-client compatibility and live FNB
layout/MFA remain separate owner-run checks. Implementation checks are recorded separately in VERIFICATION.md; the original
specification itself was documentation-only.

## Decisions to carry into implementation

The recommended defaults are explicit-target existing accounts only, separate
balance/transaction grants, existing proposal approval, asynchronous durable jobs,
no automatic retry, a 60-second MCP admission cooldown and 90-day bounded results.
MCP.md records the implemented contract; VERIFICATION.md records its evidence. Discovery through MCP,
non-admin delegated bank ownership, historical backfill and agent schedule editing
are intentionally outside version one and need separate product/consent designs.
