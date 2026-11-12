package booking

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kradalby/asparges/site"
)

var (
	metricReads = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_calendar_reads_total",
		Help: "Reads of the booking calendar feed: ok, or failed (network, status, or a body that is no calendar).",
	}, []string{"result"}))
	metricLastRead = register(prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_calendar_last_read_timestamp_seconds",
		Help: "When the booking calendar shown was read from the feed; 0 is never.",
	}))
)

func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}

// Defaults of Config.
const (
	DefaultEvery   = 15 * time.Minute
	DefaultTimeout = 30 * time.Second
	// MaxBody is far over two years of bookings (Google's feed is about
	// 250 bytes an event) and well under what the server can hold.
	MaxBody = 4 << 20
)

// FileName is the last good copy of the feed, in the data directory: not in
// tmp/, which the service empties at start.
const FileName = "calendar.ics"

// Config configures a Feed.
type Config struct {
	// URL is the public iCal address of the calendar.
	URL string
	// Path keeps the last good copy, so a restart while the feed is down
	// still has the bookings.
	Path string
	// Every spaces the reads; DefaultEvery when zero.
	Every time.Duration
	// Client reads the feed; one with DefaultTimeout when nil.
	Client *http.Client
	// Now is the clock that decides the days kept; time.Now when nil.
	Now func() time.Time
	// Changed is called when the taken days change.
	Changed func()
	Log     *slog.Logger
}

// Feed holds the taken days from the last good read of the calendar.
type Feed struct {
	cfg Config

	mu   sync.Mutex
	days Days
}

// Open returns a Feed with the days of the last good copy, if there is one.
// Run keeps them current.
func Open(cfg Config) *Feed {
	if cfg.Every == 0 {
		cfg.Every = DefaultEvery
	}

	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: DefaultTimeout}
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

	f := &Feed{cfg: cfg}

	body, err := os.ReadFile(cfg.Path)

	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		cfg.Log.Error("calendar: reading the last copy", "path", cfg.Path, "err", err)
	default:
		days, err := f.read(body)
		if err != nil {
			cfg.Log.Error("calendar: the last copy is unreadable", "path", cfg.Path, "err", err)
			break
		}

		f.days = days

		if st, err := os.Stat(cfg.Path); err == nil {
			metricLastRead.Set(float64(st.ModTime().Unix()))
		}
	}

	return f
}

// Days are the taken days, or nil when the calendar was never read. The
// map is never changed after it is returned.
func (f *Feed) Days() Days {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.days
}

// Run reads the feed at start and every cfg.Every until ctx ends. A failed
// read keeps what was there and waits for the next turn: the feed is read
// on a schedule, never retried in a burst.
func (f *Feed) Run(ctx context.Context) error {
	t := time.NewTicker(f.cfg.Every)
	defer t.Stop()

	for {
		err := f.Refresh(ctx)

		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			metricReads.WithLabelValues("failed").Inc()
			f.cfg.Log.WarnContext(ctx, "calendar: read failed, keeping the last copy", "err", err)
		default:
			metricReads.WithLabelValues("ok").Inc()
		}

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// Refresh reads the feed once and, if it is a calendar, keeps it.
func (f *Feed) Refresh(ctx context.Context) error {
	body, err := f.fetch(ctx)
	if err != nil {
		return err
	}

	days, err := f.read(body)
	if err != nil {
		return err
	}

	if err := writeFile(f.cfg.Path, body); err != nil {
		// The days are good; only a restart before the next read loses them.
		f.cfg.Log.ErrorContext(ctx, "calendar: keeping a copy", "err", err)
	}

	metricLastRead.Set(float64(f.cfg.Now().Unix()))

	f.mu.Lock()
	changed := f.days == nil || !maps.Equal(f.days, days)
	f.days = days
	f.mu.Unlock()

	if changed {
		f.cfg.Log.InfoContext(ctx, "calendar: taken days changed", "days", len(days))
		f.cfg.Changed()
	}

	return nil
}

var errStatus = errors.New("calendar feed answered")

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
		return nil, fmt.Errorf("%w: over %d bytes", errNotCalendar, MaxBody)
	}

	return body, nil
}

// read reads body into the taken days from the start of last year to the
// end of four years ahead; the feed holds the past too, and nothing is
// booked further out.
func (f *Feed) read(body []byte) (Days, error) {
	y := site.Today(f.cfg.Now()).Year
	first, last := site.Date{Year: y - 1, Month: time.January, Day: 1}, site.Date{Year: y + 4, Month: time.December, Day: 31}

	spans, _, err := parse(bytes.NewReader(body), midnight(first, 0), midnight(last, 1))
	if err != nil {
		return nil, err
	}

	return taken(spans, first, last), nil
}

// writeFile replaces path with body whole or not at all.
func writeFile(path string, body []byte) error {
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
