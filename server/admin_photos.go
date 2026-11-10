package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// maxUploadBody is the upload limit plus room for the multipart framing,
// enforced on the request before anything is read.
const maxUploadBody = images.MaxUpload + 1<<20

// maxStatusIDs bounds one status poll.
const maxStatusIDs = 100

type rectJSON struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type photographerJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type statusJSON struct {
	State        string `json:"state"`
	Done         int    `json:"done"`
	Total        int    `json:"total"`
	PreviewReady bool   `json:"previewReady"`
	Reason       string `json:"reason"`
}

type photoJSON struct {
	ID           int64               `json:"id"`
	Width        int                 `json:"width"`
	Height       int                 `json:"height"`
	Alt          string              `json:"alt"`
	Photographer *photographerJSON   `json:"photographer"`
	PreviewURL   string              `json:"previewUrl"`
	ThumbURL     string              `json:"thumbUrl"`
	Crops        map[string]rectJSON `json:"crops"`
	Status       statusJSON          `json:"status"`
}

func toPhotographerJSON(p site.Photographer) photographerJSON {
	return photographerJSON{ID: p.ID, Name: p.Name, URL: p.URL}
}

func (s *Server) photoJSON(ctx context.Context, p site.Photo) photoJSON {
	img := p.Image()
	out := photoJSON{
		ID: p.ID, Width: p.Width, Height: p.Height, Alt: p.Alt,
		PreviewURL: images.PreviewURL(img),
		ThumbURL:   images.ThumbURL(img),
		Crops:      map[string]rectJSON{},
		Status:     s.photoStatus(ctx, p.SHA256),
	}

	if p.Photographer != nil {
		out.Photographer = new(toPhotographerJSON(*p.Photographer))
	}

	// A kind never set shows where the crop tool starts, so the app never
	// computes a default itself.
	for _, k := range site.CropKinds {
		r, ok := p.Crops[k]
		if !ok {
			c := images.CentredCrop(images.Kind(k), p.Width, p.Height)
			r = site.Rect{X: c.X, Y: c.Y, W: c.W, H: c.H}
		}

		out.Crops[string(k)] = rectJSON{X: r.X, Y: r.Y, W: r.W, H: r.H}
	}

	return out
}

func (s *Server) photoStatus(ctx context.Context, sha string) statusJSON {
	st, err := s.cfg.Admin.Photos.Status(ctx, sha)
	if err != nil {
		// Not in the photo store's view yet: it is a moment behind a write.
		return statusJSON{State: "preparing"}
	}

	out := statusJSON{Done: st.Ready, Total: st.Total, PreviewReady: st.Preview != ""}

	switch st.State {
	case images.StateReady:
		out.State = "ready"
	case images.StateFailed:
		out.State, out.Reason = "failed", string(st.Reason)
		if out.Reason == "" {
			out.Reason = string(images.ReasonInternal)
		}
	case images.StateQueued, images.StatePreparing:
		out.State = "preparing"
	}

	return out
}

func (s *Server) photoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/api/photos", s.uploadPhoto)
	mux.HandleFunc("GET /admin/api/photos/status", s.photoStatuses)
	mux.HandleFunc("GET /admin/api/photos/{id}", s.withPhoto(func(w http.ResponseWriter, r *http.Request, p site.Photo) {
		s.writeJSON(w, r, http.StatusOK, s.photoJSON(r.Context(), p))
	}))
	mux.HandleFunc("PUT /admin/api/photos/{id}", s.withPhoto(s.updatePhoto))
	mux.HandleFunc("PUT /admin/api/photos/{id}/crops", s.withPhoto(s.setCrops))
	mux.HandleFunc("POST /admin/api/photos/{id}/prepare", s.withPhoto(s.preparePhoto))
	mux.HandleFunc("DELETE /admin/api/photos/{id}", s.withPhoto(s.deletePhoto))

	mux.HandleFunc("GET /admin/api/photographers", s.listPhotographers)
	mux.HandleFunc("POST /admin/api/photographers", s.savePhotographer)
	mux.HandleFunc("PUT /admin/api/photographers/{id}", s.savePhotographer)

	mux.HandleFunc("GET /admin/api/gallery", s.writeGallery)
	mux.HandleFunc("PUT /admin/api/gallery", s.setGallery)
}

