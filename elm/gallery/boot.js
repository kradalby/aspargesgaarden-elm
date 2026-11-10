// Starts the lightbox (Gallery.elm) on every gallery ui.Gallery rendered.
// The grid stays the server's: Elm mounts on an empty node of its own, so
// nothing moves, and a click the lightbox does not want stays a plain link.
document.querySelectorAll("[data-gallery]").forEach((grid, n) => {
  const mount = document.createElement("div");
  document.body.append(mount);

  const app = Elm.Gallery.init({
    node: mount,
    flags: {
      id: "gallery-" + n,
      photos: JSON.parse(grid.dataset.gallery),
      viewport: { width: innerWidth, height: innerHeight },
    },
  });

  let opener = null;

  grid.addEventListener("click", (e) => {
    const link = e.target.closest("a[data-index]");
    // A new-tab click (or a credit link) is the visitor's choice.
    if (
      !link ||
      e.button !== 0 ||
      e.metaKey ||
      e.ctrlKey ||
      e.shiftKey ||
      e.altKey
    )
      return;
    e.preventDefault();
    opener = link;
    app.ports.open.send(Number(link.dataset.index));
  });

  app.ports.closed.subscribe(() => opener?.focus());
});
