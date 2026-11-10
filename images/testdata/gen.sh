#!/usr/bin/env bash
# Regenerates every fixture in this directory from nothing (synthetic, licence-clean).
# Needs: vips (nixpkgs', with uhdrsave), magick (ImageMagick 7), heif-enc
# (libheif, x265), exiftool.
# Usage: ./gen.sh            -> stored fixtures
#        ./gen.sh overcap D  -> writes the big generated-only inputs into D
set -euo pipefail
cd "$(dirname "$0")"

# Upright scene, 1200x800: grey gradient, 4 corner markers 160x160 and one
# off-centre marker, so every rotation/flip and every crop offset is detectable.
#   TL red (255,0,0)   TR green (0,255,0)
#   BL blue (0,0,255)  BR yellow (255,255,0)
#   magenta (255,0,255) at x=800..959, y=240..399 (upright coords)
scene() { # $1 = out (any magick format), extra args passed through
  local out=$1; shift
  magick -size 800x1200 gradient:'#909090-#505050' -rotate 90 \
    -fill '#ff0000' -draw 'rectangle 0,0 159,159' \
    -fill '#00ff00' -draw 'rectangle 1040,0 1199,159' \
    -fill '#0000ff' -draw 'rectangle 0,640 159,799' \
    -fill '#ffff00' -draw 'rectangle 1040,640 1199,799' \
    -fill '#ff00ff' -draw 'rectangle 800,240 959,399' \
    -depth 8 "$@" "$out"
}

if [[ ${1:-} == overcap ]]; then
  d=${2:?dir}; mkdir -p "$d"
  # 12000x9000 = 108 MP, over a 100 MP cap. Uniform, so the file is small but decode is not.
  vips black "$d/overcap.v" 12000 9000 --bands 3 && vips jpegsave "$d/overcap.v" "$d/overcap.jpg" --Q 50 && rm "$d/overcap.v"
  # 40000x1500 = 60 MP panorama, under the cap; checks extreme aspect.
  vips gaussnoise "$d/pano.v" 40000 1500 && vips colourspace "$d/pano.v" "$d/pano2.v" srgb && vips jpegsave "$d/pano2.v" "$d/pano.jpg" --Q 70 && rm "$d"/pano*.v
  # PNG decompression bomb: 20000x20000 (400 MP) of zeros, ~1 MB on disk.
  vips black "$d/bomb.v" 20000 20000 && vips pngsave "$d/bomb.v" "$d/bomb.png" --compression 9 && rm "$d/bomb.v"
  exit 0
fi