// pathID reads the {id} of a route; a malformed one is an unknown id.
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("id %q: %w", r.PathValue("id"), site.ErrNotFound)
	}

	return id, nil
}

func (s *Server) withPhoto(h func(http.ResponseWriter, *http.Request, site.Photo)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err == nil {
			var p site.Photo
			if p, err = s.cfg.Store.Photo(r.Context(), id); err == nil {
				h(w, r, p)
				return
			}
		}

		s.apiFail(w, r, err, nil)
	}
}

// uploadPhoto streams the "file" part through images.Intake, which stores
// it as received once it has proven to be a photo the site can use.
func (s *Server) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.ContentLength > maxUploadBody {
		s.writeProblems(w, r, http.StatusRequestEntityTooLarge, []problem{{Code: "too_large"}}, nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)

	mr, err := r.MultipartReader()
	if err != nil {
		s.apiFail(w, r, fmt.Errorf("%w: %s", errBadRequest, err.Error()), nil)
		return
	}

	var o images.Original

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			s.apiFail(w, r, fmt.Errorf("%w: no file part", errBadRequest), nil)
			return
		}

		if err != nil {
			s.uploadFailed(w, r, err)
			return
		}

		if part.FormName() != "file" {
			continue
		}

		o, err = s.cfg.Admin.Photos.Intake(ctx, part)
		if err != nil {
			s.uploadFailed(w, r, err)
			return
		}

		break
	}

	status := http.StatusOK

	p, err := s.cfg.Store.PhotoBySHA256(ctx, o.SHA256)
	switch {
	case err == nil:
		// The same bytes again: someone is about to use it, so the unused
		// photo cleanup must not take it from under them.
		err = s.cfg.Store.RenewPhoto(ctx, p.ID)
	case errors.Is(err, site.ErrNotFound):
		status = http.StatusCreated
		p, err = s.addPhoto(ctx, o)
	}

	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed(p.SHA256)
	s.audit(r, "photo uploaded", "photo", p.ID, "sha256", p.SHA256, "new", status == http.StatusCreated)

	s.writeJSON(w, r, status, s.photoJSON(ctx, p))
}

// addPhoto records a new upload with the centred crops of every kind it is
// large enough for; the rest stay unset, and the pages show it whole there.
func (s *Server) addPhoto(ctx context.Context, o images.Original) (site.Photo, error) {
	p, err := s.cfg.Store.AddPhoto(ctx, site.Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height})
	if err != nil {
		return site.Photo{}, err
	}

	crops := map[site.CropKind]site.Rect{}
	for _, k := range images.CropKinds {
		r, err := images.DefaultCrop(k, p.Width, p.Height)
		if err == nil {
			crops[site.CropKind(k)] = site.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
		}
	}

	if len(crops) > 0 {
		if err := s.cfg.Store.SetCrops(ctx, p.ID, crops); err != nil {
			return site.Photo{}, err
		}
	}

	return s.cfg.Store.Photo(ctx, p.ID)
}

// uploadFailed answers a refused upload with the codes docs/admin-api.md
// gives the app; an upload the client gave up on gets no answer at all.
func (s *Server) uploadFailed(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError

	code, status := "", 0

	switch {
	case errors.As(err, &tooBig):
		code, status = "too_large", http.StatusRequestEntityTooLarge
	case r.Context().Err() != nil:
		return
	default:
		switch images.ReasonOf(err) {
		case images.ReasonTooLarge:
			code, status = "too_large", http.StatusRequestEntityTooLarge
		case images.ReasonNotPhoto:
			code, status = "unsupported_type", http.StatusUnsupportedMediaType
		case images.ReasonDamaged:
			code, status = "damaged", http.StatusUnprocessableEntity
		case images.ReasonTooSmall:
			code, status = "too_small", http.StatusUnprocessableEntity
		case images.ReasonTooManyPixels:
			code, status = "too_many_pixels", http.StatusUnprocessableEntity
		case images.ReasonNone, images.ReasonCropOutside, images.ReasonCropAspect,
			images.ReasonMissingOriginal, images.ReasonInternal:
		}
	}

	if code == "" {
		s.apiFail(w, r, err, nil)
		return
	}

	s.cfg.Log.InfoContext(r.Context(), "admin: upload refused", "code", code, "user", userOf(r.Context()).Login, "err", err)
	s.writeProblems(w, r, status, []problem{{Code: code}}, nil)
}

