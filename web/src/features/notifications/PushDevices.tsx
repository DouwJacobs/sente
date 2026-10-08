import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { Button, Loading } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";

type Device = { id: number; endpoint_hash: string; enabled: number; created_at: number };
type PushSettings = { public_key: string; items: Device[] };
function publicKey(value: string): Uint8Array<ArrayBuffer> {
  const raw = atob(value.replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(raw, char => char.charCodeAt(0));
}
async function endpointHash(value: string) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, "0")).join("");
}
export function PushDevices({ notify }: Pick<PageProps, "notify">) {
  const supported = window.isSecureContext && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
  const [settings, setSettings] = useState<PushSettings | null>(null);
  const [currentHash, setCurrentHash] = useState("");
  const [loading, setLoading] = useState(true), [failed, setFailed] = useState(false), [revision, setRevision] = useState(0);
  const alive = useRef(true);
  const { busy, run } = useTask(notify);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => {
    let current = true;
    setLoading(true); setFailed(false);
    (async () => {
      const result = await api<PushSettings>("/notifications/push");
      let hash = "";
      if (supported) {
        const registration = await navigator.serviceWorker.getRegistration("/");
        const subscription = await registration?.pushManager.getSubscription();
        if (subscription) hash = await endpointHash(subscription.endpoint);
      }
      if (current) { setSettings(result); setCurrentHash(hash); }
    })().catch(error => { if (current) { setFailed(true); notify(error.message, true); } })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [revision]);
  const reload = () => { if (alive.current) setRevision(value => value + 1); };
  const enable = async () => {
    await run(async () => {
      const permission = await Notification.requestPermission();
      if (permission !== "granted") throw new Error("Browser notifications were not allowed. You can change this in browser settings.");
      const setup = await api<{ public_key: string }>("/notifications/push/subscriptions", "POST", { setup: true });
      const existing = await navigator.serviceWorker.getRegistration("/");
      if (existing && !existing.active?.scriptURL.endsWith("/push-sw.js")) throw new Error("An existing service worker needs push integration before this device can be enabled.");
      const registration = await navigator.serviceWorker.register("/push-sw.js", { scope: "/", updateViaCache: "none" });
      await navigator.serviceWorker.ready;
      let subscription = await registration.pushManager.getSubscription();
      let created = false;
      if (subscription && !settings?.items.some(device => device.endpoint_hash === currentHash)) {
        await subscription.unsubscribe(); subscription = null;
      }
      const key = publicKey(setup.public_key);
      if (subscription && subscription.options.applicationServerKey) {
        const previous = new Uint8Array(subscription.options.applicationServerKey);
        if (previous.length !== key.length || !previous.every((byte, index) => byte === key[index])) {
          await subscription.unsubscribe(); subscription = null;
        }
      }
      if (!subscription) {
        try {
          subscription = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
        } catch (error) {
          if (error instanceof Error && /push service/i.test(error.message)) {
            throw new Error('This browser could not connect to its push service. If you use Brave, enable “Use Google services for push messaging” in Settings → Privacy and security. Otherwise, check that your browser or network allows push messaging. Restart the browser and try again.');
          }
          throw error;
        }
        created = true;
      }
      const serialized = subscription.toJSON();
      try { await api("/notifications/push/subscriptions", "POST", { endpoint: serialized.endpoint, keys: serialized.keys }); }
      catch (error) { if (created) await subscription.unsubscribe(); throw error; }
      reload();
    }, "This browser is enabled. Choose the Browser push types above, then send a test.");
  };
  const remove = async (device: Device) => {
    await run(async () => {
      await api(`/notifications/push/subscriptions/${device.id}`, "DELETE");
      if (supported && device.endpoint_hash === currentHash) {
        const registration = await navigator.serviceWorker.getRegistration("/");
        const subscription = await registration?.pushManager.getSubscription();
        if (subscription) await subscription.unsubscribe();
      }
      reload();
    }, "Push device removed");
  };
  const currentDevice = settings?.items.find(device => device.endpoint_hash === currentHash && device.enabled);
  return <section className="panel notification-push-panel">
    <h2>Browser push devices</h2>
    <p className="muted">Push messages say only that a notification is available. Open Sente to see the details. Each browser needs its own opt-in and receives push for one user at a time.</p>
    {!supported && <p>This browser does not support push here. In-app notifications remain available. Use HTTPS and a supported browser; iOS may require a home-screen install.</p>}
    {loading ? <Loading>Loading push devices</Loading> : failed ? <Button onClick={reload}>Retry push devices</Button> : <div className="notification-push-content">
      <div className="toolbar-actions">
        <Button variant="primary" disabled={!supported || busy || !!currentDevice} loading={busy} onClick={enable}>{currentDevice ? "This browser is enabled" : "Enable this browser"}</Button>
        <Button disabled={busy} onClick={reload}>Refresh devices</Button>
      </div>
      {!settings?.items.length ? <p>No push devices enabled.</p> : <ul className="notification-list">
        {settings.items.map(device => <li className="notification-item" key={device.id}>
          <div className="section-head">
            <span>{device.endpoint_hash === currentHash ? "This browser" : `Device ${device.id}`} · {device.enabled ? "Enabled" : "Disabled"} · Added {new Date(device.created_at * 1000).toLocaleDateString()}</span>
            <div className="toolbar-actions">
              <Button disabled={busy || !device.enabled} onClick={() => run(() => api(`/notifications/push/subscriptions/${device.id}/test`, "POST"), "Test notification queued. Check this device.")}>Send test</Button>
              <Button disabled={busy} onClick={() => remove(device)}>Remove device</Button>
            </div>
          </div>
        </li>)}
      </ul>}
    </div>}
  </section>;
}
