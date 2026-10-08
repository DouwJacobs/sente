package app

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var workflowKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}()

func sealWorkflow(u User, value any) string {
	b, _ := json.Marshal(map[string]any{"user": u.ID, "expires": time.Now().Add(15 * time.Minute).Unix(), "value": value})
	mac := hmac.New(sha256.New, workflowKey)
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func openWorkflow(u User, token string, value any) error {
	p := strings.Split(token, ".")
	if len(p) != 2 {
		return fail(400, "Preview the changes first")
	}
	b, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return fail(400, "Invalid preview")
	}
	sig, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil {
		return fail(400, "Invalid preview")
	}
	mac := hmac.New(sha256.New, workflowKey)
	mac.Write(b)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return fail(400, "Invalid preview")
	}
	var envelope struct {
		User    int64
		Expires int64
		Value   json.RawMessage
	}
	if json.Unmarshal(b, &envelope) != nil || envelope.User != u.ID || envelope.Expires < time.Now().Unix() {
		return fail(409, "Preview expired; preview again")
	}
	if json.Unmarshal(envelope.Value, value) != nil {
		return fail(400, "Invalid preview")
	}
	return nil
}
func (a *App) workflowRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/transactions/navigation", wrap(a.transactionNavigation))
	m.HandleFunc("POST /api/transactions/bulk/preview", wrap(a.bulkPreview))
	m.HandleFunc("POST /api/transactions/bulk/apply", wrap(a.bulkApply))
	m.HandleFunc("GET /api/transactions/export", wrap(a.exportTransactions))
	m.HandleFunc("GET /api/accounts/health", wrap(a.accountHealth))
	m.HandleFunc("GET /api/periods/navigation", wrap(a.periodNavigation))
	m.HandleFunc("GET /api/budget/reports", wrap(a.budgetReports))
	m.HandleFunc("GET /api/budget/export", wrap(a.exportBudget))
	m.HandleFunc("POST /api/periods/{id}/rebalance/preview", wrap(a.rebalancePreview))
	m.HandleFunc("POST /api/periods/{id}/rebalance/apply", wrap(a.rebalanceApply))
	m.HandleFunc("GET /api/categories/{id}/dependencies", wrap(a.categoryDependencies))
	m.HandleFunc("PUT /api/categories/{id}", wrap(a.updateCategory))
	m.HandleFunc("GET /api/labels", wrap(a.labels))
	m.HandleFunc("POST /api/labels", wrap(a.createLabel))
	m.HandleFunc("PUT /api/labels/{id}", wrap(a.updateLabel))
	m.HandleFunc("GET /api/merchant-rules", wrap(a.merchantRules))
	m.HandleFunc("POST /api/merchant-rules", wrap(a.saveMerchantRule))
	m.HandleFunc("PUT /api/merchant-rules/{id}", wrap(a.saveMerchantRule))
	m.HandleFunc("POST /api/merchant-rules/{id}/preview", wrap(a.merchantRulePreview))
}
func (a *App) transactionNavigation(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	p := r.URL.Query()
	id, e := strconv.ParseInt(p.Get("anchor_id"), 10, 64)
	if e != nil || id <= 0 || !validDate(p.Get("anchor_date")) {
		return fail(400, "Choose a transaction position")
	}
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var account int64
	if tx.QueryRow("SELECT account_id FROM transactions WHERE id=?", id).Scan(&account) != nil || !a.can(tx, u, account, false) {
		return fail(404, "Transaction not accessible")
	}
	text, args, e := a.transactionQuery(tx, r, u)
	if e != nil {
		return e
	}
	where := text[strings.Index(text, " FROM transactions"):]
	exclude := " AND t.id!=?"
	extra := []any{id}
	if v := p.Get("processed"); v != "" {
		ids := strings.Split(v, ",")
		if len(ids) > 1000 {
			return fail(400, "Start a new review after 1000 transactions")
		}
		for _, v := range ids {
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n <= 0 {
				return fail(400, "Invalid processed transactions")
			}
			exclude += " AND t.id!=?"
			extra = append(extra, n)
		}
	}
	result := map[string]any{"previous": nil, "next": nil}
	for _, direction := range []string{"previous", "next"} {
		op, order := ">", "ASC"
		if direction == "next" {
			op, order = "<", "DESC"
		}
		v := append(append([]any{}, args...), extra...)
		v = append(v, p.Get("anchor_date"), p.Get("anchor_date"), id)
		rows, e := data(tx, "SELECT t.id,t.date,t.description"+where+exclude+" AND (t.date"+op+"? OR t.date=? AND t.id"+op+"?) ORDER BY t.date "+order+",t.id "+order+" LIMIT 1", v...)
		if e != nil {
			return e
		}
		if len(rows) > 0 {
			result[direction] = rows[0]
		}
	}
	send(w, result)
	return nil
}

