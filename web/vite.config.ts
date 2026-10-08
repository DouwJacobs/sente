import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import type { Plugin } from "vite";
// Exact build allowlist, with a worker revision derived from all cacheable bytes.
function pwaBuild(): Plugin {
  let outDir = "";
  return {
    name: "sente-pwa-build",
    configResolved(config) {
      outDir = resolve(config.root, config.build.outDir);
    },
    writeBundle(_options, bundle) {
      const files = Object.keys(bundle).filter((name) =>
        name.startsWith("assets/"),
      );
      const publicFiles = ["offline.html", "offline.css", "sente.svg"];
      const source = readFileSync(resolve(outDir, "push-sw.js"), "utf8");
      const hash = createHash("sha256").update(source);
      for (const name of [...files, ...publicFiles].sort())
        hash.update(name).update(readFileSync(resolve(outDir, name)));
      const revision = hash.digest("hex").slice(0, 20);
      const assets = [...files, ...publicFiles].map((name) => "/" + name);
      writeFileSync(
        resolve(outDir, "push-sw.js"),
        source
          .replace(
            'const CACHE = "sente-static-dev";',
            "const CACHE = " + JSON.stringify("sente-static-" + revision) + ";",
          )
          .replace(
            'const ASSETS = ["/offline.html", "/offline.css"];',
            "const ASSETS = " + JSON.stringify(assets) + ";",
          ),
      );
    },
  };
}
export default defineConfig({
  plugins: [react(), pwaBuild()],
  server: {
    strictPort: true,
    allowedHosts: [
      "dev-finance.p-rex.co.za",
      ...(process.env.DEV_PUBLIC_URL
        ? [new URL(process.env.DEV_PUBLIC_URL).hostname]
        : []),
    ],
    proxy: Object.fromEntries(
      [
        "/api",
        "/oauth",
        "/.well-known",
        "/manifest.webmanifest",
        "/branding",
        "/pwa",
      ].map((path) => [
        path,
        process.env.DEV_API_TARGET || "http://127.0.0.1:8080",
      ]),
    ),
  },
});
