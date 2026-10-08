import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { Button, Field, Form, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";
import { notificationLabels, type Preference } from "./contracts";

type PreferenceSnapshot = { items: Preference[] };

export function NotificationPreferences({ notify }: Pick<PageProps, "notify">) {
  const [saved, setSaved] = useState<Preference[] | null>(null);
  const [draft, setDraft] = useState<Preference[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  const alive = useRef(true);
  const { busy, run } = useTask(notify);
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; };
  }, []);
  useEffect(() => {
    let current = true;
    setFailed(false);
    setLoading(true);
    api<PreferenceSnapshot>("/notifications/preferences?channels=all")
      .then(result => {
        if (current) {
          setSaved(result.items);
          setDraft(result.items);
        }
      })
      .catch(e => {
        if (current) { setFailed(true); notify(e.message, true); }
      })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [retry]);
  const changed = draft.filter(item => saved?.some(previous =>
    previous.type === item.type && previous.channel === item.channel && previous.enabled !== item.enabled));
  return (
    <section className="panel notification-preference-panel">
      <h2>Notification preferences</h2>
      <p className="muted">
        Choose the channels for each notification type. Browser push needs device opt-in below. Changes apply to future messages.
      </p>
      {loading ? <Loading>Loading preferences</Loading> : failed ? (
        <div>
          <p>Preferences could not be loaded. Reload to try again.</p>
          <Button disabled={busy} onClick={() => setRetry(v => v + 1)}>Reload saved preferences</Button>
        </div>
      ) : saved && (
        <Form onSubmit={async () => {
          if (!changed.length) return;
          await run(async () => {
            const result = await api<PreferenceSnapshot>("/notifications/preferences/batch?channels=all", "PUT", { items: changed });
            if (alive.current) {
              setSaved(result.items);
              setDraft(result.items);
            }
          }, "Notification preferences saved");
        }}>
          <div className="notification-preferences">
            {[...new Set(draft.map(item => item.type))].map(type => {
              const label = notificationLabels[type] || type;
              return <div className="notification-preference-row" role="group" aria-label={label} key={type}>
                <span className="notification-preference-type">{label}</span>
                {draft.filter(item => item.type === type).map(item => {
                  const channel = item.channel === "push" ? "Browser push" : "In-app";
                  return <Field key={item.channel} label={channel}>
                    <select aria-label={`${label} · ${channel}`} value={String(item.enabled)} disabled={busy}
                      onChange={e => setDraft(previous => previous.map(value =>
                        value.type === item.type && value.channel === item.channel ? { ...value, enabled: e.target.value === "true" } : value))}>
                      <option value="true">Enabled</option>
                      <option value="false">Disabled</option>
                    </select>
                  </Field>;
                })}
              </div>;
            })}
          </div>
          <div className="notification-preference-actions">
            <Button type="submit" variant="primary" loading={busy} disabled={busy || (!changed.length)}>
              Save changes
            </Button>
            <Button variant="quiet" disabled={busy || loading} onClick={() => setRetry(v => v + 1)}>
              Reload saved preferences
            </Button>
          </div>
        </Form>
      )}
    </section>
  );
}
