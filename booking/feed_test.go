package booking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var weekend = feed("DTSTART:20270611T100000\nDTEND:20270613T170000")

// google serves *body, or fails with *status when it is set.
func google(t *testing.T, body *string, status *int) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if *status != 0 {
			w.WriteHeader(*status)
			return
		}

		_, _ = w.Write([]byte(*body))
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func TestFeedKeepsTheLastGoodCopy(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), FileName)
	now := func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }

	body, status := weekend, 0
	url := google(t, &body, &status)

	var changes atomic.Int32
	open := func() *Feed {
		return Open(Config{URL: url, Path: path, Now: now, Changed: func() { changes.Add(1) }})
	}

	// First start, nothing kept: no calendar until a read succeeds.
	f := open()
	require.Nil(t, f.Days())

	status = http.StatusServiceUnavailable
	require.Error(t, f.Refresh(ctx))
	require.Nil(t, f.Days())
	require.NoFileExists(t, path)

	status = 0
	require.NoError(t, f.Refresh(ctx))
	require.Len(t, f.Days(), 3)
	require.Equal(t, int32(1), changes.Load())

	// The same days again change nothing, though the bytes differ.
	body = strings.Replace(weekend, "VERSION:2.0", "VERSION:2.0\r\nX-STAMP:1", 1)
	require.NoError(t, f.Refresh(ctx))
	require.Equal(t, int32(1), changes.Load())

	// A failed or bad read keeps what was there.
	for _, bad := range []string{"<html>Service unavailable</html>", "", strings.TrimSuffix(weekend, "END:VCALENDAR\r\n")} {
		body = bad
		require.Error(t, f.Refresh(ctx))
		require.Len(t, f.Days(), 3)
	}

	require.Equal(t, int32(1), changes.Load())

	// A restart while the feed is down has the kept copy.
	status = http.StatusInternalServerError
	f = open()
	require.Len(t, f.Days(), 3)
	require.Error(t, f.Refresh(ctx))
	require.Len(t, f.Days(), 3)

	// A booking cancelled: the days change.
	status, body = 0, feed()
	require.NoError(t, f.Refresh(ctx))
	require.NotNil(t, f.Days())
	require.Empty(t, f.Days())
	require.Equal(t, int32(2), changes.Load())
}

func TestFeedRefusesHugeBodies(t *testing.T) {
	body, status := feed(strings.Repeat("X-PAD:"+strings.Repeat("x", 1000)+"\n", MaxBody/1000)), 0
	f := Open(Config{URL: google(t, &body, &status), Path: filepath.Join(t.TempDir(), FileName)})
	require.ErrorIs(t, f.Refresh(context.Background()), errNotCalendar)
	require.Nil(t, f.Days())
}

func TestFeedIgnoresAnUnreadableCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))
	assert.Nil(t, Open(Config{URL: "http://127.0.0.1:1/", Path: path}).Days())
}

func TestFeedTimesOut(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	f := Open(Config{URL: srv.URL, Path: filepath.Join(t.TempDir(), FileName), Client: &http.Client{Timeout: 50 * time.Millisecond}})
	require.Error(t, f.Refresh(context.Background()))
}
