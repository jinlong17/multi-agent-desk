use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    env,
    error::Error,
    fs,
    path::{Path, PathBuf},
    thread,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};
use tauri::Manager;
use tauri_plugin_shell::ShellExt;

type BoxError = Box<dyn Error + Send + Sync>;

#[derive(Debug, Deserialize)]
struct ReadyState {
    schema_version: u32,
    daemon_pid: u32,
    grandchild_pid: u32,
    owner_hash: String,
}

#[derive(Debug, Serialize)]
struct ControlCommand<'a> {
    schema_version: u32,
    id: &'a str,
    action: &'static str,
    owner_token: &'a str,
}

#[derive(Debug, Deserialize)]
struct ControlResult {
    schema_version: u32,
    id: String,
    accepted: bool,
    reason: String,
}

#[derive(Debug, Serialize)]
struct HostResult {
    schema_version: u32,
    mode: String,
    spawned: bool,
    reused: bool,
    preexisting_not_owned: bool,
    wrong_owner_denied: bool,
    sidecar_resolved_pid: Option<u32>,
    daemon_pid: Option<u32>,
    grandchild_pid: Option<u32>,
    error: Option<String>,
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            let handle = app.handle().clone();
            thread::spawn(move || match run_mode(&handle) {
                Ok(should_abort) => {
                    if should_abort {
                        thread::sleep(Duration::from_millis(250));
                        std::process::abort();
                    }
                    handle.exit(0);
                }
                Err(error) => {
                    let _ = write_error_result(&error.to_string());
                    eprintln!("Tauri sidecar host failed: {error}");
                    handle.exit(1);
                }
            });
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("failed to run Tauri sidecar Spike host");
}

fn run_mode(handle: &tauri::AppHandle) -> Result<bool, BoxError> {
    let mode = required_env("MAD_HOST_MODE")?;
    let state_dir = PathBuf::from(required_env("MAD_STATE_DIR")?);
    let owner_token = required_env("MAD_OWNER_TOKEN")?;
    let result_path = PathBuf::from(required_env("MAD_HOST_RESULT")?);
    fs::create_dir_all(&state_dir)?;

    let mut result = HostResult {
        schema_version: 1,
        mode: mode.clone(),
        spawned: false,
        reused: false,
        preexisting_not_owned: false,
        wrong_owner_denied: false,
        sidecar_resolved_pid: None,
        daemon_pid: None,
        grandchild_pid: None,
        error: None,
    };

    match mode.as_str() {
        "spawn-graceful" | "spawn-crash" => {
            if state_dir.join("ready.json").exists() {
                return Err("spawn mode found an existing daemon".into());
            }
            let (_events, child) = handle
                .shell()
                .sidecar("mad-sidecar")?
                .args(["-mode", "daemon", "-state"])
                .arg(&state_dir)
                .env("MAD_OWNER_TOKEN", &owner_token)
                .spawn()?;
            result.spawned = true;
            result.sidecar_resolved_pid = Some(child.pid());
            let ready = wait_ready(&state_dir, Duration::from_secs(5))?;
            validate_ready(&ready, &owner_token)?;
            if child.pid() != ready.daemon_pid {
                return Err(format!(
                    "Tauri child PID {} differs from ready daemon PID {}",
                    child.pid(),
                    ready.daemon_pid
                )
                .into());
            }
            result.daemon_pid = Some(ready.daemon_pid);
            result.grandchild_pid = Some(ready.grandchild_pid);
            if mode == "spawn-graceful" {
                let accepted = send_shutdown(&state_dir, &owner_token, "graceful-owner")?;
                if !accepted.accepted || accepted.reason != "owner_shutdown" {
                    return Err(format!("owner shutdown was not accepted: {accepted:?}").into());
                }
                wait_absent(&state_dir.join("ready.json"), Duration::from_secs(5))?;
            }
        }
        "reconnect-stop" => {
            let ready = wait_ready(&state_dir, Duration::from_secs(5))?;
            validate_ready(&ready, &owner_token)?;
            result.reused = true;
            result.daemon_pid = Some(ready.daemon_pid);
            result.grandchild_pid = Some(ready.grandchild_pid);

            let denied = send_shutdown(
                &state_dir,
                &format!("{owner_token}-not-owner"),
                "wrong-owner",
            )?;
            if denied.accepted || denied.reason != "owner_mismatch" {
                return Err(format!("wrong owner was not denied: {denied:?}").into());
            }
            if !state_dir.join("ready.json").exists() {
                return Err("wrong-owner request stopped the sidecar".into());
            }
            result.wrong_owner_denied = true;

            let accepted = send_shutdown(&state_dir, &owner_token, "reconnect-owner")?;
            if !accepted.accepted || accepted.reason != "owner_shutdown" {
                return Err(format!("reconnected owner shutdown failed: {accepted:?}").into());
            }
            wait_absent(&state_dir.join("ready.json"), Duration::from_secs(5))?;
        }
        "observe-unowned" => {
            let ready = wait_ready(&state_dir, Duration::from_secs(5))?;
            if ready.owner_hash == token_hash(&owner_token) {
                return Err("pre-existing instance unexpectedly belongs to Desktop token".into());
            }
            result.reused = true;
            result.preexisting_not_owned = true;
            result.daemon_pid = Some(ready.daemon_pid);
            result.grandchild_pid = Some(ready.grandchild_pid);
        }
        _ => return Err(format!("unsupported MAD_HOST_MODE {mode:?}").into()),
    }

    write_json(&result_path, &result)?;
    Ok(mode == "spawn-crash")
}

