package statements

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"finance-tracker/internal/classification"
)

func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }

// Fingerprint uses immutable source fields, never page positions or live references.
func Fingerprint(row SourceRow) string {
	return hash(row.Date + "|" + strconv.FormatInt(row.Amount, 10) + "|" + classification.Normalize(row.Description))
}
