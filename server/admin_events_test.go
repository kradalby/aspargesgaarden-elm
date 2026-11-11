package server

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// storedPhoto adds a photo the way an upload leaves it.
func (a *adminFixture) storedPhoto(t *testing.T, c byte) site.Photo {
	t.Helper()

	p, _, err := a.store.AddPhoto(t.Context(), site.Photo{
		SHA256: strings.Repeat(string(c), 64), MIME: "image/jpeg", Width: 1600, Height: 1200,
	})
	require.NoError(t, err)

	return p
}

func ref(p site.Photo, alt string) map[string]any {
	return map[string]any{"id": p.ID, "alt": alt, "photographerId": nil}
}

// julemarked is the form for a complete event with its main photo.
func julemarked(main site.Photo) map[string]any {
	return map[string]any{
		"updatedAt": "", "slug": "", "title": " Julemarked 2026 ", "summary": "Velkommen til årets julemarked.",
		"body": "## Når og hvor\n\nI Låven.", "start": "2026-11-14", "startTime": "12:00",
		"end": "2026-11-15", "endTime": "16:00", "extraDates": []string{}, "priceText": "Gratis",
		"ticketUrl": "", "showInMenu": true, "menuLabel": "Julemarked", "cancelled": false,
		"mainPhoto": ref(main, "Boder med julepynt i låven"), "extraPhotos": []any{},
	}
}

func TestAdminEventLifecycle(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	rec := a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	e := decode[eventJSON](t, rec)
	require.Equal(t, "draft", e.Status)
	require.Equal(t, "Julemarked 2026", e.Title, "trimmed")
	require.Equal(t, "julemarked-2026", e.Slug)
	require.Equal(t, "/arrangementer/2026/julemarked-2026", e.Path)
	require.Equal(t, "https://aspargesgaarden.no/arrangementer/2026/julemarked-2026", e.URL)
	require.Equal(t, "14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården", e.DateText)
	require.Equal(t, "Boder med julepynt i låven", e.MainPhoto.Alt, "the form's alt text is stored with the event")
	require.Nil(t, e.PublishedAt)
	require.Equal(t, []string{}, e.ExtraDates)

	// A draft is not on the public site.
	require.Equal(t, http.StatusNotFound, a.get(t, e.Path).Code)

	rec = a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	pub := decode[eventJSON](t, rec)
	require.Equal(t, "published", pub.Status)
	require.True(t, pub.SlugFrozen)
	require.Contains(t, a.images.enqueued, main.SHA256, "publishing asks for the photo's sizes")

	// The write is on the public pages at once: the kept list is dropped.
	page := a.get(t, e.Path)
	require.Equal(t, http.StatusOK, page.Code)
	require.Contains(t, page.Body.String(), "Julemarked 2026")
	require.Contains(t, a.get(t, "/arrangementer").Body.String(), "Julemarked 2026")
	require.Contains(t, a.get(t, "/").Body.String(), `href="/arrangementer/2026/julemarked-2026"`, "in the menu")

	lists := decode[map[string][]eventJSON](t, a.do(t, "GET", "/admin/api/events", req{}))
	require.Len(t, lists["upcoming"], 1)
	require.Empty(t, lists["drafts"])
	require.Empty(t, lists["past"])

	rec = a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/copy", req{})
	require.Equal(t, http.StatusCreated, rec.Code)

	cp := decode[eventJSON](t, rec)
	require.Equal(t, "draft", cp.Status)
	require.Empty(t, cp.Start)
	require.Empty(t, cp.Path, "no date, no address yet")
	require.Equal(t, main.ID, cp.MainPhoto.ID)

	rec = a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/unpublish", req{body: map[string]string{"updatedAt": pub.UpdatedAt}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, http.StatusNotFound, a.get(t, e.Path).Code)

	require.Equal(t, http.StatusNoContent, a.do(t, "DELETE", "/admin/api/events/"+itoa(e.ID), req{}).Code)
	require.Equal(t, http.StatusNotFound, a.do(t, "GET", "/admin/api/events/"+itoa(e.ID), req{}).Code)
	require.Equal(t, http.StatusNotFound, a.do(t, "DELETE", "/admin/api/events/"+itoa(e.ID), req{}).Code)

	_, err := a.store.Photo(t.Context(), main.ID)
	require.NoError(t, err, "the photos stay")
}

