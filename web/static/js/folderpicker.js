// Choose a folder on the ENStream computer: the native dialog in the desktop app,
// otherwise a small browser of the server's folders.
import { h, icon, swap } from "./dom.js";
import { get, query } from "./api.js";
import { desktop, pickFolderNative } from "./desktop.js";
import { loading } from "./ui.js";

const KIND_ICON = { home: "house", videos: "film", drive: "hard-drive", removable: "hard-drive" };

export async function pickFolder(start = "") {
  if (desktop.available) {
    try {
      return await pickFolderNative();
    } catch { /* fall back to the browser below */ }
  }
  return new Promise((resolve) => {
    let current = "";
    let result = null;
    const crumbs = h("div", { class: "picker-path mono" });
    const list = h("div", { class: "picker-list", role: "listbox", "aria-label": "Folders" });
    const choose = h("button", { class: "btn primary", type: "button", disabled: true }, icon("check", "sm"), "Use this folder");
    const cancel = h("button", { class: "btn", type: "button", text: "Cancel" });
    const dlg = h("dialog", { class: "modal picker", "aria-label": "Choose a folder" },
      h("div", { class: "modal-body" }, h("h2", { text: "Choose a folder" }), h("p", { text: "Folders on the computer running ENStream." }), crumbs, list),
      h("div", { class: "modal-foot" }, cancel, choose));
    async function open(path) {
      swap(list, loading());
      let d;
      try {
        d = await get("/fs/dirs" + query({ path }));
      } catch (err) {
        swap(list, h("p", { class: "muted picker-empty", text: err.message }));
        return;
      }
      current = d.path;
      choose.disabled = !current;
      crumbs.textContent = current || "Places";
      const rows = [];
      if (current) rows.push(h("button", { class: "picker-row up", type: "button", onclick: () => open(d.parent || "") }, icon("arrow-left", "sm"), d.parent ? "Up" : "Places"));
      for (const x of d.dirs) {
        rows.push(h("button", { class: "picker-row", type: "button", title: x.path, onclick: () => open(x.path) }, icon(KIND_ICON[x.kind] || "folder", "sm"), h("span", { text: x.name }), icon("chevron-right", "sm")));
      }
      if (!d.dirs.length) rows.push(h("p", { class: "muted picker-empty", text: current ? "No subfolders here. Use this folder, or go up." : "No places found." }));
      swap(list, rows);
      list.scrollTop = 0;
    }
    choose.addEventListener("click", () => { result = current; dlg.close(); });
    cancel.addEventListener("click", () => dlg.close());
    dlg.addEventListener("close", () => { dlg.remove(); resolve(result); });
    document.body.append(dlg);
    dlg.showModal();
    open(start);
  });
}
