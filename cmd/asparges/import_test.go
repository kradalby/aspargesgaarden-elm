package main

import (
	"log/slog"
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

// oldSite is a small copy of the old site, shared with e2e/gallery: files
// named as the real ones, with their quirks (trailing commas, a wrong date in
// a name, "ca."), and synthetic photos from its gen.sh.
const oldSite = "testdata/oldsite"

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
			BodyStart: strings.SplitN(e.BodyMD, "\n", 2)[0], ExtraPhotos: len(e.ExtraPhotos),
		}
		for _, d := range e.ExtraDates {
			got.Extra = append(got.Extra, d.String())
		}

		if p := e.MainPhoto; p != nil {
			got.Alt = p.Alt
			if p.Photographer != nil {
				got.Credit = p.Photographer.Name + " " + p.Photographer.URL
			}
		}

		out = append(out, got)
	}

	return out
}

func TestImport(t *testing.T) {
	store := runImport(t, oldSite, t.TempDir())

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
			// Its photo is too small to show: a draft until a larger one comes.
			Path: "/arrangementer/2026/varkonsert-med-black-lekker", Status: "draft", Start: "2026-05-19",
			StartTime: "19:30", BodyStart: "Velkommen til en stemningsfull vårkveld.",
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

	// The old menu's Julemarked entry, while the market is coming.
	menu, err := store.Menu(t.Context())
	require.NoError(t, err)
	require.Equal(t, []site.MenuEntry{{Label: "Julemarked", Path: "/arrangementer/2026/julemarked"}}, menu)

	gallery, err := store.Gallery(t.Context())
	require.NoError(t, err)

	alts := make([]string, len(gallery))
	for i, p := range gallery {
		alts[i] = p.Alt
	}

	// patrick was imported with the gourmet event first and keeps that alt;
	// liten, too small, is left out.
	require.Len(t, gallery, 12)
	require.Equal(t, []string{"", "Gourmetaften", "Blomster fra @bondenshage", "Låven pyntet til bryllup"}, alts[:4])
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
	root, data := oldSite, t.TempDir()
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
