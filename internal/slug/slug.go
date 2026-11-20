// Package slug turns text into URL-safe identifiers.
package slug

import (
	"strings"
	"unicode"
)

// Make lower-cases s and joins its letters and digits with single dashes ("Team wiki" -> "team-wiki").
// Non-ASCII letters are kept, so headings in any language still get readable ids.
func Make(s string) string {
	var b strings.Builder
	dash := true // swallow leading separators
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