func TestAdminEventLists(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')
	ctx := t.Context()

	for _, e := range []site.Event{
		{Title: "Vårkonsert", Start: date("2026-05-19")},
		{Title: "Julemarked", Start: date("2026-11-14")},
	} {
		e.MainPhoto = &site.Photo{ID: main.ID, Alt: "Låven"}
		e, err := a.store.CreateEvent(ctx, e)
		require.NoError(t, err)
		_, err = a.store.Publish(ctx, e.ID, e.UpdatedAt)
		require.NoError(t, err)
	}

	older, err := a.store.CreateEvent(ctx, site.Event{Title: "Gammel kladd"})
	require.NoError(t, err)

	a.clock.set(a.clock.now().Add(time.Minute))
	_, err = a.store.CreateEvent(ctx, site.Event{})
	require.NoError(t, err)

	lists := decode[map[string][]eventJSON](t, a.do(t, "GET", "/admin/api/events", req{}))
	require.Equal(t, []string{"Julemarked"}, titles(lists["upcoming"]))
	require.Equal(t, []string{"Vårkonsert"}, titles(lists["past"]))
	require.True(t, lists["past"][0].IsOver)
	require.Equal(t, []string{"", older.Title}, titles(lists["drafts"]), "newest change first")
	require.False(t, lists["drafts"][0].IsOver, "an undated draft is not over")
}

func titles(events []eventJSON) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Title
	}

	return out
}

func TestAdminEventRules(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')
	extra := a.storedPhoto(t, 'b')

	tests := []struct {
		name   string
		mod    func(map[string]any)
		status int
		want   []problem
	}{
		{"empty draft is fine", func(f map[string]any) {
			for _, k := range []string{"title", "start", "startTime", "end", "endTime", "summary", "body"} {
				f[k] = ""
			}
			f["mainPhoto"] = nil
		}, http.StatusCreated, nil},
		{
			"ticket link", func(f map[string]any) { f["ticketUrl"] = "billetter.no/julemarked" },
			http.StatusUnprocessableEntity,
			[]problem{{Code: "ticket_url", Field: "ticketUrl"}},
		},
		{"every broken rule at once", func(f map[string]any) {
			f["startTime"], f["endTime"], f["slug"] = "25:00", "9", "admin"
		}, http.StatusUnprocessableEntity, []problem{
			{Code: "slug_reserved", Field: "slug"},
			{Code: "invalid_time", Field: "startTime"},
			{Code: "invalid_time", Field: "endTime"},
		}},
		{"dates", func(f map[string]any) {
			f["start"], f["end"], f["extraDates"] = "14.11.2026", "2026-02-30", []string{"x"}
		}, http.StatusUnprocessableEntity, []problem{
			{Code: "invalid_date", Field: "start"},
			{Code: "invalid_date", Field: "end"},
			{Code: "invalid_date", Field: "extraDates"},
		}},
		{
			"end before start", func(f map[string]any) { f["end"] = "2026-11-13" },
			http.StatusUnprocessableEntity,
			[]problem{{Code: "end_before_start", Field: "end"}},
		},
		{
			"end time before start time", func(f map[string]any) { f["end"], f["endTime"] = "", "11:00" },
			http.StatusUnprocessableEntity,
			[]problem{{Code: "end_before_start", Field: "endTime"}},
		},
		{
			"photo twice", func(f map[string]any) { f["extraPhotos"] = []any{ref(main, "x")} },
			http.StatusUnprocessableEntity,
			[]problem{{Code: "duplicate_photo", Field: "extraPhotos", PhotoID: main.ID}},
		},
		{
			"poster is the main photo", func(f map[string]any) { f["poster"] = ref(main, "x") },
			http.StatusUnprocessableEntity,
			[]problem{{Code: "duplicate_photo", Field: "poster", PhotoID: main.ID}},
		},
		{
			"unknown poster", func(f map[string]any) { f["poster"] = map[string]any{"id": 999, "alt": "x"} },
			http.StatusNotFound,
			[]problem{{Code: "not_found", Field: "poster", PhotoID: 999}},
		},
		{"unknown photos", func(f map[string]any) {
			f["mainPhoto"] = map[string]any{"id": 999, "alt": "x"}
			f["extraPhotos"] = []any{ref(extra, "y"), map[string]any{"id": extra.ID, "alt": "y", "photographerId": 77}}
		}, http.StatusNotFound, []problem{
			{Code: "not_found", Field: "mainPhoto", PhotoID: 999},
			{Code: "not_found", Field: "extraPhotos", PhotoID: extra.ID},
		}},
		{"not JSON", nil, http.StatusBadRequest, []problem{{Code: "bad_request"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body any = "{"
			if tt.mod != nil {
				f := julemarked(main)
				tt.mod(f)
				body = f
			}

			rec := a.do(t, "POST", "/admin/api/events", req{body: body})
			require.Equal(t, tt.status, rec.Code, rec.Body.String())

			if tt.want != nil {
				require.Equal(t, tt.want, errorsOf(t, rec))
			}
		})
	}
}

func TestAdminPublishRules(t *testing.T) {
	a := newAdmin(t, false)
	extra := a.storedPhoto(t, 'b')

	f := julemarked(extra)
	f["title"], f["start"], f["end"], f["mainPhoto"] = "", "", "", nil
	f["extraPhotos"] = []any{ref(extra, "")}

	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: f}))

	rec := a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, []problem{
		{Code: "title_required", Field: "title"},
		{Code: "start_required", Field: "start"},
		{Code: "slug_invalid", Field: "slug"},
		{Code: "main_photo_required", Field: "mainPhoto"},
		{Code: "alt_required", Field: "extraPhotos", PhotoID: extra.ID},
	}, errorsOf(t, rec))

	// A published event cannot be edited into one that could not be.
	main := a.storedPhoto(t, 'a')
	e = decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)}))
	e = decode[eventJSON](t, a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}}))

	edit := julemarked(main)
	edit["updatedAt"], edit["title"], edit["slug"] = e.UpdatedAt, "", "nytt-navn"
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: edit})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, []problem{{Code: "slug_frozen", Field: "slug"}}, errorsOf(t, rec))

	edit["slug"] = e.Slug
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: edit})
	require.Equal(t, []problem{{Code: "title_required", Field: "title"}}, errorsOf(t, rec))
}

