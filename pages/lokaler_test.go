package pages

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestLokaler(t *testing.T) {
	golden.HTML(t, "lokaler", Lokaler(testSite).Render())
}
