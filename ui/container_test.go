package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"
)

func TestContainer(t *testing.T) {
	require.Equal(t, `<div class="flex flex-col md:px-24"><p></p></div>`, Container(elem.P(nil)).Render())
}
