#!/usr/bin/env python3
"""Run bounded Claude Team-subscription print and PTY compatibility probes.

The harness never emits raw Claude output, auth JSON, terminal content, prompt
text, session identifiers, file names from the Claude config directory, or
credential values. Execution is fail-closed and requires an explicit assertion
that Team usage credits are disabled before either model request can run.
"""

from __future__ import annotations

import argparse
import fcntl
import hashlib
import json
import os
import platform
import pty
import re
import select
import signal
import stat
import struct
import subprocess
import tempfile
import termios
import time
from pathlib import Path
from typing import Any


EXPECTED_VERSION = "2.1.207"
EXPECTED_BINARY_SHA256 = (
    "1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a"
)
EXPECTED_OS = "26.5.2"
EXPECTED_ARCH = "arm64"

ATTEMPT_LEDGER_NAMES = {
    "print": "2026-07-20-print-attempt-claimed.json",
    "pty": "2026-07-20-pty-attempt-claimed.json",
}

AUTH_OVERRIDE_VARS = (
    "ANTHROPIC_API_KEY",
    "ANTHROPIC_AUTH_TOKEN",
    "ANTHROPIC_BASE_URL",
    "ANTHROPIC_MODEL",
    "ANTHROPIC_CUSTOM_HEADERS",
    "CLAUDE_CODE_OAUTH_TOKEN",
    "CLAUDE_CODE_OAUTH_REFRESH_TOKEN",
    "CLAUDE_CODE_OAUTH_SCOPES",
    "CLAUDE_CONFIG_DIR",
    "CLAUDE_CODE_USE_BEDROCK",
    "CLAUDE_CODE_USE_VERTEX",
    "CLAUDE_CODE_USE_FOUNDRY",
    "CLAUDE_CODE_USE_ANTHROPIC_AWS",
    "CLAUDE_CODE_USE_MANTLE",
    "ANTHROPIC_FOUNDRY_API_KEY",
    "ANTHROPIC_FOUNDRY_AUTH_TOKEN",
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_SESSION_TOKEN",
    "AWS_PROFILE",
    "AWS_BEARER_TOKEN_BEDROCK",
    "GOOGLE_APPLICATION_CREDENTIALS",
    "CLOUD_ML_REGION",
)

NETWORK_OVERRIDE_VARS = (
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "ALL_PROXY",
    "http_proxy",
    "https_proxy",
    "all_proxy",
)

SAFE_ENV = {
    "CLAUDE_CODE_SAFE_MODE": "1",
    "CLAUDE_CODE_SKIP_PROMPT_HISTORY": "1",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB": "1",
    "DISABLE_TELEMETRY": "1",
    "DISABLE_ERROR_REPORTING": "1",
    "DISABLE_FEEDBACK_COMMAND": "1",
    "CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY": "1",
    "NO_COLOR": "1",
    "TERM": "xterm-256color",
}

INHERITED_ENV_ALLOWLIST = (
    "HOME",
    "PATH",
    "TMPDIR",
    "LANG",
    "LC_ALL",
    "LC_CTYPE",
    "SHELL",
    "USER",
    "LOGNAME",
    "__CF_USER_TEXT_ENCODING",
)

SETTINGS_CREDENTIAL_KEYS = {
    "anthropic_api_key",
    "anthropic_auth_token",
    "anthropic_base_url",
    "apikeyhelper",
    "awsauthrefresh",
    "awscredentialexport",
    "claude_code_oauth_token",
    "claude_code_oauth_refresh_token",
    "claude_code_use_bedrock",
    "claude_code_use_vertex",
    "claude_code_use_foundry",
    "claude_code_use_anthropic_aws",
    "claude_code_use_mantle",
}


class HarnessInterrupted(Exception):
    """Raised by controlled signal handlers so child cleanup always runs."""


def install_signal_handlers() -> None:
    def handle_signal(_signum: int, _frame: Any) -> None:
        raise HarnessInterrupted()

    for handled_signal in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
        signal.signal(handled_signal, handle_signal)

