// Hover preview: resting the mouse on a video card plays short, muted pieces from across the
// whole video (10 %, 25 %, 40 %, 55 %, 70 %, 85 %), about two seconds each, like a trailer.
// It streams from the same Range-enabled endpoint as the player, so nothing is generated on
// the server. Mouse only (touch screens and TVs have no hover), one preview at a time, and it
// can be turned off in Settings → Appearance.
import { h } from "./dom.js";
import { streamURL } from "./api.js";
import { prefs } from "./store.js";

const POINTS = [0.1, 0.25, 0.4, 0.55, 0.7, 0.85];
const PIECE_SECONDS = 2.2;
const START_DELAY = 450; // ms of resting before anything loads
let active = null;

const enabled = () => prefs().hoverPreview !== false && !matchMedia("(prefers-reduced-motion: reduce)").matches;

/** Adds the preview to a card thumbnail. `v` is the video from the API. */
export function attachPreview(thumb, v) {
  // Formats the browser cannot play would only show an error; keep the thumbnail.
  if (v.compatibility === "transcode" || !v.duration || v.duration < 4) return;
  let timer = null;
  thumb.addEventListener("pointerenter", (e) => {
    if (e.pointerType !== "mouse" || !enabled()) return;
    clearTimeout(timer);
    timer = setTimeout(() => start(thumb, v), START_DELAY);
  });
  thumb.addEventListener("pointerleave", () => {
    clearTimeout(timer);
    if (active?.thumb === thumb) stop();
  });
}

function start(thumb, v) {
  stop();
  const video = h("video", { class: "hover-preview", muted: true, playsinline: true, preload: "metadata", disablepictureinpicture: true, "aria-hidden": "true" });
  video.muted = true; // the attribute alone is not enough for autoplay in every browser
  const segs = h("div", { class: "preview-segs", "aria-hidden": "true" });
  const short = v.duration < 20; // a short clip just plays from its start
  const points = short ? [0] : POINTS;
  const bars = points.map(() => h("i"));
  if (!short) segs.append(...bars);
  thumb.classList.add("previewing");
  thumb.append(video, segs);

  let index = -1;
  let pieceTimer = null;
  const state = { thumb, video, segs, stopped: false };
  active = state;

  const next = () => {
    if (state.stopped) return;
    index = (index + 1) % points.length;
    bars.forEach((b, i) => b.classList.toggle("on", i === index));
    const d = video.duration || v.duration;
    const at = points[index] * d;
    if (Math.abs(video.currentTime - at) > 0.5) video.currentTime = at;
    video.play().catch(() => {});
    clearTimeout(pieceTimer);
    if (!short) pieceTimer = setTimeout(next, PIECE_SECONDS * 1000);
  };
  video.addEventListener("loadedmetadata", next, { once: true });
  video.addEventListener("playing", () => video.classList.add("on"));
  video.addEventListener("error", () => stop());
  state.cleanup = () => clearTimeout(pieceTimer);
  video.src = streamURL(v.id);
}

export function stop() {
  const s = active;
  if (!s) return;
  active = null;
  s.stopped = true;
  s.cleanup?.();
  s.thumb.classList.remove("previewing");
  s.video.pause();
  // Dropping the source ends the download at once.
  s.video.removeAttribute("src");
  s.video.load();
  s.video.remove();
  s.segs.remove();
}

// Leaving the page or switching tabs ends a running preview.
window.addEventListener("enstream:navigate", stop);
document.addEventListener("visibilitychange", () => { if (document.hidden) stop(); });
