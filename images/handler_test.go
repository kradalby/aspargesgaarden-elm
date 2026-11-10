package images

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	card := VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: AVIF}

	do := func(method, path string, hdr ...string) (int, http.Header, []byte) {
		t.Helper()

		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, nil)
		require.NoError(t, err)

		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		return res.StatusCode, res.Header, b
	}

	// Not built yet: a 404 nobody keeps, and the request builds nothing.
	code, hdr, _ := do(http.MethodGet, card.URL())
	require.Equal(t, http.StatusNotFound, code)
	require.Equal(t, "no-store", hdr.Get("Cache-Control"))
	require.NoFileExists(t, s.path(card))

	require.NoError(t, s.prepare(t.Context(), p.SHA256))

	code, hdr, body := do(http.MethodGet, card.URL())
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "image/avif", hdr.Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", hdr.Get("Cache-Control"))
	require.NotEmpty(t, body)

	etag := hdr.Get("ETag")
	require.NotEmpty(t, etag)

	code, _, _ = do(http.MethodGet, card.URL(), "If-None-Match", etag)
	require.Equal(t, http.StatusNotModified, code)

	code, _, part := do(http.MethodGet, card.URL(), "Range", "bytes=0-9")
	require.Equal(t, http.StatusPartialContent, code)
	require.Equal(t, body[:10], part)

	jpeg := card
	jpeg.Format = JPEG
	code, hdr, _ = do(http.MethodHead, jpeg.URL())
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "image/jpeg", hdr.Get("Content-Type"))

	code, _, _ = do(http.MethodPost, card.URL())
	require.Equal(t, http.StatusMethodNotAllowed, code)

	old := card
	old.Crop = "00000000"
	for _, path := range []string{old.URL(), "/img/", "/img/v1/../../etc/passwd", card.URL() + ".bak"} {
		code, _, _ := do(http.MethodGet, path)
		require.Equal(t, http.StatusNotFound, code, path)
	}
}
