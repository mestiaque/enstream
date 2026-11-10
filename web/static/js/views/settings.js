// Settings dialog, in the same layout as ENViewer, ENSave, ENPlayer and ENPost: sections on
// the left, rows of "label + hint … control" on the right.
import { h, icon, swap, fmtAgo, fmtDuration, fmtDate, fmtSize } from "../dom.js";
import { get, send, request, query } from "../api.js";
import { loading, toast, toastError, copyText, confirmDialog, errorState } from "../ui.js";
import { prefs, setPref, applyAppearance, ACCENTS } from "../store.js";
import { setPinEnabled } from "../shell.js";
import { desktop, checkForUpdate, showUpdateDialog, openAppFolder } from "../desktop.js";
import { isHost } from "../role.js";

const SECTIONS = [
  ["general", "General"], ["appearance", "Appearance"], ["library", "Library"], ["security", "Security"],
  ["backups", "Backups"], ["network", "Network"], ["about", "About"],
];
// Phones, TVs and other computers: only what concerns this device, plus About.
const VIEWER_SECTIONS = [["general", "General"], ["appearance", "Appearance"], ["about", "About"]];
const SHORTCUTS = [
  ["Play / pause", "Space"], ["Back / forward 5 s", "← →"], ["Back / forward 10 s", "J L"], ["Volume", "↑ ↓"], ["Mute", "M"], ["Full screen", "F"],
  ["Subtitles", "C"], ["Speed", "< >"], ["Next / previous video", "N P"], ["Jump to 0–90 %", "0–9"], ["Search", "/"], ["Toggle sidebar", "Ctrl B"],
];

const BUILT_WITH = [
  ["Go", "server and streaming", "BSD-3-Clause"],
  ["SQLite (modernc.org/sqlite)", "library and history", "public domain · BSD-3-Clause"],
  ["FFmpeg", "thumbnails and video details", "GPL / LGPL, separate program"],
  ["chi", "HTTP routing", "MIT"],
  ["go-qrcode", "QR codes for phones", "MIT"],
  ["Tauri", "desktop app", "MIT / Apache-2.0"],
  ["Lucide", "icons", "ISC"],
];

