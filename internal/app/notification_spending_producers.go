package app

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type alertObservation struct {
	id, amount                      int64
	date                            time.Time
	category, merchant, description string
}

func medianAmount(values []alertObservation) int64 {
	amounts := make([]int64, len(values))
	for i, v := range values {
		amounts[i] = v.amount
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })
	return amounts[len(amounts)/2]
}
func recurringBaseline(values []alertObservation) (string, int64, bool) {
	if len(values) < 4 {
		return "", 0, false
	}
	cadence := "weekly"
	for i := 1; i < len(values); i++ {
		days := int(values[i].date.Sub(values[i-1].date) / (24 * time.Hour))
		if days < 5 || days > 9 {
			cadence = "monthly"
			break
		}
	}
	if cadence == "monthly" {
		for i := 1; i < len(values); i++ {
			previous, current := values[i-1].date, values[i].date
			months := int(current.Month()) - int(previous.Month()) + 12*(current.Year()-previous.Year())
			days := int(current.Sub(previous) / (24 * time.Hour))
			if months != 1 || days < 21 || days > 38 {
				return "", 0, false
			}
		}
	}
	baseline := medianAmount(values)
	if baseline < 5000 {
		return "", 0, false
	}
	for _, v := range values {
		if !ratioAtLeast(v.amount, baseline, 80, 100) || !ratioAtLeast(baseline, v.amount, 100, 120) {
			return "", 0, false
		}
	}
	return cadence, baseline, true
}
func expectedPaymentDate(last time.Time, cadence string) time.Time {
	if cadence == "weekly" {
		return last.AddDate(0, 0, 7)
	}
	first := time.Date(last.Year(), last.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	day := min(last.Day(), first.AddDate(0, 1, -1).Day())
	return first.AddDate(0, 0, day-1)
}
func (a *App) evaluateSpendingAlertsTx(tx *sql.Tx, u User, now time.Time) error {
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	today, _ := time.Parse("2006-01-02", now.In(loc).Format("2006-01-02"))
	accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE "+accountAccessSQL(u)+" AND a.sync_hidden=0", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		aid := num(account["id"])
		// Allocation-level expense sums count a split parent once per category,
		// exclude transfers/refunds and never read another account as a baseline.
		rows, err := data(tx, `SELECT t.id,t.date,COALESCE(t.merchant_id,0) merchant_id,t.description,c.id category_id,c.name category,
  -SUM(l.amount_cents) amount_cents FROM transactions t JOIN allocations l ON l.transaction_id=t.id
  JOIN categories c ON c.id=l.category_id AND c.kind='expense'
  WHERE t.account_id=? AND t.is_transfer=0 AND t.amount_cents<0 AND l.amount_cents<0 AND t.date>=? AND t.date<=?
  GROUP BY t.id,c.id ORDER BY t.date,t.id`, aid, today.AddDate(0, 0, -180).Format("2006-01-02"), today.Format("2006-01-02"))
		if err != nil {
			return err
		}
		groups := map[string][]alertObservation{}
		for _, r := range rows {
			date, _ := time.Parse("2006-01-02", r["date"].(string))
			merchant := fmt.Sprint(num(r["merchant_id"]))
			description := strings.Join(strings.Fields(strings.ToLower(r["description"].(string))), " ")
			// Exact normalized descriptions are a conservative fallback for merchants
			// not yet assigned. Never fuzzy-match distinct commitments.
			if merchant == "0" {
				merchant = hash(description)
			}
			key := fmt.Sprintf("account:%d:category:%d:merchant:%s", aid, num(r["category_id"]), merchant)
			groups[key] = append(groups[key], alertObservation{id: num(r["id"]), amount: num(r["amount_cents"]), date: date, category: r["category"].(string), merchant: merchant, description: description})
		}
		keys := make([]string, 0, len(groups))
		for key := range groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			values := groups[key]
			latest := values[len(values)-1]
			event := notificationEvent{RecipientID: u.ID, Severity: "warning", SourceKind: "transaction", SourceID: latest.id, AccountIDs: []int64{aid}, Dismissible: true}
			unusual := false
			baseline := int64(0)
			history := []alertObservation{}
			dates := map[string]bool{}
			for _, v := range values[:len(values)-1] {
				if v.date.Before(latest.date) {
					history = append(history, v)
					dates[v.date.Format("2006-01-02")] = true
				}
			}
			if len(history) >= 5 && len(dates) >= 5 && !latest.date.Before(today.AddDate(0, 0, -7)) {
				baseline = medianAmount(history)
				unusual = latest.amount >= 100000 && latest.amount-baseline >= 50000 && ratioAtLeast(latest.amount, baseline, 3, 1)
			}
			event.Type = "unusual_spending"
			event.Title = "Unusual spending"
			event.Message = fmt.Sprintf("This %s payment of %s is at least three times the median %s of %d earlier matching payments in the same account. Transfers and refunds are excluded.", latest.category, alertMoney(latest.amount), alertMoney(baseline), len(history))
			reason := "below_significance"
			if len(history) < 5 || len(dates) < 5 {
				reason = "insufficient_history"
			}
			if latest.date.Before(today.AddDate(0, 0, -7)) {
				reason = "stale_history"
			}
			if err := a.evaluateConditionTx(tx, event, key+":unusual", unusual, 7*24*time.Hour, now, reason); err != nil {
				return err
			}
			if err := a.evaluateRecurringTx(tx, event, key, values, today, now); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *App) evaluateRecurringTx(tx *sql.Tx, event notificationEvent, key string, values []alertObservation, today, now time.Time) error {
	latest := values[len(values)-1]
	event.Type = "recurring_payment"
	event.Title = "Recurring payment changed"
	condition := false
	reason := ""
	// Compare the latest payment with four preceding stable observations.
	if len(values) >= 5 && !latest.date.Before(today.AddDate(0, 0, -7)) {
		prior := values[len(values)-5 : len(values)-1]
		cadence, baseline, ok := recurringBaseline(prior)
		if ok {
			expected := expectedPaymentDate(prior[len(prior)-1].date, cadence)
			tolerance := 3
			if cadence == "monthly" {
				tolerance = 7
			}
			delta := latest.amount - baseline
			if delta < 0 {
				delta = -delta
			}
			changedAmount := delta >= 5000 && ratioAtLeast(delta, baseline, 20, 100)
			drift := int(latest.date.Sub(expected) / (24 * time.Hour))
			changedCadence := drift > tolerance || drift < -tolerance
			if changedAmount {
				reason = fmt.Sprintf("The latest %s %s payment is %s versus a typical %s across four stable payments.", cadence, latest.category, alertMoney(latest.amount), alertMoney(baseline))
			} else if changedCadence {
				reason = fmt.Sprintf("The latest %s %s payment arrived outside the expected %d-day date tolerance.", cadence, latest.category, tolerance)
			}
			condition = changedAmount || changedCadence
		}
	}
	// Missing payments use an account source. New evidence resolves this state;
	// one continuing gap never emits a fresh alert on each scheduler pass.
	if !condition && len(values) >= 4 {
		prior := values[len(values)-4:]
		cadence, _, ok := recurringBaseline(prior)
		if ok {
			expected := expectedPaymentDate(latest.date, cadence)
			tolerance := 3
			if cadence == "monthly" {
				tolerance = 7
			}
			// Stop stale historical commitments becoming alerts at feature activation.
			condition = today.After(expected.AddDate(0, 0, tolerance)) && !today.After(expected.AddDate(0, 0, 35))
			if condition {
				event.SourceKind = "account"
				event.SourceID = event.AccountIDs[0]
				reason = fmt.Sprintf("A %s %s payment has not appeared after the expected date %s plus %d days of tolerance, based on four stable payments.", cadence, latest.category, expected.Format("2006-01-02"), tolerance)
			}
		}
	}
	event.Message = reason
	if !condition {
		event.Message = "The recurring payment pattern is within its expected range."
	}
	code := "within_expected_range"
	if len(values) < 4 {
		code = "insufficient_history"
	} else {
		_, _, stable := recurringBaseline(values[len(values)-4:])
		if !stable {
			code = "unstable_pattern"
		}
	}
	return a.evaluateConditionTx(tx, event, key+":recurring", condition, 7*24*time.Hour, now, code)
}
