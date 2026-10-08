// Push-only worker: no fetch interception or financial/API/session caching.
self.addEventListener("push", event => {
  let tag = "sente-notification", url = "/?notifications=1";
  try { const data = event.data?.json(); if (typeof data?.tag === "string" && /^[a-f0-9]{32}$/.test(data.tag)) tag = data.tag; if (typeof data?.url === "string" && /^\/\?notifications=1(&message=[1-9]\d*)?$/.test(data.url)) url = data.url; } catch {}
  event.waitUntil(self.registration.showNotification("Sente", {
    body: "You have a new notification. Open Sente to view it.", tag,
    data: { url },
  }));
});
self.addEventListener("notificationclick", event => {
  event.notification.close();
  event.waitUntil((async () => {
    const link = event.notification.data?.url;
    const url = new URL(typeof link === "string" && /^\/\?notifications=1(&message=[1-9]\d*)?$/.test(link) ? link : "/?notifications=1", self.location.origin).href;
    const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    const client = windows.find(window => new URL(window.url).origin === self.location.origin);
    if (client) { await client.navigate(url); await client.focus(); }
    else await self.clients.openWindow(url);
  })());
});
