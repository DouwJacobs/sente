import { useTransactionAccess } from "../../TransactionAccess";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import { useEffect, useState } from "react";
import { api, money } from "../../api";
import { Button, Loading, Pagination } from "../../ui";
import { type Row } from "../../shared/types";
export function RetainedPage({ id }: { id: number }) {
  const { openTransaction } = useTransactionAccess();
  const [page, setPage] = useState(0),
    [data, setData] = useState<Row | null>(null),
    [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [retry, setRetry] = useState(0);
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    api("/imports/" + id + "?page=" + page + "&page_size=50")
      .then((v) => alive && setData(v))
      .catch((e) => alive && setError(e.message))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [id, page, retry]);
  return (
    <>
      {loading && <Loading>Loading import details</Loading>}
      {error && (
        <p role="alert">
          {error} <Button onClick={() => setRetry((v) => v + 1)}>Retry</Button>
        </p>
      )}
      {data?.rows.map((row: Row) => (
        <div className="candidate" key={row.row}>
          <small>
            Row {row.row} · {row.date}
          </small>
          <div>
            {row.transaction_id ||
            (row.candidates?.length === 1 && row.candidates[0].id) ? (
              <button
                type="button"
                className="transaction-link"
                onClick={() =>
                  openTransaction(row.transaction_id || row.candidates[0].id)
                }
              >
                {row.description} · {money(row.amount_cents)}
              </button>
            ) : (
              <span>
                {row.description} · {money(row.amount_cents)}
              </span>
            )}
          </div>
          <small>
            {row.error
              ? "Rejected: " + row.error
              : data.decisions?.[row.row] === "skip" ||
                  ["exact_id", "exact_source"].includes(row.duplicate)
                ? "Skipped"
                : "Imported"}
          </small>
        </div>
      ))}
      <Pagination
        page={page}
        total={data?.total || 0}
        size={50}
        loading={loading}
        onChange={setPage}
      />
    </>
  );
}
export function RetainedRows({ id }: { id: number }) {
  const [open, setOpen] = useState(false);
  return (
    <details onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>View imported, skipped, and rejected transactions</summary>
      {open && <RetainedPage id={id} />}
    </details>
  );
}
export function RuleSuggestions({
  id,
  row,
  version,
  onStale,
}: {
  id: number;
  row: number;
  version?: string;
  onStale: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <details onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>Compare rule suggestions</summary>
      {open && (
        <SuggestionPage
          url={
            "/imports/" +
            id +
            "/rows/" +
            row +
            "/suggestions" +
            (version ? "?preview_version=" + version : "")
          }
          onStale={onStale}
        />
      )}
    </details>
  );
}
export function SuggestionPage({
  url,
  onStale,
}: {
  url: string;
  onStale: () => void;
}) {
  const list = usePagedList(url);
  useEffect(() => {
    if (list.error.includes("Import preview changed")) onStale();
  }, [list.error]);
  return (
    <>
      <ListStatus list={list} />
      {list.items.map((m) => (
        <div className="candidate" key={m.id}>
          {m.pattern} → {m.category_name}
          {m.spending_group_name ? " · " + m.spending_group_name : ""}
        </div>
      ))}
      <ListNavigation list={list} />
    </>
  );
}
