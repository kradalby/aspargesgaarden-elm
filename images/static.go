package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestFile is the name BuildStatic gives the manifest inside its output.
const ManifestFile = "manifest.json"

// BuildStatic runs the pipeline at build time over the masters in src: the
// photos embedded in the binary. Each master becomes the Full variants at
// out/<VariantSpec.Name>, and out/manifest.json maps the master's name (its
// file name without extension) to its Ref, with URLs below urlPrefix.
func BuildStatic(ctx context.Context, src, out, urlPrefix string) (map[string]Ref, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("masters: %w", err)
	}

	// Work files beside the output, so renames stay on one filesystem.
	tmp := filepath.Join(out, ".tmp")
	if err := os.MkdirAll(tmp, dirPerm); err != nil {
		return nil, fmt.Errorf("output: %w", err)
	}
	defer os.RemoveAll(tmp)

	v, err := newVips(tmp)
	if err != nil {
		return nil, err
	}

	prefix := strings.TrimSuffix(urlPrefix, "/") + "/"
	refs := map[string]Ref{}

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if _, dup := refs[name]; dup {
			return nil, fmt.Errorf("two masters are named %q", name)
		}

		ref, err := v.buildMaster(ctx, filepath.Join(src, e.Name()), out, prefix)
		if err != nil {
			return nil, fmt.Errorf("master %s: %w", e.Name(), err)
		}

		refs[name] = ref
	}

	b, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(out, ManifestFile), append(b, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	return refs, nil
}

func (v *vips) buildMaster(ctx context.Context, path, out, prefix string) (Ref, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Ref{}, fmt.Errorf("read: %w", err)
	}

	sum := sha256.Sum256(b)
	p := Photo{SHA256: hex.EncodeToString(sum[:]), MIME: sniff(b[:min(len(b), 32)])}

	hdr, err := v.preflight(ctx, path, p.MIME)
	if err != nil {
		return Ref{}, err
	}

	p.Width, p.Height = hdr.width, hdr.height

	gs := groups(p)
	if len(gs) == 0 {
		return Ref{}, fmt.Errorf("%w: %dx%d", ErrTooSmall, p.Width, p.Height)
	}

	err = v.build(ctx, p, path, gs[0], gs[0].specs, func(s VariantSpec) string {
		return filepath.Join(out, filepath.FromSlash(s.Name()))
	})
	if err != nil {
		return Ref{}, err
	}

	return refFor(p, Full, func(s VariantSpec) string { return prefix + s.Name() })
}

// ParseManifest reads the manifest BuildStatic wrote.
func ParseManifest(b []byte) (map[string]Ref, error) {
	var refs map[string]Ref
	if err := json.Unmarshal(b, &refs); err != nil {
		return nil, fmt.Errorf("photo manifest: %w", err)
	}

	return refs, nil
}
