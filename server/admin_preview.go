package server

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/site"
)

// preview renders the unsaved form with the public pages' own builders, as
// the event's page or in the event list, without writing or keeping
// anything. It renders whatever the rules say: a draft is shown while it is
// being typed.
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var in struct {
		View  string     `json:"view"`
		Event eventInput `json:"event"`
		// ID is the event being edited, 0 for a new one. With it, a
		// published event's frozen address is kept, and the list shows
		// the draft in its place rather than next to its saved self.
		ID int64 `json:"id"`
	}

	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	if in.View != "page" && in.View != "list" {
		s.apiFail(w, r, fmt.Errorf("%w: view %q, want page or list", errBadRequest, in.View), nil)
		return
	}

	e, err := s.toEvent(ctx, in.Event, false)
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	var prev *site.Event
	if in.ID != 0 {
		p, err := s.cfg.Store.Event(ctx, in.ID)
		if err != nil {
			s.apiFail(w, r, err, nil)
			return
		}

		prev = &p
	}

	e = site.Apply(prev, e)

	st, err := s.site(ctx)
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	now := s.cfg.Now()

	var page *elem.Element

	if in.View == "page" {
		page = pages.Event(st, e, now)
	} else {
		events, err := s.cfg.Store.PublishedEvents(ctx)
		if err != nil {
			s.apiFail(w, r, err, nil)
			return
		}

		if e.ID != 0 {
			events = slices.DeleteFunc(events, func(o site.Event) bool { return o.ID == e.ID })
		}

		page = pages.Events(st, append(events, e), now)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(asPreview(page, in.View).Render()))
}

// asPreview marks a page as the admin's preview and drops its scripts: the
// frame runs none, and a preview is not a visit to count.
func asPreview(page *elem.Element, view string) *elem.Element {
	if page.Attrs == nil {
		page.Attrs = map[string]string{}
	}

	page.Attrs["data-preview"] = view

	for _, n := range page.Children {
		if head, ok := n.(*elem.Element); ok && head.Tag == "head" {
			head.Children = slices.DeleteFunc(head.Children, func(c elem.Node) bool {
				el, ok := c.(*elem.Element)
				return ok && el.Tag == "script"
			})
		}
	}

	return page
}
