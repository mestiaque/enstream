// Applies the saved theme and accent before the page paints (no flash). Same themes as
// ENViewer, ENSave, ENPlayer and ENPost: dark, light, glass, system.
(function () {
  var p = {};
  try { p = JSON.parse(localStorage.getItem("enstream.prefs") || "{}"); } catch (e) {}
  var theme = p.theme || "dark";
  var dark = theme === "system" ? !(window.matchMedia && matchMedia("(prefers-color-scheme: light)").matches) : theme !== "light";
  var c = document.documentElement.classList;
  c.remove("dark", "light", "glass");
  c.add(dark ? "dark" : "light");
  if (theme === "glass") c.add("glass");
  var hex = /^#[0-9a-f]{6}$/i.test(p.accent || "") ? p.accent : "#3b82f6";
  var n = parseInt(hex.slice(1), 16);
  document.documentElement.style.setProperty("--accent", (n >> 16) + " " + ((n >> 8) & 255) + " " + (n & 255));
})();
