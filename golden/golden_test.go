package golden

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTMLMasksAssetHashes(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("testdata", 0o750))
	require.NoError(t, os.WriteFile(filepath.Join("testdata", "x.html"),
		[]byte("<p>\n<a href=\"/static/a.css?v=HASH\">\n</a>\n</p>\n"), 0o600))

	HTML(t, "x", `<p><a href="/static/a.css?v=0123456789ab"></a></p>`)
}

func TestHTMLIndentsJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("testdata", 0o750))
	require.NoError(t, os.WriteFile(filepath.Join("testdata", "x.html"), []byte(`<script type="application/ld+json">
{
  "a": [
    1
  ]
}
</script>
<div class="{x}" data-gallery="
[
  {
    "k": "Å"
  }
]
">
</div>
`), 0o600))

	HTML(t, "x", `<script type="application/ld+json">{"a":[1]}</script><div class="{x}" data-gallery="[{&quot;k&quot;:&quot;Å&quot;}]"></div>`)
}
