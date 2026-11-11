package site

import "fmt"

// Field names a part of an event that a rule is about, so a form can put the
// complaint next to it. The names are the admin API's keys.
type Field string

const (
	FieldTitle       Field = "title"
	FieldSlug        Field = "slug"
	FieldStart       Field = "start"
	FieldStartTime   Field = "startTime"
	FieldEnd         Field = "end"
	FieldEndTime     Field = "endTime"
	FieldExtraDates  Field = "extraDates"
	FieldTicketURL   Field = "ticketUrl"
	FieldMainPhoto   Field = "mainPhoto"
	FieldExtraPhotos Field = "extraPhotos"
	FieldPoster      Field = "poster"
	// FieldAlt is a photo's description where it is edited on its own.
	FieldAlt Field = "alt"
)

// FieldError is a broken rule tied to the field, and for photos the photo,
// it is about. errors.Is still sees the rule's sentinel through it.
type FieldError struct {
	Field   Field
	PhotoID int64 // 0 unless the rule is about one photo
	Err     error
}

func (e *FieldError) Error() string {
	if e.PhotoID != 0 {
		return fmt.Sprintf("%s (photo %d): %v", e.Field, e.PhotoID, e.Err)
	}

	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

func fieldErr(f Field, err error) error {
	if err == nil {
		return nil
	}

	return &FieldError{Field: f, Err: err}
}

// photoField says whether photo id is e's main photo, its poster or one of
// the others.
func (e Event) photoField(id int64) Field {
	if e.MainPhoto != nil && e.MainPhoto.ID == id {
		return FieldMainPhoto
	}

	if e.Poster != nil && e.Poster.ID == id {
		return FieldPoster
	}

	return FieldExtraPhotos
}

func (e Event) photoErr(id int64, err error) error {
	return &FieldError{Field: e.photoField(id), PhotoID: id, Err: fmt.Errorf("%w: photo %d", err, id)}
}
