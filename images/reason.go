package images

import (
	"errors"
	"io/fs"
)

// Reason is a stable code for why a photo was refused or failed, for the
// admin to turn into a plain sentence. Error texts are for logs and change;
// these do not.
type Reason string

const (
	ReasonNone            Reason = ""
	ReasonTooLarge        Reason = "too_large"        // over MaxUpload
	ReasonNotPhoto        Reason = "not_photo"        // not JPEG, PNG, WebP, HEIC or AVIF; or animated
	ReasonDamaged         Reason = "damaged"          // header fine, picture cannot be decoded
	ReasonTooManyPixels   Reason = "too_many_pixels"  // header over the pixel cap
	ReasonTooSmall        Reason = "too_small"        // too few pixels for the use
	ReasonCropOutside     Reason = "crop_outside"     // crop not inside the photo
	ReasonCropAspect      Reason = "crop_aspect"      // crop has the wrong shape
	ReasonMissingOriginal Reason = "missing_original" // the stored upload is gone; needs a restore
	ReasonInternal        Reason = "internal"         // anything else: see the log
)

// ReasonOf classifies an error from Intake, ValidateCrop, DefaultCrop or a
// build.
func ReasonOf(err error) Reason {
	for _, m := range []struct {
		err    error
		reason Reason
	}{
		{ErrTooLarge, ReasonTooLarge},
		{ErrUnsupported, ReasonNotPhoto},
		{ErrBroken, ReasonDamaged},
		{ErrTooManyPixels, ReasonTooManyPixels},
		{ErrTooSmall, ReasonTooSmall},
		{ErrCropOutside, ReasonCropOutside},
		{ErrCropAspect, ReasonCropAspect},
		{fs.ErrNotExist, ReasonMissingOriginal},
	} {
		if errors.Is(err, m.err) {
			return m.reason
		}
	}

	if err == nil {
		return ReasonNone
	}

	return ReasonInternal
}
