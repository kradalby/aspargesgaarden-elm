package pages

import (
	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// Galleri is the photo gallery (old /gallery), photos in the admin's order.
func Galleri(s Site, photos []site.Photo) *elem.Element {
	return s.page(ui.Meta{Title: "Galleri", Description: description, Path: "/galleri"},
		ui.Container(
			elem.Div(nil, ui.Headline("Galleri")),
			ui.Gallery(galleryPhotos(photos)...),
		),
	)
}

// fallbackAlt names a photo stored without alt text, so its link still says
// where it leads. The admin requires alt text; imported photos may lack it.
const fallbackAlt = "Bilde fra Aspargesgården"

// galleryPhotos is what ui.Gallery shows of stored photos, in their order:
// the whole photo, its alt text and its photographer.
func galleryPhotos(photos []site.Photo) []ui.GalleryPhoto {
	out := make([]ui.GalleryPhoto, len(photos))

	for i, p := range photos {
		out[i] = ui.GalleryPhoto{Photo: photoRef(p, images.Full), Alt: p.Alt, By: credit(p.Photographer)}
		if out[i].Alt == "" {
			out[i].Alt = fallbackAlt
		}
	}

	return out
}
