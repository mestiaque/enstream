// Per-browser preferences and the "up next" queue (localStorage / sessionStorage).
const PREFS_KEY = "enstream.prefs";
const QUEUE_KEY = "enstream.queue";
const defaults = { view: "grid", volume: 1, muted: false, speed: 1, autoNext: true, subtitleLang: "" };

let cache = null;
export function prefs() {
  if (cache) return cache;
  try {
    cache = { ...defaults, ...JSON.parse(localStorage.getItem(PREFS_KEY) || "{}") };
  } catch {
    cache = { ...defaults };
  }
  return cache;
}

export function setPref(key, value) {
  prefs()[key] = value;
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(cache)); } catch { /* private mode */ }
}

// The list a video was opened from, so the player can offer previous/next.
export function setQueue(videos, label = "") {
  try {
    sessionStorage.setItem(QUEUE_KEY, JSON.stringify({ label, items: videos.slice(0, 200).map((v) => ({ id: v.id, title: v.title, updated_at: v.updated_at, has_thumbnail: v.has_thumbnail })) }));
  } catch { /* ignore */ }
}

export function queueAround(id) {
  try {
    const q = JSON.parse(sessionStorage.getItem(QUEUE_KEY) || "null");
    const i = q?.items?.findIndex((v) => v.id === id) ?? -1;
    if (i < 0) return { prev: null, next: null, label: "" };
    return { prev: q.items[i - 1] || null, next: q.items[i + 1] || null, label: q.label };
  } catch {
    return { prev: null, next: null, label: "" };
  }
}

// Theme (dark / light / glass / system) and accent colour, as in the sister apps.
export const ACCENTS = ["#3b82f6", "#6366f1", "#8b5cf6", "#ec4899", "#ef4444", "#f97316", "#eab308", "#22c55e", "#14b8a6", "#06b6d4"];
const mq = window.matchMedia ? matchMedia("(prefers-color-scheme: light)") : null;
export function applyAppearance() {
  const p = prefs();
  const theme = p.theme || "dark";
  const dark = theme === "system" ? !(mq && mq.matches) : theme !== "light";
  const c = document.documentElement.classList;
  c.remove("dark", "light", "glass");
  c.add(dark ? "dark" : "light");
  if (theme === "glass") c.add("glass");
  const hex = /^#[0-9a-f]{6}$/i.test(p.accent || "") ? p.accent : ACCENTS[0];
  const n = parseInt(hex.slice(1), 16);
  document.documentElement.style.setProperty("--accent", `${n >> 16} ${(n >> 8) & 255} ${n & 255}`);
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? "#131315" : "#f4f4f6");
}
mq?.addEventListener?.("change", () => { if (prefs().theme === "system") applyAppearance(); });
