// <asp-md-insert text="[909 19 812](tel:+4790919812)">Telefonlenke</asp-md-insert>
// Inside <markdown-toolbar for="id">: inserts `text` at the cursor (replacing the
// selection) of the toolbar's textarea. Uses execCommand('insertText') like
// markdown-toolbar-element does, so the edit lands on the undo stack and the
// browser fires a native `input` event that Elm's onInput sees.
import "@github/markdown-toolbar-element";

class AspMdInsert extends HTMLElement {
  connectedCallback() {
    if (!this.hasAttribute("role")) this.setAttribute("role", "button");
    if (!this.hasAttribute("tabindex")) this.setAttribute("tabindex", "0");
    if (this.wired) return;
    this.wired = true;
    // mousedown would move focus off the textarea; keep the caret where it is.
    this.addEventListener("mousedown", (e) => e.preventDefault());
    this.addEventListener("click", () => this.insert());
    this.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        this.insert();
      }
    });
  }

  insert() {
    const ta = this.closest("markdown-toolbar")?.field;
    const text = this.getAttribute("text");
    if (!ta || !text) return;
    ta.focus();
    if (!document.execCommand("insertText", false, text)) {
      // shortcut: fallback loses undo; only hit on engines without insertText on textarea.
      ta.setRangeText(text, ta.selectionStart, ta.selectionEnd, "end");
      ta.dispatchEvent(
        new InputEvent("input", {
          bubbles: true,
          inputType: "insertText",
          data: text,
        }),
      );
    }
  }
}

customElements.define("asp-md-insert", AspMdInsert);