// photoStatuses is the cheap poll: ids in, states out, unknown ids left out.
func (s *Server) photoStatuses(w http.ResponseWriter, r *http.Request) {
	var ids []int64

	for f := range strings.SplitSeq(r.URL.Query().Get("ids"), ",") {
		if f == "" {
			continue
		}

		id, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			s.apiFail(w, r, fmt.Errorf("%w: id %q", errBadRequest, f), nil)
			return
		}

		ids = append(ids, id)
	}

	if len(ids) > maxStatusIDs {
		s.apiFail(w, r, fmt.Errorf("%w: %d ids, at most %d", errBadRequest, len(ids), maxStatusIDs), nil)
		return
	}

	out := map[string]statusJSON{}

	for _, id := range ids {
		p, err := s.cfg.Store.Photo(r.Context(), id)
		if errors.Is(err, site.ErrNotFound) {
			continue
		}

		if err != nil {
			s.apiFail(w, r, err, nil)
			return
		}

		out[strconv.FormatInt(id, 10)] = s.photoStatus(r.Context(), p.SHA256)
	}

	s.writeJSON(w, r, http.StatusOK, map[string]any{"statuses": out})
}

func (s *Server) preparePhoto(w http.ResponseWriter, r *http.Request, p site.Photo) {
	st := s.photoStatus(r.Context(), p.SHA256)

	if st.State == "failed" {
		if err := s.cfg.Admin.Photos.Retry(r.Context(), p.SHA256); err != nil {
			s.apiFail(w, r, err, nil)
			return
		}

		s.audit(r, "photo retried", "photo", p.ID)
		st = s.photoStatus(r.Context(), p.SHA256)
	}

	s.writeJSON(w, r, http.StatusAccepted, st)
}

func (s *Server) updatePhoto(w http.ResponseWriter, r *http.Request, p site.Photo) {
	var in struct {
		Alt            string `json:"alt"`
		PhotographerID *int64 `json:"photographerId"`
	}

	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	p.Alt, p.Photographer = in.Alt, nil

	if in.PhotographerID != nil {
		ph, err := s.cfg.Store.Photographer(r.Context(), *in.PhotographerID)
		if err != nil {
			s.apiFail(w, r, &site.FieldError{Field: "photographerId", PhotoID: p.ID, Err: err}, nil)
			return
		}

		p.Photographer = &ph
	}

	p, err := s.cfg.Store.UpdatePhoto(r.Context(), p)
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "photo described", "photo", p.ID)
	s.writeJSON(w, r, http.StatusOK, s.photoJSON(r.Context(), p))
}

// setCrops checks every crop in the body against the photo and the kind's
// output sizes (the trust boundary for crops), then writes all or none.
func (s *Server) setCrops(w http.ResponseWriter, r *http.Request, p site.Photo) {
	var in map[string]rectJSON
	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	crops := map[site.CropKind]site.Rect{}

	var errs []error

	for _, k := range slices.Sorted(maps.Keys(in)) {
		c := in[k]

		err := images.ValidateCrop(images.Kind(k), p.Width, p.Height, images.Rect{X: c.X, Y: c.Y, W: c.W, H: c.H})
		if images.Kind(k) == images.Full {
			err = fmt.Errorf("%w: %q", images.ErrKind, k)
		}

		if err != nil {
			errs = append(errs, &site.FieldError{Field: site.Field(k), Err: err})
			continue
		}

		crops[site.CropKind(k)] = site.Rect{X: c.X, Y: c.Y, W: c.W, H: c.H}
	}

	if err := errors.Join(errs...); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	if err := s.cfg.Store.SetCrops(r.Context(), p.ID, crops); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	p, err := s.cfg.Store.Photo(r.Context(), p.ID)
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed(p.SHA256)
	s.audit(r, "photo cropped", "photo", p.ID, "kinds", slices.Sorted(maps.Keys(crops)))
	s.writeJSON(w, r, http.StatusOK, s.photoJSON(r.Context(), p))
}

