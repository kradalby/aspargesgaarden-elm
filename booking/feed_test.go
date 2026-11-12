package booking

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var weekend = feed("DTSTART:20270611T100000\nDTEND:20270613T170000")

// google serves *body, or fails with *status when it is set.
func google(t *testing.T, body *string, status *int) string {
	t.Helper()

	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if *status != 0 {
			w.WriteHeader(*status)
			return
		}

		_, _ = w.Write([]byte(*body))
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// clock is a time the test moves by hand.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// testFeed is a Feed on a fake clock against a fake Google, counting the
// times it tells the site to drop its pages.
type testFeed struct {
	*Feed

	clock   *clock
	body    string
	status  int
	path    string
	url     string
	changes int
}

func newTestFeed(t *testing.T) *testFeed {
	t.Helper()

	tf := &testFeed{
		clock: &clock{t: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)},
		body:  weekend,
		path:  filepath.Join(t.TempDir(), FileName),
	}
	tf.url = google(t, &tf.body, &tf.status)
	tf.open()

	return tf
}

func (tf *testFeed) open() {
	tf.Feed = Open(Config{URL: tf.url, Path: tf.path, Now: tf.clock.now, Changed: func() { tf.changes++ }})
}

// read is one turn of Run: a read, then telling the site.
func (tf *testFeed) read() {
	tf.Refresh(context.Background())
	tf.Settle()
}

func (tf *testFeed) shown() bool {
	_, ok := tf.Shown()
	return ok
}

func TestFeedHidesAnOldCalendarAndShowsItAgain(t *testing.T) {
	tf := newTestFeed(t)

	// First start, nothing kept: no calendar until a read succeeds.
	require.False(t, tf.shown())
	tf.read()
	require.True(t, tf.shown())
	require.Equal(t, 1, tf.changes)
	require.InDelta(t, 1, testutil.ToFloat64(metricShown), 0)
	require.InDelta(t, 3, testutil.ToFloat64(metricTakenDays), 0)

	// The same days again change nothing.
	tf.clock.advance(Every)
	tf.read()
	require.Equal(t, 1, tf.changes)

	// Google down: shown through five failed reads, hidden at the sixth,
	// MaxAge after the last good one, and the site told once.
	tf.status = http.StatusServiceUnavailable
	fetchFailed := testutil.ToFloat64(metricReads.WithLabelValues(resultFetch))

	for range 5 {
		tf.clock.advance(Every)
		tf.read()
		require.True(t, tf.shown())
	}

	require.Equal(t, 1, tf.changes)

	tf.clock.advance(Every)
	tf.read()
	require.False(t, tf.shown())
	require.Equal(t, 2, tf.changes)
	require.InDelta(t, 0, testutil.ToFloat64(metricShown), 0)
	require.InDelta(t, fetchFailed+6, testutil.ToFloat64(metricReads.WithLabelValues(resultFetch)), 0)

	tf.clock.advance(Every)
	tf.read()
	require.Equal(t, 2, tf.changes, "hidden stays hidden without telling the site again")

	// Back: shown at once, the site told once.
	tf.status = 0
	tf.clock.advance(Every)
	tf.read()
	require.True(t, tf.shown())
	require.Equal(t, 3, tf.changes)
}

// Run hides the calendar at the moment it turns MaxAge old, between reads.
func TestSettleBetweenReads(t *testing.T) {
	tf := newTestFeed(t)
	tf.read()

	tf.clock.advance(MaxAge - time.Second)
	tf.Settle()
	require.Equal(t, 1, tf.changes)

	tf.clock.advance(time.Second)
	tf.Settle()
	require.False(t, tf.shown())
	require.Equal(t, 2, tf.changes)
}

func TestRestartHonoursTheCopysAge(t *testing.T) {
	tf := newTestFeed(t)
	tf.read()

	tf.status = http.StatusInternalServerError

	// Ten minutes later, Google down: the copy is shown.
	tf.clock.advance(10 * time.Minute)
	tf.open()
	require.True(t, tf.shown())
	tf.read()
	require.True(t, tf.shown())

	// Two hours after the read: not shown, however new the process.
	tf.clock.advance(110 * time.Minute)
	tf.open()
	require.False(t, tf.shown())

	// A copy's file time does not count, only the read time in it.
	require.NoError(t, os.Chtimes(tf.path, tf.clock.now(), tf.clock.now()))
	tf.open()
	require.False(t, tf.shown())

	var c copyFile

	raw, err := os.ReadFile(tf.path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &c))
	assert.Equal(t, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC), c.Read.UTC())
}

func TestRefusedReadsKeepTheCopyAndCountAsRefused(t *testing.T) {
	tf := newTestFeed(t)
	tf.read()

	refused := testutil.ToFloat64(metricReads.WithLabelValues(resultRefused))

	for _, bad := range []string{
		"<html>Service unavailable</html>",
		"",
		strings.TrimSuffix(weekend, "END:VCALENDAR\r\n"),
		// Used to be left out and counted: now it fails the read.
		feed("DTSTART:20270611T100000\nDTEND:20270613T170000", "DTSTART:not a date\nDTEND:20270614T200000"),
		// No bookings at all after a calendar full of them.
		feed(),
		feed("DTSTART:20270611T100000\nDTEND:20270613T170000\nSTATUS:CANCELLED"),
	} {
		tf.body = bad
		tf.clock.advance(Every)
		tf.read()
		require.Len(t, tf.days, 3)
	}

	assert.InDelta(t, refused+6, testutil.ToFloat64(metricReads.WithLabelValues(resultRefused)), 0)
	assert.False(t, tf.shown(), "six refused reads in a row hide it")
}

func TestFeedRefusesHugeBodies(t *testing.T) {
	tf := newTestFeed(t)
	tf.body = feed(strings.Repeat("X-PAD:"+strings.Repeat("x", 1000)+"\n", MaxBody/1000))
	tf.read()
	require.False(t, tf.shown())
}

func TestFeedIgnoresAnUnreadableCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))

	_, ok := Open(Config{URL: "http://127.0.0.1:1/", Path: path}).Shown()
	assert.False(t, ok)
}

func TestFeedTimesOut(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	f := Open(Config{URL: srv.URL, Path: filepath.Join(t.TempDir(), FileName), Client: &http.Client{Timeout: 50 * time.Millisecond}})
	_, err := f.refresh(context.Background())
	require.Error(t, err)
}