fn validate_ready(ready: &ReadyState, owner_token: &str) -> Result<(), BoxError> {
    if ready.schema_version != 1 {
        return Err(format!("unsupported ready schema {}", ready.schema_version).into());
    }
    if ready.owner_hash != token_hash(owner_token) {
        return Err("ready owner hash mismatch".into());
    }
    if ready.daemon_pid == 0 || ready.grandchild_pid == 0 {
        return Err("ready state contains a zero PID".into());
    }
    Ok(())
}

fn send_shutdown(
    state_dir: &Path,
    owner_token: &str,
    label: &str,
) -> Result<ControlResult, BoxError> {
    let id = format!("{label}-{}", unique_suffix());
    let result_path = state_dir.join("control-result.json");
    let _ = fs::remove_file(&result_path);
    write_json(
        &state_dir.join("control.json"),
        &ControlCommand {
            schema_version: 1,
            id: &id,
            action: "shutdown",
            owner_token,
        },
    )?;
    let deadline = Instant::now() + Duration::from_secs(5);
    loop {
        if let Ok(bytes) = fs::read(&result_path) {
            let result: ControlResult = serde_json::from_slice(&bytes)?;
            if result.id == id {
                if result.schema_version != 1 {
                    return Err("unsupported control-result schema".into());
                }
                return Ok(result);
            }
        }
        if Instant::now() >= deadline {
            return Err(format!("timed out waiting for control result {id}").into());
        }
        thread::sleep(Duration::from_millis(50));
    }
}

fn wait_ready(state_dir: &Path, timeout: Duration) -> Result<ReadyState, BoxError> {
    let path = state_dir.join("ready.json");
    let deadline = Instant::now() + timeout;
    loop {
        if let Ok(bytes) = fs::read(&path) {
            let ready: ReadyState = serde_json::from_slice(&bytes)?;
            return Ok(ready);
        }
        if Instant::now() >= deadline {
            return Err(format!("timed out waiting for {}", path.display()).into());
        }
        thread::sleep(Duration::from_millis(50));
    }
}

fn wait_absent(path: &Path, timeout: Duration) -> Result<(), BoxError> {
    let deadline = Instant::now() + timeout;
    while path.exists() {
        if Instant::now() >= deadline {
            return Err(format!("timed out waiting for {} removal", path.display()).into());
        }
        thread::sleep(Duration::from_millis(50));
    }
    Ok(())
}

fn write_error_result(message: &str) -> Result<(), BoxError> {
    let path = PathBuf::from(required_env("MAD_HOST_RESULT")?);
    let result = HostResult {
        schema_version: 1,
        mode: env::var("MAD_HOST_MODE").unwrap_or_else(|_| "unknown".into()),
        spawned: false,
        reused: false,
        preexisting_not_owned: false,
        wrong_owner_denied: false,
        sidecar_resolved_pid: None,
        daemon_pid: None,
        grandchild_pid: None,
        error: Some(message.to_string()),
    };
    write_json(&path, &result)
}

fn write_json(path: &Path, value: &impl Serialize) -> Result<(), BoxError> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let temporary = path.with_extension(format!("{}.tmp", std::process::id()));
    fs::write(&temporary, serde_json::to_vec_pretty(value)?)?;
    let _ = fs::remove_file(path);
    fs::rename(temporary, path)?;
    Ok(())
}

fn token_hash(value: &str) -> String {
    hex::encode(Sha256::digest(value.as_bytes()))
}

fn unique_suffix() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos()
}

fn required_env(key: &str) -> Result<String, BoxError> {
    env::var(key).map_err(|_| format!("{key} is required").into())
}
