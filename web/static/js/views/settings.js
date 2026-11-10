import { h, icon, fmtAgo, fmtDuration, swap } from "../dom.js";
import { get, send } from "../api.js";
import { pageHead, errorState, loading, toast, toastError, copyText, confirmDialog } from "../ui.js";
import { prefs, setPref } from "../store.js";

const SHORTCUTS = [
  ["Play / pause", "Space"], ["Back / forward 5 s", "← →"], ["Back / forward 10 s", "J L"], ["Volume", "↑ ↓"], ["Mute", "M"], ["Full screen", "F"],
  ["Subtitles", "C"], ["Speed", "< >"], ["Next / previous video", "N P"], ["Jump to 0–90 %", "0–9"], ["Search", "/"], ["Leave the player", "Esc"],
];

function deviceIcon(ua) {
  return /Mobi|Android|iPhone|iPad/i.test(ua || "") ? "smartphone" : "monitor";
}

function describeUA(ua) {
  if (!ua) return "Unknown browser";
  const browser = /Edg\//.test(ua) ? "Edge" : /OPR\//.test(ua) ? "Opera" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome" : /Safari\//.test(ua) ? "Safari" : "Browser";
  const os = /Windows/.test(ua) ? "Windows" : /Android/.test(ua) ? "Android" : /iPhone|iPad|iPod/.test(ua) ? "iOS" : /Mac OS X/.test(ua) ? "macOS" : /CrOS/.test(ua) ? "ChromeOS" : /Linux/.test(ua) ? "Linux" : "";
  return os ? `${browser} on ${os}` : browser;
}

function panel(title, iconName, body, cls = "") {
  return h("section", { class: `panel ${cls}` }, h("div", { class: "panel-head" }, h("h2", { class: "scan-title" }, icon(iconName), title)), h("div", { class: "panel-body" }, body));
}

function toolRow(name, t) {
  return [h("dt", { text: name }), h("dd", null, t?.available ? h("span", { class: "badge ok" }, icon("check", "sm"), t.version || "Available")
    : h("span", { class: "badge bad", title: t?.message || "", text: "Not found" }))];
}

