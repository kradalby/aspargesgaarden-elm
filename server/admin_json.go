package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// maxJSONBody bounds a JSON request: the longest is an event with its text.
const maxJSONBody = 1 << 20

// errBadRequest is a body that is not the JSON the route takes.
var errBadRequest = errors.New("bad request body")

// problem is one entry of the error list docs/admin-api.md defines. The app
// owns the words; codes and fields are all it gets.
type problem struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	PhotoID int64  `json:"photoId,omitempty"`
}

// codes maps every rule error to its code and status. A rule without a
// field of its own gets the field shown here unless its error names one.
var codes = []struct {
	err    error
	code   string
	status int
	field  string
}{
	{errBadRequest, "bad_request", http.StatusBadRequest, ""},
	{site.ErrNotFound, "not_found", http.StatusNotFound, ""},
	{images.ErrNotFound, "not_found", http.StatusNotFound, ""},
	{site.ErrStale, "conflict", http.StatusConflict, ""},
	{site.ErrInvalidDate, "invalid_date", http.StatusUnprocessableEntity, ""},
	{site.ErrInvalidTime, "invalid_time", http.StatusUnprocessableEntity, ""},
	{site.ErrSlugInvalid, "slug_invalid", http.StatusUnprocessableEntity, "slug"},
	{site.ErrSlugReserved, "slug_reserved", http.StatusUnprocessableEntity, "slug"},
	{site.ErrSlugTaken, "slug_taken", http.StatusUnprocessableEntity, "slug"},
	{site.ErrSlugFrozen, "slug_frozen", http.StatusUnprocessableEntity, "slug"},
	{site.ErrEndBeforeStart, "end_before_start", http.StatusUnprocessableEntity, ""},
	{site.ErrExtraDate, "extra_date", http.StatusUnprocessableEntity, "extraDates"},
	{site.ErrTicketURL, "ticket_url", http.StatusUnprocessableEntity, "ticketUrl"},
	{site.ErrTitleRequired, "title_required", http.StatusUnprocessableEntity, "title"},
	{site.ErrStartRequired, "start_required", http.StatusUnprocessableEntity, "start"},
	{site.ErrMainPhotoRequired, "main_photo_required", http.StatusUnprocessableEntity, "mainPhoto"},
	{site.ErrAltRequired, "alt_required", http.StatusUnprocessableEntity, ""},
	{site.ErrDuplicatePhoto, "duplicate_photo", http.StatusUnprocessableEntity, ""},
	{site.ErrPhotographerName, "photographer_name_required", http.StatusUnprocessableEntity, "name"},
	{site.ErrPhotographerTaken, "photographer_name_taken", http.StatusConflict, "name"},
	{site.ErrPhotographerURL, "photographer_url", http.StatusUnprocessableEntity, "url"},
	{site.ErrPhotoInUse, "photo_in_use", http.StatusConflict, ""},
	{images.ErrKind, "crop_kind", http.StatusUnprocessableEntity, ""},
	{images.ErrCropOutside, "crop_outside", http.StatusUnprocessableEntity, ""},
	{images.ErrCropAspect, "crop_aspect", http.StatusUnprocessableEntity, ""},
	// Uploads map their own errors (uploadProblem); here it is a crop.
	{images.ErrTooSmall, "crop_too_small", http.StatusUnprocessableEntity, ""},
}

// problems turns an error, joined or not, into the error list and its
// status. Anything it does not know is a 500 with the code "internal": its
// text stays in the log.
func problems(err error) (int, []problem) {
	var (
		out    []problem
		status int
	)

	for _, leaf := range leaves(err) {
		p, st, ok := classify(leaf)
		if !ok {
			return http.StatusInternalServerError, []problem{{Code: "internal"}}
		}

		if status == 0 {
			status = st
		}

		out = append(out, p)
	}

	return status, out
}

func classify(err error) (problem, int, bool) {
	for _, c := range codes {
		if !errors.Is(err, c.err) {
			continue
		}

		p := problem{Code: c.code, Field: c.field}

		var fe *site.FieldError
		if errors.As(err, &fe) {
			p.Field, p.PhotoID = string(fe.Field), fe.PhotoID
		}

		return p, c.status, true
	}

	return problem{}, 0, false
}

// leaves flattens err into the single errors it joins, so each broken rule
// is reported on its own, also when the join is wrapped.
func leaves(err error) []error {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			var out []error
			for _, inner := range j.Unwrap() {
				out = append(out, leaves(inner)...)
			}

			return out
		}
	}

	if err == nil {
		return nil
	}

	return []error{err}
}

// apiFail answers an error from a store with its problems. extra adds keys
// next to "errors" (current, usedBy, existing).
func (s *Server) apiFail(w http.ResponseWriter, r *http.Request, err error, extra map[string]any) {
	status, ps := problems(err)
	if status >= http.StatusInternalServerError {
		s.cfg.Log.ErrorContext(r.Context(), "admin: request failed",
			"method", r.Method, "path", logPath(r.URL.Path), "user", userOf(r.Context()).Login, "err", err)
	} else {
		s.cfg.Log.DebugContext(r.Context(), "admin: refused by a rule", "path", logPath(r.URL.Path), "err", err)
	}

	s.writeProblems(w, r, status, ps, extra)
}

func (s *Server) writeProblems(w http.ResponseWriter, r *http.Request, status int, ps []problem, extra map[string]any) {
	body := map[string]any{"errors": ps}
	for k, v := range extra {
		body[k] = v
	}

	s.writeJSON(w, r, status, body)
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		s.cfg.Log.ErrorContext(r.Context(), "admin: encode response", "err", err)
		http.Error(w, `{"errors":[{"code":"internal"}]}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// readJSON decodes a JSON body into v. Only application/json is read: a
// cross-site form cannot send it without the browser asking first, which
// backs up the origin check.
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return fmt.Errorf("%w: content type %q", errBadRequest, r.Header.Get("Content-Type"))
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(v); err != nil {
		// Text, not a wrap: the decoder's error is no rule of its own.
		return fmt.Errorf("%w: %s", errBadRequest, err.Error())
	}

	return nil
}
