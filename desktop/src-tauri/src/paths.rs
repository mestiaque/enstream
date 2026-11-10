//! The data folder is shared with the ENStream server and must be computed the same way
//! (internal/config/paths.go: DataDir).
use std::path::PathBuf;

pub fn data_dir() -> PathBuf {
    if let Some(d) = std::env::var_os("ENSTREAM_DATA_DIR").filter(|d| !d.is_empty()) {
        return PathBuf::from(d);
    }
    if cfg!(windows) {
        if let Some(d) = std::env::var_os("LOCALAPPDATA").filter(|d| !d.is_empty()) {
            return PathBuf::from(d).join("ENStream");
        }
    }
    if let Some(d) = std::env::var_os("XDG_DATA_HOME").filter(|d| !d.is_empty()) {
        return PathBuf::from(d).join("enstream");
    }
    if let Some(h) = std::env::var_os(if cfg!(windows) { "USERPROFILE" } else { "HOME" }) {
        return PathBuf::from(h)
            .join(".local")
            .join("share")
            .join("enstream");
    }
    std::env::temp_dir().join("enstream")
}

pub fn logs_dir() -> PathBuf {
    data_dir().join("logs")
}
