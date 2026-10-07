# Future work

This backlog contains unresolved work only. Implemented behavior belongs in [PLAN.md](PLAN.md), [ARCHITECTURE.md](ARCHITECTURE.md), [UI.md](UI.md) and [MCP.md](MCP.md).

## Open directions

- ACP or an embedded agent session: decide whether either is needed, then choose protocol, supported clients, transport and authentication. Existing MCP access already supports independent agents; ordinary tracker use requires no model/provider key.
- FNB coverage and recovery: validate live compatibility with the owner, improve complete pagination/history and distinguish incomplete coverage from successful polling. Preserve manual CSV/OFX fallback and the accepted capped-page overlap limitations.
- Spending-group archive: define historical-budget/rule behavior and migration requirements before implementation. Category rename/archive/restore already exists.
- Richer transaction markers: clarify the meaning and need before adding another status alongside acceptance and personal seen/unseen.
- Any additional Vault22-inspired refinements need explicit scope and must preserve the shared UI, exact-money and account-permission contracts.

Workspace branding, scheduled FNB imports, inline category creation, dashboard group/category budgets and transaction drill-down, review navigation, notes/tags/merchants, budget trends/rebalance/export, and MCP permissions/proposals are implemented. They are not pending backlog items. Verification limits are recorded in [VERIFICATION.md](VERIFICATION.md).
