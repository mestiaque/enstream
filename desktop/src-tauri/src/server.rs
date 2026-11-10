//! Runs the bundled ENStream server (`enstream-server -desktop`) as a child process.
//!
//! * It prints `ENSTREAM_READY http://127.0.0.1:<port>` once listening; the window then
//!   loads that page.
//! * We hold its stdin open. When this app exits, even after a crash, the pipe closes and
//!   the server shuts down cleanly (it also gets killed if it hasn't stopped in time).
//! * If it exits on its own, the start-up screen shows the reason and a retry button.

use parking_lot::Mutex;
use serde::Serialize;
use std::collections::VecDeque;
use std::io::{BufRead, BufReader};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU16, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager, Url};

#[derive(Serialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum State {
    Starting,
    Ready,
    Failed,
}

#[derive(Serialize, Clone)]
pub struct Status {
    pub state: State,
    pub url: Option<String>,
    pub error: Option<String>,
}

#[derive(Default)]
pub struct Server {
    child: Mutex<Option<(Child, Option<ChildStdin>)>>,
    status: Mutex<Option<Status>>,
    /// Port of the running server: the only http origin the window may show.
    pub port: Arc<AtomicU16>,
    stopping: AtomicBool,
    generation: AtomicU16,
}

fn sidecar_path() -> std::io::Result<std::path::PathBuf> {
    let exe = std::env::current_exe()?;
    let dir = exe
        .parent()
        .ok_or_else(|| std::io::Error::other("no program folder"))?;
    let name = if cfg!(windows) {
        "enstream-server.exe"
    } else {
        "enstream-server"
    };
    Ok(dir.join(name))
}

impl Server {
    pub fn status(&self) -> Status {
        self.status.lock().clone().unwrap_or(Status {
            state: State::Starting,
            url: None,
            error: None,
        })
    }

    pub fn url(&self) -> Option<String> {
        self.status.lock().as_ref().and_then(|s| s.url.clone())
    }

    fn set(&self, state: State, url: Option<String>, error: Option<String>) {
        *self.status.lock() = Some(Status { state, url, error });
    }

    pub fn start(&self, app: &AppHandle) -> Result<(), String> {
        self.stop();
        self.stopping.store(false, Ordering::SeqCst);
        self.set(State::Starting, None, None);
        let gen = self
            .generation
            .fetch_add(1, Ordering::SeqCst)
            .wrapping_add(1);
        let path = sidecar_path().map_err(|e| e.to_string())?;
        let mut cmd = Command::new(&path);
        cmd.arg("-desktop")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            cmd.creation_flags(0x0800_0000); // CREATE_NO_WINDOW
        }
        let mut child = cmd.spawn().map_err(|e| {
            let msg = format!("Could not start {}: {e}", path.display());
            self.set(State::Failed, None, Some(msg.clone()));
            msg
        })?;
        log::info!("server started (pid {})", child.id());
        let stdout = child.stdout.take();
        let stderr = child.stderr.take();
        let stdin = child.stdin.take();
        *self.child.lock() = Some((child, stdin));

        // Last lines of the server's log output, for the error screen.
        let tail: Arc<Mutex<VecDeque<String>>> = Arc::new(Mutex::new(VecDeque::new()));
        if let Some(err) = stderr {
            let tail = tail.clone();
            std::thread::spawn(move || {
                for line in BufReader::new(err).lines().map_while(Result::ok) {
                    log::debug!("server: {line}");
                    let mut t = tail.lock();
                    if t.len() >= 6 {
                        t.pop_front();
                    }
                    t.push_back(line);
                }
            });
        }
        if let Some(out) = stdout {
            let app = app.clone();
            std::thread::spawn(move || {
                for line in BufReader::new(out).lines().map_while(Result::ok) {
                    if let Some(url) = line.strip_prefix("ENSTREAM_READY ") {
                        let url = url.trim().to_string();
                        let server = app.state::<Server>();
                        if let Ok(u) = Url::parse(&url) {
                            server.port.store(u.port().unwrap_or(80), Ordering::SeqCst);
                            server.set(State::Ready, Some(url.clone()), None);
                            log::info!("server ready at {url}");
                            if let Some(w) = app.get_webview_window("main") {
                                let _ = w.navigate(u);
                            }
                            crate::tray::refresh(&app);
                        }
                    } else if !line.trim().is_empty() {
                        log::info!("server: {line}");
                    }
                }
            });
        }
        // Watch for an unexpected exit.
        let app = app.clone();
        std::thread::spawn(move || loop {
            std::thread::sleep(Duration::from_millis(300));
            let server = app.state::<Server>();
            if server.generation.load(Ordering::SeqCst) != gen {
                return;
            }
            let exited = {
                let mut guard = server.child.lock();
                match guard.as_mut() {
                    Some((c, _)) => c.try_wait().ok().flatten(),
                    None => return,
                }
            };
            if let Some(code) = exited {
                *server.child.lock() = None;
                if server.stopping.load(Ordering::SeqCst) {
                    return;
                }
                let lines: Vec<String> = tail.lock().iter().cloned().collect();
                let detail = lines
                    .iter()
                    .rev()
                    .find(|l| {
                        l.contains("could not start") || l.contains("ERROR") || l.contains("cannot")
                    })
                    .cloned()
                    .or_else(|| lines.last().cloned())
                    .unwrap_or_default();
                let msg = format!("The server stopped ({code}). {}", clean(&detail));
                log::error!("{msg}");
                server.port.store(0, Ordering::SeqCst);
                server.set(State::Failed, None, Some(msg));
                if let Some(w) = app.get_webview_window("main") {
                    if let Ok(u) = start_page(&w) {
                        let _ = w.navigate(u);
                    }
                }
                crate::tray::refresh(&app);
                return;
            }
        });
        Ok(())
    }

    /// Close stdin (the server shuts down by itself), wait up to 6 s, then kill.
    pub fn stop(&self) {
        self.stopping.store(true, Ordering::SeqCst);
        let taken = self.child.lock().take();
        if let Some((mut child, stdin)) = taken {
            drop(stdin);
            let deadline = Instant::now() + Duration::from_secs(6);
            while Instant::now() < deadline {
                if let Ok(Some(_)) = child.try_wait() {
                    log::info!("server stopped");
                    return;
                }
                std::thread::sleep(Duration::from_millis(100));
            }
            log::warn!("server did not stop in time; killing it");
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

/// Drop slog's "time=… level=… msg=" noise from an error line.
fn clean(line: &str) -> String {
    match line.find("msg=") {
        Some(i) => line[i + 4..].replace('"', ""),
        None => line.to_string(),
    }
}

/// The bundled start-up page (tauri://localhost/ or http://tauri.localhost/ on Windows).
pub fn start_page(w: &tauri::WebviewWindow) -> Result<Url, String> {
    let base = if cfg!(windows) {
        "http://tauri.localhost/index.html"
    } else {
        "tauri://localhost/index.html"
    };
    let _ = w;
    Url::parse(base).map_err(|e| e.to_string())
}
