package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// oldCheckout writes a small copy of the old site: files named as the real
// ones, with their quirks (trailing commas, a wrong date in a name, "ca.").
func oldCheckout(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()

		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	shade := uint8(0)
	photo := func(name string, w, h int) {
		t.Helper()

		shade += 20
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := range img.Pix {
			img.Pix[i] = shade
		}
		img.Set(0, 0, color.White)

		path := filepath.Join(root, "public", filepath.FromSlash(name)+".jpeg")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, jpeg.Encode(f, img, nil))
		require.NoError(t, f.Close())
	}

	write("content/events/2024-13-13-gourmet.md", `---
{
  "title": "Gourmetaften",
  "eventDate": "2024-10-01",
  "publishedFrom": "2024-09-01",
  "photo":
    {
      "path": "arrangementer/patrick",
      "photographer":
        { "name": "Lucky Thirteen", "website": "https://luckythirteen.no/" },
      "description": "",
    },
}
---

Aspargesgården inviterer til **gourmetaften**.
`)
	write("content/events/2026-10-31-takkebakstkurs.md", `---
{
  "title": "Takkebakstkurs med Anita Saga",
  "eventDate": "2026-10-31",
  "eventDates": ["2026-10-31", "2026-11-28", "2026-11-29"],
  "eventTime": "10:00",
  "eventEndDate": "2026-10-31",
  "eventEndTime": "ca. 15:00",
  "publishedFrom": "2026-10-10",
  "photo": {
    "path": "arrangementer/anita",
    "photographer": { "name": "", "website": "" },
    "description": "Anita Saga ved bakebordet"
  }
}
---

Invitasjon til takkebakstkurs.
`)
	write("content/events/2026-11-14-julemarked.md", `---
{
  "title": "Julemarked på Aspargesgården",
  "eventDate": "2026-11-14",
  "eventTime": "12:00",
  "eventEndDate": "2026-11-15",
  "eventEndTime": "16:00",
  "publishedFrom": "2026-10-10",
  "photo": {
    "path": "arrangementer/julemarked1",
    "photographer": { "name": "", "website": "" },
    "description": "Bålfat på gårdsplassen"
  }
}
---

[Les mer om julemarkedet og praktisk informasjon her](/christmas/2026)
`)
	write("content/events/2027-01-01-utkast.md", `---
{
  "title": "Nyttårskonsert",
  "eventDate": "2027-01-01",
  "eventTime": "etter middag",
  "photo": { "path": "arrangementer/patrick", "photographer": { "name": "", "website": "" }, "description": "" },
}
---

Kommer.
`)
	write("content/gallery.yaml", `---
photographers:
  lucky:
    name: Lucky Thirteen
    website: "https://luckythirteen.no/"
  sophia:
    name: Sophia Karlsen Edwards
    website: ""

photos:
  - path: galleri/b
    photographer: sophia
    description: ""
  - path: arrangementer/patrick
    photographer: lucky
    description: "Kokken"
  - path: galleri/a
    photographer: lucky
    description: "Blomster fra @bondenshage"
`)

	for _, p := range []string{
		"arrangementer/patrick", "arrangementer/anita", "arrangementer/julemarked1",
		"arrangementer/2026-11-14-julemarked", "arrangementer/julemarked2", "arrangementer/julemarked3", "arrangementer/julemarked4",
		"galleri/a",
	} {
		photo(p, 1300, 800)
	}

	// Too narrow for a banner or a share image; fine for the gallery.
	photo("galleri/b", 600, 900)

	return root
}

