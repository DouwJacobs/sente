# Branding and installed app

Settings → Branding lets administrators change the authenticated workspace name and upload or remove the shared logo. Settings → PWA gives every user install guidance, available Install app / Update and reload actions and a manual update check. Administrators also choose a separate public PWA name/icon or inherit each from Branding. Drafts survive Settings tab switches; Branding and PWA saves have independent optimistic versions and administrator checks inside browserWrite.

The manifest and icons must be readable before sign-in. Existing workspace names stay private by default: migration 28 initializes the public PWA name to Sente and does not select workspace-name inheritance. Choosing Use workspace name explicitly publishes that name. Uploaded branding logos are public and used by sign-in, navigation and favicon; the default PWA icon follows Branding. Custom PWA name/icon drafts remain saved when inheritance is selected, allowing later switching back. Removing a custom icon restores the built-in Sente mark. PNG input is square, 192–1024 pixels and at most 256 KiB; the server decodes/re-encodes to strip metadata, bounds memory and rejects URLs/SVG/other formats. Audits never store image bytes.

## Installation

The root `/manifest.webmanifest` uses a stable `/` id, start URL and scope, standalone display, teal theme/light background and generated 192/512px normal and opaque padded maskable PNGs. `/pwa/icon/180/any.png` supplies the Apple touch icon. Public identity routes and HTML use no-store. Icon URLs include both identity versions so browser manifest refreshes see branding changes. Installed name/icon refresh remains browser/OS-controlled; existing installations may need reinstalling. HTTPS or a browser-approved localhost origin is required. Chromium receives an explicit Install app action when it offers beforeinstallprompt; Safari guidance uses Share → Add to Home Screen. Ordinary navigation never requests notification permission.

After sign-in, an eligible browser receives one dismissible overlay install invitation after four seconds, delayed while hidden or while a dialog is open. Chromium uses the browser's available install event; iPhone/iPad receives Safari home-screen guidance. Showing the invitation or attempting installation remembers the choice in origin-local browser storage; it is not offered repeatedly on navigation/reload. Installed apps are excluded and update notices take precedence. Where persistent browser storage is unavailable, the page still suppresses repeats for its lifetime. Installation remains discoverable in Settings → PWA; the native install dialog opens only on an explicit click.

## Cache/privacy boundary

The existing root `/push-sw.js` is extended in place, preserving push subscriptions and generic lock-screen text/direct-link restrictions. No competing root worker is registered. Registration itself grants no push/device consent: notification preference and device opt-in remain separate explicit workflows.

Vite generates a worker revision from worker source and all listed static build bytes. Its exact allowlist includes emitted `/assets/` files, the generic offline HTML/CSS and built-in mark. Only these files enter its versioned `sente-static-*` cache. No wildcard runtime caching, API cache, authenticated HTML shell, financial payload or offline synchronization is provided. API/OAuth/MCP, public identity, manifest, cross-origin, non-GET and query-bearing static requests bypass worker caching. HTML navigations use network/no-store and fall back only to the generic reconnect page; original URLs/queries never enter Cache Storage. The fallback contains no financial or personal information and its retry link opens `/`.

Financial records already displayed can remain in the open tab's ordinary memory when connectivity drops. They are not persisted by the worker. Offline sign-out fails visibly and needs a server connection to revoke the session; it never claims successful logout. Reloading offline exposes only the reconnect screen. Reconnect/current session rejection clears browser workspace state through existing authentication handling. Successful server logout and subsequent reconnect require sign-in again.

## Updates

Installation precaches only allowed static assets. A deployed asset/worker change creates a new waiting worker. The shared overlay toast and Settings → PWA offer Update and reload, explaining that unsaved edits in that tab are cleared. There is no unconditional skipWaiting or automatic reload of other tabs. A dismissed notice remains actionable in PWA settings. Updates are checked when the app regains visibility/connectivity and hourly while visible, plus the manual action.

Explicit consent sends SKIP_WAITING and reloads the consenting tab after controllerchange. Activation prunes only old Sente static caches and claims clients. Other open tabs keep their mounted drafts and can explicitly reload after the worker has already activated. Browser worker registration/revisit checks also discover deployments; content-hashed assets and uncached HTML avoid an indefinitely retained application shell. No financial mutation is part of update handling.

## Verification boundaries

Use `python3 scripts/test-browser.py pwa.spec.ts` with the documented Linux Node runtime. That spec receives a disposable static copy and synthetic databases; its two-version worker test never edits the shared build or deployed files. Desktop/mobile Chromium manifest/installability diagnostics, real worker/cache/offline behavior, same-session tab updates and explicit synthetic install-prompt actions are automated. Physical desktop/mobile installation, Safari/OS splash screens, icon-refresh timing, real provider push display and manual visual inspection still require owner-run checks.

Lifecycle references: [MDN service workers](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API/Using_Service_Workers), [explicit activation](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerGlobalScope/skipWaiting).

MCP tools, schemas, output allowlists, capabilities, consent, proposals and financial services are unchanged. Branding/PWA editing and installation are browser-only host presentation, not agent access to financial or personal data.
