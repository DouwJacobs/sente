package app

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

func (a *App) accountHealth(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	page, size, e := listPage(r)
	if e != nil {
		return e
	}
	total := queryInt(tx, "SELECT COUNT(*) FROM accounts a WHERE "+accountAccessSQL(u), u.Member, u.ID)
	accounts, e := data(tx, "SELECT a.id,a.name,a.balance_date FROM accounts a WHERE "+accountAccessSQL(u)+" ORDER BY a.name,a.id LIMIT ? OFFSET ?", u.Member, u.ID, size, page*size)
	if e != nil {
		return e
	}
	for _, account := range accounts {
		id := num(account["id"])
		var last, checked sql.NullString
		tx.QueryRow("SELECT MAX(COALESCE(committed_at,created_at)) FROM imports WHERE account_id=? AND status='committed'", id).Scan(&last)
		tx.QueryRow("SELECT last_checked FROM account_import_checks WHERE account_id=?", id).Scan(&checked)
		var committedCheck sql.NullString
		tx.QueryRow("SELECT c.last_checked FROM account_import_checks c JOIN imports i ON i.id=c.import_id WHERE c.account_id=? AND i.status='committed'", id).Scan(&committedCheck)
		if committedCheck.Valid && (!last.Valid || committedCheck.String > last.String) {
			last = committedCheck
		}
		account["last_imported"], account["last_fetched"] = nil, nil
		if last.Valid {
			account["last_imported"] = last.String
		}
		if checked.Valid {
			account["last_fetched"] = checked.String
		}
		account["state"] = "never_imported"
		if last.Valid {
			account["state"] = "ready"
		}
		account["next_due"] = nil
		account["possible_gap"] = true
		latest, e := data(tx, "SELECT data FROM imports WHERE account_id=? ORDER BY id DESC LIMIT 1", id)
		if e != nil {
			return e
		}
		if len(latest) > 0 {
			var p ParsedFile
			if json.Unmarshal([]byte(latest[0]["data"].(string)), &p) != nil {
				return fail(500, "Import metadata unavailable")
			}
			account["coverage"] = p.Coverage
			account["possible_gap"] = true
		}
		connections, e := data(tx, `SELECT f.state,f.next_due,f.interval_hours FROM fnb_connections f JOIN users u ON u.id=f.user_id JOIN fnb_discoveries d ON d.user_id=f.user_id JOIN accounts a ON a.id=d.account_id AND a.bank_id=d.bank_id WHERE a.id=? AND d.hidden=0 AND u.disabled=0 AND u.admin=1 AND (a.household=1 AND u.budget_member=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=u.id AND g.role='editor'))`, id)
		if e != nil {
			return e
		}
		for _, c := range connections {
			state := fmt.Sprint(c["state"])
			if state != "ready" {
				if state == "refreshing" || state == "running" {
					account["state"] = "refreshing"
				} else {
					account["state"] = "needs_attention"
				}
			}
			if num(c["interval_hours"]) > 0 && c["next_due"] != nil {
				due := num(c["next_due"])
				if account["next_due"] == nil || due < num(account["next_due"]) {
					account["next_due"] = due
				}
				if due < time.Now().Unix() && account["state"] != "refreshing" && account["state"] != "needs_attention" {
					account["state"] = "overdue"
				}
			}
		}
		account["import_issues"] = queryInt(tx, "SELECT COUNT(*) FROM imports WHERE account_id=? AND status='staged'", id)
		if num(account["import_issues"]) > 0 && account["state"] != "refreshing" {
			account["state"] = "needs_attention"
		}
	}
	send(w, map[string]any{"items": accounts, "total": total})
	return nil
}
func (a *App) periodNavigation(w http.ResponseWriter, r *http.Request) error {
	if e := requireMember(Current(r)); e != nil {
		return e
	}
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	today := time.Now().In(loc).Format("2006-01-02")
	current, e := data(tx, "SELECT id,name,start_date,end_date FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", today, today)
	if e != nil {
		return e
	}
	out := map[string]any{"current": nil, "previous": nil, "next": nil}
	if len(current) > 0 {
		out["current"] = current[0]
	}
	id := r.URL.Query().Get("period")
	var start string
	var anchor int64
	if tx.QueryRow("SELECT id,start_date FROM periods WHERE id=?", id).Scan(&anchor, &start) != nil {
		if len(current) == 0 {
			send(w, out)
			return nil
		}
		anchor = num(current[0]["id"])
		start = current[0]["start_date"].(string)
	}
	for _, dir := range []string{"previous", "next"} {
		op, order := "<", "DESC"
		if dir == "next" {
			op, order = ">", "ASC"
		}
		v, e := data(tx, "SELECT id,name,start_date,end_date FROM periods WHERE start_date"+op+"? OR start_date=? AND id"+op+"? ORDER BY start_date "+order+",id "+order+" LIMIT 1", start, start, anchor)
		if e != nil {
			return e
		}
		if len(v) > 0 {
			out[dir] = v[0]
		}
	}
	send(w, out)
	return nil
}

