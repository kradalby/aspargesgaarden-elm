package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// writeFixture has a published event and a draft on one photo, an
// unused photo and a photographer, and "/" kept with its ETag. Behind the
// admin's back an event joined the menu, so a dropped "/" shows "Skjult".
type writeFixture struct {
	a                *adminFixture
	main, unused     site.Photo
	published, draft eventJSON
	photographer     site.Photographer
	etag             string
}

func newWriteFixture(t *testing.T) writeFixture {
	t.Helper()

	w := writeFixture{a: newAdmin(t, false)}
	w.main, w.unused = w.a.storedPhoto(t, 'a'), w.a.storedPhoto(t, 'b')

	e := decode[eventJSON](t, w.a.do(t, "POST", "/admin/api/events", req{body: julemarked(w.main)}))
	w.published = decode[eventJSON](t, w.a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}}))
	w.draft = decode[eventJSON](t, w.a.do(t, "POST", "/admin/api/events", req{body: titled(julemarked(w.main), "Adventsmarked")}))

	var err error
	w.main, err = w.a.store.Photo(t.Context(), w.main.ID)
	require.NoError(t, err)

	w.photographer, err = w.a.store.CreatePhotographer(t.Context(), site.Photographer{Name: "Ola"})
	require.NoError(t, err)

	w.etag = w.a.get(t, "/").Header().Get("ETag")
	require.NotEmpty(t, w.etag)

	_, err = w.a.store.CreatePublished(t.Context(), site.Event{
		Title: "Skjult", Start: date("2026-12-01"), ShowInMenu: true, MainPhoto: &w.main,
	})
	require.NoError(t, err)

	w.a.images.enqueued = nil

	return w
}

func titled(in map[string]any, title string) map[string]any {
	in["title"] = title
	return in
}

type call struct {
	method, path string
	r            req
}