func (s *Server) deletePhoto(w http.ResponseWriter, r *http.Request, p site.Photo) {
	usedBy, err := s.cfg.Store.RemovePhoto(r.Context(), p.ID)
	if err != nil {
		var extra map[string]any
		if len(usedBy) > 0 {
			refs := make([]map[string]any, len(usedBy))
			for i, e := range usedBy {
				refs[i] = map[string]any{"id": e.ID, "title": e.Title}
			}

			extra = map[string]any{"usedBy": refs}
		}

		s.apiFail(w, r, err, extra)

		return
	}

	s.Changed()
	s.audit(r, "photo deleted", "photo", p.ID, "sha256", p.SHA256)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listPhotographers(w http.ResponseWriter, r *http.Request) {
	ps, err := s.cfg.Store.Photographers(r.Context())
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	out := make([]photographerJSON, len(ps))
	for i, p := range ps {
		out[i] = toPhotographerJSON(p)
	}

	s.writeJSON(w, r, http.StatusOK, map[string]any{"photographers": out})
}

// savePhotographer creates (POST) or corrects (PUT {id}) a photographer. A
// name someone else has is answered with that one, for the app to offer.
func (s *Server) savePhotographer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	p := site.Photographer{Name: in.Name, URL: in.URL}

	var (
		err    error
		status = http.StatusCreated
	)

	if r.Method == http.MethodPut {
		status = http.StatusOK
		if p.ID, err = pathID(r); err == nil {
			p, err = s.cfg.Store.UpdatePhotographer(r.Context(), p)
		}
	} else {
		p, err = s.cfg.Store.CreatePhotographer(r.Context(), p)
	}

	if errors.Is(err, site.ErrPhotographerTaken) {
		s.apiFail(w, r, err, s.existingPhotographer(r.Context(), in.Name))
		return
	}

	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "photographer saved", "photographer", p.ID, "name", p.Name)
	s.writeJSON(w, r, status, toPhotographerJSON(p))
}

// existingPhotographer finds the photographer who already has the name, as
// site compares names: case and spacing do not count.
func (s *Server) existingPhotographer(ctx context.Context, name string) map[string]any {
	ps, err := s.cfg.Store.Photographers(ctx)
	if err != nil {
		return nil
	}

	want := strings.Join(strings.Fields(name), " ")
	for _, p := range ps {
		if strings.EqualFold(p.Name, want) {
			return map[string]any{"existing": toPhotographerJSON(p)}
		}
	}

	return nil
}

func (s *Server) writeGallery(w http.ResponseWriter, r *http.Request) {
	ps, err := s.cfg.Store.Gallery(r.Context())
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	out := make([]photoJSON, len(ps))
	for i, p := range ps {
		out[i] = s.photoJSON(r.Context(), p)
	}

	s.writeJSON(w, r, http.StatusOK, map[string]any{"photos": out})
}

// setGallery replaces the whole gallery: add, remove and reorder are all
// this one write.
func (s *Server) setGallery(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PhotoIDs *[]int64 `json:"photoIds"`
	}

	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	// A body without the list must not empty the gallery.
	if in.PhotoIDs == nil {
		s.apiFail(w, r, fmt.Errorf("%w: no photoIds", errBadRequest), nil)
		return
	}

	ids := *in.PhotoIDs
	seen := map[int64]bool{}

	for _, id := range ids {
		if seen[id] {
			s.apiFail(w, r, &site.FieldError{PhotoID: id, Err: site.ErrDuplicatePhoto}, nil)
			return
		}

		seen[id] = true

		if _, err := s.cfg.Store.Photo(r.Context(), id); err != nil {
			s.apiFail(w, r, &site.FieldError{PhotoID: id, Err: err}, nil)
			return
		}
	}

	if err := s.cfg.Store.SetGallery(r.Context(), ids); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "gallery saved", "photos", ids)
	s.writeGallery(w, r)
}
