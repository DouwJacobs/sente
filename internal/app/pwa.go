package app

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type pwaIdentity struct {
	Name    string `json:"name"`
	Logo    string `json:"logo"`
	UseName bool   `json:"use_branding_name"`
	UseLogo bool   `json:"use_branding_logo"`
	Version int64  `json:"version"`
}

func (a *App) readPWA() (pwaIdentity, error) {
	var b pwaIdentity
	err := a.DB.QueryRow("SELECT pwa_name,pwa_logo,use_branding_name,use_branding_logo,version FROM app_identity WHERE id=1").Scan(&b.Name, &b.Logo, &b.UseName, &b.UseLogo, &b.Version)
	return b, err
}
func (a *App) pwaSettings(w http.ResponseWriter, r *http.Request) error {
	b, err := a.readPWA()
	if err != nil {
		return err
	}
	send(w, b)
	return nil
}
func validPublicName(name string) bool {
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 60 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (a *App) updatePWA(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	var b pwaIdentity
	if err := decode(r, &b); err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if !validPublicName(b.Name) {
		return fail(400, "Use a PWA name of 2–60 characters without control characters")
	}
	logo, err := normalizeIcon(b.Logo)
	if err != nil {
		return err
	}
	b.Logo = logo
	err = a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		result, err := tx.Exec("UPDATE app_identity SET pwa_name=?,pwa_logo=?,use_branding_name=?,use_branding_logo=?,version=version+1 WHERE id=1 AND version=?", b.Name, b.Logo, b.UseName, b.UseLogo, b.Version)
		if err != nil {
			return err
		}
		if err := affected(result); err != nil {
			return err
		}
		return audit(tx, u, nil, "pwa_identity", 1, "updated", map[string]any{"name": b.Name, "use_branding_name": b.UseName, "use_branding_logo": b.UseLogo, "version": b.Version})
	})
	if err != nil {
		return err
	}
	return a.pwaSettings(w, r)
}

// Only bounded PNG raster input; decode/re-encode strips metadata and active content.
func normalizeIcon(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	invalid := func() (string, error) {
		return "", fail(400, "Use a square PNG icon, 192–1024 pixels and at most 256 KiB")
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") || len(value) > 350000 {
		return invalid()
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/png;base64,"))
	if err != nil || len(raw) > 256*1024 {
		return invalid()
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != cfg.Height || cfg.Width < 192 || cfg.Width > 1024 {
		return invalid()
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return invalid()
	}
	var out bytes.Buffer
	if err := png.Encode(&out, iconImage(img, 512, false)); err != nil {
		return "", err
	}
	if out.Len() > 256*1024 {
		return invalid()
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
func defaultIcon() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{15, 118, 110, 255}}, image.Point{}, draw.Src)
	for _, r := range []image.Rectangle{image.Rect(5, 13, 8, 19), image.Rect(10, 9, 13, 19), image.Rect(15, 5, 18, 19)} {
		draw.Draw(img, r, &image.Uniform{color.White}, image.Point{}, draw.Src)
	}
	return img
}
func iconImage(src image.Image, size int, maskable bool) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.RGBA{15, 118, 110, 255}}, image.Point{}, draw.Src)
	margin := 0
	if maskable {
		margin = size / 5
	}
	width := size - 2*margin
	b := src.Bounds()
	scaled := image.NewRGBA(out.Bounds())
	for y := margin; y < size-margin; y++ {
		for x := margin; x < size-margin; x++ {
			c := src.At(b.Min.X+(x-margin)*b.Dx()/width, b.Min.Y+(y-margin)*b.Dy()/width)
			scaled.Set(x, y, c)
		}
	}
	draw.Draw(out, out.Bounds(), scaled, image.Point{}, draw.Over)
	return out
}
func (a *App) publicManifest(w http.ResponseWriter, r *http.Request) error {
	var name string
	var version, brandingVersion int64
	err := a.DB.QueryRow(`SELECT CASE WHEN use_branding_name=1 THEN display_name ELSE pwa_name END,app_identity.version,workspace_branding.version FROM app_identity JOIN workspace_branding ON workspace_branding.id=app_identity.id WHERE app_identity.id=1`).Scan(&name, &version, &brandingVersion)
	if err != nil {
		return err
	}
	icons := []map[string]string{}
	for _, size := range []int{192, 512} {
		for _, purpose := range []string{"any", "maskable"} {
			icons = append(icons, map[string]string{"src": fmt.Sprintf("/pwa/icon/%d/%s.png?v=%d-%d", size, purpose, version, brandingVersion), "sizes": fmt.Sprintf("%dx%d", size, size), "type": "image/png", "purpose": purpose})
		}
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	return json.NewEncoder(w).Encode(map[string]any{"id": "/", "name": name, "short_name": name, "start_url": "/", "scope": "/", "display": "standalone", "theme_color": "#0f766e", "background_color": "#f5f7f6", "icons": icons})
}
func (a *App) publicIcon(w http.ResponseWriter, r *http.Request) error {
	size := 64
	maskable := false
	var value string
	if r.URL.Path == "/branding/icon.png" {
		if err := a.DB.QueryRow("SELECT logo FROM app_identity WHERE id=1").Scan(&value); err != nil {
			return err
		}
	} else {
		n, err := strconv.Atoi(r.PathValue("size"))
		if err != nil || (n != 192 && n != 512 && n != 180) {
			return fail(404, "Icon not found")
		}
		size = n
		purpose := r.PathValue("purpose")
		if purpose != "any.png" && purpose != "maskable.png" {
			return fail(404, "Icon not found")
		}
		maskable = purpose == "maskable.png"
		if err := a.DB.QueryRow("SELECT CASE WHEN use_branding_logo=1 THEN logo ELSE pwa_logo END FROM app_identity WHERE id=1").Scan(&value); err != nil {
			return err
		}
	}
	var img image.Image = defaultIcon()
	if value != "" {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/png;base64,"))
		if err != nil {
			return err
		}
		img, err = png.Decode(bytes.NewReader(raw))
		if err != nil {
			return err
		}
	}
	w.Header().Set("Content-Type", "image/png")
	return png.Encode(w, iconImage(img, size, maskable))
}
