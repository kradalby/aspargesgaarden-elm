// <asp-fit width="1280" height="800"><iframe …></iframe></asp-fit> shows a
// frame laid out at a fixed size, scaled down to the width it is given, so a
// desktop-width page can be seen in a narrow column. It sets --asp-scale for
// the child's transform and its own height; Elm owns neither.
class AspFit extends HTMLElement {
  static observedAttributes = ["width", "height"];

  connectedCallback() {
    this.ro ||= new ResizeObserver(() => this.fit());
    this.ro.observe(this);
    this.fit();
  }

  disconnectedCallback() {
    this.ro?.disconnect();
  }

  attributeChangedCallback() {
    if (this.isConnected) this.fit();
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
