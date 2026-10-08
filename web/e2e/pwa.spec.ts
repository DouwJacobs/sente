import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { test, expect, type Page } from "@playwright/test";
async function login(page: Page) {
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Dashboard", exact: true }),
  ).toBeVisible();
}
async function settings(page: Page, width: number) {
  if (width < 761) {
    await page
      .getByRole("navigation", { name: "Mobile navigation" })
      .getByRole("button", { name: "More", exact: true })
      .click();
    await page
      .getByRole("navigation", { name: "More pages" })
      .getByRole("button", { name: "Settings", exact: true })
      .click();
  } else
    await page
      .getByRole("navigation", { name: "Main navigation" })
      .getByRole("button", { name: "Settings", exact: true })
      .click();
}
for (const width of [1440, 360])
  test(`Branding and PWA identity preserve drafts, save independently and fit at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await login(page);
    await settings(page, width);
    await page.getByRole("tab", { name: "Branding", exact: true }).click();
    const preview = page.getByRole("img", { name: "Branding logo preview" });
    await expect(preview).toHaveAttribute("width", "80");
    expect(
      await preview.evaluate((img) => {
        const box = img.getBoundingClientRect();
        return box.width === 80 && box.height === 80;
      }),
    ).toBe(true);
    const chrome = page.locator(
      width > 760 ? ".sidebar .brand-icon" : ".mobile-brand-icon",
    );
    await expect(chrome).toBeVisible();
    expect(
      await chrome.evaluate(
        (img) =>
          (img as HTMLImageElement).complete &&
          (img as HTMLImageElement).naturalWidth > 0,
      ),
    ).toBe(true);
    await page
      .getByLabel("Display name", { exact: true })
      .fill("Synthetic workspace " + width);
    await page.getByRole("tab", { name: "PWA", exact: true }).click();
    await page.getByLabel("PWA name source").selectOption("custom");
    await page
      .getByLabel("PWA name", { exact: true })
      .fill("Synthetic installed app " + width);
    const inheritedPreview = page.getByRole("img", {
      name: "PWA icon preview",
    });
    if (await inheritedPreview.count())
      expect(
        await inheritedPreview.evaluate((img) => {
          const box = img.getBoundingClientRect();
          return box.width === 80 && box.height === 80;
        }),
      ).toBe(true);
    const actions = page
      .locator("#settings-panel-pwa .settings-save-actions")
      .last();
    expect(
      await actions.evaluate((el) => getComputedStyle(el).borderTopWidth),
    ).toBe("0px");
    await page.getByLabel("PWA icon source").selectOption("custom");
    const png = await page.evaluate(() => {
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 192;
      const ctx = canvas.getContext("2d")!;
      ctx.fillStyle = "#cc3344";
      ctx.fillRect(0, 0, 192, 192);
      return canvas.toDataURL("image/png").split(",")[1];
    });
    await page.getByLabel("PWA icon", { exact: true }).setInputFiles({
      name: "synthetic.png",
      mimeType: "image/png",
      buffer: Buffer.from(png, "base64"),
    });
    await expect(
      page.getByRole("img", { name: "PWA icon preview" }),
    ).toHaveAttribute("src", /^data:image\/png;base64,/);
    await page.getByRole("tab", { name: "Branding", exact: true }).click();
    await expect(page.getByLabel("Display name", { exact: true })).toHaveValue(
      "Synthetic workspace " + width,
    );
    await page.getByLabel("Branding logo", { exact: true }).setInputFiles({
      name: "brand.png",
      mimeType: "image/png",
      buffer: Buffer.from(png, "base64"),
    });
    await expect(preview).toHaveAttribute("src", /^data:image\/png;base64,/);
    await page
      .getByRole("button", { name: "Save branding", exact: true })
      .click();
    await expect(chrome).toHaveAttribute("src", /^data:image\/png;base64,/);
    await expect(page.locator('head link[rel="icon"]')).toHaveAttribute(
      "href",
      /^data:image\/png;base64,/,
    );
    expect(
      await page.locator('head link[rel="icon"]').getAttribute("href"),
    ).toBe(await chrome.getAttribute("src"));
    await expect
      .poll(() =>
        chrome.evaluate((img) => (img as HTMLImageElement).naturalWidth),
      )
      .toBeGreaterThan(0);
    expect(
      await preview.evaluate((img) => {
        const box = img.getBoundingClientRect();
        return box.width === 80 && box.height === 80;
      }),
    ).toBe(true);
    await expect(
      page.getByRole("status").filter({ hasText: "Branding saved" }),
    ).toBeVisible();
    await page.getByRole("tab", { name: "PWA", exact: true }).click();
    await expect(page.getByLabel("PWA name", { exact: true })).toHaveValue(
      "Synthetic installed app " + width,
    );
    await page
      .getByRole("button", { name: "Save PWA settings", exact: true })
      .click();
    await expect(
      page.getByRole("status").filter({ hasText: "PWA settings saved" }),
    ).toBeVisible();
    let manifest = await (
      await page.request.get("/manifest.webmanifest")
    ).json();
    expect(manifest.name).toBe("Synthetic installed app " + width);
    await page.getByLabel("PWA name source").selectOption("branding");
    await page.getByLabel("PWA icon source").selectOption("branding");
    const brandingSave = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/pwa" &&
        response.request().method() === "PUT",
    );
    await page
      .getByRole("button", { name: "Save PWA settings", exact: true })
      .click();
    expect((await brandingSave).status()).toBe(200);
    await expect(
      page.getByRole("button", { name: "Save PWA settings", exact: true }),
    ).toBeDisabled();
    manifest = await (await page.request.get("/manifest.webmanifest")).json();
    expect(manifest.name).toBe("Synthetic workspace " + width);
    expect(manifest.icons).toHaveLength(4);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.getByRole("tab", { name: "Branding", exact: true }).click();
    await page.getByLabel("Branding logo", { exact: true }).setInputFiles({
      name: "unsafe.svg",
      mimeType: "image/svg+xml",
      buffer: Buffer.from("<svg/>"),
    });
    await expect(
      page.getByText(
        "Use a square PNG icon, 192–1024 pixels and at most 256 KiB",
        { exact: true },
      ),
    ).toBeVisible();
  });
for (const mobile of [false, true])
  test(`Production manifest, real worker, private-cache exclusion, offline sign-out and reconnect (${mobile ? "phone" : "desktop"})`, async ({
    browser,
    baseURL,
  }) => {
    const context = await browser.newContext({
      baseURL,
      viewport: mobile
        ? { width: 390, height: 844 }
        : { width: 1440, height: 900 },
      isMobile: mobile,
      hasTouch: mobile,
    });
    const page = await context.newPage();
    await login(page);
    await page.evaluate(() => navigator.serviceWorker.ready);
    await expect
      .poll(() => page.evaluate(() => !!navigator.serviceWorker.controller))
      .toBe(true);
    const cdp = await context.newCDPSession(page);
    const app = await cdp.send("Page.getAppManifest");
    expect(app.errors).toEqual([]);
    expect(
      (await cdp.send("Page.getInstallabilityErrors")).installabilityErrors,
    ).toEqual([]);
    const manifest = JSON.parse(app.data);
    expect(manifest.display).toBe("standalone");
    expect(manifest.id).toBe("/");
    for (const icon of manifest.icons) {
      const response = await page.request.get(icon.src);
      expect(response.ok()).toBe(true);
      expect(response.headers()["content-type"]).toBe("image/png");
    }
    await page.evaluate(() =>
      fetch("/api/transactions").then((response) => response.json()),
    );
    const keys = await page.evaluate(async () => {
      const paths: string[] = [];
      for (const key of await caches.keys()) {
        for (const request of await (await caches.open(key)).keys())
          paths.push(new URL(request.url).pathname);
      }
      return paths;
    });
    expect(keys.length).toBeGreaterThan(2);
    expect(
      keys.every(
        (path) =>
          path.startsWith("/assets/") ||
          ["/offline.html", "/offline.css", "/sente.svg"].includes(path),
      ),
    ).toBe(true);
    await context.setOffline(true);
    await page.getByRole("button", { name: "Sign out", exact: true }).click();
    await expect(page.getByRole("alert")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Dashboard", exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "You’re offline", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Everyday account", { exact: true }),
    ).toHaveCount(0);
    await context.setOffline(false);
    await page.getByRole("link", { name: "Try again" }).click();
    await expect(
      page.getByRole("heading", { name: "Dashboard", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Sign out", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Sign in", exact: true }),
    ).toBeVisible();
    await context.setOffline(true);
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "You’re offline", exact: true }),
    ).toBeVisible();
    await context.setOffline(false);
    await page.getByRole("link", { name: "Try again" }).click();
    await expect(
      page.getByRole("button", { name: "Sign in", exact: true }),
    ).toBeVisible();
    await context.close();
  });
test("Install prompt is explicit; available updates leave drafts intact until the reload action", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const service = new EventTarget() as EventTarget & {
      controller: object;
      register: () => Promise<any>;
      getRegistration: () => Promise<any>;
    };
    service.controller = {};
    const registration = new EventTarget() as any;
    registration.update = async () => {};
    registration.waiting = {
      postMessage: (message: any) => {
        if (message.type === "SKIP_WAITING") {
          sessionStorage.setItem(
            "update-consents",
            String(Number(sessionStorage.getItem("update-consents") || 0) + 1),
          );
          service.dispatchEvent(new Event("controllerchange"));
        }
      },
    };
    service.register = async () => registration;
    service.getRegistration = async () => registration;
    Object.defineProperty(navigator, "serviceWorker", { value: service });
    (window as any).offerInstall = () => {
      const event = new Event("beforeinstallprompt", {
        cancelable: true,
      }) as any;
      event.prompt = async () => {
        sessionStorage.setItem("install-consents", "1");
      };
      event.userChoice = Promise.resolve({ outcome: "accepted" });
      window.dispatchEvent(event);
    };
  });
  await login(page);
  await settings(page, 1440);
  await page.getByRole("tab", { name: "Branding", exact: true }).click();
  await page
    .getByLabel("Display name", { exact: true })
    .fill("Keep this unsaved draft");
  await expect(
    page.getByRole("status").filter({ hasText: "A Sente update is available" }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => sessionStorage.getItem("update-consents")),
  ).toBeNull();
  await page.getByRole("tab", { name: "PWA", exact: true }).click();
  await page.evaluate(() => (window as any).offerInstall());
  await page.getByRole("button", { name: "Install app", exact: true }).click();
  expect(
    await page.evaluate(() => sessionStorage.getItem("install-consents")),
  ).toBe("1");
  await page.getByRole("tab", { name: "Branding", exact: true }).click();
  await expect(page.getByLabel("Display name", { exact: true })).toHaveValue(
    "Keep this unsaved draft",
  );
  await page
    .getByRole("status")
    .filter({ hasText: "A Sente update is available" })
    .getByRole("button", { name: "Update and reload", exact: true })
    .click();
  await expect
    .poll(() => page.evaluate(() => sessionStorage.getItem("update-consents")))
    .toBe("1");
  await expect(
    page.getByRole("heading", { name: "Dashboard", exact: true }),
  ).toBeVisible();
});

test("A real new worker waits for consent, prunes old assets and keeps other tabs’ drafts", async ({
  page,
  browser,
  baseURL,
}) => {
  const staticDir = process.env.E2E_PWA_STATIC_DIR;
  test.skip(
    !staticDir,
    "Use the disposable browser runner for isolated update fixtures",
  );
  const workerPath = join(staticDir!, "push-sw.js"),
    original = readFileSync(workerPath, "utf8");
  await login(page);
  await page.evaluate(() => navigator.serviceWorker.ready);
  await expect
    .poll(() => page.evaluate(() => !!navigator.serviceWorker.controller))
    .toBe(true);
  const oldCache = await page.evaluate(
    async () =>
      (await caches.keys()).find((key) => key.startsWith("sente-static-"))!,
  );
  const other = await page.context().newPage();
  await other.goto("/");
  await expect(
    other.getByRole("heading", { name: "Dashboard", exact: true }),
  ).toBeVisible();
  await settings(other, 1440);
  await other.getByRole("tab", { name: "Branding", exact: true }).click();
  await other
    .getByLabel("Display name", { exact: true })
    .fill("Other tab draft survives activation");
  await settings(page, 1440);
  await page.getByRole("tab", { name: "PWA", exact: true }).click();
  try {
    writeFileSync(
      workerPath,
      original.replace(
        /const CACHE = "sente-static-[^"]+";/,
        'const CACHE = "sente-static-synthetic-next";',
      ) + "\n// isolated synthetic deployment\n",
    );
    await page
      .getByRole("button", { name: "Check for updates", exact: true })
      .click();
    await expect(
      page
        .getByRole("status")
        .filter({ hasText: "A Sente update is available" }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        async () =>
          (await navigator.serviceWorker.getRegistration("/"))?.waiting?.state,
      ),
    ).toBe("installed");
    await expect(other.getByLabel("Display name", { exact: true })).toHaveValue(
      "Other tab draft survives activation",
    );
    await page
      .getByRole("status")
      .filter({ hasText: "A Sente update is available" })
      .getByRole("button", { name: "Update and reload", exact: true })
      .click();
    await expect(
      page.getByRole("heading", { name: "Dashboard", exact: true }),
    ).toBeVisible();
    const cachesAfter = await page.evaluate(() => caches.keys());
    expect(cachesAfter).toContain("sente-static-synthetic-next");
    expect(cachesAfter).not.toContain(oldCache);
    await expect(other.getByLabel("Display name", { exact: true })).toHaveValue(
      "Other tab draft survives activation",
    );
    await other
      .getByRole("status")
      .filter({ hasText: "A Sente update is available" })
      .getByRole("button", { name: "Update and reload", exact: true })
      .click();
    await expect(
      other.getByRole("heading", { name: "Dashboard", exact: true }),
    ).toBeVisible();
  } finally {
    writeFileSync(workerPath, original);
    await other.close();
  }
});

test("A session revoked during disconnection is rejected on reconnect without cached financial responses", async ({
  page,
  context,
}) => {
  await login(page);
  await page.evaluate(() => navigator.serviceWorker.ready);
  const session = await (await page.request.get("/api/me")).json();
  await context.setOffline(true);
  // The API fixture channel simulates server-side revocation while the page has no network.
  const revoked = await page.request.post("/api/logout", {
    headers: { "X-CSRF-Token": session.csrf },
  });
  expect(revoked.ok()).toBe(true);
  await context.setOffline(false);
  await page.getByRole("button", { name: /^Notifications,/ }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Everyday account", { exact: true })).toHaveCount(
    0,
  );
});

for (const width of [360, 1440])
  test(`First-install invitation is offered once; manual install remains available at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(() => {
      (window as any).offerInstall = () => {
        const event = new Event("beforeinstallprompt", {
          cancelable: true,
        }) as any;
        event.prompt = async () => {
          sessionStorage.setItem(
            "native-install-count",
            String(
              Number(sessionStorage.getItem("native-install-count") || 0) + 1,
            ),
          );
        };
        event.userChoice = Promise.resolve({ outcome: "dismissed" });
        window.dispatchEvent(event);
      };
    });
    await login(page);
    await page.evaluate(() => (window as any).offerInstall());
    const invitation = page
      .getByRole("status")
      .filter({ hasText: "Keep Sente handy" });
    await expect(invitation).toBeVisible({ timeout: 10000 });
    expect(
      await page.evaluate(() => sessionStorage.getItem("native-install-count")),
    ).toBeNull();
    const box = await invitation.boundingBox();
    expect(box!.width).toBeLessThanOrEqual(width);
    expect(box!.x).toBeGreaterThanOrEqual(0);
    await invitation
      .getByRole("button", { name: "Dismiss message", exact: true })
      .click();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Dashboard", exact: true }),
    ).toBeVisible();
    await settings(page, width);
    await page.getByRole("tab", { name: "PWA", exact: true }).click();
    await page.clock.install();
    await page.evaluate(() => (window as any).offerInstall());
    await expect(
      page.getByRole("button", { name: "Install app", exact: true }),
    ).toBeVisible();
    await page.clock.fastForward(8000);
    await expect(
      page.getByRole("button", { name: "Install now", exact: true }),
    ).toHaveCount(0);
    await page
      .getByRole("button", { name: "Install app", exact: true })
      .click();
    expect(
      await page.evaluate(() => sessionStorage.getItem("native-install-count")),
    ).toBe("1");
  });
test("Installed apps do not show an install invitation", async ({ page }) => {
  await page.addInitScript(() => {
    const original = window.matchMedia.bind(window);
    window.matchMedia = (query: string) =>
      query === "(display-mode: standalone)"
        ? ({ ...original(query), matches: true } as MediaQueryList)
        : original(query);
  });
  await login(page);
  await page.clock.install();
  await page.evaluate(() => {
    const event = new Event("beforeinstallprompt", { cancelable: true }) as any;
    event.prompt = async () => {};
    event.userChoice = Promise.resolve({ outcome: "accepted" });
    window.dispatchEvent(event);
  });
  await page.clock.fastForward(8000);
  await expect(
    page.getByRole("button", { name: "Install now", exact: true }),
  ).toHaveCount(0);
});
