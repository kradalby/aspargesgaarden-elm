package main

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"go.yaml.in/yaml/v3"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

var errImportArgs = errors.New("import takes one argument: the old site's checkout")

func newImportCmd(a *app, parent *ff.FlagSet) *ff.Command {
	dataDir := ""
	fs := ff.NewFlagSet("import").SetParent(parent)
	fs.StringVar(&dataDir, 0, "data-dir", "/var/lib/asparges", "state directory to import into")

	return &ff.Command{
		Name:      "import",
		Usage:     "asparges import [FLAGS] <old site checkout>",
		ShortHelp: "copy the old site's events and gallery into the database, once",
		LongHelp: "Reads content/events/*.md, content/gallery.yaml and their photos from a checkout of " +
			"the old elm-pages site. Events already there (same year and address) and a gallery " +
			"that is not empty are left alone, so running it again changes nothing. Run it with " +
			"serve stopped: a running serve keeps its rendered pages until the next change.",
		Flags: fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return errImportArgs
			}

			d, err := openData(ctx, dataDir)
			if err != nil {
				return err
			}
			defer d.Close()

			store := site.NewStore(d, time.Now)

			imgs, err := images.New(dataDir, store.Catalog(), images.WithLogger(a.log))
			if err != nil {
				return fmt.Errorf("photo store: %w", err)
			}

			return importer{store: store, images: imgs, root: args[0], log: a.log}.run(ctx)
		},
	}
}

type importer struct {
	store  *site.Store
	images *images.Store
	root   string
	log    *slog.Logger
}

// oldPhoto is a photo reference in the old content files.
type oldPhoto struct {
	Path         string          `yaml:"path"`
	Photographer oldPhotographer `yaml:"photographer"`
	Description  string          `yaml:"description"`
}

type oldPhotographer struct {
	Name    string `yaml:"name"`
	Website string `yaml:"website"`
}

// oldEvent is the frontmatter of content/events/*.md (src/Page/Events.elm).
type oldEvent struct {
	Title         string   `yaml:"title"`
	EventDate     string   `yaml:"eventDate"`
	EventTime     string   `yaml:"eventTime"`
	EventEndDate  string   `yaml:"eventEndDate"`
	EventEndTime  string   `yaml:"eventEndTime"`
	PublishedFrom string   `yaml:"publishedFrom"`
	EventDates    []string `yaml:"eventDates"`
	Photo         oldPhoto `yaml:"photo"`
}

// oldGallery is content/gallery.yaml: photographers by key, then the photos
// in their order on the page.
type oldGallery struct {
	Photographers map[string]oldPhotographer `yaml:"photographers"`
	Photos        []struct {
		Path         string `yaml:"path"`
		Photographer string `yaml:"photographer"`
		Description  string `yaml:"description"`
	} `yaml:"photos"`
}

