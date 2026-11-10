import { h, icon, clear, swap, isTyping, fmtNumber } from "./dom.js";
import { get, request } from "./api.js";
import { toast, openMenu, copyText } from "./ui.js";
import { initDesktop, desktop, openExternal } from "./desktop.js";
import { prefs, setPref, applyAppearance } from "./store.js";
import { isHost, setHost } from "./role.js";

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

const LIBRARY = [
  { href: "/", label: "Home", icon: "house", key: "home" },
  { href: "/library", label: "All Videos", short: "Library", icon: "layout-grid", key: "library", count: "videos" },
  { href: "/continue", label: "Continue Watching", short: "Continue", icon: "circle-play", key: "continue", count: "continue" },
  { href: "/recent", label: "Recently Added", icon: "clock", key: "recent" },
  { href: "/history", label: "History", icon: "history", key: "history" },
];
const BOTTOM = [["home", "/", "house", "Home"], ["library", "/library", "layout-grid", "Library"], ["continue", "/continue", "circle-play", "Continue"], ["folders", "/folders", "folder", "Folders"], ["settings", "/settings", "settings", "Settings"]];
const BOTTOM_VIEWER = [["home", "/", "house", "Home"], ["library", "/library", "layout-grid", "Library"], ["continue", "/continue", "circle-play", "Continue"], ["history", "/history", "history", "History"], ["settings", "/settings", "settings", "Settings"]];
// Pages that manage the library exist only on the host.
const HOST_PAGES = new Set(["folders", "welcome"]);

const view = document.getElementById("view");
const sidebar = document.getElementById("sidebar");
const scrim = document.getElementById("scrim");
const searchInput = document.getElementById("search");
const tbTitle = document.getElementById("tb-title");
let current = null;
let renderSeq = 0;
let authRequired = false;
let pinEnabled = false;
const counts = {};
const folderBox = h("div");

