package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db/dbsqlc"
)

func openTest(t *testing.T) *DB {
	t.Helper()

	d, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	return d
}

func draft(year int64, slug string) dbsqlc.CreateEventParams {
	return dbsqlc.CreateEventParams{
		Year:      sql.NullInt64{Int64: year, Valid: year != 0},
		Slug:      slug,
		Title:     "Julemarked",
		Status:    "draft",
		CreatedAt: 1,
		UpdatedAt: 1,
	}
}

func photo(sha byte) dbsqlc.CreatePhotoParams {
	return dbsqlc.CreatePhotoParams{
		Sha256: strings.Repeat(string("0123456789abcdef"[sha%16]), 64),
		Mime:   "image/jpeg",
		Width:  4000,
		Height: 3000,
	}
}

func TestOpenTwice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	d, err := Open(t.Context(), path)
	require.NoError(t, err)
	require.NoError(t, d.Close())

	d, err = Open(t.Context(), path)
	require.NoError(t, err, "reopening an up-to-date database must not migrate")
	require.NoError(t, d.Close())
}

// A database made before the settings table gains it on open.
func TestOpenAddsSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	d, err := Open(t.Context(), path)
	require.NoError(t, err)

	// Dropping the table gives the earlier schema back, digest and all.
	_, err = d.sql.ExecContext(t.Context(), "DROP TABLE settings")
	require.NoError(t, err)
	require.NoError(t, d.Close())

	d, err = Open(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.Queries().InsertSettingIfMissing(t.Context(), dbsqlc.InsertSettingIfMissingParams{Name: "k", Value: "v"}))
}

func TestSettingKeepsFirstValue(t *testing.T) {
	q := openTest(t).Queries()

	require.NoError(t, q.InsertSettingIfMissing(t.Context(), dbsqlc.InsertSettingIfMissingParams{Name: "k", Value: "first"}))
	require.NoError(t, q.InsertSettingIfMissing(t.Context(), dbsqlc.InsertSettingIfMissingParams{Name: "k", Value: "second"}))

	v, err := q.GetSetting(t.Context(), "k")
	require.NoError(t, err)
	require.Equal(t, "first", v)
}

// The litestream tables appear in a replicated database at runtime; squibble
// must not treat them as drift.
func TestOpenIgnoresLitestreamTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	d, err := Open(t.Context(), path)
	require.NoError(t, err)
	// The schema catalog is outside sqlc; raw SQL in a test is the exception.
	_, err = d.sql.ExecContext(t.Context(), "CREATE TABLE _litestream_seq (id INTEGER PRIMARY KEY, seq INTEGER)")
	require.NoError(t, err)
	require.NoError(t, d.Close())

	d, err = Open(t.Context(), path)
	require.NoError(t, err)
	require.NoError(t, d.Close())
}

func TestSlugUniquePerYear(t *testing.T) {
	d := openTest(t)
	q := d.Queries()
	ctx := t.Context()

	_, err := q.CreateEvent(ctx, draft(2026, "julemarked"))
	require.NoError(t, err)

	_, err = q.CreateEvent(ctx, draft(2027, "julemarked"))
	require.NoError(t, err, "same slug in another year")

	_, err = q.CreateEvent(ctx, draft(0, "julemarked"))
	require.NoError(t, err, "undated draft")
	_, err = q.CreateEvent(ctx, draft(0, "julemarked"))
	require.NoError(t, err, "two undated drafts")

	_, err = q.CreateEvent(ctx, draft(2026, ""))
	require.NoError(t, err)
	_, err = q.CreateEvent(ctx, draft(2026, ""))
	require.NoError(t, err, "two untitled drafts")

	_, err = q.CreateEvent(ctx, draft(2026, "julemarked"))
	require.True(t, IsUniqueViolation(err), "got %v", err)
}

