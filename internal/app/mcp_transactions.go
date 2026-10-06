package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Typed filters are additive to the legacy string-map list_transactions input.
type mcpTransactionFilters struct {
	DateFrom       string  `json:"date_from,omitempty" jsonschema:"Inclusive local calendar date YYYY-MM-DD"`
	DateTo         string  `json:"date_to,omitempty" jsonschema:"Inclusive local calendar date YYYY-MM-DD"`
	CategoryIDs    []int64 `json:"category_ids,omitempty" jsonschema:"Match any of 1-100 distinct positive category IDs including split allocations"`
	Categorization string  `json:"categorization,omitempty" jsonschema:"categorized or uncategorized or any; explicit transfers are category-exempt"`
	Acceptance     string  `json:"acceptance,omitempty" jsonschema:"accepted or needs_category or any; separate from personal seen state"`
	MinAmount      *int64  `json:"min_amount_cents,omitempty" jsonschema:"Inclusive signed integer cents"`
	MaxAmount      *int64  `json:"max_amount_cents,omitempty" jsonschema:"Inclusive signed integer cents"`
}

func (f mcpTransactionFilters) values(values url.Values) error {
	additions := map[string]string{}
	for key, value := range map[string]string{"date_from": f.DateFrom, "date_to": f.DateTo, "categorization": f.Categorization, "acceptance": f.Acceptance} {
		if value != "" {
			additions[key] = value
		}
	}
	if len(f.CategoryIDs) > 0 {
		ids := make([]string, len(f.CategoryIDs))
		for i, id := range f.CategoryIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		additions["category_ids"] = strings.Join(ids, ",")
	}
	if f.MinAmount != nil {
		additions["min_amount_cents"] = strconv.FormatInt(*f.MinAmount, 10)
	}
	if f.MaxAmount != nil {
		additions["max_amount_cents"] = strconv.FormatInt(*f.MaxAmount, 10)
	}
	for key, value := range additions {
		if legacy, ok := values[key]; ok && (len(legacy) != 1 || legacy[0] != value) {
			return fail(400, "Typed and legacy transaction filters disagree")
		}
		values.Set(key, value)
	}
	return nil
}

type mcpReviewQueueInput struct {
	mcpTransactionFilters
	Selector  string `json:"selector" jsonschema:"Required: needs_category or unseen; unseen is limited to entries needing categories"`
	AccountID int64  `json:"account_id,omitempty"`
	PeriodID  int64  `json:"period_id,omitempty"`
	Query     string `json:"q,omitempty"`
	Limit     *int   `json:"limit,omitempty" jsonschema:"1-100 entries; default 50"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"Opaque next_cursor returned with the same filters; restart without a cursor after stale-list or invalid-cursor errors"`
}
type mcpReviewCursor struct {
	User, Connection     int64
	Scope, Version, Date string
	ID                   int64
}

