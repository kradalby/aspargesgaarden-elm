package pages

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestKontakt(t *testing.T) {
	golden.HTML(t, "kontakt", Kontakt(testSite).Render())
}