type bulkRequest struct {
	Items      []mcpSeenItem `json:"items"`
	Operation  string        `json:"operation"`
	CategoryID *int64        `json:"category_id,omitempty"`
	GroupID    *int64        `json:"group_id,omitempty"`
	Mode       string        `json:"mode,omitempty"`
	Name       string        `json:"name,omitempty"`
	RuleID     int64         `json:"rule_id,omitempty"`
}
type bulkExact struct {
	Kind  string
	Edits []mcpExactEdit
}

func (a *App) prepareBulk(tx *sql.Tx, u User, b bulkRequest) (bulkExact, []map[string]any, error) {
	exact := bulkExact{Kind: "bulk", Edits: []mcpExactEdit{}}
	rows := []map[string]any{}
	if len(b.Items) < 1 || len(b.Items) > 100 {
		return exact, rows, fail(400, "Select 1–100 transactions")
	}
	seen := map[int64]bool{}
	for _, item := range b.Items {
		if seen[item.ID] {
			return exact, rows, fail(400, "Select distinct transactions")
		}
		seen[item.ID] = true
		before, e := a.mcpTransaction(tx, u, item.ID)
		if e != nil {
			return exact, rows, e
		}
		if before.Version != item.Version {
			return exact, rows, fail(409, "Transaction changed; reload selection")
		}
		account := queryInt(tx, "SELECT account_id FROM transactions WHERE id=?", item.ID)
		if !a.can(tx, u, account, true) || queryInt(tx, "SELECT sync_hidden FROM accounts WHERE id=?", account) != 0 {
			return exact, rows, fail(403, "Enabled account editor access required")
		}
		var note string
		var mid sql.NullInt64
		if e := tx.QueryRow("SELECT note,merchant_id FROM transactions WHERE id=?", item.ID).Scan(&note, &mid); e != nil {
			return exact, rows, e
		}
		before.Note = &note
		if mid.Valid {
			v := mid.Int64
			before.MerchantID = &v
		}
		tagRows, e := data(tx, "SELECT tag_id FROM transaction_tags WHERE transaction_id=? ORDER BY tag_id", item.ID)
		if e != nil {
			return exact, rows, e
		}
		tagIDs := []int64{}
		for _, t := range tagRows {
			tagIDs = append(tagIDs, num(t["tag_id"]))
		}
		before.Tags = &tagIDs
		after := before
		after.Allocations = append([]Allocation{}, before.Allocations...)
		changed := false
		reason := "Unchanged"
		switch b.Operation {
		case "category":
			if b.CategoryID == nil || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND archived=0", *b.CategoryID) != 1 {
				return exact, rows, fail(400, "Choose an active category")
			}
			if b.Mode != "missing" && b.Mode != "unsplit" {
				return exact, rows, fail(400, "Choose fill missing or recategorise unsplit")
			}
			if before.Transfer {
				reason = "Transfer excluded"
			} else if b.Mode == "unsplit" && len(after.Allocations) != 1 {
				reason = "Split preserved"
			} else {
				for i, l := range after.Allocations {
					if l.CategoryID == nil || b.Mode == "unsplit" {
						if l.CategoryID == nil || *l.CategoryID != *b.CategoryID {
							after.Allocations[i].CategoryID = b.CategoryID
							changed = true
						}
					}
				}
			}
		case "group":
			if b.GroupID != nil && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.GroupID) != 1 {
				return exact, rows, fail(400, "Choose an existing group")
			}
			var name string
			if b.GroupID != nil {
				tx.QueryRow("SELECT name FROM spending_groups WHERE id=?", *b.GroupID).Scan(&name)
			}
			after.SpendingGroupID = b.GroupID
			after.Transfer = strings.EqualFold(strings.TrimSpace(name), "Transfer")
			if queryInt(tx, "SELECT COUNT(*) FROM transfer_links WHERE left_id=? OR right_id=?", item.ID, item.ID) > 0 && !after.Transfer {
				reason = "Linked transfer excluded"
				after = before
			} else {
				changed = !sameID(before.SpendingGroupID, after.SpendingGroupID) || before.Transfer != after.Transfer
			}
		case "merchant", "clear_merchant", "add_tag", "remove_tag":
			var note string
			var merchant sql.NullInt64
			tx.QueryRow("SELECT note,merchant_id FROM transactions WHERE id=?", item.ID).Scan(&note, &merchant)
			if b.Operation == "clear_merchant" {
				after.ClearMerchant = true
				after.MerchantID = nil
				changed = merchant.Valid
			} else if b.Operation == "merchant" {
				mid, e := labelID(tx, u, a, account, "merchant", b.Name, true)
				if e != nil {
					return exact, rows, e
				}
				after.MerchantID = &mid
				changed = !merchant.Valid || merchant.Int64 != mid
			} else {
				tid, e := labelID(tx, u, a, account, "tag", b.Name, b.Operation == "add_tag")
				if e != nil {
					return exact, rows, e
				}
				tags, e := data(tx, "SELECT tag_id FROM transaction_tags WHERE transaction_id=? ORDER BY tag_id", item.ID)
				if e != nil {
					return exact, rows, e
				}
				ids := []int64{}
				found := false
				for _, tag := range tags {
					v := num(tag["tag_id"])
					if v == tid {
						found = true
						if b.Operation == "remove_tag" {
							continue
						}
					}
					ids = append(ids, v)
				}
				if b.Operation == "add_tag" && !found {
					ids = append(ids, tid)
				}
				after.Tags = &ids
				changed = (b.Operation == "add_tag" && !found) || (b.Operation == "remove_tag" && found)
			}
		case "merchant_rule":
			var ruleAccount sql.NullInt64
			var mid int64
			if tx.QueryRow("SELECT account_id,merchant_id FROM merchant_rules WHERE id=? AND enabled=1", b.RuleID).Scan(&ruleAccount, &mid) != nil || ruleAccount.Valid && ruleAccount.Int64 != account {
				return exact, rows, fail(400, "Choose this account's active merchant rule")
			}
			existing := queryInt(tx, "SELECT COALESCE(merchant_id,0) FROM transactions WHERE id=?", item.ID)
			matched, e := matchMerchant(tx, account, before.Description, before.Amount)
			if e != nil {
				return exact, rows, e
			}
			if existing == 0 && matched == mid {
				after.MerchantID = &mid
				changed = true
			} else {
				reason = "Named, conflicting or nonmatching transaction excluded"
			}
		default:
			return exact, rows, fail(400, "Choose a bulk operation")
		}
		if changed {
			if e := a.validateMetadata(tx, u, account, after); e != nil {
				return exact, rows, e
			}
			exact.Edits = append(exact.Edits, mcpExactEdit{ID: item.ID, Before: before, After: after})
			reason = "Will change"
		}
		rows = append(rows, map[string]any{"id": item.ID, "description": before.Description, "changed": changed, "reason": reason, "before": bulkDisplay(tx, before), "after": bulkDisplay(tx, after)})
	}
	return exact, rows, nil
}

