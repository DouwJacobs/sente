package app

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"modernc.org/sqlite"
	"net/http"
	"strconv"
	"strings"
)

// Group paging uses the same Unicode/whitespace normalization as classification.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("finance_normalize", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		value, _ := args[0].(string)
		return normalize(value), nil
	})
}

// Explicit pages retain compatibility for existing API consumers. UI list requests use this boundary.
func listPage(r *http.Request) (int, int, error) {
	page := 0
	size := 20
	if raw := r.URL.Query().Get("page"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 0 || v > 1000000 {
			return 0, 0, fail(400, "Choose a valid page")
		}
		page = v
	}
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 1 || v > 100 {
			return 0, 0, fail(400, "Page size must be 1–100")
		}
		size = v
	}
	return page, size, nil
}
func (a *App) rulePages(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	u := Current(r)
	// Rank definitions before paging so grouped account edits never lose a member at a page boundary.
	query := `WITH visible AS (
 SELECT rules.id,rules.account_id,rules.pattern,rules.category_id,rules.priority,rules.spending_group_id,rules.direction,rules.enabled,rules.version,0 builtin,c.name category_name,a.name account_name,s.name spending_group_name FROM rules JOIN accounts a ON a.id=rules.account_id JOIN categories c ON c.id=rules.category_id LEFT JOIN spending_groups s ON s.id=rules.spending_group_id WHERE ` + accessSQL + `
 UNION ALL SELECT -b.id,0,b.pattern,b.category_id,b.priority,b.spending_group_id,b.direction,b.enabled,b.version,1,c.name,'All enabled accounts',s.name FROM builtin_rules b JOIN categories c ON c.id=b.category_id LEFT JOIN spending_groups s ON s.id=b.spending_group_id
 ), definitions AS (SELECT finance_normalize(pattern) pattern_key,category_id,spending_group_id,direction,priority,enabled,builtin,MIN(abs(id)) first_id FROM visible GROUP BY finance_normalize(pattern),category_id,spending_group_id,direction,priority,enabled,builtin), ranked AS (SELECT *,ROW_NUMBER() OVER(ORDER BY builtin,priority DESC,first_id) position FROM definitions)
 `
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	counts, err := data(tx, query+"SELECT COUNT(*) total FROM ranked", u.Member, u.ID)
	if err != nil {
		return err
	}
	states, err := data(tx, query+"SELECT group_concat(id||':'||version) state FROM (SELECT id,version FROM visible ORDER BY id)", u.Member, u.ID)
	if err != nil {
		return err
	}
	version := hash(fmt.Sprint(states[0]["state"]))
	if expected := r.URL.Query().Get("list_version"); expected != "" && expected != version {
		return fail(409, "This list changed. Start from the first page.")
	}
	items, err := data(tx, query+`SELECT v.*,finance_normalize(v.pattern) normalized_pattern FROM visible v JOIN ranked d ON finance_normalize(v.pattern)=d.pattern_key AND v.category_id=d.category_id AND v.spending_group_id IS d.spending_group_id AND v.direction=d.direction AND v.priority=d.priority AND v.enabled=d.enabled AND v.builtin=d.builtin WHERE d.position>? AND d.position<=? ORDER BY d.position,abs(v.id)`, u.Member, u.ID, page*size, (page+1)*size)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items, "total": counts[0]["total"], "page": page, "page_size": size, "list_version": version})
	return nil
}

type importPage struct {
	ParsedFile
	Total          int    `json:"total"`
	ErrorCount     int    `json:"error_count"`
	CandidateCount int    `json:"candidate_count"`
	Page           int    `json:"page"`
	PageSize       int    `json:"page_size"`
	PreviewVersion string `json:"preview_version"`
}

func importPreviewVersion(p ParsedFile) string {
	rows, _ := json.Marshal(p.Rows)
	return hash(p.ClassificationVersion + string(rows))
}
func previewPage(p ParsedFile, page, size int) importPage {
	result := importPage{PreviewVersion: importPreviewVersion(p), ParsedFile: p, Total: len(p.Rows), Page: page, PageSize: size}
	for _, row := range p.Rows {
		if row.Error != "" {
			result.ErrorCount++
		} else if row.Duplicate == "possible" || row.Duplicate == "conflict" {
			result.CandidateCount++
		}
	}
	start := page * size
	if start > len(p.Rows) {
		start = len(p.Rows)
	}
	end := start + size
	if end > len(p.Rows) {
		end = len(p.Rows)
	}
	result.Rows = append([]SourceRow{}, p.Rows[start:end]...)
	for i := range result.Rows {
		row := &result.Rows[i]
		row.RuleMatchCount = len(row.RuleMatches)
		if len(row.RuleMatches) > 5 {
			row.RuleMatches = row.RuleMatches[:5]
		}
	}
	result.Decisions = map[string]string{}
	for _, row := range result.Rows {
		if decision := p.Decisions[strconv.Itoa(row.Row)]; decision != "" {
			result.Decisions[strconv.Itoa(row.Row)] = decision
		}
	}
	return result
}
func sendImportPreviews(w http.ResponseWriter, r *http.Request, previews []ParsedFile) {
	if r.URL.Query().Get("paged") != "1" {
		pages := make([]importPage, 0, len(previews))
		for _, p := range previews {
			pages = append(pages, previewPage(p, 0, 100))
		}
		send(w, pages)
		return
	}
	pages := make([]importPage, 0, len(previews))
	for _, p := range previews {
		pages = append(pages, previewPage(p, 0, 50))
	}
	send(w, pages)
}
func (a *App) importPages(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	u := Current(r)
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	where := " FROM imports i JOIN accounts a ON a.id=i.account_id WHERE " + accessSQL
	counts, err := data(tx, "SELECT COUNT(*) total"+where, u.Member, u.ID)
	if err != nil {
		return err
	}
	states, err := data(tx, "SELECT group_concat(id) state FROM (SELECT i.id"+where+" ORDER BY i.id)", u.Member, u.ID)
	if err != nil {
		return err
	}
	version := hash(fmt.Sprint(states[0]["state"]))
	if expected := r.URL.Query().Get("list_version"); expected != "" && expected != version {
		return fail(409, "This list changed. Start from the first page.")
	}
	items, err := data(tx, `SELECT i.id,i.account_id,i.name,i.format,i.status,i.created_at,json_array_length(i.data,'$.rows') row_count,json_extract(i.data,'$.inserted') inserted,json_extract(i.data,'$.skipped') skipped,json_extract(i.data,'$.error') error,(SELECT COUNT(*) FROM json_each(i.data,'$.rows') j WHERE COALESCE(json_extract(j.value,'$.error'),'')!='') error_count`+where+" ORDER BY i.id DESC LIMIT ? OFFSET ?", u.Member, u.ID, size, page*size)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items, "total": counts[0]["total"], "page": page, "list_version": version})
	return nil
}
func (a *App) importRows(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	u := Current(r)
	id := parseID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := data(tx, "SELECT i.data,i.status FROM imports i JOIN accounts a ON a.id=i.account_id WHERE i.id=? AND "+accessSQL, id, u.Member, u.ID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fail(404, "Import not found or inaccessible")
	}
	var p ParsedFile
	if err = json.Unmarshal([]byte(rows[0]["data"].(string)), &p); err != nil {
		return err
	}
	p.ID = id
	if rows[0]["status"] == "staged" {
		if err = a.annotate(tx, queryInt(tx, "SELECT account_id FROM imports WHERE id=?", id), &p); err != nil {
			return err
		}
	}
	send(w, previewPage(p, page, size))
	return nil
}

