// Bridge to the ENStream desktop app (Tauri). In a normal browser `desktop.available` is
// false and nothing here runs; the web UI works the same without it.
import { h, icon, swap } from "./dom.js";

const T = window.__TAURI__;
export const desktop = {
  available: !!(T && T.core && T.core.invoke),
  invoke(cmd, args) {
    return T.core.invoke(cmd, args);
  },
  listen(event, fn) {
    return T.event.listen(event, (e) => fn(e.payload));
  },
  win() {
    return T.window.getCurrentWindow();
  },
};

export const pickFolderNative = () => desktop.invoke("pick_folder");
export const openExternal = (url) => desktop.invoke("open_external", { url });
export const openAppFolder = (which) => desktop.invoke("open_app_folder", { which });

const WIN_BTN = [["minimize", "minus", "Minimize"], ["maximize", "square", "Maximize"], ["close", "x", "Close"]];

// Frameless window: our own title bar, like ENViewer, ENSave, ENPlayer and ENPost.
function titleBar() {
  const updatePill = h("button", { class: "update-pill", type: "button", hidden: true, id: "update-pill" }, icon("download", "sm"), "New version available");
  const controls = h("div", { class: "win-controls" }, WIN_BTN.map(([act, ic, label]) =>
    h("button", { class: `win-btn ${act}`, type: "button", "aria-label": label, title: label, onclick: () => {
      const w = desktop.win();
      if (act === "minimize") w.minimize();
      else if (act === "maximize") w.toggleMaximize();
      else w.close();
    } }, icon(ic, "sm"))));
  const bar = h("div", { class: "titlebar", id: "titlebar" },
    h("img", { src: "/static/assets/enstream-icon.svg", alt: "", width: "20", height: "20", draggable: "false" }),
    h("span", { class: "titlebar-name", text: "ENStream" }), h("div", { class: "titlebar-space" }), updatePill, controls);
  bar.addEventListener("mousedown", (e) => {
    if (e.button !== 0 || e.target.closest("button,a,input")) return;
    if (e.detail === 2) desktop.win().toggleMaximize();
    else desktop.win().startDragging();
  });
  return bar;
}

// ------------------------------------------------------------------ updates
let pending = null;

export async function checkForUpdate({ quiet = true } = {}) {
  try {
    pending = await desktop.invoke("check_update");
  } catch (err) {
    if (!quiet) throw err;
    return null;
  }
  const pill = document.getElementById("update-pill");
  if (pill) pill.hidden = !pending;
  return pending;
}

export function showUpdateDialog() {
  if (!pending) return;
  const info = pending;
  const bar = h("i");
  const status = h("p", { class: "muted" });
  const progress = h("div", { class: "bar", hidden: true }, bar);
  const install = h("button", { class: "btn grad", type: "button" }, icon("download", "sm"), "Update now");
  const later = h("button", { class: "btn", type: "button", text: "Later" });
  const dlg = h("dialog", { class: "modal", "aria-label": "Update ENStream" },
    h("div", { class: "modal-body" }, h("h2", { text: `ENStream ${info.version} is available` }),
      h("p", { class: "update-notes", text: info.notes || "Improvements and fixes." }), progress, status),
    h("div", { class: "modal-foot" }, later, install));
  later.addEventListener("click", () => dlg.close());
  dlg.addEventListener("close", () => dlg.remove());
  install.disabled = !info.installable;
  if (!info.installable) status.textContent = "This copy can't update itself. Download the new version from the release page.";
  install.addEventListener("click", async () => {
    install.disabled = later.disabled = true;
    progress.hidden = false;
    status.textContent = "Downloading…";
    const off = await desktop.listen("update-progress", ({ done, total }) => {
      if (total) bar.style.width = `${Math.round((done / total) * 100)}%`;
    });
    try {
      const r = await desktop.invoke("install_update", { info });
      off();
      if (r === "restart") {
        status.textContent = "Installed. Restarting ENStream…";
        await desktop.invoke("restart_app");
      } else {
        status.textContent = "The installer is taking over…";
      }
    } catch (err) {
      off();
      status.textContent = String(err);
      install.disabled = later.disabled = false;
    }
  });
  document.body.append(dlg);
  dlg.showModal();
}

// ------------------------------------------------------------------ setup
export async function initDesktop() {
  if (!desktop.available) return;
  document.body.classList.add("desktop");
  const bar = titleBar();
  document.body.prepend(bar);
  bar.querySelector("#update-pill").addEventListener("click", showUpdateDialog);
  const maxBtn = bar.querySelector(".win-btn.maximize");
  const syncMax = async () => {
    const m = await desktop.win().isMaximized().catch(() => false);
    swap(maxBtn, icon(m ? "copy" : "square", "sm"));
    maxBtn.title = m ? "Restore" : "Maximize";
    document.body.classList.toggle("maximized", m);
  };
  syncMax();
  desktop.win().onResized(syncMax);

  // Links that leave ENStream open in the normal browser.
  document.addEventListener("click", (e) => {
    const a = e.target.closest("a[href]");
    if (!a) return;
    const url = new URL(a.href, location.href);
    const external = url.origin !== location.origin || a.target === "_blank";
    if (external) {
      e.preventDefault();
      e.stopPropagation();
      openExternal(url.href).catch(() => {});
    }
  }, true);

  // The window on the ENStream computer itself shows up in History as "This computer".
  try {
    if (!localStorage.getItem("enstream.desktop-named")) {
      await T.core.invoke("app_info");
      const { send } = await import("./api.js");
      await send("PUT", "/devices/current", { name: "This computer" });
      localStorage.setItem("enstream.desktop-named", "1");
    }
  } catch { /* ignore */ }

  // Daily update check (Settings → Desktop can turn it off).
  try {
    const s = await desktop.invoke("desktop_settings");
    if (s.auto_update && Date.now() / 1000 - (s.last_update_check || 0) > 86400) {
      await checkForUpdate();
      await desktop.invoke("set_desktop_setting", { key: "last_update_check", value: Math.floor(Date.now() / 1000) });
    }
  } catch { /* offline */ }
}