type budgetReport struct {
	Periods    []map[string]any `json:"periods"`
	Entries    []map[string]any `json:"entries"`
	Budget     int64            `json:"budget_cents"`
	Spent      int64            `json:"spent_cents"`
	Income     int64            `json:"income_cents"`
	Excluded   int64            `json:"excluded_count"`
	HasTargets bool             `json:"has_targets"`
}

func (a *App) reportData(tx *sql.Tx, u User, r *http.Request) (budgetReport, error) {
	out := budgetReport{Periods: []map[string]any{}, Entries: []map[string]any{}}
	if e := requireMember(u); e != nil {
		return out, e
	}
	p := r.URL.Query()
	account := p.Get("account")
	if account != "" {
		aid, e := strconv.ParseInt(account, 10, 64)
		if e != nil || !a.can(tx, u, aid, false) || queryInt(tx, "SELECT sync_hidden FROM accounts WHERE id=?", aid) != 0 {
			return out, fail(403, "Account not accessible")
		}
	}
	out.HasTargets = account == ""
	for _, key := range []string{"category", "spending_group"} {
		if p.Get(key) != "" {
			id, e := strconv.ParseInt(p.Get(key), 10, 64)
			if e != nil || id < 0 {
				return out, fail(400, "Choose a report category or group")
			}
		}
	}
	var periods []map[string]any
	var e error
	if year := p.Get("year"); year != "" {
		n, e := strconv.Atoi(year)
		if e != nil || n < 1900 || n > 9998 {
			return out, fail(400, "Choose a budget year")
		}
		periods, e = data(tx, "SELECT id,name,start_date,end_date,version FROM periods WHERE start_date>=? AND start_date<? ORDER BY start_date,id LIMIT 1001", fmt.Sprintf("%04d-01-01", n), fmt.Sprintf("%04d-01-01", n+1))
		if e != nil {
			return out, e
		}
		if len(periods) > 1000 {
			return out, fail(400, "Choose individual periods for this report")
		}
	} else {
		limit := 6
		if v := p.Get("count"); v != "" {
			limit, e = strconv.Atoi(v)
			if e != nil || limit != 1 && limit != 2 && limit != 6 && limit != 12 {
				return out, fail(400, "Choose 1, 2, 6 or 12 periods")
			}
		}
		if id := p.Get("period"); id != "" {
			var start string
			var anchor int64
			if tx.QueryRow("SELECT id,start_date FROM periods WHERE id=?", id).Scan(&anchor, &start) != nil {
				return out, fail(404, "Period not found")
			}
			periods, e = data(tx, "SELECT id,name,start_date,end_date,version FROM periods WHERE start_date<? OR start_date=? AND id<=? ORDER BY start_date DESC,id DESC LIMIT ?", start, start, anchor, limit)
		} else {
			periods, e = data(tx, "SELECT id,name,start_date,end_date,version FROM periods ORDER BY start_date DESC,id DESC LIMIT ?", limit)
		}
		if e != nil {
			return out, e
		}
		for i, j := 0, len(periods)-1; i < j; i, j = i+1, j-1 {
			periods[i], periods[j] = periods[j], periods[i]
		}
	}
	for _, period := range periods {
		pid := num(period["id"])
		scope := "a.household=1 AND t.period_id=?"
		args := []any{u.Member, u.ID, pid}
		if account != "" {
			scope = "t.account_id=? AND (a.household=1 AND t.period_id=? OR a.household=0 AND t.date>=? AND t.date<=?)"
			args = []any{u.Member, u.ID, account, pid, period["start_date"], period["end_date"]}
		}
		extra := ""
		if g := p.Get("spending_group"); g != "" {
			extra += " AND COALESCE(t.spending_group_id,0)=?"
			args = append(args, g)
		}
		if c := p.Get("category"); c != "" {
			extra += " AND COALESCE(l.category_id,0)=?"
			args = append(args, c)
		}
		rows, e := data(tx, `SELECT COALESCE(t.spending_group_id,0) group_id,COALESCE(g.name,'No spending group') group_name,COALESCE(l.category_id,0) category_id,COALESCE(c.name,'Uncategorized') category_name,COALESCE(SUM(CASE WHEN c.kind='expense' OR c.id IS NULL AND l.amount_cents<0 THEN -l.amount_cents ELSE 0 END),0) spent_cents,COALESCE(SUM(CASE WHEN c.kind='income' OR c.id IS NULL AND l.amount_cents>0 THEN l.amount_cents ELSE 0 END),0) income_cents FROM allocations l JOIN transactions t ON t.id=l.transaction_id JOIN accounts a ON a.id=t.account_id LEFT JOIN categories c ON c.id=l.category_id LEFT JOIN spending_groups g ON g.id=t.spending_group_id WHERE `+accountAccessSQL(u)+" AND t.is_transfer=0 AND ("+scope+")"+extra+" GROUP BY t.spending_group_id,l.category_id ORDER BY group_name,category_name", args...)
		if e != nil {
			return out, e
		}
		entries := map[string]map[string]any{}
		var spent, income, budget int64
		for _, row := range rows {
			row["period_id"] = pid
			row["period_name"] = period["name"]
			row["budget_cents"] = int64(0)
			row["has_budget"] = false
			entries[fmt.Sprint(row["group_id"])+":"+fmt.Sprint(row["category_id"])] = row
			spent += num(row["spent_cents"])
			income += num(row["income_cents"])
		}
		if out.HasTargets {
			where := "gt.period_id=? AND gt.included=1"
			ba := []any{pid}
			if g := p.Get("spending_group"); g != "" {
				where += " AND COALESCE(gt.spending_group_id,0)=?"
				ba = append(ba, g)
			}
			if c := p.Get("category"); c != "" {
				where += " AND gt.category_id=?"
				ba = append(ba, c)
			}
			limits, e := data(tx, "SELECT COALESCE(gt.spending_group_id,0) group_id,COALESCE(g.name,'No spending group') group_name,gt.category_id,c.name category_name,gt.amount_cents budget_cents FROM group_targets gt JOIN categories c ON c.id=gt.category_id LEFT JOIN spending_groups g ON g.id=gt.spending_group_id WHERE "+where+" ORDER BY group_name,category_name", ba...)
			if e != nil {
				return out, e
			}
			for _, l := range limits {
				key := fmt.Sprint(l["group_id"]) + ":" + fmt.Sprint(l["category_id"])
				row := entries[key]
				if row == nil {
					row = l
					row["period_id"] = pid
					row["period_name"] = period["name"]
					row["spent_cents"] = int64(0)
					row["income_cents"] = int64(0)
					entries[key] = row
				}
				row["budget_cents"] = num(l["budget_cents"])
				row["has_budget"] = true
				budget += num(l["budget_cents"])
			}
		}
		for _, row := range entries {
			row["remaining_cents"] = num(row["budget_cents"]) - num(row["spent_cents"])
			out.Entries = append(out.Entries, row)
		}
		if len(out.Entries) > 10000 {
			return out, fail(400, "Report too large; select a category or spending group")
		}
		period["budget_cents"], period["spent_cents"], period["income_cents"], period["remaining_cents"] = budget, spent, income, budget-spent
		out.Periods = append(out.Periods, period)
		out.Budget += budget
		out.Spent += spent
		out.Income += income
	}
	if len(periods) > 0 {
		start, end := fmt.Sprint(periods[0]["start_date"]), fmt.Sprint(periods[0]["end_date"])
		for _, p := range periods {
			if fmt.Sprint(p["start_date"]) < start {
				start = fmt.Sprint(p["start_date"])
			}
			if fmt.Sprint(p["end_date"]) > end {
				end = fmt.Sprint(p["end_date"])
			}
		}
		args := []any{u.Member, u.ID, start, end}
		extra := " AND a.household=1"
		if account != "" {
			extra = " AND a.household=1 AND a.id=?"
			args = append(args, account)
		}
		out.Excluded = queryInt(tx, "SELECT COUNT(*) FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE "+accountAccessSQL(u)+" AND t.date>=? AND t.date<=?"+extra+" AND (t.period_id IS NULL OR t.assignment='outside')", args...)
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		a, b := out.Entries[i], out.Entries[j]
		if num(a["period_id"]) != num(b["period_id"]) {
			return num(a["period_id"]) < num(b["period_id"])
		}
		x, y := fmt.Sprint(a["group_name"]), fmt.Sprint(b["group_name"])
		if x != y {
			return x < y
		}
		return fmt.Sprint(a["category_name"]) < fmt.Sprint(b["category_name"])
	})
	return out, nil
}
func (a *App) budgetReports(w http.ResponseWriter, r *http.Request) error {
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := a.reportData(tx, Current(r), r)
	if e != nil {
		return e
	}
	send(w, v)
	return nil
}
func (a *App) exportBudget(w http.ResponseWriter, r *http.Request) error {
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := a.reportData(tx, Current(r), r)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="budget.csv"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"period_id", "period", "group_id", "group", "category_id", "category", "budget_cents", "spent_cents", "remaining_cents", "currency", "has_budget"})
	for _, row := range v.Entries {
		budget, left := "", ""
		if v.HasTargets && row["has_budget"] == true {
			budget = fmt.Sprint(row["budget_cents"])
			left = fmt.Sprint(row["remaining_cents"])
		}
		cw.Write([]string{fmt.Sprint(row["period_id"]), csvText(row["period_name"]), fmt.Sprint(row["group_id"]), csvText(row["group_name"]), fmt.Sprint(row["category_id"]), csvText(row["category_name"]), budget, fmt.Sprint(row["spent_cents"]), left, "ZAR", fmt.Sprint(row["has_budget"])})
	}
	cw.Flush()
	return cw.Error()
}

