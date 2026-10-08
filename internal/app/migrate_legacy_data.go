package app

import (
	"database/sql"
	"encoding/json"
	"strings"
)

func backfillLegacySourceKeys(tx *sql.Tx) error {
	rows, err := data(tx, "SELECT id,source_date,source_amount,source_description FROM transactions WHERE source_key=''")
	if err != nil {
		return err
	}
	for _, row := range rows {
		key := fingerprint(SourceRow{Date: row["source_date"].(string), Amount: num(row["source_amount"]), Description: row["source_description"].(string)})
		if _, err := tx.Exec("UPDATE transactions SET source_key=? WHERE id=?", key, row["id"]); err != nil {
			return err
		}
	}

	return nil
}

func seedLegacySpendingGroups(tx *sql.Tx) error {
	for _, group := range [][2]string{{"Day-to-day", "blue"}, {"Recurring", "amber"}, {"Invest-save-repay", "purple"}, {"Exceptions", "orange"}, {"Income", "teal"}, {"Transfer", "slate"}, {"Bank Fees", "orange"}, {"Communications", "purple"}, {"Debt", "rose"}, {"Utilities", "blue"}, {"Insurance", "teal"}} {
		if _, err := tx.Exec("INSERT INTO spending_groups(name,color) VALUES(?,?)", group[0], group[1]); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyAcceptance(tx *sql.Tx) error {
	// Preserve the last explicit reviewer as having seen the unchanged entry.
	if _, err := tx.Exec(`INSERT OR IGNORE INTO transaction_seen(user_id,transaction_id,transaction_version,seen_at)
 SELECT reviewed_by,id,version,COALESCE(reviewed_at,CURRENT_TIMESTAMP) FROM transactions WHERE reviewed_by IS NOT NULL AND review_state='approved'`); err != nil {
		return err
	}
	ready := acceptanceSQL
	if _, err := tx.Exec(`INSERT INTO audit(account_id,entity,entity_id,action,details)
 SELECT account_id,'transaction',id,'automatically_accepted','{"reason":"category_acceptance_migration"}' FROM transactions WHERE review_state='pending_review' AND ` + ready); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE transactions SET review_state='approved',reviewed_by=NULL,reviewed_at=NULL,version=version+1 WHERE review_state='pending_review' AND " + ready); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE transactions SET review_state='pending_review',reviewed_by=NULL,reviewed_at=NULL,version=version+1 WHERE review_state='approved' AND NOT " + ready); err != nil {
		return err
	}
	return nil
}

func migrateLegacyBudgetLimits(tx *sql.Tx) error {
	if _, err := tx.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents) SELECT period_id,category_id,amount_cents FROM targets"); err != nil {
		return err
	}
	return nil
}

func migrateLegacyBudgetInclusion(tx *sql.Tx) error {
	if _, err := tx.Exec("UPDATE group_targets SET included=0 WHERE amount_cents=0"); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO budget_groups(period_id,spending_group_id) SELECT DISTINCT period_id,spending_group_id FROM group_targets WHERE included=1"); err != nil {
		return err
	}
	return nil
}

func migrateLegacyMCPPermissions(tx *sql.Tx) error {
	for _, write := range []bool{false, true} {
		encoded, err := json.Marshal(legacyMCPPermissions(write))
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE mcp_tokens SET permissions=? WHERE permissions='{}' AND can_write=?", string(encoded), write); err != nil {
			return err
		}
	}

	return nil
}

func migrateLegacyMerchantRules(tx *sql.Tx) error {
	groups, err := data(tx, "SELECT merchant_id, account_id, direction, COUNT(*) cnt FROM merchant_rules GROUP BY merchant_id, account_id, direction HAVING cnt > 1")
	if err != nil {
		return err
	}
	for _, g := range groups {
		var rows []map[string]any
		if g["account_id"] == nil {
			rows, err = data(tx, "SELECT id, pattern FROM merchant_rules WHERE merchant_id=? AND account_id IS NULL AND direction=? ORDER BY priority DESC, id ASC", g["merchant_id"], g["direction"])
		} else {
			rows, err = data(tx, "SELECT id, pattern FROM merchant_rules WHERE merchant_id=? AND account_id=? AND direction=? ORDER BY priority DESC, id ASC", g["merchant_id"], g["account_id"], g["direction"])
		}
		if err != nil {
			return err
		}
		if len(rows) <= 1 {
			continue
		}
		primaryID := num(rows[0]["id"])
		patterns := []string{}
		seen := map[string]bool{}
		var deleteIDs []any
		for i, r := range rows {
			p := strings.TrimSpace(r["pattern"].(string))
			if p != "" && !seen[p] {
				seen[p] = true
				patterns = append(patterns, p)
			}
			if i > 0 {
				deleteIDs = append(deleteIDs, r["id"])
			}
		}
		mergedPattern := strings.Join(patterns, " | ")
		if _, err := tx.Exec("UPDATE merchant_rules SET pattern=?, version=version+1 WHERE id=?", mergedPattern, primaryID); err != nil {
			return err
		}
		for _, delID := range deleteIDs {
			if _, err := tx.Exec("DELETE FROM merchant_rules WHERE id=?", delID); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateLegacyCategoryGroups(tx *sql.Tx) error {
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM builtin_rules WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM rules WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM group_targets WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM merchants WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name='Income' LIMIT 1) WHERE kind='income' AND spending_group_id IS NULL`); err != nil {
		return err
	}
	for _, m := range [][2]string{
		{"Groceries", "Day-to-day"},
		{"Eating out", "Day-to-day"},
		{"Transport & fuel", "Day-to-day"},
		{"Personal care", "Day-to-day"},
		{"Pets", "Day-to-day"},
		{"Clothing", "Day-to-day"},
		{"Entertainment", "Day-to-day"},
		{"Bank charges", "Bank Fees"},
		{"Phone & internet", "Communications"},
		{"Utilities", "Utilities"},
		{"Insurance", "Insurance"},
		{"Housing", "Recurring"},
		{"Subscriptions", "Recurring"},
		{"Education", "Recurring"},
		{"Home & garden", "Recurring"},
		{"Interest paid", "Recurring"},
		{"Gifts & donations", "Exceptions"},
		{"Travel", "Exceptions"},
		{"Medical", "Exceptions"},
	} {
		if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name=? COLLATE NOCASE LIMIT 1) WHERE lower(name)=lower(?) AND spending_group_id IS NULL`, m[1], m[0]); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name='Day-to-day' LIMIT 1) WHERE kind='expense' AND spending_group_id IS NULL`); err != nil {
		return err
	}
	return nil
}
