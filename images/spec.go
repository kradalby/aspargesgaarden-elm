// Package images owns every image file of asparges: the uploaded originals,
// the web variants derived from them with the vips CLI, their names and URLs,
// and the cleanup that keeps the folders matching the database. Nothing else
// reads or writes the image folders.
package images

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
)

// PipelineVersion names the variants folder. Bump it whenever anything that
// changes output files changes: vips flags, the encoder settings below, the
// width ladder or the kinds. Reconcile then deletes the old folder and the
// queue rebuilds every photo under the new one.
const PipelineVersion = 1

// Encoder settings, chosen once for every variant. keep=none strips EXIF
// (GPS), XMP, IPTC and ICC; outputs are sRGB, which browsers assume.
const (
	avifOptions = "[Q=50,effort=4,keep=none]"
	webpOptions = "[Q=80,effort=4,keep=none]"
	jpegOptions = "[Q=82,optimize-coding=true,interlace=true,background=255,keep=none]"
)

// widths is the ladder every kind but Share is cut into. A variant is never
// wider than its crop, so small photos get fewer steps rather than upscaling.
var widths = []int{480, 800, 1200, 1600, 2000}

// formats is also the order a <picture> lists its sources in: best first.
var formats = []Format{AVIF, WebP, JPEG}

// Share images are what Facebook and friends recommend; they get exactly one
// JPEG because some scrapers cannot read anything else.
const (
	shareWidth  = 1200
	shareHeight = 630
)

// ErrNotFound means the variant, photo or crop does not exist or is not
// current. The HTTP handler turns it into a 404.
var ErrNotFound = errors.New("not found")

// Kind is the use a variant is cut for.
type Kind string

const (
	Card   Kind = "card"   // 4:5, event list
	Banner Kind = "banner" // 16:9, top of an event page
	Share  Kind = "share"  // 1200×630, Open Graph
	Full   Kind = "full"   // uncropped, gallery and lightbox
)

var kinds = []Kind{Card, Banner, Share, Full}

// CropKinds are the kinds the admin chooses a crop for. Full always uses the
// whole photo.
var CropKinds = []Kind{Card, Banner, Share}

// aspect returns the kind's width:height ratio, or 0, 0 for Full.
func (k Kind) aspect() (int, int) {
	switch k {
	case Card:
		return 4, 5
	case Banner:
		return 16, 9
	case Share:
		return shareWidth, shareHeight
	case Full:
		return 0, 0
	default:
		return 0, 0
	}
}

// Rect is a crop in upright source pixels: the frame after EXIF orientation,
// the one Intake reports and the admin preview (our own Full variant) shows.
type Rect struct {
	X, Y, W, H int
}

// ID names the crop in file names. A new crop gives new names, so a stale
// picture can never be served under a current URL.
func (r Rect) ID() string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d,%d,%d,%d", r.X, r.Y, r.W, r.H))
	return hex.EncodeToString(sum[:4])
}

// VariantSpec names one variant file. Path, URL and ParseURL are the only
// way names are made or read.
type VariantSpec struct {
	Photo  string // hex sha256 of the original
	Kind   Kind
	Crop   string // Rect.ID of the crop it is cut from
	Width  int
	Format Format
}

func (f Format) ext() string {
	if f == JPEG {
		return "jpg"
	}

	return string(f)
}

func (s VariantSpec) file() string {
	return fmt.Sprintf("%s-%s-%d.%s", s.Kind, s.Crop, s.Width, s.Format.ext())
}

// Name is the variant's path below a URL prefix: v<pipeline>/<sha>/<file>.
func (s VariantSpec) Name() string {
	return fmt.Sprintf("v%d/%s/%s", PipelineVersion, s.Photo, s.file())
}

// URL is the public address of the variant.
func (s VariantSpec) URL() string { return "/img/" + s.Name() }

// Path is the variant's file below the store root. The sha prefix folder
// keeps directories small.
func (s VariantSpec) Path() string {
	return filepath.Join("variants", fmt.Sprintf("v%d", PipelineVersion), s.Photo[:2], s.Photo, s.file())
}

var urlRE = regexp.MustCompile(`^/img/v([0-9]+)/([0-9a-f]{64})/(card|banner|share|full)-([0-9a-f]{8})-([0-9]+)\.(avif|webp|jpg)$`)

// ParseURL reads a URL made by VariantSpec.URL. Anything else, including an
// older pipeline version, is ErrNotFound.
func ParseURL(u string) (VariantSpec, error) {
	m := urlRE.FindStringSubmatch(u)
	if m == nil || m[1] != strconv.Itoa(PipelineVersion) {
		return VariantSpec{}, ErrNotFound
	}

	w, err := strconv.Atoi(m[5])
	if err != nil {
		return VariantSpec{}, ErrNotFound
	}

	f := Format(m[6])
	if m[6] == "jpg" {
		f = JPEG
	}

	s := VariantSpec{Photo: m[2], Kind: Kind(m[3]), Crop: m[4], Width: w, Format: f}
	// One spelling per spec: rejects leading zeros and the like.
	if s.URL() != u {
		return VariantSpec{}, ErrNotFound
	}

	return s, nil
}
