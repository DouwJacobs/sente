package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func migrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "synthetic.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func migrationExec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	if _, err := db.Exec(statement); err != nil {
		t.Fatal(err)
	}
}

func migrationSnapshot(t *testing.T, db *sql.DB, queries ...string) string {
	t.Helper()
	var snapshots [][]map[string]any
	for _, query := range queries {
		rows, err := data(db, query)
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, rows)
	}
	encoded, err := json.Marshal(snapshots)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestSequentialMigrationHistoricalSnapshots(t *testing.T) {
	for _, version := range []int{9, 19, 22} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			db := migrationDB(t)
			snapshot, err := os.ReadFile(fmt.Sprintf("testdata/migrations/v%d.sql", version))
			if err != nil {
				t.Fatal(err)
			}
			migrationExec(t, db, string(snapshot))
			migrationExec(t, db, `
INSERT INTO users(id,username,password,admin) VALUES(1,'synthetic owner','not a credential',1),(2,'synthetic viewer','not a credential',0);
INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Shared','synthetic-shared',1),(2,'Private','synthetic-private',0);
INSERT INTO grants VALUES(1,2,'editor'),(2,1,'viewer');
INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic expense','','expense');
INSERT INTO periods(id,name,start_date,end_date) VALUES(1,'Synthetic period','2026-10-01','2026-10-31');
INSERT INTO targets VALUES(1,1,123456);
INSERT INTO imports(id,user_id,account_id,name,hash,format,status,data) VALUES(1,1,2,'synthetic.ofx','synthetic hash','ofx','committed','{"synthetic":true}');
INSERT INTO transactions(id,account_id,date,amount_cents,description,fitid,source_date,source_amount,source_description,source_key,provenance,import_id,review_state,reviewed_by,reviewed_at,version,period_id,assignment)
VALUES(1,2,'2026-10-04',-2512,'Edited description','synthetic-fitid','2026-10-03',-2512,'Immutable original','','{"synthetic":true}',1,'approved',1,'2026-10-04',7,1,'manual');
INSERT INTO allocations(id,transaction_id,category_id,amount_cents,note) VALUES(1,1,1,-1000,'one'),(2,1,1,-1512,'two');
INSERT INTO audit(id,user_id,account_id,entity,entity_id,action,details) VALUES(1,1,2,'transaction',1,'synthetic history','{}');
INSERT INTO sessions VALUES('synthetic-session',1,'synthetic-csrf',9999999999);
`)
			if version >= 19 {
				migrationExec(t, db, `INSERT INTO group_targets(period_id,category_id,amount_cents,included,carry_forward) VALUES(1,1,123456,1,0);
INSERT INTO merchants(id,account_id,name,version) VALUES(42,2,'Synthetic merchant',3);
INSERT INTO merchant_rules(id,account_id,merchant_id,pattern,version) VALUES(81,2,42,'synthetic pattern',4);
UPDATE transactions SET merchant_id=42,note='Private synthetic note' WHERE id=1;
INSERT INTO mcp_tokens(id,user_id,name,token_hash,permissions,expires_at) VALUES(1,1,'Synthetic agent','synthetic token hash','{"read_context":false}',9999999999);`)
			}
			protected := []string{
				"SELECT id,account_id,date,amount_cents,description,fitid,source_date,source_amount,source_description,provenance,import_id,review_state,reviewed_by,reviewed_at,version,period_id,assignment FROM transactions ORDER BY id",
				"SELECT * FROM allocations ORDER BY id", "SELECT * FROM grants ORDER BY user_id,account_id",
				"SELECT id,user_id,account_id,name,hash,format,status,data,created_at FROM imports ORDER BY id",
				"SELECT * FROM audit ORDER BY id", "SELECT * FROM sessions ORDER BY token",
				"SELECT * FROM targets ORDER BY period_id,category_id",
			}
			if version >= 19 {
				protected = append(protected, "SELECT * FROM merchants ORDER BY id", "SELECT * FROM merchant_rules ORDER BY id", "SELECT * FROM group_targets ORDER BY period_id", "SELECT * FROM mcp_tokens ORDER BY id")
			}
			before := migrationSnapshot(t, db, protected...)
			if err := migrate(db); err != nil {
				t.Fatal(err)
			}
			if after := migrationSnapshot(t, db, protected...); before != after {
				t.Fatal("upgrade changed protected financial/history/permission data", before, after)
			}
			if queryInt(db, "SELECT MAX(version) FROM migrations") != schemaVersion {
				t.Fatal("baseline not recorded")
			}
			if queryInt(db, "SELECT COUNT(*) FROM group_targets WHERE amount_cents=123456") != 1 {
				t.Fatal("budget not preserved")
			}
			var key string
			if err := db.QueryRow("SELECT source_key FROM transactions WHERE id=1").Scan(&key); err != nil {
				t.Fatal(err)
			}
			if key != fingerprint(SourceRow{Date: "2026-10-03", Amount: -2512, Description: "Immutable original"}) {
				t.Fatal("incorrect immutable source fingerprint")
			}
			all := []string{"SELECT type,name,tbl_name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM migrations ORDER BY version", "SELECT * FROM transaction_seen ORDER BY user_id,transaction_id", "SELECT * FROM categories ORDER BY id", "SELECT * FROM transactions ORDER BY id", "SELECT * FROM group_targets ORDER BY period_id", "SELECT * FROM audit ORDER BY id"}
			once := migrationSnapshot(t, db, all...)
			if err := migrate(db); err != nil {
				t.Fatal(err)
			}
			if once != migrationSnapshot(t, db, all...) {
				t.Fatal("restart repeated migration")
			}
			if queryInt(db, "PRAGMA foreign_keys") != 1 {
				t.Fatal("foreign keys not restored")
			}
			violations, err := data(db, "PRAGMA foreign_key_check")
			if err != nil || len(violations) > 0 {
				t.Fatal("invalid references", err)
			}
		})
	}
}

