// Package golden compares rendered HTML with a reviewed copy under testdata/.
//
//	UPDATE_GOLDEN=1 go test ./...   re-records every golden file
package golden

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Asset hashes change with every stylesheet edit; a golden file pins markup,
// not those bytes, so they are masked.
var assetHash = regexp.MustCompile(`\?v=[0-9a-f]{12}`)

// HTML fails tb unless html, one tag per line, matches testdata/<name>.html.
func HTML(tb testing.TB, name, html string) {
	tb.Helper()

	got := strings.ReplaceAll(assetHash.ReplaceAllString(html, "?v=HASH"), "><", ">\n<") + "\n"
	path := filepath.Join("testdata", name+".html")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		err := os.MkdirAll("testdata", 0o750)
		if err == nil {
			err = os.WriteFile(path, []byte(got), 0o600)
		}

		if err != nil {
			tb.Fatalf("recording %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("no golden file %s; record it with UPDATE_GOLDEN=1 and review it", path)
	}

	if diff := cmp.Diff(string(want), got); diff != "" {
		tb.Errorf("%s changed (-want +got); review, then re-record with UPDATE_GOLDEN=1:\n%s", path, diff)
	}
}
