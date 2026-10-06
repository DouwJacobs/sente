# Future features

## Agent access through MCP / ACP

Requested by the owner on 2026-10-02 and authorized for implementation on 2026-10-04. The MCP finance server is now implemented; see [MCP.md](MCP.md) for the current tool surface, redaction, per-user tokens and exact browser approval workflow. ACP remains future work. The original direction below is historical; the latest acceptance/seen policy supersedes mandatory manual transaction approval.

### Goal

Allow the owner to connect a compatible agent of their choice, without tying the tracker to a specific model, provider, or agent product. The agent can work through the tracker's supported operations to:

- Give account and household overviews, explain category spending, and compare budget periods.
- Find transactions and propose or apply explicitly authorized category, split, description, and period-assignment changes.
- Propose category limits and set up budget periods based on historical spending, income, and the user's preferences.
- Explain proposed changes and return links or identifiers for inspecting them in the tracker.

### Protocol direction

MCP is the proposed interface for exposing finance resources and typed tools to external agents. ACP remains a possible interface for connecting or embedding an agent session in a future tracker UI. These are separate responsibilities, not interchangeable protocols. The exact ACP standard, transports, client support, and authentication mechanism must be selected during implementation.

Protocol references:
- [MCP architecture](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/architecture/index.mdx)
- [Agent Client Protocol](https://github.com/agentclientprotocol)

The agent runs independently of the finance backend. Avoid requiring an embedded model or provider API key for ordinary tracker use.

### Integration requirements

- Put an adapter over shared backend services used by the JSON API. Do not give agents direct database or filesystem access. Refactor route-owned business logic into those services where needed.
- Every connection acts for an identified tracker user. Enforce that user's current account permissions and household membership on every tool, summary, search, and transfer result. Use revocable, scoped access; start with read-only access.
- Proposed write behavior: show a concrete change preview and require explicit user authorization before applying it. Enforce authorization on the server, not merely through tool descriptions or client confirmation prompts. Bind approval to the exact proposed changes and record versions; stale proposals require a new preview.
- Agent edits follow the same money, allocation, transfer, period, and concurrency rules as UI edits. Preserve immutable import provenance. Financial changes to approved transactions return them to pending review; agent confidence does not bypass mandatory human review.
- Spending-based budget proposals state their account scope, dates, treatment of pending transactions, refunds and transfers, and gaps in imported history. Drafts do not alter existing limits or historical period dates until accepted.
- Audit applied changes with the initiating user, agent/client identity, operation, before/after details, and approval reference. Keep credentials and financial details out of application logs.
- Transaction descriptions and imported text are data, not agent instructions. Limit returned data to the requested scope. Explain what financial data will be shared before enabling a connection to an externally hosted agent.
- Initially expose finance operations only; bank login credentials, user administration, permission changes, backup restoration, and arbitrary SQL are outside the proposed tool surface.

### Delivery proposal and open decisions

1. Read-only tools: accessible accounts, filtered transactions, period summaries, category spending and current limits.
2. Proposal tools: transaction edits/splits and spending-based budget drafts, inspectable in the tracker.
3. Authorized apply tools: atomic updates with version checks, audit history, and ordinary transaction review.

Decide before implementation: supported MCP clients/transports; what the owner means by ACP and whether an in-app agent session is needed; delegated authorization and access revocation; where change approvals happen; budget analysis defaults and minimum history; proposal persistence and expiry.

Acceptance checks should cover inaccessible accounts in aggregates and transfer links, revoked access, stale and replayed approvals, duplicate tool calls, concurrent edits, exact split totals, untrusted imported text, and equivalent behavior through the UI and agent tools.

## Requested next implementations and reference backlog

The owner requested FNB profile syncing with privately entered credentials, periodic updates controlled in Settings, an administrator-editable workspace display name, and a backlog based on the supplied Vault22 references. These are documented with constraints, next steps and acceptance checks in [HANDOVER.md](HANDOVER.md). None is implemented yet. Spending groups and flat independent categories are implemented in the current app.

Later documentation-only requests also cover the supplied Vault22 budget-management reference, inline category creation from Budgets, and reducing mandatory computer-use inspection token usage. See HANDOVER.md for the current behavior, planning questions and intended acceptance checks.


2026-10-05 owner direction: expand dashboard spending groups into categories, then into bounded transaction lists without leaving Dashboard. Reuse the shared transaction edit modal. Each list must retain selected period/account, parent spending-group ID and category ID together; the same category in another group must not leak into that branch. Split rows must clearly distinguish that category allocation from the parent transaction amount. Refresh affected dashboard totals after saves; retain disclosure state and current scope. Implemented 2026-10-05: dashboard now provides bounded inline group/category spending lists and the shared editor, including split contribution labels, scoped refunds/transfers and refreshed totals.
