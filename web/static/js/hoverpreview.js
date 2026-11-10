// Hover preview: resting the mouse on a video card plays a short muted clip made of ten
// one-second pieces from across the whole video. The server makes these clips in the
// background (small 480p files, so they start at once and loop). A video without a clip yet
// (just added) plays pieces of the original file instead, and the request moves it to the
// front of the server's queue. On touch screens, pressing and holding a card starts the preview
// (a quick tap still opens the video). One preview at a time; it can be turned off in
// Settings → Appearance.
import { h } from "./dom.js";
import { streamURL } from "./api.js";
import { prefs } from "./store.js";

const POINTS = [0.1, 0.25, 0.4, 0.55, 0.7, 0.85];
const PIECE_SECONDS = 2.5;
const START_DELAY = 250; // ms of resting before anything loads
const HOLD_DELAY = 350; // ms a finger rests on a card before its preview starts
const MOVE_SLOP = 10; // px a finger may move while holding (more is a scroll)
const CLIP_PIECES = 10; // pieces in a server clip, one second each
const CLIP_MIN_DURATION = 20; // shorter videos get no clip
const previewURL = (id) => `/api/v1/videos/${id}/preview`;
let active = null;

const enabled = () => prefs().hoverPreview !== false && !matchMedia("(prefers-reduced-motion: reduce)").matches;

/** Adds the preview to a card thumbnail. `v` is the video from the API. */
export function attachPreview(thumb, v) {
  if (!v.duration || v.duration < 4) return;
  const hasClip = v.duration >= CLIP_MIN_DURATION;
  // Without a clip, formats the browser cannot play would only show an error.
  if (!hasClip && v.compatibility === "transcode") return;
  let timer = null;
  thumb.addEventListener("pointerenter", (e) => {
    if (e.pointerType !== "mouse" || !enabled()) return;
    clearTimeout(timer);
    timer = setTimeout(() => (hasClip ? startClip(thumb, v) : start(thumb, v)), START_DELAY);
  });
  thumb.addEventListener("pointerleave", (e) => {
    if (e.pointerType !== "mouse") return;
    clearTimeout(timer);
    if (active?.thumb === thumb) stop();
  });
  holdToPreview(thumb, () => (hasClip ? startClip(thumb, v) : start(thumb, v)));
}

// Touch: press and hold plays the preview, which keeps playing after the finger lifts until
// the card scrolls away, another card is touched or the page changes. Moving the finger is a
// scroll and cancels; a quick tap opens the video as before, also while its preview plays.
function holdToPreview(thumb, go) {
  let hold = null;
  let held = false;
  let x = 0;
  let y = 0;
  const cancel = () => { clearTimeout(hold); hold = null; };
  thumb.addEventListener("pointerdown", (e) => {
    if (e.pointerType !== "touch" || !enabled()) return;
    held = false;
    x = e.clientX; y = e.clientY;
    cancel();
    if (active?.thumb === thumb) return; // already playing: this touch is a tap to open
    hold = setTimeout(() => {
      hold = null;
      held = true;
      navigator.vibrate?.(8);
      go();
      untilGone(thumb);
    }, HOLD_DELAY);
  });
  thumb.addEventListener("pointermove", (e) => {
    if (hold && Math.hypot(e.clientX - x, e.clientY - y) > MOVE_SLOP) cancel();
  });
  thumb.addEventListener("pointerup", cancel);
  thumb.addEventListener("pointercancel", cancel); // the browser started scrolling
  // The press that started a preview must not also open the video or show a menu.
  thumb.addEventListener("click", (e) => {
    if (!held) return;
    held = false;
    e.preventDefault();
    e.stopPropagation();
  }, true);
  thumb.addEventListener("contextmenu", (e) => { if (held || hold) e.preventDefault(); });
}

// Ends a touch-started preview once its card is mostly scrolled away or another spot is touched.
function untilGone(thumb) {
  const done = () => {
    io.disconnect();
    document.removeEventListener("pointerdown", outside, true);
    if (active?.thumb === thumb) stop();
  };
  const io = new IntersectionObserver(([en]) => { if (en.intersectionRatio < 0.4) done(); }, { threshold: [0, 0.4] });
  const outside = (e) => { if (!thumb.contains(e.target)) done(); };
  io.observe(thumb);
  document.addEventListener("pointerdown", outside, true);
}

function makeVideo() {
  const v = h("video", { class: "hover-preview", muted: true, playsinline: true, preload: "auto", disablepictureinpicture: true, "aria-hidden": "true" });
  v.muted = true; // the attribute alone is not enough for autoplay in every browser
  return v;
}

// The server clip: one small looping file; the bars follow its playback.
function startClip(thumb, v) {
  stop();
  const video = makeVideo();
  video.loop = true;
  const segs = h("div", { class: "preview-segs clip", "aria-hidden": "true" });
  const bars = Array.from({ length: CLIP_PIECES }, () => h("i", null, h("b")));
  segs.append(...bars);
  thumb.classList.add("previewing");
  thumb.append(video, segs);
  const state = { thumb, players: [video], segs, stopped: false, frame: 0 };
  active = state;

  const tick = () => {
    if (state.stopped) return;
    const per = (video.duration || CLIP_PIECES) / CLIP_PIECES;
    const t = video.currentTime / per;
    const i = Math.min(CLIP_PIECES - 1, Math.floor(t));
    bars.forEach((b, k) => {
      b.className = k < i ? "done" : k === i ? "on" : "";
      b.firstChild.style.width = k < i ? "100%" : k === i ? `${(t - i) * 100}%` : "0";
    });
    state.frame = requestAnimationFrame(tick);
  };
  video.addEventListener("canplay", () => {
    if (state.stopped) return;
    video.play().catch(() => {});
    video.classList.add("on");
    cancelAnimationFrame(state.frame);
    tick();
  }, { once: true });
  // No clip yet (404) or it cannot be read: play pieces of the original file instead.
  video.addEventListener("error", () => {
    if (state.stopped) return;
    if (v.compatibility === "transcode") return stop();
    start(thumb, v);
  }, { once: true });
  state.cleanup = () => cancelAnimationFrame(state.frame);
  video.src = previewURL(v.id);
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
