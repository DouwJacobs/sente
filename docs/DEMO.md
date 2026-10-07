# Synthetic household and screenshots

All fixture values are invented and may be committed to GitHub. `fixtures/demo/household.sql` is the source of truth; do not replace it with owner exports, SQLite files or backups. Money uses signed integer ZAR cents. Fixed August–November 2026 periods make screenshots reproducible; select October 2026 if your current date is outside the seeded periods.

Run `make demo`, then `make dev-demo`. Open http://127.0.0.1:5174 and sign in as `demo` / `sente-demo-password`. Creation migrates a new database with the actual application before applying the fixture in a transaction and checking foreign keys/integrity. It refuses an existing database, including a concurrent seed. There is no reset/overwrite option. For a fresh copy, stop the demo, move its `data/demo` folder aside and rerun `make demo`.

The admin can see two household accounts and a private account. Two populated periods support trends; a third has budgets without transactions. Examples include salary, regular bills, groceries, a refund, an exact split, linked transfers, a missing category, and mixed personal seen states. There are no connected banks, stored bank credentials, MCP clients or real import files. Bank IDs are visibly synthetic.

Use only this demo service for screenshots. Select October 2026 and household account scope, then capture Dashboard, Transactions, Budgets and Accounts in light/dark themes and desktop/mobile sizes as needed. Keep captures in `docs/images/` and label them as synthetic examples. Verify readable text, no cropped controls and no private browser information before publishing. This change prepares the capture workflow; screenshots and manual visual sign-off are separate follow-up work.

The fixture is offline development tooling. It adds no HTTP or MCP seed endpoint, schema field, permission or consent expansion. Financial MCP services retain their current allowlists and approval boundaries.
