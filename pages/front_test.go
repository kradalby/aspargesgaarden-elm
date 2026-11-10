package pages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFront(t *testing.T) {
	got := Front("https://aspargesgaarden.no").Render()

	require.Equal(t,
		`<!DOCTYPE html><html lang="nb"><head><meta charset="utf-8">`+
			`<meta content="width=device-width, initial-scale=1" name="viewport">`+
			`<title>Aspargesgården</title>`+
			`<link href="https://aspargesgaarden.no/" rel="canonical"></head>`+
			`<body><h1>Aspargesgården</h1></body></html>`,
		got)
}

// elem-go before v0.37 wrote attribute values verbatim; a downgrade must fail
// here rather than open an injection hole.
func TestFrontEscapesAttributes(t *testing.T) {
	got := Front(`https://x"><script>alert(1)</script>`).Render()

	require.NotContains(t, got, "<script>")
	require.Contains(t, got, `href="https://x&quot;&gt;&lt;script&gt;alert(1)&lt;/script&gt;/"`)
}
