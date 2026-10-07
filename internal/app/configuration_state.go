package app

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// Snapshots contain only configuration. Account/grant/user changes also invalidate previews,
// but their values are hashed server-side and are never returned to the browser.
func configurationSnapshot(tx *sql.Tx) (map[string][]map[string]any, string, error) {
	result := map[string][]map[string]any{}
	for _, table := range []string{"spending_groups", "categories", "merchants", "merchant_rules", "rules", "builtin_rules"} {
		rows, err := data(tx, "SELECT * FROM "+table+" ORDER BY id")
		if err != nil {
			return nil, "", err
		}
		result[table] = rows
	}
	accounts, err := data(tx, "SELECT id,name FROM accounts ORDER BY id")
	if err != nil {
		return nil, "", err
	}
	result["accounts"] = accounts
	evidence := map[string]any{"configuration": result}
	for _, table := range []string{"accounts", "grants", "users", "configuration_sources"} {
		columns := "*"
		if table == "users" {
			columns = "id,admin,budget_member,disabled,version"
		}
		// Accounts include identity only; balance changes do not invalidate a configuration preview.
		if table == "accounts" {
			columns = "id,name,household,sync_hidden,version"
		}
		rows, err := data(tx, "SELECT "+columns+" FROM "+table+" ORDER BY rowid")
		if err != nil {
			return nil, "", err
		}
		evidence[table] = rows
	}
	raw, err := json.Marshal(evidence)
	return result, hash(string(raw)), err
}
func configurationChanges(before, after map[string][]map[string]any) []configurationChange {
	changes := []configurationChange{}
	for _, table := range []string{"spending_groups", "categories", "merchants", "merchant_rules", "rules", "builtin_rules"} {
		old := map[int64]map[string]any{}
		for _, row := range before[table] {
			old[num(row["id"])] = row
		}
		for _, row := range after[table] {
			prior := old[num(row["id"])]
			b, _ := json.Marshal(prior)
			c, _ := json.Marshal(row)
			if string(b) == string(c) {
				continue
			}
			name := exportString(row, "name")
			if name == "" {
				name = exportString(row, "pattern")
			}
			action := "create"
			if prior != nil {
				action = "update"
			}
			changes = append(changes, configurationChange{table, name, action, configurationDisplay(before, prior), configurationDisplay(after, row)})
		}
	}
	return changes
}
func configurationDisplay(snapshot map[string][]map[string]any, row map[string]any) map[string]any {
	if row == nil {
		return nil
	}
	display := map[string]any{}
	references := map[string]string{"account_id": "accounts", "category_id": "categories", "spending_group_id": "spending_groups", "merchant_id": "merchants"}
	for key, value := range row {
		if key == "id" || key == "user_id" || key == "version" {
			continue
		}
		if table, ok := references[key]; ok {
			name := ""
			for _, entity := range snapshot[table] {
				if num(entity["id"]) == num(value) {
					name = exportString(entity, "name")
					break
				}
			}
			display[strings.TrimSuffix(key, "_id")] = name
		} else if key == "enabled" || key == "archived" {
			display[key] = num(value) != 0
		} else if key == "logo_data" {
			display[key] = hash(exportString(row, key))
			if exportString(row, key) == "" {
				display[key] = ""
			}
		} else {
			display[key] = value
		}
	}
	return display
}
func (a *App) simulateConfiguration(tx *sql.Tx, u User, cfg RulesetConfig) ([]configurationChange, string, *RulesetImportSummary, error) {
	before, fingerprint, err := configurationSnapshot(tx)
	if err != nil {
		return nil, "", nil, err
	}
	if _, err = tx.Exec("SAVEPOINT configuration_validation"); err != nil {
		return nil, "", nil, err
	}
	summary := &RulesetImportSummary{}
	err = a.importRulesetTx(tx, u, cfg, summary)
	var changes []configurationChange
	if err == nil {
		var after map[string][]map[string]any
		after, _, err = configurationSnapshot(tx)
		if err == nil {
			changes = configurationChanges(before, after)
		}
	}
	if _, rollbackErr := tx.Exec("ROLLBACK TO configuration_validation"); rollbackErr != nil {
		return nil, "", nil, rollbackErr
	}
	if _, releaseErr := tx.Exec("RELEASE configuration_validation"); releaseErr != nil {
		return nil, "", nil, releaseErr
	}
	return changes, fingerprint, summary, err
}
func readConfigurationSource(q queryer, id int64) (configurationSource, error) {
	var s configurationSource
	err := q.QueryRow("SELECT id,name,kind,repository_url,ref,path,last_revision,version FROM configuration_sources WHERE id=?", id).Scan(&s.ID, &s.Name, &s.Kind, &s.RepositoryURL, &s.Ref, &s.Path, &s.Revision, &s.Version)
	if err != nil {
		return s, fail(404, "Configuration source not found")
	}
	return s, nil
}
