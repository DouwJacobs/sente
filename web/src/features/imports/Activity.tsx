import { RetainedRows } from "./SourceDetails";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import { useEffect, useState } from "react";
import { Button } from "../../ui";
import { type Row } from "../../shared/types";
export function importedCountLabel(count: number) {
  return (
    count +
    " new " +
    (count === 1 ? "transaction" : "transactions") +
    " imported"
  );
}
export function ActivityRun({
  summary,
  children,
}: {
  summary: import("react").ReactNode;
  children: import("react").ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <details
      className="details activity-run"
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary>{summary}</summary>
      {open && children}
    </details>
  );
}
export function ActivityAccounts({
  rows,
  activityKey,
  filterQuery,
  revision,
  busy,
  resume,
  onReview,
  onTransactions,
}: {
  rows: Row[];
  activityKey: string;
  filterQuery: string;
  revision: number;
  busy: boolean;
  resume: (h: Row) => Promise<void>;
  onReview: (ids: number[]) => void;
  onTransactions: (ids: number[]) => void;
}) {
  const list = usePagedList(
    "/imports?activity=" +
      encodeURIComponent(activityKey) +
      (filterQuery ? "&" + filterQuery : ""),
    revision,
  );
  useEffect(() => list.setPage(0), [activityKey, filterQuery]);
  const shown = list.loading && !list.items.length ? rows : list.items;
  return (
    <>
      <ListStatus list={list} />
      {shown.map((h) => (
        <div className="activity-account" key={h.id}>
          <div>
            <strong>{h.account_name}</strong>
            <small>
              FNB · ••{h.account_ending} ·{" "}
              {h.status === "committed"
                ? importedCountLabel(h.inserted || 0) +
                  " · " +
                  (h.skipped || 0) +
                  " already present · " +
                  (h.needs_categories || 0) +
                  " need categories"
                : "Not imported yet · " + h.row_count + " source rows"}
              {h.error_count ? " · " + h.error_count + " rejected" : ""}
            </small>
            {h.format === "fnb-live" && (
              <small>Older transactions may be missing.</small>
            )}
            {h.error && <p className="error-text">{h.error}</p>}
          </div>
          <div className="toolbar-actions">
            {h.status === "staged" ? (
              <Button disabled={busy} onClick={() => resume(h)}>
                Resolve import issues
              </Button>
            ) : (
              <>
                <Button onClick={() => onReview([h.id])}>
                  Review transactions
                </Button>
                <Button onClick={() => onTransactions([h.id])}>
                  View transactions
                </Button>
              </>
            )}
          </div>
          {h.status === "committed" && <RetainedRows id={h.id} />}
        </div>
      ))}
      <ListNavigation list={list} />
    </>
  );
}
