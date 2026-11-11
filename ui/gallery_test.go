package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/images"
)

var testGallery = []GalleryPhoto{
	{Photo: testPhoto(), Alt: "Låven pyntet til bryllup", By: Photographer{Name: "Lucky Thirteen", Website: "https://luckythirteen.no/"}},
	{Photo: testPhoto(images.JPEG), Alt: "Fjøset", By: Photographer{Name: "Sophia Karlsen Edwards"}},
	{Photo: testPhoto(images.WebP, images.JPEG), Alt: "Hagen"},
}

func TestGallery(t *testing.T) {
	golden.HTML(t, "gallery", Gallery(GalleryPage, testGallery...).Render())
	golden.HTML(t, "gallery-column", Gallery(GalleryColumn, testGallery[:2]...).Render())
}

func TestGalleryRefusesNoLayout(t *testing.T) {
	require.Panics(t, func() { Gallery(0, testGallery...) })
}

// The lightbox (Lightbox.elm photoDecoder) reads data-gallery; this pins the
// shape LightboxTest.elm decodes.
func TestGalleryData(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(Gallery(GalleryPage, testGallery...).Render()))
	require.NoError(t, err)

	var (
		data  string
		links []string
	)

	for n := range doc.Descendants() {
		for _, a := range n.Attr {
			switch a.Key {
			case "data-gallery":
				data = a.Val
			case "data-index":
				links = append(links, a.Val)
			}
		}
	}

	var got []lightboxPhoto
	require.NoError(t, json.Unmarshal([]byte(data), &got))

	jpeg := "/img/x-480.jpeg 480w, /img/x-800.jpeg 800w, /img/x-1200.jpeg 1200w, /img/x-1600.jpeg 1600w"
	want := []lightboxPhoto{
		{
			Alt: "Låven pyntet til bryllup", Width: 1600, Height: 2400, Src: "/img/x-1200.jpeg", Srcset: jpeg,
			Sources: []lightboxSource{
				{Type: "image/avif", Srcset: "/img/x-480.avif 480w, /img/x-800.avif 800w, /img/x-1200.avif 1200w, /img/x-1600.avif 1600w"},
				{Type: "image/webp", Srcset: "/img/x-480.webp 480w, /img/x-800.webp 800w, /img/x-1200.webp 1200w, /img/x-1600.webp 1600w"},
			},
			By: "Lucky Thirteen", Link: "https://luckythirteen.no/",
		},
		{
			Alt: "Fjøset", Width: 1600, Height: 2400, Src: "/img/x-1200.jpeg", Srcset: jpeg,
			Sources: []lightboxSource{}, By: "Sophia Karlsen Edwards",
		},
		{
			Alt: "Hagen", Width: 1600, Height: 2400, Src: "/img/x-1200.jpeg", Srcset: jpeg,
			Sources: []lightboxSource{{Type: "image/webp", Srcset: "/img/x-480.webp 480w, /img/x-800.webp 800w, /img/x-1200.webp 1200w, /img/x-1600.webp 1600w"}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("data-gallery (-want +got):\n%s", diff)
	}

	require.Equal(t, []string{"0", "1", "2"}, links, "data-index is each link's place in data-gallery")
	require.Contains(t, data, `"sources":[]`, "an empty list, never null")
}

// Without JavaScript the photo link is the gallery: it must lead to the
// largest photo in the format every browser shows.
func TestGalleryLinksToLargestPhoto(t *testing.T) {
	require.Contains(t, Gallery(GalleryPage, testGallery[0]).Render(), `<a class="block focus-visible:-outline-offset-4" data-index="0" href="/img/x-1600.jpeg">`)
}

func TestEmptyGalleryRendersNothing(t *testing.T) {
	require.Empty(t, Gallery(GalleryPage).Render())
}

func TestHoldsGallery(t *testing.T) {
	g := Gallery(GalleryPage, testGallery...)

	tests := []struct {
		name  string
		nodes []elem.Node
		want  bool
	}{
		{name: "none", nodes: []elem.Node{Container(Headline("x"))}},
		{name: "empty-gallery", nodes: []elem.Node{Gallery(GalleryPage)}},
		{name: "top-level", nodes: []elem.Node{g}, want: true},
		{name: "nested", nodes: []elem.Node{Container(elem.Div(nil, elem.Text("x"), g))}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, holdsGallery(tt.nodes))
		})
	}
}
