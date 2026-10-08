import { useEffect, useState } from "react";
import { Bell } from "lucide-react";
import { api } from "../../api";
import { Button } from "../../ui";
export function refreshNotifications() {
  window.dispatchEvent(new Event("notifications-changed"));
}
export function NotificationNavigation({ onOpen, active }: { onOpen: () => void; active: boolean }) {
  const [count, setCount] = useState<number | null>(null);
  useEffect(() => {
    let alive = true, generation = 0;
    const refresh = () => {
      if (document.hidden) return;
      const current = ++generation;
      api<{ unread_count: number }>("/notifications/unread-count")
        .then(result => { if (alive && current === generation) setCount(result.unread_count); })
        .catch(() => { if (alive && current === generation) setCount(null); });
    };
    refresh();
    const timer = window.setInterval(refresh, 30000);
    window.addEventListener("focus", refresh);
    window.addEventListener("notifications-changed", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      alive = false;
      clearInterval(timer);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("notifications-changed", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, []);
  return <Button variant="quiet" className="topbar-icon notification-trigger"
    aria-label={count === null ? "Notifications, unread count unavailable" : `Notifications, ${count} unread`}
    aria-current={active ? "page" : undefined} onClick={onOpen}>
    <Bell size={18} aria-hidden="true" />
    {count !== null && count > 0 && <span className="notification-count" aria-hidden="true">{count > 99 ? "99+" : count}</span>}
  </Button>;
}
