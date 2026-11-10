package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/db/dbsqlc"
	"github.com/kradalby/asparges/images"
)

// SetCrops sets several crops of a photo at once: all of them or, if one is
// refused, none. It returns the photo as stored.
func (s *Store) SetCrops(ctx context.Context, photoID int64, crops map[images.Kind]images.Rect) (Photo, error) {
	return write(ctx, s, func(q *dbsqlc.Queries) (Photo, error) {
		row, err := q.GetPhoto(ctx, photoID)
		if errors.Is(err, sql.ErrNoRows) {
			return Photo{}, fmt.Errorf("photo %d: %w", photoID, ErrNotFound)
		}
		if err != nil {
			return Photo{}, fmt.Errorf("loading photo %d: %w", photoID, err)
		}

		p := photoFromRow(row)

		var errs []error
		for _, kind := range slices.Sorted(maps.Keys(crops)) {
			// A crop's field is its kind: the admin shows the error there.
			errs = append(errs, fieldErr(Field(kind), p.checkCrop(kind, crops[kind])))
		}

		if err := errors.Join(errs...); err != nil {
			return Photo{}, err
		}

		for kind, r := range crops {
			if err := upsertCrop(ctx, q, photoID, kind, r); err != nil {
				return Photo{}, err
			}
		}

		// A new crop changes the pages that show the photo.
		if err := q.TouchPhoto(ctx, dbsqlc.TouchPhotoParams{UpdatedAt: s.now().Unix(), ID: photoID}); err != nil {
			return Photo{}, fmt.Errorf("touching photo %d: %w", photoID, err)
		}

		return photo(ctx, q, photoID)
	})
}

func upsertCrop(ctx context.Context, q *dbsqlc.Queries, photoID int64, kind images.Kind, r images.Rect) error {
	err := q.UpsertPhotoCrop(ctx, dbsqlc.UpsertPhotoCropParams{
		PhotoID: photoID, Kind: string(kind),
		X: int64(r.X), Y: int64(r.Y), W: int64(r.W), H: int64(r.H),
	})
	if err != nil {
		return fmt.Errorf("setting %s crop of photo %d: %w", kind, photoID, err)
	}

	return nil
}

// RemovePhoto deletes a photo that no event shows, taking it out of the
// gallery first. If events show it, nothing changes and they are returned
// (ID and Title only) with ErrPhotoInUse.
func (s *Store) RemovePhoto(ctx context.Context, id int64) ([]Event, error) {
	var usedBy []Event

	err := s.db.InTx(ctx, func(q *dbsqlc.Queries) error {
		rows, err := q.ListPhotoEvents(ctx, id)
		if err != nil {
			return fmt.Errorf("events of photo %d: %w", id, err)
		}

		if len(rows) > 0 {
			for _, r := range rows {
				usedBy = append(usedBy, Event{ID: r.ID, Title: r.Title})
			}

			return fmt.Errorf("photo %d: %w", id, ErrPhotoInUse)
		}

		if _, err := q.DeleteGalleryPhoto(ctx, id); err != nil {
			return fmt.Errorf("taking photo %d out of the gallery: %w", id, err)
		}

		n, err := q.DeletePhoto(ctx, id)
		switch {
		case db.IsForeignKeyViolation(err):
			return fmt.Errorf("photo %d: %w", id, ErrPhotoInUse)
		case err != nil:
			return fmt.Errorf("deleting photo %d: %w", id, err)
		case n == 0:
			return fmt.Errorf("photo %d: %w", id, ErrNotFound)
		}

		return nil
	})

	return usedBy, err
}

// DeleteUnusedPhotos deletes the photos no event and no gallery uses that
// nobody uploaded, cropped, described or took out of use for age: uploads
// for a form that was never saved, and photos an event or the gallery no
// longer shows. It returns their sha256s; images/ removes their files.
func (s *Store) DeleteUnusedPhotos(ctx context.Context, age time.Duration) ([]string, error) {
	shas, err := s.db.Queries().DeleteUnusedPhotos(ctx, s.now().Add(-age).Unix())
	if err != nil {
		return nil, fmt.Errorf("deleting unused photos: %w", err)
	}

	return shas, nil
}
