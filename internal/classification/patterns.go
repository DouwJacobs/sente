package classification

import (
	"regexp"
	"strings"
)

var reOrWord = regexp.MustCompile(`(?i)\s+or\s+`)

func buildPatternRegex(pattern string) *regexp.Regexp {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") || (strings.HasPrefix(pattern, "(") && strings.HasSuffix(pattern, ")")) {
		if re, err := regexp.Compile("(?i)" + pattern); err == nil {
			return re
		}
	}
	var tokens []string
	if strings.Contains(pattern, "|") {
		tokens = strings.Split(pattern, "|")
	} else if reOrWord.MatchString(pattern) {
		tokens = reOrWord.Split(pattern, -1)
	} else if strings.Contains(pattern, ",") {
		tokens = strings.Split(pattern, ",")
	}
	if len(tokens) > 1 {
		var parts []string
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				parts = append(parts, regexp.QuoteMeta(tok))
			}
		}
		if len(parts) > 0 {
			if re, err := regexp.Compile("(?i)(?:" + strings.Join(parts, "|") + ")"); err == nil {
				return re
			}
		}
	}
	if strings.ContainsAny(pattern, `.*+?^$[](){}\`) {
		if re, err := regexp.Compile("(?i)" + pattern); err == nil {
			return re
		}
	}
	return nil
}

func MatchPattern(description, pattern string) bool {
	normDesc := Normalize(description)
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if re := buildPatternRegex(pattern); re != nil {
		if re.MatchString(description) || re.MatchString(normDesc) {
			return true
		}
	}
	return strings.Contains(normDesc, Normalize(pattern))
}
