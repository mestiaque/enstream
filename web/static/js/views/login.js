import { h, icon } from "../dom.js";
import { request, ApiError } from "../api.js";
import { signedIn } from "../shell.js";

export async function render(ctx) {
  document.body.classList.add("signin");
  const next = ctx.query.get("next") || "/";
  const input = h("input", { class: "input pin-input", id: "pin", type: "password", autocomplete: "current-password", required: true, maxlength: "64", "aria-describedby": "pin-msg", placeholder: "PIN" });
  const msg = h("p", { class: "pin-msg", id: "pin-msg", role: "alert" });
  const btn = h("button", { class: "btn grad lg", type: "submit" }, "Unlock");
  let timer;

  function lock(seconds) {
    clearInterval(timer);
    let left = seconds;
    btn.disabled = true;
    input.disabled = true;
    const tick = () => {
      msg.textContent = `Too many wrong attempts. Try again in ${left} s.`;
      if (left-- <= 0) {
        clearInterval(timer);
        btn.disabled = false;
        input.disabled = false;
        msg.textContent = "";
        input.focus();
      }
    };
    tick();
    timer = setInterval(tick, 1000);
  }

  const form = h("form", { class: "signin-card" },
    h("img", { src: "/static/assets/enstream-icon.svg", alt: "", width: "64", height: "64" }),
    h("h1", { text: "ENStream is locked" }),
    h("p", { class: "muted", text: "Enter the PIN set on the ENStream computer." }),
    h("label", { class: "sr-only", for: "pin", text: "PIN" }), input, msg, btn,
    h("p", { class: "hint", text: "Forgot the PIN? Start ENStream with -reset-pin on its computer." }));
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (!input.value) return input.focus();
    btn.disabled = true;
    msg.textContent = "";
    try {
      await request("/auth/login", { method: "POST", body: { pin: input.value } });
      signedIn(next);
    } catch (err) {
      btn.disabled = false;
      input.value = "";
      input.focus();
      if (err instanceof ApiError && err.code === "TOO_MANY_ATTEMPTS") {
        const m = /(\d+)m(\d+)s|(\d+)s/.exec(err.message);
        lock(m ? (m[1] ? +m[1] * 60 + +m[2] : +m[3]) : 30);
      } else {
        msg.textContent = err.message;
        form.classList.remove("shake");
        void form.offsetWidth;
        form.classList.add("shake");
      }
    }
  });
  setTimeout(() => input.focus(), 0);
  return { el: h("div", { class: "signin" }, form), title: "Locked", destroy: () => clearInterval(timer) };
}
