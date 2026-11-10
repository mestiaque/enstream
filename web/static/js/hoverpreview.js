// Hover preview: resting the mouse on a video card plays short, muted pieces from across the
// whole video (10 %, 25 %, 40 %, 55 %, 70 %, 85 %), about two seconds each, like a trailer.
// It streams from the same Range-enabled endpoint as the player, so nothing is generated on
// the server. Mouse only (touch screens and TVs have no hover), one preview at a time, and it
// can be turned off in Settings → Appearance.
import { h } from "./dom.js";
import { streamURL } from "./api.js";
import { prefs } from "./store.js";

const POINTS = [0.1, 0.25, 0.4, 0.55, 0.7, 0.85];
const PIECE_SECONDS = 2.5;
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

function makeVideo() {
  const v = h("video", { class: "hover-preview", muted: true, playsinline: true, preload: "auto", disablepictureinpicture: true, "aria-hidden": "true" });
  v.muted = true; // the attribute alone is not enough for autoplay in every browser
  return v;
}

// Two players take turns: while one plays a piece, the other is already sitting at the next
// point with frames decoded, so switching is a short cross-fade instead of a seek-and-wait.
function start(thumb, v) {
  stop();
  const short = v.duration < 20; // a short clip just plays from its start
  const points = short ? [0] : POINTS;
  const players = short ? [makeVideo()] : [makeVideo(), makeVideo()];
  const segs = h("div", { class: "preview-segs", "aria-hidden": "true" });
  const bars = points.map(() => h("i", null, h("b")));
  if (!short) segs.append(...bars);
  thumb.classList.add("previewing");
  thumb.append(...players, segs);

  const state = { thumb, players, segs, stopped: false, timers: [] };
  active = state;
  const later = (fn, ms) => state.timers.push(setTimeout(fn, ms));
  const at = (i, video) => points[i] * (video.duration || v.duration);
  const ready = (video) => new Promise((resolve) => {
    if (video.readyState >= 3 && !video.seeking) return resolve();
    const done = () => { video.removeEventListener("canplay", done); video.removeEventListener("seeked", check); resolve(); };
    const check = () => { if (video.readyState >= 3) done(); };
    video.addEventListener("canplay", done);
    video.addEventListener("seeked", check);
  });
  const cue = (video, i) => { if (Math.abs(video.currentTime - at(i, video)) > 0.3) video.currentTime = at(i, video); };

  let index = 0;
  let front = players[0];

  const showBar = (i) => bars.forEach((b, k) => {
    b.classList.toggle("done", k < i);
    b.classList.toggle("on", k === i);
    b.firstChild.style.transitionDuration = k === i ? `${PIECE_SECONDS}s` : "0s";
  });

  const play = async () => {
    if (state.stopped) return;
    await ready(front);
    if (state.stopped) return;
    front.play().catch(() => {});
    front.classList.add("on");
    showBar(index);
    if (short) return;
    // Get the other player to the next point while this piece plays.
    const back = players.find((p) => p !== front);
    const nextIndex = (index + 1) % points.length;
    cue(back, nextIndex);
    later(async () => {
      await ready(back); // usually long done; if not, the current piece simply plays on
      if (state.stopped) return;
      back.play().catch(() => {});
      back.style.zIndex = "1"; // the incoming piece fades in on top of the current one
      front.style.zIndex = "0";
      back.classList.add("on");
      const old = front;
      later(() => { old.classList.remove("on"); old.pause(); }, 420);
      front = back;
      index = nextIndex;
      play();
    }, PIECE_SECONDS * 1000);
  };

  players[0].addEventListener("loadedmetadata", () => { cue(players[0], 0); play(); }, { once: true });
  for (const p of players) p.addEventListener("error", () => stop());
  state.cleanup = () => state.timers.forEach(clearTimeout);
  players[0].src = streamURL(v.id);
  if (players[1]) {
    players[1].addEventListener("loadedmetadata", () => cue(players[1], 1), { once: true });
    players[1].src = streamURL(v.id);
  }
}

export function stop() {
  const s = active;
  if (!s) return;
  active = null;
  s.stopped = true;
  s.cleanup?.();
  s.thumb.classList.remove("previewing");
  for (const video of s.players) {
    video.pause();
    // Dropping the source ends the download at once.
    video.removeAttribute("src");
    video.load();
    video.remove();
  }
  s.segs.remove();
}

// Leaving the page or switching tabs ends a running preview.
window.addEventListener("enstream:navigate", stop);
document.addEventListener("visibilitychange", () => { if (document.hidden) stop(); });
