import { Button } from "../../ui";
import { notificationLabels, type Notification } from "./contracts";
export function NotificationItem({ item, busy, onChange, onSource }: {
  item: Notification; busy: boolean; onChange: (path: string) => void; onSource: (item: Notification) => void;
}) {
  return <article aria-label={item.title} className={item.read_at ? "notification-item" : "notification-item unread"}>
    <div className="section-head"><h3>{item.title}</h3><span>{item.read_at ? "Read" : "Unread"}</span></div>
    <p>{item.message}</p>
    <p className="muted">{notificationLabels[item.type] || item.type} · {item.severity} · <time dateTime={new Date(item.occurred_at * 1000).toISOString()}>{new Date(item.occurred_at * 1000).toLocaleString()}</time></p>
    <div className="actions">
      {!item.read_at && <Button disabled={busy} onClick={() => onChange(`/notifications/${item.id}/read`)}>Mark as read</Button>}
      {!!item.dismissible && <Button disabled={busy} onClick={() => onChange(`/notifications/${item.id}/dismiss`)}>Dismiss</Button>}
      {item.source_kind !== "system" && <Button disabled={busy} onClick={() => onSource(item)}>View {item.source_kind === "budget" ? "budget" : item.source_kind}</Button>}
    </div>
  </article>;
}
