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

// Every construct the site's Markdown uses today (About, facilities, event
// bodies), plus the rest of CommonMark, so a change of styling shows up here.
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
