param(
  [Parameter(Mandatory = $true)][string]$HostExe,
  [Parameter(Mandatory = $true)][string]$SidecarExe,
  [Parameter(Mandatory = $true)][string]$InstallerPath,
  [Parameter(Mandatory = $true)][string]$ResultPath,
  [Parameter(Mandatory = $true)][string]$WorkRoot
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) {
    throw $Message
  }
}

function Test-ProcessAlive([int]$ProcessId) {
  return $null -ne (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)
}

function Wait-ProcessGone([int]$ProcessId, [int]$TimeoutSeconds = 5) {
  $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
  while (Test-ProcessAlive $ProcessId) {
    if ([DateTime]::UtcNow -ge $deadline) {
      throw "process $ProcessId remained alive"
    }
    Start-Sleep -Milliseconds 50
  }
}

function Wait-File([string]$Path, [int]$TimeoutSeconds = 5) {
  $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
  while (-not (Test-Path $Path)) {
    if ([DateTime]::UtcNow -ge $deadline) {
      throw "timed out waiting for $Path"
    }
    Start-Sleep -Milliseconds 50
  }
}

function Invoke-HostMode(
  [string]$Mode,
  [string]$StateDir,
  [string]$OwnerToken,
  [string]$HostResult
) {
  Remove-Item $HostResult -Force -ErrorAction SilentlyContinue
  $env:MAD_HOST_MODE = $Mode
  $env:MAD_STATE_DIR = $StateDir
  $env:MAD_OWNER_TOKEN = $OwnerToken
  $env:MAD_HOST_RESULT = $HostResult
  & $HostExe | Out-Host
  $exitCode = $LASTEXITCODE
  Remove-Item Env:MAD_HOST_MODE, Env:MAD_STATE_DIR, Env:MAD_OWNER_TOKEN, Env:MAD_HOST_RESULT -ErrorAction SilentlyContinue
  return $exitCode
}

function Write-Control(
  [string]$StateDir,
  [string]$OwnerToken,
  [string]$Id
) {
  $command = [ordered]@{
    schema_version = 1
    id = $Id
    action = "shutdown"
    owner_token = $OwnerToken
  }
  $path = Join-Path $StateDir "control.json"
  $temporary = "$path.powershell.tmp"
  $command | ConvertTo-Json | Set-Content -Path $temporary -Encoding utf8NoBOM
  Move-Item -Force $temporary $path
}

function Read-Json([string]$Path) {
  return Get-Content -Raw $Path | ConvertFrom-Json
}

$HostExe = (Resolve-Path $HostExe).Path
$SidecarExe = (Resolve-Path $SidecarExe).Path
$InstallerPath = (Resolve-Path $InstallerPath).Path
$WorkRoot = [System.IO.Path]::GetFullPath($WorkRoot)
$ResultPath = [System.IO.Path]::GetFullPath($ResultPath)
Remove-Item $WorkRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $WorkRoot | Out-Null
New-Item -ItemType Directory -Force ([System.IO.Path]::GetDirectoryName($ResultPath)) | Out-Null

$ownerToken = "desktop-owner-7f4c9b2e6d1a"
$systemToken = "system-owner-0d7a3c9f2b6e"

# Scenario 1: a Rust-side Tauri ShellExt sidecar launch and cooperative owner
# shutdown must leave neither the daemon nor its descendant alive.
$gracefulState = Join-Path $WorkRoot "graceful"
$gracefulHostResultPath = Join-Path $gracefulState "host-result.json"
$gracefulExit = Invoke-HostMode "spawn-graceful" $gracefulState $ownerToken $gracefulHostResultPath
Assert-True ($gracefulExit -eq 0) "graceful Tauri host exited $gracefulExit"
$graceful = Read-Json $gracefulHostResultPath
Assert-True ($graceful.spawned -eq $true) "graceful host did not spawn the sidecar"
Assert-True ($graceful.sidecar_resolved_pid -eq $graceful.daemon_pid) "Tauri-resolved sidecar PID mismatch"
Wait-ProcessGone ([int]$graceful.daemon_pid)
Wait-ProcessGone ([int]$graceful.grandchild_pid)

