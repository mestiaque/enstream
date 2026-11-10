// Start-up screen: shown until the bundled server is ready (the app then loads its page),
// or with the error and a retry button when it could not start.
(function () {
  const T = window.__TAURI__;
  if (!T) return;
  const invoke = T.core.invoke;
  const win = T.window.getCurrentWindow();
  const $ = (id) => document.getElementById(id);

  $("min").addEventListener("click", () => win.minimize());
  $("close").addEventListener("click", () => win.close());
  $("titlebar").addEventListener("mousedown", (e) => {
    if (e.button === 0 && !e.target.closest("button")) win.startDragging();
  });
  $("logs").addEventListener("click", () => invoke("open_app_folder", { which: "logs" }));
  $("retry").addEventListener("click", async () => {
    show("starting");
    try {
      await invoke("restart_server");
      poll();
    } catch (e) {
      fail(String(e));
    }
  });

  function show(which) {
    $("starting").hidden = which !== "starting";
    $("failed").hidden = which !== "failed";
  }
  function fail(msg) {
    $("error").textContent = msg || "Unknown error";
    show("failed");
  }

  async function poll() {
    try {
      const s = await invoke("server_status");
      if (s.state === "failed") return fail(s.error);
      if (s.state === "starting") show("starting");
    } catch (e) {
      return fail(String(e));
    }
    setTimeout(poll, 400);
  }
  poll();
})();