const deviceIcon = (ua) => (/Mobi|Android|iPhone|iPad/i.test(ua || "") ? "smartphone" : "monitor");
function describeUA(ua) {
  if (!ua) return "Unknown browser";
  if (/^curl\//.test(ua)) return "Command line";
  const browser = /Edg\//.test(ua) ? "Edge" : /OPR\//.test(ua) ? "Opera" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome" : /Safari\//.test(ua) ? "Safari" : "Browser";
  const os = /Windows/.test(ua) ? "Windows" : /Android/.test(ua) ? "Android" : /iPhone|iPad|iPod/.test(ua) ? "iOS" : /Mac OS X/.test(ua) ? "macOS" : /CrOS/.test(ua) ? "ChromeOS" : /Tizen|Web0S|SMART-TV/.test(ua) ? "Smart TV" : /Linux/.test(ua) ? "Linux" : "";
  return os ? `${browser} on ${os}` : browser;
}

// ------------------------------------------------------------------ row helpers
let uid = 0;
const group = (title) => h("div", { class: "group", text: title });
function row(label, hint, control) {
  return h("div", { class: "setting-row" }, h("div", null, h("strong", { text: label }), hint ? (hint instanceof Node ? h("div", { class: "sub" }, hint) : h("p", { text: hint })) : null),
    control ? h("div", { class: "ctrl" }, control) : null);
}
function toggle(label, hint, checked, onchange) {
  const id = `st-${++uid}`;
  const input = h("input", { type: "checkbox", id, "aria-label": label });
  input.checked = !!checked;
  input.addEventListener("change", () => onchange(input.checked, input));
  return row(label, hint, h("span", { class: "switch" }, input, h("span")));
}
function choose(label, hint, options, value, onchange) {
  const sel = h("select", { class: "select", "aria-label": label }, options.map(([v, t]) => h("option", { value: String(v), text: t })));
  sel.value = String(value);
  sel.addEventListener("change", () => onchange(sel.value, sel));
  return row(label, hint, sel);
}
function tokenEditor(values, { placeholder, label, onchange }) {
  let list = [...values];
  const box = h("div", { class: "tokens" });
  const input = h("input", { placeholder, "aria-label": label, maxlength: "40", autocomplete: "off", spellcheck: "false" });
  const draw = () => swap(box, list.map((v) => h("span", { class: "token" }, v,
    h("button", { type: "button", "aria-label": `Remove ${v}`, onclick: () => { list = list.filter((x) => x !== v); commit(); } }, icon("x")))), input);
  const commit = async () => {
    const ok = await onchange(list);
    if (ok === false) list = [...values];
    else values = [...list];
    draw();
    input.focus();
  };
  input.addEventListener("keydown", (e) => {
    if ((e.key === "Enter" || e.key === ",") && input.value.trim()) {
      e.preventDefault();
      const v = input.value.trim();
      input.value = "";
      if (!list.includes(v)) { list.push(v); commit(); }
    } else if (e.key === "Backspace" && !input.value && list.length) {
      list.pop();
      commit();
    }
  });
  box.addEventListener("click", (e) => { if (e.target === box) input.focus(); });
  draw();
  return box;
}

// ------------------------------------------------------------------ dialog
let openDlg = null;

export function openSettings(tab) {
  if (openDlg) openDlg.close();
  const sections = isHost() ? SECTIONS : VIEWER_SECTIONS;
  let section = sections.some(([k]) => k === tab) ? tab : "general";
  let settings = null;
  let info = null;
  const body = h("div", { class: "settings-body" });
  const nav = h("nav", { class: "settings-nav", role: "tablist", "aria-label": "Settings sections" });
  const close = h("button", { class: "icon-btn", type: "button", "aria-label": "Close", title: "Close" }, icon("x"));
  const dlg = h("dialog", { class: "modal settings-modal", "aria-label": "Settings" },
    h("div", { class: "modal-head" }, h("h2", { text: "Settings" }), close), h("div", { class: "settings-wrap" }, nav, body));
  close.addEventListener("click", () => dlg.close());
  dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
  dlg.addEventListener("close", () => { dlg.remove(); if (openDlg === dlg) openDlg = null; });
  openDlg = dlg;

  async function save(patch, okText) {
    try {
      const env = await send("PUT", "/settings", patch);
      settings = env.data;
      toast(okText || env.message || "Saved", { type: "success", timeout: env.message?.includes("scan") ? 6000 : 2000 });
      return true;
    } catch (err) {
      toastError(err);
      return false;
    }
  }

  // ---------------------------------------------------------------- general
  async function general() {
    const devices = await get("/devices");
    const me = devices.find((d) => d.current);
    const nameInput = h("input", { class: "input", maxlength: "60", placeholder: desktop.available ? "This computer" : describeUA(navigator.userAgent), autocomplete: "off", "aria-label": "Device name" });
    nameInput.value = me?.name || "";
    const nameForm = h("form", { class: "inline-form" }, nameInput, h("button", { class: "btn", type: "submit", text: "Save" }));
    nameForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      try { await send("PUT", "/devices/current", { name: nameInput.value.trim() }); toast("Device name saved", { type: "success" }); } catch (err) { toastError(err); }
    });
    const out = [
      group("This device"),
      row("Device name", "Shown in History so you know where a video was played.", nameForm),
      toggle("Play the next video automatically", "When a video ends, continue with the next one from the same list.", prefs().autoNext, (v) => setPref("autoNext", v)),
    ];
    if (desktop.available) {
      const [ds, app] = await Promise.all([desktop.invoke("desktop_settings"), desktop.invoke("app_info")]);
      const setD = async (key, value, input, msg) => {
        try { await desktop.invoke("set_desktop_setting", { key, value }); toast(msg, { type: "success", timeout: 2000 }); }
        catch (err) { toastError({ message: String(err) }); if (input) input.checked = !value; }
      };
      const status = h("span", { text: `You have version ${app.version}` });
      const check = h("button", { class: "btn sm", type: "button", text: "Check now", onclick: async () => {
        check.disabled = true;
        check.textContent = "Checking…";
        try {
          const u = await checkForUpdate({ quiet: false });
          await desktop.invoke("set_desktop_setting", { key: "last_update_check", value: Math.floor(Date.now() / 1000) });
          status.textContent = u ? `Version ${u.version} is available` : `You have the latest version (${app.version})`;
          if (u) showUpdateDialog();
        } catch (err) { status.textContent = `Couldn't check: ${err}`; }
        check.disabled = false;
        check.textContent = "Check now";
      } });
      out.push(
        group("This computer"),
        toggle("Start with the system", "ENStream starts in the tray when you log in, so phones and TVs can always watch", ds.autostart, (v, i) => setD("autostart", v, i, v ? "ENStream starts with the system" : "Start with the system is off")),
        toggle("Keep running in the tray when the window is closed", "Phones and TVs keep watching. Quit from the tray menu", ds.close_to_tray, (v, i) => setD("close_to_tray", v, i, "Saved")),
        toggle("Check for updates automatically", "Looks for a new ENStream release on GitHub once a day", ds.auto_update, (v, i) => setD("auto_update", v, i, "Saved")),
        row("Updates", status, check),
        row("Library data", h("span", { class: "mono", text: app.data_dir }), h("button", { class: "btn sm", type: "button", onclick: () => openAppFolder("data") }, icon("folder-open", "sm"), "Open")),
        row("Logs", "Useful when reporting a problem", h("button", { class: "btn sm", type: "button", onclick: () => openAppFolder("logs") }, icon("folder-open", "sm"), "Open")),
      );
    }
    const devBox = h("div");
    const drawDevices = (list) => swap(devBox, list.length ? list.map((d) => h("div", { class: "device" }, h("div", { class: "icon-wrap" }, icon(deviceIcon(d.user_agent))),
      h("div", null, h("strong", { text: d.name || describeUA(d.user_agent) }), d.current ? h("span", { class: "badge info", text: "This device" }) : null,
        h("small", { text: `${describeUA(d.user_agent)} · last seen ${fmtAgo(d.last_seen_at)}` })),
      d.current ? null : h("button", { class: "icon-btn", type: "button", "aria-label": `Forget ${d.name || "device"}`, title: "Forget device", onclick: async () => {
        if (!(await confirmDialog({ title: "Forget this device?", message: "Its name is removed. Playback positions stay in the library.", confirm: "Forget", danger: true }))) return;
        try { await send("DELETE", `/devices/${d.id}`); drawDevices(await get("/devices")); } catch (err) { toastError(err); }
      } }, icon("trash-2")))) : h("p", { class: "hint", text: "No device has played anything yet." }));
    if (isHost()) { // the device list (and forgetting devices) is part of managing the server
      drawDevices(devices);
      out.push(group("Devices"), devBox);
    }
    return out;
  }

  // ---------------------------------------------------------------- appearance
  function appearance() {
    const p = prefs();
    const themes = [["dark", "Dark"], ["light", "Light"], ["glass", "Glass"], ["system", "System"]];
    const cards = h("div", { class: "theme-cards" });
    const drawCards = () => swap(cards, themes.map(([id, label]) => h("button", { type: "button", class: `theme-card ${(prefs().theme || "dark") === id ? "sel" : ""}`, "aria-pressed": String((prefs().theme || "dark") === id),
      onclick: () => { setPref("theme", id); applyAppearance(); drawCards(); } },
      h("div", { class: `pv ${id}` }, h("i", { class: "side" }), h("div", { class: "main" }, h("i", { class: "barx" }), h("div", { class: "tiles" }, h("i"), h("i"), h("i")))), label)));
    drawCards();
    const sw = h("div", { class: "swatches", role: "radiogroup", "aria-label": "Accent color" });
    const drawSw = () => swap(sw, ACCENTS.map((c) => {
      const sel = (prefs().accent || ACCENTS[0]) === c;
      const b = h("button", { type: "button", class: `swatch ${sel ? "sel" : ""}`, role: "radio", "aria-checked": String(sel), "aria-label": c, title: c,
        onclick: () => { setPref("accent", c); applyAppearance(); drawSw(); } }, sel ? icon("check") : null);
      b.style.background = c;
      b.style.color = c;
      return b;
    }));
    drawSw();
    return [
      h("div", { class: "setting-block" }, h("strong", { text: "Theme" }), cards, h("p", { class: "hint", text: "Glass uses frosted, blurred panels over a colour gradient. Saved for this device." })),
      row("Accent color", null, sw),
      choose("Library layout", "How All Videos is shown", [["grid", "Grid"], ["list", "List"]], p.view || "grid", (v) => { setPref("view", v); window.dispatchEvent(new CustomEvent("enstream:library-changed")); }),
      toggle("Video previews", "Resting the mouse on a video, or pressing and holding it on a touch screen, plays a short muted preview. Saved for this device.", p.hoverPreview !== false, (v) => setPref("hoverPreview", v)),
    ];
  }

  // ---------------------------------------------------------------- library
  async function library() {
    settings ||= await get("/settings");
    const s = settings.scanner;
    const intervals = [[15, "Every 15 minutes"], [30, "Every 30 minutes"], [60, "Every hour"], [180, "Every 3 hours"], [360, "Every 6 hours"], [720, "Every 12 hours"], [1440, "Once a day"]];
    if (!intervals.some(([v]) => v === s.scan_interval_minutes)) intervals.push([s.scan_interval_minutes, `Every ${s.scan_interval_minutes} minutes`]);
    const intervalRow = choose("Scan interval", "Counted from the end of the last scan", intervals, s.scan_interval_minutes, (v) => save({ scanner: { scan_interval_minutes: Number(v) } }, "Scan interval saved"));
    intervalRow.hidden = !s.auto_scan;
    const exts = settings.media.extensions.map((e) => e.replace(/^\./, ""));
    return [
      group("Scanning"),
      toggle("Scan automatically", "Find new, changed and removed videos without pressing Scan", s.auto_scan, async (v, input) => {
        if (await save({ scanner: { auto_scan: v } }, v ? "Automatic scans on" : "Automatic scans off")) intervalRow.hidden = !v;
        else input.checked = !v;
      }),
      intervalRow,
      choose("Parallel workers", "Files read at once. Lower is gentler on slow or network drives", [1, 2, 3, 4, 6, 8].map((n) => [n, String(n)]), s.worker_count, (v) => save({ scanner: { worker_count: Number(v) } }, "Workers saved")),
      toggle("Include hidden files and folders", "Names starting with a dot, and hidden files on Windows", s.include_hidden, async (v, input) => { if (!(await save({ scanner: { include_hidden: v } }))) input.checked = !v; }),
      group("What to include"),
      h("div", { class: "setting-block" }, h("strong", { text: "Video formats" }),
        tokenEditor(exts, { placeholder: "Add a format, e.g. mp4", label: "Add video format", onchange: (list) => save({ media: { extensions: list } }) }),
        h("p", { class: "hint", text: "Files with other extensions are ignored. " }, h("button", { class: "btn ghost sm", type: "button", onclick: async () => { if (await save({ media: { extensions: settings.media.default_extensions } }, "Default formats restored")) draw(); } }, "Restore defaults"))),
      h("div", { class: "setting-block" }, h("strong", { text: "Skip folders named" }),
        tokenEditor(s.excluded_directories, { placeholder: "Add a folder name, e.g. Samples", label: "Add excluded folder name", onchange: (list) => save({ scanner: { excluded_directories: list } }) }),
        h("p", { class: "hint", text: "Any folder with one of these names is skipped, wherever it is." })),
      group("Thumbnails"),
      choose("Thumbnail width", "Applies to newly scanned videos", [[320, "320 px (small)"], [480, "480 px (default)"], [640, "640 px"], [960, "960 px (sharp)"]], settings.thumbnail.width, (v) => save({ thumbnail: { width: Number(v) } }, "Thumbnail width saved")),
      choose("Thumbnail quality", null, [[60, "60"], [70, "70"], [80, "80 (default)"], [90, "90"]], settings.thumbnail.quality, (v) => save({ thumbnail: { quality: Number(v) } }, "Thumbnail quality saved")),
      group("Folders"),
      row("Video folders", "Add, remove and scan the folders ENStream reads", h("a", { class: "btn sm", href: "/folders", "data-link": "", onclick: () => dlg.close() }, icon("folder-open", "sm"), "Manage folders")),
    ];
  }

  // ---------------------------------------------------------------- security
  async function security() {
    settings ||= await get("/settings");
    const enabled = settings.security.pin_enabled;
    const pinInput = (id, label, auto) => h("input", { class: "input", id, type: "password", autocomplete: auto, maxlength: "64", "aria-label": label, placeholder: label });
    const cur = enabled ? pinInput("pin-current", "Current PIN", "current-password") : null;
    const a = pinInput("pin-new", enabled ? "New PIN" : "PIN (4+ characters)", "new-password");
    const b = pinInput("pin-confirm", "Repeat the PIN", "new-password");
    const form = h("form", { class: "stack" }, cur, a, b, h("div", null, h("button", { class: "btn primary", type: "submit" }, enabled ? "Change PIN" : "Turn on PIN")));
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (a.value !== b.value) return toast("The two PINs are different", { type: "error" });
      try {
        const env = await send("PUT", "/auth/pin", { current_pin: cur?.value || "", new_pin: a.value });
        toast(env.message, { type: "success", timeout: 6000 });
        settings.security.pin_enabled = true;
        setPinEnabled(true);
        draw();
      } catch (err) { toastError(err); }
    });
    const out = [
      group("PIN lock"),
      row(enabled ? "A PIN is required on every device" : "Anyone on your network can open ENStream",
        enabled ? "Each browser stays signed in for 30 days of use" : "Turn on a PIN if guests use your Wi-Fi",
        enabled ? h("span", { class: "badge ok" }, icon("check", "sm"), "On") : h("span", { class: "badge", text: "Off" })),
      h("div", { class: "setting-block" }, h("strong", { text: enabled ? "Change the PIN" : "Set a PIN" }), form,
        h("p", { class: "hint", text: "Forgot the PIN? Start ENStream's server once with -reset-pin on this computer." })),
    ];
    if (enabled) {
      const offInput = h("input", { class: "input", type: "password", placeholder: "Current PIN", autocomplete: "current-password", "aria-label": "Current PIN" });
      const off = h("form", { class: "inline-form" }, offInput, h("button", { class: "btn danger", type: "submit", text: "Turn off" }));
      off.addEventListener("submit", async (e) => {
        e.preventDefault();
        try {
          await send("POST", "/auth/pin/disable", { current_pin: offInput.value });
          toast("PIN turned off", { type: "success" });
          settings.security.pin_enabled = false;
          setPinEnabled(false);
          draw();
        } catch (err) { toastError(err); }
      });
      out.push(row("Turn off the PIN", "Everyone on your network can open ENStream again", off));
      const sessions = await get("/auth/sessions");
      const list = h("div");
      const drawSessions = (items) => swap(list, items.map((x) => h("div", { class: "device" }, h("div", { class: "icon-wrap" }, icon(deviceIcon(x.user_agent))),
        h("div", null, h("strong", { text: describeUA(x.user_agent) }), x.current ? h("span", { class: "badge info", text: "This browser" }) : null,
          h("small", { text: `${x.ip || "unknown address"} · active ${fmtAgo(x.last_seen_at)} · signed in ${fmtDate(x.created_at)}` })),
        x.current ? null : h("button", { class: "icon-btn", type: "button", "aria-label": "Sign out this device", title: "Sign out", onclick: async () => {
          try { await send("DELETE", `/auth/sessions/${x.id}`); drawSessions(await get("/auth/sessions")); } catch (err) { toastError(err); }
        } }, icon("x")))));
      drawSessions(sessions);
      out.push(group("Signed-in devices"), list);
      if (sessions.length > 1) out.push(row("Sign out all other devices", null, h("button", { class: "btn sm", type: "button", text: "Sign out others", onclick: async () => {
        try { await send("POST", "/auth/sessions/sign-out-others"); toast("Other devices signed out", { type: "success" }); draw(); } catch (err) { toastError(err); }
      } })));
    }
    out.push(group("Network protection"), row("Local network only", "ENStream answers devices on your home network and VPNs such as Tailscale. Requests from the internet and from other websites are refused.", null));
    return out;
  }

  // ---------------------------------------------------------------- backups
  async function backups() {
    settings ||= await get("/settings");
    const d = await get("/backups");
    const kinds = { manual: ["Manual", "info"], auto: ["Automatic", ""], "pre-restore": ["Before restore", "warn"] };
    const listBox = h("div");
    const banner = h("div");
    const refresh = async () => { const x = await get("/backups"); drawList(x.backups); drawBanner(x.pending_restore); };
    const drawBanner = (pending) => swap(banner, pending ? h("div", { class: "banner" }, icon("info"),
      h("div", null, h("strong", { text: "Restart ENStream to finish restoring" }), `The library will be replaced with ${pending}. The current library is saved as a backup first.`),
      h("button", { class: "btn sm", type: "button", onclick: async () => { try { await send("DELETE", "/backups/restore"); toast("Restore cancelled"); refresh(); } catch (err) { toastError(err); } } }, "Cancel restore")) : null);
    const drawList = (items) => swap(listBox, items.length ? items.map((b) => {
      const [label, cls] = kinds[b.kind] || ["Backup", ""];
      return h("div", { class: "backup-row" }, h("div", { class: "icon-wrap" }, icon("hard-drive")),
        h("div", null, h("strong", { text: fmtDate(b.created_at) }), h("span", { class: `badge ${cls}`, text: label }), h("small", { text: `${b.name} · ${fmtSize(b.size)}` })),
        h("div", { class: "actions" },
          desktop.available
            ? h("button", { class: "icon-btn", type: "button", "aria-label": "Show in folder", title: "Show in folder", onclick: () => openAppFolder("backups") }, icon("folder-open"))
            : h("a", { class: "icon-btn", href: `/api/v1/backups/${encodeURIComponent(b.name)}`, download: b.name, "aria-label": `Download ${b.name}`, title: "Download" }, icon("download")),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Restore ${b.name}`, title: "Restore", onclick: () => restore(b) }, icon("rotate-ccw")),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Delete ${b.name}`, title: "Delete", onclick: () => remove(b) }, icon("trash-2"))));
    }) : h("p", { class: "hint", text: "No backups yet." }));
    async function restore(b) {
      const ok = await confirmDialog({ title: "Restore this backup?", message: `The library, history and settings go back to ${fmtDate(b.created_at)} after ENStream restarts. Your video files are not touched.`, confirm: "Restore on restart" });
      if (!ok) return;
      try { const env = await send("POST", `/backups/${encodeURIComponent(b.name)}/restore`); toast(env.message, { type: "success", timeout: 8000 }); refresh(); } catch (err) { toastError(err); }
    }
    async function remove(b) {
      if (!(await confirmDialog({ title: "Delete this backup?", message: b.name, confirm: "Delete", danger: true }))) return;
      try { await send("DELETE", `/backups/${encodeURIComponent(b.name)}`); refresh(); } catch (err) { toastError(err); }
    }
    drawList(d.backups);
    drawBanner(d.pending_restore);
    const createBtn = h("button", { class: "btn sm", type: "button", onclick: async () => {
      createBtn.disabled = true;
      try { await request("/backups", { method: "POST" }); toast("Backup created", { type: "success" }); refresh(); } catch (err) { toastError(err); } finally { createBtn.disabled = false; }
    } }, "Back up now");
    return [
      banner,
      group("Automatic backups"),
      toggle("Back up every day", "The library index, history, devices and PIN. Video files are never part of a backup", settings.backup.auto, async (v, input) => { if (!(await save({ backup: { auto: v } }, v ? "Daily backups on" : "Daily backups off"))) input.checked = !v; }),
      choose("Keep", "Older automatic backups are deleted", [3, 7, 14, 30].map((n) => [n, `${n} backups`]), settings.backup.keep, (v) => save({ backup: { keep: Number(v) } }, "Saved")),
      row("Make a backup now", null, createBtn),
      group("Backups"), listBox,
    ];
  }

  // ---------------------------------------------------------------- network
  async function network() {
    info ||= await get("/system/info");
    settings ||= await get("/settings");
    const addrs = (info.addresses || []).filter((a) => a.private);
    const main = addrs[0];
    return [
      group("Watch on other devices"),
      h("p", { class: "hint", text: "Phones, tablets, smart TVs and computers on the same Wi-Fi open one of these addresses in their browser." }),
      main ? h("div", { class: "qr-wrap" }, h("img", { class: "qr", src: "/api/v1/system/qr" + query({ url: main.url }), alt: `QR code for ${main.url}` }),
        h("div", null, addrs.map((a) => h("div", { class: "addr" }, icon("wifi", "sm"), h("a", { href: a.url, target: "_blank", rel: "noopener", text: a.url }), h("small", { text: a.interface }),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Copy ${a.url}`, title: "Copy", onclick: () => copyText(a.url, "Address copied") }, icon("copy", "sm"))))))
        : h("p", { class: "compat warn" }, icon("info"), "This computer isn't connected to a network. Connect it to Wi-Fi or Ethernet."),
      group("Server"),
      row("Listening on", null, h("span", { class: "mono", text: `${settings.server.host || "0.0.0.0"}:${settings.server.port}` })),
      row("Allowed networks", "Changed in config.json in the data folder (restart needed)", h("span", { class: "mono faint", text: `${settings.security.allowed_networks.length} ranges` })),
    ];
  }

  // ---------------------------------------------------------------- about
  async function about() {
    info ||= await get("/system/info");
    const app = desktop.available ? await desktop.invoke("app_info").catch(() => null) : null;
    const PLATFORM = { linux: "Linux", windows: "Windows", darwin: "macOS", macos: "macOS" };
    const [os, arch] = info.platform.split("/");
    const box = (title, rows) => h("section", { class: "about-section" }, h("div", { class: "group", text: title }), h("div", { class: "about-box" }, rows));
    const line = (label, hint, value) => h("div", { class: "about-row" }, h("span", null, label, hint ? h("small", { text: hint, title: hint }) : null), value);
    const val = (text, mono) => h("span", { class: mono ? "mono muted" : "muted", text });
    const tool = (t) => (t?.available ? val(t.version || "available", true) : h("span", { class: "badge bad", title: t?.message || "", text: "Not found" }));
    const ext = (href, text) => h("a", { class: "about-link", href, target: "_blank", rel: "noopener" }, text, href.startsWith("http") ? icon("external-link", "sm") : null);

    const copyDiagnostics = () => copyText([
      `ENStream ${info.version}${app ? ` (desktop ${app.version})` : ""}`,
      `${PLATFORM[os] || os} · ${arch}${app?.webview ? ` · WebView ${app.webview}` : ""}`,
      `FFmpeg ${info.ffmpeg?.version || "missing"} · FFprobe ${info.ffprobe?.version || "missing"}`,
      `Database schema v${info.schema_version} · ${info.go_version}`,
    ].join("\n"), "Diagnostics copied");

    if (!isHost()) return viewerAbout(info, box, line, val, ext);
    const thisCopy = [
      line("Version", null, val(info.version, true)),
      line("System", null, val(`${PLATFORM[os] || os} · ${arch}`)),
      app ? line("Tauri · WebView", null, val(`${app.tauri || "?"} · ${app.webview || "unknown"}`, true)) : null,
      line("FFmpeg · FFprobe", "Thumbnails, durations and format checks", h("span", { class: "about-pair" }, tool(info.ffmpeg), tool(info.ffprobe))),
      line("Database", null, val(`SQLite, schema v${info.schema_version}`)),
      line("Running for", null, val(fmtDuration(info.uptime_seconds))),
      app ? line("Library, history and settings", app.data_dir, h("button", { class: "btn sm", type: "button", onclick: () => openAppFolder("data") }, icon("folder-open", "sm"), "Open")) : null,
      app ? line("Logs", null, h("button", { class: "btn sm", type: "button", onclick: () => openAppFolder("logs") }, icon("folder-open", "sm"), "Open")) : null,
    ];
    const out = [
      h("div", { class: "about" }, h("img", { src: "/static/assets/enstream-icon.svg", alt: "", draggable: "false" }), h("div", null,
        h("div", { class: "about-name" }, "ENStream ", h("span", { class: "muted", text: info.version })),
        h("div", { class: "muted", text: "Your personal cinema, streamed to every screen at home." }),
        h("div", { class: "hint", text: "Pick your video folders and watch them on phones, TVs and computers on your Wi-Fi. No cloud, no account." }))),
      box("This copy", thisCopy),
      h("div", { class: "about-actions" }, h("button", { class: "btn ghost sm", type: "button", onclick: copyDiagnostics }, icon("copy", "sm"), "Copy diagnostics")),
    ];
    if (!info.ffmpeg?.available || !info.ffprobe?.available) out.push(await ffmpegRow());
    out.push(
      box("Developer", [
        line("Developed by", null, val("M. Estiaque Ahmed Khan")),
        line("Contact", "Bug reports, ideas and feedback are welcome", ext("mailto:info@mestiaque.com", "info@mestiaque.com")),
        line("Website", null, ext("https://mestiaque.com", "mestiaque.com")),
        line("Also by the developer", "Same look and themes", val("ENViewer · ENSave · ENPlayer · ENPost")),
        line("Licence", null, val("© 2026 M. Estiaque Ahmed Khan · MIT")),
      ]),
      box("Built with", BUILT_WITH.map(([name, what, licence]) => h("div", { class: "about-lib" }, h("span", null, name, h("span", { class: "faint", text: ` · ${what}` })), h("span", { class: "faint", text: licence })))),
      box("Keyboard shortcuts", h("div", { class: "kbd-list" }, SHORTCUTS.map(([a, k]) => h("div", null, h("span", { text: a }), h("span", null, k.split(" ").map((x) => h("kbd", { text: x }))))))),
    );
    return out;
  }

  // About on a phone, TV or other computer: what ENStream is, this device and the developer;
  // the server's technical details stay on the host.
  function viewerAbout(info, box, line, val, ext) {
    return [
      h("div", { class: "about" }, h("img", { src: "/static/assets/enstream-icon.svg", alt: "", draggable: "false" }), h("div", null,
        h("div", { class: "about-name" }, "ENStream ", h("span", { class: "muted", text: info.version })),
        h("div", { class: "muted", text: "Your personal cinema, streamed to every screen at home." }),
        h("div", { class: "hint", text: "Videos come from the computer that runs ENStream; nothing is uploaded to the internet." }))),
      box("This device", [
        line("Watching from", null, val(describeUA(navigator.userAgent))),
        line("Connected to", "The computer that shares the videos", val(location.host, true)),
      ]),
      box("Keyboard shortcuts", h("div", { class: "kbd-list" }, SHORTCUTS.map(([a, k]) => h("div", null, h("span", { text: a }), h("span", null, k.split(" ").map((x) => h("kbd", { text: x }))))))),
      box("Developer", [
        line("Developed by", null, val("M. Estiaque Ahmed Khan")),
        line("Contact", "Bug reports, ideas and feedback are welcome", ext("mailto:info@mestiaque.com", "info@mestiaque.com")),
        line("Website", null, ext("https://mestiaque.com", "mestiaque.com")),
        line("Licence", null, val("© 2026 M. Estiaque Ahmed Khan · MIT")),
      ]),
    ];
  }

  async function ffmpegRow() {
    const t = await get("/system/ffmpeg").catch(() => null);
    if (!t || !t.can_install) return row("Install FFmpeg", t?.hint || "", null);
    const bar = h("i");
    const progress = h("div", { class: "bar", hidden: !t.installing }, bar);
    const btn = h("button", { class: "btn primary sm", type: "button", disabled: t.installing, text: "Install FFmpeg" });
    const watch = () => {
      const id = setInterval(async () => {
        const s = await get("/system/ffmpeg").catch(() => null);
        if (!s) return;
        if (s.total) bar.style.width = `${Math.round((s.done / s.total) * 100)}%`;
        if (!s.installing) {
          clearInterval(id);
          if (s.ffmpeg.available) { toast(`FFmpeg ${s.ffmpeg.version} installed`, { type: "success" }); info = null; draw(); }
          else { toastError({ message: s.error || "FFmpeg could not be installed" }); btn.disabled = false; progress.hidden = true; }
        }
      }, 800);
    };
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      progress.hidden = false;
      try { await send("POST", "/system/ffmpeg/install"); watch(); } catch (err) { toastError(err); btn.disabled = false; }
    });
    if (t.installing) watch();
    return h("div", { class: "setting-block" }, row("Install FFmpeg", "Downloaded from the official builds and checked before use", btn), progress);
  }

  const views = { general, appearance, library, security, backups, network, about };
  let seq = 0;
  async function draw() {
    const my = ++seq;
    for (const b of nav.children) b.setAttribute("aria-selected", String(b.dataset.section === section));
    swap(body, loading());
    try {
      const content = await views[section]();
      if (my === seq) { swap(body, content); body.scrollTop = 0; }
    } catch (err) {
      if (my === seq) swap(body, errorState(err, draw));
    }
  }
  for (const [key, label] of sections) nav.append(h("button", { type: "button", role: "tab", dataset: { section: key }, text: label, onclick: () => { section = key; draw(); } }));
  document.body.append(dlg);
  dlg.showModal();
  draw();
  return dlg;
}

// /settings still works as an address: it opens the dialog (see shell.js).
export async function render() {
  return { el: h("div"), title: "Settings" };
}