// A PIN is set and this browser has no session: show the sign-in screen.
window.addEventListener("enstream:auth-required", () => {
  if (authRequired) return;
  authRequired = true;
  render();
});
export function signedIn(next) {
  authRequired = false;
  pinEnabled = true;
  navigate(next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/login") ? next : "/", { replace: true });
  pollScan();
  refreshCount();
}
export function setPinEnabled(on) {
  pinEnabled = on;
}
export const isPinEnabled = () => pinEnabled;

const navLink = (href, label, ic, key, countId) => h("a", { class: "nav-link", href, "data-link": "", title: label, dataset: { key } }, icon(ic), h("span", { text: label }),
  countId ? h("span", { class: "count", id: `count-${countId}` }) : null);

function buildNav() {
  const nav = document.getElementById("nav");
  nav.append(
    h("div", { class: "nav-section" }, h("div", { class: "nav-label" }, h("span", { text: "Library" })), LIBRARY.map((i) => navLink(i.href, i.label, i.icon, i.key, i.count))),
    // Viewers browse by folder; adding and managing folders is for the host.
    h("div", { class: "nav-section" }, h("div", { class: "nav-label" }, h("span", { text: "Folders" }),
      isHost() ? h("a", { class: "icon-btn", href: "/folders", "data-link": "", title: "Add a folder", "aria-label": "Add a folder" }, icon("plus")) : null),
      isHost() ? navLink("/folders", "All folders", "folder-open", "folders") : null, folderBox));
  const bottom = document.getElementById("bottom-nav");
  for (const [key, href, ic, label] of isHost() ? BOTTOM : BOTTOM_VIEWER) bottom.append(h("a", { href, "data-link": "", dataset: { key } }, icon(ic), h("span", { text: label })));
  scrim.addEventListener("click", () => setMenu(false));
  document.getElementById("search-icon").append(icon("search", "sm"));
  buildTopbar();
}

async function refreshFolders() {
  try {
    const list = await get("/folders");
    swap(folderBox, list.length ? list.map((f) => navLink(`/library?folder_id=${f.id}`, f.name, f.available ? "folder" : "circle-alert", `folder:${f.id}`))
      : h("div", { class: "nav-empty", text: isHost() ? "No folders yet" : "No folders shared yet" }));
    for (const f of list) {
      const a = folderBox.querySelector(`[data-key="folder:${f.id}"]`);
      if (a && f.video_count) a.append(h("span", { class: "count", text: fmtNumber(f.video_count) }));
    }
    counts.folders = list.length;
    updateStatusLeft();
    setActive(activeKey);
  } catch { /* offline */ }
}

// ------------------------------------------------------------------ top bar
function buildTopbar() {
  const left = document.getElementById("tb-left");
  const menuBtn = h("button", { class: "icon-btn only-mobile", type: "button", "aria-label": "Menu", onclick: () => setMenu(true) }, icon("menu"));
  const toggle = h("button", { class: "icon-btn hide-sm", type: "button", "aria-label": "Toggle sidebar", title: "Toggle sidebar (Ctrl+B)", onclick: toggleSidebar }, icon("panel-left"));
  const back = h("button", { class: "icon-btn tb-nav", type: "button", "aria-label": "Back", title: "Back", onclick: () => history.back() }, icon("chevron-left"));
  const fwd = h("button", { class: "icon-btn tb-nav", type: "button", "aria-label": "Forward", title: "Forward", onclick: () => history.forward() }, icon("chevron-right"));
  left.append(menuBtn, toggle, back, fwd);

  const right = document.getElementById("tb-right");
  const scanBtn = h("button", { class: "icon-btn", type: "button", "aria-label": "Scan library", title: "Scan library", onclick: scanNow }, icon("refresh-cw"));
  const addBtn = h("a", { class: "icon-btn hide-sm", href: "/folders", "data-link": "", "aria-label": "Add folder", title: "Add folder" }, icon("folder-plus"));
  const themeBtn = h("button", { class: "icon-btn", type: "button", "aria-label": "Theme", title: "Theme" }, icon(themeIcon()));
  themeBtn.addEventListener("click", () => openMenu(themeBtn, THEMES.map(([id, label, ic]) => ({ label, icon: ic, checked: prefs().theme === id || (!prefs().theme && id === "dark"),
    onSelect: () => { setPref("theme", id); applyAppearance(); swap(themeBtn, icon(themeIcon())); } }))));
  const moreBtn = h("button", { class: "icon-btn", type: "button", "aria-label": "More", title: "More" }, icon("more-horizontal"));
  moreBtn.addEventListener("click", () => openMenu(moreBtn, moreEntries()));
  const gear = h("button", { class: "icon-btn", type: "button", "aria-label": "Settings", title: "Settings (Ctrl+,)", onclick: () => openSettingsDialog() }, icon("settings"));
  if (isHost()) right.append(scanBtn, addBtn, h("span", { class: "tb-sep hide-sm" }));
  right.append(themeBtn, moreBtn, gear);
}

const THEMES = [["dark", "Dark", "moon"], ["light", "Light", "sun"], ["glass", "Glass", "sparkles"], ["system", "System", "monitor"]];
const themeIcon = () => (THEMES.find(([id]) => id === (prefs().theme || "dark")) || THEMES[0])[2];

function moreEntries() {
  const list = [
    { label: "Keyboard shortcuts", icon: "info", onSelect: () => openSettingsDialog("about") },
    { label: "Copy address for phones", icon: "smartphone", onSelect: copyPhoneAddress },
  ];
  if (desktop.available) list.push({ label: "Open in browser", icon: "external-link", onSelect: () => openExternal(location.origin) });
  if (pinEnabled && !authRequired) list.push({ separator: true }, { label: "Sign out", icon: "log-out", onSelect: signOut });
  if (desktop.available) list.push({ separator: true }, { label: "Hide to tray", onSelect: () => desktop.win().hide() }, { label: "Quit ENStream", danger: true, onSelect: () => desktop.invoke("quit_app") });
  return list;
}

async function copyPhoneAddress() {
  try {
    const info = await get("/system/info");
    const a = (info.addresses || []).find((x) => x.private);
    if (a) copyText(a.url, "Address copied");
    else toast("No network address", { type: "error", detail: "Connect this computer to Wi-Fi or Ethernet." });
  } catch (err) { toast(err.message, { type: "error" }); }
}

async function scanNow() {
  try {
    await request("/scanner/scan", { method: "POST", body: {} });
    toast("Scanning your folders");
    pollScan();
  } catch (err) { toast(err.message, { type: "error" }); }
}

function toggleSidebar() {
  const on = !document.body.classList.contains("sidebar-collapsed");
  document.body.classList.toggle("sidebar-collapsed", on);
  setPref("sidebarCollapsed", on);
}

async function openSettingsDialog(tab) {
  const mod = await import("./views/settings.js");
  mod.openSettings(tab);
}

function setMenu(open) {
  sidebar.classList.toggle("open", open);
  scrim.hidden = !open;
}

let activeKey = "";
function setActive(key) {
  activeKey = key;
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
  window.dispatchEvent(new Event("enstream:navigate")); // e.g. ends a hover preview
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
  if (match[2] === "settings") {
    const tab = new URLSearchParams(location.search).get("tab") || "";
    history.replaceState({}, "", "/");
    await render();
    openSettingsDialog(tab);
    return;
  }
  const [re, load, key] = match;
  if (HOST_PAGES.has(key) && !isHost()) {
    history.replaceState({}, "", "/");
    return render();
  }
  const folderId = key === "library" ? new URLSearchParams(location.search).get("folder_id") : null;
  setActive(folderId ? `folder:${folderId}` : key);
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
    tbTitle.textContent = result.title || "";
    view.scrollTop = 0;
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
  } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "b" && !isTyping(e)) {
    e.preventDefault();
    toggleSidebar();
  } else if ((e.ctrlKey || e.metaKey) && e.key === ",") {
    e.preventDefault();
    openSettingsDialog();
  }
});
export const setSearchText = (q) => { searchInput.value = q || ""; };

