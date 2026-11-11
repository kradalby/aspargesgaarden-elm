// Package booking reads the farm's public booking calendar (Google's iCal
// feed) and tells which days in Oslo are taken. The server reads the feed on
// a timer and keeps the last good copy on disk, so the site never waits for
// Google and survives it being down.
package booking

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/kradalby/asparges/site"
)

// span is one booking, [start, end).
type span struct{ start, end time.Time }

// maxLine bounds one unfolded content line: Google's are under a hundred
// bytes, and a feed must not make the reader grow a line without end.
const maxLine = 64 << 10

var (
	errNotCalendar = errors.New("not an iCalendar feed")
	errRecurring   = errors.New("a recurring booking of six hours or more, which the reader does not expand")
)

// event is a VEVENT's properties the reader uses.
type event struct {
	start, end, duration prop
	recurs, cancelled    bool
}

type prop struct {
	params map[string]string
	value  string
	set    bool
}

// parse reads the bookings in an iCalendar feed. It needs a whole
// VCALENDAR, so an error page or a cut-off body is an error, not "no
// bookings". An event it cannot date is left out; a recurring event is read
// as its first time only, unless each time lasts six hours or more: then the
// feed is refused, since dropping the repeats would show booked days free.
func parse(r io.Reader) (spans []span, skipped int, err error) {
	var (
		stack   []string
		ev      *event
		ended   bool
		started bool
	)

	handle := func(l string) error {
		name, params, value, ok := contentLine(l)
		if !ok {
			return nil
		}

		if !started {
			if name != "BEGIN" || !strings.EqualFold(value, "VCALENDAR") {
				return errNotCalendar
			}

			started = true
		}

		switch name {
		case "BEGIN":
			stack = append(stack, strings.ToUpper(value))
			if len(stack) == 2 && stack[1] == "VEVENT" {
				ev = &event{}
			}

			return nil
		case "END":
			if len(stack) == 0 || stack[len(stack)-1] != strings.ToUpper(value) {
				return fmt.Errorf("%w: END:%s out of place", errNotCalendar, value)
			}

			if len(stack) == 2 && ev != nil {
				s, ok, err := ev.span()
				switch {
				case err != nil:
					return err
				case ok:
					spans = append(spans, s)
				case !ev.cancelled:
					skipped++
				}

				ev = nil
			}

			stack = stack[:len(stack)-1]
			ended = len(stack) == 0

			return nil
		}

		// Only the event's own properties: an alarm inside it has others.
		if ev == nil || len(stack) != 2 {
			return nil
		}

		p := prop{params: params, value: value, set: true}

		switch name {
		case "DTSTART":
			ev.start = p
		case "DTEND":
			ev.end = p
		case "DURATION":
			ev.duration = p
		case "RRULE", "RDATE":
			ev.recurs = true
		case "STATUS":
			ev.cancelled = strings.EqualFold(value, "CANCELLED")
		}

		return nil
	}

	if err := unfold(r, handle); err != nil {
		return nil, 0, err
	}

	if !ended {
		return nil, 0, fmt.Errorf("%w: no END:VCALENDAR", errNotCalendar)
	}

	return spans, skipped, nil
}

// unfold calls handle with each content line of r, its folds joined: a
// line that starts with a space or tab continues the one before.
func unfold(r io.Reader, handle func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), maxLine)

	var line strings.Builder

	first := true

	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if first {
			l, first = strings.TrimPrefix(l, "\xef\xbb\xbf"), false
		}

		if l != "" && (l[0] == ' ' || l[0] == '\t') {
			if line.Len()+len(l)-1 > maxLine {
				return fmt.Errorf("%w: a line longer than %d bytes", errNotCalendar, maxLine)
			}

			line.WriteString(l[1:])

			continue
		}

		if line.Len() > 0 {
			if err := handle(line.String()); err != nil {
				return err
			}
		}

		line.Reset()
		line.WriteString(l)
	}

	if err := sc.Err(); err != nil {
		return fmt.Errorf("%w: %w", errNotCalendar, err)
	}

	if line.Len() > 0 {
		return handle(line.String())
	}

	return nil
}

