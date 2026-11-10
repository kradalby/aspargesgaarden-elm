// Package images owns every image file of the site and hands pages typed
// references to them.
package images

// Format is an encoded image format a variant is served in.
type Format string

// The formats a variant can have. Browsers pick the first they support, in
// this order of preference.
const (
	AVIF Format = "avif"
	WebP Format = "webp"
	JPEG Format = "jpeg"
)

// Variant is one encoded size of a photo.
type Variant struct {
	Format Format
	Width  int
	URL    string
}

// Ref is a photo as pages see it. Width and Height are the intrinsic pixel
// size of the largest variant, so the browser can reserve the box before any
// variant loads.
type Ref struct {
	Width, Height int
	Variants      []Variant
}
