import { h, icon, fmtDuration, fmtSize, fmtDate, fmtAgo, resolutionLabel, swap } from "../dom.js";
import { get, send, streamURL, ApiError } from "../api.js";
import { errorState, emptyState, link, toast, toastError, copyText } from "../ui.js";
import { thumbnail } from "../components.js";

const COMPAT = {
  likely: ["ok", "check", "Plays in most browsers", "H.264/VP9/AV1 video in an MP4 or WebM file streams directly."],
  possibly: ["warn", "info", "May play, depending on the browser", "Some browsers (often Safari or Chrome on Windows/Android) can play this; others can't."],
  transcode: ["bad", "circle-alert", "Probably won't play in a browser", "This format usually needs conversion. Copy the stream link and open it in VLC or another player."],
  unknown: ["warn", "info", "Format not checked", "FFprobe wasn't available when this file was scanned, so ENStream couldn't check its codecs."],
};

function channels(n) {
  return { 1: "Mono", 2: "Stereo", 6: "5.1", 8: "7.1" }[n] || (n ? `${n} channels` : "");
}

function container(v) {
  const f = v.container_format || "";
  if (f.includes("mp4")) return v.extension === ".mov" ? "QuickTime (MOV)" : "MPEG-4 (MP4)";
  if (f.includes("matroska")) return v.extension === ".webm" ? "WebM" : "Matroska (MKV)";
  if (f.includes("avi")) return "AVI";
  return f ? f.split(",")[0].toUpperCase() : v.extension.replace(".", "").toUpperCase();
}

export async function render(ctx) {
  const id = Number(ctx.params[0]);
  let d;
  try {
    d = await get(`/videos/${id}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return { el: emptyState({ iconName: "ban", title: "Video not found", text: "It may have been removed from its folder or the folder was taken out of the library.", actions: [link("/library", { class: "btn primary" }, "Back to library")] }), title: "Not found" };
    }
    return { el: errorState(err, () => ctx.navigate(location.pathname, { replace: true })), title: "Error" };
  }
  const v = d.video;
  const el = h("div");

  const draw = () => {
    const res = resolutionLabel(v.width, v.height);
    const resume = d.resume_from > 0;
    const pct = v.duration ? Math.round((v.position / v.duration) * 100) : 0;
    const poster = thumbnail(v, "poster");
    if (resume) {
      const bar = h("i");
      bar.style.width = `${pct}%`;
      poster.append(h("div", { class: "progress" }, bar));
    }
    poster.append(h("div", { class: "card-overlay" }, link(`/watch/${v.id}`, { class: "play-fab", "aria-label": resume ? "Resume" : "Play" }, icon("play", "lg"))));

    const [cls, ic, title, text] = COMPAT[v.compatibility] || COMPAT.unknown;
    const chips = [
      v.duration ? h("span", { class: "badge" }, icon("clock", "sm"), fmtDuration(v.duration)) : null,
      res ? h("span", { class: "badge info", text: res }) : null,
      h("span", { class: "badge", text: v.extension.replace(".", "").toUpperCase() }),
      h("span", { class: "badge" }, icon("hard-drive", "sm"), fmtSize(v.file_size)),
      d.subtitles.length ? h("span", { class: "badge" }, icon("captions", "sm"), `${d.subtitles.length} subtitle${d.subtitles.length > 1 ? "s" : ""}`) : null,
      v.completed ? h("span", { class: "badge ok" }, icon("check", "sm"), "Watched") : resume ? h("span", { class: "badge info", text: `${pct}% watched` }) : null,
    ];

    const markWatched = async () => {
      try {
        await send("POST", `/videos/${v.id}/completed`);
        v.completed = true; v.position = 0; d.resume_from = 0; v.last_played_at = Math.floor(Date.now() / 1000);
        toast("Marked as watched", { type: "success" });
        draw();
      } catch (err) { toastError(err); }
    };
    const forget = async () => {
      try {
        await send("DELETE", `/history/${v.id}`);
        v.completed = false; v.position = 0; d.resume_from = 0; v.last_played_at = null;
        toast("Progress reset", { type: "success" });
        draw();
      } catch (err) { toastError(err); }
    };
    const actions = h("div", { class: "details-actions" },
      link(`/watch/${v.id}`, { class: "btn grad lg" }, icon("play"), resume ? `Resume from ${fmtDuration(d.resume_from)}` : "Play"),
      resume ? link(`/watch/${v.id}?t=0`, { class: "btn lg" }, icon("rotate-ccw"), "Start over") : null,
      !v.completed ? h("button", { class: "btn lg", type: "button", onclick: markWatched }, icon("check"), "Mark watched") : null,
      v.last_played_at ? h("button", { class: "btn lg ghost", type: "button", onclick: forget }, icon("x"), "Reset progress") : null,
      h("button", { class: "btn lg ghost", type: "button", title: "For VLC or another player on this network", onclick: () => copyText(new URL(streamURL(v.id), location.origin).href, "Stream link copied") }, icon("copy"), "Copy stream link"));

    const tech = [
      ["File", v.file_name],
      ["Location", `${v.folder_name} / ${v.relative_path}`],
      ["Container", container(v)],
      ["Video", [v.video_codec?.toUpperCase(), v.video_profile, v.width ? `${v.width}×${v.height}` : "", v.fps ? `${Math.round(v.fps * 100) / 100} fps` : ""].filter(Boolean).join(" · ") || "—"],
      ["Audio", [v.audio_codec?.toUpperCase(), channels(v.audio_channels)].filter(Boolean).join(" · ") || "None"],
      ["Bitrate", v.bitrate ? `${(v.bitrate / 1e6).toFixed(1)} Mbit/s` : "—"],
      ["Subtitles", d.subtitles.length ? d.subtitles.map((s) => s.label).join(", ") : "None found next to the file"],
      ["Added", fmtDate(v.created_at)],
      ["Last played", v.last_played_at ? `${fmtAgo(v.last_played_at)} (${fmtDate(v.last_played_at)})` : "Never"],
    ];

    swap(el, 
      h("button", { class: "back-link", type: "button", onclick: () => (history.length > 1 ? history.back() : ctx.navigate("/library")) }, icon("arrow-left", "sm"), "Back"),
      h("div", { class: "details" },
        h("div", null, link(`/watch/${v.id}`, { "aria-label": "Play" }, poster)),
        h("div", null,
          h("h1", { class: "details-title", text: v.title }),
          h("p", { class: "details-sub", text: v.folder_name }),
          h("div", { class: "chips" }, chips),
          actions,
          h("div", { class: `compat ${cls}` }, icon(ic), h("div", null, h("strong", { text: title }), text)))),
      h("section", { class: "section panel" }, h("div", { class: "panel-head" }, h("h2", { text: "File details" })),
        h("dl", { class: "kv panel-body" }, tech.flatMap(([k, val]) => [h("dt", { text: k }), h("dd", { text: val })]))));
  };
  draw();
  return { el, title: v.title };
}
