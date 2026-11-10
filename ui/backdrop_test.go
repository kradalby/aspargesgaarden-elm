package ui

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestBackdrop(t *testing.T) {
	golden.HTML(t, "backdrop", Backdrop("Aspargesgården", testPhoto()).Render())
}