export async function render() {
  const el = h("div", null, pageHead("Settings", "This device, the server and how to reach it."), loading());
  let info, devices;
  try {
    [info, devices] = await Promise.all([get("/system/info"), get("/devices")]);
  } catch (err) {
    swap(el, pageHead("Settings"), errorState(err, () => location.reload()));
    return { el, title: "Settings" };
  }

  // ------------------------------------------------------------------ this device
  const me = devices.find((d) => d.current);
  const nameInput = h("input", { class: "input", id: "device-name", maxlength: "60", placeholder: describeUA(navigator.userAgent), autocomplete: "off" });
  nameInput.value = me?.name || "";
  const nameForm = h("form", { class: "inline-form" }, nameInput, h("button", { class: "btn primary", type: "submit", text: "Save" }));
  nameForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await send("PUT", "/devices/current", { name: nameInput.value.trim() });
      toast("Device name saved", { type: "success" });
      drawDevices(await get("/devices"));
    } catch (err) { toastError(err); }
  });
  const autoNext = h("input", { type: "checkbox", id: "pref-autonext" });
  autoNext.checked = prefs().autoNext;
  autoNext.addEventListener("change", () => setPref("autoNext", autoNext.checked));
  const thisDevice = panel("This device", deviceIcon(navigator.userAgent), [
    h("div", { class: "field" }, h("label", { for: "device-name", text: "Device name" }), nameForm, h("span", { class: "hint", text: "Shown in History so you know where a video was played." })),
    h("div", { class: "setting-row section" }, h("label", { for: "pref-autonext" }, h("strong", { text: "Play next video automatically" }), h("p", { text: "When a video ends, continue with the next one from the same list." })),
      h("span", { class: "switch" }, autoNext, h("span"))),
  ]);

  // ------------------------------------------------------------------ devices
  const devBox = h("div");
  function drawDevices(list) {
    if (!list.length) return swap(devBox, h("p", { class: "muted", text: "No devices have played anything yet." }));
    swap(devBox, ...list.map((d) => h("div", { class: "device" }, h("div", { class: "icon-wrap" }, icon(deviceIcon(d.user_agent))),
      h("div", null, h("strong", { text: d.name || describeUA(d.user_agent) }), d.current ? h("span", { class: "badge info", text: "This device" }) : null,
        h("small", { text: `${describeUA(d.user_agent)} · last seen ${fmtAgo(d.last_seen_at)}` })),
      d.current ? null : h("button", { class: "icon-btn", type: "button", "aria-label": `Forget ${d.name || "device"}`, title: "Forget device", onclick: async () => {
        const ok = await confirmDialog({ title: "Forget this device?", message: "Its name is removed. Playback positions stay in the library.", confirm: "Forget", danger: true });
        if (!ok) return;
        try { await send("DELETE", `/devices/${d.id}`); drawDevices(await get("/devices")); } catch (err) { toastError(err); }
      } }, icon("trash-2")))));
  }
  drawDevices(devices);

  // ------------------------------------------------------------------ network
  const addrs = (info.addresses || []).filter((a) => a.private);
  const network = panel("Open on other devices", "wifi", [
    h("p", { class: "muted", text: "Phones, tablets, smart TVs and computers on the same network can open ENStream in their browser." }),
    h("div", { class: "section" }, addrs.length ? addrs.map((a) => h("div", { class: "addr" }, icon("wifi", "sm"), h("a", { href: a.url, target: "_blank", rel: "noopener", text: a.url }), h("small", { class: "hide-sm", text: a.interface }),
      h("button", { class: "icon-btn", type: "button", "aria-label": `Copy ${a.url}`, onclick: () => copyText(a.url, "Address copied") }, icon("copy", "sm"))))
      : h("p", { class: "faint", text: "No network connection found. Connect this computer to Wi-Fi or Ethernet." })),
    h("p", { class: "field hint section", text: "Only devices on your local network can connect; requests from the internet are refused." }),
  ]);

  // ------------------------------------------------------------------ server
  const server = panel("Server", "server", h("dl", { class: "kv" },
    h("dt", { text: "Version" }), h("dd", { text: info.version }),
    h("dt", { text: "Running for" }), h("dd", { text: fmtDuration(info.uptime_seconds) }),
    h("dt", { text: "Port" }), h("dd", { class: "mono", text: String(info.port) }),
    h("dt", { text: "Platform" }), h("dd", { text: info.platform }),
    toolRow("FFmpeg", info.ffmpeg), toolRow("FFprobe", info.ffprobe),
    h("dt", { text: "Database" }), h("dd", { text: `Schema v${info.schema_version}` })));
  const tools = !info.ffmpeg?.available || !info.ffprobe?.available
    ? h("p", { class: "compat warn section" }, icon("info"), h("div", null, h("strong", { text: "FFmpeg is missing" }), "Videos still play, but thumbnails, durations and format checks need FFmpeg installed on the server computer.")) : null;

  // ------------------------------------------------------------------ shortcuts + about
  const shortcuts = panel("Keyboard shortcuts", "info", h("div", { class: "kbd-list" }, SHORTCUTS.map(([a, k]) => h("div", null, h("span", { text: a }), h("span", null, k.split(" ").map((x) => h("kbd", { text: x })))))));
  const about = panel("About", "clapperboard", [
    h("div", { class: "about" }, h("img", { src: "/static/assets/enstream-icon.svg", alt: "" }), h("div", null, h("h2", { text: "ENStream" }), h("p", { class: "muted", text: `Your Personal Cinema · version ${info.version}` }),
      h("p", { class: "faint", text: "Stream the videos on this computer to every screen at home. No cloud, no account, no tracking." }))),
    h("dl", { class: "kv section" }, h("dt", { text: "Family" }), h("dd", { text: "ENViewer · ENSave · ENPlayer · ENPost · ENStream" }),
      h("dt", { text: "Built with" }), h("dd", { text: `Go (${info.go_version}), SQLite, FFmpeg` }),
      h("dt", { text: "Icons" }), h("dd", { text: "Lucide (ISC license)" }),
      h("dt", { text: "License" }), h("dd", { text: "MIT" })),
  ]);

  swap(el, pageHead("Settings", "This device, the server and how to reach it."),
    h("div", { class: "settings" }, thisDevice, panel("Devices", "monitor", devBox), network, h("div", null, server, tools), shortcuts, about));
  return { el, title: "Settings" };
}
