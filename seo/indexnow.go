package seo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// IndexNowEndpoint shares a ping with every search engine in IndexNow
// (Bing, which also feeds ChatGPT search and Copilot, Yandex, Seznam…).
const IndexNowEndpoint = "https://api.indexnow.org/indexnow"

var metricIndexNow = register(prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "asparges_indexnow_submissions_total",
	Help: "IndexNow submissions of changed addresses: sent, or failed (rejected, or still failing after retrying).",
}, []string{"result"}))

// IndexNowConfig configures an IndexNow.
type IndexNowConfig struct {
	// Endpoint receives the pings; IndexNowEndpoint in production.
	Endpoint string
	// Key is the site's key, served at /<key>.txt.
	Key string
	// BaseURL is the public origin the pages live under.
	BaseURL string
	// Pages lists every indexable page and its last change: the sitemap.
	Pages func(context.Context) ([]Page, error)
	// MaxElapsed bounds the retries of one submission.
	MaxElapsed time.Duration
	// InitialInterval is the first wait before a retry; tests shorten it.
	InitialInterval time.Duration
	Client          *http.Client
	Log             *slog.Logger
}

// IndexNow tells search engines which pages changed, as soon as they do. It
// keeps the last list of pages and, on each Notify, compares it with a new
// one: added, removed and edited pages are sent. A write only has to say
// that something changed, never what.
type IndexNow struct {
	cfg  IndexNowConfig
	host string
	poke chan struct{}
}

// NewIndexNow returns a worker; Run must be running for Notify to send.
func NewIndexNow(cfg IndexNowConfig) (*IndexNow, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("indexnow: base URL %q: %w", cfg.BaseURL, err)
	}

	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}

	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	if cfg.MaxElapsed == 0 {
		cfg.MaxElapsed = time.Hour
	}

	if cfg.InitialInterval == 0 {
		cfg.InitialInterval = 30 * time.Second
	}

	return &IndexNow{cfg: cfg, host: u.Host, poke: make(chan struct{}, 1)}, nil
}

// Notify says the public pages may have changed. It never blocks: changes
// that come while a submission is under way are sent after it.
func (n *IndexNow) Notify() {
	select {
	case n.poke <- struct{}{}:
	default:
	}
}

// Run sends changes until ctx ends. The pages at start are the baseline:
// a restart is not a change. The baseline moves only past what was sent,
// or refused for good, so a change is never dropped because a read or a
// submission failed; those are tried again a moment later.
func (n *IndexNow) Run(ctx context.Context) error {
	prev, err := n.snapshot(ctx)
	if err != nil {
		// Nothing to tell changes apart by: the first one sends every page
		// rather than none.
		n.cfg.Log.ErrorContext(ctx, "indexnow: reading pages at start", "err", err)
		prev = map[string]time.Time{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-n.poke:
		}

		cur, err := n.snapshot(ctx)
		if err != nil {
			n.cfg.Log.ErrorContext(ctx, "indexnow: reading pages", "err", err)
			n.later()

			continue
		}

		changed := diff(prev, cur)
		if len(changed) == 0 {
			prev = cur
			continue
		}

		err = n.submit(ctx, changed)

		switch {
		case err == nil:
			prev = cur
			metricIndexNow.WithLabelValues("sent").Inc()
			n.cfg.Log.InfoContext(ctx, "indexnow: submitted", "urls", changed)
		case ctx.Err() != nil:
			return nil //nolint:nilerr // shutting down: what is unsent is no failure
		case errors.Is(err, errRejected):
			// Sending it again would be refused again.
			prev = cur
			metricIndexNow.WithLabelValues("failed").Inc()
			n.cfg.Log.ErrorContext(ctx, "indexnow: submission refused", "urls", len(changed), "err", err)
		default:
			metricIndexNow.WithLabelValues("failed").Inc()
			n.cfg.Log.WarnContext(ctx, "indexnow: submission failed, trying again", "urls", len(changed), "err", err)
			n.later()
		}
	}
}

// later pokes the worker again after a pause, to retry what failed.
func (n *IndexNow) later() {
	time.AfterFunc(n.cfg.InitialInterval, n.Notify)
}

// snapshot reads the page list. The list is put together from the database
// on this worker's goroutine, which nothing else recovers: a panic in that
// is an error here, not the end of the process.
func (n *IndexNow) snapshot(ctx context.Context) (out map[string]time.Time, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("listing pages: panic: %v\n%s", r, debug.Stack())
		}
	}()

	pages, err := n.cfg.Pages(ctx)
	if err != nil {
		return nil, err
	}

	out = make(map[string]time.Time, len(pages))
	for _, p := range pages {
		out[n.cfg.BaseURL+p.Path] = p.LastMod
	}

	return out, nil
}

// diff is every address added, removed or edited from prev to cur, sorted.
func diff(prev, cur map[string]time.Time) []string {
	var out []string

	for u, t := range cur {
		if old, ok := prev[u]; !ok || !old.Equal(t) {
			out = append(out, u)
		}
	}

	for u := range prev {
		if _, ok := cur[u]; !ok {
			out = append(out, u)
		}
	}

	slices.Sort(out)

	return out
}

type submission struct {
	Host    string   `json:"host"`
	Key     string   `json:"key"`
	URLList []string `json:"urlList"`
}

var (
	errStatus   = errors.New("indexnow answered")
	errRejected = errors.New("indexnow refused the submission")
)

// submit posts urls, retrying what may pass later (network, 429, 5xx) and
// giving up at once on what will not (a bad key, foreign addresses).
func (n *IndexNow) submit(ctx context.Context, urls []string) error {
	body, err := json.Marshal(submission{Host: n.host, Key: n.cfg.Key, URLList: urls})
	if err != nil {
		return fmt.Errorf("encoding: %w", err)
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = n.cfg.InitialInterval

	_, err = backoff.Retry(ctx, func() (struct{}, error) {
		return struct{}{}, n.post(ctx, body)
	}, backoff.WithBackOff(bo), backoff.WithMaxElapsedTime(n.cfg.MaxElapsed))

	return err
}

func (n *IndexNow) post(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return backoff.Permanent(fmt.Errorf("%w: %w", errRejected, err))
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := n.cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))

	switch code := resp.StatusCode; {
	case code == http.StatusOK || code == http.StatusAccepted:
		return nil
	case code == http.StatusTooManyRequests:
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			return backoff.RetryAfter(secs)
		}

		return fmt.Errorf("%w %d", errStatus, code)
	case code >= 500:
		return fmt.Errorf("%w %d", errStatus, code)
	default:
		return backoff.Permanent(fmt.Errorf("%w: %w %d", errRejected, errStatus, code))
	}
}
