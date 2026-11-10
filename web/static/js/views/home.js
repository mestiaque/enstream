import { h, icon, fmtNumber, fmtSize, fmtLongDuration, fmtAgo, swap } from "../dom.js";
import { get } from "../api.js";
import { link, errorState, loading, emptyState, copyText } from "../ui.js";
import { videoCollection } from "../components.js";
import { welcomed } from "./welcome.js";

function greeting() {
  const hr = new Date().getHours();
  return hr < 5 ? "Good night" : hr < 12 ? "Good morning" : hr < 18 ? "Good afternoon" : "Good evening";
}

function stat(iconName, value, label) {
  return h("div", { class: "stat" }, h("div", { class: "stat-icon" }, icon(iconName)), h("div", null, h("strong", { text: value }), h("span", { text: label })));
}

function section(title, iconName, href, body) {
  return h("section", { class: "section" }, h("div", { class: "section-head" }, h("h2", null, icon(iconName), title),
    href ? link(href, null, "See all", icon("chevron-right", "sm")) : null), body);
}

function addresses(info) {
  const list = (info.addresses || []).filter((a) => a.private);
  if (!list.length) return null;
  return h("section", { class: "section" }, h("div", { class: "section-head" }, h("h2", null, icon("wifi"), "Watch on another device")),
    h("div", { class: "panel panel-body" }, h("p", { class: "muted", text: "Open one of these addresses in a browser on your phone, TV or another computer on the same Wi-Fi." }),
      h("div", { class: "section" }, list.map((a) => h("div", { class: "addr" }, icon("monitor", "sm"), h("a", { href: a.url, text: a.url, target: "_blank", rel: "noopener" }),
        h("small", { class: "hide-sm", text: a.interface }), h("button", { class: "icon-btn", type: "button", "aria-label": `Copy ${a.url}`, onclick: () => copyText(a.url, "Address copied") }, icon("copy", "sm")))))));
}

export async function render(ctx) {
  // First start: no folders yet → the setup wizard.
  if (!welcomed()) {
    const folders = await get("/folders").catch(() => null);
    if (folders && !folders.length) {
      queueMicrotask(() => ctx.navigate("/welcome", { replace: true }));
      return { el: h("div"), title: "Home" };
    }
  }
  const el = h("div");
  const draw = async () => {
    swap(el, loading());
    let d, info;
    try {
      [d, info] = await Promise.all([get("/dashboard"), get("/system/info").catch(() => ({}))]);
    } catch (err) {
      swap(el, errorState(err, draw));
      return;
    }
    const s = d.stats;
    const noFolders = !s.folders;
    const hero = h("div", { class: "hero" }, h("div", null, h("h1", { text: `${greeting()}` }),
      h("p", { text: noFolders ? "Add a folder with your videos and ENStream will build a library you can stream to any device at home." :
        `${fmtNumber(s.videos)} videos in ${s.folders} ${s.folders === 1 ? "folder" : "folders"}${d.last_scan?.completed_at ? ` · last scan ${fmtAgo(d.last_scan.completed_at)}` : ""}.` })),
      h("div", { class: "actions" }, noFolders ? link("/folders", { class: "btn grad lg" }, icon("folder-plus"), "Add a folder")
        : [d.continue_watching.length ? link(`/watch/${d.continue_watching[0].id}`, { class: "btn grad lg" }, icon("play"), "Resume") : link("/library", { class: "btn grad lg" }, icon("film"), "Browse library"),
          link("/folders", { class: "btn lg" }, icon(d.scanning ? "loader-circle" : "refresh-cw", d.scanning ? "spin" : ""), d.scanning ? "Scanning…" : "Scan")]));

    const parts = [hero, h("div", { class: "stats" }, stat("film", fmtNumber(s.videos), "Videos"), stat("hard-drive", fmtSize(s.total_size), "Library size"),
      stat("clock", fmtLongDuration(s.total_duration), "Total runtime"), stat("folder", fmtNumber(s.folders), s.folders === 1 ? "Folder" : "Folders"))];

    if (noFolders) {
      parts.push(h("div", { class: "section" }, emptyState({ iconName: "folder-plus", title: "Your library is empty", text: "Pick a folder on this computer (for example your Videos or Movies folder). Your files stay where they are; ENStream only reads them.",
        actions: [link("/folders", { class: "btn primary" }, icon("folder-plus", "sm"), "Add a folder")] })));
    } else {
      if (d.continue_watching.length) {
        parts.push(section("Continue watching", "circle-play", "/continue", videoCollection(d.continue_watching, { rail: true, mode: "grid", label: "Continue watching", meta: "played" })));
      }
      if (d.recently_added.length) parts.push(section("Recently added", "clock", "/recent", videoCollection(d.recently_added, { rail: true, mode: "grid", label: "Recently added", meta: "added" })));
      else parts.push(h("div", { class: "section" }, emptyState({ iconName: "loader-circle", title: d.scanning ? "Scanning your folders…" : "No videos found yet", text: d.scanning ? "Videos appear here as soon as the scan finds them." : "Check that your folders contain video files, then run a scan." })));
    }
    parts.push(addresses(info));
    swap(el, ...parts.filter(Boolean));
  };
  await draw();
  const onChange = () => draw();
  window.addEventListener("enstream:library-changed", onChange);
  return { el, title: "Home", destroy: () => window.removeEventListener("enstream:library-changed", onChange) };
}
