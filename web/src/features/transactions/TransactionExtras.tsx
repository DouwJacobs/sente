import { useState } from "react";
import { Link2 } from "lucide-react";
import { api, money } from "../../api";
import { Button, Field, validateFields } from "../../ui";
import { useTransactionAccess } from "../../TransactionAccess";
import type { Row } from "../../shared/types";
import type { useTask } from "../../shared/useTask";
import { AuditHistory } from "./AuditHistory";
export function TransactionExtras({
  transaction: t,
  busy,
  run,
  refresh,
  onClose,
}: {
  transaction: Row;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
  refresh: () => void;
  onClose: () => void;
}) {
  const { openTransaction } = useTransactionAccess();
  const [historyOpen, setHistoryOpen] = useState(false);
  const [counterpart, setCounterpart] = useState(""),
    [candidate, setCandidate] = useState<Row | null>(null);
  return (
    <details className="details transaction-editor-extras">
      <summary>Transfer links, original details and history</summary>
      <div className="transaction-extra-grid">
        <details className="details">
          <summary>Link or unlink transfer</summary>
          {t.transfer_counterpart_id || t.transfer_counterpart_hidden ? (
            <>
              <p>
                {t.transfer_counterpart_hidden ? (
                  "The linked account is not accessible to you."
                ) : (
                  <button
                    type="button"
                    className="transaction-link"
                    onClick={() => openTransaction(t.transfer_counterpart_id)}
                  >
                    Linked to transaction #{t.transfer_counterpart_id}
                  </button>
                )}
              </p>
              {t.can_edit && !t.transfer_counterpart_hidden && (
                <Button
                  loading={busy}
                  disabled={busy}
                  onClick={async () => {
                    if (
                      await run(
                        () => api("/transfers/" + t.id, "DELETE"),
                        "Transfer unlinked",
                      )
                    ) {
                      refresh();
                      onClose();
                    }
                  }}
                >
                  Unlink transfer
                </Button>
              )}
            </>
          ) : t.can_edit ? (
            <>
              <p className="muted">
                Enter the transaction number from the other account. The amounts
                must match, with one payment and one deposit. Record bank fees
                separately.
              </p>
              <div className="form-grid transfer-lookup">
                <Field
                  label="Transaction number in the other account"
                  validate={(value) =>
                    value && !/^[1-9][0-9]*$/.test(value)
                      ? "Enter a valid transaction number."
                      : ""
                  }
                >
                  <input
                    inputMode="numeric"
                    value={counterpart}
                    onChange={(e) => {
                      setCounterpart(e.target.value);
                      setCandidate(null);
                    }}
                  />
                </Field>
                <Button
                  loading={busy}
                  disabled={busy || !counterpart}
                  onClick={(e) => {
                    if (validateFields(e.currentTarget.closest(".form-grid")!))
                      run(async () => {
                        const v = await api(
                          "/transactions?id=" + encodeURIComponent(counterpart),
                        );
                        if (!v.items.length)
                          throw new Error(
                            "This transaction was not found, or you do not have access to its account.",
                          );
                        setCandidate(v.items[0]);
                      });
                  }}
                >
                  Find transaction
                </Button>
              </div>
              {candidate && (
                <div className="notice">
                  <button
                    type="button"
                    className="transaction-link"
                    onClick={() => openTransaction(candidate.id)}
                  >
                    {candidate.date} · {candidate.description} ·{" "}
                    {candidate.account_name} · {money(candidate.amount_cents)}
                  </button>
                  <Button
                    loading={busy}
                    disabled={busy}
                    onClick={async () => {
                      if (
                        await run(
                          () =>
                            api("/transfers", "POST", {
                              left_id: t.id,
                              right_id: candidate.id,
                              left_version: t.version,
                              right_version: candidate.version,
                            }),
                          "Transfer linked and accepted.",
                        )
                      ) {
                        refresh();
                        onClose();
                      }
                    }}
                  >
                    <Link2 size={16} />
                    Link transactions
                  </Button>
                </div>
              )}
            </>
          ) : (
            <p>Editor access is required.</p>
          )}
        </details>
        <details className="details">
          <summary>Original import details</summary>
          <dl className="provenance">
            {Object.entries(t.provenance).map(([k, v]) => (
              <div key={k}>
                <dt>{k.replaceAll("_", " ")}</dt>
                <dd>
                  {typeof v === "object" ? JSON.stringify(v) : String(v ?? "—")}
                </dd>
              </div>
            ))}
          </dl>
        </details>
        <details
          className="details"
          onToggle={(e) => setHistoryOpen(e.currentTarget.open)}
        >
          <summary>Change history</summary>
          {historyOpen && <AuditHistory id={t.id} />}
        </details>
      </div>
    </details>
  );
}
