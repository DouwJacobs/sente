package app

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	"image/png"
	"strings"
)

func nullableAccount(account int64) any {
	if account == 0 {
		return nil
	}
	return account
}
func merchantScopeAccess(tx *sql.Tx, a *App, u User, account int64) bool {
	if account == 0 {
		return u.Member
	}
	return ruleAccess(tx, a, u, account)
}

// Decode and re-encode raster uploads, removing embedded metadata and external references.
func merchantLogo(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > 180000 {
		return "", fail(400, "Merchant logo must be at most 128 KB")
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 || (parts[0] != "data:image/png;base64" && parts[0] != "data:image/jpeg;base64") {
		return "", fail(400, "Choose a PNG or JPEG merchant logo")
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(parts[1])
	if err != nil || len(payload) > 131072 {
		return "", fail(400, "Choose a valid logo of at most 128 KB")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 512 || cfg.Height > 512 {
		return "", fail(400, "Merchant logo must be a valid image no larger than 512 × 512")
	}
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return "", fail(400, "Choose a valid merchant logo")
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return "", err
	}
	if out.Len() > 131072 {
		return "", fail(400, "Merchant logo must be at most 128 KB")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
func updateMerchantLogo(tx *sql.Tx, a *App, u User, id, version int64, raw string) error {
	var account sql.NullInt64
	if tx.QueryRow("SELECT account_id FROM merchants WHERE id=?", id).Scan(&account) != nil {
		return fail(404, "Merchant not found")
	}
	if !merchantScopeAccess(tx, a, u, account.Int64) {
		return fail(403, "Merchant editor access required")
	}
	logo, err := merchantLogo(raw)
	if err != nil {
		return err
	}
	res, err := tx.Exec("UPDATE merchants SET logo_data=?,version=version+1 WHERE id=? AND version=?", logo, id, version)
	if err != nil {
		return err
	}
	if err = affected(res); err != nil {
		return err
	}
	return audit(tx, u, nullableAccount(account.Int64), "merchant", id, "logo_updated", map[string]any{"has_logo": logo != ""})
}
