package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
		p["group_budgets"], err = data(a.DB, "SELECT COALESCE(gt.spending_group_id,0) id,COALESCE(g.name,'No spending group') name,SUM(gt.amount_cents) target_cents FROM group_targets gt LEFT JOIN spending_groups g ON g.id=gt.spending_group_id WHERE gt.period_id=? AND gt.included=1 GROUP BY gt.spending_group_id ORDER BY name,id LIMIT 20", p["id"])
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
		if _, err := tx.Exec("INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward,included) SELECT ?,category_id,spending_group_id,amount_cents,carry_forward,included FROM group_targets WHERE period_id=? AND carry_forward=1 AND included=1", id, previous); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents) SELECT ?,t.category_id,t.amount_cents FROM targets t WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id)", id, previous); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM targets WHERE period_id=?", id); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO targets SELECT period_id,category_id,SUM(amount_cents) FROM group_targets WHERE period_id=? AND included=1 GROUP BY period_id,category_id", id); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO budget_groups SELECT DISTINCT period_id,spending_group_id FROM group_targets WHERE period_id=?", id); err != nil {
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
	var b targetsInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := a.write(func(tx *sql.Tx) error { return updateTargetsTx(tx, u, id, b) }); err != nil {
		return err
	}
	success(w)
	return nil
}

type targetItem struct {
	CategoryID        int64  `json:"category_id"`
	Amount            int64  `json:"amount_cents"`
	SpendingGroupID   *int64 `json:"spending_group_id,omitempty"`
	GroupID           *int64 `json:"group_id,omitempty"`
	SpendingGroupName string `json:"spending_group_name,omitempty"`
}

type targetGroupItem struct {
	GroupID           *int64       `json:"group_id,omitempty"`
	SpendingGroupID   *int64       `json:"spending_group_id,omitempty"`
	SpendingGroupName string       `json:"spending_group_name,omitempty"`
	Targets           []targetItem `json:"targets"`
}

type targetsInput struct {
	Version         int64             `json:"version"`
	GroupID         *int64            `json:"group_id,omitempty"`
	SpendingGroupID *int64            `json:"spending_group_id,omitempty"`
	Merge           bool              `json:"merge,omitempty"`
	Targets         []targetItem      `json:"targets"`
	Groups          []targetGroupItem `json:"groups,omitempty"`
}

// canonicalTargetsTx freezes names to IDs and preserves the independent budget scope.
// Omitted scopes are legacy No spending group only; category/rule metadata is never a budget scope.
func canonicalTargetsTx(tx *sql.Tx, id int64, b targetsInput) (targetsInput, error) {
	resolve := func(first, second *int64, name string) (*int64, error) {
		if first != nil && second != nil && *first != *second {
			return nil, fail(400, "Conflicting spending groups")
		}
		value := first
		if value == nil {
			value = second
		}
		if strings.TrimSpace(name) != "" {
			var gid int64
			if err := tx.QueryRow("SELECT id FROM spending_groups WHERE name=? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&gid); err != nil {
				return nil, fail(400, "Choose an existing spending group")
			}
			if value != nil && *value != gid {
				return nil, fail(400, "Conflicting spending groups")
			}
			value = &gid
		}
		if value != nil {
			gid := *value
			if gid < 0 || gid > 0 && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", gid) != 1 {
				return nil, fail(400, "Choose an existing spending group")
			}
			value = &gid
		}
		return value, nil
	}
	root, err := resolve(b.GroupID, b.SpendingGroupID, "")
	if err != nil {
		return b, err
	}
	targets := append([]targetItem(nil), b.Targets...)
	canonicalGroups := []targetGroupItem{}
	groupSeen := map[int64]bool{}
	for _, g := range b.Groups {
		gid, e := resolve(g.GroupID, g.SpendingGroupID, g.SpendingGroupName)
		if e != nil {
			return b, e
		}
		if gid == nil {
			return b, fail(400, "Specify the spending group for each budget group")
		}
		if groupSeen[*gid] {
			return b, fail(400, "Each budget group can appear only once")
		}
		groupSeen[*gid] = true
		canonicalGroups = append(canonicalGroups, targetGroupItem{GroupID: gid, Targets: []targetItem{}})
		for _, t := range g.Targets {
			own, e := resolve(t.GroupID, t.SpendingGroupID, t.SpendingGroupName)
			if e != nil {
				return b, e
			}
			if own != nil && *own != *gid {
				return b, fail(400, "Conflicting spending groups")
			}
			t.GroupID = gid
			t.SpendingGroupID = nil
			t.SpendingGroupName = ""
			targets = append(targets, t)
		}
	}
	if root != nil && len(b.Groups) > 0 {
		return b, fail(400, "Choose a root group or grouped targets, not both")
	}
	legacy := false
	for i, t := range targets {
		gid, e := resolve(t.GroupID, t.SpendingGroupID, t.SpendingGroupName)
		if e != nil {
			return b, e
		}
		if root != nil {
			if gid != nil && *gid != *root {
				return b, fail(400, "Conflicting spending groups")
			}
			gid = root
		}
		if gid == nil {
			legacy = true
			if queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND spending_group_id IS NOT NULL AND included=1", id, t.CategoryID) > 0 {
				return b, fail(400, "Specify the spending group for this category budget")
			}
			zero := int64(0)
			gid = &zero
		}
		targets[i] = targetItem{CategoryID: t.CategoryID, Amount: t.Amount, GroupID: gid}
	}
	if root == nil && !b.Merge && (legacy || len(targets) == 0 && len(b.Groups) == 0) && queryInt(tx, "SELECT COUNT(*) FROM budget_groups WHERE period_id=? AND spending_group_id IS NOT NULL", id) > 0 {
		return b, fail(400, "Specify explicit group budgets before replacing a grouped budget")
	}
	b.GroupID = root
	b.SpendingGroupID = nil
	b.Groups = canonicalGroups
	b.Targets = targets
	return b, nil
}