// Sharpness only ever warns: an event whose photos are soft, or carry a crop
// under today's floor, is saved and published, and its page still shows.
func TestAdminPhotoQualityNeverBlocks(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')
	_, err := a.store.SetCrops(t.Context(), main.ID, map[images.Kind]images.Rect{
		images.Card:   {X: 0, Y: 0, W: 500, H: 625},
		images.Banner: {X: 0, Y: 0, W: 700, H: 394},
	})
	require.NoError(t, err)

	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)}))
	require.True(t, e.MainPhoto.Quality["card"].Soft)

	rec := a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	edit := julemarked(main)
	edit["updatedAt"], edit["slug"], edit["summary"] = decode[eventJSON](t, rec).UpdatedAt, e.Slug, "Ny tekst."
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: edit})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	stored, err := a.store.Photo(t.Context(), main.ID)
	require.NoError(t, err)
	require.Equal(t, images.Rect{X: 0, Y: 0, W: 700, H: 394}, stored.Crops[images.Banner], "a text edit leaves the crops alone")

	page := a.get(t, e.Path)
	require.Equal(t, http.StatusOK, page.Code)
	require.Contains(t, page.Body.String(), "/full-", "the banner under the floor is drawn as the whole photo, as before")
	require.Contains(t, a.get(t, "/arrangementer").Body.String(), "/card-", "the soft card is drawn")
}

// Two editors: a write made against an old version is refused with the
// stored one, so neither loses what they typed.
func TestAdminEventConflict(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)}))
	loaded := e.UpdatedAt

	a.clock.set(a.clock.now().Add(time.Second))

	theirs := julemarked(main)
	theirs["updatedAt"], theirs["priceText"] = loaded, "Kr 50,-"
	rec := a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: theirs})
	require.Equal(t, http.StatusOK, rec.Code)

	current := decode[eventJSON](t, rec)

	for _, w := range []struct{ method, path string }{
		{"PUT", "/admin/api/events/" + itoa(e.ID)},
		{"POST", "/admin/api/events/" + itoa(e.ID) + "/publish"},
		{"POST", "/admin/api/events/" + itoa(e.ID) + "/unpublish"},
	} {
		for _, seen := range []string{loaded, "", "not a time"} {
			mine := julemarked(main)
			mine["updatedAt"], mine["priceText"] = seen, "Kr 100,-"

			rec := a.do(t, w.method, w.path, req{body: mine})
			require.Equal(t, http.StatusConflict, rec.Code, "%s %s with %q", w.method, w.path, seen)

			got := decode[struct {
				Errors  []problem `json:"errors"`
				Current eventJSON `json:"current"`
			}](t, rec)
			require.Equal(t, []problem{{Code: "conflict"}}, got.Errors)
			require.Equal(t, current, got.Current)
		}
	}

	// Keep mine: send again with theirs as the version seen.
	mine := julemarked(main)
	mine["updatedAt"], mine["priceText"] = current.UpdatedAt, "Kr 100,-"
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: mine})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Kr 100,-", decode[eventJSON](t, rec).PriceText)
}

