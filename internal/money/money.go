// Package money parses signed decimal amounts into exact integer minor units.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var decimalRE = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]{1,2})?$`)

func Cents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !decimalRE.MatchString(s) {
		return 0, fmt.Errorf("invalid money: use a signed decimal with at most two fractional digits")
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 9000000000000 {
		return 0, fmt.Errorf("amount is too large")
	}
	fraction := int64(0)
	if len(parts) > 1 {
		v := parts[1]
		if len(v) == 1 {
			v += "0"
		}
		fraction, _ = strconv.ParseInt(v, 10, 64)
	}
	v := whole*100 + fraction
	if negative {
		v = -v
	}
	return v, nil
}
