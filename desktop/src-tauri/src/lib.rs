//! ENStream desktop app: a window and a tray icon around the bundled ENStream server.
//! The window shows the server's own page, so the desktop and every phone or TV see the
//! same interface; this app adds the native parts (folder picker, tray, start-up, updates).

mod paths;
mod server;
mod settings;
mod tray;
mod update;

use serde::Serialize;
use server::Server;
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{AppHandle, Manager, RunEvent, Url, WebviewUrl, WebviewWindowBuilder, WindowEvent};
use tauri_plugin_autostart::ManagerExt as _;
use tauri_plugin_dialog::DialogExt;
use tauri_plugin_opener::OpenerExt;

/// Set by tray → Quit so the window really closes instead of hiding.
static QUITTING: AtomicBool = AtomicBool::new(false);
const HIDDEN_FLAG: &str = "--hidden";

pub fn quit(app: &AppHandle) {
    QUITTING.store(true, Ordering::SeqCst);
    log::info!("quitting");
    app.exit(0);
}

// ------------------------------------------------------------------ commands

#[tauri::command]
async fn pick_folder(app: AppHandle) -> Result<Option<String>, String> {
    let picked = tauri::async_runtime::spawn_blocking(move || {
        app.dialog()
            .file()
            .set_title("Choose a video folder")
            .blocking_pick_folder()
    })
    .await
    .map_err(|e| e.to_string())?;
    Ok(picked
        .and_then(|p| p.into_path().ok())
        .map(|p| p.to_string_lossy().into_owned()))
}

#[tauri::command]
fn open_external(app: AppHandle, url: String) -> Result<(), String> {
    let u = Url::parse(&url).map_err(|e| e.to_string())?;
    if !matches!(u.scheme(), "http" | "https" | "mailto") {
        return Err("only web and e-mail links can be opened".into());
    }
    app.opener()
        .open_url(u.as_str(), None::<&str>)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn open_app_folder(app: AppHandle, which: String) -> Result<(), String> {
    let dir = match which.as_str() {
        "data" => paths::data_dir(),
        "logs" => paths::logs_dir(),
        "backups" => paths::data_dir().join("backups"),
        _ => return Err("unknown folder".into()),
    };
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    app.opener()
        .open_path(dir.to_string_lossy(), None::<&str>)
        .map_err(|e| e.to_string())
}

#[derive(Serialize)]
struct DesktopView {
    close_to_tray: bool,
    auto_update: bool,
    last_update_check: i64,
    autostart: bool,
}

#[tauri::command]
fn desktop_settings(app: AppHandle) -> DesktopView {
    let s = settings::load(&app);
    DesktopView {
        close_to_tray: s.close_to_tray,
        auto_update: s.auto_update,
        last_update_check: s.last_update_check,
        autostart: app.autolaunch().is_enabled().unwrap_or(false),
    }
}

#[tauri::command]
fn set_desktop_setting(
    app: AppHandle,
    key: String,
    value: serde_json::Value,
) -> Result<(), String> {
    let mut s = settings::load(&app);
    match key.as_str() {
        "autostart" => {
            let on = value.as_bool().ok_or("expected true or false")?;
            let al = app.autolaunch();
            return if on { al.enable() } else { al.disable() }
                .map_err(|e| format!("Could not change start-up: {e}"));
        }
        "close_to_tray" => s.close_to_tray = value.as_bool().ok_or("expected true or false")?,
        "auto_update" => s.auto_update = value.as_bool().ok_or("expected true or false")?,
        "last_update_check" => s.last_update_check = value.as_i64().ok_or("expected a number")?,
        _ => return Err(format!("unknown setting {key}")),
    }
    settings::save(&app, &s)
}

#[derive(Serialize)]
struct AppInfo {
    version: String,
    tauri: &'static str,
    webview: String,
    platform: &'static str,
    arch: &'static str,
    data_dir: String,
}

#[tauri::command]
fn app_info(app: AppHandle) -> AppInfo {
    AppInfo {
        version: app.package_info().version.to_string(),
        platform: std::env::consts::OS,
        arch: std::env::consts::ARCH,
        data_dir: paths::data_dir().to_string_lossy().into_owned(),
    }
}

#[tauri::command]
fn server_status(app: AppHandle) -> server::Status {
    app.state::<Server>().status()
}

#[tauri::command]
async fn restart_server(app: AppHandle) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || app.state::<Server>().start(&app))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
fn quit_app(app: AppHandle) {
    quit(&app);
}

// ------------------------------------------------------------------ window

