// Package golden compares rendered HTML with a reviewed copy under testdata/.
//
//	UPDATE_GOLDEN=1 go test ./...   re-records every golden file
package golden

import (
	"bytes"
	"encoding/json"
	"html"
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

// JSON in a script or an attribute renders as one long line; indented, a
// change in it reads as a diff rather than a wall to re-record blind.
var (
	ldJSON   = regexp.MustCompile(`(<script type="application/ld\+json">)(.*?)(</script>)`)
	attrJSON = regexp.MustCompile(`="([\[{][^"]*)"`)
)

func indentJSON(s string) (string, bool) {
	var b bytes.Buffer
	if json.Indent(&b, []byte(s), "", "  ") != nil {
		return "", false
	}

	return "\n" + b.String() + "\n", true
}

// HTML fails tb unless page, one tag per line and its JSON indented, matches
// testdata/<name>.html.
func HTML(tb testing.TB, name, page string) {
	tb.Helper()

	got := ldJSON.ReplaceAllStringFunc(page, func(m string) string {
		p := ldJSON.FindStringSubmatch(m)
		if j, ok := indentJSON(p[2]); ok {
			return p[1] + j + p[3]
		}

		return m
	})
	got = attrJSON.ReplaceAllStringFunc(got, func(m string) string {
		if j, ok := indentJSON(html.UnescapeString(attrJSON.FindStringSubmatch(m)[1])); ok {
			return `="` + j + `"`
		}

		return m
	})
	got = strings.ReplaceAll(assetHash.ReplaceAllString(got, "?v=HASH"), "><", ">\n<") + "\n"
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
