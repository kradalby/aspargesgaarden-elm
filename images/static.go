package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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
//
// Names change with the master's bytes and the pipeline version, so a file
// already in out is current and is not built again; anything else in out is
// deleted, so out holds exactly what the manifest names.
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
	keep := map[string]bool{ManifestFile: true}

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if _, dup := refs[name]; dup {
			return nil, fmt.Errorf("two masters are named %q", name)
		}

		ref, files, err := v.buildMaster(ctx, filepath.Join(src, e.Name()), out, prefix)
		if err != nil {
			return nil, fmt.Errorf("master %s: %w", e.Name(), err)
		}

		refs[name] = ref
		for _, f := range files {
			keep[f] = true
		}
	}

	b, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(out, ManifestFile), append(b, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	if err := prune(out, keep); err != nil {
		return nil, err
	}

	return refs, nil
}

// buildMaster builds the missing variants of one master and returns its Ref
// and the variant files' names below out.
func (v *vips) buildMaster(ctx context.Context, path, out, prefix string) (Ref, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Ref{}, nil, fmt.Errorf("read: %w", err)
	}

	sum := sha256.Sum256(b)
	p := Photo{SHA256: hex.EncodeToString(sum[:]), MIME: sniff(b[:min(len(b), 32)])}

	hdr, err := v.preflight(ctx, path, p.MIME)
	if err != nil {
		return Ref{}, nil, err
	}

	p.Width, p.Height = hdr.width, hdr.height

	gs := groups(p)
	if len(gs) == 0 {
		return Ref{}, nil, fmt.Errorf("%w: %dx%d", ErrTooSmall, p.Width, p.Height)
	}

	dst := func(s VariantSpec) string { return filepath.Join(out, filepath.FromSlash(s.Name())) }

	var (
		files   []string
		missing []VariantSpec
	)

	for _, s := range gs[0].specs {
		files = append(files, s.Name())
		if !exists(dst(s)) {
			missing = append(missing, s)
		}
	}

	if len(missing) > 0 {
		if err := v.build(ctx, p, path, gs[0], missing, dst, nil); err != nil {
			return Ref{}, nil, err
		}
	}

	ref, err := refFor(p, Full, func(s VariantSpec) string { return prefix + s.Name() })

	return ref, files, err
}

// prune deletes files below out that keep (slash-separated names) does not
// list, then the folders that leaves empty.
func prune(out string, keep map[string]bool) error {
	root, err := os.OpenRoot(out)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	defer root.Close()

	var dirs []string

	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case name == ".tmp": // the running build's work files
			return fs.SkipDir
		case d.IsDir():
			dirs = append(dirs, name)
			return nil
		case keep[name]:
			return nil
		}

		return root.Remove(name)
	})
	if err != nil {
		return fmt.Errorf("prune %s: %w", out, err)
	}

	for i := len(dirs) - 1; i > 0; i-- {
		_ = root.Remove(dirs[i]) // only succeeds when empty
	}

	return nil
}

// ParseManifest reads the manifest BuildStatic wrote.
func ParseManifest(b []byte) (map[string]Ref, error) {
	var refs map[string]Ref
	if err := json.Unmarshal(b, &refs); err != nil {
		return nil, fmt.Errorf("photo manifest: %w", err)
	}

	return refs, nil
}
