package app

import "database/sql"

func migrateNotificationAlerts(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`
CREATE TABLE notification_conditions (
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, scope_hash TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 0,
 cycle INTEGER NOT NULL DEFAULT 0, consumed INTEGER NOT NULL DEFAULT 0,
 last_sent INTEGER NOT NULL DEFAULT 0, evaluated_at INTEGER NOT NULL,
 PRIMARY KEY(user_id,type,scope_hash)
);
CREATE TABLE notification_diagnostics (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, key_hash TEXT NOT NULL, evaluated_at INTEGER NOT NULL,
 channel TEXT NOT NULL, status TEXT NOT NULL, error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX notification_diagnostics_time ON notification_diagnostics(evaluated_at,id);
CREATE TABLE notification_push_config (
 id INTEGER PRIMARY KEY CHECK(id=1), private_key TEXT NOT NULL, public_key TEXT NOT NULL
);
CREATE TABLE notification_push_subscriptions (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 endpoint TEXT NOT NULL UNIQUE, p256dh TEXT NOT NULL, auth TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL
);
CREATE TABLE notification_push_preferences (
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, type TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL DEFAULT 1,
 PRIMARY KEY(user_id,type)
);
CREATE TABLE notification_push_outbox (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 subscription_id INTEGER NOT NULL REFERENCES notification_push_subscriptions(id) ON DELETE CASCADE,
 notification_id INTEGER REFERENCES notifications(id) ON DELETE CASCADE,
 type TEXT NOT NULL, key_hash TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending',
 attempts INTEGER NOT NULL DEFAULT 0, next_attempt INTEGER NOT NULL, created_at INTEGER NOT NULL,
 UNIQUE(subscription_id,key_hash)
);
CREATE INDEX notification_push_due ON notification_push_outbox(status,next_attempt);
`)
	return err
}
