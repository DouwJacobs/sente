import { useRef, useState } from "react";
import { api, money } from "../../api";
import { Button, Field, Form, Modal, Pagination, Empty } from "../../ui";
import { PagedSelect } from "../../PagedList";
import { useTask } from "../../shared/useTask";
import type { PageProps, Row } from "../../shared/types";
import type { RuleRef } from "./types";

export function RuleBackfill({
  refs,
  notify,
  refresh,
  onClose,
  offered = false,
  canSelectPeriod = true,
}: {
  refs: RuleRef[];
  notify: PageProps["notify"];
  refresh: () => void;
  onClose: () => void;
  offered?: boolean;
  canSelectPeriod?: boolean;
}) {
  const [scope, setScope] = useState("all"),
    [period, setPeriod] = useState(""),
    [preview, setPreview] = useState<Row | null>(null),
    [page, setPage] = useState(0);
  const { busy, run } = useTask(notify);
  const selection = useRef("");
  selection.current = scope + ":" + period;
  const reset = () => {
    setPreview(null);
    setPage(0);
  };
  return (
    <Modal
      title={
        offered
          ? "Apply this rule to previous transactions?"
          : "Apply to previous transactions"
      }
      size="wide"
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <p>
        The rule is saved for future imports. You can also fill missing
        assignments in previous matches. Existing merchants, categories and
        spending groups are kept. Selecting a budget period limits changes to
        household transactions assigned to it. Transfers and split transactions
        are excluded.
      </p>
      <Form
        onSubmit={() =>
          run(async () => {
            const currentSelection = selection.current;
            const result = await api("/rule-backfill/preview", "POST", {
              refs,
              period_id: scope === "period" ? Number(period) : 0,
            });
            if (selection.current !== currentSelection) return;
            setPreview(result);
            setPage(0);
          })
        }
      >
        <Field label="Previous transactions">
          <select
            value={scope}
            disabled={busy}
            onChange={(e) => {
              setScope(e.target.value);
              reset();
            }}
          >
            <option value="all">All previous transactions</option>
            {canSelectPeriod && (
              <option value="period">A specific budget period</option>
            )}
          </select>
        </Field>
        {scope === "period" && (
          <PagedSelect
            url="/periods"
            label="Budget period"
            value={period}
            required
            onChange={(value) => {
              setPeriod(value);
              reset();
            }}
          />
        )}
        <div className="editor-actions">
          <Button type="submit" loading={busy}>
            Preview changes
          </Button>
          <Button disabled={busy} onClick={onClose}>
            {offered ? "Not now" : "Cancel"}
          </Button>
        </div>
      </Form>
      {preview && (
        <section aria-label="Backfill preview">
          <p role="status">
            {preview.items.length} transactions will change ·{" "}
            {preview.conflicts} conflicting matches · {preview.skipped} skipped
            or already assigned.
          </p>
          {!preview.items.length && (
            <Empty kind="complete" title="No changes to apply">
              <p>
                No eligible matches have missing assignments this rule can fill.
              </p>
            </Empty>
          )}
          {preview.items.slice(page * 20, (page + 1) * 20).map((item: Row) => (
            <article className="rule-backfill-row" key={item.id}>
              <strong>{item.description}</strong>
              <small>
                {item.date} · {item.account} · {money(item.amount_cents)}
              </small>
              <dl>
                {(["merchant", "category", "group"] as const).map((key) => (
                  <div key={key}>
                    <dt>
                      {key === "group"
                        ? "Spending group"
                        : key === "merchant"
                          ? "Merchant"
                          : "Category"}
                    </dt>
                    <dd>
                      {item.before[key]} → {item.after[key]}
                    </dd>
                  </div>
                ))}
              </dl>
            </article>
          ))}
          <Pagination
            page={page}
            total={preview.items.length}
            size={20}
            onChange={setPage}
          />
          {!!preview.items.length && (
            <Button
              variant="primary"
              loading={busy}
              onClick={() =>
                run(async () => {
                  let result: Row;
                  try {
                    result = await api("/rule-backfill/apply", "POST", {
                      token: preview.token,
                    });
                  } catch (error) {
                    setPreview(null);
                    throw error;
                  }
                  notify(
                    "Rule applied to " +
                      result.applied +
                      " transactions; changed entries are unseen.",
                  );
                  refresh();
                  onClose();
                })
              }
            >
              Apply {preview.items.length} changes
            </Button>
          )}
        </section>
      )}
    </Modal>
  );
}
