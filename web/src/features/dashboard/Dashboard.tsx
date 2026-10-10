import type { Row, PageProps } from "../../shared/types";
import { IncomeBucket } from "../../IncomeBucket";
import { DailyGuide } from "../../CoreWorkflows";
import { SpendingTransactions } from "../../DashboardTransactions";
import { useTransactionAccess } from "../../TransactionAccess";
import { SpendingBucket } from "../../SpendingBucket";
import { useEffect, useState } from "react";
import {
  ArrowLeftRight,
  CheckCheck,
  Upload,
  Wallet,
  ArrowUpRight,
  TrendingUp,
  TrendingDown,
  Target,
} from "lucide-react";
import { api, money } from "../../api";
import { ActionMenu, Button, Field, Empty, Loading, Pagination } from "../../ui";
export function Dashboard({
  data,
  period,
  account,
  revision,
  notify,
  onReview,
  onUnassigned,
  onImport,
  stagedCount,
  onAccounts,
  onBudgets,
  onSpendingFlow,
  refresh,
}: PageProps & {
  period: string;
  account: string;
  onReview: () => void;
  onUnassigned: () => void;
  onImport: () => void;
  stagedCount: number;
  onAccounts: () => void;
  onBudgets: (id: number) => void;
  onSpendingFlow: () => void;
}) {
  const { viewTransactions } = useTransactionAccess();
  const [budgetSort, setBudgetSort] = useState("alphabetical");
  const [categoryPage, setCategoryPage] = useState(0),
    [balancePage, setBalancePage] = useState(0),
    [groupPage, setGroupPage] = useState(0);
  useEffect(() => {
    setCategoryPage(0);
    setBalancePage(0);
    setGroupPage(0);
  }, [period, account]);
  const [d, setD] = useState<Row | null>(null),
    [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    api(
      "/dashboard?period=" +
        period +
        "&account=" +
        account +
        "&category_page=" +
        categoryPage +
        "&balance_page=" +
        balancePage +
        "&group_page=" +
        groupPage +
        "&sort=" +
        budgetSort,
    )
      .then((v) => alive && setD(v))
      .catch((e) => {
        if (alive) {
          setError(e.message);
          notify(e.message, true);
        }
      })
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [
    period,
    account,
    revision,
    categoryPage,
    balancePage,
    groupPage,
    budgetSort,
  ]);
  if (loading && !d) return <Loading>Loading this period</Loading>;
  if (error) return <Empty kind="error" title="Dashboard unavailable"><p>{error}</p><Button variant="primary" onClick={refresh}>Retry dashboard</Button></Empty>;
  if (!d) return null;
  const expense = d.categories.filter(
    (c: Row) => c.kind === "expense" && (c.target_cents || c.spent_cents),
  );
  const longTotals = [d.budget_cents, d.remaining_cents, d.income_cents, d.spent_cents,
    d.income_cents - d.spent_cents].some(value => money(value).length > 14);
  return (
    <>
      {loading && <Loading>Loading this period</Loading>}
      <section className={"stats dashboard-overview" + (longTotals ? " long-values" : "")} aria-label="Period totals">
        <Stat
          tone="budget"
          label={d.has_targets ? "Budget remaining" : "Net movement"}
          value={money(
            d.has_targets ? d.remaining_cents : d.income_cents - d.spent_cents,
          )}
          hint={
            d.has_targets
              ? "Of " + money(d.budget_cents) + " in limits"
              : "Income less spending"
          }
          negative={d.has_targets && d.remaining_cents < 0}
        />
        <Stat
          tone="income"
          label="Income"
          value={money(d.income_cents)}
          hint="Transfers excluded"
        />
        <Stat
          tone="spending"
          label="Spending"
          value={money(d.spent_cents)}
          hint={"Includes " + money(d.pending_spend_cents) + " pending"}
        />
        <Stat
          tone="review"
          label="Needs categories"
          value={String(d.pending_count)}
          hint="Categorize missing allocations"
        />
      </section>
      <DailyGuide
        period={d.period}
        remaining={d.remaining_cents}
        hasTargets={d.has_targets && d.budget_cents > 0}
        account={account}
      />
      <div className="dashboard-grid">
        <section
          className="panel spending-panel"
          aria-label="Spending by group"
        >
          <div className="section-head">
            <div>
              <h2>Spending by group</h2>
              <p className="muted">Budget, spending and what is left.</p>
            </div>
            <div className="toolbar-actions spending-controls">
              <Field label="Sort budgets">
                <select
                  value={budgetSort}
                  onChange={(e) => {
                    setBudgetSort(e.target.value);
                    setCategoryPage(0);
                    setGroupPage(0);
                  }}
                >
                  <option value="alphabetical">Alphabetical</option>
                  <option value="spending">Spending: highest first</option>
                  <option value="remaining">Remaining: lowest first</option>
                </select>
              </Field>
              <ActionMenu label="Spending actions">
                {d.has_targets && d.spending_groups?.length > 0 && (
                  <Button variant="quiet" onClick={() => onBudgets(d.period.id)}>Edit budgets</Button>
                )}
                <Button variant="quiet" onClick={onSpendingFlow}>Sankey graph</Button>
              </ActionMenu>
            </div>
          </div>
          {!d.spending_groups?.length ? (
            <Empty kind="budget" title="No spending yet">
              <p>Spending for this period will appear here once transactions are imported and categorized.</p>
              <Button variant="primary" onClick={data.accounts.length ? onImport : onAccounts}>{data.accounts.length ? 'Import transactions' : 'Add an account'}</Button>
              <Button variant="quiet" onClick={() => onBudgets(d.period.id)}>Set up a budget</Button>
            </Empty>
          ) : (
            d.spending_groups.map((g: Row) => (
              <SpendingBucket
                key={period + ":" + account + ":" + groupPage + ":" + g.id}
                group={g}
                period={String(d.period.id)}
                periodName={d.period.name}
                account={account}
                groupPage={groupPage}
                sort={budgetSort}
                hasTargets={d.has_targets}
                notify={notify}
                revision={revision}
                canEditBudget={data.user.budget_member}
                refresh={refresh}
              />
            ))
          )}
          <IncomeBucket
            key={String(d.period.id) + ":" + account}
            value={d}
            account={account}
            revision={revision}
            notify={notify}
          />
          <Pagination
            page={groupPage}
            total={d.group_total || 0}
            loading={loading}
            onChange={setGroupPage}
          />
          <details className="category-limits-summary">
            <summary>
              {d.has_targets
                ? "Category totals across all groups"
                : "Category totals across all groups"}
            </summary>
            <p className="footnote">
              {d.has_targets
                ? "Category totals combine the separate budgets and spending in each group."
                : "Totals cover the selected account only."}
            </p>
            {!expense.length ? (
              <Empty kind="categories" title="No category spending yet">
                <p>Categorized expenses for the selected accounts and period will appear here.</p>
                <Button onClick={onReview}>Review categories</Button>
              </Empty>
            ) : (
              <>
                <div className="category-rows">
                  {expense.map((c: Row) => (
                    <div className="category-row" key={c.id}>
                      <div>
                        <strong>{c.name}</strong>
                        {c.pending_cents > 0 && (
                          <small>{money(c.pending_cents)} pending</small>
                        )}
                      </div>
                      <div className="category-value">
                        <strong>{money(c.spent_cents)}</strong>
                        {d.has_targets && (
                          <small>of {money(c.target_cents)}</small>
                        )}
                        {d.has_targets &&
                          c.target_cents > 0 &&
                          c.spent_cents > c.target_cents && (
                            <small className="negative">
                              {money(c.spent_cents - c.target_cents)} over limit
                            </small>
                          )}
                      </div>
                      {d.has_targets && c.target_cents > 0 && (
                        <progress
                          className={
                            c.spent_cents > c.target_cents
                              ? "over-budget"
                              : undefined
                          }
                          aria-label={c.name + " budget used"}
                          max={c.target_cents}
                          value={Math.max(0, c.spent_cents)}
                        />
                      )}
                      <SpendingTransactions
                        label={c.name + " transactions across all groups"}
                        scope={{
                          category: String(c.id),
                          period: String(d.period.id),
                          account,
                        }}
                        revision={revision}
                        notify={notify}
                      />
                    </div>
                  ))}
                </div>
              </>
            )}
            <div className="panel-footer">
              <Pagination
                page={categoryPage}
                total={d.category_total}
                loading={loading}
                onChange={setCategoryPage}
              />
            </div>
          </details>
          <div className="panel-footer">
            {!!d.uncategorized_count && (
              <p className="footnote">
                {d.uncategorized_count}{" "}
                {d.uncategorized_count === 1
                  ? "transaction still needs"
                  : "transactions still need"}{" "}
                categories; their amounts are included in totals.
              </p>
            )}
          </div>
        </section>
        <div className="dashboard-support">
          <section className="panel" aria-label="Next steps">
            <h2>Next steps</h2>
            {!data.accounts.length && (
              <button className="action-row" onClick={onAccounts}>
                <Wallet size={20} />
                <span>
                  <strong>
                    {data.user.admin
                      ? "Set up accounts"
                      : "View account access"}
                  </strong>
                  <small>
                    {data.user.admin
                      ? "Connect FNB, discover accounts, or add one manually"
                      : "Ask an administrator to grant account access"}
                  </small>
                </span>
                <ArrowUpRight size={17} />
              </button>
            )}
            {stagedCount > 0 && (
              <button className="action-row" onClick={onImport}>
                <Upload size={20} />
                <span>
                  <strong>Resolve import issues</strong>
                  <small>
                    {stagedCount}{" "}
                    {stagedCount === 1
                      ? "account import needs"
                      : "account imports need"}{" "}
                    attention
                  </small>
                </span>
                <ArrowUpRight size={17} />
              </button>
            )}
            {d.pending_count > 0 && (
              <button className="action-row" onClick={onReview}>
                <CheckCheck size={20} />
                <span>
                  <strong>Review transactions</strong>
                  <small>{d.pending_count} need categories</small>
                </span>
                <ArrowUpRight size={17} />
              </button>
            )}
            {data.accounts.length > 0 && !stagedCount && (
              <button className="action-row" onClick={onImport}>
                <Upload size={20} />
                <span>
                  <strong>Import transactions</strong>
                  <small>Get transactions from FNB or upload a statement</small>
                </span>
                <ArrowUpRight size={17} />
              </button>
            )}
            {d.unassigned_count > 0 && (
              <button className="action-row" onClick={onUnassigned}>
                <ArrowLeftRight size={20} />
                <span>
                  <strong>Assign budget periods</strong>
                  <small>
                    {d.unassigned_count}{" "}
                    {d.unassigned_count === 1 ? "transaction" : "transactions"}{" "}
                    outside date ranges
                  </small>
                </span>
                <ArrowUpRight size={17} />
              </button>
            )}
          </section>
          <section className="panel" aria-label="Bank-reported balances">
            <h2>Bank-reported balances</h2>
            {!d.balances.length ? (
              <Empty kind="accounts"
                title={
                  data.accounts.length
                    ? "No balances for these accounts"
                    : "No accounts yet"
                }
              >
                {data.accounts.length
                  ? "Choose another account or refresh balances on the Accounts page."
                  : "Set up an account to begin."}
                <Button variant="primary" onClick={onAccounts}>Open Accounts</Button>
              </Empty>
            ) : (
              d.balances.map((a: Row) => (
                <div className="balance-row" key={a.id}>
                  <div>
                    <button
                      type="button"
                      className="transaction-link"
                      onClick={() =>
                        viewTransactions({ account: String(a.id) })
                      }
                    >
                      <strong>{a.name}</strong>
                    </button>
                    <small>
                      {a.balance_date
                        ? "As of " + a.balance_date
                        : "No balance imported"}
                      {a.household ? "" : " · Private"}
                    </small>
                  </div>
                  <strong>
                    {a.balance_cents === null ? "—" : money(a.balance_cents)}
                  </strong>
                </div>
              ))
            )}
            <Pagination
              page={balancePage}
              total={d.balance_total}
              loading={loading}
              onChange={setBalancePage}
            />
            <p className="footnote">
              Balances reflect the date shown. Transaction history may be
              incomplete.
            </p>
          </section>
        </div>
      </div>
    </>
  );
}
function Stat({
  label,
  value,
  hint,
  negative = false,
  tone,
}: {
  label: string;
  value: string;
  hint: string;
  negative?: boolean;
  tone: "income" | "spending" | "budget" | "review";
}) {
  const Icon = {
    income: TrendingUp,
    spending: TrendingDown,
    budget: Target,
    review: CheckCheck,
  }[tone];
  return (
    <div className={"stat " + tone}>
      <div className="stat-heading">
        <span>{label}</span>
        <span className="stat-icon" aria-hidden="true">
          <Icon size={18} />
        </span>
      </div>
      <strong className={negative ? "negative" : ""}>{value}</strong>
      <small>{hint}</small>
    </div>
  );
}
