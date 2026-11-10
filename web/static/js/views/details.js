import { h, icon, niceTitle, fmtDuration, fmtSize, fmtDate, fmtAgo, resolutionLabel, swap } from "../dom.js";
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
    const copyLink = () => copyText(new URL(streamURL(v.id), location.origin).href, "Stream link copied");
    const actions = h("div", { class: "details-actions" },
      link(`/watch/${v.id}`, { class: "btn grad lg play-main" }, icon("play"), resume ? `Resume from ${fmtDuration(d.resume_from)}` : "Play"),
      resume ? link(`/watch/${v.id}?t=0`, { class: "btn lg" }, icon("rotate-ccw"), "Start over") : null,
      !v.completed ? h("button", { class: "btn lg", type: "button", onclick: markWatched }, icon("check"), "Mark watched") : null,
      v.last_played_at ? h("button", { class: "btn lg", type: "button", onclick: forget }, icon("x"), "Reset") : null,
      h("button", { class: "btn lg", type: "button", title: "Copy the stream link for VLC or another player on this network", onclick: copyLink }, icon("copy"), "Copy link"));

    const tech = [
      ["File", `${v.folder_name} / ${v.relative_path}`, "mono"],
      ["Video", [v.video_codec?.toUpperCase(), v.video_profile, v.width ? `${v.width}×${v.height}` : "", v.fps ? `${Math.round(v.fps * 100) / 100} fps` : ""].filter(Boolean).join(" · ") || "—"],
      ["Audio", [v.audio_codec?.toUpperCase(), channels(v.audio_channels)].filter(Boolean).join(" · ") || "None"],
      ["Format", [container(v), v.bitrate ? `${(v.bitrate / 1e6).toFixed(1)} Mbit/s` : ""].filter(Boolean).join(" · ")],
      d.subtitles.length ? ["Subtitles", d.subtitles.map((s) => s.label).join(", ")] : null,
      ["Added", fmtDate(v.created_at)],
      v.last_played_at ? ["Last played", `${fmtAgo(v.last_played_at)} (${fmtDate(v.last_played_at)})`] : null,
    ].filter(Boolean);

    // Only formats that may not play get a note; "plays in most browsers" is the normal case.
    const note = v.compatibility === "likely" ? null : h("div", { class: `compat ${cls}` }, icon(ic), h("div", null, h("strong", { text: title }), text));

    swap(el,
      h("button", { class: "back-link", type: "button", onclick: () => (history.length > 1 ? history.back() : ctx.navigate("/library")) }, icon("arrow-left", "sm"), "Back"),
      h("div", { class: "details" },
        link(`/watch/${v.id}`, { class: "details-poster", "aria-label": "Play" }, poster),
        h("div", { class: "details-side" },
          h("h1", { class: "details-title", title: v.title, text: niceTitle(v.title) }),
          h("p", { class: "details-sub" }, icon("folder", "sm"), v.folder_name),
          h("div", { class: "chips" }, chips),
          actions,
          note,
          h("dl", { class: "kv details-kv" }, tech.flatMap(([k, val, c]) => [h("dt", { text: k }), h("dd", { class: c || null, text: val })])))));
  };
  draw();
  return { el, title: niceTitle(v.title) };
}
