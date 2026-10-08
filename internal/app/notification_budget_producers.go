package app

import (
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"
)

var defaultBudgetNotificationThresholds = []int64{75, 90, 100}

// Integer ratios avoid float rounding and intermediate int64 overflow.
func ratioAtLeast(value, limit, numerator, denominator int64) bool {
	left := new(big.Int).Mul(big.NewInt(value), big.NewInt(denominator))
	right := new(big.Int).Mul(big.NewInt(limit), big.NewInt(numerator))
	return left.Cmp(right) >= 0
}
func scaledCents(value, numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	result := new(big.Int).Mul(big.NewInt(value), big.NewInt(numerator))
	result.Quo(result, big.NewInt(denominator))
	if !result.IsInt64() {
		return int64(^uint64(0) >> 1)
	}
	return result.Int64()
}
func alertMoney(cents int64) string { return fmt.Sprintf("R%d.%02d", cents/100, cents%100) }
func (a *App) evaluateBudgetAlertsTx(tx *sql.Tx, u User, now time.Time, baseline ...map[string]bool) error {
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	today := now.In(loc).Format("2006-01-02")
	periods, err := data(tx, "SELECT id,name,start_date,end_date FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", today, today)
	if err != nil || len(periods) == 0 {
		return err
	}
	p := periods[0]
	pid := num(p["id"])
	accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE a.household=1 AND a.sync_hidden=0"+strings.ReplaceAll(accountScopeSQL(u), "t.account_id", "a.id"))
	if err != nil {
		return err
	}
	ids := []int64{}
	for _, r := range accounts {
		ids = append(ids, num(r["id"]))
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := data(tx, `SELECT t.id,t.account_id,t.date,t.is_transfer,COALESCE(t.spending_group_id,0) group_id,l.amount_cents,c.id category_id,c.kind FROM transactions t JOIN accounts a ON a.id=t.account_id JOIN allocations l ON l.transaction_id=t.id LEFT JOIN categories c ON c.id=l.category_id WHERE a.household=1 AND a.sync_hidden=0 AND t.period_id=?`+accountScopeSQL(u), pid)
	if err != nil {
		return err
	}
	targets, err := data(tx, `SELECT c.id,c.name,b.amount_cents,b.group_id,COALESCE(g.name,'No spending group') group_name,COALESCE(pref.enabled,1) enabled,pref.threshold
 FROM (SELECT category_id,amount_cents,COALESCE(spending_group_id,0) group_id FROM group_targets WHERE period_id=? AND included=1
 UNION ALL SELECT category_id,amount_cents,0 FROM targets t WHERE period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets gt WHERE gt.period_id=t.period_id AND gt.category_id=t.category_id)) b
 JOIN categories c ON c.id=b.category_id LEFT JOIN spending_groups g ON g.id=b.group_id
 LEFT JOIN notification_budget_preferences pref ON pref.user_id=? AND pref.category_id=b.category_id AND pref.group_id=b.group_id
 WHERE c.kind='expense' ORDER BY b.group_id,c.id`, pid, pid, u.ID)
	if err != nil {
		return err
	}
	start, _ := time.Parse("2006-01-02", p["start_date"].(string))
	end, _ := time.Parse("2006-01-02", p["end_date"].(string))
	date, _ := time.Parse("2006-01-02", today)
	elapsed := int64(date.Sub(start)/(24*time.Hour)) + 1
	days := int64(end.Sub(start)/(24*time.Hour)) + 1
	allRows := rows
	for _, target := range targets {
		cid := num(target["id"])
		gid := num(target["group_id"])
		rows := []map[string]any{}
		for _, row := range allRows {
			if num(row["group_id"]) == gid {
				rows = append(rows, row)
			}
		}
		totals := categoryExpenseTotals(rows)
		thresholds := defaultBudgetNotificationThresholds
		if target["threshold"] != nil {
			thresholds = []int64{num(target["threshold"])}
		}
		quiet := false
		if len(baseline) > 0 {
			quiet = baseline[0][fmt.Sprintf("%d:%d", cid, gid)]
			if !quiet {
				continue
			}
		}
		enabled := num(target["enabled"]) == 1
		evaluate := func(e notificationEvent, scope string, active bool, cooldown time.Duration, reason string) error {
			if active && (!enabled || quiet) {
				return a.consumeConditionTx(tx, e, scope, now)
			}
			return a.evaluateConditionTx(tx, e, scope, active, cooldown, now, reason)
		}
		limit := num(target["amount_cents"])
		spent := totals[cid]
		base := budgetAlertScope(pid, cid, gid)
		name := target["name"].(string) + " · " + target["group_name"].(string)
		event := notificationEvent{BudgetCategoryID: cid, BudgetGroupID: gid, RecipientID: u.ID, Severity: "warning", SourceKind: "budget", SourceID: pid, AccountIDs: ids, Dismissible: true}
		// Coalesce an import jump to the highest crossed threshold. Still evaluate
		// every lower state, consuming it without sending when a higher one wins.
		highest := int64(0)
		for _, threshold := range thresholds {
			if limit > 0 && ratioAtLeast(spent, limit, threshold, 100) {
				highest = threshold
			}
		}
		for _, threshold := range thresholds {
			event.Type = "budget_threshold"
			event.Title = "Budget threshold reached"
			event.Message = fmt.Sprintf("%s has reached %d%% of its %s budget (%s of %s).", name, threshold, p["name"], alertMoney(spent), alertMoney(limit))
			condition := limit > 0 && ratioAtLeast(spent, limit, threshold, 100)
			// Highest state emits; lower crossings are consumed in the same transaction.
			if condition && threshold != highest {
				if err := a.consumeConditionTx(tx, event, fmt.Sprintf("%s:threshold:%d", base, threshold), now); err != nil {
					return err
				}
			} else if err := evaluate(event, fmt.Sprintf("%s:threshold:%d", base, threshold), condition, 0, "below_threshold"); err != nil {
				return err
			}
		}
		event.Type = "budget_overspend"
		event.Title = "Budget exceeded"
		event.Message = fmt.Sprintf("%s is %s over its %s budget: %s spent against %s.", name, alertMoney(max(spent-limit, 0)), p["name"], alertMoney(spent), alertMoney(limit))
		if err := evaluate(event, base+":overspend", spent > limit, 0, "at_or_below_budget"); err != nil {
			return err
		}
		spendingDays := map[string]bool{}
		for _, r := range rows {
			if num(r["category_id"]) == cid && num(r["is_transfer"]) == 0 && num(r["amount_cents"]) < 0 && r["date"].(string) <= today {
				spendingDays[r["date"].(string)] = true
			}
		}
		paceRows := []map[string]any{}
		for _, r := range rows {
			if r["date"].(string) <= today {
				paceRows = append(paceRows, r)
			}
		}
		paceSpent := categoryExpenseTotals(paceRows)[cid]
		projected := scaledCents(paceSpent, days, elapsed)
		significant := limit > 0 && elapsed >= 7 && elapsed < days && len(spendingDays) >= 3 && spent > 0 && spent <= limit && projected-limit >= 10000 && ratioAtLeast(projected, limit, 110, 100)
		event.Type = "budget_projection"
		event.Title = "Projected budget overrun"
		event.Message = fmt.Sprintf("%s projects to %s by the end of %s (%s over budget, %d%% of the limit), using %d of %d days at the current spending pace. This is an estimate.", name, alertMoney(projected), p["name"], alertMoney(max(projected-limit, 0)), scaledCents(projected, 100, max(limit, 1)), elapsed, days)
		reason := "below_significance"
		if elapsed < 7 || len(spendingDays) < 3 {
			reason = "insufficient_projection_history"
		}
		if spent > limit {
			reason = "actual_over_budget"
		}
		if elapsed >= days {
			reason = "period_complete"
		}
		if err := evaluate(event, base+":projection", significant, 24*time.Hour, reason); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) consumeConditionTx(tx *sql.Tx, e notificationEvent, scope string, now time.Time) error {
	key := hash(scope)
	_, err := tx.Exec(`INSERT INTO notification_conditions(user_id,type,scope_hash,active,cycle,consumed,evaluated_at,evaluated_run) VALUES(?,?,?,1,1,1,?,?) ON CONFLICT(user_id,type,scope_hash) DO UPDATE SET cycle=cycle+CASE WHEN active=0 THEN 1 ELSE 0 END,active=1,consumed=1,evaluated_at=excluded.evaluated_at,evaluated_run=excluded.evaluated_run`, e.RecipientID, e.Type, key, now.Unix(), queryInt(tx, "SELECT run FROM notification_evaluation_runs WHERE id=1"))
	if err != nil {
		return err
	}
	return notificationDiagnosticTx(tx, e.RecipientID, e.Type, key, "in_app", "coalesced", "", now)
}
