package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpPatch struct {
	ID                 int64           `json:"id"`
	Version            int64           `json:"version"`
	Date               *string         `json:"date,omitempty"`
	Amount             *int64          `json:"amount_cents,omitempty"`
	Description        *string         `json:"description,omitempty"`
	CategoryID         *int64          `json:"category_id,omitempty"`
	Allocations        []mcpAllocation `json:"allocations,omitempty"`
	SpendingGroupID    *int64          `json:"spending_group_id,omitempty"`
	ClearSpendingGroup bool            `json:"clear_spending_group,omitempty"`
	Transfer           *bool           `json:"is_transfer,omitempty"`
	Assignment         *string         `json:"assignment,omitempty"`
	PeriodID           *int64          `json:"period_id,omitempty"`
}
type mcpAllocation struct {
	CategoryID *int64  `json:"category_id"`
	Amount     int64   `json:"amount_cents"`
	Note       *string `json:"note,omitempty"`
}
type mcpChange struct {
	BudgetAlerts    []mcpBudgetAlertInput   `json:"budget_alerts,omitempty"`
	Merchant        *mcpMerchantInput       `json:"merchant,omitempty"`
	MerchantRule    *merchantRuleInput      `json:"merchant_rule,omitempty"`
	MerchantItems   []mcpMerchantAssignment `json:"merchant_items,omitempty"`
	Seen            *bool                   `json:"seen,omitempty"`
	SeenItems       []mcpSeenItem           `json:"seen_items,omitempty"`
	Operation       string                  `json:"operation"`
	Categorizations []mcpCategoryAssignment `json:"categorizations,omitempty"`
	Edits           []mcpPatch              `json:"edits,omitempty"`
	Category        *categoryInput          `json:"category,omitempty"`
	Rule            *ruleInput              `json:"rule,omitempty"`
	Budget          *targetsInput           `json:"budget,omitempty"`
	ID              int64                   `json:"id,omitempty"`
	Version         int64                   `json:"version,omitempty"`
}
type mcpExactEdit struct {
	ID     int64            `json:"id"`
	Before transactionInput `json:"before"`
	After  transactionInput `json:"after"`
}
type mcpExactChange struct {
	AutomaticApproval bool                    `json:"automatic_approval,omitempty"`
	Change            mcpChange               `json:"change"`
	Transactions      []mcpExactEdit          `json:"transactions,omitempty"`
	SeenStates        []mcpSeenState          `json:"seen_states,omitempty"`
	Before            any                     `json:"before,omitempty"`
	BudgetAlertBefore []budgetAlertPreference `json:"budget_alert_before,omitempty"`
	BudgetAlertAfter  []budgetAlertPreference `json:"budget_alert_after,omitempty"`
	BudgetAfter       any                     `json:"budget_after,omitempty"`
}
type mcpProposalID struct {
	ProposalID string `json:"proposal_id"`
}

