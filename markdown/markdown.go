// Package markdown renders CommonMark to elem-go nodes styled like the old
// site's Markdown. It walks goldmark's AST instead of
// using its HTML renderer, so the output is ordinary elem nodes that escape
// every value; raw HTML in the source is dropped.
package markdown

import (
	"bufio"
	"bytes"
	"html"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var parser = goldmark.New().Parser()

// Render turns Markdown into nodes. Headings come out one level lower than
// written (# is an <h2> that looks like the old <h1>): the page's own
// headline is the only <h1>, and the text is written as if it stood alone.
func Render(src string) []elem.Node {
	return renderer{src: []byte(src)}.render()
}

// RenderEvent is Render in the look of the old hand-made Christmas market
// page, so an event written in Markdown reads as well:
// # is the introduction, ## a section, and a > quote a tinted callout box.
func RenderEvent(src string) []elem.Node {
	return renderer{src: []byte(src), event: true}.render()
}

type renderer struct {
	src   []byte
	event bool
}

func (r renderer) render() []elem.Node {
	return r.children(parser.Parse(text.NewReader(r.src)))
}

func (r renderer) children(n ast.Node) []elem.Node {
	var out []elem.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, r.node(c)...)
	}

	return out
}

func (r renderer) node(n ast.Node) []elem.Node {
	switch n := n.(type) {
	case *ast.Paragraph:
		return one(elem.P(attrs.Props{attrs.Class: "pb-3 text-sort"}, r.children(n)...))
	case *ast.TextBlock: // the body of a tight list item
		return r.children(n)
	case *ast.Heading:
		return one(r.heading(n))
	case *ast.ThematicBreak:
		return one(elem.Hr(nil))
	case *ast.Blockquote:
		if r.event {
			// A callout, not a quotation: a div keeps screen readers from
			// announcing a quote.
			return one(elem.Div(attrs.Props{attrs.Class: "bg-brown/20 p-6 my-8"}, r.children(n)...))
		}

		return one(elem.Blockquote(nil, r.children(n)...))
	case *ast.CodeBlock, *ast.FencedCodeBlock:
		return one(elem.Pre(nil, elem.Code(nil, elem.Text(r.lines(n)))))
	case *ast.List:
		return one(r.list(n))
	case *ast.ListItem:
		return one(elem.Li(nil, r.children(n)...))
	case *ast.Text:
		return r.text(n)
	case *ast.String:
		return one(elem.Text(string(n.Value)))
	case *ast.Emphasis:
		if n.Level == 2 {
			return one(elem.Strong(attrs.Props{attrs.Class: "font-bold"}, r.children(n)...))
		}

		return one(elem.Em(attrs.Props{attrs.Class: "italic"}, r.children(n)...))
	case *ast.CodeSpan:
		var code strings.Builder
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				code.Write(t.Segment.Value(r.src))
			}
		}

		return one(elem.Code(attrs.Props{attrs.Class: "font-medium text-[#e2007c]!"},
			elem.Text(strings.ReplaceAll(code.String(), "\n", " "))))
	case *ast.Link:
		return r.link(n.Destination, r.children(n))
	case *ast.AutoLink:
		dest := n.URL(r.src)
		if n.AutoLinkType == ast.AutoLinkEmail {
			dest = append([]byte("mailto:"), dest...)
		}

		return r.link(dest, one(elem.Text(string(n.Label(r.src)))))
	case *ast.Image:
		// A bare URL cannot meet ui.Picture's contract; keep the words, drop
		// the picture.
		return r.children(n)
	case *ast.HTMLBlock, *ast.RawHTML:
		return nil
	}

	return r.children(n)
}

func one(n elem.Node) []elem.Node { return []elem.Node{n} }

// text resolves escapes and entities exactly as goldmark's HTML renderer
// does, then hands elem plain text to escape again.
func (r renderer) text(n *ast.Text) []elem.Node {
	v := n.Segment.Value(r.src)
	if !n.IsRaw() {
		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)
		gmhtml.DefaultWriter.Write(w, v)
		_ = w.Flush() // a bytes.Buffer cannot fail
		v = []byte(html.UnescapeString(buf.String()))
	}

	s := string(v)
	if n.SoftLineBreak() {
		s += "\n"
	}

	if n.HardLineBreak() {
		return []elem.Node{elem.Text(s), elem.Br(nil)}
	}

	return one(elem.Text(s))
}

func (r renderer) lines(n ast.Node) string {
	var b strings.Builder

	for i := range n.Lines().Len() {
		seg := n.Lines().At(i)
		b.Write(seg.Value(r.src))
	}

	return b.String()
}