func runImport(t *testing.T, root, data string) *site.Store {
	t.Helper()

	d, err := db.Open(t.Context(), filepath.Join(data, "asparges.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	store := site.NewStore(d, func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, site.Oslo) })

	imgs, err := images.New(data, store.Catalog())
	require.NoError(t, err)

	im := importer{store: store, images: imgs, root: root, log: slog.New(slog.DiscardHandler)}
	require.NoError(t, im.run(t.Context()))

	return store
}

// imported is what the import is to produce, per event.
type imported struct {
	Path, Status       string
	Start, End         string
	StartTime, EndTime string
	Extra              []string
	BodyStart          string
	Alt, Credit        string
	ExtraPhotos        int
}

func summarize(t *testing.T, store *site.Store) []imported {
	t.Helper()

	events, err := store.Events(t.Context())
	require.NoError(t, err)

	out := make([]imported, 0, len(events))
	for _, e := range events {
		got := imported{
			Path: e.Path(), Status: string(e.Status),
			Start: e.Start.String(), End: e.End.String(), StartTime: e.StartTime, EndTime: e.EndTime,
			BodyStart: strings.SplitN(e.BodyMD, "\n", 2)[0], Alt: e.MainPhoto.Alt, ExtraPhotos: len(e.ExtraPhotos),
		}
		for _, d := range e.ExtraDates {
			got.Extra = append(got.Extra, d.String())
		}

		if e.MainPhoto.Photographer != nil {
			got.Credit = e.MainPhoto.Photographer.Name + " " + e.MainPhoto.Photographer.URL
		}

		out = append(out, got)
	}

	return out
}

func TestImport(t *testing.T) {
	store := runImport(t, oldCheckout(t), t.TempDir())

	want := []imported{
		{
			Path: "/arrangementer/2027/nyttarskonsert", Status: "draft", Start: "2027-01-01",
			// The same photo as the gourmet evening, imported first: one
			// photo, one alt text.
			BodyStart: "*Start: etter middag.*", Alt: "Gourmetaften", Credit: "Lucky Thirteen https://luckythirteen.no/",
		},
		{
			Path: "/arrangementer/2026/julemarked", Status: "published", Start: "2026-11-14", End: "2026-11-15",
			StartTime: "12:00", EndTime: "16:00", BodyStart: "# Velkommen til Julemarked på Aspargesgården!",
			Alt: "Bålfat på gårdsplassen", ExtraPhotos: 4,
		},
		{
			Path: "/arrangementer/2026/takkebakstkurs-med-anita-saga", Status: "published", Start: "2026-10-31",
			StartTime: "10:00", EndTime: "15:00", Extra: []string{"2026-11-28", "2026-11-29"},
			BodyStart: "*Slutt ca. kl. 15:00.*", Alt: "Anita Saga ved bakebordet",
		},
		{
			// The file says 2024-13-13; the page said 1 October.
			Path: "/arrangementer/2024/gourmetaften", Status: "published", Start: "2024-10-01",
			BodyStart: "Aspargesgården inviterer til **gourmetaften**.", Alt: "Gourmetaften",
			Credit: "Lucky Thirteen https://luckythirteen.no/",
		},
	}

	if diff := cmp.Diff(want, summarize(t, store)); diff != "" {
		t.Errorf("imported events (-want +got):\n%s", diff)
	}

	gallery, err := store.Gallery(t.Context())
	require.NoError(t, err)

	alts := make([]string, len(gallery))
	for i, p := range gallery {
		alts[i] = p.Alt
	}

	// patrick was imported with the gourmet event first and keeps that alt.
	require.Equal(t, []string{"", "Gourmetaften", "Blomster fra @bondenshage"}, alts)
	require.Equal(t, "Sophia Karlsen Edwards", gallery[0].Photographer.Name)

	// Centred crops of every kind the photo is large enough for; the narrow
	// one gets a card only and is shown whole elsewhere.
	require.Equal(t, map[site.CropKind]site.Rect{site.CropCard: {X: 0, Y: 75, W: 600, H: 750}}, gallery[0].Crops)
	require.Equal(t, map[site.CropKind]site.Rect{
		site.CropCard:   {X: 330, Y: 0, W: 640, H: 800},
		site.CropBanner: {X: 0, Y: 34, W: 1300, H: 731},
		site.CropShare:  {X: 0, Y: 59, W: 1300, H: 682},
	}, gallery[2].Crops)

	photographers, err := store.Photographers(t.Context())
	require.NoError(t, err)
	require.Len(t, photographers, 2)
}

// Running the import again changes nothing, and keeps what the admin has
// changed since.
func TestImportIsIdempotent(t *testing.T) {
	root, data := oldCheckout(t), t.TempDir()
	store := runImport(t, root, data)
	ctx := t.Context()

	events, err := store.Events(ctx)
	require.NoError(t, err)

	edited := events[0]
	edited.Title = "Endret i admin"
	_, err = store.UpdateEvent(ctx, edited)
	require.NoError(t, err)

	before, err := store.Events(ctx)
	require.NoError(t, err)
	beforeGallery, err := store.Gallery(ctx)
	require.NoError(t, err)
	beforePhotos, err := store.Photos(ctx)
	require.NoError(t, err)

	again := runImport(t, root, data)

	after, err := again.Events(ctx)
	require.NoError(t, err)
	afterGallery, err := again.Gallery(ctx)
	require.NoError(t, err)
	afterPhotos, err := again.Photos(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(before, after))
	require.Empty(t, cmp.Diff(beforeGallery, afterGallery))
	require.Empty(t, cmp.Diff(beforePhotos, afterPhotos))
	require.Equal(t, "Endret i admin", after[0].Title)
}

func TestParseOldEventNeedsFrontmatter(t *testing.T) {
	_, _, err := parseOldEvent([]byte("Bare tekst"))
	require.ErrorIs(t, err, errNoFrontmatter)
}

func TestLegacyTime(t *testing.T) {
	tests := []struct {
		in, want, note string
	}{
		{in: "", want: ""},
		{in: "19:00", want: "19:00"},
		{in: "kl. 19", want: "19:00"},
		{in: "ca. 15:00", want: "15:00", note: "Slutt ca. kl. 15:00."},
		{in: "etter middag", want: "", note: "Slutt: etter middag."},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, notes := legacyTime(tt.in, "Slutt", nil)

			require.Equal(t, tt.want, got)
			require.Equal(t, tt.note, strings.Join(notes, ""))
		})
	}
}
