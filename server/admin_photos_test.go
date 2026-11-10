package server

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// withVips puts the real photo store behind the admin, for uploads, and
// returns its folder.
func (a *adminFixture) withVips(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	imgs, err := images.New(dir, a.store.Catalog())
	require.NoError(t, err, "the tests run the real vips: it must be on PATH")

	a.srv.cfg.Admin.Photos = imgs
	a.admin = a.srv.Admin()

	return dir
}

func multipartBody(t *testing.T, name string, data []byte) ([]byte, string) {
	t.Helper()

	var b bytes.Buffer

	mw := multipart.NewWriter(&b)
	require.NoError(t, mw.WriteField("note", "ignored"))

	fw, err := mw.CreateFormFile("file", name)
	require.NoError(t, err)

	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	return b.Bytes(), mw.FormDataContentType()
}

func (a *adminFixture) upload(t *testing.T, name string, data []byte) *httptest.ResponseRecorder {
	t.Helper()

	body, ctype := multipartBody(t, name, data)

	return a.do(t, "POST", "/admin/api/photos", req{body: body, ctype: ctype})
}

func testdata(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("..", "images", "testdata", name))
	require.NoError(t, err)

	return b
}

func grayPNG(t *testing.T, w, h int) []byte {
	t.Helper()

	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))))

	return b.Bytes()
}

