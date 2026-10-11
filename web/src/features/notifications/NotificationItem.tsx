import { CheckCheck } from "lucide-react";
import { ActionMenu, Button } from "../../ui";
import { notificationLabels, type Notification } from "./contracts";
export function NotificationItem({ item, busy, onChange, onSource }: {
  item: Notification; busy: boolean; onChange: (path: string) => void; onSource: (item: Notification) => void;
}) {
  const occurred = new Date(item.occurred_at * 1000);
  const severity = item.severity.charAt(0).toUpperCase() + item.severity.slice(1).replaceAll("_", " ");
  return <article aria-label={item.title} className={item.read_at ? "notification-item" : "notification-item unread"}>
    <div className="notification-item-heading"><h3>{item.title}</h3><span className="notification-read-state">{!item.read_at && <span className="notification-unread-dot" aria-hidden="true"/>}{item.read_at ? "Read" : "Unread"}</span></div>
    <p className="notification-message">{item.message}</p>
    <div className="notification-meta"><span>{notificationLabels[item.type] || item.type}</span><span className={"notification-severity severity-" + item.severity}>{severity}</span><time title={occurred.toLocaleString()} dateTime={occurred.toISOString()}>{new Intl.DateTimeFormat(undefined, { dateStyle:"medium", timeStyle:"short" }).format(occurred)}</time></div>
    <div className="notification-item-actions">
      {item.source_kind !== "system" && <Button disabled={busy} onClick={() => onSource(item)}>View {item.source_kind === "budget" ? "budget" : item.source_kind}</Button>}
      <div className="notification-secondary-actions">
        {!item.read_at && <Button variant="quiet" className="notification-icon-action" aria-label="Mark as read" title="Mark as read" disabled={busy} onClick={() => onChange(`/notifications/${item.id}/read`)}><CheckCheck size={18} aria-hidden="true"/></Button>}
        {!!item.dismissible && <ActionMenu label={"Notification actions for " + item.title}><Button variant="quiet" disabled={busy} onClick={() => onChange(`/notifications/${item.id}/dismiss`)}>Dismiss</Button></ActionMenu>}
      </div>
    </div>
  </article>;
}
