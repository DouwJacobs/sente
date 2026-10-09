import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { Button, Empty, Loading, Tabs } from "../../ui";
import { useTask } from "../../shared/useTask";
import type { PageProps } from "../../shared/types";
import { useTransactionAccess } from "../../TransactionAccess";
import { NotificationItem } from "./NotificationItem";
import { type Notification, type Inbox } from "./contracts";
import { refreshNotifications } from "./NotificationNavigation";
export function NotificationCentre({ notify, onPreferences, onSource }: Pick<PageProps, "notify"> & {
  onPreferences: () => void; onSource: (kind: "budget" | "account", id: number) => void;
}) {
  const [inbox, setInbox] = useState<Inbox | null>(null);
  const [linked, setLinked] = useState<Notification | null>(null);
  const linkedID = new URLSearchParams(window.location.search).get("message");
  const [state, setState] = useState(""), [page, setPage] = useState(0);
  const [revision, setRevision] = useState(0), [loading, setLoading] = useState(true), [failed, setFailed] = useState(false);
  const request = useRef(0), alive = useRef(true);
  const { busy, run } = useTask(notify);
  const { openTransaction } = useTransactionAccess();
  useEffect(() => {
    alive.current = true;
    document.querySelector<HTMLElement>("#main-content")?.focus();
    return () => { alive.current = false; ++request.current; };
  }, []);
  const reload = () => { setPage(0); setRevision(v => v + 1); refreshNotifications(); };
  useEffect(() => {
    const changed = () => { setPage(0); setRevision(v => v + 1); };
    const focused = () => { if (!document.hidden) changed(); };
    window.addEventListener("notifications-changed", changed);
    window.addEventListener("focus", focused);
    document.addEventListener("visibilitychange", focused);
    const timer = window.setInterval(focused, 30000);
    return () => {
      clearInterval(timer);
      window.removeEventListener("notifications-changed", changed);
      window.removeEventListener("focus", focused);
      document.removeEventListener("visibilitychange", focused);
    };
  }, []);
  useEffect(() => {
    const current = ++request.current;
    setLinked(null);
    if (linkedID && /^[1-9]\d*$/.test(linkedID)) {
      api<Notification>(`/notifications/push/messages/${linkedID}`)
        .then(item => { if (alive.current && current === request.current) setLinked(item); })
        .catch(error => { if (alive.current && current === request.current) { setLinked(null); notify(error.message, true); } });
    }
    setLoading(true); setFailed(false); setInbox(null);
    api<Inbox>(`/notifications?page=${page}&page_size=20&state=${state}`)
      .then(result => { if (alive.current && current === request.current) setInbox(result); })
      .catch(e => { if (alive.current && current === request.current) { setFailed(true); notify(e.message, true); } })
      .finally(() => { if (alive.current && current === request.current) setLoading(false); });
    return () => { ++request.current; };
  }, [page, state, revision]);
  const change = async (path: string) => {
    const saved = await run(() => api(path, "POST"));
    if (saved && path === `/notifications/${linkedID}/dismiss`) {
      const url = new URL(window.location.href); url.searchParams.delete("message");
      window.history.replaceState(null, "", url); setLinked(null);
    }
    if (alive.current) {
      reload(); // Recheck current privacy after successful or rejected mutations.
      document.querySelector<HTMLElement>("#main-content")?.focus();
    }
  };
  const openSource = async (item: Notification) => {
    if (item.source_kind === "transaction") { openTransaction(item.source_id); return; }
    if (item.source_kind !== "budget" && item.source_kind !== "account") return;
    const kind = item.source_kind;
    await run(async () => {
      const result = await api<{ items: { id: number }[] }>(`${kind === "budget" ? "/periods" : "/accounts"}?page=0&page_size=1&id=${item.source_id}`);
      if (!result.items.some(source => source.id === item.source_id)) throw new Error("This notification target is no longer available.");
      if (alive.current) onSource(kind, item.source_id);
    });
  };
  return <section className="panel notification-centre" aria-label="Notification centre" aria-busy={loading || busy}>
    <div className="section-head"><h2>Your notifications</h2><div className="actions">
      <Button disabled={busy || loading} onClick={reload}>Refresh notifications</Button>
      <Button disabled={busy || loading || !inbox?.unread_count} onClick={() => change("/notifications/read-all")}>Mark all as read</Button>
      <Button onClick={onPreferences}>Notification preferences</Button>
    </div></div>
    <Tabs id="notification-state" label="Notification state" items={[{ id: "", label: "All" }, { id: "unread", label: "Unread" }, { id: "read", label: "Read" }]}
      value={state} onChange={value => { setState(value); setPage(0); }} />
    {linked && !inbox?.items.some(item => item.id === linked.id) && <div aria-label="Opened push message"><NotificationItem item={linked} busy={busy} onChange={change} onSource={openSource} /></div>}
    <div role="tabpanel" id={"notification-state-panel-" + state} aria-labelledby={"notification-state-tab-" + state}>
    {loading ? <Loading>Loading notifications</Loading> : failed ? <Empty kind="error" title="Notifications could not be loaded"><p>Try refreshing your inbox.</p><Button variant="primary" onClick={reload}>Retry notifications</Button></Empty> : inbox && <>
      <p className="muted" role="status">{inbox.unread_count} unread · {inbox.total} {state || "total"} notifications</p>
      {!inbox.items.length ? <Empty kind={state === 'unread' ? 'complete' : 'start'} title="No notifications"><p>{state === 'unread' ? "You're up to date. New unread messages will appear here." : state === 'read' ? "Messages you mark as read will appear here." : "Budget alerts and account updates will appear here when there is something to know."}</p>{state && <Button onClick={() => { setState(''); setPage(0); }}>View all notifications</Button>}<Button variant="quiet" onClick={onPreferences}>Manage preferences</Button></Empty> :
      <ul className="notification-list">{inbox.items.map(item => <li key={item.id}>
        <NotificationItem item={item} busy={busy} onChange={change} onSource={openSource} />
      </li>)}</ul>}
      {inbox.total > 20 && <nav className="actions" aria-label="Notification pages">
        <Button disabled={busy || page === 0} onClick={() => setPage(v => v - 1)}>Previous notifications</Button>
        <span>Page {page + 1} of {Math.ceil(inbox.total / 20)}</span>
        <Button disabled={busy || (page + 1) * 20 >= inbox.total} onClick={() => setPage(v => v + 1)}>Next notifications</Button>
      </nav>}
    </>}
    </div>
  </section>;
}
