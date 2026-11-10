//! Desktop-only preferences (desktop.json in the app's config folder). Library settings live
//! in the server; these are about the window, the tray, start-up and updates.
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use tauri::{AppHandle, Manager};

#[derive(Serialize, Deserialize, Clone, Debug)]
#[serde(default)]
pub struct DesktopSettings {
    pub close_to_tray: bool,
    pub auto_update: bool,
    pub last_update_check: i64,
    /// Autostart is switched on once, on the very first start (a home server should be
    /// running when the TV is), and is the user's choice after that.
    pub first_run_done: bool,
}

impl Default for DesktopSettings {
    fn default() -> Self {
        Self {
            close_to_tray: true,
            auto_update: true,
            last_update_check: 0,
            first_run_done: false,
        }
    }
}

fn file(app: &AppHandle) -> Option<PathBuf> {
    app.path()
        .app_config_dir()
        .ok()
        .map(|d| d.join("desktop.json"))
}

pub fn load(app: &AppHandle) -> DesktopSettings {
    file(app)
        .and_then(|p| std::fs::read(p).ok())
        .and_then(|b| serde_json::from_slice(&b).ok())
        .unwrap_or_default()
}

pub fn save(app: &AppHandle, s: &DesktopSettings) -> Result<(), String> {
    let p = file(app).ok_or("no config folder")?;
    if let Some(dir) = p.parent() {
        std::fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    let tmp = p.with_extension("json.tmp");
    std::fs::write(
        &tmp,
        serde_json::to_vec_pretty(s).map_err(|e| e.to_string())?,
    )
    .map_err(|e| e.to_string())?;
    std::fs::rename(&tmp, &p).map_err(|e| e.to_string())
}
