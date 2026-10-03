package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type periodInput struct {
	Name         string `json:"name"`
	Start        string `json:"start_date"`
	End          string `json:"end_date"`
	Version      int64  `json:"version"`
	PreviewToken string `json:"preview_token"`
}

func validatePeriod(b periodInput) error {
	if len(b.Name) == 0 || len(b.Name) > 100 || !validDate(b.Start) || !validDate(b.End) || b.Start > b.End {
		return fail(400, "Provide a period name and valid start/end dates")
	}
	return nil
}
func (a *App) periods(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	sizeForTargets := 20
	if !r.URL.Query().Has("page") {
		size = 100
	}
	filterSQL := " FROM periods WHERE 1=1"
	args := []any{}
	if q := r.URL.Query().Get("q"); q != "" {
		filterSQL += " AND name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	if id := r.URL.Query().Get("id"); id != "" {
		filterSQL += " AND id=?"
		args = append(args, id)
	}
	states, err := data(a.DB, "SELECT group_concat(id||':'||version) state,COUNT(*) total FROM (SELECT id,version"+filterSQL+" ORDER BY id)", args...)
	if err != nil {
		return err
	}
	version := hash(fmt.Sprint(states[0]["state"]))
	if expected := r.URL.Query().Get("list_version"); expected != "" && expected != version {
		return fail(409, "This list changed. Start from the first page.")
	}
	query := "SELECT *" + filterSQL + " ORDER BY start_date DESC,id DESC LIMIT ? OFFSET ?"
	args = append(args, size, page*size)
	items, err := data(a.DB, query, args...)
	if err != nil {
		return err
	}
	for _, p := range items {
		query := "SELECT t.category_id,t.amount_cents,c.name FROM targets t JOIN categories c ON c.id=t.category_id WHERE period_id=? ORDER BY c.name,c.id"
		args := []any{p["id"]}
		query += " LIMIT ?"
		args = append(args, sizeForTargets)
		v, err := data(a.DB, query, args...)
		if err != nil {
			return err
		}
		p["targets"] = v
		p["target_total"] = queryInt(a.DB, "SELECT COALESCE(SUM(amount_cents),0) FROM targets WHERE period_id=?", p["id"])
	}

	day := queryInt(a.DB, "SELECT start_day FROM settings WHERE id=1")
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	base := time.Now().In(loc)
	var latest string
	if a.DB.QueryRow("SELECT start_date FROM periods ORDER BY start_date DESC,id DESC LIMIT 1").Scan(&latest) == nil {
		base, _ = time.Parse("2006-01-02", latest)
		base = base.AddDate(0, 1, -base.Day()+1)
	}
	start := clamped(base.Year(), base.Month(), int(day))
	endBase := start.AddDate(0, 1, -start.Day()+1)
	end := clamped(endBase.Year(), endBase.Month(), int(day)).AddDate(0, 0, -1)
	send(w, map[string]any{"items": items, "total": states[0]["total"], "list_version": version, "next": map[string]string{"name": start.Format("January 2006"), "start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}})
	return nil
}
func clamped(year int, month time.Month, day int) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
func (a *App) createPeriod(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b periodInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := validatePeriod(b); err != nil {
		return err
	}
	var id int64
	err := a.write(func(tx *sql.Tx) error {
		preview, err := periodChanges(tx, 0, b)
		if err != nil {
			return err
		}
		if b.PreviewToken == "" || b.PreviewToken != preview["preview_token"] {
			return fail(409, "Preview the new period before saving")
		}
		previous := queryInt(tx, "SELECT id FROM periods ORDER BY start_date DESC,id DESC LIMIT 1")
		res, err := tx.Exec("INSERT INTO periods(name,start_date,end_date) VALUES(?,?,?)", b.Name, b.Start, b.End)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if _, err := tx.Exec("INSERT INTO targets SELECT ?,category_id,amount_cents FROM targets WHERE period_id=?", id, previous); err != nil {
			return err
		}
		if err := reassign(tx); err != nil {
			return err
		}
		return audit(tx, u, nil, "period", id, "created", b)
	})
	if err != nil {
		return err
	}
	send(w, map[string]int64{"id": id})
	return nil
}
func periodChanges(q queryer, id int64, b periodInput) (map[string]any, error) {
	periods, err := data(q, "SELECT * FROM periods ORDER BY start_date DESC,id DESC")
	if err != nil {
		return nil, err
	}
	found := id == 0
	before, _ := json.Marshal(periods)
	if id == 0 {
		var next int64
		for _, p := range periods {
			if num(p["id"]) > next {
				next = num(p["id"])
			}
		}
		periods = append(periods, map[string]any{"id": next + 1, "start_date": b.Start, "end_date": b.End, "version": int64(0)})
	}
	for _, p := range periods {
		if num(p["id"]) == id {
			found = true
			if num(p["version"]) != b.Version {
				return nil, fail(409, "Period changed; reload before previewing")
			}
			p["start_date"] = b.Start
			p["end_date"] = b.End
		}
	}
	if !found {
		return nil, fail(404, "Period not found")
	}
	rows, err := data(q, "SELECT t.id,t.date,t.period_id,t.assignment,t.version FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.household=1 ORDER BY t.id")
	if err != nil {
		return nil, err
	}
	fingerprint, _ := json.Marshal(rows)
	affectedRows := []map[string]any{}
	manualOutside := 0
	for _, t := range rows {
		date := t["date"].(string)
		if t["assignment"] == "manual" {
			for _, p := range periods {
				if num(p["id"]) == num(t["period_id"]) && (date < p["start_date"].(string) || date > p["end_date"].(string)) {
					manualOutside++
				}
			}
			continue
		}
		if t["assignment"] != "auto" {
			continue
		}
		var next any
		latest := ""
		for _, p := range periods {
			start := p["start_date"].(string)
			if start <= date && p["end_date"].(string) >= date && (next == nil || start > latest || start == latest && num(p["id"]) > num(next)) {
				latest = start
				next = p["id"]
			}
		}
		if (next == nil) != (t["period_id"] == nil) || num(next) != num(t["period_id"]) {
			affectedRows = append(affectedRows, map[string]any{"id": t["id"], "date": date, "from": t["period_id"], "to": next})
		}
	}
	token := hash(string(before) + string(fingerprint) + b.Name + b.Start + b.End)
	return map[string]any{"affected": affectedRows, "manual_outside": manualOutside, "preview_token": token}, nil
}
func (a *App) previewPeriod(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	var b periodInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := validatePeriod(b); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	result, err := periodChanges(a.DB, parseID(r), b)
	if err != nil {
		return err
	}
	{
		page, size, err := listPage(r)
		if !r.URL.Query().Has("page") {
			size = 100
		}
		if err != nil {
			return err
		}
		rows := result["affected"].([]map[string]any)
		result["total"] = len(rows)
		start := page * size
		if start > len(rows) {
			start = len(rows)
		}
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}
		result["affected"] = rows[start:end]
	}
	send(w, result)
	return nil
}
func (a *App) updatePeriod(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b periodInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := validatePeriod(b); err != nil {
		return err
	}
	id := parseID(r)
	err := a.write(func(tx *sql.Tx) error {
		preview, err := periodChanges(tx, id, b)
		if err != nil {
			return err
		}
		if b.PreviewToken == "" || b.PreviewToken != preview["preview_token"] {
			return fail(409, "Preview these dates again before saving; affected transactions may have changed")
		}
		res, err := tx.Exec("UPDATE periods SET name=?,start_date=?,end_date=?,version=version+1 WHERE id=? AND version=?", b.Name, b.Start, b.End, id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		if err := reassign(tx); err != nil {
			return err
		}
		return audit(tx, u, nil, "period", id, "updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) updateTargets(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	id := parseID(r)
	var b struct {
		Version int64 `json:"version"`
		Merge   bool  `json:"merge"`
		Targets []struct {
			CategoryID int64 `json:"category_id"`
			Amount     int64 `json:"amount_cents"`
		} `json:"targets"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE periods SET version=version+1 WHERE id=? AND version=?", id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		if !b.Merge {
			if _, err := tx.Exec("DELETE FROM targets WHERE period_id=?", id); err != nil {
				return err
			}
		}
		seen := map[int64]bool{}
		for _, t := range b.Targets {
			if t.Amount < 0 || t.Amount > 900000000000000 || seen[t.CategoryID] || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND kind='expense'", t.CategoryID) != 1 {
				return fail(400, "Choose unique expense categories and nonnegative limits")
			}
			seen[t.CategoryID] = true
			if _, err := tx.Exec("INSERT INTO targets VALUES(?,?,?) ON CONFLICT(period_id,category_id) DO UPDATE SET amount_cents=excluded.amount_cents", id, t.CategoryID, t.Amount); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "period", id, "limits_updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	send(w, map[string]any{"start_day": queryInt(a.DB, "SELECT start_day FROM settings WHERE id=1"), "currency": "ZAR", "timezone": "Africa/Johannesburg"})
	return nil
}
func (a *App) updateSettings(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b struct {
		Day int `json:"start_day"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Day < 1 || b.Day > 31 {
		return fail(400, "Start day must be 1–31")
	}
	err := a.write(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE settings SET start_day=? WHERE id=1", b.Day); err != nil {
			return err
		}
		return audit(tx, u, nil, "settings", 1, "updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	pid, _ := strconv.ParseInt(r.URL.Query().Get("period"), 10, 64)
	if pid == 0 {
		loc, _ := time.LoadLocation("Africa/Johannesburg")
		today := time.Now().In(loc).Format("2006-01-02")
		pid = queryInt(a.DB, "SELECT id FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", today, today)
		if pid == 0 {
			pid = queryInt(a.DB, "SELECT id FROM periods ORDER BY start_date DESC,id DESC LIMIT 1")
		}
	}
	periods, err := data(a.DB, "SELECT * FROM periods WHERE id=?", pid)
	if err != nil {
		return err
	}
	if len(periods) == 0 {
		return fail(404, "Period not found")
	}
	p := periods[0]
	account, _ := strconv.ParseInt(r.URL.Query().Get("account"), 10, 64)
	condition := "a.household=1 AND t.period_id=?"
	args := []any{pid}
	showTargets := true
	if account != 0 {
		if !a.can(a.DB, u, account, false) {
			return fail(403, "Account access required")
		}
		household := queryInt(a.DB, "SELECT household FROM accounts WHERE id=?", account) == 1
		condition = "t.account_id=? AND t.date>=? AND t.date<=?"
		args = []any{account, p["start_date"], p["end_date"]}
		if household {
			condition = "t.account_id=? AND t.period_id=?"
			args = []any{account, pid}
		}
		showTargets = false
	}
	rows, err := data(a.DB, "SELECT t.id,t.review_state,t.is_transfer,l.amount_cents,c.id category_id,c.name,c.group_name,c.kind FROM transactions t JOIN accounts a ON a.id=t.account_id JOIN allocations l ON l.transaction_id=t.id LEFT JOIN categories c ON c.id=l.category_id WHERE a.sync_hidden=0 AND "+condition, args...)
	if err != nil {
		return err
	}
	income, spent, pendingSpend := int64(0), int64(0), int64(0)
	pendingCount := map[int64]bool{}
	uncategorized := map[int64]bool{}
	categories := map[int64]map[string]any{}
	catRows, err := data(a.DB, "SELECT c.*,COALESCE(t.amount_cents,0) target_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? ORDER BY c.name,c.id", pid)
	if err != nil {
		return err
	}
	for _, c := range catRows {
		if !showTargets {
			c["target_cents"] = int64(0)
		}
		c["spent_cents"] = int64(0)
		c["pending_cents"] = int64(0)
		categories[num(c["id"])] = c
	}
	for _, row := range rows {
		if row["review_state"] == "pending_review" {
			pendingCount[num(row["id"])] = true
		}
		if num(row["is_transfer"]) == 1 {
			continue
		}
		amount := num(row["amount_cents"])
		expense := row["kind"] == "expense" || (row["category_id"] == nil && amount < 0)
		if expense {
			spent -= amount
			if row["review_state"] == "pending_review" {
				pendingSpend -= amount
			}
		} else {
			income += amount
		}
		if row["category_id"] == nil {
			uncategorized[num(row["id"])] = true
		} else if expense {
			c := categories[num(row["category_id"])]
			c["spent_cents"] = num(c["spent_cents"]) - amount
			if row["review_state"] == "pending_review" {
				c["pending_cents"] = num(c["pending_cents"]) - amount
			}
		}
	}
	var limits int64
	for _, c := range catRows {
		limits += num(c["target_cents"])
	}
	unassigned := queryInt(a.DB, "SELECT COUNT(*) FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.sync_hidden=0 AND a.household=1 AND t.period_id IS NULL AND t.assignment!='outside'")
	balanceScope := " AND a.household=1"
	balanceArgs := []any{u.Member, u.ID}
	if account != 0 {
		balanceScope = " AND a.id=?"
		balanceArgs = append(balanceArgs, account)
	}
	balances, err := data(a.DB, "SELECT a.id,a.name,a.household,a.balance_cents,a.balance_date FROM accounts a WHERE "+accessSQL+balanceScope+" ORDER BY a.name", balanceArgs...)
	if err != nil {
		return err
	}
	categoryTotal, balanceTotal := len(catRows), len(balances)
	if !r.URL.Query().Has("category_page") && len(catRows) > 100 {
		catRows = catRows[:100]
	}
	if !r.URL.Query().Has("balance_page") && len(balances) > 100 {
		balances = balances[:100]
	}
	if r.URL.Query().Has("category_page") {
		page, err := strconv.Atoi(r.URL.Query().Get("category_page"))
		if err != nil || page < 0 || page > 1000000 {
			return fail(400, "Choose a valid category page")
		}
		expense := []map[string]any{}
		for _, c := range catRows {
			if c["kind"] == "expense" && (num(c["target_cents"]) != 0 || num(c["spent_cents"]) != 0) {
				expense = append(expense, c)
			}
		}
		categoryTotal = len(expense)
		start := page * 20
		if start > len(expense) {
			start = len(expense)
		}
		end := start + 20
		if end > len(expense) {
			end = len(expense)
		}
		catRows = expense[start:end]
	}
	if r.URL.Query().Has("balance_page") {
		page, err := strconv.Atoi(r.URL.Query().Get("balance_page"))
		if err != nil || page < 0 || page > 1000000 {
			return fail(400, "Choose a valid balance page")
		}
		start := page * 20
		if start > len(balances) {
			start = len(balances)
		}
		end := start + 20
		if end > len(balances) {
			end = len(balances)
		}
		balances = balances[start:end]
	}
	send(w, map[string]any{"period": p, "income_cents": income, "spent_cents": spent, "pending_spend_cents": pendingSpend, "pending_count": len(pendingCount), "uncategorized_count": len(uncategorized), "budget_cents": limits, "remaining_cents": limits - spent, "has_targets": showTargets, "categories": catRows, "category_total": categoryTotal, "balances": balances, "balance_total": balanceTotal, "unassigned_count": unassigned})
	return nil
}

func (a *App) targetPages(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	id := parseID(r)
	if queryInt(a.DB, "SELECT COUNT(*) FROM periods WHERE id=?", id) == 0 {
		return fail(404, "Period not found")
	}
	return a.metadataPage(w, r, "SELECT c.id,c.name,c.kind,COALESCE(t.amount_cents,0) amount_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? WHERE c.kind='expense'", []any{id}, "name", "name,id")
}
