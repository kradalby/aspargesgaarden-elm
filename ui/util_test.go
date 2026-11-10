package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCls(t *testing.T) {
	require.Equal(t, "a c", cls("a", onClass(false, "b"), "", onClass(true, "c")))
	require.Empty(t, cls())
}