func TestAdminUpload(t *testing.T) {
	a := newAdmin(t, false)
	a.withVips(t)

	rec := a.upload(t, "julemarked.jpg", testdata(t, "upright.jpg"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	p := decode[photoJSON](t, rec)
	require.Equal(t, 1200, p.Width)
	require.Equal(t, 800, p.Height)
	require.Contains(t, p.PreviewURL, "/full-")
	require.Contains(t, p.ThumbURL, "-480.jpg")
	require.True(t, p.Status.PreviewReady, "Intake builds the preview while it checks the file")
	require.Equal(t, map[string]rectJSON{
		"card":   {X: 280, Y: 0, W: 640, H: 800},
		"banner": {X: 0, Y: 62, W: 1200, H: 675},
		"share":  {X: 0, Y: 85, W: 1200, H: 630},
	}, p.Crops)

	stored, err := a.store.Photo(t.Context(), p.ID)
	require.NoError(t, err)
	require.Len(t, stored.Crops, 3, "default crops for every kind it is large enough for")
	require.Contains(t, a.images.enqueued, stored.SHA256)

	// The same bytes again: the same photo.
	rec = a.upload(t, "igjen.jpg", testdata(t, "upright.jpg"))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, p.ID, decode[photoJSON](t, rec).ID)

	// Too narrow for a share image: that crop is left unset, and the app
	// gets the centred rectangle to start from.
	rec = a.upload(t, "smal.png", grayPNG(t, 1000, 700))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	small := decode[photoJSON](t, rec)
	stored, err = a.store.Photo(t.Context(), small.ID)
	require.NoError(t, err)
	require.NotContains(t, stored.Crops, images.Share)
	require.Contains(t, small.Crops, "share")
}

func TestAdminUploadRefused(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		status int
		code   string
	}{
		{"text", []byte("hei, dette er tekst"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"empty", nil, http.StatusUnsupportedMediaType, "unsupported_type"},
		{"gif", nil, http.StatusUnsupportedMediaType, "unsupported_type"},
		{"truncated", nil, http.StatusUnprocessableEntity, "damaged"},
		{"pixel bomb", nil, http.StatusUnprocessableEntity, "too_many_pixels"},
		{"too small", nil, http.StatusUnprocessableEntity, "too_small"},
	}

	files := map[string]string{"gif": "animated.gif", "truncated": "truncated.jpg", "pixel bomb": "bomb-dims.jpg"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAdmin(t, false)
			imgs := a.withVips(t)

			data := tt.data
			if f, ok := files[tt.name]; ok {
				data = testdata(t, f)
			}

			if tt.name == "too small" {
				data = grayPNG(t, 300, 200)
			}

			rec := a.upload(t, "x.jpg", data)
			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			require.Equal(t, []problem{{Code: tt.code}}, errorsOf(t, rec))

			ps, err := a.store.Photos(t.Context())
			require.NoError(t, err)
			require.Empty(t, ps, "nothing recorded")

			for _, d := range []string{"originals", "tmp"} {
				entries, err := os.ReadDir(filepath.Join(imgs, d))
				require.NoError(t, err)
				require.Empty(t, entries, "nothing left in %s", d)
			}
		})
	}

	a := newAdmin(t, false)
	require.Equal(t, http.StatusBadRequest, a.do(t, "POST", "/admin/api/photos", req{body: "x", ctype: "text/plain"}).Code)

	body, ctype := multipartBody(t, "x", nil)
	body = bytes.Replace(body, []byte(`name="file"`), []byte(`name="other"`), 1)
	rec := a.do(t, "POST", "/admin/api/photos", req{body: body, ctype: ctype})
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// The limit holds before anything is buffered: by the declared length at
// once, and while streaming for a body that declares none.
func TestAdminUploadLimit(t *testing.T) {
	a := newAdmin(t, false)
	a.withVips(t)

	rq := httptest.NewRequestWithContext(t.Context(), "POST", "/admin/api/photos", strings.NewReader(""))
	rq.RemoteAddr, rq.Host = kristine, adminHost
	rq.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rq.ContentLength = maxUploadBody + 1

	rec := httptest.NewRecorder()
	a.admin.ServeHTTP(rec, rq)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Equal(t, []problem{{Code: "too_large"}}, errorsOf(t, rec))

	head, ctype := multipartBody(t, "stor.jpg", []byte{0xff, 0xd8, 0xff})
	cut := bytes.LastIndex(head, []byte("\r\n--"))
	stream := io.MultiReader(bytes.NewReader(head[:cut]), io.LimitReader(zeros{}, images.MaxUpload), bytes.NewReader(head[cut:]))

	rq = httptest.NewRequestWithContext(t.Context(), "POST", "/admin/api/photos", stream)
	rq.RemoteAddr, rq.Host = kristine, adminHost
	rq.Header.Set("Content-Type", ctype)
	rq.ContentLength = -1

	rec = httptest.NewRecorder()
	a.admin.ServeHTTP(rec, rq)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Equal(t, []problem{{Code: "too_large"}}, errorsOf(t, rec))
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestAdminPhoto(t *testing.T) {
	a := newAdmin(t, false)
	p := a.storedPhoto(t, 'a')
	path := "/admin/api/photos/" + itoa(p.ID)

	got := decode[photoJSON](t, a.do(t, "GET", path, req{}))
	require.Equal(t, p.ID, got.ID)
	require.Nil(t, got.Photographer)
	require.Equal(t, rectJSON{X: 0, Y: 150, W: 1600, H: 900}, got.Crops["banner"], "unset kinds show the centred default")

	ola := decode[photographerJSON](t, a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "Ola", "url": ""}}))

	rec := a.do(t, "PUT", path, req{body: map[string]any{"alt": "", "photographerId": ola.ID}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, &ola, decode[photoJSON](t, rec).Photographer, "an empty alt is allowed here")

	rec = a.do(t, "PUT", path, req{body: map[string]any{"alt": "Hagen", "photographerId": 999}})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, []problem{{Code: "not_found", Field: "photographerId", PhotoID: p.ID}}, errorsOf(t, rec))

	// Not on a photo a published event shows.
	rec = a.do(t, "PUT", path, req{body: map[string]any{"alt": "Hagen", "photographerId": nil}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(p)}))
	require.Equal(t, http.StatusOK, a.do(t, "POST", "/admin/api/events/"+itoa(e.ID)+"/publish", req{body: map[string]string{"updatedAt": e.UpdatedAt}}).Code)

	rec = a.do(t, "PUT", path, req{body: map[string]any{"alt": "", "photographerId": nil}})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	require.Equal(t, []problem{{Code: "alt_required", Field: "alt", PhotoID: p.ID}}, errorsOf(t, rec))

	for _, path := range []string{"/admin/api/photos/999", "/admin/api/photos/x", "/admin/api/photos/-1"} {
		require.Equal(t, http.StatusNotFound, a.do(t, "GET", path, req{}).Code, path)
	}
}

