package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeadline(t *testing.T) {
	require.Equal(t,
		`<h1 class="text-center py-6 text-4xl font-medium font-display text-tre md:text-left md:text-6xl lg:pt-12">Om gården &amp; oss</h1>`,
		Headline("Om gården & oss").Render())
}
