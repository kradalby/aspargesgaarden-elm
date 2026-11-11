package pages

import (
	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// Galleri is the photo gallery (old /gallery), photos in the admin's order.
func Galleri(s Site, photos []site.Photo) *elem.Element {
	m := meta("/galleri", "Bilder fra Aspargesgården i Tjølling, Larvik: låven og fjøset pyntet til fest, det engelske lysthuset og hagen.")

	// The first photo is the one the owner put first: the link preview.
	if len(photos) > 0 {
		m.Image, _ = seo.PhotoImage(photos[0])
	}

	return s.page(m,
		ui.Container(
			elem.Div(nil, ui.Headline("Galleri")),
			ui.Gallery(ui.GalleryPage, galleryPhotos(photos)...),
		),
	)
}

// fallbackAlt names a photo stored without alt text, so its link still says
// where it leads. The admin requires alt text only on photos a published
// event shows; gallery photos may lack it.
const fallbackAlt = "Bilde fra Aspargesgården"

// galleryPhotos is what ui.Gallery shows of stored photos, in their order:
// the whole photo, its alt text and its photographer.
func galleryPhotos(photos []site.Photo) []ui.GalleryPhoto {
	out := make([]ui.GalleryPhoto, len(photos))

	for i, p := range photos {
		out[i] = ui.GalleryPhoto{Photo: photoRef(p, images.Full), Alt: p.Alt, Caption: p.Caption, By: credit(p.Photographer)}
		if out[i].Alt == "" {
			out[i].Alt = fallbackAlt
		}
	}

	return out
}
