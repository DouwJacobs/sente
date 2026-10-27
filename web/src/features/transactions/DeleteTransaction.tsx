import { useState } from "react";
import { api, money } from "../../api";
import { Button, Field, Form, Modal } from "../../ui";
import type { Row } from "../../shared/types";
import type { useTask } from "../../shared/useTask";

export function DeleteTransaction({
  transaction: t,
  dirty,
  busy,
  run,
  onClose,
  onDeleted,
}: {
  transaction: Row;
  dirty: boolean;
  busy: boolean;
  run: ReturnType<typeof useTask>["run"];
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [reason, setReason] = useState("");
  const linked = !!(t.transfer_counterpart_id || t.transfer_counterpart_hidden);
  return (
    <Modal title="Delete transaction?" onClose={() => { if (!busy) onClose(); }}>
      <p>
        <strong>{t.description}</strong><br />
        {t.date} · {t.account_name} · {money(t.amount_cents)}
      </p>
      <p>
        This removes the saved transaction from your totals. Its history is kept
        so future imports recognise its bank entry. You cannot undo this here.
      </p>
      {dirty && <p>Your unsaved edits will be discarded if you delete this transaction.</p>}
      {linked ? (
        <>
          <p>Unlink the transfer before deleting this transaction.</p>
          <Button onClick={onClose}>Back to transaction</Button>
        </>
      ) : (
        <Form onSubmit={async () => {
          const ok = await run(
            () => api("/transactions/" + t.id, "DELETE", {
              version: t.version,
              reason: reason.trim(),
              confirmed: true,
            }),
            "Transaction deleted",
          );
          if (ok) onDeleted();
        }}>
          <Field
            label="Reason for deletion"
            validate={value => !value.trim()
              ? "Enter a reason for deleting this transaction."
              : new TextEncoder().encode(value.trim()).length > 500
                ? "Keep the reason to a few short sentences."
                : ""}
          >
            <textarea
              autoFocus required rows={2} disabled={busy}
              placeholder="For example, duplicate transaction"
              value={reason} onChange={event => setReason(event.target.value)}
            />
          </Field>
          <div className="editor-actions">
            <Button variant="danger" type="submit" loading={busy} disabled={busy}>
              Delete transaction
            </Button>
            <Button disabled={busy} onClick={onClose}>Cancel</Button>
          </div>
        </Form>
      )}
    </Modal>
  );
}
