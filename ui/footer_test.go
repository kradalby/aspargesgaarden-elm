package ui

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestFooter(t *testing.T) {
	golden.HTML(t, "footer", Footer().Render())
}
