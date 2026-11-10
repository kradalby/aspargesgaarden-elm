// <asp-cropper> on Cropper.js v1.6: getData()/setData() already speak preview
// (natural image) pixels, so the wrapper only scales to source pixels.
import Cropper from "cropperjs";
import css from "cropperjs/dist/cropper.css";
import { CropperBase, toPreview } from "./rect.js";

const hostCss = `
:host { display: block; }
.box { width: 100%; max-height: var(--asp-cropper-max-height, 55vh); }
img { display: block; max-width: 100%; }
`;

// A constructed sheet, not a <style> element: the admin's CSP is
// style-src 'self', which blocks inline style elements but not the CSSOM.
const sheet = new CSSStyleSheet();
sheet.replaceSync(css + hostCss);

class AspCropper extends CropperBase {
  build() {
    const root = this.shadowRoot;
    root.adoptedStyleSheets = [sheet];
    root.innerHTML = `<div class="box"><img alt=""></div>`;
    this.box = root.querySelector(".box");
    this.img = root.querySelector("img");
    this.img.onload = () => {
      const { naturalWidth: nw, naturalHeight: nh } = this.img;
      // Cropper sizes itself from the container, which needs a real height.
      this.box.style.aspectRatio = `${nw} / ${nh}`;
      this.create(this.sourceRect());
    };
    this.img.src = this.getAttribute("src");
  }

  // Only her own drags are reported. The rect it starts from, or that Elm
  // sets, is already Elm's; reporting it back in rounded preview pixels
  // would change a crop she never touched.
  create(rect) {
    this.settle(rect);
    this.cropper = new Cropper(this.img, {
      viewMode: 1, // crop box stays inside the image
      dragMode: "move", // dragging outside the box pans the image, never starts a new box
      aspectRatio: this.aspect,
      autoCropArea: 1,
      background: false, // its checkered background is a data: image, which the CSP forbids
      checkOrientation: false, // the browser already applies EXIF orientation to <img>
      rotatable: false,
      scalable: false,
      // The wheel scrolls the dialog; Større and Mindre resize the frame.
      zoomable: false,
      toggleDragModeOnDblclick: false,
      responsive: false, // ResizeObserver below covers window and container resizes
      data: this.dataFor(rect),
      ready: () => {
        this.ready = true;
      },
      cropstart: () => {
        this.dragging = true;
      },
      crop: () => this.dragging && this.emit(),
      cropend: () => {
        this.emit();
        this.dragging = false;
      },
    });
  }

  dataFor(r) {
    const { W, H } = this.sourceSize(this.img);
    const p = toPreview(r, this.img.naturalWidth, this.img.naturalHeight, W, H);
    return { x: p.x, y: p.y, width: p.w, height: p.h };
  }

  emit() {
    const d = this.cropper.getData();
    this.report({ x: d.x, y: d.y, w: d.width, h: d.height });
  }

  applyRect(r) {
    this.settle(r);
    this.cropper.setData(this.dataFor(r));
  }

  // Cropper's own resize() restores by scaling the old box and drifted in tests;
  // a fresh instance from the last source rect is exact.
  onResize() {
    this.ready = false;
    this.cropper.destroy();
    this.create(this.current);
  }

  teardown() {
    this.cropper?.destroy();
    this.cropper = null;
  }
}

customElements.define("asp-cropper", AspCropper);
