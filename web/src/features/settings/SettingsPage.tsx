import { BrandingSettings } from "./BrandingSettings";
import { PWASettings } from "./PWASettings";
import { NotificationSettings } from "../notifications/NotificationSettings";
import { ConfigurationSettings } from "./ConfigurationSettings";
import { AboutSettings } from "./AboutSettings";
import { GeneralSettings } from "./GeneralSettings";
import { SecuritySettings } from "./SecuritySettings";
import { UserAccessSettings } from "./UserAccessSettings";
import { UserDialogs } from "./UserDialogs";
import { useUserAccess } from "./useUserAccess";
import { MCPSettings } from "../../MCPSettings";
import { NetworkSettings } from "../../NetworkSettings";
import { FNBConnection } from "../../FNBConnection";
import { useEffect, useState } from "react";
import { api } from "../../api";
import { Button, Tabs, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import { type PageProps, type Row } from "../../shared/types";
export function SettingsPage({
  data,
  revision,
  refresh,
  notify,
  theme,
  onTheme,
  section,
  onSectionChange,
  onAccounts,
  onTransactions,
}: PageProps & {
  onAccounts: () => void;
  onTransactions: () => void;
  theme: string;
  onTheme: (v: string) => void;
  section: string;
  onSectionChange: (v: string) => void;
}) {
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [notificationsVisited, setNotificationsVisited] = useState(section === "notifications");
  useEffect(() => { if (section === "notifications") setNotificationsVisited(true); }, [section]);
  const [mcpVisited, setMCPVisited] = useState(section === "mcp");
  useEffect(() => {
    if (section === "mcp") setMCPVisited(true);
  }, [section]);
  const [startDay, setStartDay] = useState("20"),
    [users, setUsers] = useState<Row[]>([]),
    [backups, setBackups] = useState<Row>({ items: [] }),
    [manageAccounts, setManageAccounts] = useState<Row[]>([]);
  const { busy, run } = useTask(notify);
  const access = useUserAccess(data, revision);
  useEffect(() => {
    let alive = true;
    setSettingsLoading(true);
    Promise.all([
      data.user.budget_member
        ? api("/settings")
        : Promise.resolve({ start_day: 20 }),
      data.user.admin
        ? api("/users?page=0&page_size=100")
        : Promise.resolve([]),
      data.user.admin ? Promise.resolve({ items: [] }) : Promise.resolve([]),
      data.user.admin ? api("/backups") : Promise.resolve({ items: [] }),
      data.user.admin
        ? api("/accounts/manage?page=0&page_size=100")
        : Promise.resolve([]),
    ])
      .then(([s, u, _g, b, a]) => {
        if (alive) {
          setStartDay(String(s.start_day));
          setUsers(u.items || []);
          setBackups(b);
          setManageAccounts(a.items || []);
        }
      })
      .catch((e) => alive && notify(e.message, true))
      .finally(() => alive && setSettingsLoading(false));
    return () => {
      alive = false;
    };
  }, [revision]);
  const tabs = [
    { id: "general", label: "General" },
    ...(data.user.admin
      ? [
          { id: "branding", label: "Branding" },
          ...(data.user.budget_member
            ? [{ id: "configuration", label: "Configuration" }]
            : []),
          { id: "banking", label: "Banking" },
          { id: "accounts", label: "Accounts" },
          { id: "access", label: "Users & access" },
          { id: "backups", label: "Backups" },
          { id: "network", label: "Network" },
        ]
      : []),
    { id: "pwa", label: "PWA" },
    { id: "mcp", label: "MCP" },
    { id: "notifications", label: "Notifications" },
    { id: "security", label: "Security" },
    { id: "about", label: "About" },
  ];
  const active = tabs.some((tab) => tab.id === section) ? section : "general";
  const panel = (id: string) => ({
    id: "settings-panel-" + id,
    role: "tabpanel",
    "aria-labelledby": "settings-tab-" + id,
    hidden: active !== id,
    className: "settings-panel",
  });
  return (
    <>
      {settingsLoading && <Loading>Loading settings</Loading>}
      <Tabs
        id="settings"
        label="Settings sections"
        overflowNavigation
        items={tabs}
        value={active}
        onChange={onSectionChange}
      />
      <div {...panel("general")}>
        <GeneralSettings
          data={data}
          refresh={refresh}
          theme={theme}
          onTheme={onTheme}
          startDay={startDay}
          setStartDay={setStartDay}
          busy={busy}
          run={run}
        />
      </div>
      {data.user.admin && <div {...panel("branding")}><BrandingSettings data={data} revision={revision} refresh={refresh} notify={notify} /></div>}
      <div {...panel("pwa")}><PWASettings data={data} revision={revision} refresh={refresh} notify={notify} /></div>
      <div {...panel("mcp")}>
        {(mcpVisited || active === "mcp") && (
          <MCPSettings notify={notify} refresh={refresh} />
        )}
      </div>
      <div {...panel("notifications")}>
        {(notificationsVisited || active === "notifications") && <NotificationSettings admin={!!data.user.admin} notify={notify} />}
      </div>
      <div {...panel("security")}>
        <SecuritySettings busy={busy} run={run} />
      </div>
      <div {...panel("about")}>
        {active === "about" && <AboutSettings notify={notify} />}
      </div>
      {data.user.admin && (
        <>
          {data.user.budget_member && (
            <div {...panel("configuration")}>
              <ConfigurationSettings notify={notify} refresh={refresh} />
            </div>
          )}
          <div {...panel("banking")}>
            {active === "banking" && (
              <FNBConnection
                onAccounts={onAccounts}
                onTransactions={onTransactions}
                data={data}
                revision={revision}
                refresh={refresh}
                notify={notify}
              />
            )}
          </div>
          <div {...panel("network")}>
            {active === "network" && (
              <NetworkSettings
                data={data}
                revision={revision}
                refresh={refresh}
                notify={notify}
              />
            )}
          </div>
          <div {...panel("accounts")}>
            {active === "accounts" && (
              <section className="panel">
                <h2>Manage your accounts</h2>
                <p className="muted">
                  Account discovery, names, sharing, balances and visibility are
                  together on Accounts.
                </p>
                <Button onClick={onAccounts}>Open Accounts</Button>
              </section>
            )}
          </div>
          <div {...panel("access")}>
            <UserAccessSettings
              access={access}
              users={users}
              manageAccounts={manageAccounts}
              revision={revision}
              refresh={refresh}
              busy={busy}
              run={run}
            />
          </div>
          <div {...panel("backups")}>
            <section className="panel">
              <div className="section-head">
                <h2>Backups</h2>
                <Button
                  loading={busy}
                  disabled={busy}
                  onClick={async () => {
                    if (
                      await run(
                        () => api("/backups", "POST"),
                        "Backup completed and integrity checked",
                      )
                    )
                      refresh();
                  }}
                >
                  Back up now
                </Button>
              </div>
              <p className="muted">
                A backup is made every 24 hours. The latest 14 successful
                backups are kept.
              </p>
              {backups.error && (
                <p role="alert" className="error-text">
                  {backups.error}
                </p>
              )}
              {backups.items.length ? (
                <p>
                  Last successful backup:{" "}
                  <strong>{backups.items[0].created_at} UTC</strong>
                </p>
              ) : (
                <p>No successful backup recorded yet.</p>
              )}
              <p className="footnote">
                Restore with the service stopped, using the documented command.
                A restore preserves the previous database and signs out old
                sessions. Store backup copies on a separate device.
              </p>
            </section>
          </div>
        </>
      )}
      <UserDialogs access={access} refresh={refresh} busy={busy} run={run} />
    </>
  );
}
