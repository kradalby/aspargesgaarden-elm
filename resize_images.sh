#!/usr/bin/env sh

for width in 320 640 768 1024 1280 1536 2048; do
	fd -e jpeg -E "*resize*" -x convert {} -resize "${width}x>" {.}"_${width}w_resize.jpeg"
done

./node_modules/.bin/optimizt --avif --webp public/
