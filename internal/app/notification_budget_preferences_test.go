package app

import (
	"context"
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func scopedAlertFixture(t *testing.T) *testEnv {
	e := alertFixture(t)
	migrationExec(t, e.a.DB, "INSERT INTO spending_groups(id,name,color) VALUES(900,'Special Exceptions','#123456'); INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents) VALUES(1,1,NULL,10000),(1,1,900,20000); UPDATE targets SET amount_cents=30000")
	return e
}
func budgetAlertInput(gid, version int64, enabled bool, threshold any) map[string]any {
	return map[string]any{"category_id": 1, "group_id": gid, "enabled": enabled, "threshold": threshold, "version": version}
}
func saveBudgetAlerts(t *testing.T, e *testEnv, uid int64, code int, inputs ...any) {
	t.Helper()
	status(t, e.req(t, int(uid), "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"items": []any{}, "budget_items": inputs}), code)
}
func TestScopedBudgetAlertsExactBoundariesRefundsAndIndependentGroups(t *testing.T) {
	e := scopedAlertFixture(t)
	now := alertTestTime()
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70), budgetAlertInput(900, 0, true, 90))
	alertTransaction(t, e, 1, 1, -6999, "2026-10-08")
	alertTransaction(t, e, 2, 1, -17999, "2026-10-08")
	migrationExec(t, e.a.DB, "UPDATE transactions SET spending_group_id=900 WHERE id=2")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("aggregate spending or rounding crossed a scoped threshold")
	}
	changeAlertAmount(t, e, -7000)
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("70 percent boundary missing")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET amount_cents=-18000 WHERE id=2; UPDATE allocations SET amount_cents=-18000 WHERE transaction_id=2")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 2 {
		t.Fatal("independent 90 percent boundary missing")
	}
	evaluateAlerts(t, e, now.Add(time.Hour))
	if alertCount(e, "budget_threshold") != 2 {
		t.Fatal("replayed scoped crossing")
	}
	alertTransaction(t, e, 3, 1, 1, "2026-10-08")
	evaluateAlerts(t, e, now)
	migrationExec(t, e.a.DB, "DELETE FROM transactions WHERE id=3")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 3 {
		t.Fatal("refund reset did not allow a new crossing")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=1 AND type='budget_threshold' AND message LIKE '%Exceptions%'") != 1 {
		t.Fatal("wrong scoped message")
	}
}
func TestScopedBudgetPreferenceSaveBaselineDefaultsAndGlobalPrecedence(t *testing.T) {
	e := scopedAlertFixture(t)
	now := alertTestTime()
	alertTransaction(t, e, 1, 1, -8000, "2026-10-08")
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70))
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("preference save backfilled history")
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 1, false, 70))
	changeAlertAmount(t, e, -11000)
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_overspend") != 0 {
		t.Fatal("disabled scope delivered overspend")
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 2, true, 70))
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 || alertCount(e, "budget_overspend") != 0 {
		t.Fatal("re-enable replayed consumed occurrence")
	}
	changeAlertAmount(t, e, -6000)
	evaluateAlerts(t, e, now)
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", map[string]any{"type": "budget_threshold", "channel": "in_app", "enabled": false, "version": 0}), 200)
	changeAlertAmount(t, e, -7000)
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("scope overrode global channel choice")
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 3, true, nil))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE user_id=1 AND threshold IS NULL AND enabled=1") != 1 {
		t.Fatal("reset did not restore defaults")
	}
}
func TestScopedBudgetPreferenceValidationAtomicityAndPrivacy(t *testing.T) {
	e := scopedAlertFixture(t)
	for _, threshold := range []any{0, 101, 70.5, "70"} {
		saveBudgetAlerts(t, e, 1, 400, budgetAlertInput(0, 0, true, threshold))
	}
	saveBudgetAlerts(t, e, 1, 400, budgetAlertInput(0, 0, true, 70), budgetAlertInput(0, 0, true, 90))
	saveBudgetAlerts(t, e, 3, 403, budgetAlertInput(0, 0, true, 70))
	if strings.Contains(e.req(t, 3, "/api/notifications/preferences?channels=all", "GET", nil).Body.String(), "Groceries") {
		t.Fatal("nonmember saw budget scopes")
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70))
	if strings.Contains(e.req(t, 2, "/api/notifications/preferences?channels=all", "GET", nil).Body.String(), `"threshold":70`) {
		t.Fatal("personal setting disclosed")
	}
	saveBudgetAlerts(t, e, 1, 409, budgetAlertInput(900, 0, true, 80), budgetAlertInput(0, 0, true, 90))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE group_id=900") != 0 {
		t.Fatal("stale batch partially saved")
	}
	migrationExec(t, e.a.DB, "CREATE TRIGGER reject_scoped_audit BEFORE INSERT ON audit WHEN NEW.entity='notification_budget_preferences' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	saveBudgetAlerts(t, e, 1, 500, budgetAlertInput(900, 0, true, 80))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE group_id=900") != 0 {
		t.Fatal("failed audit saved settings")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_scoped_audit")
	status(t, admittedMutation(t, e, "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"budget_items": []any{budgetAlertInput(900, 0, true, 80)}}, "UPDATE users SET budget_member=0 WHERE id=1"), 403)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE group_id=900") != 0 {
		t.Fatal("revoked member wrote settings")
	}
}
func TestScopedBudgetPreferenceMigrationPreservesConsumedHistory(t *testing.T) {
	e := scopedAlertFixture(t)
	migrationExec(t, e.a.DB, "DROP TABLE notification_budget_scopes; DROP TABLE notification_budget_preferences; DELETE FROM migrations WHERE version>=29")
	old := fmt.Sprintf("household-budget:%d:category:%d:threshold:75", 1, 1)
	migrationExec(t, e.a.DB, fmt.Sprintf("INSERT INTO notification_conditions(user_id,type,scope_hash,active,cycle,consumed,last_sent,evaluated_at,evaluated_run) VALUES(1,'budget_threshold','%s',1,1,1,123,456,0)", hash(old)))
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	for _, gid := range []int64{0, 900} {
		if queryInt(e.a.DB, "SELECT consumed FROM notification_conditions WHERE user_id=1 AND scope_hash=?", hash(budgetAlertScope(1, 1, gid)+":threshold:75")) != 1 {
			t.Fatal("upgrade lost suppression")
		}
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(900, 0, true, 70))
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT threshold FROM notification_budget_preferences WHERE user_id=1 AND group_id=900") != 70 {
		t.Fatal("restart lost preference")
	}
}

