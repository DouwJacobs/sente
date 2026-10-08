import { NotificationCentre } from "./features/notifications/NotificationCentre";
import { NotificationNavigation } from "./features/notifications/NotificationNavigation";
import { useSession } from "./features/auth/useSession";
import { SessionScreen } from "./features/auth/SessionScreen";
import { useWorkspaceData } from "./features/workspace/useWorkspaceData";
import { useAppearance } from "./shared/useAppearance";
import { WorkspaceShell } from "./features/workspace/WorkspaceShell";
import { Dashboard } from "./features/dashboard/Dashboard";
import { type PageProps } from "./shared/types";
import { useTask } from "./shared/useTask";
import { GlobalSearch } from "./GlobalSearch";
import { PeriodNavigation } from "./CoreWorkflows";
import {
  TransactionAccess,
  useTransactionAccess,
  type TransactionScope,
} from "./TransactionAccess";
import { OAuthConsent } from "./OAuthConsent";
import { PagedSelect } from "./PagedList";
import { useEffect, useState, useRef } from "react";
import { api, setCSRF } from "./api";
import { Button, Toast, Loading, Tabs } from "./ui";
import { Transactions } from "./features/transactions/Transactions";
import { Imports } from "./features/imports/Imports";
import {
  TransactionFilters,
  emptyTransactionFilters,
  transactionFilterQuery,
} from "./TransactionFilters";
import { Budgets } from "./features/budgets/Budgets";
import { Accounts } from "./features/accounts/Accounts";
import { Categories } from "./features/categories/Categories";
import { SettingsPage } from "./features/settings/SettingsPage";
export default function App() {
  const consent = window.location.pathname === "/mcp/authorize";
  const [revision, setRevision] = useState(0);
  const [transactionTab, setTransactionTab] = useState("all"),
    [importScope, setImportScope] = useState<number[]>([]);
  const [reviewFilters, setReviewFilters] = useState({
    seen: "",
    acceptance: "needs_category",
  });
  const [transactionFilters, setTransactionFilters] = useState(
      emptyTransactionFilters,
    ),
    [transactionQuery, setTransactionQuery] = useState("");
  useEffect(() => {
    const timer = setTimeout(
      () => setTransactionQuery(transactionFilters.query),
      250,
    );
    return () => clearTimeout(timer);
  }, [transactionFilters.query]);
  const [settingsSection, setSettingsSection] = useState("general");
  const [categoryTab, setCategoryTab] = useState("categories");
  const [searchOpen, setSearchOpen] = useState(false);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearchOpen((v) => !v);
      } else if (e.key === "/" && !searchOpen) {
        const el = document.activeElement;
        if (
          el &&
          (el.tagName === "INPUT" ||
            el.tagName === "TEXTAREA" ||
            el.getAttribute("contenteditable") === "true")
        )
          return;
        e.preventDefault();
        setSearchOpen(true);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [searchOpen]);
  const [view, setView] = useState("Dashboard"),
    [period, setPeriod] = useState(""),
    [account, setAccount] = useState(""),
    [more, setMore] = useState(false),
    [unassigned, setUnassigned] = useState(false);
  const noticeSequence = useRef(0);
  const [notice, setNotice] = useState<{
    id: number;
    message: string;
    error: boolean;
  } | null>(null);
  const { theme, setTheme, resolvedTheme } = useAppearance();
  const notify = (message: string, error = false) => {
    setNotice({ id: ++noticeSequence.current, message, error });
    window.dispatchEvent(
      new CustomEvent("finance-notice", { detail: { message, error } }),
    );
  };
  const { busy: signingOut, run: signOut } = useTask(notify);
  const refresh = () => setRevision((v) => v + 1);
  const session = useSession({
    notify,
    onExpired: () => {
      setData(null);
      setCSRF("");
      setNotice({
        id: ++noticeSequence.current,
        message: "Your session expired. Please sign in again.",
        error: true,
      });
    },
  });
  const {
    user,
    setUser,
    ready,
    setup,
    setSetup,
    startupError,
    setStartupRetry,
  } = session;
  const { data, setData, refreshingData, workCounts } = useWorkspaceData({
    user,
    setUser,
    revision,
    consent,
    account,
    setAccount,
    period,
    setPeriod,
    notify,
  });
  useEffect(() => {
    document.title =
      user && data
        ? data.branding.display_name + " · Sente"
        : "Sente";
  }, [user, data?.branding.display_name]);
  if (!ready || startupError || setup || !user)
    return (
      <SessionScreen
        session={session}
        notice={notice}
        setNotice={setNotice}
        notify={notify}
        noticeSequence={noticeSequence}
      />
    );
  if (consent)
    return (
      <>
        <OAuthConsent
          notify={notify}
          onSignOut={async () => {
            await api("/logout", "POST");
            setUser(null);
            setData(null);
            setCSRF("");
          }}
        />
        {notice && (
          <Toast
            key={notice.id}
            message={notice.message}
            error={notice.error}
            autoDismiss={false}
            onDismiss={() => setNotice(null)}
          />
        )}
      </>
    );
  if (!data)
    return (
      <div className="initial">
        {notice ? (
          <>
            <p role="alert">{notice.message}</p>
            <Button onClick={refresh}>Retry</Button>
          </>
        ) : (
          <Loading>Loading accounts</Loading>
        )}
      </div>
    );
  const props = { data, revision, refresh, notify };
  const activeTransactionFilters =
    transactionTab === "review"
      ? { ...transactionFilters, ...reviewFilters }
      : transactionFilters;
  const changeTransactionFilters = (value: typeof transactionFilters) => {
    if (transactionTab === "review") {
      setReviewFilters({
        seen: value.seen || "",
        acceptance: value.acceptance || "",
      });
      setTransactionFilters({
        ...value,
        seen: transactionFilters.seen,
        acceptance: transactionFilters.acceptance,
      });
    } else setTransactionFilters(value);
  };
  const filterQuery = transactionFilterQuery({
    ...activeTransactionFilters,
    query: transactionQuery,
  });
  const clearTransactionFilters = () => {
    setTransactionFilters(emptyTransactionFilters);
    setReviewFilters({ seen: "", acceptance: "" });
    setTransactionQuery("");
    setPeriod("");
    setAccount("");
    setUnassigned(false);
    setImportScope([]);
  };
  const go = (page: string) => {
    setView(page);
    setMore(false);
    setUnassigned(false);
    if (page === "Transactions" && view !== "Transactions") {
      setPeriod("");
      setAccount("");
    }
    window.scrollTo({ top: 0 });
  };
  const openTransactions = (tab: string, ids: number[] = []) => {
    setTransactionTab(tab);
    setImportScope(ids);
    if (tab === "review" || ids.length) {
      setPeriod("");
      setAccount("");
    }
    if (tab === "review") {
      setTransactionFilters(emptyTransactionFilters);
      setTransactionQuery("");
      setReviewFilters({ seen: "", acceptance: "needs_category" });
    }
    go("Transactions");
  };
  const openBanking = () => {
    setSettingsSection("banking");
    go("Settings");
  };
  const viewTransactions = (scope: TransactionScope) => {
    setTransactionTab("all");
    setImportScope([]);
    setTransactionFilters({
      ...emptyTransactionFilters,
      category: scope.category || "",
      group: scope.group || "",
      date_from: scope.dateFrom || "",
      date_to: scope.dateTo || "",
      excluded: scope.excluded ? "1" : "",
    });
    setTransactionQuery("");
    go("Transactions");
    setPeriod(scope.period || "");
    setAccount(scope.account || "");
    setUnassigned(false);
  };
  const navigateFromSearch = (page: string, tab?: string) => {
    if (page === "Categories" && tab) setCategoryTab(tab);
    go(page);
  };
  const current =
    !user.budget_member && view === "Dashboard" ? "Accounts" : view;
  return (
    <TransactionAccess {...props} viewTransactions={viewTransactions}>
      <GlobalSearchWrapper
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        onNavigate={navigateFromSearch}
        notify={notify}
        refresh={refresh}
        data={data}
        revision={revision}
      />
      <WorkspaceShell
        user={user}
        data={data}
        current={current}
        refreshingData={refreshingData}
        resolvedTheme={resolvedTheme}
        setTheme={setTheme}
        more={more}
        setMore={setMore}
        go={(page) => page === "Transactions" ? openTransactions("all") : go(page)}
        onReview={() => openTransactions("review")}
        reviewActive={current === "Transactions" && transactionTab === "review"}
        signingOut={signingOut}
        onAbout={() => {
          setSettingsSection("about");
          go("Settings");
        }}
        notificationNavigation={<NotificationNavigation active={current === "Notifications"} onOpen={() => go("Notifications")} />}
        onSearch={() => setSearchOpen(true)}
        onSignOut={() =>
          signOut(async () => {
            await api("/logout", "POST");
            setUser(null);
            setData(null);
            setCSRF("");
          })
        }
      >
        {current === "Transactions" && (
          <Tabs
            id="transactions"
            label="Transaction workspace"
            items={[
              { id: "all", label: "All transactions" },
              {
                id: "review",
                label: "Needs review (" + workCounts.review + ")",
              },
              {
                id: "imports",
                label: "Import activity (" + workCounts.imports + ")",
              },
            ]}
            value={transactionTab}
            onChange={(tab) => {
              setTransactionTab(tab);
              setImportScope([]);
              setUnassigned(false);
              if (tab === "review") {
                setPeriod("");
                setAccount("");
                setTransactionFilters(emptyTransactionFilters);
                setTransactionQuery("");
                setReviewFilters({
                  seen: "",
                  acceptance: "needs_category",
                });
              }
            }}
          />
        )}
        {current === "Dashboard" && user.budget_member && (
          <div className="context-bar dashboard-scope" aria-label="Dashboard scope">
            {user.budget_member && (
              <div className="context-filter">
                <PagedSelect
                  url="/periods"
                  label="Budget period"
                  hint={
                    data.periods.find((p) => String(p.id) === period)
                      ? data.periods.find((p) => String(p.id) === period)!
                          .start_date +
                        " – " +
                        data.periods.find((p) => String(p.id) === period)!
                          .end_date
                      : undefined
                  }
                  optionLabel={(p) => p.name}
                  value={period}
                  onChange={setPeriod}
                  options={data.periods}
                  empty={
                    current === "Dashboard" ? "Current period" : "All periods"
                  }
                  revision={revision}
                />
              </div>
            )}
            <div className="context-filter">
              <PagedSelect
                url="/accounts"
                label="Accounts"
                optionLabel={(a) => a.name + (a.household ? "" : " · Private")}
                value={account}
                onChange={setAccount}
                options={data.accounts}
                empty={
                  current === "Dashboard"
                    ? "Household accounts"
                    : "All accessible accounts"
                }
                revision={revision}
              />
            </div>
            <PeriodNavigation
              compact
              period={period}
              revision={revision}
              notify={notify}
              onChange={setPeriod}
            />
          </div>
        )}
        {current === "Budgets" && user.budget_member && (
          <PeriodNavigation
            period={period}
            revision={revision}
            notify={notify}
            onChange={setPeriod}
          />
        )}
        {current === "Transactions" && (
          <TransactionFilters
            data={data}
            revision={revision}
            value={activeTransactionFilters}
            onChange={changeTransactionFilters}
            onClear={clearTransactionFilters}
            active={
              !!(
                transactionFilterQuery(activeTransactionFilters) ||
                account ||
                period ||
                unassigned ||
                importScope.length
              )
            }
            imports={transactionTab === "imports"}
            account={account}
            period={period}
            onAccountChange={setAccount}
            onPeriodChange={setPeriod}
            unassigned={unassigned}
            onUnassignedChange={setUnassigned}
          />
        )}
        {notice && (
          <Toast
            key={notice.id}
            message={notice.message}
            error={notice.error}
            onDismiss={() => setNotice(null)}
          />
        )}
        {current === "Dashboard" && (
          <Dashboard
            {...props}
            period={period}
            account={account}
            onReview={() => openTransactions("review")}
            onUnassigned={() => {
              openTransactions("all");
              setUnassigned(true);
            }}
            onImport={() => openTransactions("imports")}
            stagedCount={workCounts.imports}
            onAccounts={() => go("Accounts")}
          />
        )}
        {current === "Transactions" && (
          <div
            role="tabpanel"
            id={"transactions-panel-" + transactionTab}
            aria-labelledby={"transactions-tab-" + transactionTab}
          >
            {transactionTab === "imports" ? (
              <Imports
                {...props}
                filterQuery={transactionFilterQuery(
                  {
                    ...transactionFilters,
                    query: transactionQuery,
                    seen: "",
                    acceptance: "",
                  },
                  account,
                )}
                onClearFilters={clearTransactionFilters}
                onReview={(ids) => openTransactions("review", ids)}
                onTransactions={(ids) => openTransactions("all", ids)}
                onBanking={openBanking}
              />
            ) : (
              <>
                {importScope.length > 0 && (
                  <div className="toolbar">
                    <Button onClick={() => setImportScope([])}>
                      Show all imports
                    </Button>
                  </div>
                )}
                <Transactions
                  {...props}
                  filterQuery={filterQuery}
                  review={transactionTab === "review"}
                  period={period}
                  account={account}
                  importIds={importScope}
                  onClearFilters={clearTransactionFilters}
                  onImport={() => openTransactions("imports")}
                  stagedCount={workCounts.imports}
                  unassigned={unassigned}
                />
              </>
            )}
          </div>
        )}
        {current === "Budgets" && (
          <Budgets
            {...props}
            period={period}
            account={account}
            onDashboard={(id) => {
              setPeriod(String(id));
              setAccount("");
              go("Dashboard");
            }}
          />
        )}
        {current === "Accounts" && (
          <Accounts
            {...props}
            onManage={openBanking}
            onTransactions={() => openTransactions("imports")}
          />
        )}
        {current === "Categories" && (
          <Categories {...props} onAccounts={() => go("Accounts")} />
        )}
        {current === "Notifications" && <NotificationCentre notify={notify}
          onPreferences={() => { setSettingsSection("notifications"); go("Settings"); }}
          onSource={(kind, id) => {
            if (kind === "budget") { go("Budgets"); setPeriod(String(id)); setAccount(""); }
            else viewTransactions({ account: String(id) });
          }} />}
        {current === "Settings" && (
          <SettingsPage
            {...props}
            theme={theme}
            onTheme={setTheme}
            section={settingsSection}
            onSectionChange={setSettingsSection}
            onAccounts={() => go("Accounts")}
            onTransactions={() => openTransactions("imports")}
          />
        )}
      </WorkspaceShell>
    </TransactionAccess>
  );
}
function GlobalSearchWrapper({
  open,
  onClose,
  onNavigate,
  notify,
  refresh,
  data,
  revision,
}: {
  open: boolean;
  onClose: () => void;
  onNavigate: (page: string, tab?: string) => void;
  notify: PageProps["notify"];
  refresh: () => void;
  data: PageProps["data"];
  revision?: number;
}) {
  const { openTransaction, viewTransactions } = useTransactionAccess();
  return (
    <GlobalSearch
      open={open}
      onClose={onClose}
      onNavigate={onNavigate}
      openTransaction={openTransaction}
      viewTransactions={viewTransactions}
      notify={notify}
      refresh={refresh}
      data={data}
      revision={revision}
    />
  );
}
