//! Tray icon. ENStream keeps serving phones and TVs while the window is closed; the tray
//! menu opens it again, copies the address for phones, or quits.
use crate::server::Server;
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Manager};
use tauri_plugin_clipboard_manager::ClipboardExt;
use tauri_plugin_opener::OpenerExt;

fn build_menu(app: &AppHandle) -> tauri::Result<Menu<tauri::Wry>> {
    let ready = app.state::<Server>().url().is_some();
    let status = MenuItem::with_id(
        app,
        "status",
        if ready {
            "ENStream is running"
        } else {
            "ENStream is starting…"
        },
        false,
        None::<&str>,
    )?;
    let open = MenuItem::with_id(app, "open", "Open ENStream", true, None::<&str>)?;
    let browser = MenuItem::with_id(app, "browser", "Open in browser", ready, None::<&str>)?;
    let copy = MenuItem::with_id(app, "copy", "Copy address for phones", ready, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "Quit ENStream", true, None::<&str>)?;
    Menu::with_items(
        app,
        &[
            &status,
            &PredefinedMenuItem::separator(app)?,
            &open,
            &browser,
            &copy,
            &PredefinedMenuItem::separator(app)?,
            &quit,
        ],
    )
}

pub fn install(app: &AppHandle) -> tauri::Result<()> {
    let menu = build_menu(app)?;
    let mut builder = TrayIconBuilder::with_id("main")
        .tooltip("ENStream")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id.as_ref() {
            "open" => show_main(app),
            "browser" => {
                if let Some(url) = app.state::<Server>().url() {
                    let _ = app
                        .opener()
                        .open_url(url.replace("127.0.0.1", "localhost"), None::<&str>);
                }
            }
            "copy" => {
                let app = app.clone();
                std::thread::spawn(move || match lan_address(&app) {
                    Some(url) => {
                        let _ = app.clipboard().write_text(url);
                    }
                    None => log::warn!("no network address to copy"),
                });
            }
            "quit" => crate::quit(app),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                show_main(tray.app_handle());
            }
        });
    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }
    builder.build(app)?;
    Ok(())
}

/// Update the menu after the server started or stopped.
pub fn refresh(app: &AppHandle) {
    if let (Some(tray), Ok(menu)) = (app.tray_by_id("main"), build_menu(app)) {
        let _ = tray.set_menu(Some(menu));
    }
}

/// The first private network address the server reports (what phones should open).
pub fn lan_address(app: &AppHandle) -> Option<String> {
    let base = app.state::<Server>().url()?;
    let info: serde_json::Value = ureq::get(&format!("{base}/api/v1/system/info"))
        .timeout(std::time::Duration::from_secs(5))
        .call()
        .ok()?
        .into_json()
        .ok()?;
    info["data"]["addresses"]
        .as_array()?
        .iter()
        .find(|a| a["private"].as_bool() == Some(true))
        .and_then(|a| a["url"].as_str())
        .map(String::from)
}

pub fn show_main(app: &AppHandle) {
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.show();
        let _ = w.unminimize();
        let _ = w.set_focus();
    }
}
