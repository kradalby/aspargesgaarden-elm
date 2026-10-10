# Website maintenance

- Keep elm-pages pinned to v2.
- Follow existing layouts, typography, and formatting.
- Preserve historical pages and events. Add year-specific routes for recurring pages and update menu links.

## Events

- Add Markdown files to `content/events/`, following existing frontmatter.
- Set `title`, `eventDate`, `publishedFrom`, and `photo`. Use `YYYY-MM-DD` dates and `HH:mm` times; publication must precede the first event date.
- Use end-date/time fields for continuous events. Use `eventDates` for separate dates of the same course, keeping one listing.
- Preserve supplied programme, price, venue, and registration details. Make phone and SMS registration clickable.
- Check publication visibility, ordering, and upcoming/past placement before, between, and after event dates.

## Images

- Preserve untouched, full-resolution originals in `grafikk/`. Put website JPEG masters in `public/`.
- Inspect subjects before cropping; preserve faces, hands, and useful context.
- Target approximately 4:5 for new event portrait crops, matching the course images. Reused photos may retain their original ratio.
- Preserve aspect ratios when resizing and displaying images; never stretch them.
- Use `process-images` locally and cached per-image Nix builds in CI. Never commit generated variants or force unnecessary regeneration.

## Validation and publishing

- Use the Nix development shell. Run a local server and capture desktop/mobile screenshots.
- Check dates, registration links, image proportions, broken images, and overflow.
- Run the Nix build and formatting check. Avoid bulk formatting event Markdown with JSON frontmatter.
- Use the commit skill and mandatory hooks. Push only when authorized.
- Follow both CI and GitHub Pages deployment. Verify updated content and images on the public site before reporting it live.
