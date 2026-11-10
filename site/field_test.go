package site

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// fieldsOf lists the field (and photo) of every FieldError joined in err.
func fieldsOf(err error) []FieldError {
	var out []FieldError

	var walk func(error)
	walk = func(err error) {
		if j, ok := err.(interface{ Unwrap() []error }); ok {
			for _, e := range j.Unwrap() {
				walk(e)
			}

			return
		}

		var fe *FieldError
		if errors.As(err, &fe) {
			out = append(out, FieldError{Field: fe.Field, PhotoID: fe.PhotoID})
		}
	}
	walk(err)

	return out
}

func TestFieldErrors(t *testing.T) {
	base := Event{Title: "Kurs", Slug: "kurs", Start: d("2026-10-31"), StartTime: "10:00"}

	tests := []struct {
		name string
		mod  func(*Event)
		want []FieldError
	}{
		{"bad start time", func(e *Event) { e.StartTime = "25:00" }, []FieldError{{Field: FieldStartTime}}},
		{"bad end time", func(e *Event) { e.EndTime = "ca. 15" }, []FieldError{{Field: FieldEndTime}}},
		{"end day before start", func(e *Event) { e.End = d("2026-10-30") }, []FieldError{{Field: FieldEnd}}},
		{"end time before start time", func(e *Event) { e.EndTime = "09:00" }, []FieldError{{Field: FieldEndTime}}},
		{"extra date", func(e *Event) { e.ExtraDates = []Date{d("2026-10-01")} }, []FieldError{{Field: FieldExtraDates}}},
		{"ticket link", func(e *Event) { e.TicketURL = "x" }, []FieldError{{Field: FieldTicketURL}}},
		{"slug", func(e *Event) { e.Slug = "admin" }, []FieldError{{Field: FieldSlug}}},
		{"photo twice", func(e *Event) {
			e.MainPhoto = &Photo{ID: 1}
			e.ExtraPhotos = []Photo{{ID: 1}}
		}, []FieldError{{Field: FieldExtraPhotos, PhotoID: 1}}},
		{"publish rules", func(e *Event) {
			e.Status = Published
			e.Title, e.Start, e.StartTime, e.Slug = "", Date{}, "", ""
			e.ExtraPhotos = []Photo{{ID: 2}, {ID: 3, Alt: "Boder"}}
		}, []FieldError{
			{Field: FieldTitle},
			{Field: FieldStart},
			{Field: FieldSlug},
			{Field: FieldMainPhoto},
			{Field: FieldExtraPhotos, PhotoID: 2},
		}},
		{"alt on the main photo", func(e *Event) {
			e.Status = Published
			e.MainPhoto = &Photo{ID: 5}
		}, []FieldError{{Field: FieldMainPhoto, PhotoID: 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.mod(&e)
			require.Equal(t, tt.want, fieldsOf(e.Validate()))
		})
	}
}

func TestFieldErrorKeepsTheRule(t *testing.T) {
	err := Event{Status: Published}.Validate()
	require.ErrorIs(t, err, ErrTitleRequired)
	require.ErrorIs(t, err, ErrMainPhotoRequired)
	require.ErrorContains(t, err, "title: title is required")
}
