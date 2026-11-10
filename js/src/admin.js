// Entry for the admin page: the custom elements the Elm app renders, and the
// start of the app itself (the page has no inline script, so the CSP can
// forbid them). Load after the Elm output; both with `defer`.
import "./asp-cropper.js";
import "./asp-md-insert.js";
import "./asp-modal.js";
import "./asp-fit.js";

// Elm cannot listen to beforeunload, and the browser only warns before leaving
// when the handler asks for it, so the app reports its unsaved state here.
let dirty = false;
addEventListener("beforeunload", (e) => {
  if (!dirty) return;
  e.preventDefault();
  e.returnValue = "";
});

const app = window.Elm.Main.init({ flags: null });
app.ports.setDirty.subscribe((d) => {
  dirty = d;
});
