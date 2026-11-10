package site

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	tests := []struct{ title, want string }{
		{"Julemarked på Aspargesgården", "julemarked-pa-aspargesgarden"},
		{"Vårkonsert med Black & Lekker", "varkonsert-med-black-lekker"},
		{"Afternoon tea - ladies and gentlemen", "afternoon-tea-ladies-and-gentlemen"},
		{"O sole mio med Didrik Solli-Tangen", "o-sole-mio-med-didrik-solli-tangen"},
		{"Smørbrød og kaffe", "smorbrod-og-kaffe"},
		{"ÆRLIG TALT, Østlandet", "aerlig-talt-ostlandet"},
		{"Crème brûlée à la Müller", "creme-brulee-a-la-muller"},
		{"Vårkonsert 2026!", "varkonsert-2026"},
		{"«Do & don’ts» på fest", "do-don-ts-pa-fest"},
		{"  --  ", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			require.Equal(t, tt.want, Slugify(tt.title))
		})
	}
}

func TestSlugifyLong(t *testing.T) {
	title := strings.Repeat("julemarked ", 20)
	got := Slugify(title)
	require.LessOrEqual(t, len(got), maxSlugLen)
	require.NoError(t, ValidateSlug(got))
	require.True(t, strings.HasSuffix(got, "-julemarked"), "cut at a word boundary: %s", got)

	got = Slugify(strings.Repeat("a", 100))
	require.Len(t, got, maxSlugLen)
}

func TestValidateSlug(t *testing.T) {
	for _, ok := range []string{"julemarked", "takkebakstkurs-med-anita-saga", "kurs-2026", "a"} {
		require.NoError(t, ValidateSlug(ok), ok)
	}

	for _, bad := range []string{"", "Julemarked", "vår", "-a", "a-", "a--b", "a b", "a/b", "2026", strings.Repeat("a", maxSlugLen+1)} {
		require.ErrorIs(t, ValidateSlug(bad), ErrSlugInvalid, bad)
	}

	for _, reserved := range []string{"admin", "galleri", "om-oss", "christmas", "static"} {
		require.ErrorIs(t, ValidateSlug(reserved), ErrSlugReserved, reserved)
	}

	require.NoError(t, ValidateSlug("julemarked"), "julemarked is an event slug, not a fixed route")
}
