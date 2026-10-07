> Historical record, archived 7 October 2026. This describes earlier development and includes superseded decisions. Use the [current documentation](../../README.md) for new work.

# Separate connector service: mock-only scaffold

The owner has accepted deployment as a separate isolated service. This source is prepared here for review and synthetic testing. It must ultimately run on an independently operated host outside Codex/agent permissions; launching a container on this workstation does not meet that requirement.

No live provider or credential-input endpoint exists. The server rejects all mutations, including credential submission. It does not call upstream, persist reports or secrets, run a scheduler or expose credentials. Live mode is rejected. The tracker has no client for this server yet.

Run tests with Node 22+: `node --test` in this directory. The server requires `FNB_CONNECTOR_MODE=mock` and `FNB_TRANSPORT_TOKEN_FILE` pointing to an owner-provisioned token of at least 32 bytes. Token provisioning stays outside the repo; startup errors never print its value. It binds to 127.0.0.1:19090 by default (`PORT` is configurable); there is no added public port or production compose change.

Authenticated GET routes: `/v1/status`, `/v1/accounts`, `/v1/report`. Each returns explicit mock status or synthetic account/report data. This single mock transport token is not a production per-user authorization design. Production must bind tokens to user-owned connections and recheck owner/account permissions at report delivery and import execution.

`vault.mjs` contains internal encryption primitives, disconnected from HTTP and storage. AES-256-GCM uses a fresh 12-byte nonce, 16-byte tag, and associated data containing purpose, user, connection and key version. Both username and password are encrypted. Missing keys, wrong identity and tampered records return a generic owner-action failure. Rotation decrypts with the old key and reseals with a new nonce/version, retaining owner/connection identity. No decrypt/read route may be added. Byte buffers are wiped where practical; JavaScript strings and process memory still contain plaintext briefly, which is why host isolation is required.

Owner provisioning/recovery requirements before production:

- Provision encryption keys in the isolated service's runtime secret manager; never use a default key or store keys alongside ciphertext, source or ordinary database snapshots. Separate transport and encryption keys.
- Back up key versions through an independently protected owner-controlled channel, separate from encrypted-data backups. Keep old keys until every associated record is successfully rotated and rollback snapshots have expired.
- Missing/lost keys stop authentication. Recover from the separately protected key backup or ask each connection owner to re-enter credentials; do not silently reset the vault or try default keys.
- Add transactional encrypted storage, key-version metadata, user authentication, write-only owner credential replacement, pause/disconnect deletion, sanitized status and audit. Admins may disable a connection but cannot read its secrets.
- Deploy authenticated TLS transport on the separate host with access controls. Do not paste runtime tokens or bank credentials into chat, command arguments, repository files, screenshots or logs. Owner-managed live login/MFA tests happen outside agent observation.
- Correct/review upstream money parsing, session lifecycle, current bank compatibility and dependencies before any live execution. Do not use no-sandbox Chromium flags or a persistent browser profile in the source tree.
