package app

import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strconv"
	"strings"
)

const accessSQL = "(a.sync_hidden=0 AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=?)))"

func (a *App) accounts(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT a.*,COALESCE((SELECT json_extract(i.data,'$.account_type') FROM imports i WHERE i.account_id=a.id AND COALESCE(json_extract(i.data,'$.account_type'),'')!='' AND COALESCE(json_extract(i.data,'$.error'),'')='' ORDER BY i.id DESC LIMIT 1),'') account_type,EXISTS(SELECT 1 FROM fnb_discoveries d WHERE d.account_id=a.id AND d.bank_id=a.bank_id AND d.user_id="+strconv.FormatInt(u.ID, 10)+" AND d.hidden=0) fnb_connected,CASE WHEN a.household=1 AND ?=1 THEN 'editor' ELSE (SELECT role FROM grants WHERE user_id=? AND account_id=a.id) END role FROM accounts a WHERE a.sync_hidden=0 AND "+accountAccessSQL(u), []any{u.Member, u.ID, u.Member, u.ID}, "name", "name,id")
	}
	v, err := data(a.DB, "SELECT a.*,COALESCE((SELECT json_extract(i.data,'$.account_type') FROM imports i WHERE i.account_id=a.id AND COALESCE(json_extract(i.data,'$.account_type'),'')!='' AND COALESCE(json_extract(i.data,'$.error'),'')='' ORDER BY i.id DESC LIMIT 1),'') account_type,EXISTS(SELECT 1 FROM fnb_discoveries d WHERE d.account_id=a.id AND d.bank_id=a.bank_id AND d.user_id="+strconv.FormatInt(u.ID, 10)+" AND d.hidden=0) fnb_connected,CASE WHEN a.household=1 AND ?=1 THEN 'editor' ELSE (SELECT role FROM grants WHERE user_id=? AND account_id=a.id) END role FROM accounts a WHERE a.sync_hidden=0 AND "+accountAccessSQL(u)+" ORDER BY a.name,a.id LIMIT 100", u.Member, u.ID, u.Member, u.ID)
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}

type accountInput struct {
	Name      string `json:"name"`
	BankID    string `json:"bank_id"`
	Household bool   `json:"household"`
	Version   int64  `json:"version"`
}