func (im importer) run(ctx context.Context) error {
	files, err := filepath.Glob(filepath.Join(im.root, "content", "events", "*.md"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no content/events/*.md under %s: %w", im.root, cmp.Or(err, os.ErrNotExist))
	}

	existing, err := im.store.Events(ctx)
	if err != nil {
		return err
	}

	have := map[string]bool{}
	for _, e := range existing {
		have[e.Path()] = true
	}

	for _, f := range files {
		if err := im.event(ctx, f, have); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}

	return im.gallery(ctx)
}

func (im importer) event(ctx context.Context, file string, have map[string]bool) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading: %w", err)
	}

	old, body, err := parseOldEvent(src)
	if err != nil {
		return err
	}

	e, publish, err := old.toEvent(body)
	if err != nil {
		return err
	}

	if have[e.Path()] {
		im.log.InfoContext(ctx, "event already there, kept as is", "path", e.Path())
		return nil
	}

	extras := []oldPhoto(nil)
	if page, ok := oldChristmasPages[e.Path()]; ok {
		e.BodyMD, extras = page.body, page.photos
	}

	// Publishing needs alt text, and the old event photos had none.
	main, err := im.photo(ctx, old.Photo, cmp.Or(strings.TrimSpace(old.Photo.Description), e.Title))

	switch {
	case errors.Is(err, images.ErrTooSmall):
		// An event is published only with a main photo; this one waits as a
		// draft until someone finds a larger one.
		im.log.WarnContext(ctx, "main photo too small to use, event kept as a draft", "path", e.Path(), "err", err)
		publish = false
	case err != nil:
		return err
	default:
		e.MainPhoto = &main
	}

	for _, op := range extras {
		p, err := im.photo(ctx, op, op.Description)
		if errors.Is(err, images.ErrTooSmall) {
			im.log.WarnContext(ctx, "photo too small to use, left out", "path", e.Path(), "err", err)
			continue
		}

		if err != nil {
			return err
		}

		e.ExtraPhotos = append(e.ExtraPhotos, p)
	}

	e, err = im.store.CreateEvent(ctx, e)
	if err != nil {
		return err
	}

	if publish {
		if e, err = im.store.Publish(ctx, e.ID); err != nil {
			return err
		}
	}

	have[e.Path()] = true
	im.log.InfoContext(ctx, "event imported", "path", e.Path(), "status", e.Status, "title", e.Title)

	return nil
}

// parseOldEvent splits a content file into its frontmatter and Markdown.
// The frontmatter is JSON with trailing commas, which YAML's flow style
// reads as the old site did.
func parseOldEvent(src []byte) (oldEvent, string, error) {
	rest, ok := bytes.CutPrefix(src, []byte("---\n"))
	front, body, found := bytes.Cut(rest, []byte("\n---\n"))

	if !ok || !found {
		return oldEvent{}, "", errNoFrontmatter
	}

	var old oldEvent
	if err := yaml.Unmarshal(front, &old); err != nil {
		return oldEvent{}, "", fmt.Errorf("frontmatter: %w", err)
	}

	return old, strings.TrimSpace(string(body)), nil
}

var errNoFrontmatter = errors.New("no --- frontmatter ---")

// toEvent maps the old fields. The file name is not used: two files carry a
// wrong date in theirs, and eventDate is what the old page showed.
func (o oldEvent) toEvent(body string) (site.Event, bool, error) {
	start, err := site.ParseDate(o.EventDate)
	if err != nil {
		return site.Event{}, false, err
	}

	e := site.Event{
		Title: o.Title,
		// The venue is every event's; it need not be in the address.
		Slug:  site.Slugify(strings.TrimSuffix(o.Title, " på Aspargesgården")),
		Start: start,
		Year:  start.Year,
	}

	var notes []string

	e.StartTime, notes = legacyTime(o.EventTime, "Start", notes)
	e.EndTime, notes = legacyTime(o.EventEndTime, "Slutt", notes)

	if o.EventEndDate != "" && o.EventEndDate != o.EventDate {
		if e.End, err = site.ParseDate(o.EventEndDate); err != nil {
			return site.Event{}, false, err
		}
	}

	// eventDates listed every day, the first one included.
	for _, s := range o.EventDates {
		d, err := site.ParseDate(s)
		if err != nil {
			return site.Event{}, false, err
		}

		if d != start && !slices.Contains(e.ExtraDates, d) {
			e.ExtraDates = append(e.ExtraDates, d)
		}
	}

	// The old menu linked the Christmas market; it stays there while the
	// market is coming.
	if e.Slug == "julemarked" {
		e.ShowInMenu, e.MenuLabel = true, "Julemarked"
	}

	e.BodyMD = body
	if len(notes) > 0 {
		e.BodyMD = "*" + strings.Join(notes, " ") + "*\n\n" + body
	}

	return e, o.PublishedFrom != "", nil
}

// legacyTime makes a strict time of an old free-text one. What the strict
// form loses ("ca.", or wording that is no time at all) goes into a note
// for the event text.
func legacyTime(s, what string, notes []string) (string, []string) {
	if strings.TrimSpace(s) == "" {
		return "", notes
	}

	clock, approx, err := site.ParseLegacyTime(s)

	switch {
	case err != nil:
		return "", append(notes, what+": "+strings.TrimSpace(s)+".")
	case approx:
		return clock, append(notes, what+" ca. kl. "+clock+".")
	}

	return clock, notes
}

// photo stores the old public/<path>.jpeg and records it, crediting its
// photographer, with centred crops. A photo already there keeps its alt
// text, credit and crops, which the admin may have improved since.
func (im importer) photo(ctx context.Context, op oldPhoto, alt string) (site.Photo, error) {
	f, err := os.Open(filepath.Join(im.root, "public", filepath.FromSlash(op.Path)+".jpeg"))
	if err != nil {
		return site.Photo{}, fmt.Errorf("photo: %w", err)
	}
	defer f.Close()

	o, err := im.images.Intake(ctx, f)
	if err != nil {
		return site.Photo{}, fmt.Errorf("photo %s: %w", op.Path, err)
	}

	p, err := im.addPhoto(ctx, op, o, alt)
	if err != nil {
		return site.Photo{}, err
	}

	for _, k := range images.CropKinds {
		if _, set := p.Crops[site.CropKind(k)]; set {
			continue
		}

		r, err := images.DefaultCrop(k, p.Width, p.Height)
		if errors.Is(err, images.ErrTooSmall) {
			// The pages show such a photo whole; only the share image is
			// missing, as for any small upload.
			im.log.WarnContext(ctx, "photo too small for a crop, left without", "photo", op.Path, "kind", k, "err", err)
			continue
		}

		if err == nil {
			err = im.store.SetCrop(ctx, p.ID, site.CropKind(k), site.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
		}

		if err != nil {
			return site.Photo{}, fmt.Errorf("photo %s: %s crop: %w", op.Path, k, err)
		}
	}

	return p, nil
}

// addPhoto records an original with its credit; see photo.
func (im importer) addPhoto(ctx context.Context, op oldPhoto, o images.Original, alt string) (site.Photo, error) {
	p := site.Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height, Alt: strings.TrimSpace(alt)}

	if strings.TrimSpace(op.Photographer.Name) != "" {
		ph, err := im.store.FindOrCreatePhotographer(ctx, site.Photographer{Name: op.Photographer.Name, URL: op.Photographer.Website})
		if err != nil {
			return site.Photo{}, fmt.Errorf("photo %s: %w", op.Path, err)
		}

		p.Photographer = &ph
	}

	return im.store.AddPhoto(ctx, p)
}

func (im importer) gallery(ctx context.Context) error {
	current, err := im.store.Gallery(ctx)
	if err != nil {
		return err
	}

	if len(current) > 0 {
		im.log.InfoContext(ctx, "gallery already there, kept as is", "photos", len(current))
		return nil
	}

	src, err := os.ReadFile(filepath.Join(im.root, "content", "gallery.yaml"))
	if err != nil {
		return fmt.Errorf("gallery: %w", err)
	}

	var g oldGallery
	if err := yaml.Unmarshal(src, &g); err != nil {
		return fmt.Errorf("gallery.yaml: %w", err)
	}

	ids := make([]int64, 0, len(g.Photos))
	for i, gp := range g.Photos {
		by, ok := g.Photographers[gp.Photographer]
		if !ok && gp.Photographer != "" {
			return fmt.Errorf("gallery photo %d (%s): %w: %q", i, gp.Path, errUnknownPhotographer, gp.Photographer)
		}

		p, err := im.photo(ctx, oldPhoto{Path: gp.Path, Photographer: by}, gp.Description)
		if errors.Is(err, images.ErrTooSmall) {
			im.log.WarnContext(ctx, "gallery photo too small to use, left out", "photo", gp.Path, "err", err)
			continue
		}

		if err != nil {
			return err
		}

		ids = append(ids, p.ID)
	}

	if err := im.store.SetGallery(ctx, ids); err != nil {
		return err
	}

	im.log.InfoContext(ctx, "gallery imported", "photos", len(ids))

	return nil
}

var errUnknownPhotographer = errors.New("photographer not in the photographers list")

// oldChristmasPage is a hand-made Christmas market page of the old site
// (src/Page/Christmas/Year_.elm), whose address now leads to that year's
// market event. Its text is fuller than the event file's, which only
// linked to it, so it becomes the event's text, with its photos.
type oldChristmasPage struct {
	body   string
	photos []oldPhoto
}

var oldChristmasPages = map[string]oldChristmasPage{
	"/arrangementer/2026/julemarked": {
		body: `# Velkommen til Julemarked på Aspargesgården!

Bli med oss på en herlig helg den 14. og 15. november, fra kl. 12 til 16, når vi forvandler Aspargesgården og skaper herlig jul. Vi gleder oss til å presentere et marked fylt med kreativitet og julestemning!

Her vil du finne mer enn 30 dyktige gründere som tilbyr en rekke flotte brukskunst og kortreiste delikatesser. Her finner du den ekstra fine gaven, hjemmelagde spiselige gaver til deg selv eller noen du er glad i.

Ta deg tid til å nyte den nydelige kafeen vår, hvor hjemmebakst garantert vil smake.

## Når og hvor

**Dato:** 14.–15. november 2026

**Åpningstider:** Lørdag og søndag kl. 12:00–16:00

**Sted:** Aspargesgården, Østbyveien 75, 3280 Tjodalyng (Larvik)

## Høydepunkter

- Mer enn 30 dyktige gründere med brukskunst og kortreiste delikatesser
- Nydelig kafe med hjemmebakst
- Julepyntet gård med koselig bålfat ute
- Den ekstra fine gaven eller hjemmelagde spiselige gaver

Gården vil være julepyntet, og vi har bålfat ute for å skape en koselig atmosfære.

**På søndagen, ca. kl. 13, vil Tjølling Kirkes barnekor BiTs synge julesanger for oss.**

Vi er utrolig takknemlige for at Aspargesgården kan være en vakker ramme for slike arrangementer og opplevelser.

> **Velkommen!**
>
> Ta med familie og venner, og bli med oss for å komme i førjulsstemning! Vi lover en helg fylt med glede, dufter og stemning. Velkommen!
`,
		// The old page also showed julemarked1, which is the event's
		// banner here; one event cannot show a photo twice.
		photos: []oldPhoto{
			{Path: "arrangementer/2026-11-14-julemarked", Description: "Julemarked på Aspargesgården lørdag og søndag 14.–15. november kl. 12–16. Mer enn 30 utstillere, kafeteria og tombola."},
			{Path: "arrangementer/julemarked3", Description: "Fra et tidligere julemarked på Aspargesgården"},
			{Path: "arrangementer/julemarked2", Description: "Et tilbakeblikk på julemarkedet"},
			{Path: "arrangementer/julemarked4", Description: "Julemarked på gården"},
		},
	},
}
