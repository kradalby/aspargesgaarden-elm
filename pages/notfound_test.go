package pages

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestNotFound(t *testing.T) {
	got := NotFound(testSite).Render()

	golden.HTML(t, "notfound", got)
	require.NotContains(t, got, `rel="canonical"`)
}
