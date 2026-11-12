package booking

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kradalby/asparges/site"
)

// errUnsupported refuses a feed with a repeating booking the reader cannot
// expand exactly: leaving its times out, or guessing them, could show a
// booked day free. The error names the event and the property, so the
// journal says what to change in the calendar.
var errUnsupported = errors.New("a repeating booking the calendar cannot read")

// maxSteps bounds the work of one read's repeats: periods and days looked
// at, over all events. A venue's repeats take a few thousand; a feed that
// asks for more is refused rather than left to burn the server's CPU.
const maxSteps = 500_000

// expand turns events into spans between lo and hi. A repeating event
// (RRULE, RDATE) is every time its rule makes, on its own zone's wall
// clock, so 10:00 stays 10:00 across a DST change; its EXDATEs are left
// out, and so is any time another event of its UID stands in for with a
// RECURRENCE-ID (that one counts as itself, unless cancelled).
func expand(events []*event, lo, hi time.Time) ([]span, error) {
	repeating := map[string]bool{}

	for _, e := range events {
		if e.unsupported != "" {
			return nil, fmt.Errorf("%w: %s: %s", errUnsupported, e.name(), e.unsupported)
		}

		if !e.recurrenceID.set && (len(e.rrules) > 0 || len(e.rdates) > 0) && e.uid != "" {
			repeating[e.uid] = true
		}
	}

	replaced := map[string]occurrences{}

	for _, e := range events {
		if !e.recurrenceID.set {
			continue
		}

		if !repeating[e.uid] {
			return nil, e.fail("RECURRENCE-ID:%s of no repeating event", e.recurrenceID.value)
		}

		o, ok := replaced[e.uid]
		if !ok {
			o = newOccurrences()
		}

		if err := o.add(e.recurrenceID); err != nil {
			return nil, e.fail("RECURRENCE-ID:%s: %v", e.recurrenceID.value, err)
		}

		replaced[e.uid] = o
	}

	budget := maxSteps

	var spans []span

	for _, e := range events {
		if e.cancelled {
			continue
		}

		first, allDay, err := e.times()
		if err != nil {
			return nil, err
		}

		if e.recurrenceID.set || (len(e.rrules) == 0 && len(e.rdates) == 0) {
			spans = append(spans, first)
			continue
		}

		times, err := e.repeats(first.start, allDay, lo.Add(-first.end.Sub(first.start)), hi, &budget)
		if err != nil {
			return nil, err
		}

		not := newOccurrences()
		for _, x := range e.exdates {
			if err := not.add(x); err != nil {
				return nil, e.fail("EXDATE:%s: %v", x.value, err)
			}
		}

		nights := int(first.end.Sub(first.start).Hours()/24 + 0.5)
		length := first.end.Sub(first.start)

		for _, t := range times {
			if not.has(t, allDay) || replaced[e.uid].has(t, allDay) {
				continue
			}

			s := span{t, t.Add(length)}
			if allDay {
				// Whole days: a DST day is 23 or 25 hours long.
				s.end = t.AddDate(0, 0, nights)
			}

			spans = append(spans, s)
		}
	}

	return spans, nil
}

// repeats is every start of e, by its rule and RDATEs, between from and hi.
func (e *event) repeats(start time.Time, allDay bool, from, hi time.Time, budget *int) ([]time.Time, error) {
	if len(e.rrules) > 1 {
		return nil, fmt.Errorf("%w: %s: more than one RRULE", errUnsupported, e.name())
	}

	var out []time.Time

	for _, p := range e.rrules {
		r, err := parseRule(p.value, start, allDay)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: RRULE:%s: %w", errUnsupported, e.name(), p.value, err)
		}

		times, err := r.times(start, from, hi, budget)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: RRULE:%s: %w", errUnsupported, e.name(), p.value, err)
		}

		out = append(out, times...)
	}

	for _, p := range e.rdates {
		if strings.EqualFold(p.params["VALUE"], "PERIOD") {
			return nil, fmt.Errorf("%w: %s: RDATE;VALUE=PERIOD", errUnsupported, e.name())
		}

		for v := range strings.SplitSeq(p.value, ",") {
			t, day, err := prop{params: p.params, value: v, floating: p.floating}.time()
			if err != nil || day != allDay {
				return nil, e.fail("RDATE:%s", v)
			}

			if !t.Before(from) && !t.After(hi) {
				out = append(out, t)
			}
		}
	}

	return out, nil
}

