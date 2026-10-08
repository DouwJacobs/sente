package app

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func alertTestTime() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }
func alertFixture(t *testing.T) *testEnv {
	e := setup(t)
	migrationExec(t, e.a.DB, "UPDATE periods SET start_date='2026-10-01',end_date='2026-10-31'; INSERT INTO targets VALUES(1,1,10000)")
	return e
}
func alertTransaction(t *testing.T, e *testEnv, id, account, amount int64, date string) {
	t.Helper()
	_, err := e.a.DB.Exec(`INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance,period_id,review_state) VALUES(?,?,?,?,'Synthetic payment',?,?,'Synthetic payment',?,'{}',1,'approved')`, id, account, date, amount, date, amount, fmt.Sprint(id))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,1,?)", id, amount)
	if err != nil {
		t.Fatal(err)
	}
}
func evaluateAlerts(t *testing.T, e *testEnv, now time.Time) {
	t.Helper()
	if err := e.a.evaluateNotificationAlerts(now); err != nil {
		t.Fatal(err)
	}
}
func alertCount(e *testEnv, kind string) int64 {
	return queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=1 AND type=?", kind)
}
func changeAlertAmount(t *testing.T, e *testEnv, amount int64) {
	t.Helper()
	_, err := e.a.DB.Exec("UPDATE transactions SET amount_cents=? WHERE id=1; UPDATE allocations SET amount_cents=? WHERE transaction_id=1", amount, amount)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNotificationBudgetBoundariesResetAndPrivacy(t *testing.T) {
	e := alertFixture(t)
	now := alertTestTime()
	alertTransaction(t, e, 1, 1, -7499, "2026-10-15")
	alertTransaction(t, e, 2, 2, -999999, "2026-10-15")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("below threshold alerted")
	}
	for i, amount := range []int64{-7500, -9000, -10000} {
		changeAlertAmount(t, e, amount)
		evaluateAlerts(t, e, now)
		if alertCount(e, "budget_threshold") != int64(i+1) {
			t.Fatal("threshold boundary", amount)
		}
	}
	if alertCount(e, "budget_overspend") != 0 {
		t.Fatal("exactly 100 percent is not overspent")
	}
	changeAlertAmount(t, e, -10001)
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_overspend") != 1 {
		t.Fatal("over budget not alerted")
	}
	migrationExec(t, e.a.DB, "INSERT INTO accounts(id,name,bank_id,household) VALUES(3,'Empty household','synthetic-extra',1)")
	for i := 0; i < 3; i++ {
		evaluateAlerts(t, e, now)
	}
	if alertCount(e, "budget_overspend") != 1 {
		t.Fatal("repeated evaluation duplicated")
	}
	changeAlertAmount(t, e, -6000)
	evaluateAlerts(t, e, now)
	changeAlertAmount(t, e, -10001)
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 4 || alertCount(e, "budget_overspend") != 2 {
		t.Fatal("reset/re-cross did not coalesce highest threshold")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_accounts WHERE account_id=2") != 0 {
		t.Fatal("private dependency entered shared budget")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=3 AND source_kind='budget'") != 0 {
		t.Fatal("non-member budget recipient")
	}
	// Authoritative dashboard uses the exact shared allocation arithmetic.
	w := e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"spent_cents":10001`) {
		t.Fatal(w.Body.String())
	}
	migrationExec(t, e.a.DB, "UPDATE targets SET amount_cents=20000 WHERE period_id=1")
	evaluateAlerts(t, e, now)
	migrationExec(t, e.a.DB, "UPDATE targets SET amount_cents=10000 WHERE period_id=1")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_overspend") != 3 {
		t.Fatal("limit changes did not reset crossing")
	}
}
func TestNotificationBudgetRefundSplitsTransfersPeriodAndProjection(t *testing.T) {
	e := alertFixture(t)
	now := alertTestTime()
	migrationExec(t, e.a.DB, "UPDATE targets SET amount_cents=100000")
	for i, date := range []string{"2026-10-01", "2026-10-03", "2026-10-05"} {
		alertTransaction(t, e, int64(i+1), 1, -20000, date)
	}
	evaluateAlerts(t, e, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	if alertCount(e, "budget_projection") != 0 {
		t.Fatal("early projection noise")
	}
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_projection") != 1 {
		t.Fatal("significant projection missing")
	}
	evaluateAlerts(t, e, now.Add(time.Hour))
	if alertCount(e, "budget_projection") != 1 {
		t.Fatal("projection repeated")
	}
	alertTransaction(t, e, 4, 1, 50000, "2026-10-15")
	evaluateAlerts(t, e, now.Add(2*time.Hour))
	if queryInt(e.a.DB, "SELECT active FROM notification_conditions WHERE type='budget_projection' AND user_id=1") != 0 {
		t.Fatal("refund failed to reset projection")
	}
	migrationExec(t, e.a.DB, "DELETE FROM transactions WHERE id=4; UPDATE transactions SET is_transfer=1 WHERE id=1")
	evaluateAlerts(t, e, now.Add(25*time.Hour))
	if alertCount(e, "budget_projection") != 1 {
		t.Fatal("transfer principal projected")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET is_transfer=0 WHERE id=1; UPDATE allocations SET amount_cents=-10000 WHERE transaction_id=1; INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,2,-10000)")
	evaluateAlerts(t, e, now.Add(26*time.Hour))
	// Only expense allocation contributes, and there are still three spending days.
	if alertCount(e, "budget_projection") != 1 {
		t.Fatal("split allocation counted twice")
	}
	migrationExec(t, e.a.DB, "UPDATE allocations SET amount_cents=-20000 WHERE transaction_id=1 AND category_id=1; DELETE FROM allocations WHERE transaction_id=1 AND category_id=2")
	evaluateAlerts(t, e, now.Add(27*time.Hour))
	if alertCount(e, "budget_projection") != 2 {
		t.Fatal("new projection crossing missing")
	}
	migrationExec(t, e.a.DB, "INSERT INTO periods(id,name,start_date,end_date) VALUES(2,'Overlap','2026-10-10','2026-10-31'); INSERT INTO targets VALUES(2,1,10000); UPDATE transactions SET period_id=2 WHERE id=1")
	evaluateAlerts(t, e, now.Add(28*time.Hour))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE source_id=2 AND type='budget_overspend'") != 1 {
		t.Fatal("latest-start active period not selected")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_conditions WHERE active=1 AND evaluated_run<>(SELECT run FROM notification_evaluation_runs)") != 0 {
		t.Fatal("absent scope not reset")
	}
}
func TestNotificationProducerConcurrentRollbackAndDisabledOccurrence(t *testing.T) {
	e := alertFixture(t)
	now := alertTestTime()
	alertTransaction(t, e, 1, 1, -9500, "2026-10-15")
	migrationExec(t, e.a.DB, `CREATE TRIGGER reject_alert BEFORE INSERT ON notifications BEGIN SELECT RAISE(ABORT,'synthetic'); END`)
	if err := e.a.evaluateNotificationAlerts(now); err == nil {
		t.Fatal("expected emission rollback")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_conditions") != 0 {
		t.Fatal("failed evaluation consumed state")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_alert; INSERT INTO notification_preferences(user_id,type,channel,enabled) VALUES(1,'budget_threshold','in_app',0)")
	evaluateAlerts(t, e, now)
	migrationExec(t, e.a.DB, "UPDATE notification_preferences SET enabled=1")
	evaluateAlerts(t, e, now)
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("enabling replayed an old disabled crossing")
	}
	changeAlertAmount(t, e, -5000)
	evaluateAlerts(t, e, now)
	changeAlertAmount(t, e, -9500)
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); failures <- e.a.evaluateNotificationAlerts(now) }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("concurrent crossing duplicated")
	}
}
func TestNotificationUnusualHistoryTransfersAndRecipientScope(t *testing.T) {
	e := alertFixture(t)
	now := alertTestTime()
	for i := int64(1); i <= 5; i++ {
		alertTransaction(t, e, i, 2, -10000, fmt.Sprintf("2026-10-%02d", i))
	}
	alertTransaction(t, e, 6, 2, -100000, "2026-10-15")
	evaluateAlerts(t, e, now)
	if alertCount(e, "unusual_spending") != 1 {
		t.Fatal("high-confidence outlier missing")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=3 AND type='unusual_spending'") != 0 {
		t.Fatal("private baseline leaked")
	}
	evaluateAlerts(t, e, now)
	if alertCount(e, "unusual_spending") != 1 {
		t.Fatal("outlier repeated")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET is_transfer=1 WHERE id=6")
	evaluateAlerts(t, e, now)
	if queryInt(e.a.DB, "SELECT active FROM notification_conditions WHERE user_id=1 AND type='unusual_spending'") != 0 {
		t.Fatal("transfer edit failed to resolve")
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET is_transfer=0 WHERE id=6; DELETE FROM grants WHERE user_id=1 AND account_id=2")
	evaluateAlerts(t, e, now)
	if strings.Contains(e.req(t, 1, "/api/notifications", "GET", nil).Body.String(), "Unusual spending") {
		t.Fatal("revocation retained readable payload")
	}
}
func TestNotificationRecurringWeeklyMonthlyVariableAndDrift(t *testing.T) {
	observations := func(dates []string, amounts []int64) []alertObservation {
		out := []alertObservation{}
		for i, date := range dates {
			d, _ := time.Parse("2006-01-02", date)
			out = append(out, alertObservation{date: d, amount: amounts[i]})
		}
		return out
	}
	cases := []struct {
		name    string
		dates   []string
		amounts []int64
		cadence string
		valid   bool
	}{
		{"weekly", []string{"2026-09-17", "2026-09-24", "2026-10-01", "2026-10-08"}, []int64{10000, 10000, 10000, 10000}, "weekly", true},
		{"monthly drift variable", []string{"2026-06-28", "2026-07-30", "2026-08-28", "2026-09-30"}, []int64{10000, 11000, 9000, 10000}, "monthly", true},
		{"skipped month", []string{"2026-05-01", "2026-06-01", "2026-08-01", "2026-09-01"}, []int64{10000, 10000, 10000, 10000}, "", false},
		{"repeated purchases", []string{"2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04"}, []int64{10000, 10000, 10000, 10000}, "", false},
		{"too variable", []string{"2026-06-01", "2026-07-01", "2026-08-01", "2026-09-01"}, []int64{10000, 20000, 8000, 30000}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cadence, _, valid := recurringBaseline(observations(c.dates, c.amounts))
			if cadence != c.cadence || valid != c.valid {
				t.Fatal(cadence, valid)
			}
		})
	}
	jan31, _ := time.Parse("2006-01-02", "2026-01-31")
	if expectedPaymentDate(jan31, "monthly").Format("2006-01-02") != "2026-02-28" {
		t.Fatal("monthly clamp")
	}
	e := alertFixture(t)
	for i, date := range []string{"2026-09-17", "2026-09-24", "2026-10-01", "2026-10-08"} {
		alertTransaction(t, e, int64(i+1), 2, -10000, date)
	}
	evaluateAlerts(t, e, alertTestTime())
	if alertCount(e, "recurring_payment") != 0 {
		t.Fatal("before date tolerance missed")
	}
	evaluateAlerts(t, e, alertTestTime().AddDate(0, 0, 4))
	if alertCount(e, "recurring_payment") != 1 {
		t.Fatal("missed weekly payment absent")
	}
	evaluateAlerts(t, e, alertTestTime().AddDate(0, 0, 5))
	if alertCount(e, "recurring_payment") != 1 {
		t.Fatal("missed payment repeated")
	}
}
func TestNotificationRecurringAmountChange(t *testing.T) {
	e := alertFixture(t)
	for i, date := range []string{"2026-09-17", "2026-09-24", "2026-10-01", "2026-10-08", "2026-10-15"} {
		amount := int64(-10000)
		if i == 4 {
			amount = -15000
		}
		alertTransaction(t, e, int64(i+1), 2, amount, date)
	}
	evaluateAlerts(t, e, alertTestTime())
	if alertCount(e, "recurring_payment") != 1 {
		t.Fatal("changed recurring amount missing")
	}
	// Financial mutation remains committed if a later independent evaluation fails.
	if queryInt(e.a.DB, "SELECT amount_cents FROM transactions WHERE id=5") != -15000 {
		t.Fatal("detector changed ledger")
	}
}
func TestNotificationDiagnosticAuthorizationRedactionAndRetention(t *testing.T) {
	e := alertFixture(t)
	alertTransaction(t, e, 1, 1, -8000, "2026-10-15")
	evaluateAlerts(t, e, time.Now())
	evaluateAlerts(t, e, time.Now())
	status(t, e.req(t, 3, "/api/notifications/diagnostics", "GET", nil), 403)
	w := e.req(t, 1, "/api/notifications/diagnostics?type=budget_threshold&user=1&days=14", "GET", nil)
	status(t, w, 200)
	for _, secret := range []string{"Synthetic payment", "Groceries", "account_id", "endpoint", "p256dh", "auth", "title", "message", "budget:1"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("diagnostic payload exposed", secret)
		}
	}
	if !strings.Contains(w.Body.String(), `"username":"owner"`) || !strings.Contains(w.Body.String(), "delivered") || strings.Contains(w.Body.String(), "deduplicated") || strings.Contains(w.Body.String(), "below_threshold") {
		t.Fatal(w.Body.String())
	}
	status(t, e.req(t, 1, "/api/notifications/diagnostics?days=15", "GET", nil), 400)
	migrationExec(t, e.a.DB, "UPDATE notification_diagnostics SET evaluated_at=1")
	if err := e.a.writeQuiet(func(tx *sql.Tx) error { return pruneAlertStateTx(tx, time.Now()) }); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics") != 0 {
		t.Fatal("retention")
	}
}

func TestNotificationCooldownResolvedAndNewCycle(t *testing.T) {
	e := setup(t)
	now := alertTestTime()
	event := notificationEvent{RecipientID: 1, Type: "unusual_spending", Severity: "warning", Title: "Synthetic alert", Message: "Synthetic condition", SourceKind: "account", SourceID: 1, AccountIDs: []int64{1}, Dismissible: true}
	evaluate := func(active bool, at time.Time) {
		t.Helper()
		if err := e.a.writeQuiet(func(tx *sql.Tx) error {
			return e.a.evaluateConditionTx(tx, event, "synthetic-cooldown", active, 7*24*time.Hour, at)
		}); err != nil {
			t.Fatal(err)
		}
	}
	evaluate(true, now)
	evaluate(false, now.Add(time.Minute))
	evaluate(true, now.Add(time.Hour))
	if alertCount(e, "unusual_spending") != 1 {
		t.Fatal("cooldown ignored")
	}
	evaluate(true, now.AddDate(0, 0, 8))
	evaluate(true, now.AddDate(0, 0, 8))
	if alertCount(e, "unusual_spending") != 2 {
		t.Fatal("eligible new cycle not delivered once")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics WHERE status='cooldown_suppressed'") == 0 {
		t.Fatal("cooldown reason absent")
	}
}
func TestNotificationRecurringMonthlyDecreaseAndMissedMonth(t *testing.T) {
	e := alertFixture(t)
	for i, date := range []string{"2026-06-15", "2026-07-15", "2026-08-15", "2026-09-15", "2026-10-15"} {
		amount := int64(-20000)
		if i == 4 {
			amount = -10000
		}
		alertTransaction(t, e, int64(i+1), 2, amount, date)
	}
	evaluateAlerts(t, e, alertTestTime())
	if alertCount(e, "recurring_payment") != 1 {
		t.Fatal("material monthly decrease missing")
	}
	migrationExec(t, e.a.DB, "DELETE FROM transactions WHERE id=5")
	evaluateAlerts(t, e, alertTestTime().AddDate(0, 0, 8))
	// The ongoing changed/missed episode remains one alert until resolved.
	if alertCount(e, "recurring_payment") != 1 {
		t.Fatal("same monthly episode repeated")
	}
}