BLOCK_PATTERNS = {
    "billing": (
        b"usage credits",
        b"extra usage",
        b"hit your limit",
        b"usage limit",
        b"rate limit",
        b"upgrade your plan",
        b"enable extra",
    ),
    "auth": (
        b"please log in",
        b"please login",
        b"not logged in",
        b"authentication required",
        b"select a login",
        b"api key required",
        b"invalid api key",
    ),
    "trust": (
        b"do you trust",
        b"trust this folder",
        b"trust this workspace",
    ),
    "tool": (
        b"allow this tool",
        b"tool permission required",
        b"approve tool use",
        b"permission to use",
    ),
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--preflight", action="store_true")
    mode.add_argument("--execute", action="store_true")
    parser.add_argument(
        "--claude",
        default=str(Path.home() / ".local" / "bin" / "claude"),
        help="Path to the pinned official Claude Code executable.",
    )
    parser.add_argument(
        "--usage-credits-disabled-operator-attested",
        action="store_true",
        help=(
            "Record the operator's attestation that a Team Owner confirmed "
            "Organization Settings > Usage credits are disabled. This external "
            "fact is mandatory for --execute and is not machine-verified."
        ),
    )
    return parser.parse_args()


def scrubbed_environment() -> dict[str, str]:
    env = {
        name: os.environ[name]
        for name in INHERITED_ENV_ALLOWLIST
        if name in os.environ
    }
    env.update(SAFE_ENV)
    return env


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def attempt_ledger_path(arm: str) -> Path:
    return Path(__file__).resolve().parent / ATTEMPT_LEDGER_NAMES[arm]


def claim_attempt(arm: str) -> str:
    path = attempt_ledger_path(arm)
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(path, flags, 0o600)
    payload = json.dumps(
        {
            "schemaVersion": 1,
            "arm": arm,
            "attemptClaimed": True,
            "claimedBeforeProcessSpawn": True,
            "rawContentRetained": False,
        },
        indent=2,
        sort_keys=True,
    ).encode() + b"\n"
    try:
        view = memoryview(payload)
        while view:
            written = os.write(descriptor, view)
            view = view[written:]
        os.fsync(descriptor)
    finally:
        os.close(descriptor)
    directory_flags = os.O_RDONLY
    if hasattr(os, "O_DIRECTORY"):
        directory_flags |= os.O_DIRECTORY
    directory_descriptor = os.open(path.parent, directory_flags)
    try:
        os.fsync(directory_descriptor)
    finally:
        os.close(directory_descriptor)
    return path.name


def process_group_exists(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return False


def terminate_process_group(
    process: subprocess.Popen[bytes],
    pgid: int,
) -> str:
    if not process_group_exists(pgid):
        return "already_exited"
    try:
        os.killpg(pgid, signal.SIGTERM)
    except ProcessLookupError:
        return "already_exited"
    except PermissionError:
        return "permission_denied"
    deadline = time.monotonic() + 2.0
    while time.monotonic() < deadline and process_group_exists(pgid):
        time.sleep(0.05)
    if not process_group_exists(pgid):
        if process.poll() is None:
            process.wait(timeout=0.5)
        return "terminated"
    try:
        os.killpg(pgid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    except PermissionError:
        return "permission_denied"
    if process.poll() is None:
        try:
            process.wait(timeout=2.0)
        except subprocess.TimeoutExpired:
            return "kill_timeout"
    return "killed"


def bounded_command(
    argv: list[str],
    *,
    env: dict[str, str],
    cwd: Path | None = None,
    stdin_data: bytes = b"",
    timeout_seconds: float = 20.0,
    stdout_cap: int = 16 * 1024,
    stderr_cap: int = 16 * 1024,
) -> dict[str, Any]:
    started = time.monotonic()
    process: subprocess.Popen[bytes] | None = None
    pgid: int | None = None
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    timed_out = False
    output_cap_exceeded = False
    cleanup = "not_needed"
    try:
        process = subprocess.Popen(
            argv,
            cwd=str(cwd) if cwd else None,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
            close_fds=True,
        )
        pgid = process.pid
        assert process.stdin is not None
        assert process.stdout is not None
        assert process.stderr is not None
        try:
            process.stdin.write(stdin_data)
            process.stdin.close()
        except BrokenPipeError:
            pass

        streams = {
            process.stdout.fileno(): ("stdout", process.stdout, stdout_cap),
            process.stderr.fileno(): ("stderr", process.stderr, stderr_cap),
        }
        for _, stream, _ in streams.values():
            os.set_blocking(stream.fileno(), False)

        open_fds = set(streams)
        deadline = started + timeout_seconds
        while open_fds:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                timed_out = True
                cleanup = terminate_process_group(process, pgid)
                break
            readable, _, _ = select.select(
                list(open_fds), [], [], min(0.2, remaining)
            )
            for fd in readable:
                name, _, cap = streams[fd]
                try:
                    chunk = os.read(fd, 4096)
                except BlockingIOError:
                    continue
                if not chunk:
                    open_fds.discard(fd)
                    continue
                buffers[name].extend(chunk)
                if len(buffers[name]) > cap:
                    output_cap_exceeded = True
                    del buffers[name][cap:]
                    cleanup = terminate_process_group(process, pgid)
                    open_fds.clear()
                    break
            if process.poll() is not None and not readable:
                for fd in list(open_fds):
                    try:
                        chunk = os.read(fd, 4096)
                    except (BlockingIOError, OSError):
                        chunk = b""
                    if chunk:
                        name, _, cap = streams[fd]
                        buffers[name].extend(chunk)
                        if len(buffers[name]) > cap:
                            output_cap_exceeded = True
                            del buffers[name][cap:]
                    else:
                        open_fds.discard(fd)

        if process.poll() is None or process_group_exists(pgid):
            cleanup = terminate_process_group(process, pgid)
        exit_code = process.poll()
    finally:
        if process is not None and pgid is not None and process_group_exists(pgid):
            cleanup = terminate_process_group(process, pgid)
        if process is not None:
            for stream in (process.stdin, process.stdout, process.stderr):
                if stream is not None and not stream.closed:
                    stream.close()

    duration_ms = int((time.monotonic() - started) * 1000)
    return {
        "stdout": bytes(buffers["stdout"]),
        "stderr": bytes(buffers["stderr"]),
        "stdoutByteCount": len(buffers["stdout"]),
        "stderrByteCount": len(buffers["stderr"]),
        "exitCode": exit_code,
        "timedOut": timed_out,
        "outputCapExceeded": output_cap_exceeded,
        "cleanup": cleanup,
        "durationMs": duration_ms,
    }


def find_settings_credential_override() -> dict[str, Any]:
    settings_paths = (
        Path.home() / ".claude.json",
        Path.home() / ".claude" / "settings.json",
        Path.home() / ".claude" / "settings.local.json",
        Path("/Library/Application Support/ClaudeCode/managed-settings.json"),
    )
    detected = False
    parse_failures = 0
    inspected = 0

    def inspect_value(value: Any) -> None:
        nonlocal detected
        if isinstance(value, dict):
            for key, child in value.items():
                normalized = re.sub(r"[^a-z0-9_]", "", str(key).lower())
                if normalized in SETTINGS_CREDENTIAL_KEYS:
                    detected = True
                inspect_value(child)
        elif isinstance(value, list):
            for child in value:
                inspect_value(child)

    for path in settings_paths:
        if not path.is_file():
            continue
        inspected += 1
        try:
            inspect_value(json.loads(path.read_text(encoding="utf-8")))
        except (OSError, UnicodeDecodeError, json.JSONDecodeError):
            parse_failures += 1
    return {
        "inspectedFileCount": inspected,
        "credentialOverrideDetected": detected,
        "parseFailureCount": parse_failures,
    }


def manifest(root: Path) -> dict[str, Any]:
    digest = hashlib.sha256()
    entry_count = 0
    total_bytes = 0
    errors = 0
    if not root.exists():
        return {
            "present": False,
            "entryCount": 0,
            "totalBytes": 0,
            "metadataSha256": digest.hexdigest(),
            "errorCount": 0,
        }

    try:
        root_stat = root.lstat()
        root_kind = "file" if stat.S_ISREG(root_stat.st_mode) else "directory"
        root_size = root_stat.st_size if root_kind == "file" else 0
        digest.update(
            (
                f"root\0{root_kind}\0{stat.S_IMODE(root_stat.st_mode)}\0"
                f"{root_size}\0{root_stat.st_mtime_ns}"
            ).encode()
        )
        entry_count += 1
        total_bytes += root_size
    except OSError:
        errors += 1

    for current, directories, files in os.walk(root, followlinks=False):
        directories.sort()
        files.sort()
        for name in [*directories, *files]:
            path = Path(current) / name
            try:
                relative = path.relative_to(root).as_posix()
                path_digest = hashlib.sha256(relative.encode()).hexdigest()
                info = path.lstat()
                kind = (
                    "symlink"
                    if stat.S_ISLNK(info.st_mode)
                    else "directory"
                    if stat.S_ISDIR(info.st_mode)
                    else "file"
                )
                size = info.st_size if kind == "file" else 0
                entry_count += 1
                total_bytes += size
                digest.update(
                    (
                        f"{path_digest}\0{kind}\0{stat.S_IMODE(info.st_mode)}\0"
                        f"{size}\0{info.st_mtime_ns}\n"
                    ).encode()
                )
            except (OSError, ValueError):
                errors += 1
    return {
        "present": True,
        "entryCount": entry_count,
        "totalBytes": total_bytes,
        "metadataSha256": digest.hexdigest(),
        "errorCount": errors,
    }


def combined_manifest(roots: tuple[Path, ...]) -> dict[str, Any]:
    digest = hashlib.sha256()
    present_count = 0
    entry_count = 0
    total_bytes = 0
    errors = 0
    for index, root in enumerate(roots):
        current = manifest(root)
        present_count += 1 if current["present"] else 0
        entry_count += current["entryCount"]
        total_bytes += current["totalBytes"]
        errors += current["errorCount"]
        digest.update(
            (
                f"{index}\0{current['present']}\0{current['entryCount']}\0"
                f"{current['totalBytes']}\0{current['metadataSha256']}\0"
                f"{current['errorCount']}\n"
            ).encode()
        )
    return {
        "rootCount": len(roots),
        "presentRootCount": present_count,
        "entryCount": entry_count,
        "totalBytes": total_bytes,
        "metadataSha256": digest.hexdigest(),
        "errorCount": errors,
    }


def manifests_equal(before: dict[str, Any], after: dict[str, Any]) -> bool:
    return before == after and before["errorCount"] == 0


def detect_block_class(content: bytes) -> str | None:
    normalized = re.sub(rb"\x1b\[[0-?]*[ -/]*[@-~]", b"", content)
    normalized = re.sub(rb"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", b"", normalized)
    lowered = normalized.lower()
    for block_class, patterns in BLOCK_PATTERNS.items():
        if any(pattern in lowered for pattern in patterns):
            return block_class
    return None


def build_prompt(mode: str) -> tuple[bytes, bytes, str]:
    tokens = ("TEAM", "SUBSCRIPTION", mode, "OK")
    prompt = (
        "Join the four public test tokens "
        + ", ".join(tokens[:-1])
        + ", and "
        + tokens[-1]
        + " with underscores. Output only the joined string.\n"
    ).encode()
    marker = "_".join(tokens).encode()
    return prompt, marker, hashlib.sha256(prompt).hexdigest()


def system_prompt() -> str:
    return " ".join(
        (
            "This is a transport acceptance probe.",
            "Use no tools.",
            "Return only the requested synthetic marker.",
        )
    )


def common_args(binary: Path) -> list[str]:
    return [
        str(binary),
        "--safe-mode",
        "--no-chrome",
        "--disable-slash-commands",
        "--strict-mcp-config",
        "--tools",
        "",
        "--disallowedTools",
        "mcp__*",
        "--permission-mode",
        "dontAsk",
        "--model",
        "haiku",
        "--system-prompt",
        system_prompt(),
    ]


def collect_preflight(binary: Path, env: dict[str, str]) -> dict[str, Any]:
    parent_presence = {
        name: bool(os.environ.get(name))
        for name in (*AUTH_OVERRIDE_VARS, *NETWORK_OVERRIDE_VARS)
    }
    os_version = platform.mac_ver()[0]
    architecture = platform.machine()
    executable_ok = binary.is_file() and os.access(binary, os.X_OK)
    digest = sha256_file(binary) if executable_ok else None
    pinned_file_ok = (
        executable_ok
        and platform.system() == "Darwin"
        and os_version == EXPECTED_OS
        and architecture == EXPECTED_ARCH
        and digest == EXPECTED_BINARY_SHA256
    )
    settings = find_settings_credential_override()
    config_roots = (Path.home() / ".claude", Path.home() / ".claude.json")
    config_before = combined_manifest(config_roots)

    # No executable may start until the host and binary pass the pure-file gate.
    version_result = (
        bounded_command([str(binary), "--version"], env=env, timeout_seconds=10.0)
        if pinned_file_ok
        else None
    )
    version_text = (
        version_result["stdout"].decode("utf-8", "replace")
        if version_result is not None
        else ""
    )
    version_match = re.search(r"\b(\d+\.\d+\.\d+)\b", version_text)
    version = version_match.group(1) if version_match else None

    help_result = bounded_command(
        [str(binary), "--help"],
        env=env,
        timeout_seconds=10.0,
        stdout_cap=64 * 1024,
        stderr_cap=8 * 1024,
    ) if pinned_file_ok else None
    help_text = (
        help_result["stdout"].decode("utf-8", "replace")
        if help_result is not None
        else ""
    )
    required_flags = (
        "--safe-mode",
        "--no-chrome",
        "--disable-slash-commands",
        "--strict-mcp-config",
        "--tools",
        "--disallowedTools",
        "--permission-mode",
        "--model",
        "--system-prompt",
        "--print",
        "--no-session-persistence",
        "--output-format",
        "--ax-screen-reader",
    )
    flag_surface_supported = all(flag in help_text for flag in required_flags)
    positional_prompt_supported = (
        "Usage: claude [options] [command] [prompt]" in help_text
        and "prompt" in help_text
    )

    auth_result = bounded_command(
        [str(binary), "auth", "status", "--json"],
        env=env,
        timeout_seconds=15.0,
        stdout_cap=32 * 1024,
        stderr_cap=8 * 1024,
    ) if pinned_file_ok else None
    auth_projection: dict[str, Any] = {
        "loggedIn": False,
        "authMethod": None,
        "apiProvider": None,
        "subscriptionType": None,
        "parseSucceeded": False,
    }
    if auth_result is not None and auth_result["exitCode"] == 0:
        try:
            raw_auth = json.loads(auth_result["stdout"].decode("utf-8"))
            auth_projection = {
                "loggedIn": raw_auth.get("loggedIn") is True,
                "authMethod": (
                    "claude.ai"
                    if raw_auth.get("authMethod") == "claude.ai"
                    else "unexpected"
                ),
                "apiProvider": (
                    "firstParty"
                    if raw_auth.get("apiProvider") == "firstParty"
                    else "unexpected"
                ),
                "subscriptionType": (
                    "team"
                    if raw_auth.get("subscriptionType") == "team"
                    else "unexpected"
                ),
                "parseSucceeded": True,
            }
        except (UnicodeDecodeError, json.JSONDecodeError, AttributeError):
            pass

    config_after = combined_manifest(config_roots)
    preflight_config_unchanged = manifests_equal(config_before, config_after)
    auth_ok = auth_projection == {
        "loggedIn": True,
        "authMethod": "claude.ai",
        "apiProvider": "firstParty",
        "subscriptionType": "team",
        "parseSucceeded": True,
    }
    override_absent = not any(parent_presence.values())
    settings_ok = (
        not settings["credentialOverrideDetected"]
        and settings["parseFailureCount"] == 0
    )
    exact_environment = pinned_file_ok and version == EXPECTED_VERSION
    ready = (
        executable_ok
        and exact_environment
        and override_absent
        and settings_ok
        and auth_ok
        and version_result is not None
        and version_result["exitCode"] == 0
        and not version_result["timedOut"]
        and help_result is not None
        and help_result["exitCode"] == 0
        and not help_result["timedOut"]
        and not help_result["outputCapExceeded"]
        and flag_surface_supported
        and positional_prompt_supported
        and auth_result is not None
        and auth_result["exitCode"] == 0
        and not auth_result["timedOut"]
        and not auth_result["outputCapExceeded"]
        and preflight_config_unchanged
    )
    return {
        "ready": ready,
        "platform": {
            "system": platform.system(),
            "osVersion": os_version,
            "architecture": architecture,
        },
        "claude": {
            "version": version,
            "binarySha256": digest,
            "executable": executable_ok,
            "pinnedFileAndHostMatchedBeforeExecution": pinned_file_ok,
            "positionalPromptSurfaceSupported": positional_prompt_supported,
            "requiredFlagSurfaceSupported": flag_surface_supported,
        },
        "auth": auth_projection,
        "parentOverridePresence": parent_presence,
        "selectedConfigScopesMetadataUnchanged": preflight_config_unchanged,
        "settings": settings,
    }


def print_probe(
    binary: Path,
    env: dict[str, str],
    config_roots: tuple[Path, ...],
) -> dict[str, Any]:
    prompt, marker, fixture_sha = build_prompt("PRINT")
    with tempfile.TemporaryDirectory(prefix="claude-team-print-") as directory:
        workspace = Path(directory)
        workspace_before = manifest(workspace)
        config_before = combined_manifest(config_roots)
        result = bounded_command(
            [
                *common_args(binary),
                "--print",
                "--no-session-persistence",
                "--output-format",
                "text",
            ],
            env=env,
            cwd=workspace,
            stdin_data=prompt,
            timeout_seconds=45.0,
            stdout_cap=512,
            stderr_cap=4 * 1024,
        )
        workspace_after = manifest(workspace)
        config_after = combined_manifest(config_roots)

    combined = result["stdout"] + b"\n" + result["stderr"]
    block_class = detect_block_class(combined)
    marker_match = result["stdout"].strip() == marker
    workspace_unchanged = manifests_equal(workspace_before, workspace_after)
    config_unchanged = manifests_equal(config_before, config_after)

    if block_class == "billing":
        classification = "LIMIT_OR_CREDIT_PROMPT"
    elif block_class in {"auth", "trust"}:
        classification = "AUTH_BOUNDARY_FAIL"
    elif block_class == "tool":
        classification = "TOOL_PROMPT"
    elif result["timedOut"]:
        classification = "TIMEOUT"
    elif result["outputCapExceeded"]:
        classification = "OUTPUT_CAP_EXCEEDED"
    elif result["exitCode"] != 0:
        classification = "PROCESS_ERROR"
    elif not marker_match:
        classification = "OUTPUT_MISMATCH"
    elif not workspace_unchanged or not config_unchanged:
        classification = "UNEXPECTED_LOCAL_WRITE"
    else:
        classification = "PASS"

    return {
        "requestSent": True,
        "requestCount": 1,
        "model": "haiku",
        "fixtureSha256": fixture_sha,
        "classification": classification,
        "exitCode": result["exitCode"],
        "durationMs": result["durationMs"],
        "timedOut": result["timedOut"],
        "outputCapExceeded": result["outputCapExceeded"],
        "stdoutByteCount": result["stdoutByteCount"],
        "stderrByteCount": result["stderrByteCount"],
        "markerMatched": marker_match,
        "blockingPromptClass": block_class,
        "workspaceUnchanged": workspace_unchanged,
        "selectedConfigScopesMetadataUnchanged": config_unchanged,
        "cleanup": result["cleanup"],
    }


def set_terminal_size(fd: int, rows: int, columns: int) -> None:
    fcntl.ioctl(
        fd,
        termios.TIOCSWINSZ,
        struct.pack("HHHH", rows, columns, 0, 0),
    )


def safe_signal_group(pgid: int, sig: signal.Signals) -> None:
    try:
        os.killpg(pgid, sig)
    except ProcessLookupError:
        pass


def wait_for_exit(process: subprocess.Popen[bytes], seconds: float) -> bool:
    try:
        process.wait(timeout=seconds)
        return True
    except subprocess.TimeoutExpired:
        return False


def pty_probe(
    binary: Path,
    env: dict[str, str],
    config_roots: tuple[Path, ...],
) -> dict[str, Any]:
    prompt, marker, fixture_sha = build_prompt("PTY")
    prompt_argument = prompt.decode("ascii").rstrip("\n")
    started = time.monotonic()
    total_deadline = started + 60.0
    rolling = bytearray()
    total_bytes = 0
    output_cap_exceeded = False
    request_sent = False
    marker_matched = False
    block_class: str | None = None
    resize_count = 0
    timed_out = False
    cleanup = "not_started"
    exit_code: int | None = None
    process: subprocess.Popen[bytes] | None = None
    pgid: int | None = None
    master: int | None = None
    slave: int | None = None

    with tempfile.TemporaryDirectory(prefix="claude-team-pty-") as directory:
        workspace = Path(directory)
        workspace_before = manifest(workspace)
        config_before = combined_manifest(config_roots)
        try:
            master, slave = pty.openpty()
            set_terminal_size(slave, 24, 80)
            process = subprocess.Popen(
                [
                    *common_args(binary),
                    "--ax-screen-reader",
                    prompt_argument,
                ],
                cwd=str(workspace),
                env=env,
                stdin=slave,
                stdout=slave,
                stderr=slave,
                preexec_fn=os.setsid,
                close_fds=True,
            )
            pgid = process.pid
            request_sent = True
            os.close(slave)
            slave = None
            os.set_blocking(master, False)

            def read_available(wait_seconds: float) -> None:
                nonlocal total_bytes, output_cap_exceeded, block_class
                assert master is not None
                readable, _, _ = select.select(
                    [master], [], [], max(0.0, wait_seconds)
                )
                if not readable:
                    return
                try:
                    chunk = os.read(master, 4096)
                except (BlockingIOError, OSError):
                    return
                if not chunk:
                    return
                total_bytes += len(chunk)
                rolling.extend(chunk)
                if len(rolling) > 64 * 1024:
                    del rolling[: len(rolling) - 64 * 1024]
                if total_bytes > 256 * 1024:
                    output_cap_exceeded = True
                detected = detect_block_class(bytes(rolling))
                if detected is not None:
                    block_class = detected

            # The synthetic initial prompt is positional, never typed into an
            # unknown menu. The PTY is used only for rendering, resize, and exit.
            for rows, columns in ((30, 100), (40, 120), (24, 80)):
                read_available(0.2)
                if (
                    process.poll() is not None
                    or block_class is not None
                    or output_cap_exceeded
                ):
                    break
                set_terminal_size(master, rows, columns)
                safe_signal_group(pgid, signal.SIGWINCH)
                resize_count += 1

            while (
                time.monotonic() < total_deadline
                and process.poll() is None
                and block_class is None
                and not output_cap_exceeded
            ):
                read_available(0.2)
                if marker in rolling:
                    marker_matched = True
                    break

            if process.poll() is not None and not marker_matched:
                drain_deadline = min(total_deadline, time.monotonic() + 0.5)
                while time.monotonic() < drain_deadline:
                    read_available(0.05)
                    if marker in rolling:
                        marker_matched = True
                        break

            if time.monotonic() >= total_deadline and not marker_matched:
                timed_out = True

            if marker_matched and process.poll() is None:
                settle_deadline = min(total_deadline, time.monotonic() + 1.0)
                while time.monotonic() < settle_deadline and process.poll() is None:
                    read_available(0.1)
                try:
                    os.write(master, b"\x04")
                except OSError:
                    pass
                if wait_for_exit(process, 5.0):
                    cleanup = "clean_eof"
                else:
                    safe_signal_group(pgid, signal.SIGINT)
                    cleanup = (
                        "clean_interrupt"
                        if wait_for_exit(process, 3.0)
                        else "pending"
                    )

            if process.poll() is None or process_group_exists(pgid):
                cleanup = terminate_process_group(process, pgid)
            elif cleanup == "not_started":
                cleanup = "self_exit"
            exit_code = process.poll()
        finally:
            if (
                process is not None
                and pgid is not None
                and process_group_exists(pgid)
            ):
                cleanup = terminate_process_group(process, pgid)
            for descriptor in (master, slave):
                if descriptor is not None:
                    try:
                        os.close(descriptor)
                    except OSError:
                        pass
            workspace_after = manifest(workspace)
            config_after = combined_manifest(config_roots)

    workspace_unchanged = manifests_equal(workspace_before, workspace_after)
    config_unchanged = manifests_equal(config_before, config_after)
    duration_ms = int((time.monotonic() - started) * 1000)

    if block_class == "billing":
        classification = "LIMIT_OR_CREDIT_PROMPT"
    elif block_class in {"auth", "trust"}:
        classification = "AUTH_BOUNDARY_FAIL"
    elif block_class == "tool":
        classification = "TOOL_PROMPT"
    elif not request_sent:
        classification = "PROCESS_NOT_STARTED"
    elif timed_out:
        classification = "TIMEOUT"
    elif output_cap_exceeded:
        classification = "OUTPUT_CAP_EXCEEDED"
    elif not marker_matched:
        classification = "OUTPUT_MISMATCH"
    elif exit_code != 0:
        classification = "PROCESS_ERROR"
    elif cleanup not in {"clean_eof", "self_exit"}:
        classification = "UNCLEAN_EXIT"
    elif not workspace_unchanged or not config_unchanged:
        classification = "UNEXPECTED_LOCAL_WRITE"
    else:
        classification = "PASS"

    return {
        "requestSent": request_sent,
        "requestCount": 1 if request_sent else 0,
        "model": "haiku",
        "fixtureSha256": fixture_sha,
        "classification": classification,
        "exitCode": exit_code,
        "durationMs": duration_ms,
        "timedOut": timed_out,
        "outputCapExceeded": output_cap_exceeded,
        "capturedByteCount": total_bytes,
        "promptDelivery": "initial_positional_argument",
        "markerMatched": marker_matched,
        "blockingPromptClass": block_class,
        "resizeCount": resize_count,
        "workspaceUnchanged": workspace_unchanged,
        "selectedConfigScopesMetadataUnchanged": config_unchanged,
        "cleanup": cleanup,
    }


def emit(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, indent=2, sort_keys=True))


def main() -> int:
    args = parse_args()
    install_signal_handlers()
    try:
        binary = Path(args.claude).expanduser().resolve(strict=True)
    except (OSError, RuntimeError):
        emit(
            {
                "schemaVersion": 1,
                "status": "BLOCKED",
                "failureClass": "CLAUDE_BINARY_UNAVAILABLE",
                "providerRequestCount": 0,
            }
        )
        return 2

    env = scrubbed_environment()
    preflight = collect_preflight(binary, env)
    payload: dict[str, Any] = {
        "schemaVersion": 1,
        "target": "spike-claude-subscription-cli-pty-compatibility",
        "scope": "direct-official-cli-external-experimental",
        "billingSource": "claude_ai_team_subscription",
        "apiKeyUsed": False,
        "cloudCredentialUsed": False,
        "dollarBudgetFlagUsed": False,
        "usageCreditsDisabledOperatorAttested": bool(
            args.usage_credits_disabled_operator_attested
        ),
        "preflight": preflight,
        "providerRequestCount": 0,
    }

    if args.preflight:
        payload["status"] = "READY" if preflight["ready"] else "BLOCKED"
        emit(payload)
        return 0 if preflight["ready"] else 2

    if not preflight["ready"]:
        payload["status"] = "BLOCKED"
        payload["failureClass"] = "PREFLIGHT_FAILED"
        emit(payload)
        return 2

    if not args.usage_credits_disabled_operator_attested:
        payload["status"] = "BLOCKED"
        payload["failureClass"] = "USAGE_CREDITS_OPERATOR_ATTESTATION_MISSING"
        emit(payload)
        return 2

    existing_ledgers = [
        arm for arm in ATTEMPT_LEDGER_NAMES if attempt_ledger_path(arm).exists()
    ]
    if existing_ledgers:
        payload["status"] = "BLOCKED"
        payload["failureClass"] = "ONE_SHOT_ATTEMPT_ALREADY_CLAIMED"
        payload["existingAttemptLedgerArms"] = existing_ledgers
        emit(payload)
        return 2

    config_roots = (Path.home() / ".claude", Path.home() / ".claude.json")
    payload["attemptLedgers"] = {"print": claim_attempt("print")}
    payload["armPreflight"] = {"print": collect_preflight(binary, env)}
    if not payload["armPreflight"]["print"]["ready"]:
        payload["status"] = "FAIL"
        payload["failureClass"] = "PRINT_ARM_REVALIDATION_FAILED"
        emit(payload)
        return 1
    print_result = print_probe(binary, env, config_roots)
    payload["print"] = print_result
    payload["providerRequestCount"] = print_result["requestCount"]

    if print_result["classification"] == "PASS":
        payload["attemptLedgers"]["pty"] = claim_attempt("pty")
        payload["armPreflight"]["pty"] = collect_preflight(binary, env)
        if not payload["armPreflight"]["pty"]["ready"]:
            payload["pty"] = {
                "classification": "NOT_RUN_AFTER_ARM_REVALIDATION_FAILURE",
                "requestSent": False,
                "requestCount": 0,
            }
            payload["status"] = "FAIL"
            payload["failureClass"] = "PTY_ARM_REVALIDATION_FAILED"
            emit(payload)
            return 1
        pty_result = pty_probe(binary, env, config_roots)
        payload["pty"] = pty_result
        payload["providerRequestCount"] += pty_result["requestCount"]
    else:
        payload["pty"] = {
            "classification": "NOT_RUN_AFTER_PRINT_FAILURE",
            "requestSent": False,
            "requestCount": 0,
        }

    all_passed = (
        payload["print"]["classification"] == "PASS"
        and payload["pty"]["classification"] == "PASS"
        and payload["providerRequestCount"] == 2
    )
    payload["status"] = "PASS" if all_passed else "FAIL"
    payload["providerSideRetention"] = (
        "Team plan standard provider retention applies; this harness proves only "
        "that project and local evidence omit raw content."
    )
    emit(payload)
    return 0 if all_passed else 1


def guarded_main() -> int:
    try:
        return main()
    except HarnessInterrupted:
        claimed_arms = [
            arm for arm in ATTEMPT_LEDGER_NAMES if attempt_ledger_path(arm).exists()
        ]
        emit(
            {
                "schemaVersion": 1,
                "status": "FAIL",
                "failureClass": "INTERRUPTED",
                "claimedAttemptArms": claimed_arms,
                "providerRequestAttemptUpperBound": len(claimed_arms),
                "rawExceptionRetained": False,
            }
        )
        return 1
    except (Exception, KeyboardInterrupt):
        claimed_arms = [
            arm for arm in ATTEMPT_LEDGER_NAMES if attempt_ledger_path(arm).exists()
        ]
        emit(
            {
                "schemaVersion": 1,
                "status": "FAIL",
                "failureClass": "HARNESS_INTERNAL_ERROR",
                "claimedAttemptArms": claimed_arms,
                "providerRequestAttemptUpperBound": len(claimed_arms),
                "rawExceptionRetained": False,
            }
        )
        return 1


if __name__ == "__main__":
    raise SystemExit(guarded_main())