// rule is an RRULE of the parts Google's calendar writes for its repeats:
// every day, week, month or year, every n-th, on given weekdays (the
// second Saturday, the last Friday), month days or months, a number of
// times or until a day. Anything else is refused, not approximated.
type rule struct {
	freq       string
	interval   int
	count      int
	until      time.Time
	byDay      []weekdayNum
	byMonthDay []int
	byMonth    []time.Month
	wkst       time.Weekday
}

// weekdayNum is a BYDAY entry: a weekday, and n for the n-th of the month
// (negative from its end), or 0 for every one.
type weekdayNum struct {
	n   int
	day time.Weekday
}

var weekdayCodes = map[string]time.Weekday{
	"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday,
	"FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday,
}

var errRule = errors.New("not a rule the calendar expands")

// The frequencies rule expands.
const (
	daily   = "DAILY"
	weekly  = "WEEKLY"
	monthly = "MONTHLY"
	yearly  = "YEARLY"
)

func parseRule(value string, start time.Time, allDay bool) (rule, error) {
	r := rule{interval: 1, wkst: time.Monday}
	seen := map[string]bool{}

	for part := range strings.SplitSeq(strings.ToUpper(value), ";") {
		key, v, ok := strings.Cut(part, "=")
		if !ok || v == "" || seen[key] {
			return rule{}, fmt.Errorf("%w: part %s", errRule, part)
		}

		seen[key] = true

		if err := setPart(&r, key, v, start, allDay); err != nil {
			return rule{}, err
		}
	}

	return r, r.check()
}

// setPart reads one part of the rule into r.
func setPart(r *rule, key, v string, start time.Time, allDay bool) error {
	bad := fmt.Errorf("%w: part %s=%s", errRule, key, v)

	ints := func(lo, hi int) ([]int, error) {
		var out []int

		for s := range strings.SplitSeq(v, ",") {
			n, err := strconv.Atoi(s)
			if err != nil || n < lo || n > hi || n == 0 {
				return nil, bad
			}

			out = append(out, n)
		}

		return out, nil
	}

	var (
		err error
		ns  []int
	)

	switch key {
	case "FREQ":
		if v != daily && v != weekly && v != monthly && v != yearly {
			return bad
		}

		r.freq = v
	case "INTERVAL":
		ns, err = ints(1, 1000)
		if err == nil {
			r.interval = ns[0]
		}
	case "COUNT":
		ns, err = ints(1, 100_000)
		if err == nil {
			r.count = ns[0]
		}
	case "UNTIL":
		r.until, err = parseUntil(v, start, allDay)
	case "BYMONTHDAY":
		r.byMonthDay, err = ints(-31, 31)
	case "BYMONTH":
		ns, err = ints(1, 12)
		for _, m := range ns {
			r.byMonth = append(r.byMonth, time.Month(m))
		}
	case "BYDAY":
		r.byDay, err = parseByDay(v)
	case "WKST":
		wd, known := weekdayCodes[v]
		if !known {
			return bad
		}

		r.wkst = wd
	default:
		return bad
	}

	return err
}

// check refuses combinations RFC 5545 forbids or Google never writes.
func (r rule) check() error {
	ordinals := slices.ContainsFunc(r.byDay, func(w weekdayNum) bool { return w.n != 0 })

	switch {
	case r.freq == "":
		return fmt.Errorf("%w: no FREQ", errRule)
	case r.count > 0 && !r.until.IsZero():
		return fmt.Errorf("%w: both COUNT and UNTIL", errRule)
	case r.freq == weekly && len(r.byMonthDay) > 0:
		return fmt.Errorf("%w: BYMONTHDAY in a weekly rule", errRule)
	case ordinals && (r.freq == daily || r.freq == weekly):
		return fmt.Errorf("%w: a numbered BYDAY in a %s rule", errRule, strings.ToLower(r.freq))
	case r.freq == yearly && len(r.byDay) > 0 && len(r.byMonth) == 0:
		// Weekdays of the year (weeks numbered across it): not written by
		// Google's calendar, not expanded here.
		return fmt.Errorf("%w: BYDAY in a yearly rule without BYMONTH", errRule)
	}

	return nil
}

