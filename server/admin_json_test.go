package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

func TestProblems(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		want   []problem
	}{
		{"not found", fmt.Errorf("event 4: %w", site.ErrNotFound), http.StatusNotFound, []problem{{Code: "not_found"}}},
		{"bad body", fmt.Errorf("%w: unexpected EOF", errBadRequest), http.StatusBadRequest, []problem{{Code: "bad_request"}}},
		{"stale", fmt.Errorf("x: %w", site.ErrStale), http.StatusConflict, []problem{{Code: "conflict"}}},
		{
			"every publish rule, with fields",
			site.Event{Status: site.Published, ExtraPhotos: []site.Photo{{ID: 13}}}.Validate(),
			http.StatusUnprocessableEntity,
			[]problem{
				{Code: "title_required", Field: "title"},
				{Code: "start_required", Field: "start"},
				{Code: "slug_invalid", Field: "slug"},
				{Code: "main_photo_required", Field: "mainPhoto"},
				{Code: "alt_required", Field: "extraPhotos", PhotoID: 13},
			},
		},
		{
			"wrapped join",
			fmt.Errorf("saving: %w", errors.Join(
				&site.FieldError{Field: site.FieldStartTime, Err: site.ErrInvalidTime},
				&site.FieldError{Field: site.FieldEnd, Err: site.ErrEndBeforeStart},
			)),
			http.StatusUnprocessableEntity,
			[]problem{{Code: "invalid_time", Field: "startTime"}, {Code: "end_before_start", Field: "end"}},
		},
		{
			"photographer rules take their own field",
			errors.Join(site.ErrPhotographerName, fmt.Errorf("%w: %q", site.ErrPhotographerURL, "x")),
			http.StatusUnprocessableEntity,
			[]problem{{Code: "photographer_name_required", Field: "name"}, {Code: "photographer_url", Field: "url"}},
		},
		{"name taken", site.ErrPhotographerTaken, http.StatusConflict, []problem{{Code: "photographer_name_taken", Field: "name"}}},
		{"in use", site.ErrPhotoInUse, http.StatusConflict, []problem{{Code: "photo_in_use"}}},
		{
			"crops by kind",
			errors.Join(
				&site.FieldError{Field: "card", Err: images.ErrCropAspect},
				&site.FieldError{Field: "share", Err: images.ErrTooSmall},
				&site.FieldError{Field: "banner", Err: images.ErrCropOutside},
				&site.FieldError{Field: "full", Err: images.ErrKind},
			),
			http.StatusUnprocessableEntity,
			[]problem{
				{Code: "crop_aspect", Field: "card"},
				{Code: "crop_too_small", Field: "share"},
				{Code: "crop_outside", Field: "banner"},
				{Code: "crop_kind", Field: "full"},
			},
		},
		{
			"unknown photo on an event",
			&site.FieldError{Field: site.FieldMainPhoto, PhotoID: 12, Err: site.ErrNotFound},
			http.StatusNotFound,
			[]problem{{Code: "not_found", Field: "mainPhoto", PhotoID: 12}},
		},
		// The database's words never reach the app, even next to a rule.
		{"internal", errors.New("database is locked"), http.StatusInternalServerError, []problem{{Code: "internal"}}},
		{
			"internal beside a rule",
			errors.Join(site.ErrTitleRequired, errors.New("disk full")),
			http.StatusInternalServerError,
			[]problem{{Code: "internal"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, got := problems(tt.err)
			require.Equal(t, tt.status, status)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestInternalErrorsStayInTheLog(t *testing.T) {
	a := newAdmin(t, false)

	rec := httptest.NewRecorder()
	a.srv.apiFail(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/admin/api/events", nil),
		fmt.Errorf("listing events: %w", errors.New("SQL logic error: no such table: events")), nil)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"errors":[{"code":"internal"}]}`, rec.Body.String())
	require.Contains(t, a.logs.String(), "no such table", "the cause is in the log")
}
