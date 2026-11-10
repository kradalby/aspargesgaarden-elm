#!/usr/bin/env bash
# Regenerates this miniature old-site checkout's photos: synthetic gradients,
# licence-clean and small, one colour each so no two share a hash. The sizes
# are the ones import_test.go asserts crops for. Needs vips.
set -euo pipefail
cd "$(dirname "$0")"

i=0
photo() { # path width height
  local out=public/$1.jpeg tmp r g b
  tmp=$(mktemp -d)
  i=$((i + 1))
  r=$(((i * 97) % 200 + 40)) g=$(((i * 53) % 200 + 40)) b=$(((i * 31) % 200 + 40))
  mkdir -p "$(dirname "$out")"
  vips grey "$tmp/ramp.v" "$2" "$3"
  vips linear "$tmp/ramp.v" "$tmp/rgb.v" "$((r / 2)) $((g / 2)) $((b / 2))" "$((r / 2)) $((g / 2)) $((b / 2))"
  vips cast "$tmp/rgb.v" "$tmp/rgb8.v" uchar
  vips copy "$tmp/rgb8.v" "$tmp/srgb.v" --interpretation srgb
  vips jpegsave "$tmp/srgb.v" "$out" --Q 80 --keep none
  rm -r "$tmp"
}

for p in arrangementer/patrick arrangementer/anita arrangementer/julemarked1 \
  arrangementer/2026-11-14-julemarked arrangementer/julemarked2 arrangementer/julemarked3 \
  arrangementer/julemarked4 galleri/a galleri/c galleri/d galleri/e galleri/f galleri/g \
  galleri/h galleri/i galleri/j galleri/k; do
  photo "$p" 1300 800
done
# Too narrow for a banner or a share image; fine for the gallery.
photo galleri/b 600 900
# Too narrow for anything, as one old event photo is.
photo arrangementer/liten 393 479
