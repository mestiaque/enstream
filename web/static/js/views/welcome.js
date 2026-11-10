// First-run setup: pick video folders, optionally set a PIN, then open it on a phone.
import { h, icon, swap, fmtSize } from "../dom.js";
import { get, send, query } from "../api.js";
import { toast, toastError, copyText } from "../ui.js";
import { pickFolder } from "../folderpicker.js";
import { setPinEnabled, pollScan, refreshCount } from "../shell.js";

export const WELCOMED_KEY = "enstream.welcomed";
export function markWelcomed() {
  try { localStorage.setItem(WELCOMED_KEY, "1"); } catch { /* ignore */ }
}
export function welcomed() {
  try { return localStorage.getItem(WELCOMED_KEY) === "1"; } catch { return true; }
}

export async function render(ctx) {
  let step = 0;
  const card = h("div", { class: "panel-body" });
  const steps = h("div", { class: "steps", "aria-hidden": "true" }, [0, 1, 2].map(() => h("span")));
  let timer;
  const cleanups = [];

  const finish = () => { markWelcomed(); ctx.navigate("/", { replace: true }); };
  const foot = (back, next, nextLabel = "Continue", skip) => h("div", { class: "welcome-foot" },
    back ? h("button", { class: "btn ghost", type: "button", onclick: back }, icon("arrow-left", "sm"), "Back") : h("button", { class: "btn ghost", type: "button", onclick: finish, text: "Skip setup" }),
    h("div", { class: "actions" }, skip ? h("button", { class: "btn ghost", type: "button", onclick: skip, text: "Skip" }) : null,
      h("button", { class: "btn grad", type: "button", onclick: next }, nextLabel, icon("chevron-right", "sm"))));

  // ------------------------------------------------------------------ 1. folders
  async function folders() {
    const [list, places, tools] = await Promise.all([get("/folders"), get("/fs/dirs"), get("/system/ffmpeg").catch(() => null)]);
    const chosen = h("div", { class: "chosen" });
    const drawChosen = (items) => swap(chosen, items.length ? items.map((f) => h("div", { class: "addr" }, icon("folder", "sm"),
      h("span", { class: "mono", title: f.path, text: f.path }), h("small", { text: `${f.video_count} videos · ${fmtSize(f.total_size)}` })))
      : h("p", { class: "faint", text: "No folders yet." }));
    const add = async (path) => {
      try {
        const env = await send("POST", "/folders", { path });
        toast(`Added “${env.data.name}”`, { type: "success", detail: "Scanning it in the background" });
        pollScan();
        drawChosen(await get("/folders"));
        refreshCount();
      } catch (err) { toastError(err); }
    };
    drawChosen(list);
    // Counts fill in when the background scan finishes.
    const onChange = async () => { if (chosen.isConnected) drawChosen(await get("/folders")); };
    window.addEventListener("enstream:library-changed", onChange);
    cleanups.push(() => window.removeEventListener("enstream:library-changed", onChange));
    const suggestions = places.dirs.filter((d) => d.kind === "videos" || d.kind === "removable").slice(0, 6);
    let ffmpegNote = null;
    if (tools && !(tools.ffmpeg.available && tools.ffprobe.available)) {
      ffmpegNote = h("div", { class: "compat warn section" }, icon("info"), h("div", null, h("strong", { text: "FFmpeg is needed for thumbnails and video details" }), tools.can_install ? "ENStream is installing it for you…" : tools.hint));
      if (tools.can_install) installFFmpeg(ffmpegNote, tools);
    }
    return [
      h("h1", { text: "Welcome to ENStream" }),
      h("p", { class: "lead", text: "Your own private streaming service. Choose the folders with your videos; they stay where they are and are never changed." }),
      h("div", { class: "section" }, h("button", { class: "btn primary lg", type: "button", onclick: async () => { const p = await pickFolder(); if (p) add(p); } }, icon("folder-plus"), "Choose a folder…"),
        suggestions.length ? h("div", { class: "suggest" }, h("span", { class: "faint", text: "Quick add:" }),
          suggestions.map((d) => h("button", { class: "btn sm", type: "button", title: d.path, onclick: () => add(d.path) }, icon(d.kind === "removable" ? "hard-drive" : "film", "sm"), d.name))) : null),
      chosen, ffmpegNote,
      foot(null, () => go(1)),
    ];
  }

  async function installFFmpeg(note, tools) {
    const bar = h("i");
    const box = h("div", { class: "bar" }, bar);
    note.querySelector("div").append(box);
    if (!tools.installing) {
      try { await send("POST", "/system/ffmpeg/install"); } catch (err) { box.replaceWith(h("span", { text: ` ${err.message}` })); return; }
    }
    clearInterval(timer);
    timer = setInterval(async () => {
      const t = await get("/system/ffmpeg").catch(() => null);
      if (!t) return;
      if (t.total) bar.style.width = `${Math.round((t.done / t.total) * 100)}%`;
      if (!t.installing) {
        clearInterval(timer);
        if (t.ffmpeg.available) {
          note.className = "compat ok section";
          swap(note, icon("check"), h("div", null, h("strong", { text: `FFmpeg ${t.ffmpeg.version} installed` }), "Thumbnails and details are on."));
          send("POST", "/scanner/scan", {}).catch(() => {});
        } else {
          swap(note.querySelector("div"), h("strong", { text: "FFmpeg could not be installed" }), t.error || "Try again from Settings → About.");
        }
      }
    }, 800);
  }

  // ------------------------------------------------------------------ 2. PIN
  function pin() {
    const a = h("input", { class: "input", type: "password", id: "w-pin", autocomplete: "new-password", maxlength: "64", placeholder: "At least 4 characters" });
    const b = h("input", { class: "input", type: "password", id: "w-pin2", autocomplete: "new-password", maxlength: "64" });
    const save = async () => {
      if (!a.value) return go(2);
      if (a.value !== b.value) return toast("The two PINs are different", { type: "error" });
      try {
        await send("PUT", "/auth/pin", { new_pin: a.value });
        setPinEnabled(true);
        toast("PIN turned on", { type: "success" });
        go(2);
      } catch (err) { toastError(err); }
    };
    return [
      h("h1", { text: "Keep it private?" }),
      h("p", { class: "lead", text: "Anyone on your Wi-Fi can open ENStream. Add a PIN if guests use your network; every device asks for it once." }),
      h("div", { class: "stack section" }, h("div", { class: "field" }, h("label", { for: "w-pin", text: "PIN (optional)" }), a), h("div", { class: "field" }, h("label", { for: "w-pin2", text: "Repeat the PIN" }), b)),
      foot(() => go(0), save, "Continue", () => go(2)),
    ];
  }

  // ------------------------------------------------------------------ 3. phones
  async function phones() {
    const info = await get("/system/info");
    const addrs = (info.addresses || []).filter((x) => x.private);
    const main = addrs[0];
    return [
      h("h1", { text: "Watch on your phone and TV" }),
      h("p", { class: "lead", text: "Devices on the same Wi-Fi open ENStream in their browser. Scan the code with your phone's camera, or type the address." }),
      main ? h("div", { class: "qr-wrap" }, h("img", { class: "qr", src: "/api/v1/system/qr" + query({ url: main.url }), alt: `QR code for ${main.url}` }),
        h("div", null, addrs.map((x) => h("div", { class: "addr" }, icon("wifi", "sm"), h("a", { href: x.url, target: "_blank", rel: "noopener", text: x.url }),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Copy ${x.url}`, onclick: () => copyText(x.url, "Address copied") }, icon("copy", "sm")))),
          h("p", { class: "hint section", text: "ENStream keeps running in the system tray when you close the window, so phones can still watch." })))
        : h("p", { class: "compat warn section" }, icon("info"), "This computer isn't connected to a network yet. Connect it to Wi-Fi or Ethernet, then find the address in Settings."),
      foot(() => go(1), finish, "Start watching"),
    ];
  }

  const views = [folders, pin, phones];
  async function go(n) {
    step = n;
    [...steps.children].forEach((s, i) => s.classList.toggle("on", i <= step));
    try {
      swap(card, await views[step]());
    } catch (err) { toastError(err); }
  }
  await go(0);

  return { el: h("div", { class: "welcome" }, steps, h("section", { class: "panel" }, card)), title: "Welcome", destroy: () => { clearInterval(timer); cleanups.forEach((f) => f()); } };
}