func updateTargetsTx(tx *sql.Tx, u User, id int64, b targetsInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	var err error
	b, err = canonicalTargetsTx(tx, id, b)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, t := range b.Targets {
		gid := *t.GroupID
		key := fmt.Sprintf("%d:%d", gid, t.CategoryID)
		if t.Amount < 0 || t.Amount > 900000000000000 || seen[key] || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND kind='expense'", t.CategoryID) != 1 {
			return fail(400, "Choose unique expense categories and nonnegative limits")
		}
		if queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 1 && queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND COALESCE(spending_group_id,0)=? AND included=1", id, t.CategoryID, gid) == 0 {
			return fail(400, "Archived categories cannot receive new budgets")
		}
		seen[key] = true
	}
	res, err := tx.Exec("UPDATE periods SET version=version+1 WHERE id=? AND version=?", id, b.Version)
	if err != nil {
		return err
	}
	if err = affected(res); err != nil {
		return err
	}
	// Preserve undistributed legacy aggregate rows before rebuilding totals, just as period copying does.
	// Never infer a category-owned group or overwrite any existing canonical entry.
	legacy, err := data(tx, "SELECT t.category_id,c.name category_name,t.amount_cents FROM targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id) ORDER BY t.category_id", id)
	if err != nil {
		return err
	}
	if len(legacy) > 0 {
		if _, err = tx.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward,included) SELECT t.period_id,t.category_id,t.amount_cents,CASE WHEN c.kind='expense' AND c.archived=0 AND t.amount_cents>0 THEN 1 ELSE 0 END,1 FROM targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id)", id); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,NULL)", id); err != nil {
			return err
		}
		if err = audit(tx, u, nil, "period", id, "legacy_limits_preserved", legacy); err != nil {
			return err
		}
	}
	// Replacement preserves recurrence for retained entries rather than restarting it.
	oldRows, err := data(tx, "SELECT COALESCE(spending_group_id,0) group_id,category_id,carry_forward FROM group_targets WHERE period_id=?", id)
	if err != nil {
		return err
	}
	carry := map[string]bool{}
	for _, row := range oldRows {
		carry[fmt.Sprintf("%d:%d", num(row["group_id"]), num(row["category_id"]))] = num(row["carry_forward"]) != 0
	}
	if !b.Merge {

		if b.GroupID != nil {
			_, err = tx.Exec("DELETE FROM group_targets WHERE period_id=? AND COALESCE(spending_group_id,0)=?", id, *b.GroupID)
		} else {
			_, err = tx.Exec("DELETE FROM group_targets WHERE period_id=?", id)
			if err == nil {
				_, err = tx.Exec("DELETE FROM budget_groups WHERE period_id=?", id)
			}
		}
		if err != nil {
			return err
		}
	}
	for _, t := range b.Targets {
		var group any
		if *t.GroupID != 0 {
			group = *t.GroupID
		}
		recurring := queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 0
		if saved, ok := carry[fmt.Sprintf("%d:%d", *t.GroupID, t.CategoryID)]; ok {
			recurring = saved
		}
		if _, err = tx.Exec("INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward,included) VALUES(?,?,?,?,?,1) ON CONFLICT DO UPDATE SET amount_cents=excluded.amount_cents,included=1", id, t.CategoryID, group, t.Amount, recurring); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	for _, g := range b.Groups {
		var group any
		if *g.GroupID != 0 {
			group = *g.GroupID
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	if b.GroupID != nil {
		var group any
		if *b.GroupID != 0 {
			group = *b.GroupID
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT COUNT(*) FROM (SELECT category_id FROM group_targets WHERE period_id=? AND included=1 GROUP BY category_id HAVING SUM(amount_cents)>900000000000000)", id) > 0 {
		return fail(400, "Combined category budgets exceed the supported limit")
	}
	if err = syncBudgetTotalsTx(tx, id); err != nil {
		return err
	}
	return audit(tx, u, nil, "period", id, "limits_updated", b)
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
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pid, _ := strconv.ParseInt(r.URL.Query().Get("period"), 10, 64)
	if pid == 0 {
		loc, _ := time.LoadLocation("Africa/Johannesburg")
		today := time.Now().In(loc).Format("2006-01-02")
		pid = queryInt(tx, "SELECT id FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", today, today)
		if pid == 0 {
			pid = queryInt(tx, "SELECT id FROM periods ORDER BY start_date DESC,id DESC LIMIT 1")
		}
	}
	periods, err := data(tx, "SELECT * FROM periods WHERE id=?", pid)
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
		if !a.can(tx, u, account, false) {
			return fail(403, "Account access required")
		}
		household := queryInt(tx, "SELECT household FROM accounts WHERE id=?", account) == 1
		condition = "t.account_id=? AND t.date>=? AND t.date<=?"
		args = []any{account, p["start_date"], p["end_date"]}
		if household {
			condition = "t.account_id=? AND t.period_id=?"
			args = []any{account, pid}
		}
		showTargets = false
	}
	rows, err := data(tx, "SELECT t.id,t.spending_group_id,g.name spending_group_name,g.color spending_group_color,t.review_state,t.is_transfer,l.amount_cents,c.id category_id,c.name,c.group_name,c.kind FROM transactions t JOIN accounts a ON a.id=t.account_id JOIN allocations l ON l.transaction_id=t.id LEFT JOIN categories c ON c.id=l.category_id LEFT JOIN spending_groups g ON g.id=t.spending_group_id WHERE a.sync_hidden=0 AND "+condition+accountScopeSQL(u), args...)
	if err != nil {
		return err
	}
	income, spent, pendingSpend := int64(0), int64(0), int64(0)
	pendingCount := map[int64]bool{}
	uncategorized := map[int64]bool{}
	categories := map[int64]map[string]any{}
	catRows, err := data(tx, "SELECT c.*,COALESCE(t.amount_cents,0) target_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? ORDER BY c.name,c.id", pid)
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
	budgetRows := []map[string]any{}
	if showTargets {
		budgetRows, err = data(tx, "SELECT gt.category_id,gt.amount_cents,gt.spending_group_id,g.name spending_group_name,g.color spending_group_color,c.name FROM group_targets gt LEFT JOIN spending_groups g ON g.id=gt.spending_group_id JOIN categories c ON c.id=gt.category_id WHERE gt.period_id=? AND gt.included=1", pid)
		if err != nil {
			return err
		}
	}
	groups, groupTotal, err := dashboardGroups(r, rows, categories, budgetRows)
	if err != nil {
		return err
	}
	incomeCategories, incomeTotal, err := dashboardIncome(r, rows)
	if err != nil {
		return err
	}
	var limits int64
	for _, c := range catRows {
		limits += num(c["target_cents"])
	}
	unassigned := queryInt(tx, "SELECT COUNT(*) FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.sync_hidden=0 AND a.household=1 AND t.period_id IS NULL AND t.assignment!='outside'"+accountScopeSQL(u))
	balanceScope := " AND a.household=1"
	balanceArgs := []any{u.Member, u.ID}
	if account != 0 {
		balanceScope = " AND a.id=?"
		balanceArgs = append(balanceArgs, account)
	}
	balances, err := data(tx, "SELECT a.id,a.name,a.household,a.balance_cents,a.balance_date FROM accounts a WHERE "+accountAccessSQL(u)+balanceScope+" ORDER BY a.name", balanceArgs...)
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
	budgetSort(catRows, r.URL.Query().Get("sort"))
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
	send(w, map[string]any{"period": p, "income_cents": income, "income_categories": incomeCategories, "income_category_total": incomeTotal, "spent_cents": spent, "pending_spend_cents": pendingSpend, "pending_count": len(pendingCount), "uncategorized_count": len(uncategorized), "budget_cents": limits, "remaining_cents": limits - spent, "has_targets": showTargets, "spending_groups": groups, "group_total": groupTotal, "categories": catRows, "category_total": categoryTotal, "balances": balances, "balance_total": balanceTotal, "unassigned_count": unassigned})
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
	if r.URL.Query().Has("group") {
		gid, err := strconv.ParseInt(r.URL.Query().Get("group"), 10, 64)
		if err != nil || gid < 0 || gid > 0 && queryInt(a.DB, "SELECT COUNT(*) FROM spending_groups WHERE id=?", gid) != 1 {
			return fail(400, "Choose a spending group")
		}
		query := "SELECT c.id,c.name,c.kind,COALESCE(t.amount_cents,0) amount_cents,t.carry_forward FROM categories c LEFT JOIN group_targets t ON t.category_id=c.id AND t.period_id=? AND COALESCE(t.spending_group_id,0)=? WHERE c.kind='expense'"
		if r.URL.Query().Get("budget_only") == "1" {
			query += " AND t.included=1"
		}
		return a.metadataPage(w, r, query, []any{id, gid}, "name", "name,id")
	}
	return a.metadataPage(w, r, "SELECT c.id,c.name,c.kind,COALESCE(t.amount_cents,0) amount_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? WHERE c.kind='expense'", []any{id}, "name", "name,id")
}