/// The window may show the bundled start-up page or the local server, nothing else. Other
/// links open in the normal browser.
fn allowed(app: &AppHandle, url: &Url) -> bool {
    match url.scheme() {
        "tauri" => return true,
        "http" if url.host_str() == Some("tauri.localhost") => return true,
        "http" if url.host_str() == Some("127.0.0.1") => {
            let port = app.state::<Server>().port.load(Ordering::SeqCst);
            if port != 0 && url.port() == Some(port) {
                return true;
            }
        }
        "about" | "data" | "blob" => return url.as_str() == "about:blank",
        _ => {}
    }
    if matches!(url.scheme(), "http" | "https") {
        let _ = app.opener().open_url(url.as_str(), None::<&str>);
    }
    false
}

fn create_window(app: &AppHandle, visible: bool) -> tauri::Result<()> {
    let handle = app.clone();
    let w = WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
        .title("ENStream")
        .inner_size(1320.0, 840.0)
        .min_inner_size(860.0, 560.0)
        .center()
        .decorations(false)
        .visible(false)
        .background_color(tauri::window::Color(15, 16, 19, 255))
        .disable_drag_drop_handler()
        .on_navigation(move |url| allowed(&handle, url))
        .build()?;
    fit_to_monitor(&w);
    if visible {
        w.show()?;
        let _ = w.set_focus();
    }
    Ok(())
}

/// Shrink the window on small screens so the whole interface stays visible.
fn fit_to_monitor(w: &tauri::WebviewWindow) {
    let Some(m) = w
        .current_monitor()
        .ok()
        .flatten()
        .or_else(|| w.primary_monitor().ok().flatten())
    else {
        return;
    };
    let scale = m.scale_factor();
    let screen = m.size().to_logical::<f64>(scale);
    let (max_w, max_h) = (screen.width * 0.94, screen.height * 0.9);
    if 1320.0 > max_w || 840.0 > max_h {
        let size = tauri::LogicalSize::new(1320f64.min(max_w), 840f64.min(max_h));
        let _ = w.set_size(size);
        let origin = m.position().to_logical::<f64>(scale);
        let _ = w.set_position(tauri::LogicalPosition::new(
            origin.x + (screen.width - size.width) / 2.0,
            origin.y + (screen.height - size.height) / 2.0,
        ));
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let result = tauri::Builder::default()
        // First: a second launch only brings the running window forward.
        .plugin(tauri_plugin_single_instance::init(|app, argv, _| {
            if !argv.iter().any(|a| a == HIDDEN_FLAG) {
                tray::show_main(app);
            }
        }))
        .plugin(
            tauri_plugin_log::Builder::new()
                .level(if cfg!(debug_assertions) {
                    log::LevelFilter::Debug
                } else {
                    log::LevelFilter::Info
                })
                .targets([
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stdout),
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Folder {
                        path: paths::logs_dir(),
                        file_name: Some("desktop".into()),
                    }),
                ])
                .max_file_size(5 * 1024 * 1024)
                .rotation_strategy(tauri_plugin_log::RotationStrategy::KeepOne)
                .build(),
        )
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_clipboard_manager::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec![HIDDEN_FLAG]),
        ))
        .manage(Server::default())
        .setup(|app| {
            let handle = app.handle().clone();
            let mut s = settings::load(&handle);
            if !s.first_run_done {
                // A home media server should be up when the TV is: start with the computer
                // (in the tray). Settings → Desktop turns this off.
                if let Err(e) = handle.autolaunch().enable() {
                    log::warn!("could not enable start-up: {e}");
                }
                s.first_run_done = true;
                let _ = settings::save(&handle, &s);
            }
            let hidden = std::env::args().any(|a| a == HIDDEN_FLAG);
            create_window(&handle, !hidden)?;
            tray::install(&handle)?;
            std::thread::spawn(move || {
                if let Err(e) = handle.state::<Server>().start(&handle) {
                    log::error!("{e}");
                }
            });
            log::info!(
                "ENStream desktop {} started{}",
                app.package_info().version,
                if hidden { " in the tray" } else { "" }
            );
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                if !QUITTING.load(Ordering::SeqCst)
                    && settings::load(window.app_handle()).close_to_tray
                {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            pick_folder,
            open_external,
            open_app_folder,
            desktop_settings,
            set_desktop_setting,
            app_info,
            server_status,
            restart_server,
            quit_app,
            update::check_update,
            update::install_update,
            update::restart_app,
        ])
        .build(tauri::generate_context!());
    match result {
        Ok(app) => app.run(|app, event| match event {
            RunEvent::Exit => app.state::<Server>().stop(),
            // Closing the last window when "keep running" is off quits the app.
            RunEvent::ExitRequested { .. } => {}
            _ => {}
        }),
        Err(e) => {
            eprintln!("ENStream could not start: {e}");
            std::process::exit(1);
        }
    }
}
