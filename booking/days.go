package booking

import (
	"slices"
	"time"

	"github.com/kradalby/asparges/site"
)

// Long is how much of a day bookings must cover to take it: a one-hour
// viewing must not make a free day look booked.
const Long = 6 * time.Hour

// Days are the days in Oslo the venue is taken. A nil Days is "no
// calendar"; an empty one is a calendar with every day free.
type Days map[site.Date]bool

// taken applies the rule to spans, counting only days from first to last:
//
//   - Overlapping and touching bookings count as one.
//   - A day is taken when bookings cover Long or more of it, midnight to
//     midnight in Oslo (a DST day has 23 or 25 hours).
//   - A booking of Long or more that takes none of its days that way, an
//     evening party from 20:00 to 03:00 say, takes the day it starts on.
//
// Friday 10:00 to Sunday 17:00 takes all three days (14, 24 and 17 hours);
// Friday 18:00 to Saturday 02:00 takes Friday only.
func taken(spans []span, first, last site.Date) Days {
	lo := midnight(first, 0)
	hi := midnight(last, 1)

	clipped := make([]span, 0, len(spans))
	for _, s := range spans {
		s.start, s.end = later(s.start, lo), earlier(s.end, hi)
		if s.end.After(s.start) {
			clipped = append(clipped, s)
		}
	}

	slices.SortFunc(clipped, func(a, b span) int { return a.start.Compare(b.start) })

	var merged []span
	for _, s := range clipped {
		if n := len(merged); n > 0 && !s.start.After(merged[n-1].end) {
			merged[n-1].end = later(merged[n-1].end, s.end)
			continue
		}

		merged = append(merged, s)
	}

	// The spans are disjoint and inside the window, so this walks each day
	// of the window at most once per span that touches it.
	covered := map[site.Date]time.Duration{}
	for _, s := range merged {
		for d := site.DateOf(s.start.In(site.Oslo)); ; d = next(d) {
			from, to := midnight(d, 0), midnight(d, 1)
			if !from.Before(s.end) {
				break
			}

			covered[d] += earlier(s.end, to).Sub(later(s.start, from))
		}
	}

	out := Days{}
	for d, c := range covered {
		if c >= Long {
			out[d] = true
		}
	}

	for _, s := range merged {
		if s.end.Sub(s.start) < Long {
			continue
		}

		hit := false
		for d := site.DateOf(s.start.In(site.Oslo)); midnight(d, 0).Before(s.end); d = next(d) {
			hit = hit || out[d]
		}

		if !hit {
			out[site.DateOf(s.start.In(site.Oslo))] = true
		}
	}

	return out
}

// midnight is the start of the day plus days after d, in Oslo.
func midnight(d site.Date, days int) time.Time {
	return time.Date(d.Year, d.Month, d.Day+days, 0, 0, 0, 0, site.Oslo)
}

func next(d site.Date) site.Date { return site.DateOf(midnight(d, 1)) }

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}

	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}

	return b
}
