// Development-phase status page: shows that the server, database and FFmpeg are ready.
(async () => {
  const $ = (id) => document.getElementById(id);
  const set = (id, text, cls) => {
    const el = $(id);
    el.textContent = text;
    if (cls) el.className = cls;
  };
  try {
    const res = await fetch("/api/v1/system/info");
    const body = await res.json();
    if (!body.success) throw new Error(body.error?.message || "error");
    const d = body.data;
    set("status", "Online", "ok");
    set("version", d.version);
    for (const t of ["ffmpeg", "ffprobe"]) {
      set(t, d[t].available ? `Available ${d[t].version || ""}` : "Missing", d[t].available ? "ok" : "bad");
    }
    const list = $("urls");
    const urls = d.addresses.length ? d.addresses : [{ url: location.origin, interface: "this computer" }];
    for (const a of urls) {
      const li = document.createElement("li");
      const link = document.createElement("a");
      link.href = a.url;
      link.textContent = a.url;
      li.append(link, document.createTextNode(` (${a.interface})`));
      list.append(li);
    }
  } catch (e) {
    set("status", "Not reachable", "bad");
  }
})();
