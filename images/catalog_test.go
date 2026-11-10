package images

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

// fakeCatalog stands in for the database in every test of this package.
type fakeCatalog struct {
	mu     sync.Mutex
	photos map[string]Photo
	err    error
	// onPhoto runs once, on the next Photo lookup: what the admin does
	// while the queue works.
	onPhoto func(sha string)
}

func newFakeCatalog() *fakeCatalog { return &fakeCatalog{photos: map[string]Photo{}} }

func (c *fakeCatalog) put(p Photo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.photos[p.SHA256] = p
}

func (c *fakeCatalog) remove(sha string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.photos, sha)
}

func (c *fakeCatalog) Photo(_ context.Context, sha string) (Photo, error) {
	c.mu.Lock()
	hook := c.onPhoto
	c.onPhoto = nil
	c.mu.Unlock()

	if hook != nil {
		hook(sha)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return Photo{}, c.err
	}

	p, ok := c.photos[sha]
	if !ok {
		return Photo{}, fmt.Errorf("photo %s: %w", sha, ErrNotFound)
	}

	return p, nil
}

func (c *fakeCatalog) Photos(context.Context) ([]Photo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}

	return slices.Collect(maps.Values(c.photos)), nil
}

func TestGroupsLadder(t *testing.T) {
	card := Rect{X: 0, Y: 0, W: 1000, H: 1250}
	banner := Rect{X: 0, Y: 0, W: 3000, H: 1687}
	share := Rect{X: 0, Y: 0, W: 1200, H: 630}
	p := Photo{
		SHA256: testSHA, MIME: "image/jpeg", Width: 3000, Height: 2000,
		Crops: map[Kind]Rect{Card: card, Banner: banner, Share: share},
	}

	got := map[Kind][]string{}
	for _, g := range groups(p) {
		for _, s := range g.specs {
			require.Equal(t, g.rect.ID(), s.Crop)
			got[g.kind] = append(got[g.kind], fmt.Sprintf("%s/%d", s.Format, s.Width))
		}
	}

	want := map[Kind][]string{
		// Never wider than the crop, and the crop's own width on top.
		Card:   ladder(480, 800, 1000),
		Banner: ladder(480, 800, 1200, 1600, 2000),
		Share:  {"jpeg/1200"},
		Full:   ladder(480, 800, 1200, 1600, 2000),
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("groups (-want +got):\n%s", d)
	}

	require.Len(t, Expected(p), 9+15+1+15)
}

func TestRungs(t *testing.T) {
	for _, tt := range []struct {
		crop int
		want []int
	}{
		{480, []int{480}},
		{500, []int{480}},             // within topSlack of 480
		{768, []int{480, 768}},        // the old site's small originals
		{1000, []int{480, 800, 1000}}, // between rungs
		{1210, []int{480, 800, 1200}}, // within topSlack of 1200
		{1824, []int{480, 800, 1200, 1600, 1824}},
		{2000, []int{480, 800, 1200, 1600, 2000}},
		{2106, []int{480, 800, 1200, 1600, 2000}}, // the ladder's top caps it
		{2400, []int{480, 800, 1200, 1600, 2000}},
	} {
		require.Equal(t, tt.want, rungs(tt.crop), "crop %d", tt.crop)
	}
}

// A photo between rungs is offered at its own width: the largest variant is
// the photo, so the page and the share image are as sharp as it allows.
func TestRefForPhotoBetweenRungs(t *testing.T) {
	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 768, Height: 1024}

	ref, err := RefFor(p, Full)
	require.NoError(t, err)
	require.Equal(t, 768, ref.Width)
	require.Equal(t, 1024, ref.Height)
	require.Len(t, ref.Variants, 6)

	// The crop tool takes it too.
	require.Contains(t, PreviewURL(p), "-768.jpg")
}

func ladder(ws ...int) []string {
	out := make([]string, 0, len(ws)*len(formats))
	for _, f := range formats {
		for _, w := range ws {
			out = append(out, fmt.Sprintf("%s/%d", f, w))
		}
	}

	return out
}

func TestGroupsSkipBadCrops(t *testing.T) {
	p := Photo{
		SHA256: testSHA, Width: 1000, Height: 800,
		Crops: map[Kind]Rect{
			Card:   {X: 0, Y: 0, W: 640, H: 800},
			Banner: {X: 0, Y: 0, W: 2000, H: 1125}, // bigger than the photo
			Share:  {X: 0, Y: 0, W: 1000, H: 525},  // under 1200
		},
	}

	got := make([]Kind, 0, len(kinds))
	for _, g := range groups(p) {
		got = append(got, g.kind)
	}

	require.Equal(t, []Kind{Full, Card}, got)
}

func TestRefFor(t *testing.T) {
	card := Rect{X: 10, Y: 20, W: 1300, H: 1625}
	p := Photo{SHA256: testSHA, Width: 1500, Height: 2000, Crops: map[Kind]Rect{Card: card}}

	ref, err := RefFor(p, Card)
	require.NoError(t, err)
	require.Equal(t, 1200, ref.Width)
	require.Equal(t, 1500, ref.Height)
	require.Len(t, ref.Variants, 9)
	require.Equal(t, Variant{
		Format: AVIF, Width: 480,
		URL: VariantSpec{Photo: testSHA, Kind: Card, Crop: card.ID(), Width: 480, Format: AVIF}.URL(),
	}, ref.Variants[0])
	require.Equal(t, JPEG, ref.Variants[8].Format)
	require.Equal(t, 1200, ref.Variants[8].Width)

	full, err := RefFor(p, Full)
	require.NoError(t, err)
	require.Equal(t, 1500, full.Width)
	require.Equal(t, 2000, full.Height)

	_, err = RefFor(p, Banner)
	require.ErrorIs(t, err, ErrNotFound)
}
