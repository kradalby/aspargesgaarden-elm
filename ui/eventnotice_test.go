package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEventNotice(t *testing.T) {
	golden.HTML(t, "eventnotice-over", EventNotice(NoticeOver).Render())
	golden.HTML(t, "eventnotice-cancelled", EventNotice(NoticeCancelled).Render())
	require.Panics(t, func() { EventNotice(0) })
}
