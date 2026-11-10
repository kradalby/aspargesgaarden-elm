package seo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEngine answers IndexNow submissions with the given codes in turn,
// then 200, and records what it was sent.
type fakeEngine struct {
	mu    sync.Mutex
	codes []int
	got   []submission
}

func (e *fakeEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var s submission
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json; charset=utf-8" ||
		json.NewDecoder(r.Body).Decode(&s) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.got = append(e.got, s)

	code := http.StatusOK
	if len(e.codes) > 0 {
		code, e.codes = e.codes[0], e.codes[1:]
	}

	w.WriteHeader(code)
}

func (e *fakeEngine) submissions() []submission {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]submission(nil), e.got...)
}

// fakePages is the site's page list, editable while the worker runs. The
// reads listed in fail fail, those in panics panic.
type fakePages struct {
	mu     sync.Mutex
	pages  []Page
	reads  int
	fail   map[int]bool
	panics map[int]bool
}

func (p *fakePages) list(context.Context) ([]Page, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.reads++

	switch {
	case p.fail[p.reads]:
		return nil, errors.New("database is locked")
	case p.panics[p.reads]:
		panic("assignment to entry in nil map")
	}

	return append([]Page(nil), p.pages...), nil
}

func (p *fakePages) set(pages ...Page) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pages = pages
}

func (p *fakePages) readCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.reads
}

var (
	t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

// startIndexNow runs a worker against engine until the test ends, and
// returns once it has its baseline (or failed to read one).
func startIndexNow(t *testing.T, engine *fakeEngine, pages *fakePages, opts ...func(*IndexNowConfig)) *IndexNow {
	t.Helper()

	srv := httptest.NewServer(engine)
	t.Cleanup(srv.Close)

	cfg := IndexNowConfig{
		Endpoint: srv.URL, Key: "0123456789abcdef0123456789abcdef", BaseURL: testBase,
		Pages: pages.list, InitialInterval: time.Millisecond, MaxElapsed: 5 * time.Second,
	}
	for _, o := range opts {
		o(&cfg)
	}

	n, err := NewIndexNow(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)

	go func() { done <- n.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})

	require.Eventually(t, func() bool { return pages.readCount() > 0 }, 5*time.Second, time.Millisecond)

	return n
}

func TestIndexNowSendsWhatChanged(t *testing.T) {
	engine := &fakeEngine{}
	pages := &fakePages{pages: []Page{
		{Path: "/"},
		{Path: "/arrangementer", LastMod: t0},
		{Path: "/arrangementer/2025/julemarked", LastMod: t0},
		{Path: "/arrangementer/2026/varkonsert", LastMod: t0},
	}}
	n := startIndexNow(t, engine, pages)
	sent := testutil.ToFloat64(metricIndexNow.WithLabelValues("sent"))

	// The 2026 market is published, the concert edited, last year's deleted.
	pages.set(
		Page{Path: "/"},
		Page{Path: "/arrangementer", LastMod: t1},
		Page{Path: "/arrangementer/2026/julemarked", LastMod: t1},
		Page{Path: "/arrangementer/2026/varkonsert", LastMod: t1},
	)
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []submission{{
			Host: "aspargesgaarden.no",
			Key:  "0123456789abcdef0123456789abcdef",
			URLList: []string{
				"https://aspargesgaarden.no/arrangementer",
				"https://aspargesgaarden.no/arrangementer/2025/julemarked",
				"https://aspargesgaarden.no/arrangementer/2026/julemarked",
				"https://aspargesgaarden.no/arrangementer/2026/varkonsert",
			},
		}}, engine.submissions())
		assert.InDelta(c, sent+1, testutil.ToFloat64(metricIndexNow.WithLabelValues("sent")), 0)
	}, 5*time.Second, time.Millisecond)
}

// A save that changes no public page (a draft) sends nothing.
func TestIndexNowIgnoresNoChange(t *testing.T) {
	engine := &fakeEngine{}
	pages := &fakePages{pages: []Page{{Path: "/"}}}
	n := startIndexNow(t, engine, pages)

	n.Notify()
	require.Eventually(t, func() bool { return pages.readCount() >= 2 }, 5*time.Second, time.Millisecond)

	pages.set(Page{Path: "/"}, Page{Path: "/arrangementer/2026/julemarked", LastMod: t1})
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		got := engine.submissions()
		if assert.Len(c, got, 1) {
			assert.Equal(c, []string{"https://aspargesgaarden.no/arrangementer/2026/julemarked"}, got[0].URLList)
		}
	}, 5*time.Second, time.Millisecond)
}

