package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
)

// Icon is an embedded pictogram (static/files). Only these exist, so a page
// cannot point at a missing file.
type Icon string

// The icons.
const (
	IconBryllup    Icon = "bryllup.svg"
	IconMinnestund Icon = "minnestund.svg"
	IconDaap       Icon = "daap.svg"
	IconJulebord   Icon = "julebord.svg"
	IconBursdag    Icon = "bursdag.svg"
	IconKonferanse Icon = "konferanse.svg"
	IconFeiring    Icon = "feiring.svg"
	IconNyttaar    Icon = "nyttaar.svg"
	IconVitnemaal  Icon = "vitnemaal.svg"
	IconPhone      Icon = "tlf.svg"
	IconMail       Icon = "mail.svg"
	IconLocation   Icon = "location.svg"
)

// IconImage shows an icon at its natural size. Alt says what it stands for.
func IconImage(icon Icon, alt string) elem.Node {
	return iconImg(icon, alt, "")
}

func iconImg(icon Icon, alt, class string) elem.Node {
	p := attrs.Props{attrs.Src: static.URL(string(icon)), attrs.Alt: alt}
	if class != "" {
		p[attrs.Class] = class
	}

	return elem.Img(p)
}
