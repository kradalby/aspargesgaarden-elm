// Package render keeps rendered public pages. A page is built on its first
// request and then served from memory, with gzip and a strong ETag, until
// anything changes: the admin calls Invalidate after every write, and a new
// day in Oslo (which moves events from upcoming to past) drops every page
// by itself. Error pages and the admin preview never go through it.
package render

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chasefleming/elem-go"
	"golang.org/x/sync/singleflight"

	"github.com/kradalby/asparges/site"
)

// Builder renders one page. An error (a missing event, a failing database)
// is returned to the caller and nothing is kept.
type Builder func(context.Context) (*elem.Element, error)

// Store holds the kept pages by key, usually the page's path.
type Store struct {
	now func() time.Time

	mu    sync.Mutex
	pages map[string]*Page
	day   site.Date
	// gen counts invalidations, so a render that began before one is
	// neither kept nor shared with requests that come after it.
	gen uint64

	flight singleflight.Group
}

// New returns an empty Store; now is the clock that decides the day.
func New(now func() time.Time) *Store {
	return &Store{now: now, pages: map[string]*Page{}}
}

// Get returns the kept page for key, or builds it once, however many
// requests ask at the same time, and keeps it.
func (s *Store) Get(ctx context.Context, key string, build Builder) (*Page, error) {
	s.mu.Lock()
	if today := site.Today(s.now()); today != s.day {
		s.dropLocked()
		s.day = today
	}

	if p, ok := s.pages[key]; ok {
		s.mu.Unlock()
		return p, nil
	}

	gen := s.gen
	s.mu.Unlock()

	// One caller giving up must not fail the others waiting on the render.
	ctx = context.WithoutCancel(ctx)

	v, err, _ := s.flight.Do(strconv.FormatUint(gen, 10)+" "+key, func() (any, error) {
		page, err := build(ctx)
		if err != nil {
			return nil, err
		}

		p := newPage(page.Render())

		s.mu.Lock()
		if s.gen == gen {
			s.pages[key] = p
		}
		s.mu.Unlock()

		return p, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(*Page), nil //nolint:forcetypeassert // the only type stored above
}

// Invalidate drops every kept page. Call it after any write that can change
// a public page: events, photos, crops, photographers, the gallery.
func (s *Store) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dropLocked()
}

func (s *Store) dropLocked() {
	s.pages = map[string]*Page{}
	s.gen++
}

// Page is one rendered page, ready to send.
type Page struct {
	html, gz     []byte
	etag, gzETag string
}

func newPage(html string) *Page {
	var gz bytes.Buffer

	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression) // the level is valid
	_, _ = zw.Write([]byte(html))                           // a bytes.Buffer cannot fail
	_ = zw.Close()

	sum := sha256.Sum256([]byte(html))
	tag := hex.EncodeToString(sum[:16])

	return &Page{
		html:   []byte(html),
		gz:     gz.Bytes(),
		etag:   `"` + tag + `"`,
		gzETag: `"` + tag + `-gz"`,
	}
}

// HTML is the page as rendered.
func (p *Page) HTML() []byte { return p.html }

// ServeHTTP sends the page, gzipped when the client takes it, or 304 when
// the client's copy is current. Browsers revalidate every time (no-cache):
// a page changes whenever the admin saves, so it must never go stale.
func (p *Page) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Add("Vary", "Accept-Encoding")

	body, etag := p.html, p.etag
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		body, etag = p.gz, p.gzETag
		h.Set("Content-Encoding", "gzip")
	}

	h.Set("ETag", etag)
	// ServeContent answers If-None-Match with 304, and HEAD and ranges.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

// acceptsGzip reports whether an Accept-Encoding header names gzip with a
// non-zero quality.
func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}

		q, ok := strings.CutPrefix(strings.TrimSpace(params), "q=")
		if !ok {
			return true
		}

		v, err := strconv.ParseFloat(strings.TrimSpace(q), 64)

		return err == nil && v > 0
	}

	return false
}