func TestSequentialMigrationFreshAndNoStartupRepair(t *testing.T) {
	db := migrationDB(t)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"categories", "spending_groups", "merchants", "builtin_rules", "rules"} {
		if queryInt(db, "SELECT COUNT(*) FROM "+table) != 0 {
			t.Fatal("fresh install seeded", table)
		}
	}
	migrationExec(t, db, "DROP TABLE workspace_branding")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='workspace_branding'") != 0 {
		t.Fatal("latest startup reapplied baseline schema")
	}
}

func TestSequentialMigrationOrderingRollbackAndRetry(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:1], migrationBaseline); err != nil {
		t.Fatal(err)
	}
	calls := 0
	steps := append([]schemaMigration{}, schemaMigrations[:1]...)
	steps = append(steps, schemaMigration{24, "synthetic schema", false, func(tx *sql.Tx, _ migrationOrigin) error {
		calls++
		_, err := tx.Exec("CREATE TABLE migration_probe(id INTEGER PRIMARY KEY); INSERT INTO migration_probe VALUES(1)")
		return err
	}}, schemaMigration{25, "synthetic backfill", false, func(tx *sql.Tx, _ migrationOrigin) error {
		if queryInt(tx, "SELECT COUNT(*) FROM migration_probe") != 1 {
			t.Fatal("steps out of order")
		}
		if _, err := tx.Exec("INSERT INTO migration_probe VALUES(2)"); err != nil {
			return err
		}
		return errors.New("injected failure")
	}})
	err := runSchemaMigrations(db, steps, 25)
	if err == nil || !strings.Contains(err.Error(), "migration 25 (synthetic backfill)") {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 23 || queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='migration_probe'") != 0 {
		t.Fatal("failed batch left schema/version effects")
	}
	steps[2].apply = func(tx *sql.Tx, _ migrationOrigin) error {
		_, err := tx.Exec("INSERT INTO migration_probe VALUES(2)")
		return err
	}
	if err := runSchemaMigrations(db, steps, 25); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM migration_probe") != 2 || queryInt(db, "SELECT COUNT(*) FROM migrations WHERE version>=23") != 3 {
		t.Fatal("retry incomplete")
	}
	if err := runSchemaMigrations(db, steps, 25); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("applied migration ran again", calls)
	}
}

