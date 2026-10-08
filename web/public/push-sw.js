// Vite replaces these two values for each production build.
const CACHE = "sente-static-dev";
const ASSETS = ["/offline.html", "/offline.css"];
const STATIC = new Set(ASSETS);
self.addEventListener("install", event => {
  event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(ASSETS)));
});
self.addEventListener("activate", event => {
  event.waitUntil((async () => {
    for (const key of await caches.keys()) {
      if (key.startsWith("sente-static-") && key !== CACHE) await caches.delete(key);
    }
    await self.clients.claim();
  })());
});
self.addEventListener("message", event => {
  if (event.data?.type === "SKIP_WAITING") self.skipWaiting();
});
self.addEventListener("fetch", event => {
  const request = event.request, url = new URL(request.url);
  if (request.method !== "GET" || url.origin !== self.location.origin) return;
  // Never intercept financial, authentication, MCP, branding or install metadata.
  if (/^\/(api|oauth|mcp|\.well-known|branding|pwa)(\/|$)/.test(url.pathname) || url.pathname === "/manifest.webmanifest") return;
  if (request.mode === "navigate") {
    // HTML and queries are never cached; no authenticated app shell offline.
    event.respondWith(fetch(request, {cache:"no-store"}).catch(async () =>
      await (await caches.open(CACHE)).match("/offline.html") || Response.error()));
    return;
  }
  if (url.search || !STATIC.has(url.pathname)) return;
  event.respondWith((async () => {
    const cache = await caches.open(CACHE), cached = await cache.match(url.pathname);
    if (cached) return cached;
    const response = await fetch(request);
    if (response.ok && !response.redirected && new URL(response.url).origin === self.location.origin) await cache.put(url.pathname,response.clone());
    return response;
  })());
});

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