// ------------------------------------------------------------------ status bar + scan status
// One poller for the whole app; views listen for "enstream:scan".
const statusLeft = document.getElementById("status-left");
const statusRight = document.getElementById("status-right");
const online = h("span", { class: "dot", title: "Server online" });
const scanText = h("span");
const scanIcon = h("span", { class: "status-item" });
statusRight.classList.add("status-item");
statusRight.append(scanIcon, scanText, online);
let lastStatus = null;
let pollTimer;

function updateStatusLeft() {
  const v = counts.videos ?? 0;
  const f = counts.folders ?? 0;
  statusLeft.textContent = `${fmtNumber(v)} ${v === 1 ? "video" : "videos"} · ${f} ${f === 1 ? "folder" : "folders"}`;
}

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
      swap(scanIcon, icon("loader-circle", "spin"));
      scanText.textContent = j.total_files ? `Scanning ${fmtNumber(j.processed_files + j.skipped_files)} of ${fmtNumber(j.total_files)}…` : "Scanning…";
    } else {
      swap(scanIcon);
      const j = s.job;
      scanText.textContent = j?.status === "failed" ? "Last scan failed" : j ? `Library up to date${j.new_files ? ` · ${fmtNumber(j.new_files)} new` : ""}` : "Not scanned yet";
      if (wasRunning && j) {
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

async function signOut() {
  try { await request("/auth/logout", { method: "POST" }); } catch { /* ignore */ }
  authRequired = true;
  navigate("/login", { replace: true });
}

function setOnline(on) {
  online.classList.toggle("off", !on);
  online.title = on ? "Server online" : "Server offline";
}

export async function refreshCount() {
  try {
    const [all, cont] = await Promise.all([request("/videos?limit=1"), request("/continue-watching?limit=1")]);
    counts.videos = all.pagination?.total || 0;
    const set = (id, n) => { const el = document.getElementById(`count-${id}`); if (el) el.textContent = n ? fmtNumber(n) : ""; };
    set("videos", counts.videos);
    set("continue", cont.pagination?.total || 0);
    updateStatusLeft();
  } catch { /* ignore */ }
  refreshFolders();
}
window.addEventListener("enstream:library-changed", () => refreshCount());

export async function boot() {
  if (prefs().sidebarCollapsed) document.body.classList.add("sidebar-collapsed");
  await initDesktop().catch((e) => console.error(e));
  try {
    const st = await get("/auth/status");
    pinEnabled = st.pin_enabled;
    authRequired = !st.authenticated;
    setHost(st.host);
  } catch { /* offline: the page shows the error */ }
  buildNav(); // after the role is known: viewers get a shorter menu

  render();
  pollScan();
  refreshCount();
}