func validAccount(b accountInput) error {
	if strings.TrimSpace(b.Name) == "" || len(b.Name) > 100 || len(b.BankID) < 3 || len(b.BankID) > 64 {
		return fail(400, "Provide an account name and bank account number")
	}
	return nil
}
func (a *App) createAccount(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b accountInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := validAccount(b); err != nil {
		return err
	}
	var id int64
	err := a.write(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO accounts(name,bank_id,household) VALUES(?,?,?)", strings.TrimSpace(b.Name), strings.TrimSpace(b.BankID), b.Household)
		if err != nil {
			return fail(409, "Bank account already exists")
		}
		id, _ = res.LastInsertId()
		if _, err := tx.Exec("INSERT INTO grants VALUES(?,?,'editor')", u.ID, id); err != nil {
			return err
		}
		return audit(tx, u, id, "account", id, "created", map[string]any{"name": b.Name, "household": b.Household})
	})
	if err != nil {
		return err
	}
	send(w, map[string]int64{"id": id})
	return nil
}
func (a *App) updateAccount(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b accountInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := validAccount(b); err != nil {
		return err
	}
	id := parseID(r)
	err := a.write(func(tx *sql.Tx) error {
		var oldBank string
		if err := tx.QueryRow("SELECT bank_id FROM accounts WHERE id=?", id).Scan(&oldBank); err != nil {
			return fail(404, "Account not found")
		}
		if b.BankID != oldBank && queryInt(tx, "SELECT COUNT(*) FROM transactions WHERE account_id=?", id) > 0 {
			return fail(400, "Bank account number cannot change after importing")
		}
		result, err := tx.Exec("UPDATE accounts SET name=?,bank_id=?,household=?,version=version+1 WHERE id=? AND version=?", b.Name, b.BankID, b.Household, id, b.Version)
		if err != nil {
			return fail(409, "Bank account number already exists")
		}
		if err := affected(result); err != nil {
			return err
		}
		if err := reassign(tx); err != nil {
			return err
		}
		return audit(tx, u, id, "account", id, "updated", map[string]any{"name": b.Name, "household": b.Household})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) users(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT id,username,admin,budget_member,disabled,version FROM users", []any{}, "username", "username,id")
	}
	v, err := data(a.DB, "SELECT id,username,admin,budget_member,disabled,version FROM users ORDER BY username LIMIT 100")
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}
func (a *App) createUser(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
		Member   bool   `json:"budget_member"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	id, err := a.CreateUser(b.Username, b.Password, b.Admin, b.Member)
	if err != nil {
		return err
	}
	if err := audit(a.DB, u, nil, "user", id, "created", map[string]any{"username": b.Username, "admin": b.Admin, "budget_member": b.Member}); err != nil {
		return err
	}
	send(w, map[string]int64{"id": id})
	return nil
}
func (a *App) grants(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT g.*,u.username,a.name account_name FROM grants g JOIN users u ON u.id=g.user_id JOIN accounts a ON a.id=g.account_id", []any{}, "username||' '||account_name", "user_id,account_id")
	}
	v, err := data(a.DB, "SELECT g.*,u.username,a.name account_name FROM grants g JOIN users u ON u.id=g.user_id JOIN accounts a ON a.id=g.account_id ORDER BY g.user_id,g.account_id LIMIT 100")
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}
func (a *App) grant(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		UserID    int64  `json:"user_id"`
		AccountID int64  `json:"account_id"`
		Role      string `json:"role"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Role != "viewer" && b.Role != "editor" && b.Role != "" {
		return fail(400, "Role must be viewer or editor, or empty to revoke")
	}
	err := a.write(func(tx *sql.Tx) error {
		if queryInt(tx, "SELECT COUNT(*) FROM users WHERE id=?", b.UserID) == 0 || queryInt(tx, "SELECT COUNT(*) FROM accounts WHERE id=?", b.AccountID) == 0 {
			return fail(404, "User or account not found")
		}
		var household bool
		tx.QueryRow("SELECT household FROM accounts WHERE id=?", b.AccountID).Scan(&household)
		var member bool
		tx.QueryRow("SELECT budget_member FROM users WHERE id=?", b.UserID).Scan(&member)
		if household && member && b.Role != "editor" {
			return fail(400, "Household members always have editor access; make the account private before restricting it")
		}
		if _, err := tx.Exec("DELETE FROM grants WHERE user_id=? AND account_id=?", b.UserID, b.AccountID); err != nil {
			return err
		}
		if b.Role != "" {
			if _, err := tx.Exec("INSERT INTO grants VALUES(?,?,?)", b.UserID, b.AccountID, b.Role); err != nil {
				return err
			}
		}
		return audit(tx, u, b.AccountID, "account", b.AccountID, "access_changed", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) categories(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Query().Has("spending_group") || r.URL.Query().Has("spending_group_id") {
		return fail(400, "Categories are flat; filter transaction or budget groups instead")
	}
	u := Current(r)
	if r.URL.Query().Has("page") {
		query := "SELECT c.id,c.name,c.group_name,c.kind,c.archived,c.version,(SELECT COUNT(DISTINCT l.transaction_id) FROM allocations l JOIN transactions t ON t.id=l.transaction_id JOIN accounts a ON a.id=t.account_id WHERE l.category_id=c.id AND " + accountAccessSQL(u) + ") usage_count FROM categories c WHERE 1=1"
		if r.URL.Query().Get("kind") == "expense" {
			query += " AND c.kind='expense'"
		}
		if r.URL.Query().Get("active") == "1" {
			query += " AND c.archived=0"
		}

		return a.metadataPage(w, r, query, []any{u.Member, u.ID}, "name", "kind,name,id")
	}
	v, err := data(a.DB, "SELECT c.id,c.name,c.group_name,c.kind,c.archived,c.version,(SELECT COUNT(DISTINCT l.transaction_id) FROM allocations l JOIN transactions t ON t.id=l.transaction_id JOIN accounts a ON a.id=t.account_id WHERE l.category_id=c.id AND "+accountAccessSQL(u)+") usage_count FROM categories c ORDER BY c.kind,c.name,c.id LIMIT 100", u.Member, u.ID)
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}
func (a *App) createCategory(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	var b categoryInput
	if err := decode(r, &b); err != nil {
		return err
	}
	var id int64
	err := a.write(func(tx *sql.Tx) error { var err error; id, err = createCategoryTx(tx, Current(r), b); return err })
	if err != nil {
		return err
	}
	send(w, map[string]any{"id": id})
	return nil
}

type categoryInput struct {
	Name  string `json:"name"`
	Group string `json:"group_name,omitempty"`
	Kind  string `json:"kind"`
}

func createCategoryTx(tx *sql.Tx, u User, b categoryInput) (int64, error) {
	return createCategoryRecordTx(tx, u, b, "", false)
}

// Import preserves historical category identity without creating spending-group ownership.
func createCategoryRecordTx(tx *sql.Tx, u User, b categoryInput, legacy string, imported bool) (int64, error) {
	if err := requireMember(u); err != nil {
		return 0, err
	}
	if strings.TrimSpace(b.Name) == "" || (!imported && len(b.Name) > 80) || (imported && len([]rune(b.Name)) > 100) || (b.Kind != "expense" && b.Kind != "income") {
		return 0, fail(400, "Provide a category name and expense/income type")
	}
	if !imported && queryInt(tx, "SELECT COUNT(*) FROM categories WHERE finance_normalize(name)=finance_normalize(?)", strings.TrimSpace(b.Name)) > 0 {
		return 0, fail(409, "Category already exists")
	}
	if len(legacy) > 100 {
		return 0, fail(400, "Historical category key is too long")
	}
	res, err := tx.Exec("INSERT INTO categories(name,group_name,kind) VALUES(?,?,?)", strings.TrimSpace(b.Name), legacy, b.Kind)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, audit(tx, u, nil, "category", id, "created", map[string]any{"name": strings.TrimSpace(b.Name), "kind": b.Kind, "group_name": legacy})
}

func (a *App) manageAccounts(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT id,name,bank_id,household,version,sync_hidden,CASE WHEN household=1 AND "+strconv.FormatBool(u.Member)+" THEN 1 ELSE EXISTS(SELECT 1 FROM grants g WHERE g.account_id=accounts.id AND g.user_id="+strconv.FormatInt(u.ID, 10)+" AND g.role='editor') END visibility_allowed,EXISTS(SELECT 1 FROM fnb_discoveries d WHERE d.account_id=accounts.id AND d.bank_id=accounts.bank_id AND d.user_id="+strconv.FormatInt(u.ID, 10)+" AND d.hidden=0) fnb_connected FROM accounts", []any{}, "name", "name,id")
	}
	v, err := data(a.DB, "SELECT id,name,bank_id,household,version,sync_hidden,CASE WHEN household=1 AND "+strconv.FormatBool(u.Member)+" THEN 1 ELSE EXISTS(SELECT 1 FROM grants g WHERE g.account_id=accounts.id AND g.user_id="+strconv.FormatInt(u.ID, 10)+" AND g.role='editor') END visibility_allowed,EXISTS(SELECT 1 FROM fnb_discoveries d WHERE d.account_id=accounts.id AND d.bank_id=accounts.bank_id AND d.user_id="+strconv.FormatInt(u.ID, 10)+" AND d.hidden=0) fnb_connected FROM accounts ORDER BY name,id LIMIT 100")
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}
func (a *App) changePassword(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	var b struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.New) < 12 || len(b.New) > 72 {
		return fail(400, "New password must be 12–72 bytes")
	}
	var old string
	if err := a.DB.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&old); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(old), []byte(b.Old)) != nil {
		return fail(400, "Current password is incorrect")
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(b.New), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	cookie, _ := r.Cookie("finance_session")
	err = a.write(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE users SET password=? WHERE id=?", string(digest), u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM mcp_tokens WHERE user_id=?", u.ID); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM sessions WHERE user_id=? AND token!=?", u.ID, hash(cookie.Value))
		return err
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

func (a *App) updateUser(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	id := parseID(r)
	var b struct {
		Username string `json:"username"`
		Admin    bool   `json:"admin"`
		Member   bool   `json:"budget_member"`
		Disabled bool   `json:"disabled"`
		Version  int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	b.Username = strings.TrimSpace(b.Username)
	if len(b.Username) < 2 || len(b.Username) > 80 {
		return fail(400, "Username must be 2–80 characters")
	}
	err := a.write(func(tx *sql.Tx) error {
		var wasAdmin bool
		if err := tx.QueryRow("SELECT admin FROM users WHERE id=?", id).Scan(&wasAdmin); err != nil {
			return fail(404, "User not found")
		}
		if wasAdmin && (!b.Admin || b.Disabled) && queryInt(tx, "SELECT COUNT(*) FROM users WHERE admin=1 AND disabled=0 AND id!=?", id) == 0 {
			return fail(400, "Keep at least one enabled administrator")
		}
		res, err := tx.Exec("UPDATE users SET username=?,admin=?,budget_member=?,disabled=?,version=version+1 WHERE id=? AND version=?", b.Username, b.Admin, b.Member, b.Disabled, id, b.Version)
		if err != nil {
			return fail(409, "Username already exists")
		}
		if err := affected(res); err != nil {
			return err
		}
		if b.Disabled {
			if _, err := tx.Exec("DELETE FROM mcp_tokens WHERE user_id=?", id); err != nil {
				return err
			}
			if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", id); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "user", id, "updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

func (a *App) accountVisibility(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Hidden  bool  `json:"hidden"`
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	id := parseID(r)
	err := a.write(func(tx *sql.Tx) error {
		if !a.can(tx, u, id, true) {
			return fail(403, "Account editor access required")
		}
		result, err := tx.Exec("UPDATE accounts SET sync_hidden=?,version=version+1 WHERE id=? AND version=?", b.Hidden, id, b.Version)
		if err != nil {
			return err
		}
		if err = affected(result); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE fnb_discoveries SET hidden=? WHERE account_id=? AND user_id=?", b.Hidden, id, u.ID); err != nil {
			return err
		}
		return audit(tx, u, id, "account", id, "visibility_updated", map[string]any{"hidden": b.Hidden})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
