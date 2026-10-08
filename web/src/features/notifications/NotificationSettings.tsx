import { useState } from "react";
import { Tabs } from "../../ui";
import type { PageProps } from "../../shared/types";
import { NotificationPreferences } from "./NotificationPreferences";
import { PushDevices } from "./PushDevices";
import { NotificationDiagnostics } from "./NotificationDiagnostics";

export function NotificationSettings({ admin, notify }: Pick<PageProps, "notify"> & { admin: boolean }) {
  const [section, setSection] = useState("preferences");
  const active = admin ? section : "preferences";
  return <>
    {admin && <Tabs id="notification-settings" label="Notification sections"
      items={[{ id: "preferences", label: "Preferences" }, { id: "diagnostics", label: "Diagnostics" }]}
      value={active} onChange={setSection} />}
    <div hidden={active !== "preferences"} {...(admin ? {
      role: "tabpanel", id: "notification-settings-panel-preferences", "aria-labelledby": "notification-settings-tab-preferences",
    } : {})}>
      <NotificationPreferences notify={notify} />
      <PushDevices notify={notify} />
    </div>
    {admin && <div hidden={active !== "diagnostics"} role="tabpanel"
      id="notification-settings-panel-diagnostics" aria-labelledby="notification-settings-tab-diagnostics">
      {active === "diagnostics" && <NotificationDiagnostics notify={notify} />}
    </div>}
  </>;
}
