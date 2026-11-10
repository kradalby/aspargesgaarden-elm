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