func (a *App) mcpPrepareTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpChange) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	if !identity.Write {
		return mcpFailure(fail(403, "This token is read-only"))
	}
	exact := mcpExactChange{Change: in}
	// Validate using the same write services, then roll back the preview in full.
	a.mu.Lock()
	tx, err := a.DB.Begin()
	if err == nil {
		if in.Operation == "update_budget_alerts" {
			exact.BudgetAlertBefore, err = prepareMCPBudgetAlerts(tx, identity, in)
		}
		if in.Operation == "save_merchant" {
			exact.Before, err = data(tx, "SELECT id,account_id,name,version,logo_data FROM merchants WHERE id=?", in.ID)
		}
		if in.Operation == "save_merchant_rule" || in.Operation == "delete_merchant_rule" {
			exact.Before, err = data(tx, "SELECT * FROM merchant_rules WHERE id=?", in.ID)
		}
		if in.Operation == "assign_merchants" {
			exact.Transactions, err = a.prepareMCPMerchantAssignments(tx, identity, in)
		}
		if in.Operation == "set_seen" {
			exact.SeenStates, err = a.prepareMCPSeen(tx, identity, in)
		}
		if in.Operation == "assign_categories" {
			exact.Transactions, err = a.prepareCategoryAssignments(tx, identity, in)
		}
		if in.Operation == "edit_transactions" {
			if len(in.Edits) < 1 || len(in.Edits) > 100 {
				err = fail(400, "Select 1–100 transaction edits")
			} else {
				seen := map[int64]bool{}
				for _, patch := range in.Edits {
					if seen[patch.ID] {
						err = fail(400, "Each transaction can appear only once")
						break
					}
					seen[patch.ID] = true
					var before, after transactionInput
					before, err = a.mcpTransaction(tx, identity.User, patch.ID)
					if err != nil {
						break
					}
					if before.Version != patch.Version {
						err = fail(409, "Transaction changed; reload before preparing changes")
						break
					}
					after = before
					if patch.Date != nil {
						after.Date = *patch.Date
					}
					if patch.Amount != nil {
						after.Amount = *patch.Amount
					}
					if patch.Description != nil {
						after.Description = *patch.Description
					}
					if patch.Transfer != nil {
						after.Transfer = *patch.Transfer
					}
					if patch.Assignment != nil {
						after.Assignment = *patch.Assignment
					}
					if patch.PeriodID != nil {
						after.PeriodID = patch.PeriodID
					}
					if patch.SpendingGroupID != nil {
						after.SpendingGroupID = patch.SpendingGroupID
					}
					if patch.ClearSpendingGroup {
						after.SpendingGroupID = nil
					}
					if patch.Allocations != nil {
						after.Allocations = make([]Allocation, len(patch.Allocations))
						for index, allocation := range patch.Allocations {
							note := ""
							if index < len(before.Allocations) {
								note = before.Allocations[index].Note
							}
							if allocation.Note != nil {
								note = *allocation.Note
							}
							after.Allocations[index] = Allocation{CategoryID: allocation.CategoryID, Amount: allocation.Amount, Note: note}
						}
					}
					if patch.CategoryID != nil {
						if len(after.Allocations) != 1 || patch.Allocations != nil {
							err = fail(400, "Use explicit allocations for split transactions")
							break
						}
						after.Allocations = append([]Allocation(nil), after.Allocations...)
						after.Allocations[0].CategoryID = patch.CategoryID
					}
					// An amount-only edit keeps a single allocation in balance; splits must be explicit.
					if patch.Amount != nil && patch.Allocations == nil && len(after.Allocations) == 1 {
						after.Allocations = append([]Allocation(nil), after.Allocations...)
						after.Allocations[0].Amount = after.Amount
					}
					exact.Transactions = append(exact.Transactions, mcpExactEdit{ID: patch.ID, Before: before, After: after})
				}
			}
		}
		if err == nil && in.Operation == "update_budget" {
			if in.Budget == nil {
				err = fail(400, "Provide budget")
			} else {
				var canonical targetsInput
				canonical, err = canonicalTargetsTx(tx, in.ID, *in.Budget)
				if err == nil {
					exact.Change.Budget = &canonical
					exact.Before, err = budgetEvidenceTx(tx, in.ID)
				}
			}
		}
		if err == nil && (in.Operation == "save_rule" || in.Operation == "delete_rule") && in.ID != 0 {
			table, id := "rules", in.ID
			if id < 0 {
				table, id = "builtin_rules", -id
			}
			accountField := "account_id"
			if table == "builtin_rules" {
				accountField = "0 account_id"
			}
			exact.Before, err = data(tx, "SELECT id,"+accountField+",pattern,category_id,direction,priority,enabled,spending_group_id,version FROM "+table+" WHERE id=?", id)
		}
		if err == nil {
			_, err = a.executeMCPChange(tx, identity, exact, "")
			if err == nil && in.Operation == "update_budget_alerts" {
				exact.BudgetAlertAfter, err = readMCPBudgetAlertSelection(tx, identity.User, in.BudgetAlerts)
			}
			if err == nil && in.Operation == "update_budget" {
				exact.BudgetAfter, err = budgetEvidenceTx(tx, in.ID)
			}
		}
		tx.Rollback()
	}
	a.mu.Unlock()
	if err != nil {
		return mcpFailure(err)
	}
	exact.Change.Edits = nil // Store canonical full edits; original hidden description/notes are preserved.
	payload, _ := json.Marshal(exact)
	id := randomToken()
	expires := time.Now().Add(time.Hour).Unix()
	status := "pending"
	err = a.write(func(tx *sql.Tx) error {
		if !a.mcpTokenActive(tx, identity) {
			return fail(401, "MCP access revoked")
		}
		if err := a.authorizeMCPExact(tx, identity, exact); err != nil {
			return err
		}
		if queryInt(tx, "SELECT COUNT(*) FROM mcp_proposals WHERE token_id=? AND status IN ('pending','approved') AND expires_at>?", identity.TokenID, time.Now().Unix()) >= 20 {
			return fail(400, "Resolve existing proposals before creating more")
		}
		automatic, err := a.mcpAutomaticApproval(tx, identity, exact)
		if err != nil {
			return err
		}
		if automatic {
			status = "approved"
		}
		exact.AutomaticApproval = automatic
		payload, _ = json.Marshal(exact)
		_, err = tx.Exec("INSERT INTO mcp_proposals(id,token_id,user_id,operation,payload,expires_at,status) VALUES(?,?,?,?,?,?,?)", id, identity.TokenID, identity.User.ID, in.Operation, string(payload), expires, status)
		if err != nil {
			return err
		}
		if automatic {
			return audit(tx, identity.User, nil, "mcp", identity.TokenID, "automatically_approved", map[string]any{"proposal_id": id, "connection_id": identity.TokenID, "required_capabilities": mcpRequiredCapabilities(exact)})
		}
		return nil
	})
	if err != nil {
		return mcpFailure(err)
	}
	review := "Open Settings → MCP to inspect and approve the exact changes. Then call apply_change."
	if status == "approved" {
		review = "Automatically approved by this connection’s configured permissions. Call apply_change; current access and versions will be rechecked."
	}
	return mcpResult(map[string]any{"proposal_id": id, "status": status, "expires_at": expires, "review": review}), nil, nil
}

