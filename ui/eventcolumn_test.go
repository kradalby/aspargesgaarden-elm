package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/golden"
)

func TestEventColumn(t *testing.T) {
	golden.HTML(t, "eventcolumn", EventColumn(Headline("Julemarked"), elem.P(nil, elem.Text("Tekst"))).Render())
}
