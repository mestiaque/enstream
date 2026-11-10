import { h, icon, fmtNumber, swap } from "../dom.js";
import { get, request, query } from "../api.js";
import { pageHead, errorState, emptyState, link } from "../ui.js";
import { videoCollection, pagination, skeletonGrid } from "../components.js";
import { isHost } from "../role.js";
import { prefs, setPref } from "../store.js";

const SORTS = [
  ["title:asc", "Title A–Z"], ["title:desc", "Title Z–A"], ["created_at:desc", "Newest added"], ["created_at:asc", "Oldest added"],
  ["last_played_at:desc", "Recently played"], ["duration:desc", "Longest"], ["duration:asc", "Shortest"], ["file_size:desc", "Largest"], ["file_size:asc", "Smallest"],
];
const RESOLUTIONS = [["", "Any resolution"], ["4k", "4K"], ["1080p", "1080p"], ["720p", "720p"], ["sd", "SD"]];
const WATCHED = [["", "All videos"], ["unwatched", "Unwatched"], ["in_progress", "In progress"], ["watched", "Watched"]];
const PAGE_SIZE = 48;

function select(options, value, label, onchange) {
  const s = h("select", { class: "select", "aria-label": label, onchange: (e) => onchange(e.target.value) }, options.map(([v, t]) => h("option", { value: v, text: t })));
  s.value = value;
  return s;
}

