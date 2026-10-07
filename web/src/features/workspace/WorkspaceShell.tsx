import type { ReactNode } from "react";
import {
  LayoutDashboard,
  ArrowLeftRight,
  Wallet,
  ChartNoAxesCombined,
  Tags,
  Settings,
  LogOut,
  Menu,
  Search,
  Sun,
  Moon,
} from "lucide-react";
import { PixelMark } from "../../PixelScene";
import { Button, PageHeader } from "../../ui";
import type { Data, Row } from "../../shared/types";
const nav = [
  { name: "Dashboard", icon: LayoutDashboard },
  { name: "Transactions", icon: ArrowLeftRight },
  { name: "Accounts", icon: Wallet },
  { name: "Budgets", icon: ChartNoAxesCombined },
  { name: "Categories", icon: Tags },
  { name: "Settings", icon: Settings },
];
const descriptions: Record<string, string> = {
  Dashboard: "Income, spending, and review for your selected period.",
  Transactions: "Import, categorize, and review your transactions.",
  Review: "Check the details before approving.",
  Imports: "Get bank transactions or upload a statement for review.",
  Accounts: "Shared household accounts and private accounts.",
  Budgets: "Category limits and the dates that work for you.",
  Categories: "Organize categories, spending groups, and automatic rules.",
  Settings: "Household preferences, access, and backups.",
};
export function WorkspaceShell({
  user,
  data,
  current,
  refreshingData,
  resolvedTheme,
  setTheme,
  more,
  setMore,
  go,
  signingOut,
  onSignOut,
  onSearch,
  children,
}: {
  user: Row;
  data: Data;
  current: string;
  refreshingData: boolean;
  resolvedTheme: string;
  setTheme: (theme: string) => void;
  more: boolean;
  setMore: (more: boolean) => void;
  go: (view: string) => void;
  signingOut: boolean;
  onSignOut: () => void;
  onSearch: () => void;
  children: ReactNode;
}) {
  const visibleNav = nav.filter(
    (n) => user.budget_member || !["Dashboard", "Budgets"].includes(n.name),
  );
  return (
    <div className="shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <PixelMark />
          </span>
          <div className="brand-text">
            <span title={data.branding.display_name}>
              {data.branding.display_name}
            </span>
            <small>Sente</small>
          </div>
        </div>
        <nav aria-label="Main navigation">
          {visibleNav.map((n) => (
            <button
              key={n.name}
              className={current === n.name ? "nav-item active" : "nav-item"}
              aria-current={current === n.name ? "page" : undefined}
              onClick={() => go(n.name)}
            >
              <n.icon size={19} />
              {n.name}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span className="dot" />
          ZAR · South Africa<small>Self-hosted. Your data stays here.</small>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <span className="mobile-brand" title={data.branding.display_name}>
            {data.branding.display_name}
          </span>
          <span className="desktop-label">Personal finance</span>
          <div className="top-actions">
            <Button
              variant="secondary"
              className="topbar-search-btn"
              onClick={() => onSearch()}
              aria-label="Search workspace"
            >
              <Search size={15} aria-hidden="true" />
              <span>Search</span>
            </Button>
            <span className="username">{user.username}</span>
            <Button
              variant="quiet"
              className="topbar-icon"
              aria-label={
                resolvedTheme === "dark"
                  ? "Switch to light theme"
                  : "Switch to dark theme"
              }
              title={
                resolvedTheme === "dark"
                  ? "Switch to light theme"
                  : "Switch to dark theme"
              }
              onClick={() =>
                setTheme(resolvedTheme === "dark" ? "light" : "dark")
              }
            >
              {resolvedTheme === "dark" ? (
                <Sun size={18} aria-hidden="true" />
              ) : (
                <Moon size={18} aria-hidden="true" />
              )}
            </Button>
            <Button
              variant="quiet"
              aria-label="Sign out"
              loading={signingOut}
              onClick={() => onSignOut()}
            >
              <LogOut size={18} />
            </Button>
          </div>
        </header>
        <main id="main-content" tabIndex={-1}>
          <PageHeader
            title={current}
            description={descriptions[current]}
            workspace={data.branding.display_name}
            loading={refreshingData}
          />
          {children}
        </main>
      </div>
      <nav className="mobile-nav" aria-label="Mobile navigation">
        {visibleNav
          .filter((n) =>
            ["Dashboard", "Transactions", "Accounts"].includes(n.name),
          )
          .map((n) => (
            <button
              key={n.name}
              className={current === n.name ? "active" : ""}
              onClick={() => go(n.name)}
              aria-current={current === n.name ? "page" : undefined}
            >
              <n.icon size={21} />
              <span>{n.name}</span>
            </button>
          ))}
        <button onClick={() => setMore(!more)} aria-expanded={more}>
          <Menu size={21} />
          <span>More</span>
        </button>
      </nav>
      {more && (
        <div className="mobile-more">
          <nav aria-label="More pages">
            {visibleNav
              .filter(
                (n) =>
                  !["Dashboard", "Transactions", "Accounts"].includes(n.name),
              )
              .map((n) => (
                <button key={n.name} onClick={() => go(n.name)}>
                  <n.icon size={19} />
                  {n.name}
                </button>
              ))}
          </nav>
        </div>
      )}
    </div>
  );
}
