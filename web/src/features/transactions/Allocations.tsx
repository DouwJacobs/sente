import type { Dispatch, SetStateAction } from "react";
import { Trash2 } from "lucide-react";
import { cents, money } from "../../api";
import { Button, Field } from "../../ui";
import { CategoryChoice } from "../../Choices";
import { moneyError } from "../../validation";
import type { Row, PageProps } from "../../shared/types";
export function Allocations({
  transaction: t,
  data,
  refresh,
  notify,
  alloc,
  setAlloc,
  update,
  original,
  allocated,
  busy,
}: {
  transaction: Row;
  data: PageProps["data"];
  refresh: () => void;
  notify: PageProps["notify"];
  alloc: Row[];
  setAlloc: Dispatch<SetStateAction<Row[]>>;
  update: (index: number, key: string, value: unknown) => void;
  original: number;
  allocated: number;
  busy: boolean;
}) {
  return (
    <>
      {alloc.length > 1 && <div className="allocation-heading"><h4>Split categories</h4></div>}
      {alloc.map((a, i) => (
        <div
          className={
            "allocation editor-allocation " +
            (alloc.length === 1 ? "single-allocation" : "split-allocation")
          }
          key={i}
        >
          <CategoryChoice
            label={alloc.length === 1 ? "Category" : "Category " + (i + 1)}
            data={data}
            value={a.category_id || null}
            onChange={(id) => update(i, "category_id", id)}
            refresh={refresh}
            notify={notify}
            disabled={!t.can_edit || busy}
          />
          {(alloc.length > 1 || allocated !== original) && (
            <Field
              label="Amount"
              validate={(value) => {
                const error = moneyError(value);
                if (error) return error;
                const parsed = cents(value);
                if (
                  (original < 0 && parsed > 0) ||
                  (original > 0 && parsed < 0)
                )
                  return "Use the same sign as the transaction.";
                if (i === alloc.length - 1 && allocated !== original)
                  return (
                    "Category amounts must add up to " + money(original) + "."
                  );
                return "";
              }}
            >
              <input
                inputMode="decimal"
                required
                value={a.amount}
                onChange={(e) => update(i, "amount", e.target.value)}
              />
            </Field>
          )}
          <details className="allocation-note" open={!!a.note}>
            <summary>
              {alloc.length > 1 ? "Split note" : "Category note"}
              {a.note ? " · Added" : " (optional)"}
            </summary>
            <Field label="Note">
              <input
                maxLength={500}
                value={a.note || ""}
                onChange={(e) => update(i, "note", e.target.value)}
              />
            </Field>
          </details>
          {alloc.length > 1 && (
            <Button
              variant="quiet"
              aria-label={"Remove allocation " + (i + 1)}
              onClick={() => setAlloc(alloc.filter((_, n) => n !== i))}
            >
              <Trash2 size={17} />
            </Button>
          )}
        </div>
      ))}
      {alloc.length > 1 && (
        <div
          className={
            "split-total " + (original !== allocated ? "negative" : "")
          }
        >
          <span>Assigned {money(allocated)}</span>
          <strong>Remaining {money(original - allocated)}</strong>
        </div>
      )}
    </>
  );
}
