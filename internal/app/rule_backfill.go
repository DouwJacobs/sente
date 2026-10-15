package app

import (
	"database/sql"
	"encoding/json"
	"finance-tracker/internal/classification"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

type ruleBackfillInput struct {
	Refs     []RuleRef `json:"refs"`
	PeriodID int64     `json:"period_id"`
}
type ruleBackfillSeal struct {
	Input       ruleBackfillInput `json:"input"`
	Fingerprint string            `json:"fingerprint"`
}
type ruleBackfillChange struct {
	ID             int64             `json:"id"`
	AccountID      int64             `json:"account_id"`
	Account        string            `json:"account"`
	Version        int64             `json:"version"`
	Date           string            `json:"date"`
	Description    string            `json:"description"`
	Amount         int64             `json:"amount_cents"`
	BeforeMerchant *int64            `json:"before_merchant"`
	BeforeCategory *int64            `json:"before_category"`
	BeforeGroup    *int64            `json:"before_group"`
	Merchant       *int64            `json:"merchant_id"`
	Category       *int64            `json:"category_id"`
	Group          *int64            `json:"spending_group_id"`
	Before         map[string]string `json:"before"`
	After          map[string]string `json:"after"`
	AllocationID   int64             `json:"allocation_id"`
}
type ruleBackfillResult struct {
	Items       []ruleBackfillChange `json:"items"`
	Conflicts   int                  `json:"conflicts"`
	Skipped     int                  `json:"skipped"`
	Matched     int                  `json:"matched"`
	Fingerprint string               `json:"-"`
}

func ruleMatch(description, direction string, amount int64, pattern string, merchant bool) bool {
	if direction == "debit" && amount >= 0 || direction == "credit" && amount <= 0 {
		return false
	}
	if merchant {
		return classification.MatchPattern(description, pattern)
	}
	return containsNormalized(description, pattern)
}
func containsNormalized(description, pattern string) bool {
	return strings.Contains(normalize(description), normalize(pattern))
}
func pointerValue(row map[string]any, key string) *int64 {
	if row[key] == nil {
		return nil
	}
	value := num(row[key])
	return &value
}
func (a *App) previewRuleBackfillTx(tx *sql.Tx, u User, b ruleBackfillInput) (ruleBackfillResult, error) {
	out := ruleBackfillResult{Items: []ruleBackfillChange{}}
	if len(b.Refs) == 0 || len(b.Refs) > 10000 || b.PeriodID < 0 {
		return out, fail(400, "Choose saved rules and a valid period")
	}
	if b.PeriodID != 0 {
		if !u.Member {
			return out, fail(403, "Budget membership required")
		}
		if queryInt(tx, "SELECT COUNT(*) FROM periods WHERE id=?", b.PeriodID) != 1 {
			return out, fail(400, "Choose an existing budget period")
		}
	}
	selected := []map[string]any{}
	seen := map[string]bool{}
	for _, ref := range b.Refs {
		table, id, err := ruleTable(ref.Kind, ref.ID)
		if err != nil {
			return out, err
		}
		key := fmt.Sprintf("%s:%d", ref.Kind, ref.ID)
		if seen[key] {
			return out, fail(400, "Choose each rule once")
		}
		seen[key] = true
		records, err := data(tx, "SELECT * FROM "+table+" WHERE id=?", id)
		if err != nil {
			return out, err
		}
		if len(records) != 1 {
			return out, fail(404, "Rule not found")
		}
		record := records[0]
		if num(record["version"]) != ref.Version {
			return out, fail(409, "Rule changed. Reload it before previewing")
		}
		if num(record["enabled"]) != 1 {
			return out, fail(400, "Resume the rule before applying it")
		}
		account := num(record["account_id"])
		if ref.Kind == "merchant" {
			if !merchantScopeAccess(tx, a, u, account) {
				return out, fail(403, "Merchant rule access required")
			}
		} else if ref.ID < 0 {
			if !u.Member {
				return out, fail(403, "Budget membership required")
			}
		} else if !ruleAccess(tx, a, u, account) {
			return out, fail(403, "Account editor access required")
		}
		record["ref"] = ref
		selected = append(selected, record)
	}
	// Only accounts in the selected rule scopes with current editor access are read.
	accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE "+accountAccessSQL(u)+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) ORDER BY a.id", u.Member, u.ID, u.Member, u.ID)
	if err != nil {
		return out, err
	}
	_, config, err := configurationSnapshot(tx)
	if err != nil {
		return out, err
	}
	evidence := []any{b, config}
	if b.PeriodID != 0 {
		periods, err := data(tx, "SELECT id,name,start_date,end_date,version FROM periods WHERE id=?", b.PeriodID)
		if err != nil {
			return out, err
		}
		evidence = append(evidence, periods)
	}
	scanned := 0
	readCount := 0
	for _, acc := range accounts {
		account := num(acc["id"])
		inScope := false
		for _, rule := range selected {
			scope := num(rule["account_id"])
			inScope = inScope || scope == 0 || scope == account
		}
		if !inScope {
			continue
		}
		rules, err := loadRules(tx, account)
		if err != nil {
			return out, err
		}
		rows, err := data(tx, `SELECT t.id,t.account_id,t.version,t.date,t.description,t.amount_cents,t.merchant_id,t.spending_group_id,t.is_transfer,t.period_id,t.assignment,a.name account,
  (SELECT COUNT(*) FROM allocations WHERE transaction_id=t.id) allocation_count,
  (SELECT id FROM allocations WHERE transaction_id=t.id ORDER BY id LIMIT 1) allocation_id,
  (SELECT category_id FROM allocations WHERE transaction_id=t.id ORDER BY id LIMIT 1) category_id,
  (SELECT amount_cents FROM allocations WHERE transaction_id=t.id ORDER BY id LIMIT 1) allocation_amount
  FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE t.account_id=? AND (?=0 OR a.household=1 AND t.period_id=?) ORDER BY t.id LIMIT 100001`, account, b.PeriodID, b.PeriodID)
		if err != nil {
			return out, err
		}
		readCount += len(rows)
		if readCount > 100000 {
			return out, fail(400, "Too many transactions to check. Choose a specific budget period")
		}
		for _, row := range rows {
			refs := []RuleRef{}
			for _, rule := range selected {
				scope := num(rule["account_id"])
				ref := rule["ref"].(RuleRef)
				if scope != 0 && scope != account {
					continue
				}
				if ruleMatch(row["description"].(string), rule["direction"].(string), num(row["amount_cents"]), rule["pattern"].(string), ref.Kind == "merchant") {
					refs = append(refs, ref)
				}
			}
			if len(refs) == 0 {
				continue
			}
			scanned++
			if scanned > 10000 {
				return out, fail(400, "More than 10,000 matches. Choose a specific budget period")
			}
			out.Matched++
			evidence = append(evidence, row)
			if num(row["is_transfer"]) == 1 || num(row["allocation_count"]) != 1 || num(row["allocation_amount"]) != num(row["amount_cents"]) {
				out.Skipped++
				continue
			}
			result, merchant, err := evaluateDescription(tx, account, row["description"].(string), num(row["amount_cents"]), rules)
			if err != nil {
				return out, err
			}
			if result.RuleConflict {
				out.Conflicts++
				continue
			}
			merchantSelected, categorySelected := false, false
			for _, ref := range refs {
				if ref.Kind == "merchant" && merchant != nil {
					for _, id := range merchant.RuleIDs {
						merchantSelected = merchantSelected || ref.ID == id
					}
				}
				if ref.Kind == "category" {
					for _, match := range result.RuleMatches {
						categorySelected = categorySelected || ref.ID == match.ID
					}
				}
			}
			change := ruleBackfillChange{ID: num(row["id"]), AccountID: account, Account: row["account"].(string), Version: num(row["version"]), Date: row["date"].(string), Description: row["description"].(string), Amount: num(row["amount_cents"]), AllocationID: num(row["allocation_id"]), BeforeMerchant: pointerValue(row, "merchant_id"), BeforeCategory: pointerValue(row, "category_id"), BeforeGroup: pointerValue(row, "spending_group_id")}
			change.Merchant, change.Category, change.Group = change.BeforeMerchant, change.BeforeCategory, change.BeforeGroup
			if merchantSelected && change.Merchant == nil {
				change.Merchant = result.MerchantID
			}
			// Category-only actions cannot piggyback classification from a winning merchant.
			if categorySelected || merchantSelected && merchant != nil && merchant.CategoryID != nil {
				if change.Category == nil {
					change.Category = result.CategoryID
				}
			}
			if categorySelected || merchantSelected && merchant != nil && merchant.SpendingGroupID != nil {
				if change.Group == nil {
					change.Group = result.SpendingGroupID
				}
			}
			if reflect.DeepEqual(change.Merchant, change.BeforeMerchant) && reflect.DeepEqual(change.Category, change.BeforeCategory) && reflect.DeepEqual(change.Group, change.BeforeGroup) {
				out.Skipped++
				continue
			}
			change.Before = map[string]string{}
			change.After = map[string]string{}
			for _, field := range []struct {
				key, table    string
				before, after *int64
			}{{"merchant", "merchants", change.BeforeMerchant, change.Merchant}, {"category", "categories", change.BeforeCategory, change.Category}, {"group", "spending_groups", change.BeforeGroup, change.Group}} {
				before, after := "Not set", "Not set"
				if field.before != nil {
					if err := tx.QueryRow("SELECT name FROM "+field.table+" WHERE id=?", field.before).Scan(&before); err != nil {
						return out, err
					}
				}
				if field.after != nil {
					if err := tx.QueryRow("SELECT name FROM "+field.table+" WHERE id=?", field.after).Scan(&after); err != nil {
						return out, err
					}
				}
				change.Before[field.key], change.After[field.key] = before, after
			}
			out.Items = append(out.Items, change)
		}
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].ID < out.Items[j].ID })
	raw, _ := json.Marshal([]any{evidence, out.Items, out.Conflicts, out.Skipped})
	out.Fingerprint = hash(string(raw))
	return out, nil
}
func (a *App) ruleBackfillPreview(w http.ResponseWriter, r *http.Request) error {
	var b ruleBackfillInput
	if err := decode(r, &b); err != nil {
		return err
	}
	var out ruleBackfillResult
	var token string
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		var err error
		out, err = a.previewRuleBackfillTx(tx, u, b)
		if err == nil {
			token = sealWorkflow(u, ruleBackfillSeal{b, out.Fingerprint})
		}
		return err
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": out.Items, "conflicts": out.Conflicts, "skipped": out.Skipped, "matched": out.Matched, "token": token})
	return nil
}
func (a *App) ruleBackfillApply(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Token string `json:"token"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	count := 0
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		var seal ruleBackfillSeal
		if err := openWorkflow(u, b.Token, &seal); err != nil {
			return err
		}
		out, err := a.previewRuleBackfillTx(tx, u, seal.Input)
		if err != nil {
			return err
		}
		if out.Fingerprint != seal.Fingerprint {
			return fail(409, "Transactions, rules or access changed. Preview again before applying")
		}
		for _, change := range out.Items {
			res, err := tx.Exec("UPDATE transactions SET merchant_id=COALESCE(merchant_id,?),spending_group_id=COALESCE(spending_group_id,?),reviewed_at=NULL,reviewed_by=NULL,version=version+1 WHERE id=? AND account_id=? AND version=?", change.Merchant, change.Group, change.ID, change.AccountID, change.Version)
			if err != nil {
				return err
			}
			if err = affected(res); err != nil {
				return err
			}
			if change.BeforeCategory == nil && change.Category != nil {
				res, err = tx.Exec("UPDATE allocations SET category_id=? WHERE id=? AND transaction_id=? AND category_id IS NULL", change.Category, change.AllocationID, change.ID)
				if err != nil {
					return err
				}
				if err = affected(res); err != nil {
					return err
				}
			}
			if err = acceptCategorized(tx, u, change.AccountID, change.ID); err != nil {
				return err
			}
			if err = audit(tx, u, change.AccountID, "transaction", change.ID, "rule_backfilled", map[string]any{"rules": seal.Input.Refs, "period_id": seal.Input.PeriodID, "change": change}); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"applied": count})
	return nil
}
