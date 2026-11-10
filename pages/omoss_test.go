package pages

import (
	"testing"

	"github.com/kradalby/asparges/golden"
)

func TestOmOss(t *testing.T) {
	golden.HTML(t, "omoss", OmOss(testSite).Render())
}
