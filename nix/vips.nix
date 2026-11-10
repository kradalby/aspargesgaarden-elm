# libvips with only what the images package needs. Every loader dropped here
# is one a hostile upload can no longer reach, and the closure halves.
{ lib, vips }:
(vips.override {
  # Loaders the pipeline never calls.
  imagemagick = null;
  poppler = null;
  openslide = null;
  librsvg = null;
  cfitsio = null;
  matio = null;
  openexr = null;
  openjpeg = null;
  libraw = null;
  libjxl = null;
  # Text rendering, dzsave archives, FFT ops: unused.
  pango = null;
  libarchive = null;
  fftw = null;
  withIntrospection = false;
  withDevDoc = false;
}).overrideAttrs
  (o: {
    # meson would otherwise require or auto-detect these.
    mesonFlags =
      o.mesonFlags
      ++ map (f: lib.mesonEnable f false) [
        "archive"
        "poppler"
        "openslide"
        "rsvg"
        "cfitsio"
        "matio"
        "openexr"
        "openjpeg"
        "raw"
        "jpeg-xl"
        "pangocairo"
        "fontconfig"
        "fftw"
      ];
  })
