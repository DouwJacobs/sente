package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
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
