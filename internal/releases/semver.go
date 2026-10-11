// Package releases checks public release metadata without application or user context.
package releases

import (
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var numericPattern = regexp.MustCompile(`^[0-9]+$`)

type version struct {
	core     []string
	pre      []string
	metadata string
}

func parse(value string) (version, bool) {
	m := versionPattern.FindStringSubmatch(value)
	if m == nil || len(value) > 200 {
		return version{}, false
	}
	v := version{core: m[1:4], metadata: m[5]}
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
		for _, s := range v.pre {
			if numericPattern.MatchString(s) && len(s) > 1 && s[0] == '0' {
				return version{}, false
			}
		}
	}
	return v, true
}
func numericCompare(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func compare(a, b version) int {
	for i := range a.core {
		if c := numericCompare(a.core[i], b.core[i]); c != 0 {
			return c
		}
	}
	if len(a.pre) == 0 && len(b.pre) == 0 {
		return 0
	}
	if len(a.pre) == 0 {
		return 1
	}
	if len(b.pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, yn := numericPattern.MatchString(x), numericPattern.MatchString(y)
		if xn && !yn {
			return -1
		}
		if !xn && yn {
			return 1
		}
		c := strings.Compare(x, y)
		if xn && yn {
			c = numericCompare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}
func channel(v version) string {
	if v.metadata != "" {
		return ""
	}
	if len(v.pre) == 0 {
		return "stable"
	}
	if v.pre[0] == "beta" {
		return "beta"
	}
	return ""
}
