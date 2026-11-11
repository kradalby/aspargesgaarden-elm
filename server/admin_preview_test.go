package server

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/ui"
)

func (a *adminFixture) preview(t *testing.T, view string, form map[string]any, id int64) (int, string) {
	t.Helper()

	body := map[string]any{"view": view, "event": form}
	if id != 0 {
		body["id"] = id
	}

	rec := a.do(t, "POST", "/admin/api/preview", req{body: body})

	return rec.Code, rec.Body.String()
}

// A draft is shown while it is being typed: no title, no date, no photo,
// no alt text, a time half written.
func TestAdminPreviewIncompleteDraft(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	empty := julemarked(main)
	for _, k := range []string{"title", "start", "startTime", "end", "endTime", "summary", "body"} {
		empty[k] = ""
	}
	empty["mainPhoto"] = nil

	half := julemarked(main)
	half["startTime"], half["end"], half["mainPhoto"] = "1", "2026-11-3", ref(main, "")

	for name, form := range map[string]map[string]any{"empty": empty, "half typed": half} {
		for _, view := range []string{"page", "list"} {
			t.Run(name+" "+view, func(t *testing.T) {
				code, html := a.preview(t, view, form, 0)
				require.Equal(t, http.StatusOK, code, html)
				require.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
				require.Contains(t, html, `data-preview="`+view+`"`)
				require.NotContains(t, html, "<script", "the frame runs no scripts, and a preview is no visit")
			})
		}
	}

	// Nothing is written or kept.
	events, err := a.store.Events(t.Context())
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestAdminPreviewIsThePage(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	code, html := a.preview(t, "page", julemarked(main), 0)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, html, ">Julemarked 2026</h1>")
	require.Contains(t, html, "14.–15. november 2026 kl. 12.00–16.00 på Aspargesgården")
	require.Contains(t, html, `alt="Boder med julepynt i låven"`, "the form's alt text, not the stored one")
	require.Contains(t, html, "/"+main.SHA256+"/", "its stored photo, by the stored crops")
}

func TestAdminPreviewList(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	// Published, upcoming: the draft of an edit replaces it in the list.
	pub := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)}))
	pub = decode[eventJSON](t, a.do(t, "POST", "/admin/api/events/"+itoa(pub.ID)+"/publish", req{body: map[string]string{"updatedAt": pub.UpdatedAt}}))

	edit := julemarked(main)
	edit["title"] = "Julemarked med nytt navn"
	_, html := a.preview(t, "list", edit, pub.ID)
	require.Contains(t, html, "Julemarked med nytt navn")
	require.NotContains(t, html, ">Julemarked 2026<")
	require.Contains(t, html, `href="`+pub.Path+`"`, "the frozen address, not one from the new title")

	// A new, undated draft comes first among what is coming.
	fresh := julemarked(main)
	fresh["title"], fresh["start"], fresh["end"] = "Påskemarked", "", ""
	_, html = a.preview(t, "list", fresh, 0)
	require.Less(t, strings.Index(html, "Påskemarked"), strings.Index(html, "Julemarked 2026"))
	require.NotContains(t, html, "Tidligere arrangementer")
}

func TestAdminPreviewRefused(t *testing.T) {
	a := newAdmin(t, false)

	rec := a.do(t, "POST", "/admin/api/preview", req{body: "{"})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = a.do(t, "POST", "/admin/api/preview", req{body: map[string]any{"view": "poster", "event": map[string]any{}}})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = a.do(t, "POST", "/admin/api/preview", req{body: map[string]any{
		"view": "page", "event": map[string]any{"mainPhoto": map[string]any{"id": 42, "alt": "x"}},
	}})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, []problem{{Code: "not_found", Field: "mainPhoto", PhotoID: 42}}, errorsOf(t, rec))
}

// assetRE finds what a page loads: images and sources, and its <link>s
// (stylesheet, fonts, icons). Anchors are followed, not loaded.
var assetRE = regexp.MustCompile(`(?:src|srcset)="([^"]+)"|<link [^>]*href="([^"]+)"`)

// The admin shows the preview in <iframe srcdoc sandbox="allow-same-origin">:
// the frame takes the admin's address and policy. Every stylesheet, font,
// icon and photo it names must load from the admin's own origin, which the
// admin listener serves (/static/, /img/), and the admin's policy allows
// nothing else.
func TestAdminPreviewLoadsUnderTheAdminPolicy(t *testing.T) {
	a := newAdmin(t, true)
	main := a.storedPhoto(t, 'a')
	// With analytics on: the preview still carries no tracker.
	a.srv.umami = ui.Umami{SiteID: "11111111-2222-4333-8444-555555555555", Domain: "127.0.0.1"}

	rec := a.do(t, "POST", "/admin/api/preview", req{
		from: "127.0.0.1:5000", host: "127.0.0.1:8090",
		body: map[string]any{"view": "page", "event": julemarked(main)},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, ui.AdminCSP(), rec.Header().Get("Content-Security-Policy"))
	require.NotContains(t, ui.AdminCSP(), "http", "the admin allows no outside origin")
	require.NotContains(t, rec.Body.String(), ui.UmamiOrigin)

	var loads []string

	for _, m := range assetRE.FindAllStringSubmatch(rec.Body.String(), -1) {
		for c := range strings.SplitSeq(m[1]+m[2], ",") {
			u, _, _ := strings.Cut(strings.TrimSpace(c), " ")
			loads = append(loads, u)
		}
	}

	for _, u := range loads {
		if strings.HasPrefix(u, "https://aspargesgaarden.no/") {
			continue // the canonical address, which is not loaded
		}

		require.Regexp(t, `^/(static|img)/`, u)
		require.Equal(t, http.StatusOK, a.get(t, u).Code, u)
	}

	require.Contains(t, strings.Join(loads, " "), "/static/site.css")
	require.Contains(t, strings.Join(loads, " "), "/img/")
}
