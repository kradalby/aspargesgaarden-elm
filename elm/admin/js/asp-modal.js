// <asp-modal><dialog>…</dialog></asp-modal> opens its dialog as a modal as
// soon as Elm inserts it, so the page behind is inert and focus stays inside;
// Elm cannot call showModal() itself. Elm closes it by removing the element,
// and hears Escape through the dialog's own `cancel` event.
class AspModal extends HTMLElement {
  connectedCallback() {
    const d = this.querySelector(":scope > dialog");
    if (d && !d.open) d.showModal();
  }
}

customElements.define("asp-modal", AspModal);