// Every write route that changes what the public pages show drops the kept
// pages and asks for the photos it needs built; a refused write does neither.
func TestAdminWritesReachThePages(t *testing.T) {
	upload := []byte("a new photo")
	uploadSum := sha256.Sum256(upload)

	tests := []struct {
		route   string // as registered
		ok      func(w writeFixture) call
		refused func(w writeFixture) call
		// enqueued is what a successful write asks to be built.
		enqueued func(w writeFixture) []string
		kept     bool // nothing public changed
	}{
		{
			route: "POST /admin/api/events",
			ok: func(w writeFixture) call {
				return call{"POST", "/admin/api/events", req{body: titled(julemarked(w.main), "Påskemarked")}}
			},
			refused: func(writeFixture) call { return call{"POST", "/admin/api/events", req{body: "{"}} },
		},
		{
			route: "PUT /admin/api/events/{id}",
			ok: func(w writeFixture) call {
				in := julemarked(w.main)
				in["updatedAt"], in["slug"] = w.published.UpdatedAt, w.published.Slug

				return call{"PUT", "/admin/api/events/" + itoa(w.published.ID), req{body: in}}
			},
			refused: func(w writeFixture) call {
				in := julemarked(w.main)
				in["updatedAt"] = "2000-01-01T00:00:00Z"

				return call{"PUT", "/admin/api/events/" + itoa(w.published.ID), req{body: in}}
			},
			enqueued: func(w writeFixture) []string { return []string{w.main.SHA256} },
		},
		{
			route: "POST /admin/api/events/{id}/publish",
			ok: func(w writeFixture) call {
				return call{"POST", "/admin/api/events/" + itoa(w.draft.ID) + "/publish", req{body: map[string]string{"updatedAt": w.draft.UpdatedAt}}}
			},
			refused: func(w writeFixture) call {
				return call{"POST", "/admin/api/events/" + itoa(w.draft.ID) + "/publish", req{body: map[string]string{"updatedAt": ""}}}
			},
			enqueued: func(w writeFixture) []string { return []string{w.main.SHA256} },
		},
		{
			route: "POST /admin/api/events/{id}/unpublish",
			ok: func(w writeFixture) call {
				return call{"POST", "/admin/api/events/" + itoa(w.published.ID) + "/unpublish", req{body: map[string]string{"updatedAt": w.published.UpdatedAt}}}
			},
			refused: func(w writeFixture) call {
				return call{"POST", "/admin/api/events/999/unpublish", req{body: map[string]string{"updatedAt": w.published.UpdatedAt}}}
			},
		},
		{
			route: "POST /admin/api/events/{id}/copy",
			ok: func(w writeFixture) call {
				return call{"POST", "/admin/api/events/" + itoa(w.published.ID) + "/copy", req{}}
			},
			refused: func(writeFixture) call { return call{"POST", "/admin/api/events/999/copy", req{}} },
		},
		{
			route:   "DELETE /admin/api/events/{id}",
			ok:      func(w writeFixture) call { return call{"DELETE", "/admin/api/events/" + itoa(w.published.ID), req{}} },
			refused: func(writeFixture) call { return call{"DELETE", "/admin/api/events/999", req{}} },
		},
		{
			route: "POST /admin/api/photos",
			ok: func(writeFixture) call {
				body, ctype := multipartBody(t, "ny.jpg", upload)
				return call{"POST", "/admin/api/photos", req{body: body, ctype: ctype}}
			},
			refused: func(writeFixture) call {
				return call{"POST", "/admin/api/photos", req{body: "no parts", ctype: "text/plain"}}
			},
			enqueued: func(writeFixture) []string { return []string{hex.EncodeToString(uploadSum[:])} },
		},
		{
			route: "PUT /admin/api/photos/{id}",
			ok: func(w writeFixture) call {
				return call{"PUT", "/admin/api/photos/" + itoa(w.main.ID), req{body: map[string]any{"alt": "Låven i snø", "photographerId": nil}}}
			},
			refused: func(w writeFixture) call {
				return call{"PUT", "/admin/api/photos/" + itoa(w.main.ID), req{body: map[string]any{"alt": " ", "photographerId": nil}}}
			},
		},
		{
			route: "PUT /admin/api/photos/{id}/crops",
			ok: func(w writeFixture) call {
				c := w.main.Crops[images.Card]
				return call{"PUT", "/admin/api/photos/" + itoa(w.main.ID) + "/crops", req{body: map[string]rectJSON{"card": {X: c.X - 10, Y: c.Y, W: c.W, H: c.H}}}}
			},
			refused: func(w writeFixture) call {
				return call{"PUT", "/admin/api/photos/" + itoa(w.main.ID) + "/crops", req{body: map[string]rectJSON{"card": {W: 10, H: 10}}}}
			},
			enqueued: func(w writeFixture) []string { return []string{w.main.SHA256} },
		},
		{
			route:    "DELETE /admin/api/photos/{id}",
			ok:       func(w writeFixture) call { return call{"DELETE", "/admin/api/photos/" + itoa(w.unused.ID), req{}} },
			refused:  func(w writeFixture) call { return call{"DELETE", "/admin/api/photos/" + itoa(w.main.ID), req{}} },
			enqueued: func(w writeFixture) []string { return []string{w.unused.SHA256} },
		},
		{
			route: "POST /admin/api/photos/{id}/prepare",
			ok: func(w writeFixture) call {
				return call{"POST", "/admin/api/photos/" + itoa(w.main.ID) + "/prepare", req{}}
			},
			refused: func(writeFixture) call { return call{"POST", "/admin/api/photos/999/prepare", req{}} },
			kept:    true,
		},
		{
			route: "POST /admin/api/photographers",
			ok: func(writeFixture) call {
				return call{"POST", "/admin/api/photographers", req{body: map[string]string{"name": "Kari"}}}
			},
			refused: func(writeFixture) call {
				return call{"POST", "/admin/api/photographers", req{body: map[string]string{"name": " "}}}
			},
		},
		{
			route: "PUT /admin/api/photographers/{id}",
			ok: func(w writeFixture) call {
				return call{"PUT", "/admin/api/photographers/" + itoa(w.photographer.ID), req{body: map[string]string{"name": "Ola Foto"}}}
			},
			refused: func(writeFixture) call {
				return call{"PUT", "/admin/api/photographers/999", req{body: map[string]string{"name": "X"}}}
			},
		},
		{
			route: "PUT /admin/api/gallery",
			ok: func(w writeFixture) call {
				return call{"PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{w.main.ID}}}}
			},
			refused: func(writeFixture) call {
				return call{"PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{999}}}}
			},
		},
	}

	covered := make([]string, len(tests))
	for i, tt := range tests {
		covered[i] = tt.route
	}

	require.ElementsMatch(t, writeRoutes(t), covered, "a write route without a row here")

	for _, tt := range tests {
		for _, refused := range []bool{false, true} {
			name := tt.route
			if refused {
				name += " refused"
			}

			t.Run(name, func(t *testing.T) {
				w := newWriteFixture(t)

				c := tt.ok(w)
				if refused {
					c = tt.refused(w)
				}

				rec := w.a.do(t, c.method, c.path, c.r)
				require.Equal(t, refused, rec.Code >= http.StatusBadRequest, "%d %s", rec.Code, rec.Body.String())

				page := w.a.get(t, "/", "If-None-Match", w.etag)

				if refused || tt.kept {
					require.Equal(t, http.StatusNotModified, page.Code, "the kept page stays")
					require.Empty(t, w.a.images.enqueued)

					return
				}

				require.Equal(t, http.StatusOK, page.Code, "the kept page is dropped")
				require.Contains(t, page.Body.String(), "Skjult")

				var want []string
				if tt.enqueued != nil {
					want = tt.enqueued(w)
				}

				require.Equal(t, want, w.a.images.enqueued)
			})
		}
	}
}

// writeRoutes is every route the server registers that is not a GET, except
// preview, which renders and stores nothing.
func writeRoutes(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	re := regexp.MustCompile(`HandleFunc\("((?:POST|PUT|DELETE|PATCH) [^"]+)"`)

	var out []string

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		b, err := os.ReadFile(f)
		require.NoError(t, err)

		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if m[1] != "POST /admin/api/preview" {
				out = append(out, m[1])
			}
		}
	}

	return out
}
