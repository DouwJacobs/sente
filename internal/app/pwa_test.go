package app

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"testing"
)

func syntheticIcon(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 192, 192))
	for y := 0; y < 192; y++ {
		for x := 0; x < 192; x++ {
			img.Set(x, y, color.RGBA{200, 30, 40, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
func publicRequest(e *testEnv, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}
func TestPWAIdentityPrivacyPermissionsConcurrency(t *testing.T) {
	e := setup(t)
	manifest := publicRequest(e, "/manifest.webmanifest")
	status(t, manifest, 200)
	if manifest.Header().Get("Content-Type") != "application/manifest+json" || manifest.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(manifest.Header())
	}
	var got map[string]any
	json.Unmarshal(manifest.Body.Bytes(), &got)
	if got["name"] != "Sente" {
		t.Fatal(got)
	}
	status(t, e.req(t, 1, "/api/branding", "PUT", map[string]any{"display_name": "Private family", "version": 1, "logo": syntheticIcon(t)}), 200)
	if bytes.Contains(publicRequest(e, "/manifest.webmanifest").Body.Bytes(), []byte("Private family")) {
		t.Fatal("private name published without opt-in")
	}
	body := map[string]any{"name": "Public app", "logo": "", "use_branding_name": true, "use_branding_logo": true, "version": 1}
	status(t, e.req(t, 2, "/api/pwa", "PUT", body), 403)
	status(t, e.req(t, 1, "/api/pwa", "PUT", body), 200)
	status(t, e.req(t, 1, "/api/pwa", "PUT", body), 409)
	if !bytes.Contains(publicRequest(e, "/manifest.webmanifest").Body.Bytes(), []byte("Private family")) {
		t.Fatal("explicit name not inherited")
	}
	icon := publicRequest(e, "/pwa/icon/512/any.png")
	status(t, icon, 200)
	img, err := png.Decode(bytes.NewReader(icon.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 512 {
		t.Fatal(err)
	}
	red, _, _, _ := img.At(256, 256).RGBA()
	if red != 200*257 {
		t.Fatal("branding icon not inherited")
	}
	body["version"] = 2
	body["use_branding_name"] = false
	body["use_branding_logo"] = false
	body["logo"] = syntheticIcon(t)
	status(t, e.req(t, 1, "/api/pwa", "PUT", body), 200)
	if !bytes.Contains(publicRequest(e, "/manifest.webmanifest").Body.Bytes(), []byte("Public app")) {
		t.Fatal("custom name not applied")
	}
	status(t, e.req(t, 1, "/api/branding", "PUT", map[string]any{"display_name": "New family", "version": 2}), 200)
	b, _ := e.a.readPWA()
	if b.Name != "Public app" || b.Logo == "" {
		t.Fatal("custom identity overwritten by branding")
	}
	for _, size := range []string{"192", "512", "180"} {
		status(t, publicRequest(e, "/pwa/icon/"+size+"/maskable.png"), 200)
	}
	status(t, publicRequest(e, "/pwa/icon/2048/any.png"), 404)
	status(t, publicRequest(e, "/api/pwa"), 401)
	var details string
	e.a.DB.QueryRow("SELECT details FROM audit WHERE entity='workspace_branding' ORDER BY id DESC LIMIT 1").Scan(&details)
	if bytes.Contains([]byte(details), []byte("base64")) {
		t.Fatal("logo content audited")
	}
}
func TestPWAIconValidationAndRevokedActor(t *testing.T) {
	for _, value := range []string{"https://remote.invalid/icon.png", "data:image/svg+xml;base64,AAAA", "data:image/png;base64,bm90IGEgcG5n"} {
		if _, err := normalizeIcon(value); err == nil {
			t.Fatal(value)
		}
	}
	for _, revoke := range []string{"DELETE FROM sessions WHERE user_id=1", "UPDATE users SET admin=0 WHERE id=1", "UPDATE sessions SET csrf='rotated' WHERE user_id=1"} {
		e := setup(t)
		w := admittedMutation(t, e, "/api/pwa", "PUT", map[string]any{"name": "New app", "version": 1}, revoke)
		if w.Code != 401 && w.Code != 403 {
			t.Fatal(w.Code)
		}
		if queryInt(e.a.DB, "SELECT version FROM app_identity") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='pwa_identity'") != 0 {
			t.Fatal("revoked actor mutated identity")
		}
	}
}
func TestPWAMigrationRollbackRetryPreservation(t *testing.T) {
	e := setup(t)
	migrationExec(t, e.a.DB, "DROP TABLE app_identity; DELETE FROM migrations WHERE version=28")
	protected := []string{"SELECT * FROM transactions", "SELECT * FROM allocations", "SELECT * FROM grants", "SELECT * FROM sessions", "SELECT * FROM workspace_branding", "SELECT * FROM audit", "SELECT * FROM mcp_tokens"}
	before := migrationSnapshot(t, e.a.DB, protected...)
	steps := append([]schemaMigration(nil), schemaMigrations...)
	steps[len(steps)-1].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := migratePWA(tx, origin); err != nil {
			return err
		}
		return errors.New("synthetic failure")
	}
	if runSchemaMigrations(e.a.DB, steps, schemaVersion) == nil {
		t.Fatal("expected rollback")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM sqlite_master WHERE name='app_identity'") != 0 || queryInt(e.a.DB, "SELECT MAX(version) FROM migrations") != 27 {
		t.Fatal("partial upgrade")
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, e.a.DB, protected...) {
		t.Fatal("protected data changed")
	}
	migrationExec(t, e.a.DB, "UPDATE app_identity SET pwa_name='Saved app',version=7")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	b, err := e.a.readPWA()
	if err != nil || b.Name != "Saved app" || b.Version != 7 {
		t.Fatal("restart lost identity", err)
	}
}

func TestPWAMaskableTransparencyAndBounds(t *testing.T) {
	transparent := image.NewRGBA(image.Rect(0, 0, 192, 192))
	icon := iconImage(transparent, 512, true)
	for _, point := range []image.Point{{0, 0}, {256, 256}, {511, 511}} {
		_, _, _, alpha := icon.At(point.X, point.Y).RGBA()
		if alpha != 65535 {
			t.Fatal("maskable icon must be opaque")
		}
	}
	for _, bounds := range []image.Rectangle{image.Rect(0, 0, 100, 100), image.Rect(0, 0, 192, 193), image.Rect(0, 0, 1025, 1025)} {
		var b bytes.Buffer
		if err := png.Encode(&b, image.NewRGBA(bounds)); err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeIcon("data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())); err == nil {
			t.Fatal("invalid dimensions accepted", bounds)
		}
	}
}