# Scenario 2: the selected continuity policy intentionally preserves the
# Rust-spawned sidecar when the Desktop process aborts. A restarted host must
# reuse it, deny a wrong owner, then cooperatively stop the whole tree.
$crashState = Join-Path $WorkRoot "crash-reconnect"
$crashHostResultPath = Join-Path $crashState "host-crash-result.json"
$crashExit = Invoke-HostMode "spawn-crash" $crashState $ownerToken $crashHostResultPath
Assert-True ($crashExit -ne 0) "crash mode unexpectedly exited successfully"
$crash = Read-Json $crashHostResultPath
Assert-True ($crash.spawned -eq $true) "crash host did not spawn the sidecar"
Assert-True (Test-ProcessAlive ([int]$crash.daemon_pid)) "sidecar did not survive the Desktop abort"
Assert-True (Test-ProcessAlive ([int]$crash.grandchild_pid)) "sidecar descendant did not survive the Desktop abort"
Start-Sleep -Seconds 2
$survivedCrashWindow = (Test-ProcessAlive ([int]$crash.daemon_pid)) -and (Test-ProcessAlive ([int]$crash.grandchild_pid))
Assert-True $survivedCrashWindow "sidecar process tree did not survive the two-second crash window"

$reconnectHostResultPath = Join-Path $crashState "host-reconnect-result.json"
$reconnectExit = Invoke-HostMode "reconnect-stop" $crashState $ownerToken $reconnectHostResultPath
Assert-True ($reconnectExit -eq 0) "reconnect Tauri host exited $reconnectExit"
$reconnect = Read-Json $reconnectHostResultPath
Assert-True ($reconnect.reused -eq $true) "restart did not reuse the existing sidecar"
Assert-True ($reconnect.spawned -eq $false) "restart spawned a duplicate sidecar"
Assert-True ($reconnect.wrong_owner_denied -eq $true) "wrong owner was not denied"
Assert-True ($reconnect.daemon_pid -eq $crash.daemon_pid) "restart attached to a different daemon PID"
Wait-ProcessGone ([int]$crash.daemon_pid)
Wait-ProcessGone ([int]$crash.grandchild_pid)

# Scenario 3: a pre-existing system-style daemon has a different ownership
# token. Desktop may observe it but must not claim or stop it.
$systemState = Join-Path $WorkRoot "preexisting-system"
New-Item -ItemType Directory -Force $systemState | Out-Null
$env:MAD_OWNER_TOKEN = $systemToken
$systemProcess = Start-Process -FilePath $SidecarExe -ArgumentList @("-mode", "daemon", "-state", $systemState) -PassThru
Remove-Item Env:MAD_OWNER_TOKEN
$systemReadyPath = Join-Path $systemState "ready.json"
Wait-File $systemReadyPath
$systemReady = Read-Json $systemReadyPath
Assert-True ($systemReady.daemon_pid -eq $systemProcess.Id) "pre-existing daemon PID mismatch"

$observeHostResultPath = Join-Path $systemState "host-observe-result.json"
$observeExit = Invoke-HostMode "observe-unowned" $systemState $ownerToken $observeHostResultPath
Assert-True ($observeExit -eq 0) "unowned-observer host exited $observeExit"
$observe = Read-Json $observeHostResultPath
Assert-True ($observe.preexisting_not_owned -eq $true) "Desktop treated pre-existing daemon as owned"
Assert-True ($observe.spawned -eq $false) "Desktop spawned beside pre-existing daemon"
Assert-True (Test-ProcessAlive ([int]$systemReady.daemon_pid)) "Desktop stopped the pre-existing daemon"
Assert-True (Test-ProcessAlive ([int]$systemReady.grandchild_pid)) "Desktop stopped the pre-existing daemon descendant"

