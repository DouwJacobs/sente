import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
const source = readFileSync(new URL("../../../public/push-sw.js", import.meta.url), "utf8");
function worker() {
  const handlers: Record<string, (event: any) => void> = {};
  const notifications: any[] = [], links: string[] = [];
  const self = {
    location: { origin: "https://sente.example" },
    addEventListener: (kind: string, fn: (event: any) => void) => { handlers[kind] = fn; },
    registration: { showNotification: async (title: string, options: any) => { notifications.push({ title, ...options }); } },
    clients: { matchAll: async () => [], openWindow: async (url: string) => { links.push(url); } },
  };
  new Function("self", source)(self);
  return { handlers, notifications, links };
}
describe("privacy-preserving push worker", () => {
  it("retains push/click integration and uses generic lock-screen text", async () => {
    const { handlers, notifications } = worker();
    expect(handlers.push).toBeTypeOf("function");
    expect(handlers.notificationclick).toBeTypeOf("function");
    let pending: Promise<unknown> = Promise.resolve();
    handlers.push({ data: { json: () => ({ title: "Private merchant", body: "Account balance secret", tag: "a".repeat(32), url: "/?notifications=1&message=12" }) }, waitUntil: (value: Promise<unknown>) => { pending = value; } });
    await pending;
    expect(notifications[0].title).toBe("Sente");
    expect(notifications[0].body).toBe("You have a new notification. Open Sente to view it.");
    expect(notifications[0].data.url).toBe("/?notifications=1&message=12");
  });
  it("refuses injected remote links and tolerates malformed provider data", async () => {
    const { handlers, notifications, links } = worker();
    let pending: Promise<unknown> = Promise.resolve();
    handlers.push({ data: { json: () => { throw new Error("invalid payload"); } }, waitUntil: (value: Promise<unknown>) => { pending = value; } });
    await pending;
    expect(notifications[0].data.url).toBe("/?notifications=1");
    handlers.notificationclick({ notification: { close: () => {}, data: { url: "https://attacker.invalid/" } }, waitUntil: (value: Promise<unknown>) => { pending = value; } });
    await pending;
    expect(links).toEqual(["https://sente.example/?notifications=1"]);
  });
});
