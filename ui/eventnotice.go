package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Notice is a state of an event a visitor must not miss.
type Notice int

// The notices. The zero value is invalid.
const (
	_ Notice = iota
	// NoticeOver marks a past event; its page stays for the record.
	NoticeOver
	// NoticeCancelled marks an event that will not take place.
	NoticeCancelled
)

// EventNotice is a band at the top of an event page saying it is over or
// cancelled. Over points on to what is coming.
func EventNotice(n Notice) elem.Node {
	switch n {
	case NoticeOver:
		return elem.P(attrs.Props{attrs.Class: "my-4 p-4 bg-brown/20 text-sort"},
			elem.Strong(attrs.Props{attrs.Class: "font-bold"}, elem.Text("Arrangementet er over.")),
			elem.Text(" "),
			elem.A(attrs.Props{attrs.Href: "/arrangementer", attrs.Class: "underline"}, elem.Text("Se kommende arrangementer")))
	case NoticeCancelled:
		return elem.P(attrs.Props{attrs.Class: "my-4 p-4 bg-rust text-white font-bold"}, elem.Text("Arrangementet er avlyst."))
	}

	panic("ui: EventNotice needs a Notice")
}
