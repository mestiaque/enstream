import { h, icon, swap, fmtAgo, fmtDuration, fmtDate, fmtSize } from "../dom.js";
import { get, send, request } from "../api.js";
import { pageHead, errorState, loading, toast, toastError, copyText, confirmDialog } from "../ui.js";
import { prefs, setPref } from "../store.js";
import { setPinEnabled } from "../shell.js";
import { desktop, checkForUpdate, showUpdateDialog, openAppFolder } from "../desktop.js";

const TABS = [
  ["general", "General", "monitor"],
  ...(desktop.available ? [["desktop", "Desktop", "power"]] : []), ["library", "Library", "film"], ["security", "Security", "lock"],
  ["backups", "Backups", "hard-drive"], ["about", "About", "info"],
];
const SHORTCUTS = [
  ["Play / pause", "Space"], ["Back / forward 5 s", "← →"], ["Back / forward 10 s", "J L"], ["Volume", "↑ ↓"], ["Mute", "M"], ["Full screen", "F"],
  ["Subtitles", "C"], ["Speed", "< >"], ["Next / previous video", "N P"], ["Jump to 0–90 %", "0–9"], ["Search", "/"], ["Leave the player", "Esc"],
];

const deviceIcon = (ua) => (/Mobi|Android|iPhone|iPad/i.test(ua || "") ? "smartphone" : "monitor");
function describeUA(ua) {
  if (!ua) return "Unknown browser";
  if (/^curl\//.test(ua)) return "Command line";
  const browser = /Edg\//.test(ua) ? "Edge" : /OPR\//.test(ua) ? "Opera" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome" : /Safari\//.test(ua) ? "Safari" : "Browser";
  const os = /Windows/.test(ua) ? "Windows" : /Android/.test(ua) ? "Android" : /iPhone|iPad|iPod/.test(ua) ? "iOS" : /Mac OS X/.test(ua) ? "macOS" : /CrOS/.test(ua) ? "ChromeOS" : /Tizen|Web0S|SMART-TV/.test(ua) ? "Smart TV" : /Linux/.test(ua) ? "Linux" : "";
  return os ? `${browser} on ${os}` : browser;
}

function panel(title, iconName, body, cls = "") {
  return h("section", { class: `panel ${cls}` }, h("div", { class: "panel-head" }, h("h2", { class: "scan-title" }, icon(iconName), title)), h("div", { class: "panel-body" }, body));
}

let uid = 0;
function switchRow(title, text, checked, onchange) {
  const id = `sw-${++uid}`;
  const input = h("input", { type: "checkbox", id });
  input.checked = checked;
  input.addEventListener("change", () => onchange(input.checked, input));
  return h("div", { class: "setting-row" }, h("label", { for: id }, h("strong", { text: title }), text ? h("p", { text }) : null), h("span", { class: "switch" }, input, h("span")));
}

function selectRow(title, text, options, value, onchange) {
  const id = `sel-${++uid}`;
  const sel = h("select", { class: "select", id }, options.map(([v, t]) => h("option", { value: String(v), text: t })));
  sel.value = String(value);
  sel.addEventListener("change", () => onchange(sel.value, sel));
  return h("div", { class: "setting-row" }, h("label", { for: id }, h("strong", { text: title }), text ? h("p", { text }) : null), sel);
}

// Editable list of short values (formats, folder names).
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

export async function render(ctx) {
  const el = h("div", null, pageHead("Settings"), loading());
  let info, settings;
  try {
    [info, settings] = await Promise.all([get("/system/info"), get("/settings")]);
  } catch (err) {
    swap(el, pageHead("Settings"), errorState(err, () => location.reload()));
    return { el, title: "Settings" };
  }
  let tab = TABS.some(([k]) => k === ctx.query.get("tab")) ? ctx.query.get("tab") : "general";
  const body = h("div");
  const tabBar = h("div", { class: "tabs", role: "tablist" });

  async function save(patch, okText) {
    try {
      const env = await send("PUT", "/settings", patch);
      settings = env.data;
      toast(okText || env.message || "Saved", { type: "success", timeout: env.message?.includes("scan") ? 6000 : 2200 });
      return true;
    } catch (err) {
      toastError(err);
      return false;
    }
  }

  // ------------------------------------------------------------------ general
  async function general() {
    const devices = await get("/devices");
    const me = devices.find((d) => d.current);
    const nameInput = h("input", { class: "input", id: "device-name", maxlength: "60", placeholder: describeUA(navigator.userAgent), autocomplete: "off" });
    nameInput.value = me?.name || "";
    const nameForm = h("form", { class: "inline-form" }, nameInput, h("button", { class: "btn primary", type: "submit", text: "Save" }));
    const devBox = h("div");
    const drawDevices = (list) => {
      if (!list.length) return swap(devBox, h("p", { class: "muted", text: "No devices have played anything yet." }));
      swap(devBox, list.map((d) => h("div", { class: "device" }, h("div", { class: "icon-wrap" }, icon(deviceIcon(d.user_agent))),
        h("div", null, h("strong", { text: d.name || describeUA(d.user_agent) }), d.current ? h("span", { class: "badge info", text: "This device" }) : null,
          h("small", { text: `${describeUA(d.user_agent)} · last seen ${fmtAgo(d.last_seen_at)}` })),
        d.current ? null : h("button", { class: "icon-btn", type: "button", "aria-label": `Forget ${d.name || "device"}`, title: "Forget device", onclick: async () => {
          if (!(await confirmDialog({ title: "Forget this device?", message: "Its name is removed. Playback positions stay in the library.", confirm: "Forget", danger: true }))) return;
          try { await send("DELETE", `/devices/${d.id}`); drawDevices(await get("/devices")); } catch (err) { toastError(err); }
        } }, icon("trash-2")))));
    };
    nameForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      try {
        await send("PUT", "/devices/current", { name: nameInput.value.trim() });
        toast("Device name saved", { type: "success" });
        drawDevices(await get("/devices"));
      } catch (err) { toastError(err); }
    });
    drawDevices(devices);
    const addrs = (info.addresses || []).filter((a) => a.private);
    return h("div", { class: "settings" },
      panel("This device", deviceIcon(navigator.userAgent), [
        h("div", { class: "field" }, h("label", { for: "device-name", text: "Device name" }), nameForm, h("span", { class: "hint", text: "Shown in History so you know where a video was played." })),
        h("div", { class: "section" }, switchRow("Play next video automatically", "When a video ends, continue with the next one from the same list.", prefs().autoNext, (v) => setPref("autoNext", v))),
      ]),
      panel("Devices", "monitor", devBox),
      panel("Open on other devices", "wifi", [
        h("p", { class: "muted", text: "Phones, tablets, smart TVs and computers on the same network can open ENStream in their browser." }),
        h("div", { class: "section" }, addrs.length ? addrs.map((a) => h("div", { class: "addr" }, icon("wifi", "sm"), h("a", { href: a.url, target: "_blank", rel: "noopener", text: a.url }), h("small", { class: "hide-sm", text: a.interface }),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Copy ${a.url}`, onclick: () => copyText(a.url, "Address copied") }, icon("copy", "sm"))))
          : h("p", { class: "faint", text: "No network connection found. Connect this computer to Wi-Fi or Ethernet." })),
      ], "wide"));
  }

  // ------------------------------------------------------------------ library
  function library() {
    const s = settings.scanner;
    const intervals = [[15, "Every 15 minutes"], [30, "Every 30 minutes"], [60, "Every hour"], [180, "Every 3 hours"], [360, "Every 6 hours"], [720, "Every 12 hours"], [1440, "Once a day"]];
    if (!intervals.some(([v]) => v === s.scan_interval_minutes)) intervals.push([s.scan_interval_minutes, `Every ${s.scan_interval_minutes} minutes`]);
    const intervalRow = selectRow("Scan interval", "Counted from the end of the last scan.", intervals, s.scan_interval_minutes, (v) => save({ scanner: { scan_interval_minutes: Number(v) } }, "Scan interval saved"));
    intervalRow.hidden = !s.auto_scan;
    const exts = settings.media.extensions.map((e) => e.replace(/^\./, ""));
    return h("div", { class: "settings" },
      panel("Scanning", "refresh-cw", [
        switchRow("Scan automatically", "Find new, changed and removed videos without pressing Scan.", s.auto_scan, async (v, input) => {
          if (await save({ scanner: { auto_scan: v } }, v ? "Automatic scans on" : "Automatic scans off")) intervalRow.hidden = !v;
          else input.checked = !v;
        }),
        intervalRow,
        selectRow("Parallel workers", "How many files are read at once. Lower is gentler on slow or network drives.", [1, 2, 3, 4, 6, 8].map((n) => [n, String(n)]), s.worker_count,
          (v) => save({ scanner: { worker_count: Number(v) } }, "Workers saved")),
        switchRow("Include hidden files and folders", "Names starting with a dot (and hidden files on Windows).", s.include_hidden, async (v, input) => { if (!(await save({ scanner: { include_hidden: v } }))) input.checked = !v; }),
      ]),
      panel("What to include", "folder", [
        h("div", { class: "field" }, h("label", { text: "Video formats" }),
          tokenEditor(exts, { placeholder: "Add a format, e.g. mp4", label: "Add video format", onchange: (list) => save({ media: { extensions: list } }) }),
          h("span", { class: "hint", text: "Files with other extensions are ignored." }),
          h("button", { class: "btn ghost sm", type: "button", onclick: async () => { if (await save({ media: { extensions: settings.media.default_extensions } }, "Default formats restored")) draw(); } }, icon("rotate-ccw", "sm"), "Restore defaults")),
        h("div", { class: "field section" }, h("label", { text: "Skip folders named" }),
          tokenEditor(s.excluded_directories, { placeholder: "Add a folder name, e.g. Samples", label: "Add excluded folder name", onchange: (list) => save({ scanner: { excluded_directories: list } }) }),
          h("span", { class: "hint", text: "Any folder with one of these names is skipped, wherever it is." })),
      ]),
      panel("Thumbnails", "clapperboard", [
        selectRow("Width", "Applies to newly scanned videos.", [[320, "320 px (small)"], [480, "480 px (default)"], [640, "640 px"], [960, "960 px (sharp, larger)"]], settings.thumbnail.width,
          (v) => save({ thumbnail: { width: Number(v) } }, "Thumbnail width saved")),
        selectRow("Quality", "", [[60, "60"], [70, "70"], [80, "80 (default)"], [90, "90"]], settings.thumbnail.quality, (v) => save({ thumbnail: { quality: Number(v) } }, "Thumbnail quality saved")),
      ]),
      panel("Server", "server", h("dl", { class: "kv" },
        h("dt", { text: "Listening on" }), h("dd", { class: "mono", text: `${settings.server.host || "0.0.0.0"}:${settings.server.port}` }),
        h("dt", { text: "Allowed networks" }), h("dd", { class: "mono", text: settings.security.allowed_networks.join(", ") }),
        h("dt", { text: "Change" }), h("dd", { class: "faint", text: "Edit config.json in the data folder, then restart ENStream." }))));
  }

  // ------------------------------------------------------------------ security
  async function security() {
    const enabled = settings.security.pin_enabled;
    const pinField = (id, label, auto) => h("div", { class: "field" }, h("label", { for: id, text: label }),
      h("input", { class: "input", id, type: "password", autocomplete: auto, minlength: "4", maxlength: "64", required: true }));
    const form = h("form", { class: "stack" },
      enabled ? pinField("pin-current", "Current PIN", "current-password") : null,
      pinField("pin-new", enabled ? "New PIN" : "PIN", "new-password"),
      pinField("pin-confirm", "Repeat the PIN", "new-password"),
      h("div", null, h("button", { class: "btn primary", type: "submit" }, icon("check", "sm"), enabled ? "Change PIN" : "Turn on PIN")));
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const v = (id) => form.querySelector("#" + id)?.value || "";
      if (v("pin-new") !== v("pin-confirm")) return toast("The two PINs are different", { type: "error" });
      try {
        const env = await send("PUT", "/auth/pin", { current_pin: v("pin-current"), new_pin: v("pin-new") });
        toast(env.message, { type: "success", timeout: 6000 });
        settings.security.pin_enabled = true;
        setPinEnabled(true);
        draw();
      } catch (err) { toastError(err); }
    });

    const parts = [panel("PIN lock", "lock", [
      h("p", { class: "muted", text: enabled ? "A PIN is required on every device. This browser stays signed in for 30 days of use." :
        "Anyone on your network can open ENStream. Turn on a PIN to keep the library private, for example from guests on your Wi-Fi." }),
      h("div", { class: "section" }, enabled ? h("span", { class: "badge ok" }, icon("check", "sm"), "PIN is on") : h("span", { class: "badge warn", text: "No PIN" })),
      h("div", { class: "section" }, form),
      h("p", { class: "hint section", text: "Forgot the PIN? Start ENStream with the -reset-pin option on its computer." }),
    ])];

    if (enabled) {
      const off = h("form", { class: "inline-form" }, h("input", { class: "input", type: "password", id: "pin-off", placeholder: "Current PIN", autocomplete: "current-password", required: true, "aria-label": "Current PIN" }),
        h("button", { class: "btn danger", type: "submit", text: "Turn off" }));
      off.addEventListener("submit", async (e) => {
        e.preventDefault();
        try {
          await send("POST", "/auth/pin/disable", { current_pin: off.querySelector("input").value });
          toast("PIN turned off", { type: "success" });
          settings.security.pin_enabled = false;
          setPinEnabled(false);
          draw();
        } catch (err) { toastError(err); }
      });
      const sessions = await get("/auth/sessions");
      const list = h("div");
      const drawSessions = (items) => swap(list, items.map((x) => h("div", { class: "device" }, h("div", { class: "icon-wrap" }, icon(deviceIcon(x.user_agent))),
        h("div", null, h("strong", { text: describeUA(x.user_agent) }), x.current ? h("span", { class: "badge info", text: "This browser" }) : null,
          h("small", { text: `${x.ip || "unknown address"} · active ${fmtAgo(x.last_seen_at)} · signed in ${fmtDate(x.created_at)}` })),
        x.current ? null : h("button", { class: "icon-btn", type: "button", "aria-label": "Sign out this device", title: "Sign out", onclick: async () => {
          try { await send("DELETE", `/auth/sessions/${x.id}`); drawSessions(await get("/auth/sessions")); } catch (err) { toastError(err); }
        } }, icon("x")))));
      drawSessions(sessions);
      parts.push(panel("Signed-in devices", "monitor", [list,
        sessions.length > 1 ? h("div", { class: "section" }, h("button", { class: "btn", type: "button", onclick: async () => {
          try { await send("POST", "/auth/sessions/sign-out-others"); toast("Other devices signed out", { type: "success" }); drawSessions(await get("/auth/sessions")); } catch (err) { toastError(err); }
        } }, "Sign out all other devices")) : null]));
      parts.push(panel("Turn off the PIN", "x", [h("p", { class: "muted", text: "Everyone on your network can open ENStream again." }), h("div", { class: "section" }, off)]));
    }
    parts.push(panel("Network protection", "shield-check", h("p", { class: "muted", text: "ENStream only answers devices on your local network (home Wi-Fi, Ethernet, VPNs such as Tailscale). Requests from the internet and from other websites are refused." })));
    return h("div", { class: "settings" }, parts);
  }

  // ------------------------------------------------------------------ backups
  async function backups() {
    const d = await get("/backups");
    const kinds = { manual: ["Manual", "info"], auto: ["Automatic", ""], "pre-restore": ["Before restore", "warn"] };
    const listBox = h("div");
    const drawList = (items) => {
      if (!items.length) return swap(listBox, h("p", { class: "muted", text: "No backups yet." }));
      swap(listBox, items.map((b) => {
        const [label, cls] = kinds[b.kind] || ["Backup", ""];
        return h("div", { class: "backup-row" }, h("div", { class: "icon-wrap" }, icon("hard-drive")),
          h("div", null, h("strong", { text: fmtDate(b.created_at) }), " ", h("span", { class: `badge ${cls}`, text: label }), h("small", { text: `${b.name} · ${fmtSize(b.size)}` })),
          h("div", { class: "actions" },
            desktop.available
              ? h("button", { class: "icon-btn", type: "button", "aria-label": "Show in folder", title: "Show in folder", onclick: () => openAppFolder("backups") }, icon("folder-open"))
              : h("a", { class: "icon-btn", href: `/api/v1/backups/${encodeURIComponent(b.name)}`, download: b.name, "aria-label": `Download ${b.name}`, title: "Download" }, icon("download")),
            h("button", { class: "icon-btn", type: "button", "aria-label": `Restore ${b.name}`, title: "Restore", onclick: () => restore(b) }, icon("rotate-ccw")),
            h("button", { class: "icon-btn", type: "button", "aria-label": `Delete ${b.name}`, title: "Delete", onclick: () => remove(b) }, icon("trash-2"))));
      }));
    };
    const refresh = async () => { const x = await get("/backups"); drawList(x.backups); drawBanner(x.pending_restore); };
    const banner = h("div");
    const drawBanner = (pending) => swap(banner, pending ? h("div", { class: "banner" }, icon("info"),
      h("div", null, h("strong", { text: "Restart ENStream to finish restoring" }), `The library will be replaced with ${pending}. The current library is saved as a backup first.`),
      h("button", { class: "btn sm", type: "button", onclick: async () => { try { await send("DELETE", "/backups/restore"); toast("Restore cancelled"); refresh(); } catch (err) { toastError(err); } } }, "Cancel restore")) : null);
    async function restore(b) {
      const ok = await confirmDialog({ title: "Restore this backup?", message: `The library, history and settings stored in the database go back to ${fmtDate(b.created_at)} after ENStream restarts. Your video files are not touched.`, confirm: "Restore on restart" });
      if (!ok) return;
      try { const env = await send("POST", `/backups/${encodeURIComponent(b.name)}/restore`); toast(env.message, { type: "success", timeout: 8000 }); refresh(); } catch (err) { toastError(err); }
    }
    async function remove(b) {
      if (!(await confirmDialog({ title: "Delete this backup?", message: b.name, confirm: "Delete", danger: true }))) return;
      try { await send("DELETE", `/backups/${encodeURIComponent(b.name)}`); refresh(); } catch (err) { toastError(err); }
    }
    drawList(d.backups);
    drawBanner(d.pending_restore);
    const createBtn = h("button", { class: "btn primary", type: "button", onclick: async () => {
      createBtn.disabled = true;
      try { await request("/backups", { method: "POST" }); toast("Backup created", { type: "success" }); refresh(); } catch (err) { toastError(err); } finally { createBtn.disabled = false; }
    } }, icon("hard-drive", "sm"), "Back up now");
    return h("div", null, banner, h("div", { class: "settings" },
      panel("Automatic backups", "refresh-cw", [
        h("p", { class: "muted", text: "A backup holds the library index, playback history, devices and PIN. Video files are never part of it." }),
        h("div", { class: "section" },
          switchRow("Back up every day", "Made in the backups folder of the data directory.", settings.backup.auto, async (v, input) => { if (!(await save({ backup: { auto: v } }, v ? "Daily backups on" : "Daily backups off"))) input.checked = !v; }),
          selectRow("Keep", "Older automatic backups are deleted.", [3, 7, 14, 30].map((n) => [n, `${n} backups`]), settings.backup.keep, (v) => save({ backup: { keep: Number(v) } }, "Saved"))),
      ]),
      panel("Backups", "hard-drive", [h("div", null, createBtn), h("div", { class: "section" }, listBox)])));
  }

  // ------------------------------------------------------------------ desktop app
  async function desktopTab() {
    const [ds, app] = await Promise.all([desktop.invoke("desktop_settings"), desktop.invoke("app_info")]);
    const setD = async (key, value, input, msg) => {
      try { await desktop.invoke("set_desktop_setting", { key, value }); toast(msg, { type: "success", timeout: 2000 }); }
      catch (err) { toastError({ message: String(err) }); if (input) input.checked = !value; }
    };
    const status = h("p", { class: "muted" });
    const checkBtn = h("button", { class: "btn", type: "button", onclick: async () => {
      checkBtn.disabled = true;
      status.textContent = "Checking…";
      try {
        const u = await checkForUpdate({ quiet: false });
        await desktop.invoke("set_desktop_setting", { key: "last_update_check", value: Math.floor(Date.now() / 1000) });
        status.textContent = u ? `Version ${u.version} is available.` : `You have the latest version (${app.version}).`;
        if (u) showUpdateDialog();
      } catch (err) { status.textContent = `Couldn't check: ${err}`; }
      checkBtn.disabled = false;
    } }, icon("refresh-cw", "sm"), "Check for updates");
    return h("div", { class: "settings" },
      panel("Startup and tray", "power", [
        switchRow("Start with this computer", "ENStream starts in the tray when you sign in, so phones and TVs can always watch.", ds.autostart, (v, i) => setD("autostart", v, i, v ? "ENStream starts with the computer" : "Autostart off")),
        switchRow("Keep running when the window is closed", "Closing the window leaves ENStream in the system tray. Quit from the tray menu.", ds.close_to_tray, (v, i) => setD("close_to_tray", v, i, "Saved")),
      ]),
      panel("Updates", "download", [
        switchRow("Check for updates every day", "Only asks for the latest version number; nothing about you is sent.", ds.auto_update, (v, i) => setD("auto_update", v, i, "Saved")),
        h("div", { class: "setting-row" }, h("div", null, h("strong", { text: `Version ${app.version}` }), status), checkBtn),
      ]),
      panel("Folders", "folder-open", h("dl", { class: "kv" },
        h("dt", { text: "Library data" }), h("dd", null, h("span", { class: "mono", text: app.data_dir }), " ", h("button", { class: "btn sm ghost", type: "button", onclick: () => openAppFolder("data") }, icon("folder-open", "sm"), "Open")),
        h("dt", { text: "Logs" }), h("dd", null, h("button", { class: "btn sm ghost", type: "button", onclick: () => openAppFolder("logs") }, icon("folder-open", "sm"), "Open")))));
  }

  // ------------------------------------------------------------------ about
  function ffmpegBox() {
    const msg = h("div", null, h("strong", { text: "FFmpeg is missing" }), "Videos still play, but thumbnails, durations and format checks need FFmpeg.");
    const box = h("div", { class: "compat warn section" }, icon("info"), msg);
    get("/system/ffmpeg").then((t) => {
      if (!t.can_install) return msg.append(h("p", { class: "mono", text: t.hint }));
      const bar = h("i");
      const progress = h("div", { class: "bar", hidden: !t.installing }, bar);
      const btn = h("button", { class: "btn primary sm", type: "button", disabled: t.installing }, icon("download", "sm"), "Install FFmpeg");
      msg.append(progress, h("div", { class: "section" }, btn));
      const watch = () => {
        const id = setInterval(async () => {
          const s = await get("/system/ffmpeg").catch(() => null);
          if (!s) return;
          if (s.total) bar.style.width = `${Math.round((s.done / s.total) * 100)}%`;
          if (!s.installing) {
            clearInterval(id);
            if (s.ffmpeg.available) { toast(`FFmpeg ${s.ffmpeg.version} installed`, { type: "success" }); draw(); }
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
    }).catch(() => {});
    return box;
  }

  function about() {
    const toolRow = (name, t) => [h("dt", { text: name }), h("dd", null, t?.available ? h("span", { class: "badge ok" }, icon("check", "sm"), t.version || "Available")
      : h("span", { class: "badge bad", title: t?.message || "", text: "Not found" }))];
    return h("div", { class: "settings" },
      panel("About", "clapperboard", [
        h("div", { class: "about" }, h("img", { src: "/static/assets/enstream-icon.svg", alt: "" }), h("div", null, h("h2", { text: "ENStream" }), h("p", { class: "muted", text: `Your Personal Cinema · version ${info.version}` }),
          h("p", { class: "faint", text: "Stream the videos on this computer to every screen at home. No cloud, no account, no tracking." }))),
        h("dl", { class: "kv section" }, h("dt", { text: "Family" }), h("dd", { text: "ENViewer · ENSave · ENPlayer · ENPost · ENStream" }),
          h("dt", { text: "Built with" }), h("dd", { text: `Go (${info.go_version}), SQLite, FFmpeg` }),
          h("dt", { text: "Icons" }), h("dd", { text: "Lucide (ISC license)" }),
          h("dt", { text: "License" }), h("dd", { text: "MIT" })),
      ]),
      h("div", null, panel("Server", "server", h("dl", { class: "kv" },
        h("dt", { text: "Version" }), h("dd", { text: info.version }),
        h("dt", { text: "Running for" }), h("dd", { text: fmtDuration(info.uptime_seconds) }),
        h("dt", { text: "Platform" }), h("dd", { text: info.platform }),
        toolRow("FFmpeg", info.ffmpeg), toolRow("FFprobe", info.ffprobe),
        h("dt", { text: "Database" }), h("dd", { text: `Schema v${info.schema_version}` }))),
        !info.ffmpeg?.available || !info.ffprobe?.available ? ffmpegBox() : null),
      panel("Keyboard shortcuts", "info", h("div", { class: "kbd-list" }, SHORTCUTS.map(([a, k]) => h("div", null, h("span", { text: a }), h("span", null, k.split(" ").map((x) => h("kbd", { text: x })))))), "wide"));
  }

  const views = { general, desktop: desktopTab, library, security, backups, about };
  let seq = 0;
  async function draw() {
    const my = ++seq;
    for (const b of tabBar.children) b.setAttribute("aria-selected", String(b.dataset.tab === tab));
    swap(body, loading());
    try {
      const content = await views[tab]();
      if (my === seq) swap(body, content);
    } catch (err) {
      if (my === seq) swap(body, errorState(err, draw));
    }
  }
  for (const [key, label, ic] of TABS) {
    tabBar.append(h("button", { class: "tab", type: "button", role: "tab", dataset: { tab: key }, onclick: () => { tab = key; ctx.setQuery({ tab: key === "general" ? "" : key }); draw(); } }, icon(ic, "sm"), label));
  }
  swap(el, pageHead("Settings", "This device, the library, security and backups."), tabBar, body);
  draw();
  return { el, title: "Settings" };
}