func TestSequentialMigrationBridgeRollbackAndForeignKeys(t *testing.T) {
	db := migrationDB(t)
	snapshot, err := os.ReadFile("testdata/migrations/v9.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, string(snapshot))
	migrationExec(t, db, `INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused');
INSERT INTO accounts(id,name,bank_id) VALUES(1,'Synthetic','synthetic');
INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic','','expense');
INSERT INTO targets(period_id,category_id,amount_cents) VALUES(999,1,100);`)
	before := migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM migrations ORDER BY version", "SELECT * FROM targets")
	if err := migrate(db); err == nil {
		t.Fatal("orphaned budget allowed")
	}
	if before != migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM migrations ORDER BY version", "SELECT * FROM targets") {
		t.Fatal("bridge failure left DDL/data/history")
	}
	if queryInt(db, "PRAGMA foreign_keys") != 1 {
		t.Fatal("failure left foreign keys disabled")
	}
	migrationExec(t, db, "DELETE FROM targets")
	if err := migrate(db); err != nil {
		t.Fatal("retry", err)
	}
}

func TestSequentialMigrationRejectsInvalidHistoryAndRegistry(t *testing.T) {
	noOp := func(*sql.Tx, migrationOrigin) error { return nil }
	for _, tc := range []struct {
		name   string
		steps  []schemaMigration
		target int
	}{
		{"empty", nil, 23}, {"gap", []schemaMigration{{23, "one", false, noOp}, {25, "two", false, noOp}}, 25},
		{"duplicate", []schemaMigration{{23, "one", false, noOp}, {23, "two", false, noOp}}, 23},
		{"version mismatch", []schemaMigration{{23, "one", false, noOp}}, 24},
		{"missing callback", []schemaMigration{{23, "one", false, nil}}, 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := migrationDB(t)
			if err := runSchemaMigrations(db, tc.steps, tc.target); err == nil {
				t.Fatal("invalid registry accepted")
			}
			if queryInt(db, "SELECT COUNT(*) FROM sqlite_master") != 0 {
				t.Fatal("invalid registry wrote schema")
			}
		})
	}
	for _, tc := range []struct{ name, history string }{
		{"future", "24"}, {"negative", "-1"}, {"gapped sequential", "25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := migrationDB(t)
			migrationExec(t, db, "CREATE TABLE migrations(version INTEGER PRIMARY KEY); INSERT INTO migrations VALUES("+tc.history+")")
			steps := schemaMigrations[:1]
			target := 23
			if tc.name == "gapped sequential" {
				steps = append(append([]schemaMigration{}, steps...), schemaMigration{24, "one", false, noOp}, schemaMigration{25, "two", false, noOp})
				target = 25
			}
			if err := runSchemaMigrations(db, steps, target); err == nil {
				t.Fatal("invalid history accepted")
			}
			if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='users'") != 0 {
				t.Fatal("invalid history wrote schema")
			}
		})
	}
}

