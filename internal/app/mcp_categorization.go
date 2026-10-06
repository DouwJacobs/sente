package app

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpContextRuleLimit = 2000
const mcpRulePreviewRowLimit = 10000

func distinctPositiveIDs(ids []int64, label string) error {
	if len(ids) < 1 || len(ids) > 100 {
		return fail(400, "Choose 1–100 "+label)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return fail(400, "Choose distinct positive "+label)
		}
		seen[id] = true
	}
	return nil
}
func ruleSummaries(rules []classificationRule) []map[string]any {
	out := []map[string]any{}
	for _, r := range rules {
		if len(out) >= 10 {
			break
		}
		out = append(out, map[string]any{"id": r.ID, "builtin": r.Builtin, "pattern": mcpRedact(r.Pattern), "category_id": r.CategoryID, "category_name": mcpRedact(r.CategoryName), "spending_group_id": r.SpendingGroupID, "spending_group_name": mcpRedact(r.SpendingGroupName), "direction": r.Direction, "priority": r.Priority})
	}
	return out
}

type mcpRuleImpactInput struct {
	Rule        ruleInput `json:"rule"`
	AccountIDs  []int64   `json:"account_ids,omitempty" jsonschema:"Explicit authorized accounts; defaults to rule.account_id; 1-100 distinct IDs"`
	ReplaceID   int64     `json:"replace_id,omitempty" jsonschema:"Existing rule ID to replace in this preview; zero is a new rule"`
	SampleLimit *int      `json:"sample_limit,omitempty" jsonschema:"1-50; default 10"`
}

