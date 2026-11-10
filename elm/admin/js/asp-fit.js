// <asp-fit width="1280" height="800"><iframe …></iframe></asp-fit> shows a
// frame laid out at a fixed size, scaled down to the width it is given, so a
// desktop-width page can be seen in a narrow column. It sets --asp-scale for
// the child's transform and its own height; Elm owns neither.
//
// A new srcdoc reloads the frame at its top; it is put back where she had
// scrolled, so she can watch the part she is writing.
class AspFit extends HTMLElement {
  static observedAttributes = ["width", "height"];

  constructor() {
    super();
    this.frameY = 0;
    // Where she has the old page, read as Elm swaps in the new one. The
    // frame runs no script (WebKit then fires no scroll events there), so
    // its position is read, not followed.
    new MutationObserver((ms) => {
      const w = ms[0].target.contentWindow;
      if (w) this.frameY = w.scrollY;
    }).observe(this, { subtree: true, attributeFilter: ["srcdoc"] });
    // load does not bubble, but an ancestor sees it while it is captured.
    this.addEventListener("load", (e) => this.restore(e.target), true);
  }

  connectedCallback() {
    // Its own height changes in fit(), so it waits a frame rather than
    // resizing inside the observer's callback.
    this.ro ||= new ResizeObserver(() =>
      requestAnimationFrame(() => this.fit()),
    );
    this.ro.observe(this);
    this.fit();
  }

  disconnectedCallback() {
    this.ro?.disconnect();
  }

  attributeChangedCallback() {
    if (this.isConnected) this.fit();
  }

  restore(frame) {
    const w = frame.contentWindow;
    if (!w) return;
    // A page she has already scrolled while its pictures loaded stays where
    // she put it.
    if (w.scrollY > 0) this.frameY = w.scrollY;
    const y = this.frameY;
    w.scrollTo({ top: y, behavior: "instant" });
    const put = w.scrollY;
    // The site's fonts can arrive after load and make the page taller than
    // it was when it was put back; try again then, unless she has scrolled.
    w.document.fonts.ready.then(() => {
      if (w.scrollY === put) w.scrollTo({ top: y, behavior: "instant" });
    });
  }

  fit() {
    const w = Number(this.getAttribute("width"));
    const h = Number(this.getAttribute("height"));
    if (!(w > 0 && h > 0) || !this.clientWidth) return;
    const s = Math.min(1, this.clientWidth / w);
    const cs = getComputedStyle(this);
    const border =
      parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth);
    this.style.setProperty("--asp-scale", String(s));
    this.style.height = `${Math.round(h * s + border)}px`;
  }
}

customElements.define("asp-fit", AspFit);
