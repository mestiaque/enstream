import { h, icon, clear, isTyping } from "./dom.js";
import { get, request } from "./api.js";
import { toast } from "./ui.js";
import { initDesktop } from "./desktop.js";

// ------------------------------------------------------------------ routes
// Each view module exports render(ctx) -> { el, destroy?, title? } (may be async).
const routes = [
  [/^\/$/, () => import("./views/home.js"), "home"],
  [/^\/library$/, () => import("./views/library.js"), "library"],
  [/^\/recent$/, () => import("./views/library.js"), "recent"],
  [/^\/continue$/, () => import("./views/history.js"), "continue"],
  [/^\/history$/, () => import("./views/history.js"), "history"],
  [/^\/video\/(\d+)$/, () => import("./views/details.js"), "details"],
  [/^\/watch\/(\d+)$/, () => import("./views/player.js"), "watch"],
  [/^\/folders$/, () => import("./views/folders.js"), "folders"],
  [/^\/settings$/, () => import("./views/settings.js"), "settings"],
  [/^\/login$/, () => import("./views/login.js"), "login"],
  [/^\/welcome$/, () => import("./views/welcome.js"), "welcome"],
];

const NAV = [
  { href: "/", label: "Home", icon: "house", key: "home" },
  { href: "/library", label: "All videos", short: "Library", icon: "film", key: "library", count: true },
  { href: "/continue", label: "Continue watching", short: "Continue", icon: "circle-play", key: "continue" },
  { href: "/recent", label: "Recently added", icon: "clock", key: "recent" },
  { href: "/history", label: "History", icon: "history", key: "history" },
  { section: "Manage" },
  { href: "/folders", label: "Folders & scan", short: "Folders", icon: "folder", key: "folders" },
  { href: "/settings", label: "Settings", icon: "settings", key: "settings" },
];
const BOTTOM = ["home", "library", "continue", "folders", "settings"];

const view = document.getElementById("view");
const sidebar = document.getElementById("sidebar");
const scrim = document.getElementById("scrim");
const searchInput = document.getElementById("search");
let current = null;
let renderSeq = 0;
let authRequired = false;
let pinEnabled = false;

// A PIN is set and this browser has no session: show the sign-in screen.
window.addEventListener("enstream:auth-required", () => {
  if (authRequired) return;
  authRequired = true;
  render();
});
export function signedIn(next) {
  authRequired = false;
  pinEnabled = true;
  updateSignOut();
  navigate(next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/login") ? next : "/", { replace: true });
  pollScan();
  refreshCount();
}
export function setPinEnabled(on) {
  pinEnabled = on;
  updateSignOut();
}

function buildNav() {
  const nav = document.getElementById("nav");
  for (const item of NAV) {
    if (item.section) {
      nav.append(h("div", { class: "nav-label", text: item.section }));
      continue;
    }
    nav.append(h("a", { class: "nav-link", href: item.href, "data-link": "", dataset: { key: item.key } }, icon(item.icon), h("span", { text: item.label }),
      item.count ? h("span", { class: "count", id: "nav-count" }) : null));
  }
  const bottom = document.getElementById("bottom-nav");
  for (const key of BOTTOM) {
    const item = NAV.find((n) => n.key === key);
    bottom.append(h("a", { href: item.href, "data-link": "", dataset: { key } }, icon(item.icon), h("span", { text: item.short || item.label })));
  }
  const menu = document.getElementById("menu-btn");
  menu.append(icon("menu"));
  menu.addEventListener("click", () => setMenu(true));
  scrim.addEventListener("click", () => setMenu(false));
  document.getElementById("search-icon").append(icon("search", "sm"));
}

function setMenu(open) {
  sidebar.classList.toggle("open", open);
  scrim.hidden = !open;
}

function setActive(key) {
  const k = key === "details" ? "library" : key;
  for (const a of document.querySelectorAll("[data-key]")) {
    const on = a.dataset.key === k;
    a.classList.toggle("active", on);
    if (on) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  }
}

// ------------------------------------------------------------------ navigation
export function navigate(to, { replace = false } = {}) {
  const url = new URL(to, location.origin);
  if (url.origin !== location.origin) return void (location.href = to);
  if (url.pathname + url.search === location.pathname + location.search && !replace) return;
  history[replace ? "replaceState" : "pushState"]({}, "", url.pathname + url.search + url.hash);
  render();
}

// Update the query string without re-rendering (filters, paging).
export function setQuery(params) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== null && v !== "") q.set(k, v);
  const s = q.toString();
  history.replaceState({}, "", location.pathname + (s ? "?" + s : ""));
}

async function render() {
  const seq = ++renderSeq;
  setMenu(false);
  const path = location.pathname.replace(/\/+$/, "") || "/";
  const match = routes.find(([re]) => re.test(path));
  try { current?.destroy?.(); } catch (e) { console.error(e); }
  current = null;
  document.body.classList.remove("watching", "signin", "setup");
  if (authRequired && path !== "/login") {
    history.replaceState({}, "", "/login?next=" + encodeURIComponent(location.pathname + location.search));
    return render();
  }
  if (!match) {
    clear(view).append(notFound());
    setActive("");
    document.title = "Not found · ENStream";
    return;
  }
  const [re, load, key] = match;
  setActive(key);
  const params = path.match(re).slice(1);
  const ctx = { key, params, query: new URLSearchParams(location.search), navigate, setQuery };
  // The search box mirrors the library's search; elsewhere it starts empty.
  searchInput.value = key === "library" ? ctx.query.get("search") || "" : "";
  try {
    const mod = await load();
    if (seq !== renderSeq) return;
    const result = await mod.render(ctx);
    if (seq !== renderSeq) {
      result?.destroy?.();
      return;
    }
    current = result;
    clear(view).append(result.el);
    document.title = result.title ? `${result.title} · ENStream` : "ENStream";
    if (!history.state?.scroll) window.scrollTo(0, 0);
    if (key !== "watch") view.focus({ preventScroll: true });
  } catch (err) {
    console.error(err);
    if (seq === renderSeq) clear(view).append(h("div", { class: "empty" }, icon("circle-alert", "xl"), h("h3", { text: "This page failed to load" }), h("p", { text: String(err.message || err) })));
  }
}

