import { AccountHealth } from "../../CoreWorkflows";
import { useTransactionAccess } from "../../TransactionAccess";
import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
import {
  useAccountManagement,
  AccountActions,
  HiddenAccounts,
} from "../../AccountManagement";
import { AccountDiscovery } from "../../AccountDiscovery";
import { Plus } from "lucide-react";
import { AccountIcon } from "../../AccountIcon";
import { money } from "../../api";
import { Button, Empty, Loading, ActionMenu } from "../../ui";
import { type PageProps } from "../../shared/types";
export function Accounts(
  props: PageProps & {
    onManage: () => void;
    onTransactions: () => void;
  },
) {
  const { viewTransactions } = useTransactionAccess();
  const { data, onManage, onTransactions } = props,
    m = useAccountManagement(props),
    list = usePagedList("/accounts", props.revision);
  return (
    <>
      <div className="toolbar">
        <p className="muted">
          {list.total} {list.total === 1 ? "account" : "accounts"} · Household
          accounts contribute to the shared budget.
        </p>
        {data.user.admin && (
          <div className="toolbar-actions">
            <Button
              loading={m.refreshing()}
              disabled={!m.canRefresh || m.busy}
              onClick={() => m.refreshBank()}
            >
              Refresh balances
            </Button>
            <Button variant="primary" onClick={onTransactions}>
              Get transactions
            </Button>
            <ActionMenu label="Account management">
              <Button variant="quiet" onClick={onManage}>
                Bank connection
              </Button>
              <Button
                variant="quiet"
                onClick={() =>
                  m.edit({ name: "", bank_id: "", household: false })
                }
              >
                <Plus size={17} />
                Add account
              </Button>
            </ActionMenu>
          </div>
        )}
      </div>
      {m.loading && data.user.admin && (
        <Loading>Loading account controls</Loading>
      )}
      <section className="panel account-list">
        <ListStatus list={list} />
        {list.items.map((a) => (
          <div
            className="account-summary"
            key={a.id}
            aria-busy={!!m.refreshing(a.id, !!a.fnb_connected)}
          >
            <div className="account-identity">
              <AccountIcon type={a.account_type} />
              <div>
                <h2>
                  <button
                    type="button"
                    className="transaction-link"
                    title={"View transactions for " + a.name}
                    onClick={() => viewTransactions({ account: String(a.id) })}
                  >
                    {a.name}
                  </button>
                </h2>
                <small>
                  FNB · ••{a.bank_id.slice(-4)} ·{" "}
                  {a.household ? "Household" : "Private"} ·{" "}
                  {a.role === "editor" ? "Editor" : "View only"}
                </small>
              </div>
            </div>
            <div className="account-value">
              <strong>
                {a.balance_cents === null
                  ? "Balance unavailable"
                  : money(a.balance_cents)}
              </strong>
              <small>
                {m.refreshing(a.id, !!a.fnb_connected) ? (
                  <Loading>Updating balance</Loading>
                ) : a.balance_date ? (
                  "As of " + a.balance_date
                ) : (
                  "No bank balance recorded"
                )}
              </small>
            </div>
            {data.user.admin && (
              <AccountActions
                account={m.accounts.find((x) => x.id === a.id) || a}
                management={m}
              />
            )}
          </div>
        ))}
        {!list.loading && !list.error && !list.items.length && (
          <Empty kind="accounts" title="No accessible accounts">
            {data.user.admin
              ? "Connect FNB to discover accounts, add one manually, or import a discovery file."
              : "Ask an administrator to grant account access."}
            {data.user.admin && <Button variant="primary" onClick={onManage}>Connect FNB</Button>}
            {data.user.admin && <Button variant="quiet" onClick={() => m.edit({ name: "", bank_id: "", household: false })}>Add account manually</Button>}
          </Empty>
        )}
        <ListNavigation list={list} />
      </section>
      <details className="panel secondary-section">
        <summary>Transaction import health</summary>
        <AccountHealth
          revision={props.revision}
          notify={props.notify}
          onBanking={onManage}
          onImports={onTransactions}
        />
      </details>
      {data.user.admin && (
        <>
          <details className="panel">
            <summary>Import an account discovery file</summary>
            <p className="footnote">
              Use an account-only file from the owner-run discovery tool.
            </p>
            <AccountDiscovery {...props} />
          </details>
          <HiddenAccounts management={m} />
        </>
      )}{" "}
      {m.editor}
    </>
  );
}
