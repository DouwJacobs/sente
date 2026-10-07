package statements

import (
	"bytes"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

func text(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(b) {
		v, _ := charmap.Windows1252.NewDecoder().Bytes(b)
		return string(v)
	}
	return string(b)
}

func dateValue(s string) string {
	s = strings.TrimSpace(s)
	for _, format := range []string{"2006/01/02", "2006-01-02", "20060102"} {
		if t, err := time.Parse(format, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if regexp.MustCompile(`^[0-9]{8}([0-9]{6}(\.[0-9]+)?(\[[-+][0-9]+(:[A-Z]+)?\])?)?$`).MatchString(s) {
		if t, err := time.Parse("20060102", s[:8]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}
func ParseFile(f InputFile) ParsedFile {
	p := ParsedFile{Name: f.Name, Hash: hash(string(f.Content)), Rows: []SourceRow{}}
	switch strings.ToLower(path.Ext(f.Name)) {
	case ".csv":
		p.Format = "csv"
		parseCSV(&p, text(f.Content))
	case ".ofx":
		p.Format = "ofx"
		parseOFX(&p, text(f.Content))
	default:
		p.Error = "Unsupported file format"
	}
	if p.AccountID == "" && p.Error == "" {
		p.Error = "Bank account identity is missing"
	}
	if p.Currency != "ZAR" && p.Error == "" {
		p.Error = "Only ZAR imports are supported"
	}
	if len(p.Rows) == 0 && p.Error == "" {
		p.Error = "No transactions found"
	}
	return p
}
