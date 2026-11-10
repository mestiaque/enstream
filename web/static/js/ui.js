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
    const dlg = h("dialog", { class: "modal", "aria-label": title }, h("div", { class: "modal-body" }, h("h2", { text: title }), message ? h("p", { text: message }) : null),
      h("div", { class: "modal-foot" }, cancel, ok));
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

export function pageHead(title, sub, actions = []) {
  return h("div", { class: "page-head" }, h("div", null, h("h1", { text: title }), sub ? h("p", { text: sub }) : null),
    actions.length ? h("div", { class: "actions" }, actions) : null);
}
