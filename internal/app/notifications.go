package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const notificationRetention = 90 * 24 * time.Hour
const notificationReceiptRetention = 180 * 24 * time.Hour
const notificationInboxLimit = 500
const notificationReceiptLimit = 5000

var notificationTypes = []string{"system", "budget_threshold", "budget_projection", "budget_overspend", "unusual_spending", "recurring_payment"}

// Producers calculate conditions separately and supply a stable occurrence/key.
// Financial events require an explicit source and every contributing account.
// This contract is internal: no HTTP or MCP endpoint accepts arbitrary events.
type notificationEvent struct {
	RecipientID int64
	Type        string
	Severity    string
	Title       string
	Message     string
	SourceKind  string
	SourceID    int64
	AccountIDs  []int64
	DedupeKey   string
	OccurredAt  time.Time
	Dismissible bool
}

type notificationDelivery struct {
	ID     int64
	Status string // delivered, duplicate, disabled, expired
}

// Adapters persist channel work using the caller's transaction. A future push
// adapter must enqueue an outbox job here, never make a network request inside
// the write. In-app record creation and retry receipts therefore commit together.
type notificationAdapter interface {
	channel() string
	deliver(*sql.Tx, notificationEvent, int64, int64) (int64, error)
}
type inAppNotificationAdapter struct{}