type rebalanceInput struct {
	Version      int64 `json:"version"`
	FromGroup    int64 `json:"from_group"`
	FromCategory int64 `json:"from_category"`
	ToGroup      int64 `json:"to_group"`
	ToCategory   int64 `json:"to_category"`
	Amount       int64 `json:"amount_cents"`
}
type rebalanceExact struct {
	Kind   string
	Period int64
	Input  rebalanceInput
	Before []map[string]any
}

func (a *App) rebalanceState(tx *sql.Tx, u User, id int64, b rebalanceInput) ([]map[string]any, error) {
	if e := requireMember(u); e != nil {
		return nil, e
	}
	if b.Amount <= 0 || b.FromGroup == b.ToGroup && b.FromCategory == b.ToCategory {
		return nil, fail(400, "Choose different entries and a positive amount")
	}
	if queryInt(tx, "SELECT version FROM periods WHERE id=?", id) != b.Version {
		return nil, fail(409, "Budget changed; preview again")
	}
	out := []map[string]any{}
	for _, key := range [][2]int64{{b.FromGroup, b.FromCategory}, {b.ToGroup, b.ToCategory}} {
		rows, e := data(tx, "SELECT gt.amount_cents,gt.carry_forward,c.name category_name,COALESCE(g.name,'No spending group') group_name FROM group_targets gt JOIN categories c ON c.id=gt.category_id LEFT JOIN spending_groups g ON g.id=gt.spending_group_id WHERE period_id=? AND COALESCE(spending_group_id,0)=? AND category_id=? AND included=1", id, key[0], key[1])
		if e != nil {
			return nil, e
		}
		if len(rows) != 1 {
			return nil, fail(400, "Choose existing budget entries")
		}
		spentRows, e := data(tx, "SELECT COALESCE(SUM(-l.amount_cents),0) spent FROM allocations l JOIN transactions t ON t.id=l.transaction_id JOIN accounts a ON a.id=t.account_id JOIN categories c ON c.id=l.category_id WHERE "+accountAccessSQL(u)+" AND a.household=1 AND t.is_transfer=0 AND c.kind='expense' AND t.period_id=? AND COALESCE(t.spending_group_id,0)=? AND l.category_id=?", u.Member, u.ID, id, key[0], key[1])
		if e != nil {
			return nil, e
		}
		rows[0]["spent_cents"] = num(spentRows[0]["spent"])
		out = append(out, rows[0])
	}
	source := num(out[0]["amount_cents"])
	available := source - num(out[0]["spent_cents"])
	if available > source {
		available = source
	}
	if b.Amount > available {
		return nil, fail(400, "Move only the source entry's unspent budget")
	}
	if num(out[1]["amount_cents"])+b.Amount > 900000000000000 {
		return nil, fail(400, "Destination limit is too large")
	}
	return out, nil
}
func (a *App) rebalancePreview(w http.ResponseWriter, r *http.Request) error {
	var b rebalanceInput
	if e := decode(r, &b); e != nil {
		return e
	}
	u := Current(r)
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	before, e := a.rebalanceState(tx, u, parseID(r), b)
	if e != nil {
		return e
	}
	send(w, map[string]any{"before": before, "from_after": num(before[0]["amount_cents"]) - b.Amount, "to_after": num(before[1]["amount_cents"]) + b.Amount, "token": sealWorkflow(u, rebalanceExact{"rebalance", parseID(r), b, before})})
	return nil
}
func (a *App) rebalanceApply(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Token string `json:"token"`
	}
	if e := decode(r, &b); e != nil {
		return e
	}
	u := Current(r)
	var exact rebalanceExact
	if e := openWorkflow(u, b.Token, &exact); e != nil {
		return e
	}
	if exact.Kind != "rebalance" || exact.Period != parseID(r) {
		return fail(400, "Wrong preview type")
	}
	e := a.write(func(tx *sql.Tx) error {
		before, e := a.rebalanceState(tx, u, exact.Period, exact.Input)
		if e != nil {
			return e
		}
		old, _ := json.Marshal(exact.Before)
		fresh, _ := json.Marshal(before)
		if string(old) != string(fresh) {
			return fail(409, "Spending changed; preview again")
		}
		for i, key := range [][2]int64{{exact.Input.FromGroup, exact.Input.FromCategory}, {exact.Input.ToGroup, exact.Input.ToCategory}} {
			delta := exact.Input.Amount
			if i == 0 {
				delta = -delta
			}
			if _, e := tx.Exec("UPDATE group_targets SET amount_cents=amount_cents+? WHERE period_id=? AND COALESCE(spending_group_id,0)=? AND category_id=? AND included=1", delta, exact.Period, key[0], key[1]); e != nil {
				return e
			}
		}
		if _, e := tx.Exec("UPDATE periods SET version=version+1 WHERE id=?", exact.Period); e != nil {
			return e
		}
		if e = syncBudgetTotalsTx(tx, exact.Period); e != nil {
			return e
		}
		return audit(tx, u, nil, "period", exact.Period, "rebalanced", exact)
	})
	if e != nil {
		return e
	}
	success(w)
	return nil
}