func TestEventChecks(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*dbsqlc.CreateEventParams)
	}{
		{"uppercase slug", func(p *dbsqlc.CreateEventParams) { p.Slug = "Julemarked" }},
		{"non-ascii slug", func(p *dbsqlc.CreateEventParams) { p.Slug = "vårkonsert" }},
		{"bad date", func(p *dbsqlc.CreateEventParams) { p.StartDate = "2026-02-30" }},
		{"loose date", func(p *dbsqlc.CreateEventParams) { p.StartDate = "2026-2-3" }},
		{"loose time", func(p *dbsqlc.CreateEventParams) { p.StartTime = "ca. 15:00" }},
		{"hour 24", func(p *dbsqlc.CreateEventParams) { p.StartTime = "24:00" }},
		{"end before start", func(p *dbsqlc.CreateEventParams) {
			p.StartDate, p.EndDate = "2026-11-14", "2026-11-13"
		}},
		{"unknown status", func(p *dbsqlc.CreateEventParams) { p.Status = "hidden" }},
		{"published without date", func(p *dbsqlc.CreateEventParams) {
			p.Status = "published"
			p.PublishedAt = sql.NullInt64{Int64: 1, Valid: true}
		}},
		{"published without published_at", func(p *dbsqlc.CreateEventParams) {
			p.Status = "published"
			p.StartDate = "2026-11-14"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := openTest(t)
			p := draft(2026, "julemarked")
			tt.mod(&p)
			_, err := d.Queries().CreateEvent(t.Context(), p)
			require.ErrorContains(t, err, "CHECK constraint failed")
		})
	}
}

func TestPhotoInUseCannotBeDeleted(t *testing.T) {
	d := openTest(t)
	q := d.Queries()
	ctx := t.Context()

	eventID, err := q.CreateEvent(ctx, draft(2026, "julemarked"))
	require.NoError(t, err)
	used, err := q.CreatePhoto(ctx, photo(1))
	require.NoError(t, err)
	inGallery, err := q.CreatePhoto(ctx, photo(2))
	require.NoError(t, err)
	free, err := q.CreatePhoto(ctx, photo(3))
	require.NoError(t, err)

	require.NoError(t, q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{EventID: eventID, PhotoID: used, Role: "main"}))
	require.NoError(t, q.InsertGalleryPhoto(ctx, dbsqlc.InsertGalleryPhotoParams{PhotoID: inGallery}))

	_, err = q.DeletePhoto(ctx, used)
	require.True(t, IsForeignKeyViolation(err), "event photo: got %v", err)
	_, err = q.DeletePhoto(ctx, inGallery)
	require.True(t, IsForeignKeyViolation(err), "gallery photo: got %v", err)

	n, err := q.DeletePhoto(ctx, free)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	// Deleting the event releases its photo.
	_, err = q.DeleteEvent(ctx, eventID)
	require.NoError(t, err)
	_, err = q.DeletePhoto(ctx, used)
	require.NoError(t, err)
}

func TestEventPhotoInvariants(t *testing.T) {
	d := openTest(t)
	q := d.Queries()
	ctx := t.Context()

	eventID, err := q.CreateEvent(ctx, draft(2026, "julemarked"))
	require.NoError(t, err)
	a, err := q.CreatePhoto(ctx, photo(1))
	require.NoError(t, err)
	b, err := q.CreatePhoto(ctx, photo(2))
	require.NoError(t, err)

	require.NoError(t, q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{EventID: eventID, PhotoID: a, Role: "main"}))

	err = q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{EventID: eventID, PhotoID: b, Role: "main", Position: 1})
	require.True(t, IsUniqueViolation(err), "second main photo: got %v", err)

	err = q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{EventID: eventID, PhotoID: b, Role: "extra", Position: 0})
	require.True(t, IsUniqueViolation(err), "duplicate position: got %v", err)

	err = q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{EventID: eventID, PhotoID: 999, Role: "extra", Position: 1})
	require.True(t, IsForeignKeyViolation(err), "missing photo: got %v", err)
}

func TestCreatePhotoDedupes(t *testing.T) {
	d := openTest(t)
	q := d.Queries()

	first, err := q.CreatePhoto(t.Context(), photo(1))
	require.NoError(t, err)

	again := photo(1)
	again.Alt = "ignored"
	second, err := q.CreatePhoto(t.Context(), again)
	require.NoError(t, err)
	require.Equal(t, first, second)

	p, err := q.GetPhoto(t.Context(), first)
	require.NoError(t, err)
	require.Empty(t, p.Alt, "a repeated upload must not overwrite the stored photo")
}

