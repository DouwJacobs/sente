import { BudgetTrends, Rebalance } from "../../CoreWorkflows";
import { useTransactionAccess } from "../../TransactionAccess";
import { LimitEditor } from "../../LimitEditor";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import { useState } from "react";
import { Plus } from "lucide-react";
import { api, money } from "../../api";
import {
  Button,
  Field,
  Badge,
  Empty,
  Modal,
  validateFields,
  Tabs,
  Pagination,
  ActionMenu,
} from "../../ui";
import { useTask } from "../../shared/useTask";
import { type PageProps, type Row } from "../../shared/types";
export function Budgets({
  data,
  refresh,
  notify,
  revision,
  period,
  account,
  onDashboard,
}: PageProps & {
  period: string;
  account: string;
  onDashboard: (id: number) => void;
}) {
  const { openTransaction, viewTransactions } = useTransactionAccess();
  const [budgetTab, setBudgetTab] = useState("periods");
  const [rebalance, setRebalance] = useState<Row | null>(null);
  const [editing, setEditing] = useState<Row | null>(null),
    [preview, setPreview] = useState<Row | null>(null),
    [limits, setLimits] = useState<Row | null>(null);
  const { busy, run } = useTask(notify);
  const periods = usePagedList("/periods", revision);
  const selectedPeriod = data.periods.find((p) => String(p.id) === period);
  const visiblePeriods =
    selectedPeriod && !periods.items.some((p) => p.id === selectedPeriod.id)
      ? [selectedPeriod, ...periods.items]
      : periods.items;
  const [previewPage, setPreviewPage] = useState(0);
  const startEdit = (p: Row) => {
    setEditing({ ...p });
    setPreview(null);
    setPreviewPage(0);
  };
  const update = (key: string, value: string) => {
    setEditing((v) => (v ? { ...v, [key]: value } : null));
    setPreview(null);
    setPreviewPage(0);
  };
  const payload = () => ({
    name: editing!.name,
    start_date: editing!.start_date,
    end_date: editing!.end_date,
    version: editing!.version || 0,
    preview_token: preview?.preview_token || "",
  });
  return (
    <>
      <Tabs
        id="budgets"
        label="Budget workspace"
        items={[
          { id: "periods", label: "Budget periods" },
          { id: "trends", label: "Trends & reports" },
        ]}
        value={budgetTab}
        onChange={setBudgetTab}
      />
      <div
        role="tabpanel"
        id="budgets-panel-periods"
        className="budget-periods-panel"
        aria-labelledby="budgets-tab-periods"
        hidden={budgetTab !== "periods"}
      >
        <div className="toolbar">
          <p className="muted">
            Set limits for each spending group and category.
          </p>
          <Button
            variant="primary"
            onClick={() => startEdit({ ...data.next, version: 0 })}
          >
            <Plus size={17} />
            New period
          </Button>
        </div>
        <ListStatus list={periods} />
        {!periods.loading && !visiblePeriods.length ? (
          <section className="panel">
            <Empty title="No budget periods">
              Create your first period to start budgeting.
            </Empty>
          </section>
        ) : (
          visiblePeriods.map((p) => (
            <section className="panel" key={p.id}>
              <div className="section-head">
                <div>
                  <h2>
                    <button
                      type="button"
                      className="transaction-link"
                      title={"View transactions for " + p.name}
                      onClick={() => viewTransactions({ period: String(p.id) })}
                    >
                      {p.name}
                    </button>{" "}
                    {String(p.id) === period && <Badge>Selected period</Badge>}
                  </h2>
                  <p className="muted">
                    {p.start_date} — {p.end_date}
                  </p>
                </div>
                <div className="toolbar-actions">
                  <Button variant="primary" onClick={() => setLimits(p)}>
                    Edit budgets
                  </Button>
                  <Button variant="quiet" onClick={() => onDashboard(p.id)}>
                    View spending
                  </Button>
                  <ActionMenu label={"Actions for " + p.name}>
                    <Button variant="quiet" onClick={() => startEdit(p)}>
                      Edit dates
                    </Button>
                    <Button variant="quiet" onClick={() => setRebalance(p)}>
                      Rebalance
                    </Button>
                  </ActionMenu>
                </div>
              </div>
              <div className="budget-total">
                <span>Total budget</span>
                <strong>{money(p.target_total)}</strong>
              </div>
              {p.group_budgets
                ?.filter((g: Row) => g.target_cents > 0)
                .map((g: Row) => (
                  <div className="line" key={g.id}>
                    <span>{g.name}</span>
                    <strong>{money(g.target_cents)}</strong>
                  </div>
                ))}
              {!p.target_total && (
                <p className="footnote">No spending limits set yet.</p>
              )}
              {p.group_budgets?.length === 20 && (
                <p className="footnote">
                  Use Edit budgets to browse all groups and categories.
                </p>
              )}
            </section>
          ))
        )}
        <ListNavigation list={periods} />
      </div>
      {editing && (
        <Modal
          title={editing.id ? "Edit budget period" : "New budget period"}
          onClose={() => setEditing(null)}
        >
          <Field label="Period name">
            <input
              required
              maxLength={100}
              value={editing.name || ""}
              onChange={(e) => update("name", e.target.value)}
            />
          </Field>
          <div className="form-grid">
            <Field label="Start date">
              <input
                type="date"
                required
                value={editing.start_date || ""}
                onChange={(e) => update("start_date", e.target.value)}
              />
            </Field>
            <Field
              label="End date (inclusive)"
              validate={(value) =>
                editing.start_date && value < editing.start_date
                  ? "End date must be on or after the start date."
                  : ""
              }
            >
              <input
                type="date"
                required
                value={editing.end_date || ""}
                onChange={(e) => update("end_date", e.target.value)}
              />
            </Field>
          </div>
          <p className="muted">
            Choose any date range. If periods overlap, transactions go into the
            period that starts later. Transactions in a gap need a period chosen
            manually. Periods you chose manually stay unchanged.
          </p>
          <Button
            loading={busy}
            disabled={busy}
            onClick={(e) => {
              if (validateFields(e.currentTarget.closest(".modal-body")!))
                run(async () => {
                  setPreview(
                    await api(
                      "/periods/" +
                        (editing.id || "new") +
                        "/preview?page=0&page_size=50",
                      "POST",
                      payload(),
                    ),
                  );
                  setPreviewPage(0);
                });
            }}
          >
            Preview affected transactions
          </Button>
          {preview && (
            <div className="notice">
              <div>
                <strong>
                  {preview.total} transactions will move to a different budget
                  period.
                </strong>
                <p>
                  {preview.manual_outside} transactions will keep their chosen
                  period but fall outside its dates.
                </p>
                <details>
                  <summary>View affected transactions</summary>
                  {preview.affected.map((t: Row) => (
                    <button
                      type="button"
                      className="transaction-link"
                      key={t.id}
                      onClick={() =>
                        openTransaction(t.id, () => setPreview(null))
                      }
                    >
                      #{t.id} · {t.date} ·{" "}
                      {t.from
                        ? data.periods.find((p) => p.id === t.from)?.name ||
                          t.from
                        : "Unassigned"}{" "}
                      →{" "}
                      {t.to
                        ? data.periods.find((p) => p.id === t.to)?.name ||
                          "New period"
                        : "Unassigned"}
                    </button>
                  ))}
                  <Pagination
                    page={previewPage}
                    total={preview.total}
                    size={50}
                    loading={busy}
                    onChange={(page) =>
                      run(async () => {
                        const next = await api(
                          "/periods/" +
                            (editing.id || "new") +
                            "/preview?page=" +
                            page +
                            "&page_size=50",
                          "POST",
                          payload(),
                        );
                        setPreview(next);
                        setPreviewPage(page);
                      })
                    }
                  />
                </details>
              </div>
            </div>
          )}
          <div className="editor-actions">
            <Button
              variant="primary"
              loading={busy}
              disabled={busy || !preview}
              onClick={async () => {
                if (
                  await run(
                    () =>
                      api(
                        "/periods" + (editing.id ? "/" + editing.id : ""),
                        editing.id ? "PUT" : "POST",
                        payload(),
                      ),
                    "Budget period saved",
                  )
                ) {
                  setEditing(null);
                  refresh();
                }
              }}
            >
              Save period
            </Button>
            <Button onClick={() => setEditing(null)}>Cancel</Button>
          </div>
        </Modal>
      )}
      {rebalance && (
        <Rebalance
          period={rebalance}
          notify={notify}
          onClose={() => setRebalance(null)}
          onDone={() => {
            setRebalance(null);
            refresh();
          }}
        />
      )}
      <div
        role="tabpanel"
        id="budgets-panel-trends"
        aria-labelledby="budgets-tab-trends"
        hidden={budgetTab !== "trends"}
      >
        {budgetTab === "trends" && (
          <BudgetTrends
            data={data}
            refresh={refresh}
            revision={revision}
            notify={notify}
            period={period}
            account={account}
          />
        )}
      </div>
      {limits && (
        <Modal
          title={"Group budgets · " + limits.name}
          onClose={() => setLimits(null)}
        >
          <LimitEditor
            period={limits}
            notify={notify}
            refresh={refresh}
            revision={revision}
            onDone={() => setLimits(null)}
          />
        </Modal>
      )}
    </>
  );
}
