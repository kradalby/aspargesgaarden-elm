package ui

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestContactLink(t *testing.T) {
	golden.HTML(t, "contactlink", ContactLink().Render())
}
