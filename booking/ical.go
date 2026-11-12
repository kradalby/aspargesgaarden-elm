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
	// errUnreadable is an event the reader cannot be sure of. It fails the
	// whole read: leaving the event out could show a booked day free.
	errUnreadable = errors.New("an event the calendar cannot read")
)

// event is a VEVENT's properties the reader uses.
type event struct {
	// n is its place in the feed, to name it when it has no UID.
	n                    int
	uid                  string
	start, end, duration prop
	// recurrenceID marks one time of a repeating event, moved, changed or
	// cancelled: it stands in for the time the rule makes.
	recurrenceID            prop
	rrules, rdates, exdates []prop
	// unsupported names a property the reader cannot follow.
	unsupported string
	cancelled   bool
}

type prop struct {
	params map[string]string
	value  string
	set    bool
	// floating is the calendar's zone, once its header has been read.
	floating **time.Location
}

// parse reads the bookings in an iCalendar feed between lo and hi, every
// time of a repeating one included (see expand), and counts the events that
// are bookings (not cancelled). It reads all of the feed or fails: a whole
// VCALENDAR, every event dated without a guess. An error page, a cut-off
// body or one event it cannot be sure of is an error, never fewer bookings.
func parse(r io.Reader, lo, hi time.Time) (spans []span, bookings int, err error) {
	var (
		stack   []string
		ev      *event
		events  []*event
		ended   bool
		started bool
		// floating is the calendar's own zone (X-WR-TIMEZONE), for times
		// written without one.
		floating *time.Location
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
				ev = &event{n: len(events) + 1}
			}

			return nil
		case "END":
			if len(stack) == 0 || stack[len(stack)-1] != strings.ToUpper(value) {
				return fmt.Errorf("%w: END:%s out of place", errNotCalendar, value)
			}

			if len(stack) == 2 && ev != nil {
				events = append(events, ev)
				ev = nil
			}

			stack = stack[:len(stack)-1]
			ended = len(stack) == 0

			return nil
		}

		if name == "X-WR-TIMEZONE" && len(stack) == 1 {
			loc, err := time.LoadLocation(value)
			if err != nil {
				return fmt.Errorf("%w: X-WR-TIMEZONE %q unknown", errNotCalendar, value)
			}

			floating = loc
		}

		// Only the event's own properties: an alarm inside it has others.
		if ev != nil && len(stack) == 2 {
			ev.set(name, prop{params: params, value: value, set: true, floating: &floating})
		}

		return nil
	}

	if err := unfold(r, handle); err != nil {
		return nil, 0, err
	}

	if !ended {
		return nil, 0, fmt.Errorf("%w: no END:VCALENDAR", errNotCalendar)
	}

	for _, e := range events {
		if !e.cancelled {
			bookings++
		}
	}

	spans, err = expand(events, lo, hi)

	return spans, bookings, err
}

// set keeps the property name of e that the reader uses.
func (e *event) set(name string, p prop) {
	params, value := p.params, p.value

	switch name {
	case "UID":
		e.uid = value
	case "DTSTART":
		e.start = p
	case "DTEND":
		e.end = p
	case "DURATION":
		e.duration = p
	case "RECURRENCE-ID":
		e.recurrenceID = p
		if params["RANGE"] != "" {
			e.unsupported = "RECURRENCE-ID;RANGE=" + params["RANGE"]
		}
	case "RRULE":
		e.rrules = append(e.rrules, p)
	case "RDATE":
		e.rdates = append(e.rdates, p)
	case "EXDATE":
		e.exdates = append(e.exdates, p)
	case "EXRULE":
		e.unsupported = "EXRULE"
	case "STATUS":
		e.cancelled = strings.EqualFold(value, "CANCELLED")
	}
}

// name says which event it is, for the journal.
func (e *event) name() string {
	if e.uid != "" {
		return "event UID " + e.uid
	}

	return fmt.Sprintf("event %d", e.n)
}

// fail is an error about e that fails the read.
func (e *event) fail(format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", errUnreadable, e.name(), fmt.Sprintf(format, args...))
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

// times is the event's first time. One with a start and no end or length
// is an instant (RFC 5545): it takes no time.
func (e *event) times() (s span, allDay bool, err error) {
	if !e.start.set {
		return span{}, false, e.fail("no DTSTART")
	}

	start, allDay, err := e.start.time()
	if err != nil {
		return span{}, false, e.fail("DTSTART:%s: %v", e.start.value, err)
	}

	var end time.Time

	switch {
	case e.end.set:
		var endAllDay bool

		end, endAllDay, err = e.end.time()
		if err == nil && endAllDay != allDay {
			return span{}, false, e.fail("DTSTART and DTEND are not both days or both times")
		}
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

	switch {
	case err != nil:
		return span{}, false, e.fail("end: %v", err)
	case end.Before(start):
		return span{}, false, e.fail("ends before it starts")
	}

	return span{start, end}, allDay, nil
}

// time reads a DATE or DATE-TIME value: a day is Oslo's, a time is in its
// TZID, in UTC, or without either in the calendar's own zone. A zone the
// binary does not know (Outlook's Windows names) is an error, not a guess.
func (p prop) time() (t time.Time, allDay bool, err error) {
	v := p.value

	switch {
	case strings.EqualFold(p.params["VALUE"], "DATE") || len(v) == 8:
		t, err = time.ParseInLocation("20060102", v, site.Oslo)
		allDay = true
	case strings.HasSuffix(v, "Z"):
		t, err = time.Parse("20060102T150405Z", v)
	default:
		loc, zerr := p.zone()
		if zerr != nil {
			return time.Time{}, false, zerr
		}

		t, err = time.ParseInLocation("20060102T150405", v, loc)
	}

	return t, allDay, err
}

// zone is where p's wall clock is: its TZID, or the calendar's zone.
func (p prop) zone() (*time.Location, error) {
	if tz := p.params["TZID"]; tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("TZID %q unknown", tz)
		}

		return l, nil
	}

	if p.floating == nil || *p.floating == nil {
		return nil, errors.New("a time without a zone, in a calendar without X-WR-TIMEZONE")
	}

	return *p.floating, nil
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