func (a *App) mcpRuleImpactTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpRuleImpactInput) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	accounts := in.AccountIDs
	if len(accounts) == 0 {
		accounts = []int64{in.Rule.AccountID}
	}
	if err := distinctPositiveIDs(accounts, "account IDs"); err != nil {
		return mcpFailure(err)
	}
	if in.ReplaceID > 0 && (len(accounts) != 1 || accounts[0] != in.Rule.AccountID) {
		return mcpFailure(fail(400, "Preview a saved account rule in its own account"))
	}
	limit := 10
	if in.SampleLimit != nil {
		limit = *in.SampleLimit
	}
	if limit < 1 || limit > 50 {
		return mcpFailure(fail(400, "Choose a sample limit from 1 to 50"))
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	if err := in.Rule.validate(tx); err != nil {
		return mcpFailure(err)
	}
	result := map[string]any{"description_match_count": int64(0), "future_rule_match_count": int64(0), "future_classified_count": int64(0), "uncategorized_count": int64(0), "existing_eligible_count": int64(0), "conflict_count": int64(0), "samples": []map[string]any{}, "standalone_rule_sweeps_existing": false, "evidence": "Current authorized ledger descriptions and actual rule precedence; not a forecast of future imports"}
	total := int64(0)
	for _, account := range accounts {
		if !a.can(tx, identity.User, account, false) || queryInt(tx, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", account) != 1 {
			return mcpFailure(fail(403, "Enabled account access required"))
		}
		// Deterministic contains/direction filtering runs in SQL before the bounded matcher.
		where := " FROM transactions t WHERE t.account_id=? AND instr(finance_normalize(t.description),finance_normalize(?))>0"
		args := []any{account, in.Rule.Pattern}
		if in.Rule.Direction == "debit" {
			where += " AND t.amount_cents<0"
		}
		if in.Rule.Direction == "credit" {
			where += " AND t.amount_cents>0"
		}
		countRows, err := data(tx, "SELECT COUNT(*) n"+where, args...)
		if err != nil {
			return mcpFailure(err)
		}
		n := num(countRows[0]["n"])
		total += n
		if total > mcpRulePreviewRowLimit {
			return mcpFailure(fail(400, "Preview matches more than 10000 transactions; narrow the pattern or account scope"))
		}
		draft := in.Rule
		draft.AccountID = account
		rules, err := a.previewRuleSet(tx, identity.User, rulePreviewInput{AccountID: account, Draft: &draft, ReplaceID: in.ReplaceID}, false)
		if err != nil {
			return mcpFailure(err)
		}
		if len(rules) > mcpContextRuleLimit {
			return mcpFailure(fail(400, "Too many active rules for this account to preview safely"))
		}
		rows, err := data(tx, `SELECT t.id,t.version,t.date,t.description,t.amount_cents,t.account_id,t.is_transfer,t.review_state,
 (t.is_transfer=0 AND (NOT EXISTS(SELECT 1 FROM allocations l WHERE l.transaction_id=t.id) OR EXISTS(SELECT 1 FROM allocations l WHERE l.transaction_id=t.id AND l.category_id IS NULL))) uncategorized,
 (t.review_state='pending_review' AND t.is_transfer=0 AND (SELECT COUNT(*) FROM allocations l WHERE l.transaction_id=t.id)=1 AND EXISTS(SELECT 1 FROM allocations l WHERE l.transaction_id=t.id AND l.category_id IS NULL AND l.amount_cents=t.amount_cents)) eligible`+where+" ORDER BY t.date DESC,t.id DESC", args...)
		if err != nil {
			return mcpFailure(err)
		}
		for _, entry := range rows {
			row := SourceRow{Description: entry["description"].(string), Amount: num(entry["amount_cents"])}
			classify(&row, rules)
			participates := false
			for _, r := range row.RuleMatches {
				participates = participates || r.ID == in.ReplaceID
			}
			result["description_match_count"] = result["description_match_count"].(int64) + 1
			if num(entry["uncategorized"]) == 1 {
				result["uncategorized_count"] = result["uncategorized_count"].(int64) + 1
			}
			if participates {
				result["future_rule_match_count"] = result["future_rule_match_count"].(int64) + 1
			}
			if row.RuleConflict {
				result["conflict_count"] = result["conflict_count"].(int64) + 1
			}
			if participates && !row.RuleConflict && row.CategoryID != nil {
				result["future_classified_count"] = result["future_classified_count"].(int64) + 1
				if num(entry["eligible"]) == 1 && ruleAccess(tx, a, identity.User, account) {
					result["existing_eligible_count"] = result["existing_eligible_count"].(int64) + 1
				}
			}
			samples := result["samples"].([]map[string]any)
			if len(samples) < limit {
				sample := mcpSafe(entry).(map[string]any)
				sample["currency"] = "ZAR"
				sample["conflict"] = row.RuleConflict
				sample["draft_participates"] = participates
				sample["matching_rule_count"] = len(row.RuleMatches)
				sample["matching_rules"] = ruleSummaries(row.RuleMatches)
				result["samples"] = append(samples, sample)
			}
		}
	}
	return mcpResult(result), nil, nil
}

// Referenced by context history queries: authorized accounts must always be joined.
func contextIDScope(ids []int64) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

type mcpCategorizationContextInput struct {
	TransactionIDs []int64 `json:"transaction_ids" jsonschema:"1-100 distinct authorized transaction IDs"`
}