func parseByDay(v string) ([]weekdayNum, error) {
	var out []weekdayNum

	for s := range strings.SplitSeq(v, ",") {
		code := s[max(0, len(s)-2):]

		wd, known := weekdayCodes[code]
		if !known {
			return nil, fmt.Errorf("%w: BYDAY %q", errRule, v)
		}

		n := 0
		if num := s[:len(s)-2]; num != "" {
			var err error

			n, err = strconv.Atoi(num)
			if err != nil || n == 0 || n < -5 || n > 5 {
				return nil, fmt.Errorf("%w: BYDAY %q", errRule, v)
			}
		}

		out = append(out, weekdayNum{n: n, day: wd})
	}

	return out, nil
}

// parseUntil reads UNTIL: a day for an all-day event (that day included),
// a time in UTC or on the event's own clock for a timed one.
func parseUntil(v string, start time.Time, allDay bool) (time.Time, error) {
	var (
		t   time.Time
		err error
	)

	switch {
	case len(v) == 8 && allDay:
		t, err = time.ParseInLocation("20060102", v, site.Oslo)
	case strings.HasSuffix(v, "Z") && !allDay:
		t, err = time.Parse("20060102T150405Z", v)
	case len(v) == 15 && !allDay:
		t, err = time.ParseInLocation("20060102T150405", v, start.Location())
	default:
		return time.Time{}, fmt.Errorf("%w: UNTIL %s does not match DTSTART", errRule, v)
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("%w: UNTIL %s", errRule, v)
	}

	return t, nil
}

// times is every time r makes from start, the first, that can reach from
// and does not start after hi. It walks period by period (day, week, month
// or year) and stops at the first period starting after hi, so it ends
// whatever the rule says; each period and day looked at costs budget.
func (r rule) times(start, from, hi time.Time, budget *int) ([]time.Time, error) {
	loc := start.Location()
	first := site.DateOf(start)
	h, m, s := start.Clock()

	at := func(d site.Date) time.Time { return time.Date(d.Year, d.Month, d.Day, h, m, s, 0, loc) }

	if !slices.Contains(r.days(first, 0), first) {
		// RFC 5545 leaves such a set undefined: refuse it rather than guess.
		return nil, fmt.Errorf("%w: DTSTART %s is not one of its days", errRule, first)
	}

	// With no COUNT, nothing before from matters: start a period early.
	k := 0
	if r.count == 0 {
		k = max(0, r.periodOf(first, site.DateOf(from.In(loc)))/r.interval-1)
	}

	var (
		out []time.Time
		n   int
	)

	for ; ; k++ {
		period := r.periodStart(first, k*r.interval)
		if at(period).After(hi) {
			return out, nil
		}

		days := r.days(first, k*r.interval)

		if *budget -= 1 + len(days); *budget < 0 {
			return nil, errors.New("too many times to expand")
		}

		for _, d := range days {
			t := at(d)

			switch {
			case t.Before(start):
				continue
			case !r.until.IsZero() && t.After(r.until), t.After(hi):
				return out, nil
			}

			if n++; r.count > 0 && n > r.count {
				return out, nil
			}

			if !t.Before(from) {
				out = append(out, t)
			}
		}
	}
}

// periodStart is the first day of the period p periods after first's.
func (r rule) periodStart(first site.Date, p int) site.Date {
	switch r.freq {
	case daily:
		return first.AddDays(p)
	case weekly:
		return r.weekStart(first).AddDays(7 * p)
	case monthly:
		return first.MonthStart(p)
	}

	return site.Date{Year: first.Year + p, Month: time.January, Day: 1}
}

// periodOf is how many periods d's is after first's.
func (r rule) periodOf(first, d site.Date) int {
	switch r.freq {
	case daily:
		return daysBetween(first, d)
	case weekly:
		return daysBetween(r.weekStart(first), r.weekStart(d)) / 7
	case monthly:
		return first.MonthsUntil(d)
	}

	return d.Year - first.Year
}

// daysBetween counts in seconds: a time.Duration ends at 292 years.
func daysBetween(a, b site.Date) int {
	unix := func(d site.Date) int64 { return time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC).Unix() }
	return int((unix(b) - unix(a)) / (24 * 60 * 60))
}

func (r rule) weekStart(d site.Date) site.Date {
	return d.AddDays(-((int(d.Weekday()) - int(r.wkst) + 7) % 7))
}

