// Small DOM helpers. Text always goes in as text nodes, never as HTML, so file
// names and metadata can't inject markup.
const SVG = "http://www.w3.org/2000/svg";
const SPRITE = "/static/assets/icons.svg";

export function h(tag, props, ...children) {
  const el = document.createElement(tag);
  if (props) {
    for (const [k, v] of Object.entries(props)) {
      if (v === undefined || v === null || v === false) continue;
      if (k === "class") el.className = v;
      else if (k === "text") el.textContent = v;
      else if (k === "dataset") Object.assign(el.dataset, v);
      else if (k.startsWith("on") && typeof v === "function") el.addEventListener(k.slice(2).toLowerCase(), v);
      else if (k in el && typeof v !== "string") el[k] = v;
      else el.setAttribute(k, v === true ? "" : String(v));
    }
  }
  append(el, children);
  return el;
}

export function append(el, children) {
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}

export function icon(name, cls = "") {
  const svg = document.createElementNS(SVG, "svg");
  svg.setAttribute("class", ("icon " + cls).trim());
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS(SVG, "use");
  use.setAttribute("href", `${SPRITE}#i-${name}`);
  svg.append(use);
  return svg;
}

export function setIcon(svg, name) {
  svg.firstChild.setAttribute("href", `${SPRITE}#i-${name}`);
}

export function clear(el) {
  while (el.firstChild) el.firstChild.remove();
  return el;
}

// ------------------------------------------------------------------ formatting
export function fmtDuration(sec) {
  if (!Number.isFinite(sec) || sec < 0) sec = 0;
  sec = Math.floor(sec);
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
  const pad = (n) => String(n).padStart(2, "0");
  return h ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

export function fmtLongDuration(sec) {
  if (!sec) return "0 min";
  const h = Math.floor(sec / 3600), m = Math.round((sec % 3600) / 60);
  if (h >= 48) return `${Math.round(h / 24)} days`;
  return h ? `${h} h ${m} min` : `${m} min`;
}

export function fmtSize(bytes) {
  if (!bytes) return "0 B";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (bytes >= 1024 && i < u.length - 1) { bytes /= 1024; i++; }
  return `${bytes >= 100 || i === 0 ? Math.round(bytes) : bytes.toFixed(1)} ${u[i]}`;
}

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
export function fmtAgo(unix) {
  if (!unix) return "never";
  const diff = unix - Date.now() / 1000;
  const abs = Math.abs(diff);
  const steps = [[60, "second"], [3600, "minute"], [86400, "hour"], [604800, "day"], [2629800, "week"], [31557600, "month"], [Infinity, "year"]];
  let prev = 1;
  for (const [limit, unit] of steps) {
    if (abs < limit) return abs < 45 ? "just now" : rtf.format(Math.round(diff / prev), unit);
    prev = limit;
  }
  return "";
}

export function fmtDate(unix) {
  return unix ? new Date(unix * 1000).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : "—";
}

export function fmtNumber(n) {
  return new Intl.NumberFormat().format(n || 0);
}

export function resolutionLabel(w, hgt) {
  const p = Math.min(w || 0, hgt || 0) || hgt || 0;
  if (!p) return "";
  if (p >= 2000) return "4K";
  if (p >= 1400) return "1440p";
  if (p >= 1000) return "1080p";
  if (p >= 700) return "720p";
  return `${p}p`;
}

export function debounce(fn, ms) {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
}

export function isTyping(e) {
  const t = e.target;
  return t instanceof HTMLElement && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName));
}

// Like el.replaceChildren(), but skips null/false (replaceChildren would print "null").
export function swap(el, ...children) {
  clear(el);
  return append(el, children);
}
