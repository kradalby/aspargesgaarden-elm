// Geometry for the cropper wrapper. All rects are {x,y,w,h}.
// "source" = full-size upright original; "preview" = naturalWidth/Height of the <img> shown.

// "4:5", "1200:630", "16/9" or "0.8".
export function parseAspect(s) {
  const m = String(s || "").match(/^\s*([\d.]+)\s*[:/]\s*([\d.]+)\s*$/);
  const v = m ? Number(m[1]) / Number(m[2]) : Number(s);
  return v > 0 && Number.isFinite(v) ? v : NaN;
}

export function parseRect(s) {
  const p = String(s || "")
    .split(",")
    .map(Number);
  return p.length === 4 && p.every(Number.isFinite) && p[2] > 0 && p[3] > 0
    ? { x: p[0], y: p[1], w: p[2], h: p[3] }
    : null;
}

export const rectString = (r) => `${r.x},${r.y},${r.w},${r.h}`;

// Largest centred rect of `aspect` inside W x H.
export function defaultRect(W, H, aspect) {
  let w = W,
    h = Math.round(W / aspect);
  if (h > H) {
    h = H;
    w = Math.round(H * aspect);
  }
  return { x: Math.floor((W - w) / 2), y: Math.floor((H - h) / 2), w, h };
}

// Preview-space float rect -> integer source rect. h is derived from w so the
// aspect stays as exact as integers allow; the result is clamped inside the source.
export function toSource(p, previewW, previewH, W, H, aspect) {
  const kx = W / previewW,
    ky = H / previewH;
  let w = Math.min(W, Math.round(p.w * kx));
  let h = Math.round(w / aspect);
  if (h > H) {
    h = H;
    w = Math.round(h * aspect);
  }
  const x = Math.min(Math.max(0, Math.round(p.x * kx)), W - w);
  const y = Math.min(Math.max(0, Math.round(p.y * ky)), H - h);
  return { x, y, w, h };
}

export function toPreview(r, previewW, previewH, W, H) {
  const kx = previewW / W,
    ky = previewH / H;
  return { x: r.x * kx, y: r.y * ky, w: r.w * kx, h: r.h * ky };
}

// Common attribute handling for <asp-cropper>. Subclasses implement build() and
// applyRect(sourceRect), and call this.report(previewRect) on every change.
export class CropperBase extends HTMLElement {
  static observedAttributes = [
    "src",
    "aspect",
    "rect",
    "source-width",
    "source-height",
  ];

  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" });
    this.schedule("build");
    this.ro ||= new ResizeObserver(() => this.ready && this.onResize());
    this.ro.observe(this);
  }

  disconnectedCallback() {
    this.ro?.disconnect();
    this.teardown?.();
    this.ready = false;
  }

  // Elm sets attributes one at a time during a patch; batch them into one rebuild.
  attributeChangedCallback(name, old, val) {
    if (!this.isConnected || old === val) return;
    if (name === "rect") {
      // Elm echoing back what we just emitted must not move the selection.
      if (val === this.lastEmitted) return;
      this.schedule(this.pending || "rect");
    } else {
      this.schedule("build");
    }
  }

  schedule(kind) {
    if (kind === "build" || !this.pending) this.pending = kind;
    if (this.queued) return;
    this.queued = true;
    queueMicrotask(() => {
      this.queued = false;
      const k = this.pending;
      this.pending = null;
      if (k === "build") {
        this.ready = false;
        this.teardown?.();
        this.build();
      } else if (this.ready) this.applyRect(this.sourceRect());
    });
  }

  get aspect() {
    return parseAspect(this.getAttribute("aspect"));
  }

  // Source size; defaults to the preview's own size (no downscale).
  sourceSize(img) {
    const W = Number(this.getAttribute("source-width")) || img.naturalWidth;
    const H = Number(this.getAttribute("source-height")) || img.naturalHeight;
    return { W, H };
  }

  // Rect to show: attribute if valid and inside the source, else centred default.
  sourceRect() {
    const { W, H } = this.sourceSize(this.img);
    const r = parseRect(this.getAttribute("rect"));
    return r && r.x >= 0 && r.y >= 0 && r.x + r.w <= W && r.y + r.h <= H
      ? r
      : defaultRect(W, H, this.aspect);
  }

  // r is the rect shown now, as Elm has it; Elm echoing it back is no move.
  settle(r) {
    this.current = r;
    this.lastEmitted = rectString(r);
  }

  report(p) {
    const { W, H } = this.sourceSize(this.img);
    const r = toSource(
      p,
      this.img.naturalWidth,
      this.img.naturalHeight,
      W,
      H,
      this.aspect,
    );
    const s = rectString(r);
    if (s === this.lastEmitted) return;
    this.lastEmitted = s;
    this.current = r;
    this.dispatchEvent(new CustomEvent("crop", { detail: r, bubbles: true }));
  }
}