func TestCropKinds(t *testing.T) {
	d := openTest(t)
	q := d.Queries()

	id, err := q.CreatePhoto(t.Context(), photo(1))
	require.NoError(t, err)

	for _, kind := range []string{"card", "banner", "share"} {
		require.NoError(t, q.UpsertPhotoCrop(t.Context(), dbsqlc.UpsertPhotoCropParams{PhotoID: id, Kind: kind, W: 10, H: 10}))
	}

	err = q.UpsertPhotoCrop(t.Context(), dbsqlc.UpsertPhotoCropParams{PhotoID: id, Kind: "full", W: 10, H: 10})
	require.ErrorContains(t, err, "CHECK constraint failed")

	require.NoError(t, q.UpsertPhotoCrop(t.Context(), dbsqlc.UpsertPhotoCropParams{PhotoID: id, Kind: "card", X: 5, W: 20, H: 25}))
	crops, err := q.ListPhotoCrops(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, crops, 3)
	require.Equal(t, dbsqlc.PhotoCrop{PhotoID: id, Kind: "card", X: 5, W: 20, H: 25}, crops[1])
}

func TestPhotographers(t *testing.T) {
	d := openTest(t)
	q := d.Queries()
	ctx := t.Context()

	anita := dbsqlc.CreatePhotographerParams{Name: "Anita Saga", NameKey: "anita saga"}
	id, err := q.CreatePhotographer(ctx, anita)
	require.NoError(t, err)

	_, err = q.CreatePhotographer(ctx, dbsqlc.CreatePhotographerParams{Name: "ANITA SAGA", NameKey: "anita saga"})
	require.True(t, IsUniqueViolation(err), "same key: got %v", err)

	again, err := q.EnsurePhotographer(ctx, dbsqlc.EnsurePhotographerParams{
		Name: "Anita Saga", NameKey: "anita saga", Url: "https://ignored.example",
	})
	require.NoError(t, err)
	require.Equal(t, id, again)
	got, err := q.GetPhotographer(ctx, id)
	require.NoError(t, err)
	require.Empty(t, got.Url, "ensuring a known name must not overwrite it")

	for _, p := range []dbsqlc.CreatePhotographerParams{
		{Name: "", NameKey: "x"},
		{Name: " Lucky", NameKey: "lucky"},
		{Name: "Lucky", NameKey: ""},
		{Name: "Lucky", NameKey: "lucky", Url: "javascript:alert(1)"},
		{Name: "Lucky", NameKey: "lucky", Url: "https://"},
	} {
		_, err := q.CreatePhotographer(ctx, p)
		require.ErrorContains(t, err, "CHECK constraint failed", "%+v", p)
	}

	_, err = q.CreatePhotographer(ctx, dbsqlc.CreatePhotographerParams{Name: "Lucky", NameKey: "lucky", Url: "HTTPS://lucky.example"})
	require.NoError(t, err, "scheme case does not matter")

	p := photo(1)
	p.PhotographerID = sql.NullInt64{Int64: id, Valid: true}
	photoID, err := q.CreatePhoto(ctx, p)
	require.NoError(t, err)

	_, err = q.DeletePhotographer(ctx, id)
	require.True(t, IsForeignKeyViolation(err), "credited photographer: got %v", err)

	p = photo(2)
	p.PhotographerID = sql.NullInt64{Int64: 999, Valid: true}
	_, err = q.CreatePhoto(ctx, p)
	require.True(t, IsForeignKeyViolation(err), "missing photographer: got %v", err)

	_, err = q.DeletePhoto(ctx, photoID)
	require.NoError(t, err)
	n, err := q.DeletePhotographer(ctx, id)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}

func TestInTxRollsBack(t *testing.T) {
	d := openTest(t)
	boom := errors.New("boom")

	err := d.InTx(t.Context(), func(q *dbsqlc.Queries) error {
		_, err := q.CreateEvent(t.Context(), draft(2026, "julemarked"))
		require.NoError(t, err)
		return boom
	})
	require.ErrorIs(t, err, boom)

	events, err := d.Queries().ListEvents(t.Context())
	require.NoError(t, err)
	require.Empty(t, events)
}
