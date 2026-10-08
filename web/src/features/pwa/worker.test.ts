import { expect, it } from "vitest";
import { readFileSync } from "node:fs";
const source = readFileSync(
  new URL("../../../public/push-sw.js", import.meta.url),
  "utf8",
);
function worker() {
  const handlers: Record<string, (event: any) => void> = {};
  const stores = new Map<string, Map<string, Response>>();
  const fetched: string[] = [];
  let skipped = 0,
    claimed = 0;
  const caches = {
    open: async (key: string) => {
      if (!stores.has(key)) stores.set(key, new Map());
      const store = stores.get(key)!;
      return {
        addAll: async (paths: string[]) => {
          paths.forEach((path) => store.set(path, new Response(path)));
        },
        match: async (path: string) => store.get(path),
        put: async (path: string, value: Response) => {
          store.set(path, value);
        },
      };
    },
    keys: async () => [...stores.keys()],
    delete: async (key: string) => stores.delete(key),
  };
  const self = {
    location: { origin: "https://sente.example" },
    addEventListener: (name: string, handler: any) => {
      handlers[name] = handler;
    },
    skipWaiting: () => {
      skipped++;
    },
    clients: {
      claim: async () => {
        claimed++;
      },
    },
  };
  let offline = false;
  const fetch = async (request: any) => {
    const url = typeof request === "string" ? request : request.url;
    fetched.push(url);
    if (offline) throw new Error("offline");
    return new Response("network");
  };
  new Function("self", "caches", "fetch", source)(self, caches, fetch);
  const lifecycle = async (name: string) => {
    let pending = Promise.resolve();
    handlers[name]({
      waitUntil: (value: any) => {
        pending = value;
      },
    });
    await pending;
  };
  const request = (path: string, method = "GET", mode = "cors") => {
    let response: Promise<Response> | undefined;
    handlers.fetch({
      request: {
        url: new URL(path, "https://sente.example").href,
        method,
        mode,
      },
      respondWith: (value: any) => {
        response = Promise.resolve(value);
      },
    });
    return response;
  };
  return {
    handlers,
    stores,
    caches,
    fetched,
    lifecycle,
    request,
    setOffline: () => {
      offline = true;
    },
    skipped: () => skipped,
    claimed: () => claimed,
  };
}
it("financial/session/OAuth/MCP/identity/external requests and mutations never enter cache handling", () => {
  const w = worker();
  for (const path of [
    "/api/transactions",
    "/api/me",
    "/api/logout",
    "/api/exports.csv",
    "/oauth/token",
    "/.well-known/openid-configuration",
    "/mcp/authorize",
    "/branding/icon.png",
    "/pwa/icon/512/any.png",
    "/manifest.webmanifest",
    "https://remote.example/offline.css",
    "/assets/unlisted.js",
    "/offline.css?secret=private",
  ]) {
    expect(w.request(path)).toBeUndefined();
  }
  expect(w.request("/offline.css", "POST")).toBeUndefined();
  expect(w.stores.size).toBe(0);
  expect(w.fetched).toEqual([]);
});
it("offline navigation returns only generic cached fallback and never stores HTML or private query URLs", async () => {
  const w = worker();
  await w.lifecycle("install");
  expect(
    await (await w.request("/?message=private", "GET", "navigate"))?.text(),
  ).toBe("network");
  w.setOffline();
  expect(
    await (await w.request("/?message=private", "GET", "navigate"))?.text(),
  ).toBe("/offline.html");
  expect([...w.stores.values()][0].size).toBe(2);
});
it("activation prunes only owned previous static caches and update activation requires the explicit message", async () => {
  const w = worker();
  await w.caches.open("sente-static-old");
  await w.caches.open("another-app");
  await w.lifecycle("install");
  expect(w.skipped()).toBe(0);
  await w.lifecycle("activate");
  expect(w.stores.has("sente-static-old")).toBe(false);
  expect(w.stores.has("another-app")).toBe(true);
  expect(w.claimed()).toBe(1);
  w.handlers.message({ data: { type: "untrusted" } });
  expect(w.skipped()).toBe(0);
  w.handlers.message({ data: { type: "SKIP_WAITING" } });
  expect(w.skipped()).toBe(1);
});
