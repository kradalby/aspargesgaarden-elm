package render

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = t
}

func oslo(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, site.Oslo)
	if err != nil {
		panic(err)
	}

	return t
}

// counter builds a page whose text is the number of times it was built.
type counter struct{ n atomic.Int32 }

func (c *counter) build(context.Context) (*elem.Element, error) {
	return elem.P(nil, elem.Text(strconv.Itoa(int(c.n.Add(1))))), nil
}

func get(t *testing.T, s *Store, key string, build Builder) *Page {
	t.Helper()

	p, err := s.Get(t.Context(), key, build)
	require.NoError(t, err)

	return p
}

func TestGetKeepsThePage(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	var c counter

	first := get(t, s, "/a", c.build)
	require.Equal(t, "<p>1</p>", string(first.HTML()))
	require.Same(t, first, get(t, s, "/a", c.build))
	require.Equal(t, "<p>2</p>", string(get(t, s, "/b", c.build).HTML()))
}

func TestInvalidateDropsEverything(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	var c counter

	a := get(t, s, "/a", c.build)
	get(t, s, "/b", c.build)

	s.Invalidate()

	require.NotSame(t, a, get(t, s, "/a", c.build))
	require.Equal(t, int32(3), c.n.Load())
}

// The pages depend on what day it is in Oslo (upcoming vs past, menu), so
// the first request after midnight there renders afresh.
func TestNewOsloDayDropsEverything(t *testing.T) {
	clk := &clock{t: oslo("2026-11-15 23:59")}
	s := New(clk.now)

	var c counter

	before := get(t, s, "/a", c.build)

	clk.set(oslo("2026-11-15 23:59").Add(59 * time.Second))
	require.Same(t, before, get(t, s, "/a", c.build))

	// 23:00 UTC is midnight in Oslo in winter: a UTC clock would not notice.
	clk.set(oslo("2026-11-16 00:00"))
	after := get(t, s, "/a", c.build)
	require.NotSame(t, before, after)
	require.Equal(t, "<p>2</p>", string(after.HTML()))
}

var errBroken = errors.New("broken")

func TestErrorsAreNotKept(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	fail := true
	build := func(context.Context) (*elem.Element, error) {
		if fail {
			return nil, errBroken
		}

		return elem.P(nil), nil
	}

	_, err := s.Get(t.Context(), "/a", build)
	require.ErrorIs(t, err, errBroken)

	fail = false
	require.Equal(t, "<p></p>", string(get(t, s, "/a", build).HTML()))
}

func TestConcurrentMissesRenderOnce(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	release := make(chan struct{})
	started := make(chan struct{})

	var builds atomic.Int32

	var build Builder = func(context.Context) (*elem.Element, error) {
		if builds.Add(1) == 1 {
			close(started)
		}
		<-release

		return elem.P(nil), nil
	}

	const n = 8

	pages := make(chan *Page, n)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			p, _ := s.Get(t.Context(), "/a", build)
			pages <- p
		})
	}

	// Callers arriving while the render runs share it; later ones find it kept.
	<-started
	close(release)
	wg.Wait()
	close(pages)

	require.Equal(t, int32(1), builds.Load())

	first := <-pages
	for p := range pages {
		require.Same(t, first, p)
	}
}

// A page that panics while rendering is one error with its cause for each
// request that waited on it, not a panic thrown into all of them.
func TestPanickingRenderIsAnError(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	release := make(chan struct{})
	started := make(chan struct{})

	var builds atomic.Int32

	var build Builder = func(context.Context) (*elem.Element, error) {
		if builds.Add(1) == 1 {
			close(started)
		}
		<-release

		panic("photo without a picture")
	}

	const n = 4

	errs := make(chan error, n)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			_, err := s.Get(t.Context(), "/a", build)
			errs <- err
		})
	}

	<-started
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.ErrorContains(t, err, "photo without a picture")
		require.ErrorContains(t, err, "render.TestPanickingRenderIsAnError", "the error carries where it panicked")
	}

	_, err := s.Get(t.Context(), "/a", func(context.Context) (*elem.Element, error) { return elem.P(nil), nil })
	require.NoError(t, err, "nothing is kept of it")
}

// A render that started before Invalidate shows data from before the change;
// it may answer the requests that waited for it, but must not be kept.
func TestRenderRacingInvalidateIsNotKept(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)

	var c counter

	build := func(ctx context.Context) (*elem.Element, error) {
		s.Invalidate()

		return c.build(ctx)
	}

	get(t, s, "/a", build)
	require.Equal(t, "<p>2</p>", string(get(t, s, "/a", c.build).HTML()))
}

func serve(t *testing.T, p *Page, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a", nil)
	req.Header = header
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	return rec
}

func TestServe(t *testing.T) {
	s := New((&clock{t: oslo("2026-11-10 12:00")}).now)
	p := get(t, s, "/a", func(context.Context) (*elem.Element, error) {
		return elem.P(nil, elem.Text("Julemarked på Aspargesgården")), nil
	})

	plain := serve(t, p, http.Header{})
	require.Equal(t, http.StatusOK, plain.Code)
	require.Equal(t, "<p>Julemarked på Aspargesgården</p>", plain.Body.String())
	require.Equal(t, "text/html; charset=utf-8", plain.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", plain.Header().Get("Cache-Control"))
	require.Equal(t, "Accept-Encoding", plain.Header().Get("Vary"))
	require.Empty(t, plain.Header().Get("Content-Encoding"))

	etag := plain.Header().Get("ETag")
	require.Regexp(t, `^"[0-9a-f]{32}"$`, etag)

	zipped := serve(t, p, http.Header{"Accept-Encoding": {"br, gzip"}})
	require.Equal(t, "gzip", zipped.Header().Get("Content-Encoding"))
	require.NotEqual(t, etag, zipped.Header().Get("ETag"), "each encoding is its own representation")

	zr, err := gzip.NewReader(bytes.NewReader(zipped.Body.Bytes()))
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.Equal(t, plain.Body.String(), string(body))

	require.Empty(t, serve(t, p, http.Header{"Accept-Encoding": {"gzip;q=0"}}).Header().Get("Content-Encoding"))

	notModified := serve(t, p, http.Header{"If-None-Match": {`"other", ` + etag}})
	require.Equal(t, http.StatusNotModified, notModified.Code)
	require.Empty(t, notModified.Body.String())

	zippedNotModified := serve(t, p, http.Header{"If-None-Match": {zipped.Header().Get("ETag")}, "Accept-Encoding": {"gzip"}})
	require.Equal(t, http.StatusNotModified, zippedNotModified.Code)

	require.Equal(t, http.StatusOK, serve(t, p, http.Header{"If-None-Match": {`"stale"`}}).Code)
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":                  false,
		"gzip":              true,
		"GZIP":              true,
		"deflate, gzip":     true,
		"gzip;q=0.5":        true,
		"gzip; q=0":         false,
		"gzip;q=0.000":      false,
		"x-gzip":            false,
		"*":                 false,
		"br;q=1, gzip;q=.2": true,
	} {
		require.Equal(t, want, acceptsGzip(header), header)
	}
}