rm -f ./*.jpg ./*.png ./*.webp ./*.heic ./*.gif
scene upright.png
q=(-quality 85 -strip)
magick upright.png "${q[@]}" upright.jpg

# EXIF orientation N: store pixels pre-transformed by the inverse, so a correct
# viewer shows exactly upright.png.
magick upright.png -rotate 180 "${q[@]}" orient3.jpg
magick upright.png -rotate 270 "${q[@]}" orient6.jpg # stored 800x1200, viewer turns 90 CW
magick upright.png -rotate 90 "${q[@]}" orient8.jpg  # stored 800x1200, viewer turns 90 CCW
exiftool -q -overwrite_original -n -Orientation=3 orient3.jpg
exiftool -q -overwrite_original -n -Orientation=6 orient6.jpg
exiftool -q -overwrite_original -n -Orientation=8 orient8.jpg
# The mirrored ones: each transform is its own inverse.
magick upright.png -flop "${q[@]}" orient2.jpg
magick upright.png -flip "${q[@]}" orient4.jpg
magick upright.png -transpose "${q[@]}" orient5.jpg  # stored 800x1200
magick upright.png -transverse "${q[@]}" orient7.jpg # stored 800x1200
for o in 2 4 5 7; do exiftool -q -overwrite_original -n -Orientation=$o orient$o.jpg; done

# HEIC: pixels stored sideways, rotation carried by the HEIF 'irot' box (not EXIF).
# heif-enc turns the JPEG's EXIF orientation 6 into irot and also copies the EXIF
# (still saying 6), like iPhone files: both describe the same turn, apply only one.
heif-enc -q 60 -o rotated.heic orient6.jpg >/dev/null

# Display-P3: sRGB scene converted to P3 numbers with the P3 profile embedded.
# Correct pipeline output is the sRGB scene again (red ~ 255,0,0); a pipeline that
# ignores the profile gives red ~ 234,51,35.
vips icc_transform upright.png p3.v p3 --input-profile srgb --embedded=false
vips jpegsave p3.v p3.jpg --Q 90 --keep icc
rm p3.v

# GPS + camera EXIF + XMP GPS: everything must be gone from every output.
cp upright.jpg gps.jpg
exiftool -q -overwrite_original \
  -GPSLatitude=59.9139 -GPSLatitudeRef=N -GPSLongitude=10.7522 -GPSLongitudeRef=E \
  -XMP-exif:GPSLatitude=59.9139N -XMP-exif:GPSLongitude=10.7522E \
  -Make=SpikeCam -Model=Fixture -DateTimeOriginal='2020:01:01 12:00:00' \
  -IPTC:City=Oslo gps.jpg
# The same planted metadata in the other containers an upload brings it in:
# an iPhone's HEIC, a PNG's eXIf and iTXt chunks, and an Ultra HDR JPEG,
# whose gain map rides in an MPF segment.
heif-enc -q 60 -o gps.heic gps.jpg >/dev/null
magick upright.png -depth 8 gps.png
vips colourspace upright.png hdr.v scrgb && vips linear hdr.v hdr2.v 3 0 && vips uhdrsave hdr2.v uhdr.jpg
rm hdr.v hdr2.v
plant=(-GPSLatitude=59.9139 -GPSLatitudeRef=N -XMP-exif:GPSLatitude=59.9139N -Make=SpikeCam -Model=Fixture)
exiftool -q -overwrite_original "${plant[@]}" gps.png
exiftool -q -overwrite_original "${plant[@]}" -IPTC:City=Oslo uhdr.jpg

# PNG with alpha: transparent hole at x=400..719, y=300..499 (upright coords).
magick upright.png \( -size 1200x800 xc:white -fill black -draw 'rectangle 400,300 719,499' \) \
  -alpha off -compose CopyOpacity -composite -depth 8 PNG32:alpha.png

magick upright.png -quality 80 upright.webp
# WebP/PNG can carry EXIF orientation too and vips honours it; browsers vary.
magick upright.png -rotate 270 -quality 80 orient6.webp
exiftool -q -overwrite_original -n -Orientation=6 orient6.webp

# Broken inputs.
head -c "$(($(stat -c %s upright.jpg) / 2))" upright.jpg > truncated.jpg
: > empty.jpg
printf 'this is not an image\n' > text.jpg
# Text that libvips' matrixload accepts as a 2x2 image. matrixload is not
# flagged untrusted, so VIPS_BLOCK_UNTRUSTED does not stop it.
printf '2 2\n0 0\n0 0\n' > matrix.jpg

# Header claims 60000x60000 (3.6 GP) while the file is tiny: pixel cap must trip
# from the header alone. Patch the SOF0 dimensions of a small baseline JPEG.
magick -size 16x16 xc:red -strip -quality 50 small.jpg
LC_ALL=C perl -0777 -pe 's/(\xff\xc0..\x08)....([\x01\x03])/$1\xea\x60\xea\x60$2/s' small.jpg > bomb-dims.jpg
rm small.jpg

magick upright.png -colorspace CMYK -quality 85 cmyk.jpg
magick upright.png -alpha off -depth 16 -define png:bit-depth=16 rgb16.png

# Animated: 3 frames, first frame is the scene.
magick upright.png \( upright.png -negate \) \( upright.png -flop \) -set delay 50 -loop 0 -resize 50% animated.gif
magick upright.png \( upright.png -negate \) \( upright.png -flop \) -set delay 50 -loop 0 -resize 50% -quality 70 animated.webp

rm upright.png