// A copy shares its photos with the original. A form that loaded a photo
// before someone described it in the other event gets a conflict naming the
// photo, not a silent revert; an app that sends no version is not checked.
func TestAdminEventPhotoConflict(t *testing.T) {
	a := newAdmin(t, false)
	main := a.storedPhoto(t, 'a')

	orig := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(main)}))
	cp := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events/"+itoa(orig.ID)+"/copy", req{}))
	loaded := cp.MainPhoto.UpdatedAt
	require.NotEmpty(t, loaded)

	a.clock.set(a.clock.now().Add(time.Second))

	theirs := julemarked(main)
	theirs["updatedAt"] = orig.UpdatedAt
	theirs["mainPhoto"] = map[string]any{"id": main.ID, "alt": "Boder med julepynt", "photographerId": nil, "updatedAt": loaded}
	rec := a.do(t, "PUT", "/admin/api/events/"+itoa(orig.ID), req{body: theirs})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	described := decode[eventJSON](t, rec).MainPhoto

	a.clock.set(a.clock.now().Add(time.Second))

	mine := julemarked(main)
	mine["updatedAt"], mine["start"], mine["end"] = cp.UpdatedAt, "2027-11-13", "2027-11-14"
	mine["mainPhoto"] = map[string]any{"id": main.ID, "alt": "Boder i låven", "photographerId": nil, "updatedAt": loaded}
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(cp.ID), req{body: mine})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	got := decode[struct {
		Errors  []problem   `json:"errors"`
		Current eventJSON   `json:"current"`
		Photos  []photoJSON `json:"photos"`
	}](t, rec)
	require.Equal(t, []problem{{Code: "conflict", Field: "mainPhoto", PhotoID: main.ID}}, got.Errors)
	require.Equal(t, cp.ID, got.Current.ID)
	require.Equal(t, []photoJSON{*described}, got.Photos)

	// Keep mine: the photo's version from the conflict.
	mine["mainPhoto"].(map[string]any)["updatedAt"] = got.Photos[0].UpdatedAt
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(cp.ID), req{body: mine})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "Boder i låven", decode[eventJSON](t, rec).MainPhoto.Alt)

	// An app that does not send the version saves as before.
	a.clock.set(a.clock.now().Add(time.Second))
	cp = decode[eventJSON](t, a.do(t, "GET", "/admin/api/events/"+itoa(cp.ID), req{}))
	old := julemarked(main)
	old["updatedAt"], old["start"], old["end"] = cp.UpdatedAt, "2027-11-13", "2027-11-14"
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(cp.ID), req{body: old})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// A version that is no time is a malformed body.
	mine["updatedAt"] = decode[eventJSON](t, rec).UpdatedAt
	mine["mainPhoto"].(map[string]any)["updatedAt"] = "igår"
	rec = a.do(t, "PUT", "/admin/api/events/"+itoa(cp.ID), req{body: mine})
	require.Equal(t, []problem{{Code: "bad_request", Field: "mainPhoto", PhotoID: main.ID}}, errorsOf(t, rec))
}

// The app sends the stored slug back with every save; while it is the
// title's, a corrected title moves it, so a typo never becomes the address.
func TestAdminDraftSlugFollowsTheTitle(t *testing.T) {
	a := newAdmin(t, false)

	form := map[string]any{"updatedAt": "", "slug": "", "title": "Julemarkd", "start": "2026-11-14"}
	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: form}))
	require.Equal(t, "julemarkd", e.Slug)
	require.True(t, e.SlugFromTitle)

	form["updatedAt"], form["slug"], form["title"] = e.UpdatedAt, e.Slug, "Julemarked"
	e = decode[eventJSON](t, a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: form}))
	require.Equal(t, "/arrangementer/2026/julemarked", e.Path)

	form["updatedAt"], form["slug"] = e.UpdatedAt, "jul"
	e = decode[eventJSON](t, a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: form}))
	require.Equal(t, "jul", e.Slug)
	require.False(t, e.SlugFromTitle, "one she typed is hers")
}

func TestAdminEventPosterAndCaptions(t *testing.T) {
	a := newAdmin(t, false)
	main, extra, poster := a.storedPhoto(t, 'a'), a.storedPhoto(t, 'b'), a.storedPhoto(t, 'c')

	form := julemarked(main)
	captioned := ref(extra, "Boder i fjøset")
	captioned["caption"] = "Fjøset fylt av boder"
	form["extraPhotos"] = []any{captioned}
	form["poster"] = ref(poster, "Plakat for julemarkedet 14.–15. november")

	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: form}))
	require.Equal(t, poster.ID, e.Poster.ID)
	require.Equal(t, "Plakat for julemarkedet 14.–15. november", e.Poster.Alt)
	require.Equal(t, "Fjøset fylt av boder", e.ExtraPhotos[0].Caption)

	// An older form, without captions or a poster.
	delete(captioned, "caption")
	delete(form, "poster")
	form["updatedAt"] = e.UpdatedAt
	e = decode[eventJSON](t, a.do(t, "PUT", "/admin/api/events/"+itoa(e.ID), req{body: form}))
	require.Equal(t, "Fjøset fylt av boder", e.ExtraPhotos[0].Caption, "left out, the caption is kept")
	require.Nil(t, e.Poster)
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
