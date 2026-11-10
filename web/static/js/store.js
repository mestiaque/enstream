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
