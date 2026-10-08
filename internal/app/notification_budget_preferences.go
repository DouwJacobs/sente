package app

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type budgetAlertPreference struct {
	CategoryID   int64  `json:"category_id"`
	GroupID      int64  `json:"group_id"`
	CategoryName string `json:"category_name"`
	GroupName    string `json:"group_name"`
	Enabled      bool   `json:"enabled"`
	Threshold    *int64 `json:"threshold"`
	Version      int64  `json:"version"`
}
type budgetAlertPreferenceInput struct {
	CategoryID int64  `json:"category_id"`
	GroupID    int64  `json:"group_id"`
	Enabled    *bool  `json:"enabled"`
	Threshold  *int64 `json:"threshold"`
	Version    *int64 `json:"version"`
}

func budgetAlertScope(pid, cid, gid int64) string {
	return fmt.Sprintf("household-budget:%d:category:%d:group:%d", pid, cid, gid)
}

const budgetAlertPairsSQL = `SELECT category_id,COALESCE(spending_group_id,0) group_id FROM group_targets WHERE included=1
 UNION SELECT category_id,0 FROM targets t WHERE NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id)`

func readBudgetAlertPreferences(q queryer, u User) ([]budgetAlertPreference, error) {
	items := []budgetAlertPreference{}
	if !u.Member {
		return items, nil
	}
	rows, err := q.Query(`SELECT c.id,s.group_id,c.name,COALESCE(g.name,'No spending group'),COALESCE(p.enabled,1),p.threshold,COALESCE(p.version,0)
 FROM (`+budgetAlertPairsSQL+`) s JOIN categories c ON c.id=s.category_id LEFT JOIN spending_groups g ON g.id=s.group_id
 LEFT JOIN notification_budget_preferences p ON p.user_id=? AND p.category_id=c.id AND p.group_id=s.group_id
 WHERE c.kind='expense' ORDER BY COALESCE(g.name,'No spending group'),c.name,c.id`, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item budgetAlertPreference
		if err := rows.Scan(&item.CategoryID, &item.GroupID, &item.CategoryName, &item.GroupName, &item.Enabled, &item.Threshold, &item.Version); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func saveBudgetAlertPreferencesTx(tx *sql.Tx, u User, inputs []budgetAlertPreferenceInput) error {
	if len(inputs) == 0 {
		return nil
	}
	if err := requireMember(u); err != nil {
		return err
	}
	if len(inputs) > 1000 {
		return fail(400, "Choose at most 1000 budget alert settings")
	}
	seen := map[string]bool{}
	for _, input := range inputs {
		key := fmt.Sprintf("%d:%d", input.CategoryID, input.GroupID)
		if input.CategoryID <= 0 || input.GroupID < 0 || input.Enabled == nil || input.Version == nil || *input.Version < 0 || seen[key] {
			return fail(400, "Choose valid, distinct budget alert settings")
		}
		if input.Threshold != nil && (*input.Threshold < 1 || *input.Threshold > 100) {
			return fail(400, "Alert threshold must be a whole percentage from 1 to 100")
		}
		seen[key] = true
		if queryInt(tx, `SELECT COUNT(*) FROM (`+budgetAlertPairsSQL+`) s JOIN categories c ON c.id=s.category_id WHERE c.kind='expense' AND s.category_id=? AND s.group_id=?`, input.CategoryID, input.GroupID) != 1 {
			return fail(409, "Budget category or spending group changed; reload before saving")
		}
		var version int64
		err := tx.QueryRow("SELECT version FROM notification_budget_preferences WHERE user_id=? AND category_id=? AND group_id=?", u.ID, input.CategoryID, input.GroupID).Scan(&version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if version != *input.Version {
			return fail(409, "Budget alert settings changed; reload before saving")
		}
		_, err = tx.Exec(`INSERT INTO notification_budget_preferences(user_id,category_id,group_id,enabled,threshold) VALUES(?,?,?,?,?) ON CONFLICT(user_id,category_id,group_id) DO UPDATE SET enabled=excluded.enabled,threshold=excluded.threshold,version=version+1`, u.ID, input.CategoryID, input.GroupID, *input.Enabled, input.Threshold)
		if err != nil {
			return err
		}
		if err := audit(tx, u, nil, "notification_budget_preferences", u.ID, "updated", map[string]any{"category_id": input.CategoryID, "group_id": input.GroupID, "enabled": *input.Enabled, "threshold": input.Threshold, "version": version + 1}); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) baselineBudgetAlertPreferencesTx(tx *sql.Tx, u User, inputs []budgetAlertPreferenceInput) error {
	if len(inputs) == 0 {
		return nil
	}
	changed := map[string]bool{}
	for _, input := range inputs {
		changed[fmt.Sprintf("%d:%d", input.CategoryID, input.GroupID)] = true
	}
	// Consume currently active crossings under the new settings in this same
	// transaction. A save never backfills historical alerts or changes other scopes.
	return a.evaluateBudgetAlertsTx(tx, u, time.Now().UTC(), changed)
}
