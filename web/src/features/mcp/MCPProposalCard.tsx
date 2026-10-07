import { MerchantAvatar } from "../../MerchantAvatar";
import { useTransactionAccess } from "../../TransactionAccess";
import { Badge, Button } from "../../ui";
import type { Row } from "../../shared/types";
export function MCPProposalCard({
  proposal: p,
  busy,
  selected,
  onSelect,
  onDecide,
}: {
  proposal: Row;
  busy: boolean;
  selected: boolean;
  onSelect: (selected: boolean) => void;
  onDecide: (approve: boolean) => void;
}) {
  const { openTransaction } = useTransactionAccess();
  return (
    <article className="mcp-proposal" key={p.id}>
      <div className="section-head">
        <div>
          {p.status === "pending" && (
            <label className="check">
              <input
                type="checkbox"
                aria-label={"Select proposal " + p.id.slice(0, 8)}
                disabled={busy}
                checked={selected}
                onChange={(e) => onSelect(e.target.checked)}
              />
              Select proposal
            </label>
          )}
          <h3>
            {p.operation.replaceAll("_", " ")} · {p.token_name}
          </h3>
          <small>
            Expires {new Date(p.expires_at * 1000).toLocaleTimeString("en-ZA")}{" "}
            · Proposal {p.id.slice(0, 8)}
          </small>
        </div>
        <Badge>
          {p.status === "approved" && p.payload.automatic_approval
            ? "Automatically approved"
            : p.status}
        </Badge>
      </div>
      <details>
        <summary>Inspect exact changes (amounts in cents)</summary>
        {p.payload.transactions?.map((t: Row) => (
          <div key={t.id}>
            <button
              type="button"
              className="transaction-link"
              onClick={() => openTransaction(t.id)}
            >
              <strong>Transaction #{t.id}</strong>
              <span> · {t.before.description}</span>
            </button>
            <div className="form-grid">
              <div>
                <strong>Before</strong>
                <pre className="mcp-code">
                  {JSON.stringify(t.before, null, 2)}
                </pre>
              </div>
              <div>
                <strong>After</strong>
                <pre className="mcp-code">
                  {JSON.stringify(t.after, null, 2)}
                </pre>
              </div>
            </div>
          </div>
        ))}
        {!p.payload.transactions?.length && (
          <>
            {p.payload.before && (
              <>
                <strong>Before</strong>
                <pre className="mcp-code">
                  {JSON.stringify(
                    p.payload.before,
                    (key, value) =>
                      key === "logo_data" && value
                        ? "[Current local logo]"
                        : value,
                    2,
                  )}
                </pre>
              </>
            )}
            {p.payload.seen_states && (
              <>
                <strong>Current personal seen state</strong>
                <pre className="mcp-code">
                  {JSON.stringify(p.payload.seen_states, null, 2)}
                </pre>
              </>
            )}
            {(p.payload.change.merchant?.logo_data ||
              p.payload.change.merchant_rule?.merchant_logo) && (
              <div className="merchant-identity">
                <MerchantAvatar
                  name={p.payload.change.merchant?.name || "Merchant"}
                  logo={
                    p.payload.change.merchant?.logo_data ||
                    p.payload.change.merchant_rule?.merchant_logo
                  }
                />
                <span>Proposed merchant logo</span>
              </div>
            )}
            {p.payload.budget_after && (
              <>
                <strong>After · exact group budgets and carry-forward</strong>
                <pre className="mcp-code">
                  {JSON.stringify(p.payload.budget_after, null, 2)}
                </pre>
              </>
            )}
            <strong>Requested change</strong>
            <pre className="mcp-code">
              {JSON.stringify(
                p.payload.change,
                (key, value) =>
                  ["logo_data", "merchant_logo"].includes(key) && value
                    ? "[Local image shown above]"
                    : value,
                2,
              )}
            </pre>
          </>
        )}
      </details>
      {p.status === "pending" && (
        <div className="editor-actions">
          <Button
            variant="primary"
            disabled={busy}
            onClick={() => onDecide(true)}
          >
            Approve exact changes
          </Button>
          <Button disabled={busy} onClick={() => onDecide(false)}>
            Reject
          </Button>
        </div>
      )}
    </article>
  );
}
