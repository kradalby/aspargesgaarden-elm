package images

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

var contentTypes = map[Format]string{AVIF: "image/avif", WebP: "image/webp", JPEG: "image/jpeg"}

// Handler serves /img/… : only variants the catalogue currently expects,
// built on first request if the queue has not got to them. Names change with
// every crop and pipeline version, so responses are cached forever.
func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

			return
		}

		spec, err := ParseURL(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		start := time.Now()

		f, built, err := s.open(r.Context(), spec)
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		}

		if err != nil {
			s.log.Error("images: serve variant", "url", r.URL.Path, "err", err)
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

		result := "hit"
		if built {
			result = "built"
		}

		metricServeSeconds.WithLabelValues(result).Observe(time.Since(start).Seconds())
	})
}
