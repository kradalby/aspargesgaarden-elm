// Package site holds aspargesgaarden.no's domain rules: events and their
// lifecycle, slugs, upcoming vs past on the Oslo calendar, the Norwegian date
// line, the menu, photos and the gallery. Time is always passed in, so every
// rule is testable without waiting. Store maps the database to these types;
// nothing outside it sees sqlc rows.
package site

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidDate       = errors.New("invalid date, want YYYY-MM-DD")
	ErrInvalidTime       = errors.New("invalid time, want HH:MM")
	ErrSlugInvalid       = errors.New("invalid slug")
	ErrSlugReserved      = errors.New("slug is a fixed address")
	ErrSlugTaken         = errors.New("slug is already used that year")
	ErrSlugFrozen        = errors.New("slug cannot change after first publish")
	ErrEndBeforeStart    = errors.New("event ends before it starts")
	ErrExtraDate         = errors.New("extra dates must come after the start date, once each")
	ErrTicketURL         = errors.New("ticket link must be an http or https address")
	ErrTitleRequired     = errors.New("title is required")
	ErrStartRequired     = errors.New("start date is required")
	ErrMainPhotoRequired = errors.New("main photo is required")
	ErrAltRequired       = errors.New("photo needs alt text")
	ErrDuplicatePhoto    = errors.New("photo used twice in one event")
	ErrPhotoInUse        = errors.New("photo is used by an event or the gallery")
	ErrCropOutside       = errors.New("crop is outside the photo")
	ErrCropKind          = errors.New("unknown crop kind")
)
