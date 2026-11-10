import { h, icon } from "./dom.js";

// ------------------------------------------------------------------ toasts
const ICONS = { success: "check", error: "circle-alert", info: "info" };

export function toast(message, { type = "info", detail = "", timeout = 4200 } = {}) {
  const box = document.getElementById("toasts");
  const el = h("div", { class: `toast ${type}` }, icon(ICONS[type] || "info"), h("div", null, message, detail ? h("small", { text: detail }) : null),
    h("button", { class: "icon-btn", type: "button", "aria-label": "Dismiss", onclick: () => close() }, icon("x", "sm")));
  const close = () => {
    el.classList.add("out");
    setTimeout(() => el.remove(), 200);
  };
  box.append(el);
  while (box.children.length > 4) box.firstChild.remove();
  if (timeout) setTimeout(close, timeout);
}

export const toastError = (err, fallback = "Something went wrong") => toast(err?.message || fallback, { type: "error", timeout: 6000 });

// ------------------------------------------------------------------ dialogs
export function confirmDialog({ title, message, confirm = "Confirm", danger = false }) {
  return new Promise((resolve) => {
    const ok = h("button", { class: `btn ${danger ? "solid-danger" : "primary"}`, type: "button", text: confirm });
    const cancel = h("button", { class: "btn", type: "button", text: "Cancel" });
    const dlg = h("dialog", { class: "modal", "aria-label": title }, h("div", { class: "modal-head" }, h("h2", { text: title })),
      message ? h("div", { class: "modal-body" }, h("p", { text: message })) : null, h("div", { class: "modal-foot" }, cancel, ok));
    let result = false;
    ok.addEventListener("click", () => { result = true; dlg.close(); });
    cancel.addEventListener("click", () => dlg.close());
    dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
    dlg.addEventListener("close", () => { dlg.remove(); resolve(result); });
    document.body.append(dlg);
    dlg.showModal();
    (danger ? cancel : ok).focus();
  });
}

// ------------------------------------------------------------------ clipboard
// The LAN address is plain http, where navigator.clipboard is often missing.
export async function copyText(text, label = "Copied") {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
    } else {
      const ta = h("textarea", { class: "sr-only", readonly: true });
      ta.value = text;
      document.body.append(ta);
      ta.select();
      const ok = document.execCommand("copy");
      ta.remove();
      if (!ok) throw new Error("copy failed");
    }
    toast(label, { type: "success", timeout: 2000 });
  } catch {
    toast("Couldn't copy automatically", { type: "error", detail: text });
  }
}

// ------------------------------------------------------------------ states
export const loading = (text = "Loading…") => h("div", { class: "loading" }, icon("loader-circle", "spin"), text);

export function emptyState({ iconName = "clapperboard", title, text, actions = [] }) {
  return h("div", { class: "empty" }, icon(iconName, "xl"), h("h3", { text: title }), text ? h("p", { text }) : null,
    actions.length ? h("div", { class: "actions" }, actions) : null);
}

export function errorState(err, retry) {
  return emptyState({
    iconName: "circle-alert",
    title: err?.status === 0 ? "Server unreachable" : "Couldn't load this page",
    text: err?.message || "Unknown error",
    actions: retry ? [h("button", { class: "btn", type: "button", onclick: retry }, icon("refresh-cw", "sm"), "Try again")] : [],
  });
}

export function link(href, props, ...children) {
  return h("a", { href, "data-link": "", ...props }, ...children);
}

// Page title as in the sister apps: "All Videos  8 videos". `count` may be a node updated later.
export function pageHead(title, sub, actions = [], count = null) {
  return h("div", { class: "page-head" }, h("div", null, h("h1", null, title, count), sub ? h("p", { text: sub }) : null),
    actions.length ? h("div", { class: "actions" }, actions) : null);
}

// ------------------------------------------------------------------ dropdown menu
// entries: { label, icon?, checked?, danger?, onSelect } | { separator: true } | { heading }
let openMenuEl = null;
export function closeMenu() {
  openMenuEl?.remove();
  openMenuEl = null;
}
export function openMenu(anchor, entries) {
  if (openMenuEl && openMenuEl.dataset.owner === anchor.dataset.menuId) return closeMenu();
  closeMenu();
  anchor.dataset.menuId ||= String(Math.random());
  const menu = h("div", { class: "menu", role: "menu", dataset: { owner: anchor.dataset.menuId } });
  for (const e of entries) {
    if (e.separator) { menu.append(h("hr")); continue; }
    if (e.heading) { menu.append(h("div", { class: "menu-label", text: e.heading })); continue; }
    menu.append(h("button", { type: "button", role: e.checked !== undefined ? "menuitemradio" : "menuitem", "aria-checked": e.checked !== undefined ? String(!!e.checked) : null, class: e.danger ? "danger" : "",
      onclick: () => { closeMenu(); e.onSelect?.(); } }, h("span", { class: "check" }, e.checked ? icon("check", "sm") : e.icon ? icon(e.icon, "sm") : null), h("span", { text: e.label })));
  }
  document.body.append(menu);
  const r = anchor.getBoundingClientRect();
  const w = menu.offsetWidth;
  menu.style.top = `${Math.round(r.bottom + 6)}px`;
  menu.style.left = `${Math.round(Math.max(8, Math.min(window.innerWidth - w - 8, r.right - w)))}px`;
  openMenuEl = menu;
  menu.querySelector("button")?.focus({ preventScroll: true });
  setTimeout(() => {
    const off = (ev) => {
      if (!menu.isConnected) return document.removeEventListener("pointerdown", off, true);
      if (!menu.contains(ev.target) && !anchor.contains(ev.target)) { closeMenu(); document.removeEventListener("pointerdown", off, true); }
    };
    document.addEventListener("pointerdown", off, true);
  });
}
document.addEventListener("keydown", (e) => { if (e.key === "Escape" && openMenuEl) closeMenu(); });

// A dialog in the family style: title bar with a close button, body, optional footer.
export function modal({ title, body, footer, cls = "" }) {
  const close = h("button", { class: "icon-btn", type: "button", "aria-label": "Close", title: "Close" }, icon("x"));
  const dlg = h("dialog", { class: `modal ${cls}`.trim(), "aria-label": title },
    h("div", { class: "modal-head" }, h("h2", { text: title }), close), body, footer ? h("div", { class: "modal-foot" }, footer) : null);
  close.addEventListener("click", () => dlg.close());
  dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
  dlg.addEventListener("close", () => dlg.remove());
  document.body.append(dlg);
  dlg.showModal();
  return dlg;
}
