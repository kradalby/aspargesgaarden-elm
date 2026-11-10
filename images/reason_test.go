package images

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReasonOf(t *testing.T) {
	_, statErr := os.Stat("/nonexistent/original.jpg")

	tests := []struct {
		err  error
		want Reason
	}{
		{nil, ReasonNone},
		{fmt.Errorf("upload: %w", ErrTooLarge), ReasonTooLarge},
		{fmt.Errorf("%w: gifload", ErrUnsupported), ReasonNotPhoto},
		{errors.Join(errors.New("other"), fmt.Errorf("decode: %w", ErrBroken)), ReasonDamaged},
		{ErrTooManyPixels, ReasonTooManyPixels},
		{ErrTooSmall, ReasonTooSmall},
		{ErrCropOutside, ReasonCropOutside},
		{ErrCropAspect, ReasonCropAspect},
		{fmt.Errorf("original of x: %w", statErr), ReasonMissingOriginal},
		{errors.New("disk full"), ReasonInternal},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, ReasonOf(tt.err), "%v", tt.err)
	}
}