// days are the days, in order, the period p periods after first's gives.
func (r rule) days(first site.Date, p int) []site.Date {
	var out []site.Date

	switch r.freq {
	case daily:
		d := first.AddDays(p)
		if r.inMonths(d.Month) && r.onMonthDay(d) && r.onWeekday(d) {
			out = append(out, d)
		}
	case weekly:
		ws := r.weekStart(first).AddDays(7 * p)
		for i := range 7 {
			d := ws.AddDays(i)

			on := d.Weekday() == first.Weekday()
			if len(r.byDay) > 0 {
				on = r.onWeekday(d)
			}

			if on && r.inMonths(d.Month) {
				out = append(out, d)
			}
		}
	case monthly:
		if m := first.MonthStart(p); r.inMonths(m.Month) {
			out = r.monthDays(m, first.Day)
		}
	case yearly:
		months := r.byMonth
		if len(months) == 0 {
			months = []time.Month{first.Month}
			if len(r.byMonthDay) > 0 {
				months = []time.Month{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
			}
		}

		months = slices.Sorted(slices.Values(months))
		for _, m := range slices.Compact(months) {
			out = append(out, r.monthDays(site.Date{Year: first.Year + p, Month: m, Day: 1}, first.Day)...)
		}
	}

	return out
}

func (r rule) inMonths(m time.Month) bool {
	return len(r.byMonth) == 0 || slices.Contains(r.byMonth, m)
}

func (r rule) onMonthDay(d site.Date) bool {
	if len(r.byMonthDay) == 0 {
		return true
	}

	n := monthLength(d)

	return slices.ContainsFunc(r.byMonthDay, func(v int) bool { return v == d.Day || n+1+v == d.Day })
}

func (r rule) onWeekday(d site.Date) bool {
	return len(r.byDay) == 0 || slices.ContainsFunc(r.byDay, func(w weekdayNum) bool { return w.day == d.Weekday() })
}

func monthLength(d site.Date) int { return d.MonthStart(1).AddDays(-1).Day }

// monthDays are the days of month m (its 1st) the rule gives: its month
// days and weekdays (both: the days in both), or day of the month def.
// A day the month lacks (the 31st of April) is no day.
func (r rule) monthDays(m site.Date, def int) []site.Date {
	n := monthLength(m)

	var mdays, wdays []int

	for _, v := range r.byMonthDay {
		if v < 0 {
			v = n + 1 + v
		}

		if v >= 1 && v <= n {
			mdays = append(mdays, v)
		}
	}

	for _, w := range r.byDay {
		firstOn := 1 + (int(w.day)-int(m.Weekday())+7)%7

		switch {
		case w.n == 0:
			for d := firstOn; d <= n; d += 7 {
				wdays = append(wdays, d)
			}
		case w.n > 0:
			wdays = append(wdays, firstOn+7*(w.n-1))
		default:
			lastOn := firstOn + 7*((n-firstOn)/7)
			wdays = append(wdays, lastOn+7*(w.n+1))
		}
	}

	var days []int

	switch {
	case len(r.byMonthDay) > 0 && len(r.byDay) > 0:
		for _, d := range mdays {
			if slices.Contains(wdays, d) {
				days = append(days, d)
			}
		}
	case len(r.byMonthDay) > 0:
		days = mdays
	case len(r.byDay) > 0:
		days = wdays
	default:
		days = []int{def}
	}

	slices.Sort(days)

	var out []site.Date

	for _, d := range slices.Compact(days) {
		if d >= 1 && d <= n {
			out = append(out, site.Date{Year: m.Year, Month: m.Month, Day: d})
		}
	}

	return out
}

// occurrences is a set of times of a repeating event: instants for times
// of day, days for all-day ones.
type occurrences struct {
	instants map[int64]bool
	days     map[site.Date]bool
}

func newOccurrences() occurrences {
	return occurrences{instants: map[int64]bool{}, days: map[site.Date]bool{}}
}

// add puts in every time of p, a RECURRENCE-ID or an EXDATE (a list).
func (o occurrences) add(p prop) error {
	for v := range strings.SplitSeq(p.value, ",") {
		t, allDay, err := prop{params: p.params, value: v, floating: p.floating}.time()
		if err != nil {
			return err
		}

		if allDay {
			o.days[site.DateOf(t)] = true
		} else {
			o.instants[t.Unix()] = true
		}
	}

	return nil
}

// has reports whether t, a time an event's rule made, is in o.
func (o occurrences) has(t time.Time, allDay bool) bool {
	// A day stands for that day's time, as for an all-day event.
	return o.days[site.DateOf(t.In(site.Oslo))] || !allDay && o.instants[t.Unix()]
}