$systemControlId = "system-owner-shutdown"
Remove-Item (Join-Path $systemState "control-result.json") -Force -ErrorAction SilentlyContinue
Write-Control $systemState $systemToken $systemControlId
Wait-File (Join-Path $systemState "control-result.json")
$systemControl = Read-Json (Join-Path $systemState "control-result.json")
Assert-True ($systemControl.id -eq $systemControlId -and $systemControl.accepted -eq $true) "system owner shutdown failed"
Wait-ProcessGone ([int]$systemReady.daemon_pid)
Wait-ProcessGone ([int]$systemReady.grandchild_pid)

# The NSIS archive must contain the target-suffix-stripped external binary.
$archiveListing = (& 7z l $InstallerPath | Out-String)
$installerContainsSidecar = $archiveListing -match "mad-sidecar\.exe"
Assert-True $installerContainsSidecar "NSIS installer does not contain mad-sidecar.exe"

$hostItem = Get-Item $HostExe
$sidecarItem = Get-Item $SidecarExe
$installerItem = Get-Item $InstallerPath
$result = [ordered]@{
  schema_version = 1
  supported = $true
  executed_at_utc = [DateTime]::UtcNow.ToString("o")
  runner_image_os = $env:ImageOS
  runner_image_version = $env:ImageVersion
  windows_version = (& cmd.exe /d /c ver | Out-String).Trim()
  architecture = $env:PROCESSOR_ARCHITECTURE
  rustc_version = (& rustc --version | Out-String).Trim()
  cargo_version = (& cargo --version | Out-String).Trim()
  go_version = (& go version | Out-String).Trim()
  tauri_version = "2.11.5"
  tauri_cli_version = "2.11.4"
  tauri_plugin_shell_version = "2.3.5"
  host_executable_bytes = $hostItem.Length
  host_executable_sha256 = (Get-FileHash -Algorithm SHA256 $HostExe).Hash.ToLowerInvariant()
  sidecar_executable_bytes = $sidecarItem.Length
  sidecar_executable_sha256 = (Get-FileHash -Algorithm SHA256 $SidecarExe).Hash.ToLowerInvariant()
  nsis_installer_name = $installerItem.Name
  nsis_installer_bytes = $installerItem.Length
  nsis_installer_sha256 = (Get-FileHash -Algorithm SHA256 $InstallerPath).Hash.ToLowerInvariant()
  nsis_installer_contains_sidecar = $installerContainsSidecar
  rust_shell_ext_sidecar_pid_matched = ($graceful.sidecar_resolved_pid -eq $graceful.daemon_pid)
  graceful_owner_shutdown_tree_clean = $true
  desktop_abort_exit_code = $crashExit
  crash_process_tree_survived_two_seconds = $survivedCrashWindow
  restart_reused_same_daemon = ($reconnect.daemon_pid -eq $crash.daemon_pid)
  restart_spawned_duplicate = $reconnect.spawned
  wrong_owner_shutdown_denied = $reconnect.wrong_owner_denied
  preexisting_daemon_not_owned_or_stopped = $observe.preexisting_not_owned
  final_orphan_processes = 0
  security_review_required = $true
  limitations = @(
    "The GitHub-hosted runner is Windows Server, not a physical Windows 11 workstation.",
    "The NSIS bundle contents are inspected but the installer is not executed or signed.",
    "The ownership-token file protocol is a Spike fixture only; production lifecycle control must use authenticated local IPC and packaged-binary signature/provenance checks.",
    "Power loss, OS logoff, updater replacement, system-service installation, sleep/resume, and real Provider session continuity remain Windows 11 acceptance items."
  )
}
$result | ConvertTo-Json -Depth 8 | Set-Content -Path $ResultPath -Encoding utf8NoBOM
Get-Content -Raw $ResultPath
