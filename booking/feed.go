package booking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kradalby/asparges/site"
)

// How often the feed is read, and how long after its last good read the
// calendar is still shown. The owner's rule: no calendar, and an alert, is
// better than a wrong one. A good read every Every keeps a new booking's
// lag short; MaxAge is six reads in a row failing, not one hiccup, and the
// longest the site will show bookings that may have changed since.
const (
	Every  = 5 * time.Minute
	MaxAge = 30 * time.Minute
)

const (
	// Timeout bounds one read.
	Timeout = 30 * time.Second
	// MaxBody is far over two years of bookings (Google's feed is about
	// 250 bytes an event) and well under what the server can hold.
	MaxBody = 4 << 20
)

// FileName is the last good copy of the feed and when it was read, in the
// data directory: not in tmp/, which the service empties at start.
const FileName = "calendar.json"

// copyFile is FileName's content. The read time is in it, not the file's
// mtime, which a copy or a restore changes.
type copyFile struct {
	Read time.Time `json:"read"`
	Feed string    `json:"feed"`
}

// The read results: the feed could not be fetched, or its content was
// refused (no calendar, an event the reader cannot be sure of, no bookings).
const (
	resultOK      = "ok"
	resultFetch   = "fetch_failed"
	resultRefused = "refused"
)

var (
	metricReads = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_calendar_reads_total",
		Help: "Reads of the booking calendar feed by result: ok, fetch_failed (network, timeout, HTTP status, size), refused (content the reader cannot be sure of).",
	}, []string{"result"}))
	metricLastRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_calendar_last_read_timestamp_seconds",
		Help: "When the booking calendar in use was last read whole from the feed; 0 is never.",
	})
	metricShown = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_calendar_shown",
		Help: "1 while the site shows the booking calendar, 0 while it is hidden for being older than its limit.",
	})
	metricEvents = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_calendar_events",
		Help: "Bookings (events not cancelled) in the booking calendar in use.",
	})
	metricTakenDays = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_calendar_taken_dates",
		Help: "Taken days in the booking calendar in use.",
	})
	// The gauges exist only where a calendar is configured, so an alert on
	// them never fires for a site without one.
	registerGauges sync.Once
)

func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}

var (
	errStatus = errors.New("calendar feed answered")
	errEmpty  = errors.New("the calendar has no bookings at all: more likely a sharing change or a fault than every booking gone")
)

// Config configures a Feed.
type Config struct {
	// URL is the public iCal address of the calendar.
	URL string
	// Path keeps the last good copy, so a restart while the feed is down
	// still has the bookings, within MaxAge of their read.
	Path string
	// Client reads the feed; one with Timeout when nil.
	Client *http.Client
	// Now is the clock that decides the days kept and their age; time.Now
	// when nil.
	Now func() time.Time
	// Changed is called when what the site shows changes: the taken days,
	// or the calendar hidden or shown again.
	Changed func()
	Log     *slog.Logger
}

// Feed holds the taken days from the last good read of the calendar.
type Feed struct {
	cfg Config

	mu    sync.Mutex
	days  Days
	read  time.Time
	shown shownState

	// fails counts reads in a row that failed, for the journal.
	fails atomic.Int32
}

// shownState is what the site was last told to show.
type shownState struct {
	on   bool
	days Days
}

// Open returns a Feed with the days of the last good copy, if there is one
// and it is younger than MaxAge. Run keeps them current.
func Open(cfg Config) *Feed {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: Timeout}
	}

	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	if cfg.Changed == nil {
		cfg.Changed = func() {}
	}

	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	registerGauges.Do(func() {
		prometheus.MustRegister(metricLastRead, metricShown, metricEvents, metricTakenDays)
	})

	f := &Feed{cfg: cfg}
	f.load()
	f.shown = f.current()
	f.publish()

	return f
}

// load reads the last good copy.
func (f *Feed) load() {
	raw, err := os.ReadFile(f.cfg.Path)

	switch {
	case errors.Is(err, os.ErrNotExist):
		return
	case err != nil:
		f.cfg.Log.Error("calendar: reading the last copy", "path", f.cfg.Path, "err", err)
		return
	}

	var c copyFile
	if err := json.Unmarshal(raw, &c); err != nil || c.Read.IsZero() {
		f.cfg.Log.Error("calendar: the last copy is unreadable", "path", f.cfg.Path, "err", err)
		return
	}

	days, bookings, err := f.parse([]byte(c.Feed))
	if err != nil {
		f.cfg.Log.Error("calendar: the last copy is unreadable", "path", f.cfg.Path, "err", err)
		return
	}

	f.days, f.read = days, c.Read
	metricEvents.Set(float64(bookings))
}

