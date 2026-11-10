// Fetch wrapper for the /api/v1 envelope: { success, data, message, pagination, error }.
const TOKEN_KEY = "enstream.device";
const TOKEN_RE = /^[A-Za-z0-9_-]{16,64}$/;
let memoryToken = "";

function newToken() {
  const bytes = new Uint8Array(18);
  crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// A random id per browser so playback history can say which device played what.
export function deviceToken() {
  try {
    let t = localStorage.getItem(TOKEN_KEY);
    if (!t || !TOKEN_RE.test(t)) {
      t = newToken();
      localStorage.setItem(TOKEN_KEY, t);
    }
    return t;
  } catch {
    return (memoryToken ||= newToken());
  }
}

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function request(path, { method = "GET", body, signal, keepalive } = {}) {
  const headers = { "X-Device-Token": deviceToken(), Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  let res;
  try {
    res = await fetch("/api/v1" + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal, keepalive });
  } catch (err) {
    if (err.name === "AbortError") throw err;
    throw new ApiError(0, "OFFLINE", "Can't reach the ENStream server. Is it still running?");
  }
  let env;
  try {
    env = await res.json();
  } catch {
    throw new ApiError(res.status, "BAD_RESPONSE", `The server answered ${res.status} without JSON`);
  }
  if (!env.success) {
    if (res.status === 401 && env.error?.code === "AUTH_REQUIRED") window.dispatchEvent(new CustomEvent("enstream:auth-required"));
    throw new ApiError(res.status, env.error?.code || "ERROR", env.error?.message || "Request failed");
  }
  return env;
}

export const get = async (path, opts) => (await request(path, opts)).data;
export const send = (method, path, body, opts) => request(path, { ...opts, method, body });

export function query(params) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== null && v !== "") q.set(k, v);
  const s = q.toString();
  return s ? "?" + s : "";
}

// For page unload: sendBeacon can't set headers, so the token travels in the body.
export function beacon(path, data) {
  const payload = JSON.stringify({ ...data, device_token: deviceToken() });
  if (navigator.sendBeacon?.("/api/v1" + path, new Blob([payload], { type: "application/json" }))) return;
  fetch("/api/v1" + path, { method: "POST", body: payload, keepalive: true, headers: { "Content-Type": "application/json" } }).catch(() => {});
}

export const thumbURL = (v) => `/api/v1/videos/${v.id}/thumbnail?v=${v.updated_at || 0}`;
export const streamURL = (id) => `/api/v1/videos/${id}/stream`;
