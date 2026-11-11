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

// Tab goes round inside the open lightbox. Here, not in Elm: Elm moves focus
// a frame later, and a quick second Tab got past it.
document.addEventListener("keydown", (e) => {
  const box = document.querySelector("[data-lightbox]");
  if (e.key !== "Tab" || !box) return;
  const controls = box.querySelectorAll("a[href], button");
  const [first, last] = [controls[0], controls[controls.length - 1]];
  const at = document.activeElement;
  if (at === box || !box.contains(at) || at === (e.shiftKey ? first : last)) {
    e.preventDefault();
    (e.shiftKey ? last : first).focus();
  }
});
