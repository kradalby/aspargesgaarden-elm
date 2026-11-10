package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/golden"
)

func TestInfoLine(t *testing.T) {
	golden.HTML(t, "infoline", elem.Fragment(
		InfoLine(IconPhone, "Telefon", "+47 90 91 98 12", "tel:+4790919812"),
		InfoLine(IconLocation, "Adresse", "Østbyveien 75, 3280 Tjodalyng", ""),
	).Render())
}