func TestIndexNowRetriesWhatMayPass(t *testing.T) {
	engine := &fakeEngine{codes: []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}}
	pages := &fakePages{}
	n := startIndexNow(t, engine, pages)

	pages.set(Page{Path: "/", LastMod: t1})
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, engine.submissions(), 3, "two failures, then accepted")
	}, 5*time.Second, time.Millisecond)
}

func TestIndexNowGivesUpOnARejectedKey(t *testing.T) {
	engine := &fakeEngine{codes: []int{http.StatusForbidden}}
	pages := &fakePages{}
	n := startIndexNow(t, engine, pages)
	failed := testutil.ToFloat64(metricIndexNow.WithLabelValues("failed"))

	pages.set(Page{Path: "/", LastMod: t1})
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.InDelta(c, failed+1, testutil.ToFloat64(metricIndexNow.WithLabelValues("failed")), 0)
	}, 5*time.Second, time.Millisecond)
	require.Len(t, engine.submissions(), 1, "a 403 is not retried")
}

// A submission that fails for good (an hour of retries in production) is
// not the end of those changes: they go out with a later try.
func TestIndexNowResendsWhatFailed(t *testing.T) {
	codes := make([]int, 40)
	for i := range codes {
		codes[i] = http.StatusServiceUnavailable
	}

	engine := &fakeEngine{codes: codes}
	pages := &fakePages{}
	n := startIndexNow(t, engine, pages, func(c *IndexNowConfig) { c.MaxElapsed = 5 * time.Millisecond })
	sent := testutil.ToFloat64(metricIndexNow.WithLabelValues("sent"))

	pages.set(Page{Path: "/arrangementer/2026/julemarked", LastMod: t1})
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.InDelta(c, sent+1, testutil.ToFloat64(metricIndexNow.WithLabelValues("sent")), 0)
	}, 10*time.Second, time.Millisecond)

	got := engine.submissions()
	require.Equal(t, []string{"https://aspargesgaarden.no/arrangementer/2026/julemarked"}, got[len(got)-1].URLList)
}

// Without a baseline nothing can be told apart: the first change sends every
// page rather than none.
func TestIndexNowWithoutBaselineSendsAll(t *testing.T) {
	engine := &fakeEngine{}
	pages := &fakePages{pages: []Page{{Path: "/"}}, fail: map[int]bool{1: true}}
	n := startIndexNow(t, engine, pages)

	pages.set(Page{Path: "/"}, Page{Path: "/arrangementer/2026/julemarked", LastMod: t1})
	n.Notify()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, []submission{{
			Host: "aspargesgaarden.no", Key: "0123456789abcdef0123456789abcdef",
			URLList: []string{"https://aspargesgaarden.no/", "https://aspargesgaarden.no/arrangementer/2026/julemarked"},
		}}, engine.submissions())
	}, 5*time.Second, time.Millisecond)
}

// A change whose page list cannot be read, failing or panicking, is tried
// again without waiting for another change; the worker lives on.
func TestIndexNowRetriesAFailedRead(t *testing.T) {
	for name, pages := range map[string]*fakePages{
		"error": {fail: map[int]bool{2: true}},
		"panic": {panics: map[int]bool{2: true}},
	} {
		t.Run(name, func(t *testing.T) {
			engine := &fakeEngine{}
			n := startIndexNow(t, engine, pages)

			pages.set(Page{Path: "/arrangementer/2026/julemarked", LastMod: t1})
			n.Notify()

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				assert.Len(c, engine.submissions(), 1)
			}, 5*time.Second, time.Millisecond)
		})
	}
}

func TestDiff(t *testing.T) {
	require.Equal(t, []string{"a", "b", "c"}, diff(
		map[string]time.Time{"a": t0, "b": t0, "d": {}},
		map[string]time.Time{"a": t1, "c": t0, "d": {}},
	))
}
