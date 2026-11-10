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

// PhotoBySHA256 loads the photo whose original has this content hash.
func (s *Store) PhotoBySHA256(ctx context.Context, sha256 string) (Photo, error) {
	row, err := s.db.Queries().GetPhotoBySha256(ctx, sha256)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, fmt.Errorf("photo %s: %w", sha256, ErrNotFound)
	}
	if err != nil {
		return Photo{}, fmt.Errorf("finding photo %s: %w", sha256, err)
	}

	return s.Photo(ctx, row.ID)
}

// RenewPhoto restarts a photo's unused grace period (DeleteUnusedPhotos):
// uploading a photo again means someone is about to use it.
func (s *Store) RenewPhoto(ctx context.Context, id int64) error {
	err := s.db.Queries().TouchPhoto(ctx, dbsqlc.TouchPhotoParams{UpdatedAt: s.now().Unix(), ID: id})
	if err != nil {
		return fmt.Errorf("renewing photo %d: %w", id, err)
	}

	return nil
}

// SetCrops sets several crops of a photo at once: all of them or, if one is
// refused, none.
func (s *Store) SetCrops(ctx context.Context, photoID int64, crops map[images.Kind]images.Rect) error {
	return s.db.InTx(ctx, func(q *dbsqlc.Queries) error {
		row, err := q.GetPhoto(ctx, photoID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("photo %d: %w", photoID, ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("loading photo %d: %w", photoID, err)
		}

		p := photoFromRow(row)

		var errs []error
		for _, kind := range slices.Sorted(maps.Keys(crops)) {
			// A crop's field is its kind: the admin shows the error there.
			errs = append(errs, fieldErr(Field(kind), p.checkCrop(kind, crops[kind])))
		}

		if err := errors.Join(errs...); err != nil {
			return err
		}

		for kind, r := range crops {
			if err := q.UpsertPhotoCrop(ctx, dbsqlc.UpsertPhotoCropParams{
				PhotoID: photoID, Kind: string(kind),
				X: int64(r.X), Y: int64(r.Y), W: int64(r.W), H: int64(r.H),
			}); err != nil {
				return fmt.Errorf("setting %s crop of photo %d: %w", kind, photoID, err)
			}
		}

		// A new crop changes the pages that show the photo.
		if err := q.TouchPhoto(ctx, dbsqlc.TouchPhotoParams{UpdatedAt: s.now().Unix(), ID: photoID}); err != nil {
			return fmt.Errorf("touching photo %d: %w", photoID, err)
		}

		return nil
	})
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

		if err := q.DeleteGalleryPhoto(ctx, id); err != nil {
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
// nobody uploaded, cropped or described for age: uploads for a form that was
// never saved, and photos an event no longer shows. It returns their
// sha256s; images/ removes their files on its next reconcile.
func (s *Store) DeleteUnusedPhotos(ctx context.Context, age time.Duration) ([]string, error) {
	shas, err := s.db.Queries().DeleteUnusedPhotos(ctx, s.now().Add(-age).Unix())
	if err != nil {
		return nil, fmt.Errorf("deleting unused photos: %w", err)
	}

	return shas, nil
}