func (a *App) mcpTransaction(tx *sql.Tx, u User, id int64) (transactionInput, error) {
	var b transactionInput
	var account int64
	err := tx.QueryRow("SELECT account_id,version,date,amount_cents,description,is_transfer,assignment,period_id,spending_group_id FROM transactions WHERE id=?", id).Scan(&account, &b.Version, &b.Date, &b.Amount, &b.Description, &b.Transfer, &b.Assignment, &b.PeriodID, &b.SpendingGroupID)
	if err != nil {
		return b, fail(404, "Transaction not found")
	}
	if !ruleAccess(tx, a, u, account) {
		return b, fail(403, "Enabled account editor access required")
	}
	rows, err := data(tx, "SELECT category_id,amount_cents,note FROM allocations WHERE transaction_id=? ORDER BY id", id)
	if err != nil {
		return b, err
	}
	for _, row := range rows {
		v := Allocation{Amount: num(row["amount_cents"]), Note: row["note"].(string)}
		if row["category_id"] != nil {
			id := num(row["category_id"])
			v.CategoryID = &id
		}
		b.Allocations = append(b.Allocations, v)
	}
	return b, nil
}

func (a *App) executeMCPChange(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange, proposal string) (any, error) {
	if err := a.authorizeMCPExact(tx, identity, exact); err != nil {
		return nil, err
	}
	u := identity.User
	b := exact.Change
	result := map[string]any{"ok": true}
	switch b.Operation {
	case "update_budget_alerts":
		if err := a.applyMCPBudgetAlerts(tx, identity, exact); err != nil {
			return nil, err
		}
		result["updated"] = len(b.BudgetAlerts)
	case "save_merchant":
		id, err := a.saveMCPMerchant(tx, u, b.ID, b.Merchant)
		if err != nil {
			return nil, err
		}
		result["id"] = id
	case "save_merchant_rule":
		id := b.ID
		if err := a.writeMerchantRule(tx, u, &id, *b.MerchantRule); err != nil {
			return nil, err
		}
		result["id"] = id
	case "delete_merchant_rule":
		if b.ID <= 0 {
			return nil, fail(400, "Provide a merchant rule id")
		}
		var accountID sql.NullInt64
		if err := tx.QueryRow("SELECT account_id FROM merchant_rules WHERE id=?", b.ID).Scan(&accountID); err != nil {
			return nil, fail(404, "Merchant rule not found")
		}
		if !merchantScopeAccess(tx, a, u, accountID.Int64) {
			return nil, fail(403, "Merchant rule access required")
		}
		res, err := tx.Exec("DELETE FROM merchant_rules WHERE id=? AND version=?", b.ID, b.Version)
		if err != nil {
			return nil, err
		}
		if err := affected(res); err != nil {
			return nil, err
		}
		if err := audit(tx, u, accountID, "merchant_rule", b.ID, "deleted", nil); err != nil {
			return nil, err
		}
	case "assign_merchants":
		for _, edit := range exact.Transactions {
			if _, err := a.editTransactionTx(tx, u, edit.ID, edit.After); err != nil {
				return nil, err
			}
		}
		result["updated"] = len(exact.Transactions)
	case "set_seen":
		if err := a.applyMCPSeen(tx, identity, exact); err != nil {
			return nil, err
		}
		result["updated"] = len(exact.SeenStates)
	case "assign_categories":
		var err error
		resultValue, err := a.applyCategoryAssignments(tx, identity, exact)
		if err != nil {
			return nil, err
		}
		result = resultValue.(map[string]any)
	case "edit_transactions":
		if len(exact.Transactions) < 1 || len(exact.Transactions) > 100 {
			return nil, fail(400, "Select 1–100 transaction edits")
		}
		for _, edit := range exact.Transactions {
			if _, err := a.mcpTransaction(tx, u, edit.ID); err != nil {
				return nil, err
			}
			if _, err := a.editTransactionTx(tx, u, edit.ID, edit.After); err != nil {
				return nil, err
			}
		}
		result["updated"] = len(exact.Transactions)
	case "create_category":
		if b.Category == nil {
			return nil, fail(400, "Provide category")
		}
		id, err := createCategoryTx(tx, u, *b.Category)
		if err != nil {
			return nil, err
		}
		result["id"] = id
	case "save_rule":
		if b.Rule == nil {
			return nil, fail(400, "Provide rule")
		}
		id := b.ID
		if err := a.writeRule(tx, u, &id, b.Rule); err != nil {
			return nil, err
		}
		result["id"] = id
	case "delete_rule":
		if b.ID < 0 {
			if err := a.deleteBuiltinRule(tx, u, b.ID, b.Version); err != nil {
				return nil, err
			}
		} else {
			var account int64
			if err := tx.QueryRow("SELECT account_id FROM rules WHERE id=?", b.ID).Scan(&account); err != nil {
				return nil, fail(404, "Rule not found")
			}
			if !ruleAccess(tx, a, u, account) {
				return nil, fail(403, "Account editor access required")
			}
			res, err := tx.Exec("DELETE FROM rules WHERE id=? AND version=?", b.ID, b.Version)
			if err != nil {
				return nil, err
			}
			if err := affected(res); err != nil {
				return nil, err
			}
			if err := audit(tx, u, account, "rule", b.ID, "deleted", nil); err != nil {
				return nil, err
			}
		}
	case "update_budget":
		if b.Budget == nil {
			return nil, fail(400, "Provide budget")
		}
		if proposal != "" {
			if exact.BudgetAfter == nil {
				return nil, fail(409, "Prepare this budget proposal again with explicit groups")
			}
			before, err := budgetEvidenceTx(tx, b.ID)
			if err != nil {
				return nil, err
			}
			expected, _ := json.Marshal(exact.Before)
			current, _ := json.Marshal(before)
			if string(expected) != string(current) {
				return nil, fail(409, "Budget changed; prepare the proposal again")
			}
		}
		if err := updateTargetsTx(tx, u, b.ID, *b.Budget); err != nil {
			return nil, err
		}
	default:
		return nil, fail(400, "Unknown finance operation")
	}
	if proposal != "" {
		evidence, err := a.mcpAuditEvidence(tx, identity, exact, result)
		if err != nil {
			return nil, err
		}
		if err := audit(tx, u, nil, "mcp", identity.TokenID, "applied", map[string]any{"proposal_id": proposal, "connection_id": identity.TokenID, "operation": b.Operation, "entities": evidence}); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (a *App) mcpTokenActive(q queryer, i mcpIdentity) bool {
	return queryInt(q, "SELECT COUNT(*) FROM mcp_tokens t JOIN users u ON u.id=t.user_id WHERE t.id=? AND t.user_id=? AND t.can_write=1 AND t.expires_at>? AND u.disabled=0 AND u.deleted_at IS NULL AND u.admin=? AND u.budget_member=?", i.TokenID, i.User.ID, time.Now().Unix(), i.User.Admin, i.User.Member) == 1
}
func (a *App) mcpApplyTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpProposalID) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	var result any
	err := a.write(func(tx *sql.Tx) error {
		if !identity.Write || !a.mcpTokenActive(tx, identity) {
			return fail(403, "MCP write access revoked")
		}
		var payload, status, saved string
		var expires int64
		if err := tx.QueryRow("SELECT payload,status,expires_at,result FROM mcp_proposals WHERE id=? AND token_id=? AND user_id=?", in.ProposalID, identity.TokenID, identity.User.ID).Scan(&payload, &status, &expires, &saved); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fail(404, "Proposal not found")
			}
			return fail(500, "Proposal could not be loaded; tracker database maintenance may be required")
		}
		if status == "applied" {
			var exact mcpExactChange
			if err := json.Unmarshal([]byte(payload), &exact); err != nil {
				return err
			}
			if err := a.authorizeMCPExact(tx, identity, exact); err != nil {
				return err
			}
			return json.Unmarshal([]byte(saved), &result)
		}
		if expires <= time.Now().Unix() {
			return fail(409, "Proposal expired; prepare a new change")
		}
		if status != "approved" {
			return fail(403, "Approve the exact proposal in Settings → MCP first")
		}
		var exact mcpExactChange
		if err := json.Unmarshal([]byte(payload), &exact); err != nil {
			return err
		}
		var err error
		if exact.AutomaticApproval {
			automatic, checkErr := a.mcpAutomaticApproval(tx, identity, exact)
			if checkErr != nil {
				return checkErr
			}
			if !automatic {
				return fail(403, "Automatic approval was removed; prepare a new proposal for browser approval")
			}
		}
		result, err = a.executeMCPChange(tx, identity, exact, in.ProposalID)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(result)
		_, err = tx.Exec("UPDATE mcp_proposals SET status='applied',result=? WHERE id=?", string(encoded), in.ProposalID)
		return err
	})
	if err != nil {
		return mcpFailure(err)
	}
	return mcpResult(result), nil, nil
}
func (a *App) mcpStatusTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpProposalID) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	var status string
	var expires int64
	if err := a.DB.QueryRow("SELECT status,expires_at FROM mcp_proposals WHERE id=? AND token_id=? AND user_id=?", in.ProposalID, identity.TokenID, identity.User.ID).Scan(&status, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mcpFailure(fail(404, "Proposal not found"))
		}
		return mcpFailure(fail(500, "Proposal could not be loaded; tracker database maintenance may be required"))
	}
	if status != "applied" && expires <= time.Now().Unix() {
		status = "expired"
	}
	return mcpResult(map[string]any{"proposal_id": in.ProposalID, "status": status}), nil, nil
}

