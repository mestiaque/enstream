import { h, icon, fmtNumber, fmtSize, fmtLongDuration, fmtAgo, swap } from "../dom.js";
import { get } from "../api.js";
import { link, errorState, loading, emptyState } from "../ui.js";
import { videoCollection } from "../components.js";
import { isHost } from "../role.js";
import { welcomed } from "./welcome.js";

function greeting() {
  const hr = new Date().getHours();
  return hr < 5 ? "Good night" : hr < 12 ? "Good morning" : hr < 18 ? "Good afternoon" : "Good evening";
}

function section(title, iconName, href, body) {
  return h("section", { class: "section" }, h("div", { class: "section-head" }, h("h2", null, icon(iconName), title),
    href ? link(href, null, "See all", icon("chevron-right", "sm")) : null), body);
}

export async function render(ctx) {
  // First start: no folders yet → the setup wizard.
  if (isHost() && !welcomed()) {
    const folders = await get("/folders").catch(() => null);
    if (folders && !folders.length) {
      queueMicrotask(() => ctx.navigate("/welcome", { replace: true }));
      return { el: h("div"), title: "Home" };
    }
  }
  const el = h("div");
  const draw = async () => {
    swap(el, loading());
    let d;
    try {
      d = await get("/dashboard");
    } catch (err) {
      swap(el, errorState(err, draw));
      return;
    }
    const s = d.stats;
    const noFolders = !s.folders;
    const summary = [`${fmtNumber(s.videos)} ${s.videos === 1 ? "video" : "videos"}`, fmtLongDuration(s.total_duration), fmtSize(s.total_size)];
    if (d.scanning) summary.push("scanning…");
    const parts = [h("div", { class: "hero" }, h("h1", { text: greeting() }), h("p", { text: noFolders ? (isHost() ? "Add a folder with your videos to start your library." : "No videos have been shared yet.") : summary.join("  ·  ") }))];
    if (noFolders) {
      parts.push(h("div", { class: "section" }, isHost()
        ? emptyState({ iconName: "folder-plus", title: "Your library is empty", text: "Pick a folder on this computer, for example Videos or Movies. Your files stay where they are; ENStream only reads them.",
          actions: [link("/folders", { class: "btn primary" }, icon("folder-plus", "sm"), "Add a folder")] })
        : emptyState({ iconName: "monitor", title: "Nothing to watch yet", text: "Videos appear here once folders are added in ENStream on the computer that shares them." })));
    } else {
      if (d.continue_watching.length) parts.push(section("Continue Watching", "circle-play", "/continue", videoCollection(d.continue_watching, { rail: true, mode: "grid", label: "Continue watching", meta: "played" })));
      if (d.recently_added.length) parts.push(section("Recently Added", "clock", "/recent", videoCollection(d.recently_added, { rail: true, mode: "grid", label: "Recently added" })));
      else parts.push(h("div", { class: "section" }, emptyState({ iconName: d.scanning ? "loader-circle" : "film", title: d.scanning ? "Scanning your folders…" : "No videos found yet", text: d.scanning ? "Videos appear here as soon as the scan finds them." : "Check that your folders contain video files, then run a scan." })));
      if (d.recently_played?.length) parts.push(section("Recently Played", "history", "/history", videoCollection(d.recently_played, { rail: true, mode: "grid", label: "Recently played", meta: "played" })));
    }
    swap(el, ...parts.filter(Boolean));
  };
  await draw();
  const onChange = () => draw();
  window.addEventListener("enstream:library-changed", onChange);
  return { el, title: "Home", destroy: () => window.removeEventListener("enstream:library-changed", onChange) };
}