func TestSequentialMigrationFreshBatchFailureIsAtomic(t *testing.T) {
	db := migrationDB(t)
	steps := append(append([]schemaMigration{}, schemaMigrations[:1]...), schemaMigration{24, "synthetic failure", false, func(tx *sql.Tx, _ migrationOrigin) error {
		if queryInt(tx, "SELECT COUNT(*) FROM migrations WHERE version=23") != 1 {
			t.Fatal("baseline not ordered before new migration")
		}
		_, err := tx.Exec("ALTER TABLE users ADD COLUMN synthetic_column TEXT; INSERT INTO users(username,password) VALUES('synthetic','unused')")
		if err != nil {
			return err
		}
		return errors.New("injected failure")
	}})
	if err := runSchemaMigrations(db, steps, 24); err == nil {
		t.Fatal("failure ignored")
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'") != 0 {
		t.Fatal("fresh failed upgrade retained tables or records")
	}
	if queryInt(db, "PRAGMA foreign_keys") != 1 {
		t.Fatal("foreign keys not restored")
	}
	if err := migrate(db); err != nil {
		t.Fatal("retry", err)
	}
}

func TestSequentialMigrationVersionRecordFailureRollsBack(t *testing.T) {
	db := migrationDB(t)
	snapshot, err := os.ReadFile("testdata/migrations/v22.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, string(snapshot))
	migrationExec(t, db, `ALTER TABLE users DROP COLUMN deleted_at;
CREATE TRIGGER reject_baseline BEFORE INSERT ON migrations WHEN NEW.version=23 BEGIN SELECT RAISE(ABORT,'injected record failure'); END;`)
	before := migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM migrations ORDER BY version")
	if err := migrate(db); err == nil || !strings.Contains(err.Error(), "record migration 23") {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM migrations ORDER BY version") {
		t.Fatal("record failure retained schema changes")
	}
	migrationExec(t, db, "DROP TRIGGER reject_baseline")
	if err := migrate(db); err != nil {
		t.Fatal("retry", err)
	}
}

func TestSequentialMigrationMerchantRebuildFailureRestoresCatalogue(t *testing.T) {
	db := migrationDB(t)
	snapshot, err := os.ReadFile("testdata/migrations/v22.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, string(snapshot))
	migrationExec(t, db, `DROP TABLE merchants;
CREATE TABLE merchants(id INTEGER PRIMARY KEY,account_id INTEGER NOT NULL REFERENCES accounts(id),name TEXT NOT NULL COLLATE NOCASE,UNIQUE(account_id,name));
INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused');
INSERT INTO accounts(id,name,bank_id) VALUES(1,'Synthetic','synthetic');
INSERT INTO merchants VALUES(42,1,'Synthetic merchant');
INSERT INTO merchant_rules(id,account_id,merchant_id,pattern) VALUES(81,1,42,'synthetic');
INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic','','expense');
CREATE TRIGGER reject_category_conversion BEFORE UPDATE ON categories BEGIN SELECT RAISE(ABORT,'injected category failure'); END;`)
	before := migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM merchants", "SELECT * FROM merchant_rules", "SELECT * FROM migrations ORDER BY version")
	if err := migrate(db); err == nil || !strings.Contains(err.Error(), "injected category failure") {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, "SELECT type,name,sql FROM sqlite_master ORDER BY type,name", "SELECT * FROM merchants", "SELECT * FROM merchant_rules", "SELECT * FROM migrations ORDER BY version") {
		t.Fatal("catalogue rebuild survived failed upgrade")
	}
	if queryInt(db, "PRAGMA foreign_keys") != 1 {
		t.Fatal("foreign keys not restored")
	}
	migrationExec(t, db, "DROP TRIGGER reject_category_conversion")
	if err := migrate(db); err != nil {
		t.Fatal("retry", err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM merchants WHERE id=42") != 1 || queryInt(db, "SELECT merchant_id FROM merchant_rules WHERE id=81") != 42 {
		t.Fatal("rebuild lost identities")
	}
}

// Legacy workflow fixtures start from a current synthetic DB then deliberately
// rewind its version. Remove later schemas too, matching a real pre-24 install.
// Production migrations must never reconcile already-applied table creation.
func removePostBaselineFixtureTables(t *testing.T, db *sql.DB) {
	t.Helper()
	migrationExec(t, db, "DROP TABLE notification_accounts; DROP TABLE notifications; DROP TABLE notification_receipts; DROP TABLE notification_preferences")
}