// Reference list pages support server-side search and exact selection hydration.
func (a *App) metadataPage(w http.ResponseWriter, r *http.Request, query string, args []any, search string, order string) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	if r.URL.Path == "/api/categories" && r.URL.Query().Get("sort") == "usage" {
		order = "usage_count DESC,name,id"
	}
	base := " FROM (" + query + ") list WHERE 1=1"
	if q := r.URL.Query().Get("q"); q != "" {
		base += " AND finance_normalize(" + search + ") LIKE finance_normalize(?)"
		args = append(args, "%"+strings.TrimSpace(q)+"%")
	}
	if r.URL.Path == "/api/accounts" && r.URL.Query().Get("role") == "editor" {
		base += " AND role='editor'"
	}
	if id := r.URL.Query().Get("id"); id != "" {
		base += " AND id=?"
		args = append(args, id)
	}
	if kind := r.URL.Query().Get("kind"); kind != "" && r.URL.Path == "/api/categories" {
		base += " AND kind=?"
		args = append(args, kind)
	}
	if hidden := r.URL.Query().Get("hidden"); hidden != "" && (r.URL.Path == "/api/accounts/manage") {
		if hidden != "0" && hidden != "1" {
			return fail(400, "Choose shown or hidden accounts")
		}
		base += " AND sync_hidden=?"
		args = append(args, hidden)
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	counts, err := data(tx, "SELECT COUNT(*) total"+base, args...)
	if err != nil {
		return err
	}
	signature := "json_array(id,version)"
	switch r.URL.Path {
	case "/api/categories":
		signature = "json_array(id,name,kind)"
	case "/api/grants":
		signature = "json_array(user_id,account_id,role)"
	default:
		if strings.HasPrefix(r.URL.Path, "/api/audit/") {
			signature = "json_array(id)"
		} else if strings.HasSuffix(r.URL.Path, "/targets") {
			signature = "json_array(id,amount_cents)"
		}
	}
	states, err := data(tx, "SELECT group_concat(signature,'|') state FROM (SELECT "+signature+" signature"+base+" ORDER BY "+order+")", args...)
	if err != nil {
		return err
	}
	version := hash(fmt.Sprint(states[0]["state"]))
	if expected := r.URL.Query().Get("list_version"); expected != "" && expected != version {
		return fail(409, "This list changed. Start from the first page.")
	}
	pageArgs := append(append([]any{}, args...), size, page*size)
	items, err := data(tx, "SELECT *"+base+" ORDER BY "+order+" LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items, "total": counts[0]["total"], "page": page, "page_size": size, "list_version": version})
	return nil
}

func (a *App) importSuggestions(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	u := Current(r)
	id := parseID(r)
	rowID, err := strconv.Atoi(r.PathValue("row"))
	if err != nil {
		return fail(400, "Choose an import row")
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	records, err := data(tx, "SELECT i.data,i.status,i.account_id FROM imports i JOIN accounts a ON a.id=i.account_id WHERE i.id=? AND "+accessSQL, id, u.Member, u.ID)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fail(404, "Import not found or inaccessible")
	}
	var p ParsedFile
	if err = json.Unmarshal([]byte(records[0]["data"].(string)), &p); err != nil {
		return err
	}
	if records[0]["status"] == "staged" {
		if err = a.annotate(tx, num(records[0]["account_id"]), &p); err != nil {
			return err
		}
	}
	version := importPreviewVersion(p)
	if expected := r.URL.Query().Get("preview_version"); expected != "" && expected != version {
		return fail(409, "Import preview changed. Reload it before confirming.")
	}
	for _, row := range p.Rows {
		if row.Row != rowID {
			continue
		}
		total := len(row.RuleMatches)
		start := page * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}
		send(w, map[string]any{"items": row.RuleMatches[start:end], "total": total, "list_version": version})
		return nil
	}
	return fail(404, "Import row not found")
}
