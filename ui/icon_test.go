package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Every icon constant names an embedded file (static.URL panics otherwise).
func TestIconsAreEmbedded(t *testing.T) {
	for _, i := range []Icon{
		IconBryllup, IconMinnestund, IconDaap, IconJulebord, IconBursdag, IconKonferanse,
		IconFeiring, IconNyttaar, IconVitnemaal, IconPhone, IconMail, IconLocation,
	} {
		require.Contains(t, IconImage(i, "x").Render(), `alt="x"`)
	}
}