function notFound() {
  return h("div", { class: "empty" }, icon("ban", "xl"), h("h3", { text: "Page not found" }), h("p", { text: "This address doesn't exist in ENStream." }),
    h("div", { class: "actions" }, h("a", { class: "btn primary", href: "/", "data-link": "" }, icon("house", "sm"), "Go home")));
}

document.addEventListener("click", (e) => {
  const a = e.target.closest("a[data-link]");
  if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || a.target) return;
  e.preventDefault();
  navigate(a.getAttribute("href"));
});
window.addEventListener("popstate", render);

// ------------------------------------------------------------------ search
const form = document.getElementById("search-form");
let searchTimer;
searchInput.addEventListener("input", () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    const q = searchInput.value.trim();
    if (location.pathname === "/library") window.dispatchEvent(new CustomEvent("enstream:search", { detail: q }));
    else if (q) navigate("/library?search=" + encodeURIComponent(q));
  }, 300);
});
form.addEventListener("submit", (e) => {
  e.preventDefault();
  clearTimeout(searchTimer);
  const q = searchInput.value.trim();
  if (location.pathname === "/library") window.dispatchEvent(new CustomEvent("enstream:search", { detail: q }));
  else navigate("/library" + (q ? "?search=" + encodeURIComponent(q) : ""));
});
document.addEventListener("keydown", (e) => {
  if (e.key === "/" && !isTyping(e) && !document.body.classList.contains("watching")) {
    e.preventDefault();
    searchInput.focus();
    searchInput.select();
  } else if (e.key === "Escape" && sidebar.classList.contains("open")) {
    setMenu(false);
  }
});
export const setSearchText = (q) => { searchInput.value = q || ""; };

// ------------------------------------------------------------------ scan status
// One poller for the whole app; views listen for "enstream:scan".
const pill = document.getElementById("scan-pill");
const pillIcon = icon("loader-circle", "sm spin");
const pillLabel = h("span", { class: "label" });
pill.append(pillIcon, pillLabel);
let lastStatus = null;
let pollTimer;

export function scanStatus() { return lastStatus; }

export async function pollScan() {
  clearTimeout(pollTimer);
  try {
    const s = await get("/scanner/status");
    const wasRunning = lastStatus?.running;
    lastStatus = s;
    window.dispatchEvent(new CustomEvent("enstream:scan", { detail: s }));
    if (s.running) {
      const j = s.job;
      pill.hidden = false;
      pillLabel.textContent = j.total_files ? `Scanning ${j.processed_files + j.skipped_files} / ${j.total_files}` : "Scanning…";
    } else {
      pill.hidden = true;
      if (wasRunning && s.job) {
        const j = s.job;
        if (j.status === "completed") toast("Library scan finished", { type: "success", detail: `${j.new_files} new · ${j.updated_files} updated · ${j.removed_files} removed${j.failed_files ? ` · ${j.failed_files} failed` : ""}` });
        else if (j.status === "failed") toast("Scan failed", { type: "error", detail: j.error_message });
        else if (j.status === "cancelled") toast("Scan cancelled");
        window.dispatchEvent(new CustomEvent("enstream:library-changed"));
        refreshCount();
      }
    }
    setOnline(true);
  } catch (err) {
    setOnline(err?.status !== 0);
  }
  const delay = lastStatus?.running ? 1000 : 15000;
  pollTimer = setTimeout(pollScan, document.hidden && !lastStatus?.running ? 60000 : delay);
}
document.addEventListener("visibilitychange", () => { if (!document.hidden) pollScan(); });

// ------------------------------------------------------------------ footer + counts
const foot = document.getElementById("sidebar-foot");
const dot = h("span", { class: "dot" });
const footStatus = h("span", { text: "Connected" });
const footVersion = h("span");
const signOutBtn = h("button", { class: "btn ghost sm", type: "button", hidden: true, onclick: signOut }, icon("log-out", "sm"), "Sign out");
foot.append(h("div", null, dot, footStatus), footVersion, signOutBtn);
function updateSignOut() {
  signOutBtn.hidden = !pinEnabled || authRequired;
}
async function signOut() {
  try { await request("/auth/logout", { method: "POST" }); } catch { /* ignore */ }
  authRequired = true;
  updateSignOut();
  navigate("/login", { replace: true });
}

function setOnline(on) {
  dot.classList.toggle("off", !on);
  footStatus.textContent = on ? "Server online" : "Server offline";
}

export async function refreshCount() {
  try {
    const env = await request("/videos?limit=1");
    const el = document.getElementById("nav-count");
    if (el) el.textContent = env.pagination?.total ? new Intl.NumberFormat().format(env.pagination.total) : "";
  } catch { /* ignore */ }
}

export async function boot() {
  await initDesktop().catch((e) => console.error(e));
  buildNav();
  try {
    const st = await get("/auth/status");
    pinEnabled = st.pin_enabled;
    authRequired = !st.authenticated;
  } catch { /* offline: the page shows the error */ }
  updateSignOut();
  render();
  pollScan();
  refreshCount();
  try {
    const info = await get("/system/info");
    footVersion.textContent = `Version ${info.version}`;
  } catch { /* ignore */ }
}
