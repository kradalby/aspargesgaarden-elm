package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// Backdrop fills the screen with a photo that stays put while the page
// scrolls, as the old CSS background-attachment: fixed did, but as a real
// <picture> the browser can pick and prioritise. The image is fixed to the
// viewport and clip-path cuts it to this block. iOS WebKit, the only engine
// with -webkit-touch-callout, never fixed a background: there it covered
// this block and scrolled with it, and so it does here. Title is the page's
// <h1>, for readers and search engines only: the photo is the visible
// content.
func Backdrop(title string, photo images.Ref) elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "relative h-screen flex flex-col justify-center [clip-path:inset(0)]"},
		elem.H1(attrs.Props{attrs.Class: "sr-only"}, elem.Text(title)),
		Picture(PictureProps{
			Photo:      photo,
			Decorative: true,
			Layout:     LayoutFullscreen,
			Priority:   Hero,
			Class:      "fixed inset-0 h-screen w-screen max-w-none object-cover supports-[-webkit-touch-callout:none]:absolute supports-[-webkit-touch-callout:none]:h-full",
		}),
	)
}
