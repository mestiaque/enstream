import { h, icon, append, fmtDuration, fmtSize, fmtAgo, resolutionLabel } from "./dom.js";
import { thumbURL } from "./api.js";
import { setQueue, prefs } from "./store.js";
import { link } from "./ui.js";

export function thumbnail(v, cls = "card-thumb") {
  const box = h("div", { class: cls });
  const fallback = h("div", { class: "thumb-fallback" }, icon("film", "lg"));
  if (v.has_thumbnail) {
    const img = h("img", { src: thumbURL(v), alt: "", loading: "lazy", decoding: "async" });
    img.addEventListener("load", () => { img.classList.add("loaded"); fallback.remove(); });
    img.addEventListener("error", () => img.remove());
    box.append(fallback, img);
  } else {
    box.append(fallback);
  }
  return box;
}

function progressBar(v) {
  if (!v.position || v.completed || !v.duration) return null;
  const bar = h("i");
  bar.style.width = `${Math.min(100, (v.position / v.duration) * 100).toFixed(1)}%`;
  return h("div", { class: "progress", "aria-hidden": "true" }, bar);
}

function badges(v) {
  const res = resolutionLabel(v.width, v.height);
  return h("div", { class: "card-badges" }, res ? h("span", { class: `tag ${res === "4K" ? "hi" : ""}`, text: res }) : null);
}

// meta: "folder" | "added" | "played" | "size"
function metaLine(v, meta) {
  if (meta === "played" && v.last_played_at) return `${v.completed ? "Watched" : `${fmtDuration(v.position)} of ${fmtDuration(v.duration)}`} · ${fmtAgo(v.last_played_at)}`;
  if (meta === "added") return `Added ${fmtAgo(v.created_at)} · ${fmtSize(v.file_size)}`;
  return `${v.folder_name} · ${fmtSize(v.file_size)}`;
}

// Opening a card remembers the surrounding list as the player's queue.
function remember(list, label) {
  return () => setQueue(list, label);
}

export function videoCard(v, { list = [v], label = "", meta = "folder" } = {}) {
  const thumb = thumbnail(v);
  append(thumb, [badges(v), v.duration ? h("span", { class: "duration", text: fmtDuration(v.duration) }) : null,
    v.completed ? h("span", { class: "watched-mark", title: "Watched" }, icon("check")) : null, progressBar(v),
    h("div", { class: "card-overlay" }, link(`/watch/${v.id}`, { class: "play-fab", "aria-label": `Play ${v.title}`, onclick: remember(list, label) }, icon("play", "lg")))]);
  return h("article", { class: `card ${!v.last_played_at ? "unwatched" : ""}` },
    link(`/video/${v.id}`, { "aria-hidden": "true", tabindex: "-1", onclick: remember(list, label) }, thumb),
    h("div", { class: "card-body" }, link(`/video/${v.id}`, { class: "card-title", title: v.title, onclick: remember(list, label) }, v.title),
      h("div", { class: "card-meta", text: metaLine(v, meta) })));
}

export function videoRow(v, { list = [v], label = "", meta = "folder", side } = {}) {
  const thumb = thumbnail(v);
  append(thumb, [v.duration ? h("span", { class: "duration", text: fmtDuration(v.duration) }) : null, progressBar(v)]);
  const res = resolutionLabel(v.width, v.height);
  return h("div", { class: "row" },
    link(`/video/${v.id}`, { "aria-hidden": "true", tabindex: "-1", onclick: remember(list, label) }, thumb),
    h("div", { class: "row-main" }, link(`/video/${v.id}`, { class: "card-title", title: v.title, onclick: remember(list, label) }, v.title),
      h("div", { class: "row-meta" }, h("span", { text: metaLine(v, meta) }), res ? h("span", { text: res }) : null,
        v.video_codec ? h("span", { text: v.video_codec.toUpperCase() }) : null, h("span", { class: "hide-sm", text: v.relative_path }))),
    h("div", { class: "row-side" }, side || [v.completed ? h("span", { class: "badge ok" }, icon("check", "sm"), "Watched") : null,
      link(`/watch/${v.id}`, { class: "icon-btn", "aria-label": `Play ${v.title}`, onclick: remember(list, label) }, icon("play"))]));
}

export function videoCollection(videos, opts = {}) {
  const mode = opts.mode || prefs().view;
  const o = { ...opts, list: videos };
  if (mode === "list") return h("div", { class: "list" }, videos.map((v) => videoRow(v, o)));
  return h("div", { class: opts.rail ? "rail" : "grid" }, videos.map((v) => videoCard(v, o)));
}

export function pagination(p, go) {
  if (!p || p.total_pages <= 1) return null;
  const btn = (n, label = String(n), aria) => h("button", { class: `btn sm page ${n === p.page ? "current" : ""}`, type: "button", "aria-label": aria || `Page ${n}`, "aria-current": n === p.page ? "page" : null, onclick: () => go(n) }, label);
  const pages = new Set([1, p.total_pages, p.page - 1, p.page, p.page + 1].filter((n) => n >= 1 && n <= p.total_pages));
  const sorted = [...pages].sort((a, b) => a - b);
  const out = [h("button", { class: "btn sm", type: "button", disabled: p.page <= 1, "aria-label": "Previous page", onclick: () => go(p.page - 1) }, icon("chevron-left", "sm"))];
  sorted.forEach((n, i) => {
    if (i && n - sorted[i - 1] > 1) out.push(h("span", { class: "gap", text: "…" }));
    out.push(btn(n));
  });
  out.push(h("button", { class: "btn sm", type: "button", disabled: p.page >= p.total_pages, "aria-label": "Next page", onclick: () => go(p.page + 1) }, icon("chevron-right", "sm")));
  return h("nav", { class: "pagination", "aria-label": "Pages" }, out);
}

export function skeletonGrid(n = 12) {
  return h("div", { class: "grid" }, Array.from({ length: n }, () => h("div", { class: "card" }, h("div", { class: "card-thumb skeleton" }),
    h("div", { class: "skeleton line" }))));
}
