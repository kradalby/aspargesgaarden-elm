package pages

import (
	"fmt"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/markdown"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

var (
	marianneOus    = ui.Photographer{Name: "Marianne Ous", Website: "https://www.instagram.com/marianneous"}
	hanneGulliksen = ui.Photographer{Name: "Hanne Gulliksen"}
	auroraHovland  = ui.Photographer{Name: "Aurora Hovland", Website: "https://www.aurorahovland.com"}
)

// facilities are the rooms, from the old content/facilities/*.md.
var facilities = []struct {
	photo, alt string
	by         ui.Photographer
	text       string
}{
	{
		photo: "loven",
		alt:   "Bordplassering på staffeli i den pyntede låven",
		by:    marianneOus,
		text: fmt.Sprintf(`# Låven

Drømmer du om låvebryllup så er «høyloftet» perfekt med bøttevis av sjarm. Låven er fra 1906 og har en nydelig patina. Det er høyt under taket, og lyslenker skaper den rette stemningen. Runde bord og gamle spisestuestoler med plass til %d personer, men kan gjøres intim ned til ca 45 personer. Om du liker det elegangt eller rustikt, så er begge deler mulig.

Det er bar, scene, dansegulv, musikkanlegg / mikrofon, prosjektor og lerret, samt belysning som kan dempes etter behov. Toaletter i tilknytning ved låvebrua.
`, seo.Venue.Guests("Låven")),
	},
	{
		photo: "fjoset",
		alt:   "Langbord dekket til fest i fjøset",
		by:    hanneGulliksen,
		text: fmt.Sprintf(`# Fjøset

I ”fjøset” er det spisestue med plassbygde langbord hvor man kan dekke til %d personer. «Møkkarennene» er i dag fyllt med gamle minner, skatter og bruksredskaper. Hvitmalte murvegger sammen med orginale smijernsvinduer, skaper en nydelig atmosfære.

Det finnes både musikkanlegg og flere skjermer for lysbildeshow. Den tidligere stallen er gjort om til en intim salong, med bar og scene/ dansegulv. Det finnes også et salongflygel på scenen.

Flott terrasse utenfor, hvor det er sol fra morgen til kveld.
`, seo.Venue.Guests("Fjøset")),
	},
	{
		photo: "kjokken",
		alt:   "Champagne og glass på et tønnebord foran steinmuren",
		by:    auroraHovland,
		text: `# Kjøkken

I fjøset er det storkjøkken med alt av utstyr. Kokker som er tilknyttet oss lager nydelig mat som er kortreist og av ypperste klasse. De er konkurranse dyktige på pris.

Vi anbefaler å bruke både kokker og servitører tilknyttet oss. Det skaper ro og profesjonalitet rundt arrangementet, da de kjenner utstyret og gården godt. Vi formidler kontakt til profesjonelle kokker og catering. Det er også mulig å bruke egne kokker mot et depositum. Drikkevarer ordner dere selv.
`,
	},
	{
		photo: "lysthus",
		alt:   "Det engelske lysthuset i hagen en sommerkveld",
		by:    auroraHovland,
		text: `# Engelsk lysthus og uteområder

På gården har vi et nydelig engelsk lysthus i hagen som mange bruker som bakgrunn for vielser og fotografering. Det pyntes etter årstider. Vakre uteområder er også til disposisjon. Stor deilig hage hvor det er fint å mingle, eller arrangere aktiviteter. Ved låven er det også terrasse utenfor, hvor det er sol fra morgen til kveld.

Se gjerne på vår [Instagram](https://instagram.com/aspargesgaarden) eller [Galleri](/galleri) for inspirasjon
`,
	},
	{
		photo: "skafferiet",
		alt:   "Festbord pyntet med lys, blomster og bordnummer",
		by:    auroraHovland,
		text: `# «Skafferiet»

Vi har alt du trenger til å dekke et lekkert festbord. Og selvfølgelig det du trenger av duker (rens av 9 duker er inkl. i prisen) og dekketøy.

Vi har mange skap i «skafferiet» med masse forskjellig pynt, og både håper og tror du finner noe du liker!

Dette er selvfølgelig inkludert i prisen når du leier hos oss.
`,
	},
}

// Lokaler presents the rooms and what comes with them (old /facilities).
func Lokaler(s Site) *elem.Element {
	rows := make([]ui.Row, len(facilities))
	for i, f := range facilities {
		rows[i] = ui.Row{Photo: static.Photo(f.photo), Alt: f.alt, By: f.by, Body: markdown.Render(f.text)}
	}

	return s.page(meta("/lokaler", fmt.Sprintf("Låven fra 1906 tar %d gjester ved runde bord, fjøset %d ved langbord. Storkjøkken, engelsk lysthus for vielser og alt til festbordet.",
		seo.Venue.Guests("Låven"), seo.Venue.Guests("Fjøset"))),
		ui.Container(elem.Div(nil, ui.Headline("Selskapslokaler"))),
		ui.PhotoRows(ui.RowFacility, rows...),
	)
}
