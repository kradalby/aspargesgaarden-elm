package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLead(t *testing.T) {
	require.Equal(t, `<p class="pb-4 text-xl text-sort">Lær å bake &lt;takker&gt;.</p>`, Lead("Lær å bake <takker>.").Render())
}
