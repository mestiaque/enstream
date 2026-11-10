import { h, icon, setIcon, fmtDuration, isTyping, swap } from "../dom.js";
import { get, send, beacon, streamURL, thumbURL, ApiError } from "../api.js";
import { emptyState, link, copyText, toast } from "../ui.js";
import { prefs, setPref, queueAround } from "../store.js";

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];
const SAVE_EVERY = 10_000; // ms between progress reports while playing
const IDLE_AFTER = 2800;
const NEXT_COUNTDOWN = 8;

export async function render(ctx) {
  const id = Number(ctx.params[0]);
  let d;
  try {
    d = await get(`/videos/${id}`);
  } catch (err) {
    const missing = err instanceof ApiError && err.status === 404;
    return { el: emptyState({ iconName: missing ? "ban" : "circle-alert", title: missing ? "Video not found" : "Couldn't open this video", text: err.message, actions: [link("/library", { class: "btn primary" }, "Back to library")] }), title: "Player" };
  }
  document.body.classList.add("watching");
  const v = d.video;
  const q = queueAround(id);
  const pref = prefs();
  const tParam = ctx.query.get("t");
  const startAt = tParam !== null && /^\d+(\.\d+)?$/.test(tParam) ? Number(tParam) : d.resume_from || 0;

  // ------------------------------------------------------------------ elements
  const video = h("video", { preload: "metadata", playsinline: "", "webkit-playsinline": "", src: streamURL(id) });
  video.playsInline = true;
  if (v.has_thumbnail) video.poster = thumbURL(v);
  for (const s of d.subtitles) {
    video.append(h("track", { kind: "subtitles", src: `/api/v1/videos/${id}/subtitles/${s.id}`, srclang: s.language || "und", label: s.label }));
  }
  video.volume = pref.volume;
  video.muted = pref.muted;

  const btn = (name, label, onclick, cls = "") => {
    const b = h("button", { class: `p-btn ${cls}`, type: "button", "aria-label": label, title: label, onclick: (e) => { e.stopPropagation(); onclick(); } }, icon(name));
    return b;
  };
  const playBtn = btn("play", "Play (k)", () => toggle(), "big");
  const prevBtn = btn("skip-back", q.prev ? `Previous: ${q.prev.title}` : "No previous video", () => go(q.prev));
  const nextBtn = btn("skip-forward", q.next ? `Next: ${q.next.title} (n)` : "No next video", () => go(q.next));
  prevBtn.disabled = !q.prev;
  nextBtn.disabled = !q.next;
  const backBtn = btn("rewind", "Back 10 s (j)", () => seekBy(-10), "hide-sm");
  const fwdBtn = btn("fast-forward", "Forward 10 s (l)", () => seekBy(10), "hide-sm");
  const muteBtn = btn("volume-2", "Mute (m)", () => setMuted(!video.muted));
  const volume = h("input", { type: "range", min: "0", max: "1", step: "0.05", "aria-label": "Volume" });
  volume.value = String(video.muted ? 0 : video.volume);
  volume.addEventListener("input", () => { video.volume = Number(volume.value); setMuted(video.volume === 0); });
  volume.addEventListener("click", (e) => e.stopPropagation());
  const time = h("span", { class: "p-time", text: "0:00 / " + fmtDuration(v.duration) });
  const ccBtn = btn("captions", "Subtitles (c)", () => openMenu("cc"));
  ccBtn.hidden = !d.subtitles.length;
  const speedBtn = btn("gauge", "Playback speed", () => openMenu("speed"));
  const pipBtn = btn("picture-in-picture-2", "Picture in picture", () => togglePiP(), "hide-sm");
  pipBtn.hidden = !(document.pictureInPictureEnabled && !video.disablePictureInPicture);
  const fsBtn = btn("maximize", "Full screen (f)", () => toggleFullscreen());

  const fill = h("div", { class: "timeline-fill" });
  const buffer = h("div", { class: "timeline-buffer" });
  const knob = h("div", { class: "timeline-knob" });
  const tip = h("div", { class: "timeline-tip" });
  const timeline = h("div", { class: "timeline", role: "slider", tabindex: "0", "aria-label": "Seek", "aria-valuemin": "0" }, h("div", { class: "timeline-track" }, buffer, fill, knob), tip);

  const center = h("div", { class: "p-center" }, icon("play"));
  const flash = h("div", { class: "p-flash" });
  const spinner = h("div", { class: "p-spinner", hidden: true }, icon("loader-circle", "spin"));
  const menuBox = h("div", { class: "p-menu", hidden: true, onclick: (e) => e.stopPropagation() });
  const overlay = h("div", { class: "p-overlay", hidden: true, onclick: (e) => e.stopPropagation() });

  const top = h("div", { class: "p-top" }, btn("arrow-left", "Back (Esc)", () => leave()),
    h("div", { class: "p-title" }, h("strong", { text: v.title }), h("small", { text: q.label && (q.prev || q.next) ? `${v.folder_name} · ${q.label}` : v.folder_name })));
  const bottom = h("div", { class: "p-bottom", onclick: (e) => e.stopPropagation() }, timeline,
    h("div", { class: "p-row" }, prevBtn, playBtn, nextBtn, backBtn, fwdBtn, h("div", { class: "volume" }, muteBtn, volume), time, h("div", { class: "spacer" }), ccBtn, speedBtn, pipBtn, fsBtn));
  const root = h("div", { class: "player", tabindex: "-1" }, video, center, flash, spinner, top, bottom, menuBox, overlay);

  // ------------------------------------------------------------------ helpers
  const cleanups = [];
  const on = (target, ev, fn, opts) => { target.addEventListener(ev, fn, opts); cleanups.push(() => target.removeEventListener(ev, fn, opts)); };
  const duration = () => (Number.isFinite(video.duration) && video.duration > 0 ? video.duration : v.duration || 0);

  function toggle() {
    if (!overlay.hidden) return;
    if (video.paused || video.ended) video.play().catch(() => {});
    else video.pause();
  }
  function seekTo(t) {
    const dur = duration();
    video.currentTime = Math.max(0, dur ? Math.min(t, dur - 0.25) : t);
    showControls();
  }
  function seekBy(s) {
    seekTo(video.currentTime + s);
    showFlash(`${s > 0 ? "+" : "−"}${Math.abs(s)} s`);
  }
  function setMuted(m) {
    video.muted = m;
    if (!m && video.volume === 0) video.volume = 0.5;
    setPref("muted", m);
    setPref("volume", video.volume);
  }
  let flashTimer;
  function showFlash(text) {
    flash.textContent = text;
    flash.classList.add("show");
    clearTimeout(flashTimer);
    flashTimer = setTimeout(() => flash.classList.remove("show"), 700);
  }
  function pulse(name) {
    setIcon(center.firstChild, name);
    center.classList.add("show");
    setTimeout(() => center.classList.remove("show"), 380);
  }

  // Controls hide while playing and the pointer is still.
  let idleTimer;
  function showControls() {
    root.classList.remove("idle");
    clearTimeout(idleTimer);
    idleTimer = setTimeout(() => {
      if (!video.paused && menuBox.hidden && overlay.hidden && !timeline.classList.contains("dragging")) root.classList.add("idle");
    }, IDLE_AFTER);
  }

  // ------------------------------------------------------------------ progress
  let lastSaved = -1;
  let completedSent = false;
  function payload() {
    return { position: Math.round(video.currentTime * 10) / 10, duration: Math.round(duration() * 10) / 10 };
  }
  function shouldSave(force) {
    const p = video.currentTime;
    if (!duration() || p < 5) return false; // a few seconds of browsing isn't "watching"
    return force || Math.abs(p - lastSaved) >= 2;
  }
  async function save(force = false) {
    if (!shouldSave(force)) return;
    const body = payload();
    lastSaved = body.position;
    try {
      const r = await send("PUT", `/videos/${id}/progress`, body);
      if (r.data?.completed) completedSent = true;
    } catch { /* offline: the next report catches up */ }
  }
  function saveOnExit() {
    if (shouldSave(true)) {
      lastSaved = video.currentTime;
      beacon(`/videos/${id}/progress`, payload());
    }
  }
  const saveTimer = setInterval(() => { if (!video.paused) save(); }, SAVE_EVERY);
  cleanups.push(() => clearInterval(saveTimer));
  let seekSave;

  // ------------------------------------------------------------------ video events
  let resumed = false;
  on(video, "loadedmetadata", () => {
    if (!resumed && startAt > 0 && startAt < duration() - 3) {
      video.currentTime = startAt;
      showFlash(`Resumed at ${fmtDuration(startAt)}`);
    }
    resumed = true;
    timeline.setAttribute("aria-valuemax", String(Math.round(duration())));
    applySubtitle(pref.subtitleLang, true);
    updateTime();
  });
  on(video, "loadeddata", () => {
    if (v.width && video.videoWidth === 0) toast("This browser can play the sound but not the picture of this file", { type: "error", detail: "Copy the stream link from the details page and open it in VLC.", timeout: 9000 });
  });
  on(video, "play", () => { setIcon(playBtn.firstChild, "pause"); playBtn.setAttribute("aria-label", "Pause (k)"); playBtn.title = "Pause (k)"; showControls(); });
  on(video, "pause", () => { setIcon(playBtn.firstChild, "play"); playBtn.setAttribute("aria-label", "Play (k)"); playBtn.title = "Play (k)"; showControls(); if (!video.ended) save(true); });
  on(video, "timeupdate", updateTime);
  on(video, "progress", updateBuffer);
  on(video, "waiting", () => { spinner.hidden = false; });
  on(video, "playing", () => { spinner.hidden = true; overlay.hidden = true; });
  on(video, "canplay", () => { spinner.hidden = true; });
  on(video, "seeked", () => { clearTimeout(seekSave); seekSave = setTimeout(() => save(true), 1500); });
  on(video, "volumechange", () => {
    setIcon(muteBtn.firstChild, video.muted || video.volume === 0 ? "volume-x" : "volume-2");
    volume.value = String(video.muted ? 0 : video.volume);
    setPref("volume", video.volume);
  });
  on(video, "ratechange", () => setPref("speed", video.playbackRate));
  on(video, "ended", ended);
  on(video, "error", () => showError(video.error));
  video.playbackRate = pref.speed || 1;
  video.defaultPlaybackRate = pref.speed || 1;

  function updateTime() {
    const dur = duration();
    const t = video.currentTime;
    if (!timeline.classList.contains("dragging")) {
      const pct = dur ? (t / dur) * 100 : 0;
      fill.style.width = `${pct}%`;
      knob.style.left = `${pct}%`;
    }
    time.textContent = `${fmtDuration(t)} / ${fmtDuration(dur)}`;
    timeline.setAttribute("aria-valuenow", String(Math.round(t)));
    timeline.setAttribute("aria-valuetext", `${fmtDuration(t)} of ${fmtDuration(dur)}`);
  }
  function updateBuffer() {
    const dur = duration();
    const b = video.buffered;
    for (let i = 0; i < b.length; i++) {
      if (b.start(i) <= video.currentTime + 0.5 && b.end(i) >= video.currentTime) {
        buffer.style.width = `${dur ? (b.end(i) / dur) * 100 : 0}%`;
        return;
      }
    }
  }

  // ------------------------------------------------------------------ timeline
  const ratioAt = (x) => {
    const r = timeline.getBoundingClientRect();
    return Math.min(1, Math.max(0, (x - r.left) / r.width));
  };
  on(timeline, "pointermove", (e) => {
    const r = ratioAt(e.clientX);
    tip.textContent = fmtDuration(r * duration());
    tip.style.left = `${r * 100}%`;
    if (timeline.classList.contains("dragging")) {
      fill.style.width = `${r * 100}%`;
      knob.style.left = `${r * 100}%`;
    }
  });
  on(timeline, "pointerdown", (e) => {
    e.stopPropagation();
    timeline.setPointerCapture(e.pointerId);
    timeline.classList.add("dragging");
    const r = ratioAt(e.clientX);
    fill.style.width = `${r * 100}%`;
    knob.style.left = `${r * 100}%`;
  });
  const endDrag = (e) => {
    if (!timeline.classList.contains("dragging")) return;
    timeline.classList.remove("dragging");
    seekTo(ratioAt(e.clientX) * duration());
  };
  on(timeline, "pointerup", endDrag);
  on(timeline, "pointercancel", () => timeline.classList.remove("dragging"));
  on(timeline, "click", (e) => e.stopPropagation());

  // ------------------------------------------------------------------ menus
  function openMenu(kind) {
    if (!menuBox.hidden && menuBox.dataset.kind === kind) return closeMenu();
    menuBox.dataset.kind = kind;
    const item = (label, selected, pick) => h("button", { class: selected ? "sel" : "", type: "button", role: "menuitemradio", "aria-checked": String(selected), onclick: () => { pick(); closeMenu(); } }, h("span", { text: label }), icon("check"));
    if (kind === "speed") {
      swap(menuBox, h("h4", { text: "Speed" }), ...SPEEDS.map((s) => item(s === 1 ? "Normal" : `${s}×`, video.playbackRate === s, () => { video.playbackRate = s; showFlash(`${s}×`); })));
    } else {
      const tracks = [...video.textTracks];
      const active = tracks.findIndex((t) => t.mode === "showing");
      swap(menuBox, h("h4", { text: "Subtitles" }), item("Off", active < 0, () => applySubtitle("")),
        ...tracks.map((t, i) => item(t.label || t.language || `Track ${i + 1}`, i === active, () => applySubtitle(t.language || t.label))));
    }
    menuBox.hidden = false;
    menuBox.setAttribute("role", "menu");
    menuBox.querySelector("button")?.focus({ preventScroll: true });
    showControls();
  }
  function closeMenu() {
    menuBox.hidden = true;
    showControls();
  }
  // lang "" = off. initial = restoring the saved choice silently.
  function applySubtitle(lang, initial = false) {
    const tracks = [...video.textTracks];
    let shown = null;
    for (const t of tracks) {
      const match = !!lang && !shown && (t.language === lang || t.label === lang);
      t.mode = match ? "showing" : "disabled";
      if (match) shown = t;
    }
    ccBtn.classList.toggle("on", !!shown);
    if (!initial) {
      setPref("subtitleLang", shown ? lang : "");
      showFlash(shown ? `Subtitles: ${shown.label}` : "Subtitles off");
    }
  }
  function cycleSubtitles() {
    const tracks = [...video.textTracks];
    if (!tracks.length) return;
    const i = tracks.findIndex((t) => t.mode === "showing");
    const next = tracks[i + 1];
    applySubtitle(next ? next.language || next.label : "");
  }

  // ------------------------------------------------------------------ fullscreen / PiP
  const fsElement = () => document.fullscreenElement || document.webkitFullscreenElement;
  async function toggleFullscreen() {
    try {
      if (fsElement()) {
        await (document.exitFullscreen || document.webkitExitFullscreen).call(document);
      } else if (root.requestFullscreen) {
        await root.requestFullscreen({ navigationUI: "hide" });
        screen.orientation?.lock?.("landscape").catch(() => {});
      } else if (root.webkitRequestFullscreen) {
        root.webkitRequestFullscreen();
      } else if (video.webkitEnterFullscreen) {
        video.webkitEnterFullscreen(); // iPhone
      }
    } catch { /* not allowed */ }
  }
  const onFs = () => {
    const fs = !!fsElement();
    setIcon(fsBtn.firstChild, fs ? "minimize" : "maximize");
    fsBtn.setAttribute("aria-label", fs ? "Exit full screen (f)" : "Full screen (f)");
  };
  on(document, "fullscreenchange", onFs);
  on(document, "webkitfullscreenchange", onFs);
  async function togglePiP() {
    try {
      if (document.pictureInPictureElement) await document.exitPictureInPicture();
      else await video.requestPictureInPicture();
    } catch { toast("Picture in picture isn't available here"); }
  }

  // ------------------------------------------------------------------ end / next
  let countdown;
  function ended() {
    clearInterval(countdown);
    if (!completedSent) {
      completedSent = true;
      send("POST", `/videos/${id}/completed`).catch(() => {});
    }
    lastSaved = video.currentTime;
    root.classList.remove("idle");
    if (q.next) {
      let left = NEXT_COUNTDOWN;
      const count = h("span", { text: String(left) });
      const auto = h("input", { type: "checkbox", id: "auto-next" });
      auto.checked = pref.autoNext;
      const label = h("p", null, pref.autoNext ? ["Playing in ", count, " s"] : "Up next");
      const card = h("div", { class: "p-card" },
        h("div", { class: "next-up" }, q.next.has_thumbnail ? h("img", { src: thumbURL(q.next), alt: "" }) : h("div", { class: "thumb-fallback" }, icon("film")),
          h("div", null, h("small", { class: "faint", text: "Up next" }), h("h3", { text: q.next.title }), label)),
        h("div", { class: "actions" },
          h("button", { class: "btn", type: "button", onclick: () => { clearInterval(countdown); overlay.hidden = true; video.currentTime = 0; video.play().catch(() => {}); } }, icon("rotate-ccw", "sm"), "Replay"),
          h("button", { class: "btn grad", type: "button", onclick: () => go(q.next) }, icon("play", "sm"), "Play now")),
        h("label", { class: "setting-row", for: "auto-next" }, h("span", { text: "Play the next video automatically" }), h("span", { class: "switch" }, auto, h("span"))));
      auto.addEventListener("change", () => {
        setPref("autoNext", auto.checked);
        if (!auto.checked) { clearInterval(countdown); label.textContent = "Up next"; }
      });
      swap(overlay, card);
      overlay.hidden = false;
      if (pref.autoNext) {
        countdown = setInterval(() => {
          left -= 1;
          count.textContent = String(left);
          if (left <= 0) { clearInterval(countdown); go(q.next); }
        }, 1000);
      }
    } else {
      swap(overlay, h("div", { class: "p-card" }, icon("check", "xl"), h("h3", { text: "Finished" }), h("p", { text: v.title }),
        h("div", { class: "actions" }, h("button", { class: "btn", type: "button", onclick: () => { overlay.hidden = true; video.currentTime = 0; video.play().catch(() => {}); } }, icon("rotate-ccw", "sm"), "Watch again"),
          h("button", { class: "btn primary", type: "button", onclick: () => leave() }, icon("arrow-left", "sm"), "Back"))));
      overlay.hidden = false;
    }
    cleanups.push(() => clearInterval(countdown));
  }

  function showError(err) {
    spinner.hidden = true;
    const unsupported = err && (err.code === 3 || err.code === 4);
    const streamLink = new URL(streamURL(id), location.origin).href;
    swap(overlay, h("div", { class: "p-card" }, icon("circle-alert", "xl"),
      h("h3", { text: unsupported ? "This browser can't play this file" : "Playback stopped" }),
      h("p", { text: unsupported ? `${(v.video_codec || "This").toUpperCase()} video in ${v.extension.replace(".", "").toUpperCase()} isn't supported here. Try another browser, or open the stream link in VLC.` : "The connection to the server was interrupted." }),
      h("div", { class: "actions" },
        unsupported ? h("button", { class: "btn", type: "button", onclick: () => copyText(streamLink, "Stream link copied") }, icon("copy", "sm"), "Copy stream link")
          : h("button", { class: "btn primary", type: "button", onclick: retry }, icon("refresh-cw", "sm"), "Retry"),
        h("button", { class: "btn", type: "button", onclick: () => leave() }, icon("arrow-left", "sm"), "Back"))));
    overlay.hidden = false;
    root.classList.remove("idle");
  }
  function retry() {
    const t = video.currentTime;
    overlay.hidden = true;
    video.src = streamURL(id);
    video.addEventListener("loadedmetadata", () => { video.currentTime = t; video.play().catch(() => {}); }, { once: true });
  }

  function go(item) {
    if (!item) return;
    saveOnExit();
    ctx.navigate(`/watch/${item.id}`, { replace: true });
  }
  function leave() {
    if (fsElement()) (document.exitFullscreen || document.webkitExitFullscreen)?.call(document).catch?.(() => {});
    if (history.length > 1) history.back();
    else ctx.navigate(`/video/${id}`);
  }

  // ------------------------------------------------------------------ input
  let lastTap = 0;
  on(root, "pointermove", (e) => { if (e.pointerType === "mouse") showControls(); });
  on(root, "pointerup", (e) => {
    if (e.target !== video && e.target !== root) return;
    if (!menuBox.hidden) return closeMenu();
    if (e.pointerType === "mouse") {
      toggle();
      pulse(video.paused ? "pause" : "play");
      return;
    }
    // Touch: tap shows controls (then toggles); double-tap the sides seeks 10 s.
    const now = Date.now();
    const third = root.clientWidth / 3;
    if (now - lastTap < 300 && (e.clientX < third || e.clientX > third * 2)) {
      seekBy(e.clientX < third ? -10 : 10);
      lastTap = 0;
      return;
    }
    lastTap = now;
    if (root.classList.contains("idle")) showControls();
    else if (e.clientX >= third && e.clientX <= third * 2) toggle();
    else showControls();
  });
  on(video, "dblclick", () => toggleFullscreen());
  on(document, "keydown", (e) => {
    if (isTyping(e) || e.ctrlKey || e.metaKey || e.altKey) return;
    const k = e.key;
    if ((k === " " || k === "Enter") && e.target instanceof HTMLButtonElement) return; // let the button act
    const handled = () => { e.preventDefault(); showControls(); };
    if (k === "Escape") {
      if (!menuBox.hidden) return handled(), closeMenu();
      if (!fsElement()) return handled(), leave();
      return;
    }
    if (!overlay.hidden && k !== "n") return;
    if (k === " " || k === "k" || k === "K") { handled(); toggle(); pulse(video.paused ? "pause" : "play"); }
    else if (k === "ArrowLeft") { handled(); seekBy(-5); }
    else if (k === "ArrowRight") { handled(); seekBy(5); }
    else if (k === "j" || k === "J") { handled(); seekBy(-10); }
    else if (k === "l" || k === "L") { handled(); seekBy(10); }
    else if (k === "ArrowUp" || k === "ArrowDown") {
      if (!menuBox.hidden) return;
      handled();
      video.volume = Math.min(1, Math.max(0, Math.round((video.volume + (k === "ArrowUp" ? 0.05 : -0.05)) * 100) / 100));
      if (video.muted && k === "ArrowUp") video.muted = false;
      showFlash(`Volume ${Math.round(video.volume * 100)}%`);
    }
    else if (k === "m" || k === "M") { handled(); setMuted(!video.muted); showFlash(video.muted ? "Muted" : `Volume ${Math.round(video.volume * 100)}%`); }
    else if (k === "f" || k === "F") { handled(); toggleFullscreen(); }
    else if (k === "c" || k === "C") { handled(); cycleSubtitles(); }
    else if (k === "n" || k === "N") { handled(); go(q.next); }
    else if (k === "p" || k === "P") { handled(); go(q.prev); }
    else if (k === ">" || k === "<") {
      handled();
      const i = SPEEDS.indexOf(video.playbackRate);
      const ni = Math.min(SPEEDS.length - 1, Math.max(0, (i < 0 ? 2 : i) + (k === ">" ? 1 : -1)));
      video.playbackRate = SPEEDS[ni];
      showFlash(`${SPEEDS[ni]}×`);
    }
    else if (k === "Home") { handled(); seekTo(0); }
    else if (k === "End") { handled(); seekTo(duration()); }
    else if (/^[0-9]$/.test(k)) { handled(); seekTo((Number(k) / 10) * duration()); }
  });
  on(document, "visibilitychange", () => { if (document.hidden) saveOnExit(); });
  on(window, "pagehide", saveOnExit);

  // Lock-screen / headset controls on phones.
  if ("mediaSession" in navigator) {
    try {
      navigator.mediaSession.metadata = new MediaMetadata({ title: v.title, artist: "ENStream", album: v.folder_name, artwork: v.has_thumbnail ? [{ src: thumbURL(v), type: "image/webp" }] : [] });
      const ms = navigator.mediaSession;
      ms.setActionHandler("play", () => video.play());
      ms.setActionHandler("pause", () => video.pause());
      ms.setActionHandler("seekbackward", () => seekBy(-10));
      ms.setActionHandler("seekforward", () => seekBy(10));
      ms.setActionHandler("previoustrack", q.prev ? () => go(q.prev) : null);
      ms.setActionHandler("nexttrack", q.next ? () => go(q.next) : null);
    } catch { /* partial support */ }
  }

  setTimeout(() => {
    root.focus({ preventScroll: true });
    video.play().catch(() => { root.classList.remove("idle"); }); // autoplay may need a tap
  }, 0);
  showControls();

  return {
    el: root,
    title: v.title,
    destroy() {
      saveOnExit();
      cleanups.forEach((fn) => fn());
      clearTimeout(idleTimer);
      clearTimeout(seekSave);
      video.pause();
      video.removeAttribute("src");
      video.load();
      if (fsElement()) (document.exitFullscreen || document.webkitExitFullscreen)?.call(document)?.catch?.(() => {});
      if (document.pictureInPictureElement === video) document.exitPictureInPicture().catch(() => {});
      if ("mediaSession" in navigator) navigator.mediaSession.metadata = null;
    },
  };
}