func (a *App) mcpCategorizationContextTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpCategorizationContextInput) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	if err := distinctPositiveIDs(in.TransactionIDs, "transaction IDs"); err != nil {
		return mcpFailure(err)
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	ids, idArgs := contextIDScope(in.TransactionIDs)
	targets, err := data(tx, "SELECT t.id,t.version,t.account_id,t.description,t.amount_cents FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE "+accountAccessSQL(identity.User)+" AND t.id IN ("+ids+") ORDER BY t.id", append([]any{identity.User.Member, identity.User.ID}, idArgs...)...)
	if err != nil {
		return mcpFailure(err)
	}
	if len(targets) != len(in.TransactionIDs) {
		return mcpFailure(fail(404, "One or more transactions are unavailable"))
	}
	descriptions := []string{}
	seen := map[string]bool{}
	for _, target := range targets {
		description := normalize(target["description"].(string))
		if !seen[description] {
			seen[description] = true
			descriptions = append(descriptions, description)
		}
	}
	places := make([]string, len(descriptions))
	descriptionArgs := make([]any, len(descriptions))
	for i, description := range descriptions {
		places[i] = "?"
		descriptionArgs[i] = description
	}
	// History excludes this requested batch, and applies present access before grouping.
	historyScope := " FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE " + accountAccessSQL(identity.User) + " AND t.id NOT IN (" + ids + ") AND finance_normalize(t.description) IN (" + strings.Join(places, ",") + ")"
	args := append(append([]any{identity.User.Member, identity.User.ID}, idArgs...), descriptionArgs...)
	counts, err := data(tx, "SELECT finance_normalize(t.description) normalized_description,COUNT(*) transaction_count"+historyScope+" GROUP BY finance_normalize(t.description)", args...)
	if err != nil {
		return mcpFailure(err)
	}
	historicalCounts := map[string]int64{}
	for _, row := range counts {
		historicalCounts[row["normalized_description"].(string)] = num(row["transaction_count"])
	}
	categoryScope := strings.Replace(historyScope, " WHERE ", " JOIN allocations l ON l.transaction_id=t.id JOIN categories c ON c.id=l.category_id WHERE ", 1)
	categoryRows, err := data(tx, `WITH counts AS (SELECT finance_normalize(t.description) normalized_description,c.id category_id,c.name category_name,c.kind,COUNT(DISTINCT t.id) transaction_count`+categoryScope+` GROUP BY finance_normalize(t.description),c.id), ranked AS (SELECT *,COUNT(*) OVER(PARTITION BY normalized_description) category_total,ROW_NUMBER() OVER(PARTITION BY normalized_description ORDER BY transaction_count DESC,category_id) position FROM counts) SELECT * FROM ranked WHERE position<=20 ORDER BY normalized_description,position`, args...)
	if err != nil {
		return mcpFailure(err)
	}
	history := map[string][]map[string]any{}
	totals := map[string]int64{}
	for _, row := range categoryRows {
		description := row["normalized_description"].(string)
		totals[description] = num(row["category_total"])
		history[description] = append(history[description], map[string]any{"category_id": row["category_id"], "category_name": mcpRedact(row["category_name"].(string)), "kind": row["kind"], "transaction_count": row["transaction_count"]})
	}
	rulesByAccount := map[int64][]classificationRule{}
	items := []map[string]any{}
	for _, target := range targets {
		account := num(target["account_id"])
		rules, loaded := rulesByAccount[account]
		if !loaded {
			rules, err = loadRules(tx, account)
			if err != nil {
				return mcpFailure(err)
			}
			if len(rules) > mcpContextRuleLimit {
				return mcpFailure(fail(400, "Too many active rules for this account to evaluate safely"))
			}
			rulesByAccount[account] = rules
		}
		row := SourceRow{Description: target["description"].(string), Amount: num(target["amount_cents"])}
		classify(&row, rules)
		description := normalize(row.Description)
		categories := history[description]
		if categories == nil {
			categories = []map[string]any{}
		}
		item := mcpSafe(target).(map[string]any)
		item["matching_rules"] = ruleSummaries(row.RuleMatches)
		item["matching_rule_count"] = len(row.RuleMatches)
		item["rules_truncated"] = len(row.RuleMatches) > 10
		item["conflict"] = row.RuleConflict
		item["suggested_category_id"] = row.CategoryID
		item["suggested_spending_group_id"] = row.SpendingGroupID
		item["historical_categories"] = categories
		item["historical_category_total"] = totals[description]
		item["history_truncated"] = totals[description] > 20
		item["historical_transaction_count"] = historicalCounts[description]
		items = append(items, item)
	}
	return mcpResult(map[string]any{"items": items, "similarity": "same_normalized_description", "evidence": "Description-based evidence using the existing Unicode normalization; no merchant identity or confidence score; history excludes this requested batch"}), nil, nil
}
