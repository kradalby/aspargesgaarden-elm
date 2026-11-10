// Starts the admin app (Admin.elm) and registers the custom elements it
// renders. build.sh bundles this with its imports (package.json pins them)
// and appends it to the Elm output; the page runs no inline script.
import "./js/asp-cropper.js";
import "./js/asp-md-insert.js";
import "./js/asp-modal.js";
import "./js/asp-fit.js";

// Elm cannot listen to beforeunload, and the browser only warns before leaving
// when the handler asks for it, so the app reports its unsaved state here.
let dirty = false;
addEventListener("beforeunload", (e) => {
  if (!dirty) return;
  e.preventDefault();
  e.returnValue = "";
});

const app = window.Elm.Admin.init({ flags: null });
app.ports.setDirty.subscribe((d) => {
  dirty = d;
});
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") app.ports.pageHidden.send(null);
});