// Ephemeral encryption makes cursors opaque and authenticated, without storing
// credentials or adding persistent state. A server restart requires fresh paging.
func (a *App) reviewCursorCipher() (cipher.AEAD, error) {
	a.mcpCursorOnce.Do(func() {
		a.mcpCursorKey = make([]byte, 32)
		_, a.mcpCursorError = rand.Read(a.mcpCursorKey)
	})
	if a.mcpCursorError != nil {
		return nil, a.mcpCursorError
	}
	block, err := aes.NewCipher(a.mcpCursorKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (a *App) encodeReviewCursor(cursor mcpReviewCursor) (string, error) {
	c, err := a.reviewCursorCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(c.Seal(nonce, nonce, payload, []byte("finance-review-queue-v1"))), nil
}
func (a *App) decodeReviewCursor(value string) (mcpReviewCursor, error) {
	var cursor mcpReviewCursor
	invalid := fail(400, "Invalid review cursor. Start from the first page.")
	if len(value) > 2048 {
		return cursor, invalid
	}
	encoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor, invalid
	}
	c, err := a.reviewCursorCipher()
	if err != nil {
		return cursor, err
	}
	if len(encoded) < c.NonceSize()+c.Overhead() {
		return cursor, invalid
	}
	payload, err := c.Open(nil, encoded[:c.NonceSize()], encoded[c.NonceSize():], []byte("finance-review-queue-v1"))
	if err != nil {
		return cursor, invalid
	}
	if json.Unmarshal(payload, &cursor) != nil || cursor.ID <= 0 || !validDate(cursor.Date) {
		return cursor, invalid
	}
	return cursor, nil
}

func (a *App) mcpReviewQueueTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpReviewQueueInput) (*mcp.CallToolResult, any, error) {
	identity := ctx.Value(mcpKey).(mcpIdentity)
	if in.Selector != "needs_category" && in.Selector != "unseen" {
		return mcpFailure(fail(400, "Choose needs_category or unseen"))
	}
	limit := 50
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 1 || limit > 100 || in.AccountID < 0 || in.PeriodID < 0 {
		return mcpFailure(fail(400, "Choose positive account/period IDs and a limit from 1 to 100"))
	}
	values := url.Values{}
	if err := in.mcpTransactionFilters.values(values); err != nil {
		return mcpFailure(err)
	}
	if in.Acceptance == "accepted" || in.Categorization == "categorized" {
		return mcpFailure(fail(400, "Review queues require entries needing categories"))
	}
	values.Set("acceptance", "needs_category")
	values.Set("categorization", "uncategorized")
	if in.Selector == "unseen" {
		values.Set("seen", "0")
	}
	if in.AccountID > 0 {
		values.Set("account", strconv.FormatInt(in.AccountID, 10))
	}
	if in.PeriodID > 0 {
		values.Set("period", strconv.FormatInt(in.PeriodID, 10))
	}
	if in.Query != "" {
		values.Set("q", in.Query)
	}
	scope := hash(values.Encode() + fmt.Sprintf("|%s|%d", in.Selector, limit))
	var cursor mcpReviewCursor
	if in.Cursor != "" {
		var err error
		cursor, err = a.decodeReviewCursor(in.Cursor)
		if err != nil {
			return mcpFailure(err)
		}
		if cursor.User != identity.User.ID || cursor.Connection != identity.TokenID || cursor.Scope != scope {
			return mcpFailure(fail(400, "Review cursor does not match this user, connection or filters"))
		}
	}
	r, _ := http.NewRequestWithContext(context.WithValue(ctx, authKey, authContext{User: identity.User}), "GET", "http://internal/transactions?"+values.Encode(), nil)
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	query, args, err := a.transactionQuery(tx, r, identity.User)
	if err != nil {
		return mcpFailure(err)
	}
	version, total, err := transactionListState(tx, query, args, identity.User)
	if err != nil {
		return mcpFailure(err)
	}
	if in.Cursor != "" && cursor.Version != version {
		return mcpFailure(fail(409, "This list changed. Start from the first page."))
	}
	// Reuse the exact authorized shared query scope, selecting only compact fields.
	query = "SELECT t.id,t.version,t.date,t.description,t.amount_cents,t.account_id,t.review_state,t.is_transfer," + transactionSeenSQL(identity.User) + " seen" + query[strings.Index(query, " FROM transactions"):]
	if in.Cursor != "" {
		query += " AND (t.date<? OR (t.date=? AND t.id<?))"
		args = append(args, cursor.Date, cursor.Date, cursor.ID)
	}
	items, err := data(tx, query+" ORDER BY t.date DESC,t.id DESC LIMIT ?", append(args, limit+1)...)
	if err != nil {
		return mcpFailure(err)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	for _, item := range items {
		allocations, err := data(tx, "SELECT l.id,l.category_id,l.amount_cents,c.name category_name,c.kind FROM allocations l LEFT JOIN categories c ON c.id=l.category_id WHERE l.transaction_id=? ORDER BY l.id", num(item["id"]))
		if err != nil {
			return mcpFailure(err)
		}
		item["allocations"] = allocations
		item["currency"] = "ZAR"
	}
	next := ""
	if more {
		last := items[len(items)-1]
		next, err = a.encodeReviewCursor(mcpReviewCursor{User: identity.User.ID, Connection: identity.TokenID, Scope: scope, Version: version, Date: last["date"].(string), ID: num(last["id"])})
		if err != nil {
			return mcpFailure(err)
		}
	}
	return mcpResult(map[string]any{"items": mcpSafe(items), "total": total, "more": more, "next_cursor": next, "list_version": version}), nil, nil
}
