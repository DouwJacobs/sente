import { Badge, Button } from "../../ui";
import { PermissionSummary, capabilityLabels } from "../../MCPPermissions";
import type { Row } from "../../shared/types";
export function MCPConnectionCard({
  connection: t,
  accounts,
  busy,
  onEdit,
  onRevoke,
}: {
  connection: Row;
  accounts: Row[];
  busy: boolean;
  onEdit: (connection: Row) => void;
  onRevoke: (connection: Row) => void;
}) {
  const granted = Object.keys(capabilityLabels).filter(
      (k) => t.permissions.capabilities[k],
    ),
    automatic = granted.filter((k) => t.permissions.automatic_approval?.[k]),
    ids: number[] = t.permissions.constraints.account_ids || [],
    expired = t.expires_at * 1000 <= Date.now();
  const accessLabel = granted.length
    ? (
        {
          categorization: "Categorisation",
          finance: "Finance editing",
          legacy_finance: "Finance editing",
        } as Record<string, string>
      )[t.permissions.preset] || "Custom permissions"
    : "Read-only";
  return (
    <article className="mcp-proposal mcp-access-row" key={t.id}>
      <div className="mcp-agent-header">
        <div className="mcp-agent-name">
          <h3>{t.name}</h3>
          {expired && <Badge tone="bad">Expired</Badge>}
        </div>
        <div className="editor-actions">
          <Button disabled={busy || expired} onClick={() => onEdit(t)}>
            Edit permissions
          </Button>
          <Button disabled={busy} onClick={() => onRevoke(t)}>
            Revoke
          </Button>
        </div>
      </div>
      <dl className="mcp-agent-summary">
        <div>
          <dt>Access</dt>
          <dd>{accessLabel}</dd>
        </div>
        <div>
          <dt>Accounts</dt>
          <dd>
            {ids.length === 1
              ? accounts.find((a: Row) => a.id === ids[0])?.name ||
                "1 selected account"
              : ids.length
                ? `${ids.length} selected accounts`
                : "All accessible accounts"}
          </dd>
        </div>
        <div>
          <dt>Approval</dt>
          <dd>
            {!granted.length
              ? "No changes allowed"
              : automatic.length
                ? `Automatic for ${automatic.length} change ${automatic.length === 1 ? "type" : "types"}`
                : "Manual approval"}
          </dd>
        </div>
      </dl>
      <p className="footnote mcp-context-status">
        Saved context:{" "}
        {t.permissions.read_context ? "Shared with this agent" : "Not shared"}
      </p>
      <p className="footnote mcp-agent-activity">
        {t.last_used_at
          ? "Last used " +
            new Date(t.last_used_at * 1000).toLocaleString("en-ZA")
          : "Not used yet"}{" "}
        · {expired ? "Expired" : "Expires"}{" "}
        {new Date(t.expires_at * 1000).toLocaleDateString("en-ZA")}
        {!t.client_id ? " · Legacy connection" : ""}
      </p>
      <details className="mcp-agent-details">
        <summary>View permission details</summary>
        <PermissionSummary value={t.permissions} accounts={accounts} />
      </details>
    </article>
  );
}
