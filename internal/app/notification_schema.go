package app

import "database/sql"

func migrateNotifications(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`
CREATE TABLE notification_receipts (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, dedupe_hash TEXT NOT NULL, event_hash TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 UNIQUE(user_id,type,dedupe_hash)
);
CREATE INDEX notification_receipts_age ON notification_receipts(created_at);
CREATE TABLE notifications (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 receipt_id INTEGER NOT NULL UNIQUE REFERENCES notification_receipts(id) ON DELETE CASCADE,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, severity TEXT NOT NULL CHECK(severity IN ('info','warning','critical')),
 title TEXT NOT NULL CHECK(length(title) BETWEEN 1 AND 120),
 message TEXT NOT NULL CHECK(length(message) BETWEEN 1 AND 1000),
 source_kind TEXT NOT NULL CHECK(source_kind IN ('system','account','transaction','budget')),
 source_id INTEGER NOT NULL, occurred_at INTEGER NOT NULL, created_at INTEGER NOT NULL,
 read_at INTEGER, dismissed_at INTEGER,
 dismissible INTEGER NOT NULL CHECK(dismissible IN (0,1))
);
CREATE INDEX notifications_inbox ON notifications(user_id,dismissed_at,id DESC);
CREATE INDEX notifications_age ON notifications(created_at);
-- Account IDs intentionally retain tombstones: deleting an account must never
-- remove a privacy dependency and make its old message visible again.
CREATE TABLE notification_accounts (
 notification_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
 account_id INTEGER NOT NULL,
 PRIMARY KEY(notification_id,account_id)
);
CREATE TABLE notification_preferences (
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, channel TEXT NOT NULL CHECK(channel='in_app'),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 version INTEGER NOT NULL DEFAULT 1,
 PRIMARY KEY(user_id,type,channel)
);`)
	return err
}
