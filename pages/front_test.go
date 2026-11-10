package pages

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestFront(t *testing.T) {
	golden.HTML(t, "front", Front(testSite).Render())
}

// elem-go before v0.37 wrote attribute values verbatim; a downgrade must fail
// here rather than open an injection hole.
func TestFrontEscapesAttributes(t *testing.T) {
	got := Front(Site{BaseURL: `https://x"><script>alert(1)</script>`}).Render()

	require.NotContains(t, got, "<script>alert")
	require.Contains(t, got, `href="https://x&quot;&gt;&lt;script&gt;alert(1)&lt;/script&gt;/"`)
}
