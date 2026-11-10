package static

import (
	"bytes"
	"cmp"
	"fmt"
	"image/jpeg"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/kradalby/asparges/images"
)

// shortcut: photos are read back from build.sh's file names; the images
// package's build-time manifest replaces this file, keeping Photo's signature.
var photos = mustPhotos(assets)

func mustPhotos(a map[string]asset) map[string]images.Ref {
	p, err := indexPhotos(a)
	if err != nil {
		panic("static: " + err.Error())
	}

	return p
}

// indexPhotos groups photos/<name>-<width>.<ext> into one Ref per name.
func indexPhotos(a map[string]asset) (map[string]images.Ref, error) {
	refs := map[string]images.Ref{}
	sizedBy := map[string]int{} // width of the JPEG each Ref's size came from

	for file, as := range a {
		rest, ok := strings.CutPrefix(file, "photos/")
		if !ok {
			continue
		}

		ext := path.Ext(rest)
		base := strings.TrimSuffix(rest, ext)
		i := strings.LastIndexByte(base, '-')

		width, err := strconv.Atoi(base[i+1:])
		if i < 0 || err != nil {
			return nil, fmt.Errorf("photo %s: want <name>-<width>.<ext>", file)
		}

		name := base[:i]

		ref := refs[name]
		ref.Variants = append(ref.Variants, images.Variant{
			Format: images.Format(strings.TrimPrefix(ext, ".")),
			Width:  width,
			URL:    url(file, as.hash),
		})

		if ext == ".jpeg" && width > sizedBy[name] {
			sizedBy[name] = width

			cfg, err := jpeg.DecodeConfig(bytes.NewReader(as.data))
			if err != nil {
				return nil, fmt.Errorf("photo %s: %w", file, err)
			}

			ref.Width, ref.Height = cfg.Width, cfg.Height
		}

		refs[name] = ref
	}

	for name, ref := range refs {
		if ref.Width == 0 {
			return nil, fmt.Errorf("photo %s: no JPEG variant to size it by", name)
		}

		slices.SortFunc(ref.Variants, func(a, b images.Variant) int {
			return cmp.Or(cmp.Compare(a.Format, b.Format), cmp.Compare(a.Width, b.Width))
		})
	}

	return refs, nil
}

// Photo returns the embedded photo built from photos-src/<name>.jpeg. Names
// are constants next to the page that shows them, so an unknown one panics.
func Photo(name string) images.Ref {
	ref, ok := photos[name]
	if !ok {
		panic("static: no embedded photo " + name)
	}

	return ref
}
