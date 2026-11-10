package ui

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestInfoLine(t *testing.T) {
	golden.HTML(t, "infoline", InfoLine(IconPhone, "Telefon", "+47 90 91 98 12").Render())
}
