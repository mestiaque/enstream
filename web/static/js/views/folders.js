import { h, icon, fmtNumber, fmtSize, fmtAgo, fmtDate, swap, clear } from "../dom.js";
import { get, send } from "../api.js";
import { pageHead, errorState, emptyState, confirmDialog, toast, toastError, loading } from "../ui.js";
import { pollScan, scanStatus, refreshCount } from "../shell.js";

const STATUS = { running: ["info", "Running"], completed: ["ok", "Completed"], cancelled: ["warn", "Cancelled"], failed: ["bad", "Failed"] };

function statusBadge(s) {
  const [cls, label] = STATUS[s] || ["", s];
  return h("span", { class: `badge ${cls}`, text: label });
}

function durationOf(j) {
  const end = j.completed_at || Math.floor(Date.now() / 1000);
  const s = Math.max(0, end - j.started_at);
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${s % 60} s`;
}

export async function render() {
  const listBox = h("div", { class: "folders" }, loading());
  const scanBox = h("div", { class: "panel scan-card" });
  const jobsBox = h("div");
  let folders = [];

  // ------------------------------------------------------------------ add form
  const pathInput = h("input", { class: "input mono", id: "folder-path", placeholder: navigator.platform?.startsWith("Win") ? "D:\\Movies" : "/home/you/Videos", required: true, autocomplete: "off", spellcheck: "false", maxlength: "1024" });
  const nameInput = h("input", { class: "input", id: "folder-name", placeholder: "Shown in the library", autocomplete: "off", maxlength: "80" });
  const addBtn = h("button", { class: "btn primary", type: "submit" }, icon("folder-plus", "sm"), "Add folder");
  const form = h("form", { class: "add-folder" },
    h("div", { class: "field" }, h("label", { for: "folder-path", text: "Folder path on the server computer" }), pathInput),
    h("div", { class: "field" }, h("label", { for: "folder-name", text: "Name (optional)" }), nameInput),
    addBtn);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const path = pathInput.value.trim();
    if (!path) return pathInput.focus();
    addBtn.disabled = true;
    try {
      const env = await send("POST", "/folders", { path, name: nameInput.value.trim() });
      toast(`Added “${env.data.name}”`, { type: "success", detail: env.message || "Scanning it now…" });
      pathInput.value = "";
      nameInput.value = "";
      await loadFolders();
      pollScan();
    } catch (err) {
      toastError(err);
      pathInput.focus();
    } finally {
      addBtn.disabled = false;
    }
  });

  // ------------------------------------------------------------------ folders
  async function scan(folder) {
    try {
      await send("POST", "/scanner/scan", folder ? { folder_id: folder.id } : {});
      toast(folder ? `Scanning “${folder.name}”` : "Scanning all folders");
      pollScan();
    } catch (err) { toastError(err); }
  }

  async function remove(f) {
    const ok = await confirmDialog({ title: `Remove “${f.name}”?`, message: `Its ${fmtNumber(f.video_count)} videos leave the library along with their thumbnails and history. The files in ${f.path} are not deleted.`, confirm: "Remove folder", danger: true });
    if (!ok) return;
    try {
      await send("DELETE", `/folders/${f.id}`);
      toast(`Removed “${f.name}”`, { type: "success" });
      loadFolders();
      refreshCount();
    } catch (err) { toastError(err); }
  }

  async function loadFolders() {
    try {
      folders = await get("/folders");
    } catch (err) {
      swap(listBox, errorState(err, loadFolders));
      return;
    }
    if (!folders.length) {
      swap(listBox, emptyState({ iconName: "folder-plus", title: "No folders yet", text: "Add the folder where you keep your videos. Subfolders are included automatically." }));
      return;
    }
    const running = scanStatus()?.running;
    swap(listBox, ...folders.map((f) => h("div", { class: `folder ${f.available ? "" : "offline"}` },
      h("div", { class: "folder-icon" }, icon(f.available ? "folder" : "circle-alert")),
      h("div", { class: "row-main" },
        h("div", { class: "folder-name" }, f.name, f.available ? null : h("span", { class: "badge bad", text: "Not reachable" })),
        h("div", { class: "folder-path", title: f.path, text: f.path }),
        h("div", { class: "folder-meta" }, h("span", { text: `${fmtNumber(f.video_count)} videos` }), h("span", { text: fmtSize(f.total_size) }),
          h("span", { text: f.last_scanned_at ? `Scanned ${fmtAgo(f.last_scanned_at)}` : "Not scanned yet" }))),
      h("div", { class: "folder-actions" },
        h("button", { class: "btn sm", type: "button", disabled: running || !f.available, onclick: () => scan(f) }, icon("refresh-cw", "sm"), "Scan"),
        h("button", { class: "icon-btn", type: "button", "aria-label": `Remove ${f.name}`, title: "Remove from library", disabled: running, onclick: () => remove(f) }, icon("trash-2"))))));
  }

  // ------------------------------------------------------------------ scan status
  function drawScan(s) {
    const j = s?.job;
    const running = !!s?.running;
    const actions = h("div", { class: "actions" });
    if (running) {
      actions.append(h("button", { class: "btn danger sm", type: "button", onclick: async () => {
        try { await send("POST", "/scanner/cancel"); toast("Cancelling scan…"); } catch (err) { toastError(err); }
      } }, icon("square-x", "sm"), "Cancel"));
    } else {
      if (s?.failed_items) actions.append(h("button", { class: "btn sm", type: "button", onclick: async () => {
        try { await send("POST", "/scanner/retry-failed"); toast("Retrying failed files"); pollScan(); } catch (err) { toastError(err); }
      } }, icon("rotate-ccw", "sm"), `Retry ${s.failed_items} failed`));
      actions.append(h("button", { class: "btn primary sm", type: "button", disabled: !folders.length, onclick: () => scan(null) }, icon("refresh-cw", "sm"), "Scan all"));
    }
    const head = h("div", { class: "scan-top" }, h("div", { class: "scan-title" }, icon(running ? "loader-circle" : "server", running ? "spin" : ""),
      running ? "Scanning your folders" : j ? "Last scan" : "Scanner", j && !running ? statusBadge(j.status) : null), actions);
    if (!j) {
      swap(scanBox, head, h("p", { class: "muted section", text: "No scan has run yet. Adding a folder starts one automatically." }));
      return;
    }
    const done = j.processed_files + j.skipped_files;
    const bar = h("i");
    const barBox = h("div", { class: `bar ${running && !j.total_files ? "indeterminate" : ""}` }, bar);
    bar.style.width = `${j.total_files ? Math.min(100, (done / j.total_files) * 100) : running ? 35 : 100}%`;
    const num = (n, label) => h("span", null, h("b", { text: fmtNumber(n) }), ` ${label}`);
    swap(scanBox, head, barBox,
      h("div", { class: "scan-numbers" }, num(done, `of ${fmtNumber(j.total_files)} files`), num(j.new_files, "new"), num(j.updated_files, "updated"),
        num(j.skipped_files, "unchanged"), num(j.removed_files, "removed"), num(j.failed_files, "failed"), h("span", { text: running ? `${durationOf(j)} so far` : `${durationOf(j)} · ${fmtAgo(j.completed_at)}` })),
      running && j.current_file ? h("div", { class: "scan-file", title: j.current_file, text: j.current_file }) : null,
      !running && j.error_message ? h("p", { class: "compat bad section" }, icon("circle-alert"), j.error_message) : null);
  }

  async function loadJobs() {
    try {
      const jobs = await get("/scanner/jobs?limit=8");
      if (!jobs.length) return clear(jobsBox);
      swap(jobsBox, h("section", { class: "section panel" }, h("div", { class: "panel-head" }, h("h2", { text: "Recent scans" })),
        h("div", { class: "table-wrap" }, h("table", { class: "table" },
          h("thead", null, h("tr", null, ["Started", "Status", "Files", "New", "Updated", "Removed", "Failed", "Took"].map((t) => h("th", { text: t })))),
          h("tbody", null, jobs.map((j) => h("tr", null, h("td", { text: fmtDate(j.started_at) }), h("td", null, statusBadge(j.status)), h("td", { text: fmtNumber(j.total_files) }),
            h("td", { text: fmtNumber(j.new_files) }), h("td", { text: fmtNumber(j.updated_files) }), h("td", { text: fmtNumber(j.removed_files) }), h("td", { text: fmtNumber(j.failed_files) }),
            h("td", { text: durationOf(j) }))))))));
    } catch { /* optional */ }
  }

  let wasRunning = false;
  const onScan = (e) => {
    const s = e.detail;
    drawScan(s);
    if (s.running !== wasRunning) {
      wasRunning = s.running;
      loadFolders();
      loadJobs();
    }
  };
  window.addEventListener("enstream:scan", onScan);
  drawScan(scanStatus());
  wasRunning = !!scanStatus()?.running;
  await loadFolders();
  loadJobs();
  pollScan();

  const el = h("div", null,
    pageHead("Folders & scan", "ENStream reads videos from these folders. Files are never moved, renamed or deleted."),
    h("section", { class: "panel" }, h("div", { class: "panel-head" }, h("h2", { text: "Add a folder" })), h("div", { class: "panel-body" }, form,
      h("p", { class: "field hint section", text: "Type the full path as it is on the computer running ENStream (USB drives and network shares work while they are connected)." }))),
    h("section", { class: "section" }, scanBox),
    h("section", { class: "section" }, h("div", { class: "section-head" }, h("h2", null, icon("folder"), "Library folders")), listBox),
    jobsBox);
  return { el, title: "Folders", destroy: () => window.removeEventListener("enstream:scan", onScan) };
}
