package pages

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/markdown"
	"github.com/kradalby/asparges/ui"
)

const omOssText = `
Aspargesgården byr på sjarmerende og unike lokaler i landlige omgivelser, ved kysten i Vestfold.

Vi byr på annerledes og stemningsfylte selskapslokaler, hvor låvebryllup er vår spesialitet. Vi ønsker velkommen til alle typer selskap; bryllup, konfirmasjoner, dåp, minnestunder, bursdager, firmasamlinger, kurs, konserter og konferanser.

Låven og fjøset rommer det dere trenger av moderne teknisk utstyr, samt komfortable fasiliteter og fremkommelighet for alle! Vi har det du trenger til å dekke et vakkert festbord og skape den rette stemning!

Her hos oss kan du velge om du vil gjøre litt- mye- eller ingenting selv!

Ta en nærmere kikk på [muligheter om hva vi kan tilby](/lokaler) - og skulle du ha spesielle ønsker; ikke nøl med å ta kontakt! La oss skape den perfekte rammen for ditt arrangement!


# Vertskapet

I 2014 flyttet vi fra Sandefjord til idylliske Tjølling og Aspargesgården. Tidligere eiere dyrket asparges her, og gjorde om fjøset til selskapslokaler, derav navnet «Aspargesgården».

Vi -  min mann Anders og jeg, trengte låven til kurs, møter og lager for vår salgsstyrke og Tupperware produkter.

Vi har tre voksne barn, og lille Angel, vår søte Bichon Havanais.

Jeg har alltid hatt en drøm om å bo på gård, selv om jeg er en skikkelig «byjente». Elsker gamle hus, historie, interiør, hage og å gjøre det vakkert rundt meg. Anders er «handyman» som restaurerer og bygger, og sammen utvikler vi gården kontinuerlig.

Det er en utrolig glede og skape så mange fantastiske øyeblikk, muligheter og opplevelser.

Vi ser frem til å ønske mange flere brudepar, selskap, og alle andre som har lyst til å gjøre noe morsomt her på gården velkommen!

Alt godt, Kristine & Anders
`

// The kinds of gatherings the farm hosts, shown as pictograms.
var occasions = []struct {
	icon ui.Icon
	name string
}{
	{ui.IconBryllup, "Bryllup"},
	{ui.IconMinnestund, "Minnestund"},
	{ui.IconDaap, "Dåp"},
	{ui.IconJulebord, "Julebord"},
	{ui.IconBursdag, "Bursdag"},
	{ui.IconKonferanse, "Konferanse"},
	{ui.IconFeiring, "Feiring"},
	{ui.IconNyttaar, "Nyttår"},
	{ui.IconVitnemaal, "Vitnemål"},
}

// OmOss is about the farm and its hosts (old /about).
func OmOss(s Site) *elem.Element {
	icons := make([]elem.Node, len(occasions))
	for i, o := range occasions {
		icons[i] = ui.IconImage(o.icon, o.name)
	}

	return s.page(ui.Meta{
		Title:       "Om gården og vertskapet",
		Description: "Kristine og Anders driver Aspargesgården i Tjølling, Larvik, der låvebryllup er spesialiteten. Les om gården, vertskapet og selskapene vi tar imot.",
		Path:        "/om-oss",
	},
		ui.Container(
			elem.Div(nil, ui.Headline("Om gården")),
			elem.Div(attrs.Props{attrs.Class: "flex flex-row flex-wrap"},
				elem.Div(attrs.Props{attrs.Class: "w-full px-5 text-sort md:w-1/2 md:px-0"},
					append(markdown.Render(omOssText), ui.ContactLink())...),
				elem.Div(attrs.Props{attrs.Class: "w-full flex flex-row flex-wrap justify-center md:w-1/2 md:-mt-12"}, icons...),
			),
		),
	)
}
