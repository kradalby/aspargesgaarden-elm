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
