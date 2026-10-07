import { RuleSuggestions } from "./SourceDetails";
import { useTransactionAccess } from "../../TransactionAccess";
import { useEffect, useState } from "react";
import { api, money } from "../../api";
import { Button, Field, Badge, Loading, Pagination } from "../../ui";
import { useTask } from "../../shared/useTask";
import { type PageProps, type Row } from "../../shared/types";
export function ImportPreview({
  preview: initial,
  initialOpen,
  accountName,
  register,
  disabled,
  onReview,
  categories,
  groups,
  notify,
  refresh,
  onDone,
}: {
  initialOpen: boolean;
  accountName: string;
  register: (id: number, action: (() => Promise<boolean>) | null) => void;
  disabled: boolean;
  onReview: () => void;
  preview: Row;
  categories: Row[];
  groups: Row[];
  notify: PageProps["notify"];
  refresh: () => void;
  onDone: () => void;
}) {
  const { openTransaction } = useTransactionAccess();
  const [p, setPreview] = useState(initial),
    [loading, setLoading] = useState(false),
    [pageError, setPageError] = useState(""),
    [skipAll, setSkipAll] = useState(false),
    [stale, setStale] = useState(false);
  const [decisions, setDecisions] = useState<Record<string, string>>({}),
    [confirm, setConfirm] = useState(false),
    [page, setPage] = useState(0);
  const { busy, run } = useTask(notify);
  const loadPage = async (next: number) => {
    setLoading(true);
    setPageError("");
    try {
      const result = await api(
        "/imports/" + p.id + "?page=" + next + "&page_size=50",
      );
      if (
        result.preview_version !== p.preview_version ||
        result.classification_version !== p.classification_version
      ) {
        setDecisions({});
        setSkipAll(false);
        setConfirm(false);
        notify(
          "Rules or possible duplicates have changed. Check these transactions again.",
          true,
        );
      }
      setPreview(result);
      setPage(next);
      setStale(false);
    } catch (e) {
      setPageError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };
  const bad = p.error_count;
  const unresolved =
    !skipAll &&
    Object.values(decisions).filter((v) => v === "keep" || v === "skip")
      .length < p.candidate_count;
  const blocked =
    busy ||
    stale ||
    loading ||
    !!pageError ||
    !p.total ||
    !!p.error ||
    p.already_imported ||
    unresolved ||
    (!!bad && !confirm);
  const commit = async () => {
    const ok = await run(
      async () => {
        const v = await api("/imports/" + p.id + "/commit", "POST", {
          confirm_valid_rows: confirm,
          decisions,
          skip_all_candidates: skipAll,
          classification_version: p.classification_version,
          ...(p.preview_version ? { preview_version: p.preview_version } : {}),
        });
        notify(
          v.inserted +
            " transactions added; " +
            v.skipped +
            " skipped. Categorized transactions are accepted and unseen.",
        );
      },
      undefined,
      (message) => {
        if (
          message.includes("preview changed") ||
          message.includes("Classification rules changed")
        )
          setStale(true);
        return false;
      },
    );
    if (ok) {
      refresh();
      onDone();
    }
    return ok;
  };
  useEffect(() => {
    register(p.id, blocked ? null : commit);
    return () => register(p.id, null);
  }, [p, decisions, skipAll, confirm, blocked]);
  return (
    <section className="panel preview">
      <details className="preview-account" open={initialOpen}>
        <summary>
          <span>
            <strong>{accountName}</strong>
            <small>
              {p.total} {p.total === 1 ? "transaction" : "transactions"} ·{" "}
              {p.candidate_count} duplicate{" "}
              {p.candidate_count === 1 ? "check" : "checks"}
              {bad ? " · " + bad + " rejected" : ""}
            </small>
          </span>
          <Badge tone={blocked ? "pending" : "good"}>
            {p.already_imported
              ? "Already added"
              : !p.total
                ? "No new transactions"
                : blocked
                  ? "Needs attention"
                  : "Ready to add"}
          </Badge>
        </summary>
        <div className="preview-content">
          <div className="section-head">
            <div>
              <h2>{accountName}</h2>
              <p className="muted">
                {p.format === "fnb-live" ? "Transactions from FNB" : p.name} ·{" "}
                {p.currency} · account ending {String(p.bank_id).slice(-4)} ·{" "}
                {p.total} transactions
              </p>
            </div>
            <Badge tone="pending">Needs attention</Badge>
          </div>
          {p.coverage && (
            <p className="footnote">
              Transactions received:{" "}
              {p.coverage.returned_start && p.coverage.returned_end
                ? `${p.coverage.returned_start} to ${p.coverage.returned_end}`
                : "No completed transactions received"}{" "}
              · {p.coverage.returned_rows} bank transactions
              {p.coverage.service_fee_rows
                ? ` · ${p.coverage.service_fee_rows} separate bank fees`
                : ""}
              . History may be incomplete.
              {p.coverage.pending_rows
                ? ` ${p.coverage.pending_rows} pending bank payments excluded.`
                : ""}
              {p.coverage.page_limit_reached
                ? " FNB returned its maximum of 150 transactions."
                : ""}
            </p>
          )}
          {p.error && (
            <div className="notice error" role="alert">
              {p.error}
            </div>
          )}
          {p.already_imported && (
            <div className="notice">
              These transactions have already been imported. They will not be
              added again.
            </div>
          )}
          {!!p.balance_date && (
            <p className="footnote">
              Bank-reported balance: {money(p.balance_cents)} as of{" "}
              {p.balance_date}.
            </p>
          )}
          {p.candidate_count > 0 && (
            <div className="notice">
              <span>
                Check possible duplicates. Similar purchases may be separate
                transactions.
              </span>
              <Button
                disabled={loading}
                onClick={() => {
                  setSkipAll(true);
                  setDecisions({});
                }}
              >
                Skip all {p.candidate_count} possible duplicates in this import
              </Button>
            </div>
          )}
          <fieldset disabled={disabled || busy} className="import-decisions">
            <div className="preview-rows" aria-busy={loading}>
              {p.rows.map((r: Row) => (
                <div className="preview-row" key={r.row}>
                  <div className="line">
                    <strong>{r.description || "Invalid row"}</strong>
                    <strong>{money(r.amount_cents)}</strong>
                  </div>
                  <small>
                    Row {r.row} · {r.date || "Invalid date"}
                  </small>
                  {r.source_component === "service_fee" && (
                    <p className="footnote">
                      Bank row {r.source_bank_row} service fee ·{" "}
                      {r.source_bank_description}
                    </p>
                  )}
                  {r.error ? (
                    <p className="error-text">{r.error}</p>
                  ) : (
                    <>
                      <div className="row-meta">
                        <span>
                          {categories.find((c) => c.id === r.category_id)
                            ?.name || "Uncategorized"}
                        </span>
                        {r.spending_group_id && (
                          <span>
                            {
                              groups.find((g) => g.id === r.spending_group_id)
                                ?.name
                            }
                          </span>
                        )}
                        {r.suggestion && <small>{r.suggestion}</small>}
                        {r.rule_conflict && (
                          <Badge tone="pending">
                            Rules disagree — check category and group
                          </Badge>
                        )}
                        {(r.rule_match_count || r.rule_matches?.length) > 1 && (
                          <RuleSuggestions
                            id={p.id}
                            row={r.row}
                            version={p.preview_version}
                            onStale={() => setStale(true)}
                          />
                        )}
                        {r.duplicate && (
                          <Badge
                            tone={
                              r.duplicate === "conflict" ? "bad" : "pending"
                            }
                          >
                            {r.duplicate === "exact_source"
                              ? "Already imported — skip"
                              : r.duplicate === "exact_id"
                                ? "Existing transaction ID — skip"
                                : r.duplicate === "conflict"
                                  ? "Conflicting transaction ID"
                                  : "Possible duplicate"}
                          </Badge>
                        )}
                      </div>
                      {r.candidates?.length > 0 && (
                        <details>
                          <summary>Compare possible duplicates</summary>
                          {r.candidates.map((c: Row, i: number) => (
                            <div className="candidate" key={i}>
                              <small>
                                {c.id
                                  ? "Existing transaction #" + c.id
                                  : c.file
                                    ? "Batch file " + c.file
                                    : "Earlier row " + c.row}
                              </small>
                              <div>
                                {c.id ? (
                                  <button
                                    type="button"
                                    className="transaction-link"
                                    onClick={() => openTransaction(c.id)}
                                  >
                                    {c.date} · {c.description} ·{" "}
                                    {money(c.amount_cents)}
                                  </button>
                                ) : (
                                  <span>
                                    {c.date} · {c.description} ·{" "}
                                    {money(c.amount_cents)}
                                  </span>
                                )}
                              </div>
                            </div>
                          ))}
                        </details>
                      )}
                      {["possible", "conflict"].includes(r.duplicate) && (
                        <Field label={"Decision for row " + r.row}>
                          <select
                            value={decisions[r.row] || (skipAll ? "skip" : "")}
                            onChange={(e) =>
                              setDecisions({
                                ...decisions,
                                [r.row]: e.target.value,
                              })
                            }
                          >
                            <option value="">Choose what to do</option>
                            <option value="skip">Skip this row</option>
                            {r.duplicate === "possible" && (
                              <option value="keep">
                                Keep as a separate transaction
                              </option>
                            )}
                          </select>
                        </Field>
                      )}
                    </>
                  )}
                </div>
              ))}
            </div>
            {stale && (
              <Button disabled={loading} onClick={() => loadPage(page)}>
                Reload preview
              </Button>
            )}
            {loading && <Loading>Loading import rows</Loading>}
            {pageError && (
              <p role="alert">
                {pageError}{" "}
                <Button onClick={() => loadPage(page)}>Retry</Button>
              </p>
            )}
            <Pagination
              page={page}
              total={p.total}
              size={50}
              loading={loading || busy}
              onChange={loadPage}
            />
            {!!bad && (
              <label className="check">
                <input
                  type="checkbox"
                  checked={confirm}
                  onChange={(e) => setConfirm(e.target.checked)}
                />
                Import valid rows and retain {bad} rejected rows in history
              </label>
            )}
          </fieldset>
          <div className="editor-actions">
            <Button
              variant="primary"
              loading={busy}
              disabled={disabled || blocked}
              onClick={async () => {
                if (await commit()) onReview();
              }}
            >
              Continue to review
            </Button>
            <span className="muted">
              Categorized transactions are accepted automatically; only missing
              categories need review.
            </span>
          </div>
        </div>
      </details>
    </section>
  );
}