func TestAdminCrops(t *testing.T) {
	a := newAdmin(t, false)
	p := a.storedPhoto(t, 'a') // 1600×1200
	path := "/admin/api/photos/" + itoa(p.ID) + "/crops"

	tests := []struct {
		name string
		body map[string]rectJSON
		want []problem
	}{
		{"unknown kind", map[string]rectJSON{"poster": {0, 0, 800, 1000}}, []problem{{Code: "crop_kind", Field: "poster"}}},
		{"full takes none", map[string]rectJSON{"full": {0, 0, 1600, 1200}}, []problem{{Code: "crop_kind", Field: "full"}}},
		{"outside", map[string]rectJSON{"card": {900, 0, 800, 1000}}, []problem{{Code: "crop_outside", Field: "card"}}},
		{"aspect", map[string]rectJSON{"banner": {0, 0, 1600, 1000}}, []problem{{Code: "crop_aspect", Field: "banner"}}},
		{"too small", map[string]rectJSON{"share": {0, 0, 800, 420}}, []problem{{Code: "crop_too_small", Field: "share"}}},
		{"one bad, none written", map[string]rectJSON{
			"card":   {0, 0, 800, 1000},
			"banner": {0, 0, 1600, 1000},
		}, []problem{{Code: "crop_aspect", Field: "banner"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := a.do(t, "PUT", path, req{body: tt.body})
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Equal(t, tt.want, errorsOf(t, rec))

			stored, err := a.store.Photo(t.Context(), p.ID)
			require.NoError(t, err)
			require.Equal(t, p.Crops, stored.Crops, "a refused change writes nothing")
		})
	}

	before := len(a.images.enqueued)
	rec := a.do(t, "PUT", path, req{body: map[string]rectJSON{"card": {100, 0, 960, 1200}, "share": {0, 0, 1600, 840}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decode[photoJSON](t, rec)
	require.Equal(t, rectJSON{100, 0, 960, 1200}, got.Crops["card"])
	require.Equal(t, []string{p.SHA256}, a.images.enqueued[before:], "the new crop's sizes are built now")

	stored, err := a.store.Photo(t.Context(), p.ID)
	require.NoError(t, err)
	require.Equal(t, images.Rect{X: 100, Y: 0, W: 960, H: 1200}, stored.Crops[images.Card])

	// Too narrow for a share image: the app sends back the centred share
	// rectangle it was shown, which is accepted and not stored; any other
	// share rectangle is refused.
	narrow, _, err := a.store.AddPhoto(t.Context(), site.Photo{SHA256: strings.Repeat("e", 64), MIME: "image/jpeg", Width: 1000, Height: 1219})
	require.NoError(t, err)

	shown := decode[photoJSON](t, a.do(t, "GET", "/admin/api/photos/"+itoa(narrow.ID), req{})).Crops
	path = "/admin/api/photos/" + itoa(narrow.ID) + "/crops"

	rec = a.do(t, "PUT", path, req{body: map[string]rectJSON{"card": {0, 0, 960, 1200}, "share": shown["share"]}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	stored, err = a.store.Photo(t.Context(), narrow.ID)
	require.NoError(t, err)
	require.Equal(t, images.Rect{X: 0, Y: 0, W: 960, H: 1200}, stored.Crops[images.Card])
	require.NotContains(t, stored.Crops, images.Share)

	rec = a.do(t, "PUT", path, req{body: map[string]rectJSON{"share": {0, 0, 950, 499}}})
	require.Equal(t, []problem{{Code: "crop_too_small", Field: "share"}}, errorsOf(t, rec))
}

// A crop the photo already has is never re-judged: sent back unchanged it is
// accepted, even under today's floor, so the other kinds can still be saved.
func TestAdminCropsKeepAnOldCrop(t *testing.T) {
	a := newAdmin(t, false)
	p := a.storedPhoto(t, 'a') // 1600×1200
	old := images.Rect{X: 10, Y: 20, W: 700, H: 394}
	_, err := a.store.SetCrops(t.Context(), p.ID, map[images.Kind]images.Rect{images.Banner: old})
	require.NoError(t, err)

	path := "/admin/api/photos/" + itoa(p.ID) + "/crops"
	rec := a.do(t, "PUT", path, req{body: map[string]rectJSON{"banner": {10, 20, 700, 394}, "card": {0, 0, 600, 750}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	stored, err := a.store.Photo(t.Context(), p.ID)
	require.NoError(t, err)
	require.Equal(t, old, stored.Crops[images.Banner])
	require.Equal(t, images.Rect{X: 0, Y: 0, W: 600, H: 750}, stored.Crops[images.Card],
		"the soft card is accepted too: sharpness only warns")

	// Moved, it is a new crop, and today's floor applies.
	rec = a.do(t, "PUT", path, req{body: map[string]rectJSON{"banner": {11, 20, 700, 394}}})
	require.Equal(t, []problem{{Code: "crop_too_small", Field: "banner"}}, errorsOf(t, rec))
}

// The app learns each kind's floor and sharp width, and whether what the
// pages draw for the kind is narrower than sharp.
func TestAdminPhotoQuality(t *testing.T) {
	a := newAdmin(t, false)

	small, _, err := a.store.AddPhoto(t.Context(), site.Photo{SHA256: strings.Repeat("c", 64), MIME: "image/jpeg", Width: 768, Height: 1024})
	require.NoError(t, err)
	_, err = a.store.SetCrops(t.Context(), small.ID, map[images.Kind]images.Rect{images.Card: {X: 0, Y: 32, W: 768, H: 960}})
	require.NoError(t, err)

	large := a.storedPhoto(t, 'a') // 1600×1200
	_, err = a.store.SetCrops(t.Context(), large.ID, map[images.Kind]images.Rect{
		images.Card: {X: 0, Y: 0, W: 600, H: 750},
		// Under the floor, as an old import may have it: the pages skip it
		// and draw the whole photo, which is sharp.
		images.Banner: {X: 0, Y: 0, W: 700, H: 394},
		images.Share:  {X: 0, Y: 0, W: 1600, H: 840},
	})
	require.NoError(t, err)

	tests := []struct {
		name  string
		photo site.Photo
		soft  map[string]bool
	}{
		// The banner is the whole photo, 768 wide; there is no share image.
		{"a photo sent on Messenger", small, map[string]bool{"card": false, "banner": true, "share": false, "full": true}},
		{"a soft card", large, map[string]bool{"card": true, "banner": false, "share": false, "full": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decode[photoJSON](t, a.do(t, "GET", "/admin/api/photos/"+itoa(tt.photo.ID), req{})).Quality

			want := map[string]qualityJSON{
				"card":   {MinWidth: 480, SharpWidth: 768, Soft: tt.soft["card"]},
				"banner": {MinWidth: 800, SharpWidth: 896, Soft: tt.soft["banner"]},
				"share":  {MinWidth: 1200, SharpWidth: 1200, Soft: tt.soft["share"]},
				"full":   {MinWidth: 480, SharpWidth: 992, Soft: tt.soft["full"]},
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("quality (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAdminPhotoStatus(t *testing.T) {
	a := newAdmin(t, false)
	ready := a.storedPhoto(t, 'a')
	building := a.storedPhoto(t, 'b')
	broken := a.storedPhoto(t, 'c')
	notPhoto := a.storedPhoto(t, 'd')
	gone := a.storedPhoto(t, 'e')

	a.photos.status[building.SHA256] = images.Status{State: images.StateQueued, Ready: 0, Total: 0}
	a.photos.status[broken.SHA256] = images.Status{State: images.StateFailed, Ready: 3, Total: 40, Reason: images.ReasonDamaged, Preview: "/img/x"}
	a.photos.status[notPhoto.SHA256] = images.Status{State: images.StateFailed, Reason: images.ReasonNotPhoto}
	a.photos.status[gone.SHA256] = images.Status{State: images.StateFailed, Reason: images.ReasonMissingOriginal}

	ids := []int64{ready.ID, building.ID, broken.ID, notPhoto.ID, gone.ID, 999}
	q := make([]string, len(ids))
	for i, id := range ids {
		q[i] = itoa(id)
	}

	rec := a.do(t, "GET", "/admin/api/photos/status?ids="+strings.Join(q, ","), req{})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, map[string]map[string]statusJSON{"statuses": {
		itoa(ready.ID):    {State: "ready", Done: 40, Total: 40, PreviewReady: true},
		itoa(building.ID): {State: "preparing"},
		itoa(broken.ID):   {State: "failed", Done: 3, Total: 40, PreviewReady: true, Reason: "damaged"},
		// The upload's word for it, not the photo store's.
		itoa(notPhoto.ID): {State: "failed", Reason: "unsupported_type"},
		// Gone from disk: uploading the same file again repairs it.
		itoa(gone.ID): {State: "failed", Reason: "missing_original"},
	}}, decode[map[string]map[string]statusJSON](t, rec))

	tooMany := strings.Repeat("1,", maxStatusIDs+1)
	require.Equal(t, http.StatusBadRequest, a.do(t, "GET", "/admin/api/photos/status?ids="+tooMany, req{}).Code)
	require.Equal(t, http.StatusBadRequest, a.do(t, "GET", "/admin/api/photos/status?ids=a", req{}).Code)

	// Prepare starts a failed photo again, and leaves the others be.
	rec = a.do(t, "POST", "/admin/api/photos/"+itoa(broken.ID)+"/prepare", req{})
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, []string{broken.SHA256}, a.photos.retried)

	rec = a.do(t, "POST", "/admin/api/photos/"+itoa(ready.ID)+"/prepare", req{})
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "ready", decode[statusJSON](t, rec).State)
	require.Len(t, a.photos.retried, 1)
}

func TestAdminDeletePhoto(t *testing.T) {
	a := newAdmin(t, false)
	inGallery := a.storedPhoto(t, 'a')
	inEvent := a.storedPhoto(t, 'b')
	require.NoError(t, a.store.SetGallery(t.Context(), []int64{inGallery.ID, inEvent.ID}))

	e := decode[eventJSON](t, a.do(t, "POST", "/admin/api/events", req{body: julemarked(inEvent)}))

	rec := a.do(t, "DELETE", "/admin/api/photos/"+itoa(inEvent.ID), req{})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.JSONEq(t, `{"errors":[{"code":"photo_in_use"}],"usedBy":[{"id":`+itoa(e.ID)+`,"title":"Julemarked 2026"}]}`, rec.Body.String())

	require.Equal(t, http.StatusNoContent, a.do(t, "DELETE", "/admin/api/photos/"+itoa(inGallery.ID), req{}).Code)
	require.Contains(t, a.images.enqueued, inGallery.SHA256, "the photo store removes its files")
	require.Equal(t, http.StatusNotFound, a.do(t, "DELETE", "/admin/api/photos/"+itoa(inGallery.ID), req{}).Code)

	g := decode[map[string][]photoJSON](t, a.do(t, "GET", "/admin/api/gallery", req{}))
	require.Len(t, g["photos"], 1)
	require.Equal(t, inEvent.ID, g["photos"][0].ID)
}

func TestAdminPhotographers(t *testing.T) {
	a := newAdmin(t, false)

	rec := a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "  Ola   Nordmann ", "url": "https://example.no"}})
	require.Equal(t, http.StatusCreated, rec.Code)

	ola := decode[photographerJSON](t, rec)
	require.Equal(t, "Ola Nordmann", ola.Name)

	rec = a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "ola nordmann", "url": ""}})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, map[string]any{
		"errors":   []any{map[string]any{"code": "photographer_name_taken", "field": "name"}},
		"existing": map[string]any{"id": float64(ola.ID), "name": "Ola Nordmann", "url": "https://example.no"},
	}, decode[map[string]any](t, rec))

	// The same name in another Unicode spelling (A and a combining ring).
	ase := decode[photographerJSON](t, a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "Åse Berg"}}))
	rec = a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "Åse berg"}})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, ase, decode[struct {
		Existing photographerJSON `json:"existing"`
	}](t, rec).Existing)

	rec = a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": " ", "url": "instagram.com/x"}})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, []problem{{Code: "photographer_name_required", Field: "name"}, {Code: "photographer_url", Field: "url"}}, errorsOf(t, rec))

	anna := decode[photographerJSON](t, a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "anna"}}))

	rec = a.do(t, "PUT", "/admin/api/photographers/"+itoa(anna.ID), req{body: map[string]string{"name": "Anna Foto", "url": "https://anna.no"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, photographerJSON{ID: anna.ID, Name: "Anna Foto", URL: "https://anna.no"}, decode[photographerJSON](t, rec))

	require.Equal(t, http.StatusNotFound, a.do(t, "PUT", "/admin/api/photographers/999", req{body: map[string]string{"name": "X"}}).Code)

	list := decode[map[string][]photographerJSON](t, a.do(t, "GET", "/admin/api/photographers", req{}))
	require.Equal(t, []string{"Anna Foto", "Ola Nordmann"}, []string{list["photographers"][0].Name, list["photographers"][1].Name})
}

func TestAdminGallery(t *testing.T) {
	a := newAdmin(t, false)
	p1, p2 := a.storedPhoto(t, 'a'), a.storedPhoto(t, 'b')

	rec := a.do(t, "PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{p2.ID, p1.ID}}})
	require.Equal(t, http.StatusOK, rec.Code)

	got := decode[map[string][]photoJSON](t, rec)["photos"]
	require.Equal(t, []int64{p2.ID, p1.ID}, []int64{got[0].ID, got[1].ID})

	rec = a.do(t, "PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{p1.ID, 999}}})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, []problem{{Code: "not_found", PhotoID: 999}}, errorsOf(t, rec))

	rec = a.do(t, "PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{p1.ID, p1.ID}}})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, []problem{{Code: "duplicate_photo", PhotoID: p1.ID}}, errorsOf(t, rec))

	rec = a.do(t, "PUT", "/admin/api/gallery", req{body: map[string]any{}})
	require.Equal(t, http.StatusBadRequest, rec.Code, "a body without the list does not empty the gallery")

	got = decode[map[string][]photoJSON](t, a.do(t, "GET", "/admin/api/gallery", req{}))["photos"]
	require.Len(t, got, 2, "refused writes changed nothing")

	rec = a.do(t, "PUT", "/admin/api/gallery", req{body: map[string]any{"photoIds": []int64{}}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, decode[map[string][]photoJSON](t, rec)["photos"])
}
