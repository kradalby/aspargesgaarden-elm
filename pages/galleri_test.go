package pages

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

var lucky = &site.Photographer{ID: 1, Name: "Lucky Thirteen", URL: "https://luckythirteen.no/"}

// Two photos as the import leaves them: one described, one not.
var testGalleryPhotos = []site.Photo{
	{ID: 1, SHA256: strings.Repeat("a", 64), MIME: "image/jpeg", Width: 5940, Height: 3952, Alt: "Blomster fra @bondenshage", Photographer: lucky},
	{ID: 2, SHA256: strings.Repeat("b", 64), MIME: "image/jpeg", Width: 3888, Height: 2592},
}

func TestGalleri(t *testing.T) {
	golden.HTML(t, "galleri", Galleri(testSite, testGalleryPhotos).Render())
}

func TestGalleryPhotos(t *testing.T) {
	got := galleryPhotos(testGalleryPhotos)

	type shown struct {
		Alt   string
		By    ui.Photographer
		Width int
	}

	gotShown := make([]shown, 0, len(got))
	for _, g := range got {
		gotShown = append(gotShown, shown{g.Alt, g.By, g.Photo.Width})
	}

	want := []shown{
		{"Blomster fra @bondenshage", ui.Photographer{Name: "Lucky Thirteen", Website: "https://luckythirteen.no/"}, 2000},
		{fallbackAlt, ui.Photographer{}, 2000},
	}
	if diff := cmp.Diff(want, gotShown); diff != "" {
		t.Errorf("gallery photos (-want +got):\n%s", diff)
	}

	require.Contains(t, got[0].Photo.Variants[0].URL, "/full-", "the uncropped kind")
}