// contentLine splits NAME;PARAM=V;…:VALUE. A colon inside a quoted
// parameter value is not the separator.
func contentLine(l string) (name string, params map[string]string, value string, ok bool) {
	quoted := false
	colon := -1

	for i := range len(l) {
		if l[i] == '"' {
			quoted = !quoted
		}

		if l[i] == ':' && !quoted {
			colon = i
			break
		}
	}

	if colon <= 0 {
		return "", nil, "", false
	}

	parts := strings.Split(l[:colon], ";")
	for _, p := range parts[1:] {
		k, v, _ := strings.Cut(p, "=")
		if params == nil {
			params = map[string]string{}
		}

		params[strings.ToUpper(k)] = strings.Trim(v, `"`)
	}

	return strings.ToUpper(parts[0]), params, l[colon+1:], true
}

// span is the event's time; ok is false for a cancelled event or one with
// no readable start, or no length.
func (e *event) span() (span, bool, error) {
	if e.cancelled || !e.start.set {
		return span{}, false, nil
	}

	start, allDay, err := e.start.time()
	if err != nil {
		return span{}, false, nil //nolint:nilerr // an undatable event is skipped, not the feed
	}

	var end time.Time

	switch {
	case e.end.set:
		end, _, err = e.end.time()
	case e.duration.set:
		var d time.Duration

		d, err = duration(e.duration.value)
		end = start.Add(d)
		if allDay && err == nil && d%(24*time.Hour) == 0 {
			// Whole days, not hours: a DST day is 23 or 25 hours long.
			end = start.AddDate(0, 0, int(d/(24*time.Hour)))
		}
	case allDay:
		end = start.AddDate(0, 0, 1)
	default:
		end = start
	}

	if err != nil || !end.After(start) {
		return span{}, false, nil //nolint:nilerr // as above
	}

	if e.recurs && (allDay || end.Sub(start) >= Long) {
		return span{}, false, errRecurring
	}

	return span{start, end}, true, nil
}

// time reads a DATE or DATE-TIME value. A time without a zone, or in a zone
// the binary does not know (Outlook's Windows names), is Oslo time.
func (p prop) time() (t time.Time, allDay bool, err error) {
	v := p.value
	loc := site.Oslo

	if tz := p.params["TZID"]; tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}

	switch {
	case strings.EqualFold(p.params["VALUE"], "DATE") || len(v) == 8:
		t, err = time.ParseInLocation("20060102", v, site.Oslo)
		allDay = true
	case strings.HasSuffix(v, "Z"):
		t, err = time.Parse("20060102T150405Z", v)
	default:
		t, err = time.ParseInLocation("20060102T150405", v, loc)
	}

	return t, allDay, err
}

// duration reads an RFC 5545 duration, "PT3H", "P1D", "P2W", "P1DT2H30M".
// A negative one is refused.
func duration(s string) (time.Duration, error) {
	rest, ok := strings.CutPrefix(strings.TrimPrefix(s, "+"), "P")
	if !ok || rest == "" {
		return 0, fmt.Errorf("duration %q", s)
	}

	units := map[byte]time.Duration{'W': 7 * 24 * time.Hour, 'D': 24 * time.Hour, 'H': time.Hour, 'M': time.Minute, 'S': time.Second}

	var (
		total  time.Duration
		inTime bool
		seen   bool
		num    string
	)

	for i := range len(rest) {
		c := rest[i]

		switch {
		case c == 'T' && !inTime && num == "":
			inTime = true
		case c >= '0' && c <= '9':
			num += string(c)
		default:
			u, known := units[c]
			n, err := strconv.Atoi(num)

			if !known || err != nil || n > 1e5 || (c == 'M' && !inTime) {
				return 0, fmt.Errorf("duration %q", s)
			}

			total += time.Duration(n) * u
			num = ""
			seen = true
		}
	}

	if num != "" || !seen {
		return 0, fmt.Errorf("duration %q", s)
	}

	return total, nil
}
