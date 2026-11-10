package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// update rewrites the file through readHashes and writeHashes: whatever it
// does not own, the yarn hashes the flake reads among them, comes back as it was.
func TestHashesRoundTrip(t *testing.T) {
	const want = `{
  "vendor": {
    "goModSum": "FS8hKMm+fwwmQftiwmctNwXPtqu0mVEiTmAb5w+5jgc=",
    "sri": "sha256-fQEry0cHttWhB/+EhDb4BRx3gvAOqSmbc+S5NZFqcSw="
  },
  "yarn": {
    "admin": "sha256-XgTVkC6K5wuZza6+5Pln5/u5YvXzCjuYgOB+Q3AIdG4="
  }
}
`

	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(hashesFile, []byte(want), 0o600))

	h, err := readHashes()
	require.NoError(t, err)
	require.NoError(t, writeHashes(h))

	got, err := os.ReadFile(hashesFile)
	require.NoError(t, err)
	require.JSONEq(t, want, string(got))
}
