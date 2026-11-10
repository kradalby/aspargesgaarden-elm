package static

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/kradalby/asparges/images"
)

// photoManifest is the images package's build-time manifest in dist/. It is
// read here and not served.
const photoManifest = "photos/" + images.ManifestFile

var photos = mustPhotos(embedded, assets)

func mustPhotos(fsys fs.FS, a map[string]asset) map[string]images.Ref {
	p, err := indexPhotos(fsys, a)
	if err != nil {
		panic("static: " + err.Error())
	}

	return p
}

// indexPhotos reads the manifest and points every variant at its hashed
// URL, so photos are cached like every other embedded file.
func indexPhotos(fsys fs.FS, a map[string]asset) (map[string]images.Ref, error) {
	b, err := fs.ReadFile(fsys, "dist/"+photoManifest)
	if err != nil {
		return nil, fmt.Errorf("dist/%s missing: run go generate ./static: %w", photoManifest, err)
	}

	refs, err := images.ParseManifest(b)
	if err != nil {
		return nil, err
	}

	for name, ref := range refs {
		for i, v := range ref.Variants {
			file, ok := strings.CutPrefix(v.URL, Prefix)

			as, embedded := a[file]
			if !ok || !embedded {
				return nil, fmt.Errorf("photo %s: %s is not embedded", name, v.URL)
			}

			ref.Variants[i].URL = url(file, as.hash)
		}
	}

	return refs, nil
}

// Photo returns the embedded photo built from photos-src/<name>.jpg. Names
// are constants next to the page that shows them, so an unknown one panics.
func Photo(name string) images.Ref {
	ref, ok := photos[name]
	if !ok {
		panic("static: no embedded photo " + name)
	}

	return ref
}