var (
	heading1 = "text-6xl tracking-tight mt-2 mb-4 font-display text-tre"
	heading2 = "text-3xl font-semibold tracking-tight mt-10 pb-1 border-b"
	headingN = "font-bold text-lg mt-8 mb-4"

	eventHeading1 = "text-4xl mt-2 mb-6 font-display text-tre"
	eventHeading2 = "text-3xl mt-8 mb-4 font-display text-tre"
)

func (r renderer) heading(n *ast.Heading) elem.Node {
	tag := "h" + strconv.Itoa(min(n.Level+1, 6))
	kids := r.children(n)

	if r.event && n.Level <= 2 {
		class := eventHeading1
		if n.Level == 2 {
			class = eventHeading2
		}

		return &elem.Element{Tag: tag, Attrs: attrs.Props{attrs.Class: class}, Children: kids}
	}

	switch n.Level {
	case 1:
		return &elem.Element{Tag: tag, Attrs: attrs.Props{attrs.Class: heading1}, Children: kids}
	case 2:
		id := anchorID(string(n.Lines().Value(r.src)))
		kids = append(kids, elem.Span(attrs.Props{attrs.Class: "ml-2 text-gray-500 select-none", attrs.AriaHidden: "true"}, elem.Text("#")))

		return &elem.Element{
			Tag:      tag,
			Attrs:    attrs.Props{attrs.ID: id, attrs.Class: heading2},
			Children: one(elem.A(attrs.Props{attrs.Href: "#" + id, attrs.Class: "no-underline!"}, kids...)),
		}
	}

	return &elem.Element{Tag: tag, Attrs: attrs.Props{attrs.Class: headingN}, Children: kids}
}

// anchorID is the old site's rawTextToId, so links to its headings still land.
func anchorID(raw string) string {
	return strings.ToLower(strings.Join(strings.Split(raw, " "), "-"))
}

func (r renderer) list(n *ast.List) elem.Node {
	if !n.IsOrdered() {
		return elem.Ul(attrs.Props{attrs.Class: "list-disc list-inside space-y-2 pb-3"}, r.children(n)...)
	}

	p := attrs.Props{attrs.Class: "list-decimal list-inside space-y-2 pb-3"}
	if n.Start != 1 {
		p["start"] = strconv.Itoa(n.Start)
	}

	return elem.Ol(p, r.children(n)...)
}

// Link schemes a reader may follow. Anything else (javascript:, data:, file:,
// an unparsable URL) keeps its words but loses the link.
var safeSchemes = []string{"", "http", "https", "mailto", "tel", "sms"}

func (r renderer) link(dest []byte, kids []elem.Node) []elem.Node {
	href := string(util.URLEscape(dest, true))

	u, err := url.Parse(href)
	if err != nil || !slices.Contains(safeSchemes, strings.ToLower(u.Scheme)) {
		return kids
	}

	return one(elem.A(attrs.Props{attrs.Href: href, attrs.Class: "underline"}, kids...))
}

// Excerpt is the opening of src as plain text, for a list or a meta
// description when no summary was written: whole paragraphs until there is
// enough to give the gist, cut at a word before maxRunes. A paragraph that
// only introduces a list ("Kursdatoer:") says nothing alone and is skipped.
func Excerpt(src string, maxRunes int) string {
	r := renderer{src: []byte(src)}

	var paras []string

	length := 0
	for n := parser.Parse(text.NewReader(r.src)).FirstChild(); n != nil && length < maxRunes/2; n = n.NextSibling() {
		if _, ok := n.(*ast.Paragraph); !ok || isNote(n) {
			continue
		}

		p := strings.Join(strings.Fields(textOf(r.children(n)...)), " ")
		if p == "" || strings.HasSuffix(p, ":") {
			continue
		}

		paras = append(paras, p)
		length += len([]rune(p)) + 1
	}

	out := []rune(strings.Join(paras, " "))
	if len(out) <= maxRunes {
		return string(out)
	}

	cut := string(out[:maxRunes])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}

	return strings.TrimRight(cut, " ,.;:–-") + "…"
}

// isNote reports whether a paragraph is set wholly in italics or bold: an
// aside ("*Slutt ca. kl. 15:00.*") or a highlight, not the opening prose.
func isNote(p ast.Node) bool {
	_, ok := p.FirstChild().(*ast.Emphasis)
	return ok && p.ChildCount() == 1
}

// textOf flattens rendered nodes back to their words, so an excerpt
// resolves escapes, entities and links exactly as the page does.
func textOf(nodes ...elem.Node) string {
	var b strings.Builder

	for _, n := range nodes {
		switch n := n.(type) {
		case elem.TextNode:
			b.WriteString(string(n))
		case *elem.Element:
			if n.Tag == "br" {
				b.WriteByte(' ')
			}

			b.WriteString(textOf(n.Children...))
		}
	}

	return b.String()
}
