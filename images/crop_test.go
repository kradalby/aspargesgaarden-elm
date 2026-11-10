package images

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCrop(t *testing.T) {
	const w, h = 4000, 3000

	tests := []struct {
		name string
		kind Kind
		r    Rect
		want error
	}{
		{"card exact", Card, Rect{100, 100, 800, 1000}, nil},
		{"card rounded by the cropper", Card, Rect{100, 100, 801, 1000}, nil},
		{"card one percent off", Card, Rect{0, 0, 2000, 2525}, nil},
		{"card two percent off", Card, Rect{0, 0, 2000, 2550}, ErrCropAspect},
		{"card square", Card, Rect{0, 0, 1000, 1000}, ErrCropAspect},
		{"card at the corner", Card, Rect{4000 - 2000, 3000 - 2500, 2000, 2500}, nil},
		{"card one past the right edge", Card, Rect{4000 - 1999, 0, 2000, 2500}, ErrCropOutside},
		{"card one past the bottom", Card, Rect{0, 501, 2000, 2500}, ErrCropOutside},
		{"negative x", Card, Rect{-1, 0, 800, 1000}, ErrCropOutside},
		{"zero size", Card, Rect{0, 0, 0, 0}, ErrCropOutside},
		{"card short by the slack at full height", Card, Rect{0, 0, 2424, 3000}, ErrCropOutside},
		{"card under the minimum", Card, Rect{0, 0, 476, 595}, ErrTooSmall},
		{"card at the minimum", Card, Rect{0, 0, 480, 600}, nil},
		{"banner", Banner, Rect{0, 0, 1600, 900}, nil},
		{"banner under the minimum", Banner, Rect{0, 0, 784, 441}, ErrTooSmall},
		{"share exact", Share, Rect{10, 10, 1200, 630}, nil},
		{"share large", Share, Rect{0, 0, 4000, 2100}, nil},
		{"share too small", Share, Rect{0, 0, 1160, 609}, ErrTooSmall},
		{"full takes no crop", Full, Rect{0, 0, w, h}, ErrKind},
		{"unknown kind", Kind("poster"), Rect{0, 0, 800, 1000}, ErrKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCrop(tt.kind, w, h, tt.r)
			if tt.want == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestDefaultCrop(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		w, h int
		want Rect
		err  error
	}{
		{"card from landscape", Card, 1200, 800, Rect{280, 0, 640, 800}, nil},
		{"card from portrait", Card, 3000, 4000, Rect{0, 125, 3000, 3750}, nil},
		{"banner from landscape", Banner, 1200, 800, Rect{0, 62, 1200, 675}, nil},
		{"banner from portrait", Banner, 3000, 4000, Rect{0, 1156, 3000, 1687}, nil},
		{"share fits exactly", Share, 1200, 800, Rect{0, 85, 1200, 630}, nil},
		{"share from a small photo", Share, 1000, 800, Rect{}, ErrTooSmall},
		{"share from a tall narrow photo", Share, 1199, 4000, Rect{}, ErrTooSmall},
		{"card from a short wide photo", Card, 4000, 590, Rect{}, ErrTooSmall},
		{"full", Full, 1200, 800, Rect{0, 0, 1200, 800}, nil},
		{"full too narrow", Full, 479, 800, Rect{}, ErrTooSmall},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DefaultCrop(tt.kind, tt.w, tt.h)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if tt.kind != Full {
				require.NoError(t, ValidateCrop(tt.kind, tt.w, tt.h, got))
			}
		})
	}
}
