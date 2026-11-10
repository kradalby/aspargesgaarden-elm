package site

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexNowKeyIsMadeOnceAndKept(t *testing.T) {
	s, _ := newTestStore(t, time.Now())

	first, err := s.IndexNowKey(t.Context())
	require.NoError(t, err)
	require.Regexp(t, `^[0-9a-f]{32}$`, first)

	again, err := s.IndexNowKey(t.Context())
	require.NoError(t, err)
	require.Equal(t, first, again)
}