// Compare semantic edits; newly created label IDs are provisional until apply.
func bulkDigest(exact bulkExact, operation string) string {
	edits := append([]mcpExactEdit{}, exact.Edits...)
	for i := range edits {
		if operation != "merchant_rule" {
			edits[i].After.MerchantID = nil
		}
		edits[i].After.Tags = nil
	}
	b, _ := json.Marshal(edits)
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func bulkDisplay(q queryer, b transactionInput) map[string]any {
	encoded, _ := json.Marshal(b)
	var row map[string]any
	json.Unmarshal(encoded, &row)
	if b.SpendingGroupID != nil {
		var name string
		q.QueryRow("SELECT name FROM spending_groups WHERE id=?", *b.SpendingGroupID).Scan(&name)
		row["spending_group_name"] = name
	}
	if b.MerchantID != nil {
		var name string
		q.QueryRow("SELECT name FROM merchants WHERE id=?", *b.MerchantID).Scan(&name)
		row["merchant_name"] = name
	}
	names := []string{}
	if b.Tags != nil {
		for _, id := range *b.Tags {
			var name string
			q.QueryRow("SELECT name FROM tags WHERE id=?", id).Scan(&name)
			names = append(names, name)
		}
	}
	row["tag_names"] = names
	for _, v := range row["allocations"].([]any) {
		l := v.(map[string]any)
		if l["category_id"] != nil {
			var name string
			q.QueryRow("SELECT name FROM categories WHERE id=?", l["category_id"]).Scan(&name)
			l["category_name"] = name
		}
	}
	return row
}
func sameID(a, b *int64) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func (a *App) bulkPreview(w http.ResponseWriter, r *http.Request) error {
	var b bulkRequest
	if e := decode(r, &b); e != nil {
		return e
	}
	u := Current(r)
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	exact, rows, e := a.prepareBulk(tx, u, b)
	if e != nil {
		return e
	}
	// Labels are resolved again by name at apply: rolled-back preview IDs cannot escape.
	send(w, map[string]any{"items": rows, "changed": len(exact.Edits), "token": sealWorkflow(u, struct {
		Kind    string
		Request bulkRequest
		Digest  string
	}{"bulk", b, bulkDigest(exact, b.Operation)})})
	return nil
}
func (a *App) bulkApply(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Token string `json:"token"`
	}
	if e := decode(r, &b); e != nil {
		return e
	}
	u := Current(r)
	var v struct {
		Kind    string
		Request bulkRequest
		Digest  string
	}
	if e := openWorkflow(u, b.Token, &v); e != nil {
		return e
	}
	if v.Kind != "bulk" {
		return fail(400, "Wrong preview type")
	}
	count := 0
	e := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		exact, _, e := a.prepareBulk(tx, u, v.Request)
		if e != nil {
			return e
		}
		if bulkDigest(exact, v.Request.Operation) != v.Digest {
			return fail(409, "Preview changed; preview again")
		}
		for _, edit := range exact.Edits {
			if _, e := a.editTransactionTx(tx, u, edit.ID, edit.After); e != nil {
				return e
			}
			if e := audit(tx, u, queryInt(tx, "SELECT account_id FROM transactions WHERE id=?", edit.ID), "transaction", edit.ID, "bulk_edited", map[string]any{"before": edit.Before, "after": edit.After}); e != nil {
				return e
			}
		}
		count = len(exact.Edits)
		return nil
	})
	if e != nil {
		return e
	}
	send(w, map[string]any{"changed": count})
	return nil
}
func csvText(v any) string {
	s := fmt.Sprint(v)
	if v == nil {
		return ""
	}
	trim := strings.TrimLeft(s, " \t\r\n")
	if trim != "" && strings.ContainsAny(trim[:1], "=+-@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") || strings.HasPrefix(s, "\n") {
		return "'" + s
	}
	return s
}
func (a *App) exportTransactions(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	text, args, e := a.transactionQuery(tx, r, u)
	if e != nil {
		return e
	}
	rows, e := data(tx, text+" ORDER BY t.date DESC,t.id DESC", args...)
	if e != nil {
		return e
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	parent, e := z.Create("transactions.csv")
	if e != nil {
		return e
	}
	cw := csv.NewWriter(parent)
	cw.Write([]string{"transaction_id", "date", "account", "description", "amount_cents", "currency", "spending_group", "period", "acceptance", "seen", "note", "merchant", "tags"})
	for _, row := range rows {
		if !u.Member {
			row["period_name"] = nil
		}
		tags, e := data(tx, "SELECT g.name FROM transaction_tags tt JOIN tags g ON g.id=tt.tag_id WHERE transaction_id=? ORDER BY g.name", row["id"])
		if e != nil {
			return e
		}
		names := []string{}
		for _, t := range tags {
			names = append(names, t["name"].(string))
		}
		cw.Write([]string{fmt.Sprint(row["id"]), fmt.Sprint(row["date"]), csvText(row["account_name"]), csvText(row["description"]), fmt.Sprint(row["amount_cents"]), "ZAR", csvText(row["spending_group_name"]), csvText(row["period_name"]), csvText(row["review_state"]), fmt.Sprint(row["seen"]), csvText(row["note"]), csvText(row["merchant_name"]), csvText(strings.Join(names, "; "))})
	}
	cw.Flush()
	if e = cw.Error(); e != nil {
		return e
	}
	alloc, e := z.Create("allocations.csv")
	if e != nil {
		return e
	}
	cw = csv.NewWriter(alloc)
	cw.Write([]string{"transaction_id", "allocation_id", "category_id", "category", "amount_cents", "currency", "note"})
	for _, row := range rows {
		entries, e := data(tx, "SELECT l.id,l.category_id,c.name,l.amount_cents,l.note FROM allocations l LEFT JOIN categories c ON c.id=l.category_id WHERE transaction_id=? ORDER BY l.id", row["id"])
		if e != nil {
			return e
		}
		for _, l := range entries {
			cw.Write([]string{fmt.Sprint(row["id"]), fmt.Sprint(l["id"]), csvText(l["category_id"]), csvText(l["name"]), fmt.Sprint(l["amount_cents"]), "ZAR", csvText(l["note"])})
		}
	}
	cw.Flush()
	if e = cw.Error(); e != nil {
		return e
	}
	if e = z.Close(); e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.zip"`)
	w.Write(buf.Bytes())
	return nil
}
