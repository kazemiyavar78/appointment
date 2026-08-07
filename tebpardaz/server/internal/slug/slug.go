package slug

import (
	"strings"
	"unicode"
)

// Make builds a URL-safe path segment from a display name.
// Inputs: raw name (Persian or Latin).
// Output: hyphenated slug; "doctor" when empty after cleanup.
func Make(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "doctor"
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/':
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "doctor"
	}
	return out
}
