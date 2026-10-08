import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";
import { notificationLabels, type Preference } from "./contracts";
export function NotificationPreferences({ notify }: Pick<PageProps, "notify">) {
  const [items, setItems] = useState<Preference[] | null>(null);
  const [loadedGeneration, setLoadedGeneration] = useState(0);
  const [failed, setFailed] = useState(false), [retry, setRetry] = useState(0);
  const alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => {
    let current = true;
    setFailed(false);
    api<{ items: Preference[] }>("/notifications/preferences")
      .then(result => { if (current) { setItems(result.items); setLoadedGeneration(v => v + 1); } })
      .catch(e => { if (current) { setFailed(true); notify(e.message, true); } });
    return () => { current = false; };
  }, [retry]);
  const { busy, run } = useTask(notify);
  return <section className="panel">
    <div className="section-head"><h2>Notification preferences</h2>
      <Button disabled={busy} onClick={() => setRetry(v => v + 1)}>Reload saved preferences</Button>
    </div>
    <p className="muted">Choose which messages appear in your notification centre. Changes apply to future messages; existing messages stay in your inbox.</p>
    {failed ? <p>Preferences could not be loaded. Reload to try again.</p> : !items ? <Loading>Loading preferences</Loading> :
      <div className="notification-preferences">{items.map(item => <PreferenceForm key={loadedGeneration + ":" + item.type + ":" + item.version}
        item={item} busy={busy} onSave={async enabled => {
          await run(async () => {
            const result = await api<{ items: Preference[] }>("/notifications/preferences", "PUT", { ...item, enabled });
            if (alive.current) setItems(result.items);
          }, "Notification preference saved");
        }} />)}</div>}
  </section>;
}
function PreferenceForm({ item, busy, onSave }: { item: Preference; busy: boolean; onSave: (enabled: boolean) => Promise<void> }) {
  const [enabled, setEnabled] = useState(item.enabled);
  return <Form className="notification-preference" onSubmit={() => onSave(enabled)}>
    <Field label={notificationLabels[item.type] || item.type} hint="In-app notifications">
      <select value={String(enabled)} disabled={busy} onChange={e => setEnabled(e.target.value === "true")}>
        <option value="true">Enabled</option><option value="false">Disabled</option>
      </select>
    </Field>
    <Button type="submit" disabled={busy || enabled === item.enabled}>Save preference</Button>
  </Form>;
}
