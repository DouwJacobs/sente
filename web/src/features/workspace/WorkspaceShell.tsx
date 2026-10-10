import { useApplicationUpdate } from "../../shared/applicationUpdate";
import { buildLabel, useBuildInfo } from "../../shared/buildInfo";
import { useEffect, useRef, type ReactNode } from "react";
import {
  LayoutDashboard,
  ArrowLeftRight,
  Wallet,
  ListChecks,
  ChartNoAxesCombined,
  Tags,
  Settings,
  LogOut,
  Menu,
  Search,
  Sun,
  Moon,
} from "lucide-react";
import { Button, PageHeader, useExitPresence } from "../../ui";
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
  Notifications: "Updates about your budgets, accounts and transactions.",
  Dashboard: "Income, spending, and review for your selected period.",
  "Sankey graph": "See how spending flows through groups and categories for your selected period.",
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
  onAbout,
  onReview,
  reviewActive,
  notificationNavigation,
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
  onAbout: () => void;
  onReview: () => void;
  reviewActive: boolean;
  notificationNavigation: ReactNode;
  children: ReactNode;
}) {
  const { info } = useBuildInfo();
  const { status: update } = useApplicationUpdate();
  const updateAvailable = update?.state === "available";
  const moreVisible = useExitPresence(more);
  const shellRef = useRef<HTMLDivElement>(null);
  const navRef = useRef<HTMLElement>(null);
  const moreRef = useRef<HTMLDivElement>(null);
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const nav = navRef.current;
    if (!nav) return;
    const measure = () => shellRef.current?.style.setProperty(
      "--mobile-nav-height", `${nav.getBoundingClientRect().height}px`,
    );
    const observer = new ResizeObserver(measure);
    observer.observe(nav);
    measure();
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!more) return;
    moreRef.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const dismiss = (event: Event) => {
      if (event.target instanceof Node && !moreRef.current?.contains(event.target)
          && !moreButtonRef.current?.contains(event.target)) setMore(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setMore(false);
        moreButtonRef.current?.focus();
      }
    };
    const resize = () => {
      if (window.innerWidth > 760) setMore(false);
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("focusin", dismiss);
    document.addEventListener("keydown", escape);
    window.addEventListener("resize", resize);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("focusin", dismiss);
      document.removeEventListener("keydown", escape);
      window.removeEventListener("resize", resize);
    };
  }, [more, setMore]);
  const navigationPage = current === "Sankey graph" ? "Dashboard" : current;
  const visibleNav = nav.filter(
    (n) => user.budget_member || !["Dashboard", "Budgets"].includes(n.name),
  );
  return (
    <div className="shell" ref={shellRef}>
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <img width={32} height={32} className="brand-icon" src={data.branding.logo || "/sente.svg"} onError={event=>{if(!event.currentTarget.src.endsWith("/sente.svg")) event.currentTarget.src="/sente.svg";}} alt="" />
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
              className={navigationPage === n.name ? "nav-item active" : "nav-item"}
              aria-current={navigationPage === n.name ? "page" : undefined}
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
          <button
            type="button"
            className="sidebar-version"
            onClick={onAbout}
            aria-label={updateAvailable ? `About Sente, update available: ${update.available_version}` : "About Sente"}
            title={info ? buildLabel(info) : "About Sente"}
          >
            {info ? (info.version === "dev" ? "Development" : info.version) : "About"}
            {updateAvailable && <span className="version-update-label">Update available</span>}
          </button>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <img width={28} height={28} className="brand-icon mobile-brand-icon" src={data.branding.logo || "/sente.svg"} onError={event=>{if(!event.currentTarget.src.endsWith("/sente.svg")) event.currentTarget.src="/sente.svg";}} alt="" />
          <span className="mobile-brand" title={data.branding.display_name}>
            {data.branding.display_name}
          </span>
          <span className="desktop-label">Personal finance</span>
          <div className="top-actions">
            {notificationNavigation}
            <Button
              variant="quiet"
              className="topbar-search-btn topbar-icon"
              onClick={() => onSearch()}
              aria-label="Search workspace"
            >
              <Search size={18} aria-hidden="true" />
            </Button>
            <span className="username" title={user.username}>{user.username}</span>
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
        <main id="main-content" tabIndex={-1} className="workspace-compact">
          <PageHeader
            title={current}
            description={descriptions[current]}
            workspace={data.branding.display_name}
            loading={refreshingData}
          />
          {children}
        </main>
      </div>
      <nav ref={navRef} className="mobile-nav" aria-label="Mobile navigation">
        {visibleNav
          .filter((n) =>
            ["Dashboard", "Transactions"].includes(n.name),
          )
          .map((n) => (
            <button
              key={n.name}
              className={navigationPage === n.name && !reviewActive ? "active" : ""}
              onClick={() => go(n.name)}
              aria-current={navigationPage === n.name && !reviewActive ? "page" : undefined}
            >
              <n.icon size={21} />
              <span>{n.name}</span>
            </button>
          ))}
        <button
          className={reviewActive ? "active" : ""}
          aria-current={reviewActive ? "page" : undefined}
          onClick={onReview}
        >
          <ListChecks size={21} />
          <span>Review</span>
        </button>
        <button ref={moreButtonRef} onClick={() => setMore(!more)} aria-expanded={more}
          aria-controls="mobile-more-pages"
          className={!["Dashboard", "Transactions"].includes(navigationPage) ? "active" : ""}>
          <Menu size={21} />
          <span>More</span>
        </button>
      </nav>
      {moreVisible && (
        <div className={"mobile-more"+(!more?" is-closing":"")} inert={!more} aria-hidden={!more} id="mobile-more-pages" ref={moreRef}>
          <nav aria-label="More pages">
            {visibleNav
              .filter(
                (n) =>
                  !["Dashboard", "Transactions"].includes(n.name),
              )
              .map((n) => (
                <button key={n.name} aria-current={navigationPage === n.name ? "page" : undefined} onClick={() => go(n.name)}>
                  <n.icon size={19} />
                  {n.name}
                  {n.name === "Settings" && updateAvailable && <small className="version-update-label">Update available</small>}
                </button>
              ))}
          </nav>
        </div>
      )}
    </div>
  );
}