export async function render(ctx) {
  const recent = ctx.key === "recent";
  const q = ctx.query;
  const state = {
    search: q.get("search") || "",
    folder_id: q.get("folder_id") || "",
    extension: q.get("extension") || "",
    resolution: q.get("resolution") || "",
    watched: q.get("watched") || "",
    sort: q.get("sort") || (recent ? "created_at" : "title"),
    order: q.get("order") || (recent ? "desc" : "asc"),
    page: Math.max(1, parseInt(q.get("page") || "1", 10) || 1),
  };
  const [folders, extensions] = await Promise.all([get("/folders").catch(() => []), get("/videos/extensions").catch(() => [])]);

  const results = h("div");
  const count = h("small");
  const chips = h("div", { class: "toolbar" });
  let abort;

  const viewToggle = h("div", { class: "segmented", role: "group", "aria-label": "Layout" });
  const gridBtn = h("button", { class: "icon-btn", type: "button", "aria-label": "Grid", title: "Grid", onclick: () => setView("grid") }, icon("layout-grid", "sm"));
  const listBtn = h("button", { class: "icon-btn", type: "button", "aria-label": "List", title: "List", onclick: () => setView("list") }, icon("list", "sm"));
  viewToggle.append(gridBtn, listBtn);
  const markView = () => {
    gridBtn.classList.toggle("active", prefs().view !== "list");
    listBtn.classList.toggle("active", prefs().view === "list");
    gridBtn.setAttribute("aria-pressed", String(prefs().view !== "list"));
    listBtn.setAttribute("aria-pressed", String(prefs().view === "list"));
  };
  const setView = (v) => { setPref("view", v); markView(); load(); };
  markView();

  const update = (patch, resetPage = true) => {
    Object.assign(state, patch);
    if (resetPage) state.page = 1;
    const { page, ...rest } = state;
    const defaults = { sort: recent ? "created_at" : "title", order: recent ? "desc" : "asc" };
    ctx.setQuery({ ...rest, sort: rest.sort === defaults.sort && rest.order === defaults.order ? "" : rest.sort, order: rest.sort === defaults.sort && rest.order === defaults.order ? "" : rest.order, page: page > 1 ? page : "" });
    load();
  };

  const folderOpts = [["", "All folders"], ...folders.map((f) => [String(f.id), f.name])];
  const extOpts = [["", "All formats"], ...extensions.map((e) => [e.replace(/^\./, ""), e.replace(/^\./, "").toUpperCase()])];
  const filters = {
    folder_id: folders.length > 1 ? select(folderOpts, state.folder_id, "Folder", (v) => update({ folder_id: v })) : null,
    extension: extensions.length > 1 ? select(extOpts, state.extension, "Format", (v) => update({ extension: v })) : null,
    resolution: select(RESOLUTIONS, state.resolution, "Resolution", (v) => update({ resolution: v })),
    watched: select(WATCHED, state.watched, "Watched", (v) => update({ watched: v })),
  };
  const resetFilter = (k) => {
    if (k === "search") document.getElementById("search").value = "";
    else if (filters[k]) filters[k].value = "";
  };
  const toolbar = h("div", { class: "toolbar" }, Object.values(filters),
    h("div", { class: "grow" }),
    select(SORTS, `${state.sort}:${state.order}`, "Sort", (v) => { const [sort, order] = v.split(":"); update({ sort, order }); }),
    viewToggle);

  function drawChips() {
    const list = [];
    if (state.search) list.push(["search", `“${state.search}”`]);
    if (state.folder_id) list.push(["folder_id", folders.find((f) => String(f.id) === state.folder_id)?.name || "Folder"]);
    if (state.extension) list.push(["extension", state.extension.toUpperCase()]);
    if (state.resolution) list.push(["resolution", RESOLUTIONS.find((r) => r[0] === state.resolution)?.[1]]);
    if (state.watched) list.push(["watched", WATCHED.find((r) => r[0] === state.watched)?.[1]]);
    swap(chips, ...list.map(([k, label]) => h("button", { class: "chip", type: "button", "aria-label": `Remove filter ${label}`,
      onclick: () => { resetFilter(k); update({ [k]: "" }); } }, label, icon("x"))));
    if (list.length > 1) {
      chips.append(h("button", { class: "btn ghost sm", type: "button", text: "Clear all", onclick: () => {
        list.forEach(([k]) => resetFilter(k));
        update({ search: "", folder_id: "", extension: "", resolution: "", watched: "" });
      } }));
    }
    chips.hidden = !list.length;
  }

  async function load() {
    abort?.abort();
    abort = new AbortController();
    drawChips();
    swap(results, skeletonGrid(12));
    const params = { ...state, limit: PAGE_SIZE, recently_added: "" };
    let env;
    try {
      env = await request("/videos" + query(params), { signal: abort.signal });
    } catch (err) {
      if (err.name === "AbortError") return;
      swap(results, errorState(err, load));
      return;
    }
    const p = env.pagination;
    count.textContent = `${fmtNumber(p.total)} ${p.total === 1 ? "video" : "videos"}`;
    if (!env.data.length) {
      const filtered = state.search || state.folder_id || state.extension || state.resolution || state.watched;
      swap(results, filtered ? emptyState({ iconName: "search", title: "No matching videos", text: "Try another search or remove a filter." })
        : isHost() ? emptyState({ iconName: "folder-plus", title: "No videos yet", text: "Add a folder and scan it to fill your library.", actions: [link("/folders", { class: "btn primary" }, icon("folder-plus", "sm"), "Add a folder")] })
          : emptyState({ iconName: "monitor", title: "No videos yet", text: "Videos appear here once folders are added on the computer that runs ENStream." }));
      return;
    }
    if (state.page > p.total_pages) return update({ page: p.total_pages }, false);
    swap(results, videoCollection(env.data, { label: recent ? "Recently added" : "Library", meta: recent ? "added" : "folder" }),
      pagination(p, (n) => { update({ page: n }, false); window.scrollTo({ top: 0, behavior: "smooth" }); }));
  }

  const onSearch = (e) => update({ search: e.detail });
  const onChange = () => load();
  window.addEventListener("enstream:search", onSearch);
  window.addEventListener("enstream:library-changed", onChange);
  load();

  const folderName = folders.find((f) => String(f.id) === state.folder_id)?.name;
  const title = recent ? "Recently Added" : folderName || "All Videos";
  const el = h("div", null, pageHead(title, null, [], count), toolbar, chips, results);
  return {
    el,
    title,
    destroy() {
      abort?.abort();
      window.removeEventListener("enstream:search", onSearch);
      window.removeEventListener("enstream:library-changed", onChange);
    },
  };
}
