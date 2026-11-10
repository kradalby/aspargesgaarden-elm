package markdown

import (
	"strings"
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func render(src string) string {
	return elem.Fragment(Render(src)...).Render()
}

// Every construct the site's Markdown uses (About, facilities, event bodies),
// plus the rest of CommonMark, so a change of styling shows up here.
const specimen = `# Vertskapet

Første avsnitt med **fet**, *kursiv* og ` + "`kode`" + `.
Myk linjeskift.

## Praktisk info

**Billettpris:** 695,-\
**Billetter:** [fabnite.com](https://fabnite.com/no/events/x)

[**Patrick Castelano**](https://www.instagram.com/patrick_kokk_fly_fishing), vår kokk.

Send [SMS](sms:+4790919812) eller ring [90 91 98 12](tel:+4790919812).
Les mer [her](/julemarked) og <https://aspargesgaarden.no>.

- **31. oktober:** Potetlefse
- Godt humør

1. Først
2. Så

3. Tredje

### Liten overskrift

> Sitat

---

    kodeblokk

AT&amp;T, &#169; og \*ikke kursiv\*.
`

func TestSpecimen(t *testing.T) {
	golden.HTML(t, "specimen", render(specimen))
}

// The parts of the old Christmas market page, written as Markdown.
const eventSpecimen = `# Velkommen til Julemarked på Aspargesgården!

Bli med oss på en herlig helg.

## Når og hvor

**Dato:** 14.–15. november 2026

## Høydepunkter

- Nydelig kafe med hjemmebakst
- Julepyntet gård

### Liten overskrift

> **Velkommen!**
>
> Ta med familie og venner.
`

func TestEventSpecimen(t *testing.T) {
	got := elem.Fragment(RenderEvent(eventSpecimen)...).Render()

	golden.HTML(t, "event-specimen", got)
	require.NotContains(t, got, "<h1")
	require.NotContains(t, got, "<blockquote")
	require.Contains(t, got, `<h2 class="text-4xl mt-2 mb-6 font-display text-tre">`)
	require.Contains(t, got, `<h3 class="text-3xl mt-8 mb-4 font-display text-tre">Når og hvor</h3>`)
	require.Contains(t, got, `<h4 class="font-bold text-lg mt-8 mb-4">`)
	require.Contains(t, got, `<div class="bg-brown/20 p-6 my-8">`)
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name, src, want string
		max             int
	}{
		{name: "empty", src: "", want: "", max: 100},
		{
			name: "markup-goes", max: 100,
			src:  "Med [**Ingunn Anvik**](https://x.no) &amp; *venner*.\\\nNy linje.",
			want: "Med Ingunn Anvik & venner. Ny linje.",
		},
		{
			name: "short-first-paragraph-takes-the-next", max: 100,
			src:  "# Tittel\n\nVelkommen!\n\nBli med oss.\n\n- liste",
			want: "Velkommen! Bli med oss.",
		},
		{
			name: "list-introductions-skipped", max: 100,
			src:  "Invitasjon til kurs.\n\n**Kursdatoer:**\n\n- 31. oktober\n\n**Sted:** Bryggerhuset.",
			want: "Invitasjon til kurs. Sted: Bryggerhuset.",
		},
		{
			name: "notes-skipped", max: 100,
			src:  "*Slutt ca. kl. 15:00.*\n\n**Velkommen!**\n\nInvitasjon til kurs.",
			want: "Invitasjon til kurs.",
		},
		{
			name: "long-cut-at-a-word", max: 20,
			src:  "Aspargesgården inviterer denne høsten til gourmetaftener.",
			want: "Aspargesgården…",
		},
		{
			name: "stops-once-there-is-enough", max: 20,
			src:  "Første avsnitt her.\n\nAndre.",
			want: "Første avsnitt her.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Excerpt(tt.src, tt.max))
		})
	}
}

func TestHeadingsLeaveH1ToThePage(t *testing.T) {
	got := render("# En\n\n## To\n\n###### Seks")

	require.NotContains(t, got, "<h1")
	require.Contains(t, got, `<h2 class="text-6xl`)
	require.Contains(t, got, `<h3 class="text-3xl`)
	require.Contains(t, got, `<h6 class="font-bold`)
}

// The old renderer dropped the numbers (no list-style) and inverted start.
func TestOrderedListsKeepTheirNumbers(t *testing.T) {
	require.Contains(t, render("1. a\n2. b"), `<ol class="list-decimal`)
	require.NotContains(t, render("1. a"), "start=")
	require.Contains(t, render("4. a\n5. b"), `start="4"`)
}

func TestLinks(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"[a](https://x.no/?a=1&b=2)", `<a class="underline" href="https://x.no/?a=1&amp;b=2">a</a>`},
		{"[a](/kontakt#kart)", `<a class="underline" href="/kontakt#kart">a</a>`},
		{"[a](tel:+4790919812)", `<a class="underline" href="tel:+4790919812">a</a>`},
		{"[a](sms:+4790919812)", `<a class="underline" href="sms:+4790919812">a</a>`},
		{"<kristine@example.no>", `<a class="underline" href="mailto:kristine@example.no">kristine@example.no</a>`},
		{`[a](https://x.no/"><script>)`, `href="https://x.no/%22%3E%3Cscript%3E"`},
		// Dangerous or unknown schemes keep the words, lose the link.
		{"[a](javascript:alert(1))", `<p class="pb-3 text-sort">a</p>`},
		{"[a](JavaScript:alert(1))", `<p class="pb-3 text-sort">a</p>`},
		{"[a](data:text/html,x)", `<p class="pb-3 text-sort">a</p>`},
		{"[a](vbscript:x)", `<p class="pb-3 text-sort">a</p>`},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			require.Contains(t, render(tt.src), tt.want)
		})
	}
}

func TestRawHTMLIsDropped(t *testing.T) {
	got := render("<script>alert(1)</script>\n\nTekst <b onclick=x>fet</b> & <i>.")

	require.NotContains(t, got, "<script")
	require.NotContains(t, got, "<b")
	require.NotContains(t, got, "<i>")
	require.Contains(t, got, "Tekst fet &amp; .")
}

func TestImagesKeepTheirAltText(t *testing.T) {
	got := render("![Låven om kvelden](/x.jpeg)")

	require.NotContains(t, got, "<img")
	require.Contains(t, got, "Låven om kvelden")
}

func TestEmptyInput(t *testing.T) {
	require.Empty(t, Render(""))
	require.Empty(t, strings.TrimSpace(render("\n\n")))
}
