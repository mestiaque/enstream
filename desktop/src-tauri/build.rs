// The window loads the ENStream server's own page (http://127.0.0.1:<port>), a "remote"
// origin for Tauri. Declaring the app's commands here turns each into a permission
// (allow-<command>), so the capabilities can grant exactly these to that page.
fn main() {
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            "pick_folder",
            "open_external",
            "open_app_folder",
            "desktop_settings",
            "set_desktop_setting",
            "app_info",
            "check_update",
            "install_update",
            "restart_app",
            "server_status",
            "restart_server",
            "quit_app",
        ]),
    ))
    .expect("tauri build script failed");
}