func (a *App) mcpSettings(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	tokens, err := data(a.DB, "SELECT id,name,client_id,last_used_at,can_write,expires_at,created_at,permissions,permission_version FROM mcp_tokens WHERE user_id=? ORDER BY id DESC", u.ID)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		var p mcpPermissions
		raw := token["permissions"].(string)
		if raw == "{}" {
			p = legacyMCPPermissions(num(token["can_write"]) == 1)
		} else {
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				return err
			}
		}
		token["permissions"] = p
	}
	accounts, err := a.mcpPermissionAccounts(a.DB, u)
	if err != nil {
		return err
	}
	proposals, err := data(a.DB, `SELECT p.id,p.operation,p.payload,p.status,p.expires_at,t.name token_name FROM mcp_proposals p JOIN mcp_tokens t ON t.id=p.token_id WHERE p.user_id=? AND p.status IN ('pending','approved') AND p.expires_at>? AND t.expires_at>? ORDER BY p.created_at DESC,p.id DESC LIMIT 500`, u.ID, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return err
	}
	for _, p := range proposals {
		var payload any
		json.Unmarshal([]byte(p["payload"].(string)), &payload)
		p["payload"] = payload
	}
	send(w, map[string]any{"endpoint": a.PublicURL + "/api/mcp", "connections": tokens, "proposals": proposals, "accounts": accounts})
	return nil
}
func (a *App) revokeMCPToken(w http.ResponseWriter, r *http.Request) error {
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		res, err := tx.Exec("DELETE FROM mcp_tokens WHERE id=? AND user_id=?", parseID(r), u.ID)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "mcp", parseID(r), "token_revoked", nil)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) decideMCPProposalTx(tx *sql.Tx, u User, id string, approve bool) error {
	status := "rejected"
	if approve {
		var token int64
		var payload string
		if err := tx.QueryRow("SELECT token_id,payload FROM mcp_proposals WHERE id=? AND user_id=?", id, u.ID).Scan(&token, &payload); err != nil {
			return fail(409, "Proposal changed or is unavailable")
		}
		permissions, err := readMCPPermissions(tx, mcpIdentity{User: u, TokenID: token})
		if err != nil {
			return err
		}
		scoped := u
		scoped.MCPAccounts = permissions.Constraints.AccountIDs
		var exact mcpExactChange
		if err = json.Unmarshal([]byte(payload), &exact); err != nil {
			return err
		}
		if err = a.authorizeMCPExact(tx, mcpIdentity{User: scoped, TokenID: token}, exact); err != nil {
			return err
		}
		status = "approved"
	}
	res, err := tx.Exec(`UPDATE mcp_proposals SET status=? WHERE id=? AND user_id=? AND status='pending' AND expires_at>? AND EXISTS(SELECT 1 FROM mcp_tokens t WHERE t.id=mcp_proposals.token_id AND t.can_write=1 AND t.expires_at>?)`, status, id, u.ID, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return err
	}
	if err = affected(res); err != nil {
		return err
	}
	return audit(tx, u, nil, "mcp", 0, status, map[string]any{"proposal_id": id, "connection_id": queryInt(tx, "SELECT token_id FROM mcp_proposals WHERE id=? AND user_id=?", id, u.ID)})
}
func (a *App) decideMCPProposal(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Approve bool `json:"approve"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := a.browserWrite(r, func(tx *sql.Tx, u User) error { return a.decideMCPProposalTx(tx, u, r.PathValue("id"), b.Approve) }); err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) decideMCPProposalBatch(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Approve bool     `json:"approve"`
		IDs     []string `json:"proposal_ids"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.IDs) < 1 || len(b.IDs) > 500 {
		return fail(400, "Select 1–500 proposals")
	}
	seen := map[string]bool{}
	for _, id := range b.IDs {
		if id == "" || len(id) > 128 || seen[id] {
			return fail(400, "Select distinct proposal IDs")
		}
		seen[id] = true
	}
	if err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		for _, id := range b.IDs {
			if err := a.decideMCPProposalTx(tx, u, id, b.Approve); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	send(w, map[string]any{"ok": true, "updated": len(b.IDs)})
	return nil
}

