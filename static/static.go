// Package static embeds the site's own files (stylesheet, fonts, logo, icons,
// fixed-page photos) and serves them under /static/ with content-hashed URLs.
//
// files/ is committed as is. dist/ is built by build.sh: in the Nix build
// before go:embed runs, and locally through `go generate ./static`.
package static

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

//go:generate ./build.sh

//go:embed files dist
var embedded embed.FS

// Prefix is where Handler is mounted.
const Prefix = "/static/"

type asset struct {
	data  []byte
	hash  string
	ctype string
}

// Only extensions the site ships; an unknown one fails load (and so every
// test) instead of being served with a guessed type.
var contentTypes = map[string]string{
	".avif":  "image/avif",
	".css":   "text/css; charset=utf-8",
	".ico":   "image/x-icon",
	".jpeg":  "image/jpeg",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".webp":  "image/webp",
	".woff2": "font/woff2",
}

var assets = mustLoad(embedded)

func mustLoad(fsys fs.FS) map[string]asset {
	a, err := load(fsys)
	if err != nil {
		panic("static: " + err.Error())
	}

	return a
}

// load reads files/ and dist/ into one namespace. A name present in both is
// an error: one of them would silently shadow the other.
func load(fsys fs.FS) (map[string]asset, error) {
	out := map[string]asset{}

	for _, root := range []string{"files", "dist"} {
		err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if p == root && errors.Is(err, fs.ErrNotExist) {
				// Reported below as the actionable "site.css missing".
				return nil
			}

			if err != nil || d.IsDir() {
				return err
			}

			name := strings.TrimPrefix(p, root+"/")

			ctype, ok := contentTypes[path.Ext(name)]
			if !ok {
				return fmt.Errorf("%s: no content type for %q", p, path.Ext(name))
			}

			if _, dup := out[name]; dup {
				return fmt.Errorf("%s: %q exists in both files/ and dist/", p, name)
			}

			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}

			out[name] = asset{data: data, hash: digest(data), ctype: ctype}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	css, ok := out["site.css"]
	if !ok {
		return nil, errors.New("dist/site.css missing: run go generate ./static")
	}

	// The stylesheet names fonts by plain path; point it at the hashed URLs so
	// they can be cached immutably too. Hashing after the rewrite makes a font
	// change a stylesheet change.
	var missing []string

	data := cssURL.ReplaceAllFunc(css.data, func(m []byte) []byte {
		name := string(cssURL.FindSubmatch(m)[1])

		a, ok := out[name]
		if !ok {
			missing = append(missing, name)

			return m
		}

		return []byte("url(" + url(name, a.hash) + ")")
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("site.css references files not embedded: %v", missing)
	}

	out["site.css"] = asset{data: data, hash: digest(data), ctype: css.ctype}

	return out, nil
}

var cssURL = regexp.MustCompile(`url\(["']?` + Prefix + `([^)"'?]+)["']?\)`)

func digest(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])[:12]
}

func url(name, hash string) string { return Prefix + name + "?v=" + hash }

// URL returns the cache-busting URL of an embedded file, e.g.
// URL("site.css") = "/static/site.css?v=1a2b3c4d5e6f". Names are constants in
// the code, so an unknown one is a programming error and panics.
func URL(name string) string {
	a, ok := assets[name]
	if !ok {
		panic("static: no embedded file " + name)
	}

	return url(name, a.hash)
}

var fontRef = regexp.MustCompile(Prefix + `fonts/[^)"'?]+\?v=[0-9a-f]+`)

// FontURLs returns the hashed URLs of the fonts the stylesheet uses, in its
// order, so a page can preload exactly those (and a stale font left in
// dist/ is not among them).
func FontURLs() []string {
	return slices.Compact(fontRef.FindAllString(string(assets["site.css"].data), -1))
}

// Handler serves the embedded files. Requests that carry the current hash are
// cached for a year; anything else (a stale or missing ?v=) must revalidate,
// so an old URL can never pin new bytes.
func Handler() http.Handler {
	return http.StripPrefix(Prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)

			return
		}

		h := w.Header()
		h.Set("Content-Type", a.ctype)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("ETag", `"`+a.hash+`"`)

		if r.URL.Query().Get("v") == a.hash {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}

		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(a.data))
	}))
}
