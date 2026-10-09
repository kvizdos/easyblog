package builder

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// asciiFold maps common accented Latin letters to ASCII. Anything not listed
// and not ASCII is treated as a separator by slugify.
var asciiFold = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae",
	'ç': "c", 'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
}

// slugify lowercases s, folds accents to ASCII, turns every run of
// non-alphanumerics into a single "-", and trims leading/trailing dashes.
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case asciiFold[r] != "":
			b.WriteString(asciiFold[r])
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

var dateLayouts = []string{"01/02/2006", "2006-01-02", time.RFC3339}

// parseDate accepts "01/02/2006", "2006-01-02", RFC 3339, or a YAML timestamp.
func parseDate(v any) (time.Time, error) {
	switch d := v.(type) {
	case time.Time:
		return d, nil
	case string:
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, strings.TrimSpace(d)); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("unrecognized date %q (use MM/DD/YYYY or YYYY-MM-DD)", d)
	default:
		return time.Time{}, fmt.Errorf("date must be a string, got %T", v)
	}
}
