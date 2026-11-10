package site

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckCrop(t *testing.T) {
	p := Photo{Width: 4000, Height: 3000}

	require.NoError(t, p.CheckCrop(CropCard, Rect{X: 0, Y: 0, W: 2400, H: 3000}))
	require.NoError(t, p.CheckCrop(CropBanner, Rect{X: 0, Y: 750, W: 4000, H: 2250}))

	tests := []struct {
		name string
		kind CropKind
		r    Rect
		want error
	}{
		{"unknown kind", "full", Rect{W: 10, H: 10}, ErrCropKind},
		{"negative", CropCard, Rect{X: -1, W: 10, H: 10}, ErrCropOutside},
		{"empty", CropCard, Rect{W: 0, H: 10}, ErrCropOutside},
		{"past right edge", CropShare, Rect{X: 3000, W: 1001, H: 10}, ErrCropOutside},
		{"past bottom edge", CropShare, Rect{Y: 2000, W: 10, H: 1001}, ErrCropOutside},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, p.CheckCrop(tt.kind, tt.r), tt.want)
		})
	}
}

func TestPhotographerNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      Photographer
		want    Photographer
		wantErr error
	}{
		{"tidy", Photographer{Name: "Anita Saga"}, Photographer{Name: "Anita Saga"}, nil},
		{
			"spaces collapse",
			Photographer{Name: "  Anita \t Saga ", URL: " https://anitasaga.no/ "},
			Photographer{Name: "Anita Saga", URL: "https://anitasaga.no/"},
			nil,
		},
		{"no name", Photographer{Name: " "}, Photographer{}, ErrPhotographerName},
		{"other scheme", Photographer{Name: "A", URL: "javascript:alert(1)"}, Photographer{Name: "A", URL: "javascript:alert(1)"}, ErrPhotographerURL},
		{"no host", Photographer{Name: "A", URL: "https://"}, Photographer{Name: "A", URL: "https://"}, ErrPhotographerURL},
		{"bare host", Photographer{Name: "A", URL: "anitasaga.no"}, Photographer{Name: "A", URL: "anitasaga.no"}, ErrPhotographerURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Normalize()
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNameKey(t *testing.T) {
	require.Equal(t, "anita saga", nameKey(" ANITA   Saga"))
	require.Equal(t, nameKey("Øyvind Ås"), nameKey("øyvind ås"), "Norwegian letters fold too")
}