func TestScopedBudgetSaveDoesNotEvaluateOtherCombinations(t *testing.T) {
	e := scopedAlertFixture(t)
	alertTransaction(t, e, 1, 1, -19000, "2026-10-08")
	migrationExec(t, e.a.DB, "UPDATE transactions SET spending_group_id=900 WHERE id=1")
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70))
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("save evaluated an unrelated scope")
	}
	evaluateAlerts(t, e, alertTestTime())
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("save consumed an unrelated crossing")
	}
}
func TestScopedBudgetDisableCancelsQueuedPush(t *testing.T) {
	e := scopedAlertFixture(t)
	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	registerSyntheticPush(t, e, 1, syntheticPush(t, "scoped-budget"))
	enablePushType(t, e, "budget_threshold")
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70))
	alertTransaction(t, e, 1, 1, -7000, date)
	evaluateAlerts(t, e, now)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox WHERE type='budget_threshold'") != 1 {
		t.Fatal("no scoped push queued")
	}
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 1, false, 70))
	sends := 0
	e.a.pushSender = func(context.Context, webpush.Subscription, string, string, string, string, int64) (int, error) {
		sends++
		return 201, nil
	}
	if err := e.a.dispatchNotificationPush(now); err != nil {
		t.Fatal(err)
	}
	if sends != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox WHERE status='disabled'") != 1 {
		t.Fatal("disabled combination delivered queued push")
	}
}
func TestScopedBudgetMigrationRollbackRetryAndFinancialPreservation(t *testing.T) {
	e := scopedAlertFixture(t)
	alertTransaction(t, e, 1, 1, -1234, "2026-10-08")
	migrationExec(t, e.a.DB, "DROP TABLE notification_budget_scopes; DROP TABLE notification_budget_preferences; DELETE FROM migrations WHERE version>=29; CREATE TRIGGER reject_scope_version BEFORE INSERT ON migrations WHEN NEW.version=30 BEGIN SELECT RAISE(ABORT,'synthetic migration failure'); END")
	if err := migrate(e.a.DB); err == nil {
		t.Fatal("expected failed upgrade")
	}
	if queryInt(e.a.DB, "SELECT MAX(version) FROM migrations") != 28 || queryInt(e.a.DB, "SELECT COUNT(*) FROM sqlite_master WHERE name='notification_budget_preferences'") != 0 {
		t.Fatal("upgrade did not roll back pending batch")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_scope_version")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM transactions WHERE id=1") != -1234 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM group_targets WHERE period_id=1") != 30000 || queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions") == 0 {
		t.Fatal("upgrade changed financial/access state")
	}
}

