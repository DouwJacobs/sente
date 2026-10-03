# Reverse proxy setup

Serve the tracker at the root of a dedicated hostname, for example `https://finance.example.com`. Set `PUBLIC_URL` to that exact browser-facing origin, including a nonstandard port if used. Subpath deployments such as `/finance/` are not supported. HTTPS is terminated at the proxy; its upstream connection to Go can use HTTP. Secure session cookies and CSRF origin validation use PUBLIC_URL, never a forwarded scheme/host supplied by a client.

## Settings UI

Administrators can use **Settings → Network** to save the public URL and trusted proxy addresses. Active values remain in use until the tracker restarts. Saved settings override PUBLIC_URL/TRUSTED_PROXIES on subsequent starts; the panel shows the active source and whether a restart is required. Configure Nginx Proxy Manager forwarding/SSL first, save the external HTTPS URL, restart the tracker, then open that URL. You may need to sign in at the new hostname. Listener/port bindings and NPM certificate management remain deployment settings.

**Use environment settings** clears the saved override for the next restart. If an incorrect saved URL prevents access, stop the service and run:

```bash
./bin/finance reset-network
# Docker alternative while the service is stopped:
docker compose run --rm -T finance reset-network
```

Then start the tracker with valid PUBLIC_URL/TRUSTED_PROXIES environment values. Offline reset requires the normal exclusive database lock. Network settings are included in database backups, so restored snapshots can restore an override too.

## Existing proxy

Route the entire hostname (frontend and `/api/`) to the Go service on port 8080. Preserve the browser Origin header and cookies. Disable response caching for authenticated API requests. Allow at least 26 MiB request bodies (25 MiB file limit plus multipart overhead), and use a 300-second upstream response timeout for owner approvals and FNB cleanup. Proxying the production Go server does not require WebSocket support.

Ready-to-edit examples: [Nginx](../deploy/nginx.conf.example) and [Caddy](../deploy/Caddyfile.example). Replace the example domain/upstream; Nginx also requires your certificate paths. Validate the config with `nginx -t` or `caddy validate --config /path/to/Caddyfile` before reloading your proxy. References: [Nginx proxy directives](https://nginx.org/en/docs/http/ngx_http_proxy_module.html), [Caddy reverse_proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy).

## Trusted proxy addresses

Set `TRUSTED_PROXIES` to comma-separated exact IPs or CIDRs of your proxy hops, as seen by Go's incoming socket. Default empty trusts no forwarding headers. Example for a native same-host proxy:

```bash
PUBLIC_URL=https://finance.example.com \
TRUSTED_PROXIES=127.0.0.1/32,::1/128 \
LISTEN_ADDRESS=127.0.0.1 PORT=8080 ./bin/finance serve
```

This enables per-client login rate limiting through X-Forwarded-For. Only an explicitly trusted immediate peer can provide that header. The chain is walked from right to left until the first untrusted address; spoofed client prefixes cannot change identity. Invalid or oversized chains fall back to the socket peer. Forwarded, X-Real-IP, X-Forwarded-Proto and X-Forwarded-Host do not control identity or cookie/origin security. Universal trust ranges are rejected.

At the internet-facing proxy, overwrite X-Forwarded-For with the actual client address (the Nginx example does this; Caddy handles its forwarded headers). With multiple proxy hops, configure those hops to sanitize/append headers and trust only their specific addresses/ranges in the app. Do not trust a subnet containing arbitrary clients. Without TRUSTED_PROXIES the app works through a proxy, but clients share its login rate-limit bucket.

## Docker or another machine

Compose passes PUBLIC_URL and TRUSTED_PROXIES from `.env`, binds the host port to 127.0.0.1 by default, and listens on the container interface internally. A host-based proxy can use `127.0.0.1:8080`; Go may see the Docker bridge gateway rather than loopback. Configure that actual gateway IP in TRUSTED_PROXIES. A proxy in a separate container must share a Docker network and use `finance:8080`; trust the proxy container's controlled address/network. Loopback inside a proxy container points to that container, not the tracker.

For a proxy on another machine, set BIND_ADDRESS to the tracker's private interface, restrict the port to that proxy host, and set TRUSTED_PROXIES to its address. Native Go can bind a private interface using LISTEN_ADDRESS. Existing native behavior remains listening on all interfaces when LISTEN_ADDRESS is unset.

The current Docker image still needs separate Node/browser/key provisioning for FNB automation; reverse proxy support does not add that runtime.

## Verify

Open the external HTTPS URL, sign in, save a setting, and sign out. These should work without Origin/security-token errors; the session cookie must have Secure and HttpOnly. `/api/health` must return status ok through the proxy. FNB Refresh now must be allowed to finish within the proxy timeout. An Origin error usually means PUBLIC_URL differs from the browser URL, or the proxy changed Origin. No cookie-domain rewrite or wildcard CORS is required.