// Financial evidence stays inside the browser/audit boundary, never in MCP reads.
func (a *App) mcpAuditEvidence(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange, result map[string]any) ([]map[string]any, error) {
	evidence := []map[string]any{}
	b := exact.Change
	switch b.Operation {
	case "update_budget_alerts":
		after, err := readMCPBudgetAlertSelection(tx, identity.User, b.BudgetAlerts)
		if err != nil {
			return nil, err
		}
		if !sameMCPBudgetAlerts(after, exact.BudgetAlertAfter) {
			return nil, fail(409, "Budget alert effects changed; prepare the proposal again")
		}
		evidence = append(evidence, map[string]any{"entity": "personal_budget_alerts", "user_id": identity.User.ID, "before": exact.BudgetAlertBefore, "after": after})
	case "save_merchant", "save_merchant_rule", "delete_merchant_rule":
		table, entity := "merchants", "merchant"
		if b.Operation == "save_merchant_rule" {
			table, entity = "merchant_rules", "merchant_rule"
		}
		if b.Operation == "delete_merchant_rule" {
			// Deleted; no after state to fetch.
			evidence = append(evidence, map[string]any{"entity": "merchant_rule", "id": b.ID, "before": exact.Before, "after": nil})
			break
		}
		id := num(result["id"])
		after, err := data(tx, "SELECT * FROM "+table+" WHERE id=?", id)
		if err != nil {
			return nil, err
		}
		evidence = append(evidence, map[string]any{"entity": entity, "id": id, "before": exact.Before, "after": after})
	case "assign_merchants":
		for _, edit := range exact.Transactions {
			evidence = append(evidence, map[string]any{"entity": "transaction_merchant", "id": edit.ID, "before": edit.Before.MerchantID, "after": edit.After.MerchantID})
		}
	case "set_seen":
		for _, item := range exact.SeenStates {
			evidence = append(evidence, map[string]any{"entity": "personal_seen", "id": item.ID, "account_id": item.AccountID, "user_id": identity.User.ID, "version": item.Version, "before": item.Seen, "after": *b.Seen})
		}
	case "edit_transactions", "assign_categories":
		for _, edit := range exact.Transactions {
			after, err := a.mcpTransaction(tx, identity.User, edit.ID)
			if err != nil {
				return nil, err
			}
			evidence = append(evidence, map[string]any{"entity": "transaction", "id": edit.ID, "account_id": queryInt(tx, "SELECT account_id FROM transactions WHERE id=?", edit.ID), "before": edit.Before, "after": after})
		}
	case "create_category":
		id := result["id"].(int64)
		after, err := data(tx, "SELECT id,name,kind FROM categories WHERE id=?", id)
		if err != nil {
			return nil, err
		}
		evidence = append(evidence, map[string]any{"entity": "category", "id": id, "before": nil, "after": after})
	case "save_rule", "delete_rule":
		id := b.ID
		if b.Operation == "save_rule" {
			id = result["id"].(int64)
		}
		var after any
		if b.Operation == "save_rule" {
			table, lookup, accountField := "rules", id, "account_id"
			if id < 0 {
				table, lookup, accountField = "builtin_rules", -id, "0 account_id"
			}
			rows, err := data(tx, "SELECT id,"+accountField+",pattern,category_id,direction,priority,enabled,spending_group_id,version FROM "+table+" WHERE id=?", lookup)
			if err != nil {
				return nil, err
			}
			after = rows
		}
		evidence = append(evidence, map[string]any{"entity": "rule", "id": id, "before": exact.Before, "after": after})
	case "update_budget":
		after, err := budgetEvidenceTx(tx, b.ID)
		if err != nil {
			return nil, err
		}
		approved, _ := json.Marshal(exact.BudgetAfter)
		actual, _ := json.Marshal(after)
		if string(approved) != string(actual) {
			return nil, fail(409, "Budget effects changed; prepare the proposal again")
		}
		evidence = append(evidence, map[string]any{"entity": "budget_limits", "id": b.ID, "before": map[string]any{"version": b.Budget.Version, "targets": exact.Before}, "after": map[string]any{"version": queryInt(tx, "SELECT version FROM periods WHERE id=?", b.ID), "targets": after}})
	}
	return evidence, nil
}

// budgetEvidenceTx records exact independent scopes, including membership and copy behaviour.
func budgetEvidenceTx(tx *sql.Tx, id int64) (map[string]any, error) {
	groups, err := data(tx, "SELECT COALESCE(b.spending_group_id,0) group_id,COALESCE(g.name,'No spending group') group_name,COALESCE(g.version,0) group_version FROM budget_groups b LEFT JOIN spending_groups g ON g.id=b.spending_group_id WHERE b.period_id=? ORDER BY COALESCE(b.spending_group_id,0)", id)
	if err != nil {
		return nil, err
	}
	targets, err := data(tx, "SELECT COALESCE(t.spending_group_id,0) group_id,t.category_id,c.name category_name,t.amount_cents,t.carry_forward,t.included FROM group_targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? ORDER BY COALESCE(t.spending_group_id,0),t.category_id", id)
	if err != nil {
		return nil, err
	}
	legacy, err := data(tx, "SELECT t.category_id,c.name category_name,t.amount_cents FROM targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id) ORDER BY t.category_id", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"groups": groups, "targets": targets, "legacy_targets": legacy}, nil
}