func (inAppNotificationAdapter) channel() string { return "in_app" }
func (inAppNotificationAdapter) deliver(tx *sql.Tx, e notificationEvent, receipt, now int64) (int64, error) {
	res, err := tx.Exec(`INSERT INTO notifications(receipt_id,user_id,type,severity,title,message,source_kind,source_id,occurred_at,created_at,dismissible) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, receipt, e.RecipientID, e.Type, e.Severity, e.Title, e.Message, e.SourceKind, e.SourceID, e.OccurredAt.Unix(), now, e.Dismissible)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, account := range e.AccountIDs {
		if _, err := tx.Exec("INSERT INTO notification_accounts VALUES(?,?)", id, account); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func knownNotificationType(kind string) bool {
	for _, candidate := range notificationTypes {
		if kind == candidate {
			return true
		}
	}
	return false
}
func validateNotificationEvent(e *notificationEvent, now time.Time) error {
	if e.RecipientID <= 0 || !knownNotificationType(e.Type) || (e.Severity != "info" && e.Severity != "warning" && e.Severity != "critical") {
		return fail(400, "Invalid notification event")
	}
	for _, field := range []struct {
		value string
		limit int
	}{{e.Title, 120}, {e.Message, 1000}, {e.DedupeKey, 200}} {
		if !utf8.ValidString(field.value) || strings.TrimSpace(field.value) == "" || utf8.RuneCountInString(field.value) > field.limit || strings.ContainsRune(field.value, 0) {
			return fail(400, "Invalid notification content")
		}
	}
	if e.OccurredAt.IsZero() || e.OccurredAt.After(now) || e.OccurredAt.Unix() <= 0 {
		return fail(400, "Invalid notification occurrence time")
	}
	e.OccurredAt = e.OccurredAt.UTC().Truncate(time.Second)
	if len(e.AccountIDs) > 1000 {
		return fail(400, "Too many notification account dependencies")
	}
	// Canonical scope makes retries independent of producer query order.
	e.AccountIDs = append([]int64{}, e.AccountIDs...)
	sort.Slice(e.AccountIDs, func(i, j int) bool { return e.AccountIDs[i] < e.AccountIDs[j] })
	accounts := e.AccountIDs[:0]
	for _, id := range e.AccountIDs {
		if id <= 0 {
			return fail(400, "Invalid notification account dependency")
		}
		if len(accounts) == 0 || accounts[len(accounts)-1] != id {
			accounts = append(accounts, id)
		}
	}
	e.AccountIDs = accounts
	if (e.Type == "budget_threshold" || e.Type == "budget_projection" || e.Type == "budget_overspend") && e.SourceKind != "budget" {
		return fail(400, "Budget notifications require household budget scope")
	}
	if (e.Type == "unusual_spending" || e.Type == "recurring_payment") && e.SourceKind != "account" && e.SourceKind != "transaction" {
		return fail(400, "Spending notifications require account or transaction scope")
	}
	switch e.SourceKind {
	case "system":
		if e.Type != "system" || e.SourceID != 0 || len(e.AccountIDs) != 0 {
			return fail(400, "System notifications cannot carry financial sources")
		}
	case "account", "transaction", "budget":
		if e.Type == "system" || e.SourceID <= 0 || len(e.AccountIDs) == 0 {
			return fail(400, "Financial notifications require a source and account scope")
		}
	default:
		return fail(400, "Invalid notification source")
	}
	return nil
}

func (a *App) authorizeNotification(q queryer, e notificationEvent) error {
	var recipient User
	err := q.QueryRow("SELECT id,username,admin,budget_member FROM users WHERE id=? AND disabled=0 AND deleted_at IS NULL", e.RecipientID).Scan(&recipient.ID, &recipient.Username, &recipient.Admin, &recipient.Member)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(403, "Notification recipient is unavailable")
	}
	if err != nil {
		return err
	}
	for _, id := range e.AccountIDs {
		var hidden, household bool
		if err := q.QueryRow("SELECT sync_hidden,household FROM accounts WHERE id=?", id).Scan(&hidden, &household); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fail(403, "Notification scope is unavailable")
			}
			return err
		}
		if hidden || !a.can(q, recipient, id, false) || (e.SourceKind == "budget" && !household) {
			return fail(403, "Notification scope is unavailable")
		}
	}
	if e.SourceKind == "system" {
		return nil
	}
	var sourceAccount int64
	switch e.SourceKind {
	case "account":
		sourceAccount = e.SourceID
	case "transaction":
		if err := q.QueryRow("SELECT account_id FROM transactions WHERE id=?", e.SourceID).Scan(&sourceAccount); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fail(403, "Notification source is unavailable")
			}
			return err
		}
	case "budget":
		if !recipient.Member {
			return fail(403, "Household membership required for budget notifications")
		}
		var exists int
		if err := q.QueryRow("SELECT COUNT(*) FROM periods WHERE id=?", e.SourceID).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return fail(403, "Notification source is unavailable")
		}
		return nil
	}
	for _, id := range e.AccountIDs {
		if id == sourceAccount {
			return nil
		}
	}
	return fail(403, "Notification source is outside its privacy scope")
}

// Use notifyTx inside an existing authorized financial write to make emission
// atomic with its trigger. Standalone evaluations use notify, which owns a write.
func (a *App) notify(e notificationEvent) (notificationDelivery, error) {
	var result notificationDelivery
	err := a.write(func(tx *sql.Tx) error { var err error; result, err = a.notifyTx(tx, e, time.Now()); return err })
	return result, err
}
func (a *App) notifyTx(tx *sql.Tx, e notificationEvent, now time.Time) (notificationDelivery, error) {
	if err := validateNotificationEvent(&e, now); err != nil {
		return notificationDelivery{}, err
	}
	if err := a.authorizeNotification(tx, e); err != nil {
		return notificationDelivery{}, err
	}
	if !e.OccurredAt.After(now.Add(-notificationRetention)) {
		return notificationDelivery{Status: "expired"}, nil
	}
	if err := pruneNotificationsTx(tx, now); err != nil {
		return notificationDelivery{}, err
	}
	adapter := inAppNotificationAdapter{}
	result, err := routeNotificationTx(tx, e, now, adapter)
	if err == nil && result.Status == "delivered" {
		err = enqueueNotificationPushTx(tx, e, result.ID, now)
	}
	return result, err
}
func routeNotificationTx(tx *sql.Tx, e notificationEvent, now time.Time, adapter notificationAdapter) (notificationDelivery, error) {
	// Only the in-app adapter is enabled. Additional channels require independent
	// durable attempt state, explicit consent and a dispatcher (#39).
	if adapter.channel() != "in_app" {
		return notificationDelivery{}, fail(400, "Notification channel is unavailable")
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return notificationDelivery{}, err
	}
	digest := hash(string(encoded))
	key := hash(e.DedupeKey)
	var receipt int64
	var prior string
	err = tx.QueryRow("SELECT id,event_hash FROM notification_receipts WHERE user_id=? AND type=? AND dedupe_hash=?", e.RecipientID, e.Type, key).Scan(&receipt, &prior)
	if err == nil {
		if prior != digest {
			return notificationDelivery{}, fail(409, "Notification retry differs from its original event")
		}
		var id int64
		err = tx.QueryRow("SELECT id FROM notifications WHERE receipt_id=?", receipt).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return notificationDelivery{}, err
		}
		return notificationDelivery{ID: id, Status: "duplicate"}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return notificationDelivery{}, err
	}
	var enabled bool
	err = tx.QueryRow("SELECT enabled FROM notification_preferences WHERE user_id=? AND type=? AND channel=?", e.RecipientID, e.Type, adapter.channel()).Scan(&enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return notificationDelivery{}, err
	}
	inAppEnabled := err != nil || enabled
	if !inAppEnabled && queryInt(tx, "SELECT COUNT(*) FROM notification_push_preferences WHERE user_id=? AND type=? AND enabled=1", e.RecipientID, e.Type) == 0 {
		return notificationDelivery{Status: "disabled"}, nil
	}
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM notification_receipts WHERE user_id=?", e.RecipientID).Scan(&count); err != nil {
		return notificationDelivery{}, err
	}
	if count >= notificationReceiptLimit {
		return notificationDelivery{}, fail(429, "Notification retention capacity reached")
	}
	res, err := tx.Exec("INSERT INTO notification_receipts(user_id,type,dedupe_hash,event_hash,created_at) VALUES(?,?,?,?,?)", e.RecipientID, e.Type, key, digest, now.Unix())
	if err != nil {
		return notificationDelivery{}, err
	}
	receipt, err = res.LastInsertId()
	if err != nil {
		return notificationDelivery{}, err
	}
	id, err := adapter.deliver(tx, e, receipt, now.Unix())
	if err != nil {
		return notificationDelivery{}, err
	}
	if !inAppEnabled {
		if _, err := tx.Exec("UPDATE notifications SET in_app_enabled=0 WHERE id=?", id); err != nil {
			return notificationDelivery{}, err
		}
	}
	// Evict old payloads while retaining receipts so retries cannot resurrect them.
	_, err = tx.Exec("DELETE FROM notifications WHERE user_id=? AND id NOT IN (SELECT id FROM notifications WHERE user_id=? ORDER BY id DESC LIMIT ?)", e.RecipientID, e.RecipientID, notificationInboxLimit)
	return notificationDelivery{ID: id, Status: "delivered"}, err
}
func pruneNotificationsTx(tx *sql.Tx, now time.Time) error {
	statements := []struct {
		sql string
		arg any
	}{
		{"DELETE FROM notifications WHERE created_at<=?", now.Add(-notificationRetention).Unix()},
		{"DELETE FROM notification_receipts WHERE created_at<=?", now.Add(-notificationReceiptRetention).Unix()},
		{"DELETE FROM notification_receipts WHERE user_id IN (SELECT id FROM users WHERE deleted_at IS NOT NULL)", nil},
	}
	for _, s := range statements {
		var err error
		if s.arg == nil {
			_, err = tx.Exec(s.sql)
		} else {
			_, err = tx.Exec(s.sql, s.arg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