// Shown are the taken days the site may show, and whether it may: false
// when the calendar was never read whole or not for MaxAge. The map is
// never changed after it is returned.
func (f *Feed) Shown() (Days, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s := f.current()

	return s.days, s.on
}

// current is what the site should show now. Call with mu held.
func (f *Feed) current() shownState {
	if f.days == nil || f.cfg.Now().Sub(f.read) >= MaxAge {
		return shownState{}
	}

	return shownState{on: true, days: f.days}
}

// Settle tells the site when what it should show has changed since it was
// last told: new taken days, or the calendar crossing MaxAge either way.
// Run calls it after every read and at the moment the calendar would
// cross MaxAge.
func (f *Feed) Settle() {
	f.mu.Lock()
	cur := f.current()
	changed := cur.on != f.shown.on || cur.on && !maps.Equal(cur.days, f.shown.days)
	f.shown = cur
	f.mu.Unlock()

	f.publish()

	if !changed {
		return
	}

	if cur.on {
		f.cfg.Log.Info("calendar: showing", "taken_days", len(cur.days))
	} else {
		f.cfg.Log.Error("calendar: hidden, no good read for too long", "max_age", MaxAge)
	}

	f.cfg.Changed()
}

func (f *Feed) publish() {
	f.mu.Lock()
	defer f.mu.Unlock()

	metricLastRead.Set(0)
	if !f.read.IsZero() {
		metricLastRead.Set(float64(f.read.Unix()))
	}

	metricShown.Set(0)
	if f.shown.on {
		metricShown.Set(1)
	}

	metricTakenDays.Set(float64(len(f.days)))
}

// Run reads the feed at start and every Every until ctx ends, and settles
// what the site shows after each read and when the last good read turns
// MaxAge old. A failed read keeps what was there and waits for the next
// turn: never retried in a burst.
func (f *Feed) Run(ctx context.Context) error {
	t := time.NewTicker(Every)
	defer t.Stop()

	for {
		f.Refresh(ctx)

		if ctx.Err() != nil {
			return nil
		}

		f.Settle()

		f.mu.Lock()
		expiry := time.Until(f.read.Add(MaxAge))
		f.mu.Unlock()

		var expired <-chan time.Time
		if expiry > 0 {
			expired = time.After(expiry)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		case <-expired:
			f.Settle()

			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}
	}
}

// Refresh reads the feed once and keeps it if it is whole and certain.
// It counts and logs the result; Settle tells the site.
func (f *Feed) Refresh(ctx context.Context) {
	result, err := f.refresh(ctx)
	metricReads.WithLabelValues(result).Inc()

	if err == nil {
		f.fails.Store(0)
		return
	}

	if ctx.Err() != nil {
		return
	}

	f.cfg.Log.WarnContext(ctx, "calendar: read failed", "result", result, "in_a_row", f.fails.Add(1), "err", err)
}

func (f *Feed) refresh(ctx context.Context) (string, error) {
	body, err := f.fetch(ctx)
	if err != nil {
		return resultFetch, err
	}

	days, bookings, err := f.parse(body)
	if err != nil {
		return resultRefused, err
	}

	now := f.cfg.Now()

	if err := writeCopy(f.cfg.Path, copyFile{Read: now, Feed: string(body)}); err != nil {
		// The days are good; a restart before the next read loses them.
		f.cfg.Log.ErrorContext(ctx, "calendar: keeping a copy", "err", err)
	}

	f.mu.Lock()
	f.days, f.read = days, now
	f.mu.Unlock()

	metricEvents.Set(float64(bookings))
	f.publish()

	return resultOK, nil
}

func (f *Feed) fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.cfg.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "asparges (+https://aspargesgaarden.no)")

	resp, err := f.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w %d", errStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}

	if len(body) > MaxBody {
		return nil, fmt.Errorf("%w: over %d bytes", errStatus, MaxBody)
	}

	return body, nil
}

// parse reads body into the taken days from the start of last year to the
// end of four years ahead (the feed holds the past too, and nothing is
// booked further out), and counts its bookings. A calendar without any is
// refused.
func (f *Feed) parse(body []byte) (Days, int, error) {
	y := site.Today(f.cfg.Now()).Year
	first, last := site.Date{Year: y - 1, Month: time.January, Day: 1}, site.Date{Year: y + 4, Month: time.December, Day: 31}

	spans, bookings, err := parse(bytes.NewReader(body), midnight(first, 0), midnight(last, 1))
	if err != nil {
		return nil, 0, err
	}

	if bookings == 0 {
		return nil, 0, errEmpty
	}

	return taken(spans, first, last), bookings, nil
}

// writeCopy replaces path with c whole or not at all.
func writeCopy(path string, c copyFile) error {
	body, err := json.Marshal(c)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // gone after the rename

	_, err = tmp.Write(body)
	if err == nil {
		err = tmp.Sync()
	}

	if cerr := tmp.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}
