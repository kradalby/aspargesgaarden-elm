package images

// Format is the encoding of one variant file.
type Format string

const (
	AVIF Format = "avif"
	WebP Format = "webp"
	JPEG Format = "jpeg"
)

// Variant is one file a browser can pick for a photo.
type Variant struct {
	Format Format
	Width  int
	URL    string
}

// Ref is what a page needs to show a photo. Width and Height are the
// intrinsic pixel size of the largest variant.
type Ref struct {
	Width, Height int
	Variants      []Variant
}
