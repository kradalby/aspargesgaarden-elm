package site

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Times of day are strict HH:MM strings: structured data needs a real time,
// and anything vaguer ("ca.") belongs in the event text.
var clockRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// ValidClock reports whether s is a strict 24-hour HH:MM time.
func ValidClock(s string) bool { return clockRE.MatchString(s) }

// legacyClockRE is the shape of the old site's free-text times: an optional
// "ca."/"cirka", an optional "kl.", then an hour with optional minutes.
var legacyClockRE = regexp.MustCompile(`^(?:(ca\.?|cirka)\s*)?(?:kl\.?\s*)?([0-9]{1,2})(?:[:.]([0-9]{2}))?$`)

// ParseLegacyTime turns a free-text time from the old site's frontmatter into
// HH:MM, for the import. approx reports a "ca.", which the import should carry
// into the event text. Anything else, such as a range or words, fails with
// ErrInvalidTime: the import must then leave the time empty and keep the
// original wording in the text rather than guess.
func ParseLegacyTime(s string) (clock string, approx bool, err error) {
	m := legacyClockRE.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidTime, s)
	}

	h, _ := strconv.Atoi(m[2])
	mins := 0
	if m[3] != "" {
		mins, _ = strconv.Atoi(m[3])
	}

	if h > 23 || mins > 59 {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidTime, s)
	}

	return fmt.Sprintf("%02d:%02d", h, mins), m[1] != "", nil
}