func TestScopedBudgetPreferencesRetainedByBackupRestore(t *testing.T) {
	e := scopedAlertFixture(t)
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, false, 70), budgetAlertInput(900, 0, true, 90))
	snapshot, err := e.a.Backup()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored.sqlite")
	if err := Restore(target, snapshot); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(target, "http://localhost:8080", filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if queryInt(restored.DB, "SELECT threshold FROM notification_budget_preferences WHERE user_id=1 AND group_id=0") != 70 || queryInt(restored.DB, "SELECT enabled FROM notification_budget_preferences WHERE user_id=1 AND group_id=0") != 0 || queryInt(restored.DB, "SELECT threshold FROM notification_budget_preferences WHERE user_id=1 AND group_id=900") != 90 {
		t.Fatal("restore lost personal scoped choices")
	}
	if queryInt(restored.DB, "SELECT COUNT(*) FROM sessions") != 0 {
		t.Fatal("restore retained authenticated sessions")
	}
}

func TestScopedBudgetUniversalSaveAtomicityAndAuditPrivacy(t *testing.T) {
	e := scopedAlertFixture(t)
	body := func(alertVersion int64, threshold any) map[string]any {
		return map[string]any{"version": queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1"), "groups": []any{map[string]any{"group_id": 0, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 15000, "carry_forward": false}}}}, "alert_preferences": []any{budgetAlertInput(0, alertVersion, false, threshold)}}
	}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", body(0, 70)), 200)
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id IS NULL") != 15000 || queryInt(e.a.DB, "SELECT threshold FROM notification_budget_preferences WHERE user_id=1 AND group_id=0") != 70 {
		t.Fatal("universal save missed a field")
	}
	before := migrationSnapshot(t, e.a.DB, "SELECT * FROM periods", "SELECT * FROM group_targets", "SELECT * FROM targets", "SELECT * FROM notification_budget_preferences", "SELECT * FROM audit")
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", body(0, 90)), 409)
	if before != migrationSnapshot(t, e.a.DB, "SELECT * FROM periods", "SELECT * FROM group_targets", "SELECT * FROM targets", "SELECT * FROM notification_budget_preferences", "SELECT * FROM audit") {
		t.Fatal("stale alert preference partially committed budget")
	}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", body(1, 101)), 400)
	if before != migrationSnapshot(t, e.a.DB, "SELECT * FROM periods", "SELECT * FROM group_targets", "SELECT * FROM targets", "SELECT * FROM notification_budget_preferences", "SELECT * FROM audit") {
		t.Fatal("invalid threshold partially committed budget")
	}
	rows, err := data(e.a.DB, "SELECT * FROM audit WHERE entity='period' AND action='budget_built'")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(rows), "alert_preferences") {
		t.Fatal("shared budget audit disclosed personal preferences")
	}
}

func TestScopedBudgetCurrentPeriodOnlyDailySuppressionAndThresholdCarryover(t *testing.T) {
	e := scopedAlertFixture(t)
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, true, 70))
	migrationExec(t, e.a.DB, "INSERT INTO periods(id,name,start_date,end_date) VALUES(2,'Previous budget','2026-09-01','2026-09-30'),(3,'Next budget','2026-11-01','2026-11-30'); INSERT INTO group_targets(period_id,category_id,amount_cents) VALUES(2,1,10000),(3,1,20000); INSERT INTO targets VALUES(2,1,10000),(3,1,20000)")
	alertTransaction(t, e, 1, 1, -7000, "2026-10-08")
	alertTransaction(t, e, 2, 1, -99999, "2026-09-15")
	alertTransaction(t, e, 3, 1, -13999, "2026-11-01")
	migrationExec(t, e.a.DB, "UPDATE transactions SET period_id=2 WHERE id=2; UPDATE transactions SET period_id=3 WHERE id=3")
	for day := 15; day <= 18; day++ {
		evaluateAlerts(t, e, time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC))
	}
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("daily evaluation repeated a consumed current-period threshold")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=1 AND source_kind='budget' AND source_id IN (2,3)") != 0 {
		t.Fatal("past/future budget alerted before its active period")
	}
	evaluateAlerts(t, e, time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("next-period threshold rounded up below 70 percent")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET amount_cents=-14000 WHERE id=3; UPDATE allocations SET amount_cents=-14000 WHERE transaction_id=3")
	for day := 1; day <= 4; day++ {
		evaluateAlerts(t, e, time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC))
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=1 AND type='budget_threshold' AND source_id=3 AND message LIKE '%70%%'") != 1 {
		t.Fatal("saved threshold did not carry to the next period exactly once")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET amount_cents=-999999 WHERE id IN (1,2); UPDATE allocations SET amount_cents=-999999 WHERE transaction_id IN (1,2)")
	evaluateAlerts(t, e, time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC))
	if alertCount(e, "budget_threshold") != 2 || alertCount(e, "budget_overspend") != 0 {
		t.Fatal("editing prior-period spend generated historical alerts")
	}
}
