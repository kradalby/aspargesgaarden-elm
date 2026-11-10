package images

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
)

var contentTypes = map[Format]string{AVIF: "image/avif", WebP: "image/webp", JPEG: "image/jpeg"}

// Handler serves /img/… : the variant files that exist. One the queue has
// not built yet is a 404, never a build, so no request causes work. Names
// change with every crop and pipeline version, so responses are cached
// forever.
func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

			return
		}

		// Only a well-formed variant name reaches the filesystem.
		spec, err := ParseURL(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		f, err := os.Open(s.path(spec))
		if errors.Is(err, fs.ErrNotExist) {
			// The file may land in a moment; nobody may keep this answer.
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)

			return
		}

		if err != nil {
			s.log.Error("images: open variant", "url", r.URL.Path, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			s.log.Error("images: stat variant", "path", f.Name(), "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}

		h := w.Header()
		h.Set("Content-Type", contentTypes[spec.Format])
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		// Strong: a rebuild after a pipeline or vips change rewrites the file.
		h.Set("ETag", fmt.Sprintf(`"%x-%x"`, st.ModTime().UnixNano(), st.Size()))
		// ServeContent does ranges and conditionals; the page cache does the rest.
		http.ServeContent(w, r, "", st.ModTime(), f)
	})
}
