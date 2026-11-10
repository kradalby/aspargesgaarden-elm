package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kradalby/asparges/db/dbsqlc"
	"github.com/kradalby/asparges/images"
)

// Image is p as the images package sees it: the original and its crops.
func (p Photo) Image() images.Photo {
	return images.Photo{SHA256: p.SHA256, MIME: p.MIME, Width: p.Width, Height: p.Height, Crops: p.Crops}
}

// Catalog is the store as the images package's source of truth: which
// photos and crops exist, and so which files may.
func (s *Store) Catalog() images.Catalog { return catalog{s} }

type catalog struct{ s *Store }

// Photo looks the photo up and loads it in one read, so a delete cannot
// land between the two and turn "no such photo" into an error of another
// kind.
func (c catalog) Photo(ctx context.Context, sha256 string) (images.Photo, error) {
	return read(ctx, c.s, func(q *dbsqlc.Queries) (images.Photo, error) {
		id, err := q.GetPhotoIDBySha256(ctx, sha256)
		if errors.Is(err, sql.ErrNoRows) {
			return images.Photo{}, fmt.Errorf("photo %s: %w", sha256, images.ErrNotFound)
		}

		if err != nil {
			return images.Photo{}, fmt.Errorf("finding photo %s: %w", sha256, err)
		}

		p, err := photo(ctx, q, id)
		if err != nil {
			return images.Photo{}, err
		}

		return p.Image(), nil
	})
}

// Photos fails rather than return a short list: images/ deletes the files
// of photos it is not told about.
func (c catalog) Photos(ctx context.Context) ([]images.Photo, error) {
	ps, err := c.s.Photos(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]images.Photo, len(ps))
	for i, p := range ps {
		out[i] = p.Image()
	}

	return out, nil
}
