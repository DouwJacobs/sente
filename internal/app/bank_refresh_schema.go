package app

import "database/sql"

func migrateBankRefreshJobs(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`
CREATE TABLE bank_refresh_jobs (
 id TEXT PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id),
 connection_id INTEGER NOT NULL,
 connection_key TEXT NOT NULL DEFAULT '',
 proposal_id TEXT NOT NULL UNIQUE,
 intent TEXT NOT NULL,
 account_ids TEXT NOT NULL,
 mode TEXT NOT NULL CHECK(mode IN ('balances','transactions','balances_and_transactions')),
 status TEXT NOT NULL CHECK(status IN ('queued','running','succeeded','needs_review','partial','failed','cancelled','interrupted','result_expired')),
 reason_code TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 started_at INTEGER,
 finished_at INTEGER,
 details TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX bank_refresh_owner_created ON bank_refresh_jobs(user_id,created_at DESC);
CREATE UNIQUE INDEX bank_refresh_single_active ON bank_refresh_jobs((1)) WHERE status IN ('queued','running');
CREATE TABLE bank_refresh_admissions (
 user_id INTEGER PRIMARY KEY REFERENCES users(id),
 last_accepted INTEGER NOT NULL
);`)
	return err
}
