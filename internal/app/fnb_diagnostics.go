package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

func safeFNBDiagnostics(input map[string]int) map[string]int {
	out := map[string]int{}
	for _, key := range []string{"name_nodes", "number_nodes", "ledger_nodes", "matched_rows", "missing_rows", "invalid_amounts", "unsupported_entries", "hidden_rows", "blank_balances", "placeholder_balances", "sign_suffix", "parenthesized", "comma_decimal", "other_format", "evaluation_failed", "trailing_minus", "currency_suffix", "unknown_text", "repeated_decimal", "balance_label", "unavailable_text", "loading_text", "logout_clicked", "logout_confirmed", "logout_unconfirmed", "balance_failure", "reward_entries", "non_zar_entries", "refresh_phase", "refresh_timed_out", "profile_cleanup_failed", "connector_process_failed", "connector_interop_failed", "connector_response_invalid", "masked_credit_resolved", "masked_credit_failed", "masked_credit_navigation", "masked_credit_detail_masked", "masked_credit_detail_invalid", "masked_credit_number_mismatch", "masked_credit_duplicate_identity", "masked_credit_non_zar", "transaction_rows", "transaction_headers", "transaction_invalid_rows", "transaction_accounts", "transaction_failure", "transaction_identity_fields", "transaction_type_fields", "transaction_navigation", "transaction_accounts_requested", "transaction_failed_account_position", "transaction_unsupported_type", "transaction_unsupported_currency", "transaction_fee_rows", "transaction_successful_controls", "transaction_pending_controls", "transaction_selected_successful"} {
		if value, ok := input[key]; ok && value >= 0 && value <= 10000 {
			out[key] = value
		}
	}
	return out
}
func fnbDiagnosticJSON(input map[string]int) string {
	value, _ := json.Marshal(safeFNBDiagnostics(input))
	return string(value)
}
func (a *App) fnbDebug(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Debug   bool  `json:"debug_browser"`
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE fnb_connections SET debug_browser=?,version=version+1 WHERE user_id=? AND version=? AND state!='refreshing'", b.Debug, u.ID, b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "browser_mode_updated", map[string]bool{"debug_browser": b.Debug})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
