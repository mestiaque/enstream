import { h, icon, fmtNumber, swap } from "../dom.js";
import { request, send, query } from "../api.js";
import { pageHead, errorState, emptyState, link, confirmDialog, toast, toastError } from "../ui.js";
import { isHost } from "../role.js";
import { videoCollection, videoRow, pagination, skeletonGrid } from "../components.js";

const PAGE_SIZE = 48;

export async function render(ctx) {
  const cont = ctx.key === "continue";
  let page = Math.max(1, parseInt(ctx.query.get("page") || "1", 10) || 1);
  const results = h("div");
  const clearBtn = h("button", { class: "btn danger", type: "button", hidden: true, onclick: clearAll }, icon("trash-2", "sm"), "Clear history");

  async function clearAll() {
    const ok = await confirmDialog({ title: "Clear all history?", message: "Resume positions and watched marks on every device are removed. Your video files are not touched.", confirm: "Clear history", danger: true });
    if (!ok) return;
    try {
      await send("DELETE", "/history");
      toast("History cleared", { type: "success" });
      load();
    } catch (err) { toastError(err); }
  }

  async function remove(v) {
    try {
      await send("DELETE", `/history/${v.id}`);
      toast(`Removed “${v.title}” from history`, { type: "success" });
      load();
    } catch (err) { toastError(err); }
  }

  async function load() {
    swap(results, skeletonGrid(8));
    let env;
    try {
      env = await request((cont ? "/continue-watching" : "/history") + query({ page, limit: PAGE_SIZE }));
    } catch (err) {
      swap(results, errorState(err, load));
      return;
    }
    const p = env.pagination;
    clearBtn.hidden = cont || !p.total || !isHost(); // history is shared: clearing it all is host-only
    if (!env.data.length) {
      swap(results, cont
        ? emptyState({ iconName: "circle-play", title: "Nothing in progress", text: "Start a video and stop part-way: it shows up here on every device, ready to resume.", actions: [link("/library", { class: "btn primary" }, icon("film", "sm"), "Browse library")] })
        : emptyState({ iconName: "history", title: "No history yet", text: "Videos you play are listed here with the device they were played on." }));
      return;
    }
    if (page > p.total_pages) {
      page = p.total_pages;
      return load();
    }
    const go = (n) => { page = n; ctx.setQuery({ page: n > 1 ? n : "" }); load(); window.scrollTo({ top: 0, behavior: "smooth" }); };
    const label = cont ? "Continue watching" : "History";
    let body;
    if (cont) {
      body = videoCollection(env.data, { label, meta: "played", mode: "grid" });
    } else {
      body = h("div", { class: "list" }, env.data.map((v) => videoRow(v, {
        list: env.data, label, meta: "played",
        side: [
          h("span", { class: "hide-sm", title: v.device_name ? `Played on ${v.device_name}` : "" }, v.device_name ? [icon("smartphone", "sm"), " ", v.device_name] : null),
          v.completed ? h("span", { class: "badge ok" }, icon("check", "sm"), "Watched") : h("span", { class: "badge info", text: `${Math.round((v.position / (v.duration || 1)) * 100)}%` }),
          h("button", { class: "icon-btn", type: "button", "aria-label": `Remove ${v.title} from history`, title: "Remove from history", onclick: () => remove(v) }, icon("x")),
        ],
      })));
    }
    swap(results, h("p", { class: "muted", text: `${fmtNumber(p.total)} ${p.total === 1 ? "video" : "videos"}` }), h("div", { class: "section" }, body), pagination(p, go));
  }
  load();
  const el = h("div", null, pageHead(cont ? "Continue Watching" : "History", cont ? "Pick up where you left off, on any device." : "Everything played on this server, newest first.", [clearBtn]), results);
  return { el, title: cont ? "Continue Watching" : "History" };
}
