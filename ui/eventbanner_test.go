package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEventBanner(t *testing.T) {
	got := EventBanner(testPhoto(), "Bålfat på gårdsplassen", Photographer{Name: "Marianne Ous"}).Render()

	golden.HTML(t, "eventbanner", got)
	require.Contains(t, got, `fetchpriority="high"`)
	require.Contains(t, got, `sizes="(min-width: 56rem) 56rem, 100vw"`)
}
