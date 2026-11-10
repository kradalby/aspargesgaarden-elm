package ui

import (
	"strings"
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestPhotoRows(t *testing.T) {
	row := Row{Photo: testPhoto(), Alt: "Låven", By: Photographer{Name: "Marianne Ous"}, Body: []elem.Node{elem.Text("Tekst")}}

	golden.HTML(t, "photorows-facility", PhotoRows(RowFacility, row, row, row).Render())

	ev := PhotoRows(RowEvent, row).Render()
	require.Contains(t, ev, "h-1/2")
	require.Contains(t, ev, "break-words")
}

func TestPhotoRowsOnlyFirstPhotoIsHero(t *testing.T) {
	row := Row{Photo: testPhoto(), Alt: "a"}
	got := PhotoRows(RowFacility, row, row, row).Render()

	require.Equal(t, 1, strings.Count(got, `fetchpriority="high"`))
	require.Equal(t, 2, strings.Count(got, `loading="lazy"`))
}

func TestPhotoRowsNeedsAKind(t *testing.T) {
	require.Panics(t, func() { PhotoRows(0) })
}
