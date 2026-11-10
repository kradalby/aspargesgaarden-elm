package site

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// maxSlugLen keeps addresses readable; a title longer than this is cut at a
// word boundary.
const maxSlugLen = 80

var slugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedSlugs are the site's fixed top-level addresses, old and new. Event
// slugs may one day get evergreen top-level addresses of their own, as
// /julemarked does, so a slug must never shadow a fixed route.
var reservedSlugs = map[string]bool{
	"about":         true,
	"admin":         true,
	"api":           true,
	"arrangementer": true,
	"christmas":     true,
	"contact":       true,
	"debug":         true,
	"events":        true,
	"facilities":    true,
	"galleri":       true,
	"gallery":       true,
	"img":           true,
	"kontakt":       true,
	"lokaler":       true,
	"metrics":       true,
	"om-oss":        true,
	"static":        true,
}

// Letters that do not decompose into an ASCII base letter plus marks.
var slugLetters = strings.NewReplacer(
	"æ", "ae",
	"ø", "o",
	"ß", "ss",
)

// Slugify makes an address slug from a title: lower case ASCII letters and
// digits joined by single hyphens. æ, ø and å become ae, o and a; other
// accented letters lose their accents; everything else separates words. The
// result may be empty or reserved, so callers still run ValidateSlug.
func Slugify(title string) string {
	s := slugLetters.Replace(strings.ToLower(title))

	var b strings.Builder
	hyphen := false
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// accent of the letter before it
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(r)
		default:
			hyphen = true
		}
	}

	out := b.String()
	if len(out) > maxSlugLen {
		out = out[:maxSlugLen]
		if i := strings.LastIndexByte(out, '-'); i > 0 {
			out = out[:i]
		}
	}

	return out
}

// ValidateSlug checks a slug typed by hand or made by Slugify.
func ValidateSlug(s string) error {
	switch {
	case !slugRE.MatchString(s), len(s) > maxSlugLen:
		return fmt.Errorf("%w: %q", ErrSlugInvalid, s)
	case strings.Trim(s, "0123456789") == "":
		// A number would read as a year in /arrangementer/{year}.
		return fmt.Errorf("%w: %q is only digits", ErrSlugInvalid, s)
	case reservedSlugs[s]:
		return fmt.Errorf("%w: %q", ErrSlugReserved, s)
	}

	return nil
}
