import { MerchantAvatar } from "../../MerchantAvatar";
import { BulkEditor } from "../../CoreWorkflows";
import { useTransactionAccess } from "../../TransactionAccess";
import { useEffect, useState, useRef } from "react";
import { Eye, EyeOff, CircleCheck, CircleHelp, Check } from "lucide-react";
import { api, money, download } from "../../api";
import {
  Button,
  Badge,
  Empty,
  Loading,
  Pagination,
  ActionMenu,
} from "../../ui";
import { GroupDot } from "../../Choices";
import { useTask } from "../../shared/useTask";
import { type Row, type PageProps } from "../../shared/types";
export function Transactions({
  data,
  revision,
  refresh,
  notify,
  review,
  period,
  account,
  unassigned,
  importIds = [],
  onImport,
  onClearFilters,
  stagedCount = 0,
  filterQuery = "",
}: PageProps & {
  filterQuery?: string;
  importIds?: number[];
  onImport?: () => void;
  onClearFilters?: () => void;
  stagedCount?: number;
  review: boolean;
  period: string;
  account: string;
  unassigned: boolean;
}) {
  const { openTransaction } = useTransactionAccess();
  const [bulk, setBulk] = useState(false);
  const [mobile, setMobile] = useState(() => window.matchMedia("(max-width:760px)").matches);
  const [selectionMode, setSelectionMode] = useState(false);
  const press = useRef<{ timer: ReturnType<typeof setTimeout>; x: number; y: number } | null>(null);
  const heldRow = useRef<number | null>(null);
  const cancelPress = () => {
    if (press.current) clearTimeout(press.current.timer);
    press.current = null;
  };
  useEffect(() => {
    const query = window.matchMedia("(max-width:760px)");
    const changed = () => { setMobile(query.matches); setSelectionMode(false); cancelPress(); };
    query.addEventListener("change", changed);
    return () => { query.removeEventListener("change", changed); cancelPress(); };
  }, []);
  const scope = new URLSearchParams(filterQuery);
  if (account) scope.set("account", account);
  if (period && !unassigned) scope.set("period", period);
  if (unassigned) scope.set("unassigned", "1");
  if (importIds.length) scope.set("imports", importIds.join(","));
  const [items, setItems] = useState<Row[]>([]),
    [offset, setOffset] = useState(0),
    [loading, setLoading] = useState(false),
    [selectedRows, setSelectedRows] = useState<Record<number, Row>>({});
  const [listError, setListError] = useState("");
  const listVersion = useRef(""),
    [total, setTotal] = useState(0),
    [retry, setRetry] = useState(0);
  const selected = Object.keys(selectedRows).map(Number);
  const toggle = (rows: Row[], checked: boolean) => {
    const next = { ...selectedRows };
    for (const row of rows) {
      if (checked) next[row.id] = row;
      else delete next[row.id];
    }
    setSelectedRows(next);
    if (!Object.keys(next).length) setSelectionMode(false);
  };
  const { busy, run } = useTask(notify);
  useEffect(() => {
    listVersion.current = "";
    setOffset(0);
    setSelectedRows({});
    setSelectionMode(false);
    cancelPress();
  }, [filterQuery, period, account, review, unassigned, importIds]);
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setListError("");
    const params = new URLSearchParams(filterQuery);
    params.set("offset", String(offset));
    if (offset > 0 && listVersion.current)
      params.set("list_version", listVersion.current);
    if (account) params.set("account", account);
    if (period && !unassigned) params.set("period", period);
    if (importIds.length) params.set("imports", importIds.join(","));
    if (unassigned) params.set("unassigned", "1");
    api("/transactions?" + params)
      .then((v) => {
        if (alive) {
          listVersion.current = v.list_version;
          setTotal(v.total);
          setItems(v.items);
        }
      })
      .catch((e) => {
        if (alive) {
          notify(e.message, true);
          setListError(e.message);
          if (e.message === "This list changed. Start from the first page.") {
            listVersion.current = "";
            setSelectedRows({});
            setOffset(0);
            setRetry((v) => v + 1);
          }
        }
      })
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [
    filterQuery,
    offset,
    period,
    account,
    review,
    unassigned,
    revision,
    retry,
    importIds,
  ]);
  const pageRows = items,
    pageSelected =
      pageRows.length > 0 && pageRows.every((t) => selected.includes(t.id));
  const setSeen = async (seen: boolean) => {
    const entries = Object.values(selectedRows);
    if (
      await run(
        () =>
          api("/transactions/seen", "POST", {
            seen,
            items: entries.map((t) => ({ id: t.id, version: t.version })),
          }),
        seen ? "Transactions marked seen" : "Transactions marked unseen",
      )
    ) {
      setSelectedRows({});
      setSelectionMode(false);
      listVersion.current = "";
      setOffset(0);
      refresh();
    }
  };
  return (
    <>
      <div className="ledger-tools">
        <span className="muted">
          {mobile && selected.length > 0
            ? `${selected.length} selected`
            : `${total} ${total === 1 ? "transaction" : "transactions"}`}
          <span className="sr-only" role="status">{selected.length > 0 ? `${selected.length} transactions selected. Seen status applies only to you.` : ""}</span>
        </span>
        <div className="toolbar-actions">
          {onImport && items.length > 0 && (
            <Button onClick={onImport}>Import transactions</Button>
          )}
          <ActionMenu label="Transaction actions">
            {mobile && selectionMode && (
              <Button variant="quiet" disabled={busy || loading} onClick={() => { setSelectionMode(false); setSelectedRows({}); }}>Cancel selection</Button>
            )}
            {mobile && selected.length > 0 && (
              <>
                <Button variant="quiet" disabled={busy || loading} onClick={() => setBulk(true)}>Edit selected ({selected.length})</Button>
                <Button variant="quiet" disabled={busy || loading} onClick={() => setSeen(true)}>Mark seen ({selected.length})</Button>
                <Button variant="quiet" disabled={busy || loading} onClick={() => setSeen(false)}>Mark unseen ({selected.length})</Button>
              </>
            )}
            {mobile && !selectionMode && (
              <Button variant="quiet" disabled={busy || loading || !items.length} onClick={() => setSelectionMode(true)}>Select transactions</Button>
            )}
            <Button
              variant="quiet"
              disabled={busy || loading}
              onClick={() =>
                run(() =>
                  download("/transactions/export?" + scope, "transactions.zip"),
                )
              }
            >
              Export filtered transactions
            </Button>
          </ActionMenu>
        </div>
      </div>
      {bulk && (
        <BulkEditor
          rows={Object.values(selectedRows)}
          data={data}
          notify={notify}
          refresh={refresh}
          onClose={() => setBulk(false)}
          onDone={() => {
            setBulk(false);
            setSelectedRows({});
            setSelectionMode(false);
            listVersion.current = "";
            setOffset(0);
            refresh();
          }}
        />
      )}
      {!mobile && selected.length > 0 && (
        <div className="toolbar">
          <div className="toolbar-actions">
            <Button disabled={busy || loading} onClick={() => setBulk(true)}>
              Edit selected ({selected.length})
            </Button>
            {!mobile && selected.length > 0 && (
              <Button
                variant="primary"
                loading={busy}
                disabled={busy || loading}
                onClick={() => setSeen(true)}
              >
                <Eye size={17} />
                Mark seen ({selected.length})
              </Button>
            )}{" "}
            {!mobile && selected.length > 0 && (
              <Button
                loading={busy}
                disabled={busy || loading}
                onClick={() => setSeen(false)}
              >
                <EyeOff size={17} />
                Mark unseen ({selected.length})
              </Button>
            )}
          </div>
        </div>
      )}
      {!mobile && selected.length > 0 && (
        <p className="footnote">
          {selected.length} of 100 selected · Seen/unseen applies only to you.
        </p>
      )}
      {loading && <Loading>Loading transactions</Loading>}
      {listError && (
        <p role="alert">
          {listError}{" "}
          <Button onClick={() => setRetry((v) => v + 1)}>Retry</Button>
        </p>
      )}
      {!items.length && !loading && !listError ? (
        <section className="panel transaction-panel">
          <Empty title={"No transactions found"}>
            {stagedCount > 0
              ? stagedCount +
                (stagedCount === 1 ? " import needs" : " imports need") +
                " attention. Resolve possible duplicates or invalid data in Import activity."
              : review
                ? "There is nothing to review for these accounts and dates. Clear filters to check other transactions."
                : "Clear filters or import transactions to get started."}
            {(filterQuery ||
              period ||
              account ||
              unassigned ||
              importIds.length > 0) && (
              <Button onClick={onClearFilters}>Clear filters</Button>
            )}
            {onImport && (
              <Button onClick={onImport}>
                {stagedCount ? "Resolve import issues" : "Import transactions"}
              </Button>
            )}
          </Empty>
        </section>
      ) : (
        <section className="panel transaction-panel">
          <div className="transaction-head">
            <label className="check">
              <input
                aria-label="Select transactions on this page"
                disabled={
                  loading ||
                  busy ||
                  (!pageSelected &&
                    selected.length +
                      pageRows.filter((t) => !selected.includes(t.id)).length >
                      100)
                }
                type="checkbox"
                checked={pageSelected}
                onChange={(e) => toggle(items, e.target.checked)}
              />
              <span className="sr-only">Select transactions</span>
            </label>
            <span>Description / category</span>
            <span>Amount</span>
          </div>
          {items.map((t) => (
            <div className={"transaction-row" + (selected.includes(t.id) ? " transaction-selected" : "")} key={t.id}>
              <label className="row-check">
                <input
                  type="checkbox"
                  aria-label={"Select " + t.description}
                  disabled={
                    loading ||
                    busy ||
                    (!selected.includes(t.id) && selected.length >= 100)
                  }
                  checked={selected.includes(t.id)}
                  onChange={(e) => toggle([t], e.target.checked)}
                />
              </label>
              <button
                className="transaction-detail"
                aria-pressed={mobile && selectionMode ? selected.includes(t.id) : undefined}
                onPointerDown={(e) => {
                  cancelPress(); heldRow.current = null;
                  if (!mobile || e.button !== 0 || busy || loading || selectionMode || selected.length >= 100) return;
                  press.current = { x: e.clientX, y: e.clientY, timer: setTimeout(() => {
                    press.current = null; heldRow.current = t.id;
                    setSelectionMode(true); toggle([t], true);
                  }, 500) };
                }}
                onPointerMove={(e) => {
                  if (press.current && Math.hypot(e.clientX - press.current.x, e.clientY - press.current.y) > 10) cancelPress();
                }}
                onPointerLeave={cancelPress}
                onPointerUp={cancelPress}
                onPointerCancel={() => { cancelPress(); heldRow.current = null; }}
                onContextMenu={(e) => { if (mobile) e.preventDefault(); }}
                onKeyDown={(e) => {
                  if (mobile && e.key === " ") {
                    e.preventDefault();
                    if (!busy && !loading && (selected.includes(t.id) || selected.length < 100)) {
                      setSelectionMode(true); toggle([t], !selected.includes(t.id));
                    }
                  }
                }}
                onClick={() => {
                  if (heldRow.current === t.id) { heldRow.current = null; return; }
                  if (busy || loading) return;
                  if (mobile && selectionMode) {
                    if (selected.includes(t.id) || selected.length < 100) toggle([t], !selected.includes(t.id));
                    return;
                  }
                  openTransaction(t.id, () => { setSelectedRows({}); setSelectionMode(false); }, scope.toString());
                }}
              >
                <div className="merchant-identity transaction-identity">
                  <span className="transaction-logo">
                    <MerchantAvatar name={t.merchant_name || t.description} logo={t.merchant_logo} />
                    {selected.includes(t.id) && <span className="transaction-selection-mark" aria-hidden="true"><Check size={20} /></span>}
                  </span>
                  <div className="transaction-description">
                    <strong title={t.merchant_name || t.description}>{t.merchant_name || t.description}</strong>
                    {t.merchant_name && t.merchant_name !== t.description && (
                      <small className="transaction-original">{t.description}</small>
                    )}
                    <small className="transaction-account">
                      {t.date} · {t.account_name}
                      {t.household ? "" : " · Private"}
                    </small>
                    <div className="row-meta">
                      {t.spending_group_name && !(t.is_transfer && t.spending_group_name.trim().toLowerCase() === "transfer") && (
                        <span className="group-label">
                          <GroupDot color={t.spending_group_color} />
                          {t.spending_group_name}
                        </span>
                      )}
                      <span>
                        {t.is_transfer
                          ? "Transfer"
                          : t.allocations.length > 1
                            ? "Split · " + t.allocations.length + " categories"
                            : t.allocations[0]?.category_name ||
                              "Uncategorized"}
                      </span>
                      <span className="transaction-statuses">
                        <span className={"transaction-status " + (t.review_state === "pending_review" ? "review-pending" : "review-accepted")}
                          role="img" aria-label={t.review_state === "pending_review" ? "Needs review" : "Accepted"}
                          title={t.review_state === "pending_review" ? "Needs review" : "Accepted"}>
                          {t.review_state === "pending_review" ? <CircleHelp size={14} aria-hidden="true" /> : <CircleCheck size={14} aria-hidden="true" />}
                        </span>
                        <span className="transaction-status seen-state" role="img" aria-label={t.seen ? "Seen" : "Unseen"} title={t.seen ? "Seen" : "Unseen"}>
                          {t.seen ? <Eye size={14} aria-hidden="true" /> : <EyeOff size={14} aria-hidden="true" />}
                        </span>
                      </span>
                      {!!t.household &&
                        !t.period_id &&
                        t.assignment !== "outside" && (
                          <Badge>Needs a period</Badge>
                        )}
                      {t.outside_period === 1 && (
                        <Badge tone="pending">Outside assigned dates</Badge>
                      )}
                    </div>
                  </div>
                </div>
                <strong
                  className={
                    "transaction-amount " +
                    (t.amount_cents > 0
                      ? "positive"
                      : t.amount_cents < 0
                        ? "negative"
                        : "")
                  }
                >
                  {money(t.amount_cents)}
                </strong>
              </button>
            </div>
          ))}
        </section>
      )}
      <Pagination
        page={Math.floor(offset / 100)}
        total={total}
        size={100}
        loading={loading}
        range
        onChange={(page) => setOffset(page * 100)}
      />
    </>
  );
}
