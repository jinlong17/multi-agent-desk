#!/usr/bin/env python3
"""Bounded PTY-only Claude Team-subscription compatibility probe v2.

This program is deliberately fail-closed.  Its fixture and preflight modes make
no model request.  Execute mode can claim one fresh ledger and spawn at most one
official Claude CLI process.  Raw auth JSON, PTY bytes, paths, file digests,
prompt/response captures, PII, and secrets are never written to durable output.
"""

from __future__ import annotations

import argparse
import dataclasses
import errno
import fcntl
import hashlib
import json
import os
import platform
import pty
import re
import select
import signal
import socket
import stat
import struct
import subprocess
import sys
import tempfile
import termios
import time
from datetime import datetime
from pathlib import Path
from typing import Any, Callable


TARGET = "spike-claude-subscription-cli-pty-compatibility-v2"
EXPECTED_SYSTEM = "Darwin"
EXPECTED_OS_VERSION = "26.5.2"
EXPECTED_ARCH = "arm64"
EXPECTED_CLAUDE_VERSION = "2.1.207"
EXPECTED_BINARY_SHA256 = (
    "1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a"
)
MODEL = "haiku"
LIVE_DEADLINE_SECONDS = 75.0
OUTPUT_CAP_BYTES = 256 * 1024
ROLLING_BYTES = 64 * 1024
RESIZES = ((30, 100), (40, 120), (24, 80))

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = Path(__file__).resolve().parents[3]
LEDGER_PATH = SCRIPT_DIR / "2026-07-21-pty-v2-attempt-claimed.json"
EVIDENCE_JSON_PATH = SCRIPT_DIR / "2026-07-21-macos-team-cli-pty-v2.json"

AUTH_OVERRIDE_NAMES = {
    "ANTHROPIC_API_KEY",
    "ANTHROPIC_AUTH_TOKEN",
    "ANTHROPIC_BASE_URL",
    "ANTHROPIC_CUSTOM_HEADERS",
    "ANTHROPIC_MODEL",
    "CLAUDE_CODE_OAUTH_TOKEN",
    "CLAUDE_CODE_OAUTH_REFRESH_TOKEN",
    "CLAUDE_CODE_OAUTH_SCOPES",
    "CLAUDE_CODE_SETUP_TOKEN",
    "CLAUDE_CONFIG_DIR",
    "CLAUDE_CODE_USE_BEDROCK",
    "CLAUDE_CODE_USE_VERTEX",
    "CLAUDE_CODE_USE_FOUNDRY",
    "CLAUDE_CODE_USE_MANTLE",
    "CLAUDE_CODE_USE_ANTHROPIC_AWS",
    "ANTHROPIC_FOUNDRY_API_KEY",
    "ANTHROPIC_FOUNDRY_AUTH_TOKEN",
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_SESSION_TOKEN",
    "AWS_PROFILE",
    "AWS_BEARER_TOKEN_BEDROCK",
    "AWS_REGION",
    "AWS_DEFAULT_REGION",
    "GOOGLE_APPLICATION_CREDENTIALS",
    "GOOGLE_CLOUD_PROJECT",
    "CLOUD_ML_REGION",
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "ALL_PROXY",
    "http_proxy",
    "https_proxy",
    "all_proxy",
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

SETTINGS_OVERRIDE_KEYS = {
    "anthropicapikey",
    "anthropicauthtoken",
    "anthropicbaseurl",
    "anthropiccustomheaders",
    "anthropicmodel",
    "apikeyhelper",
    "credentialhelper",
    "awsauthrefresh",
    "awscredentialexport",
    "claudecodeoauthtoken",
    "claudecodeoauthrefreshtoken",
    "claudecodesetuptoken",
    "claudecodeusebedrock",
    "claudecodeusevertex",
    "claudecodeusefoundry",
    "claudecodeusemantle",
    "claudecodeuseanthropicaws",
    "proxy",
    "httpproxy",
    "httpsproxy",
    "allproxy",
    "gateway",
}

REQUIRED_HELP_FLAGS = (
    "--safe-mode",
    "--no-chrome",
    "--disable-slash-commands",
    "--strict-mcp-config",
    "--tools",
    "--disallowedTools",
    "--permission-mode",
    "--model",
    "--system-prompt",
    "--ax-screen-reader",
)

BANNED_LIVE_FLAGS = (
    "--print",
    "-p",
    "--continue",
    "-c",
    "--resume",
    "-r",
    "--session-id",
    "--fork-session",
)

BLOCK_PATTERNS = {
    "billing": (
        b"usage credits",
        b"extra usage",
        b"usage limit",
        b"rate limit",
        b"hit your limit",
        b"upgrade your plan",
        b"enable extra",
        b"pay as you go",
    ),
    "auth": (
        b"please log in",
        b"please login",
        b"not logged in",
        b"authentication required",
        b"select a login",
        b"choose a login",
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
    "browser": (
        b"opening browser",
        b"open your browser",
        b"continue in browser",
    ),
    "unexpected_input": (
        b"press enter to continue",
        b"select an option",
        b"choose an option",
    ),
}

SENSITIVE_COMPONENTS = {
    "settings.json",
    "settings.local.json",
    "projects",
    "history",
    "sessions",
    "transcripts",
    "todos",
    "plans",
    "shell-snapshots",
    "hooks",
    "mcp",
    "plugins",
    "auth",
    "credentials",
    "tokens",
    "logs",
    "crash",
    "crashes",
}


class SafeStop(Exception):
    """A classified failure that must not leak the underlying raw content."""

    def __init__(self, classification: str) -> None:
        super().__init__(classification)
        self.classification = classification


class Interrupted(SafeStop):
    pass


@dataclasses.dataclass(frozen=True)
class Entry:
    kind: str
    mode: int
    uid: int
    gid: int
    size: int
    digest: str | None
    link_target: str | None
    mtime_ns: int
    ctime_ns: int


@dataclasses.dataclass(frozen=True)
class RootSnapshot:
    present: bool
    entries: dict[str, Entry]
    error_count: int
    total_bytes: int


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--fixtures", action="store_true")
    mode.add_argument("--preflight", action="store_true")
    mode.add_argument("--execute", action="store_true")
    parser.add_argument(
        "--claude",
        default=str(Path.home() / ".local" / "bin" / "claude"),
    )
    parser.add_argument(
        "--usage-credits-disabled-operator-directed",
        action="store_true",
    )
    return parser.parse_args()


def install_signal_handlers() -> None:
    def handler(_signum: int, _frame: Any) -> None:
        raise Interrupted("INTERRUPTED")

    for sig in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, handler)


def utc_offset_timestamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(path, flags)
    try:
        before = os.fstat(descriptor)
        if not stat.S_ISREG(before.st_mode):
            raise SafeStop("NON_REGULAR_EXECUTABLE")
        while True:
            chunk = os.read(descriptor, 1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
        after = os.fstat(descriptor)
        if (
            before.st_dev,
            before.st_ino,
            before.st_size,
            before.st_mtime_ns,
            before.st_ctime_ns,
        ) != (
            after.st_dev,
            after.st_ino,
            after.st_size,
            after.st_mtime_ns,
            after.st_ctime_ns,
        ):
            raise SafeStop("FILE_CHANGED_DURING_HASH")
    finally:
        os.close(descriptor)
    return digest.hexdigest()


def classify_mode(mode: int) -> str:
    if stat.S_ISREG(mode):
        return "file"
    if stat.S_ISDIR(mode):
        return "directory"
    if stat.S_ISLNK(mode):
        return "symlink"
    if stat.S_ISFIFO(mode):
        return "fifo"
    if stat.S_ISSOCK(mode):
        return "socket"
    if stat.S_ISCHR(mode) or stat.S_ISBLK(mode):
        return "device"
    return "other"


def file_digest_stable(path: Path, original: os.stat_result) -> str:
    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(path, flags)
    try:
        before = os.fstat(descriptor)
        if not stat.S_ISREG(before.st_mode):
            raise OSError(errno.EINVAL, "not regular")
        digest = hashlib.sha256()
        while True:
            chunk = os.read(descriptor, 1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
        after = os.fstat(descriptor)
    finally:
        os.close(descriptor)
    stable_fields = (
        "st_dev",
        "st_ino",
        "st_mode",
        "st_uid",
        "st_gid",
        "st_size",
        "st_mtime_ns",
        "st_ctime_ns",
    )
    if any(
        getattr(original, field) != getattr(before, field)
        or getattr(before, field) != getattr(after, field)
        for field in stable_fields
    ):
        raise OSError(errno.EBUSY, "changed during snapshot")
    return digest.hexdigest()


def snapshot_root(root: Path) -> RootSnapshot:
    entries: dict[str, Entry] = {}
    errors = 0
    total_bytes = 0
    if not root.exists() and not root.is_symlink():
        return RootSnapshot(False, entries, 0, 0)

    def add(path: Path, relative: str) -> bool:
        nonlocal errors, total_bytes
        try:
            info = path.lstat()
            kind = classify_mode(info.st_mode)
            digest: str | None = None
            target: str | None = None
            size = info.st_size if kind == "file" else 0
            if kind == "file":
                digest = file_digest_stable(path, info)
                total_bytes += size
            elif kind == "symlink":
                target = os.readlink(path)
            entries[relative] = Entry(
                kind=kind,
                mode=stat.S_IMODE(info.st_mode),
                uid=info.st_uid,
                gid=info.st_gid,
                size=size,
                digest=digest,
                link_target=target,
                mtime_ns=info.st_mtime_ns,
                ctime_ns=info.st_ctime_ns,
            )
            return kind == "directory"
        except (OSError, ValueError):
            errors += 1
            return False

    if not add(root, "."):
        return RootSnapshot(True, entries, errors, total_bytes)

    def descend(directory: Path, relative: str) -> None:
        nonlocal errors
        try:
            children = sorted(os.scandir(directory), key=lambda child: child.name)
        except OSError:
            errors += 1
            return
        for child in children:
            child_path = Path(child.path)
            child_relative = child.name if relative == "." else f"{relative}/{child.name}"
            if add(child_path, child_relative):
                descend(child_path, child_relative)

    descend(root, ".")
    return RootSnapshot(True, entries, errors, total_bytes)


def snapshot_protected() -> dict[str, RootSnapshot]:
    home = Path.home()
    return {
        "claude_config_tree": snapshot_root(home / ".claude"),
        "claude_config_file": snapshot_root(home / ".claude.json"),
        "claude_cache_library": snapshot_root(home / "Library/Caches/Claude Code"),
        "claude_cache_bundle": snapshot_root(
            home / "Library/Caches/com.anthropic.claudecode"
        ),
    }


def entry_core(entry: Entry) -> tuple[Any, ...]:
    return (
        entry.kind,
        entry.mode,
        entry.uid,
        entry.gid,
        entry.size,
        entry.digest,
        entry.link_target,
    )


def is_sensitive_relative(relative: str) -> bool:
    components = {part.lower() for part in Path(relative).parts}
    return bool(components & SENSITIVE_COMPONENTS)


def metadata_touch_allowed(label: str, relative: str, before: Entry) -> bool:
    if before.kind not in {"file", "directory"}:
        return False
    if is_sensitive_relative(relative):
        return False
    if label == "claude_config_file" and relative == ".":
        return True
    if label in {"claude_cache_library", "claude_cache_bundle"}:
        return True
    if label == "claude_config_tree":
        return relative == "cache" or relative.startswith("cache/")
    return False


def compare_protected(
    before: dict[str, RootSnapshot], after: dict[str, RootSnapshot]
) -> dict[str, Any]:
    forbidden = 0
    allowed_touches = 0
    errors = 0
    for label in sorted(set(before) | set(after)):
        left = before[label]
        right = after[label]
        errors += left.error_count + right.error_count
        if left.present != right.present:
            forbidden += 1
            continue
        if not left.present:
            continue
        if set(left.entries) != set(right.entries):
            forbidden += len(set(left.entries) ^ set(right.entries)) or 1
        for relative in sorted(set(left.entries) & set(right.entries)):
            old = left.entries[relative]
            new = right.entries[relative]
            if entry_core(old) != entry_core(new):
                forbidden += 1
                continue
            times_changed = (
                old.mtime_ns != new.mtime_ns or old.ctime_ns != new.ctime_ns
            )
            if times_changed:
                if metadata_touch_allowed(label, relative, old):
                    allowed_touches += 1
                else:
                    forbidden += 1
    return {
        "protectedStateUnchanged": forbidden == 0 and errors == 0,
        "metadataOnlyAllowlistSatisfied": forbidden == 0 and errors == 0,
        "forbiddenCategoryCount": forbidden,
        "allowedTouchCount": allowed_touches,
        "snapshotErrorCount": errors,
        "contentRetained": False,
    }


def compare_exact(before: RootSnapshot, after: RootSnapshot) -> bool:
    return before == after and before.error_count == 0


def repo_change_is_only_new_file(
    before: RootSnapshot, after: RootSnapshot, relative: str
) -> bool:
    if before.error_count or after.error_count or not before.present or not after.present:
        return False
    expected_names = set(before.entries) | {relative}
    if set(after.entries) != expected_names or relative in before.entries:
        return False
    created = after.entries.get(relative)
    if created is None or created.kind != "file" or created.mode != 0o600:
        return False
    ancestors = {"."}
    current = Path(relative).parent
    while current != Path("."):
        ancestors.add(current.as_posix())
        current = current.parent
    for name, old in before.entries.items():
        new = after.entries[name]
        if entry_core(old) != entry_core(new):
            return False
        if name not in ancestors and (
            old.mtime_ns != new.mtime_ns or old.ctime_ns != new.ctime_ns
        ):
            return False
    return True


def normalize_key(key: Any) -> str:
    return re.sub(r"[^a-z0-9]", "", str(key).lower())


def object_has_override(value: Any) -> bool:
    if isinstance(value, dict):
        for key, child in value.items():
            if normalize_key(key) in SETTINGS_OVERRIDE_KEYS:
                return True
            if object_has_override(child):
                return True
    elif isinstance(value, list):
        return any(object_has_override(child) for child in value)
    return False


def inspect_settings() -> dict[str, Any]:
    paths = (
        Path.home() / ".claude.json",
        Path.home() / ".claude/settings.json",
        Path.home() / ".claude/settings.local.json",
        Path("/Library/Application Support/ClaudeCode/managed-settings.json"),
    )
    inspected = 0
    parse_failures = 0
    override_detected = False
    for path in paths:
        if not path.is_file():
            continue
        inspected += 1
        try:
            raw = path.read_bytes()
            if len(raw) > 1024 * 1024:
                parse_failures += 1
                continue
            parsed = json.loads(raw.decode("utf-8"))
            override_detected = override_detected or object_has_override(parsed)
        except (OSError, UnicodeDecodeError, json.JSONDecodeError):
            parse_failures += 1
    return {
        "inspectedFileCount": inspected,
        "parseSucceeded": parse_failures == 0,
        "overrideAbsent": not override_detected,
    }


def env_is_clean(environment: dict[str, str]) -> bool:
    for name, value in environment.items():
        if not value:
            continue
        if name in AUTH_OVERRIDE_NAMES:
            return False
        upper = name.upper()
        if upper.startswith("ANTHROPIC_"):
            return False
        if upper.startswith("CLAUDE_") or upper.startswith("CLAUDE_CODE_"):
            return False
        if upper.startswith("AWS_") or upper.startswith("GOOGLE_"):
            return False
        if upper.startswith("CLOUD_ML_"):
            return False
        if upper.startswith("CLAUDE_CODE_USE_"):
            return False
    return True


def child_environment(parent: dict[str, str]) -> dict[str, str]:
    result = {
        name: parent[name]
        for name in INHERITED_ENV_ALLOWLIST
        if parent.get(name)
    }
    result.update(SAFE_ENV)
    return result


def process_group_exists(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
        return True
    except (ProcessLookupError, PermissionError):
        return False


def signal_group(pgid: int, sig: signal.Signals) -> bool:
    try:
        os.killpg(pgid, sig)
        return True
    except (ProcessLookupError, PermissionError):
        return False


def wait_exit(process: subprocess.Popen[bytes], seconds: float) -> bool:
    try:
        process.wait(timeout=max(0.0, seconds))
        return True
    except subprocess.TimeoutExpired:
        return False


def cleanup_group(
    process: subprocess.Popen[bytes], pgid: int, deadline: float
) -> str:
    if not process_group_exists(pgid):
        if process.poll() is None:
            try:
                process.wait(timeout=0.2)
            except subprocess.TimeoutExpired:
                return "ambiguous"
        return "already_exited"
    for sig, label, allowance in (
        (signal.SIGINT, "interrupted", 2.0),
        (signal.SIGTERM, "terminated", 2.0),
        (signal.SIGKILL, "killed", 2.0),
    ):
        if time.monotonic() >= deadline:
            return "deadline_exhausted"
        signal_group(pgid, sig)
        wait_until = min(deadline, time.monotonic() + allowance)
        while time.monotonic() < wait_until:
            if not process_group_exists(pgid):
                if process.poll() is None:
                    wait_exit(process, 0.2)
                return label
            time.sleep(0.025)
    return "ambiguous" if process_group_exists(pgid) else "killed"


def bounded_command(
    argv: list[str],
    *,
    env: dict[str, str],
    timeout: float,
    stdout_cap: int,
    stderr_cap: int,
    cwd: Path | None = None,
) -> dict[str, Any]:
    process = subprocess.Popen(
        argv,
        cwd=str(cwd) if cwd else None,
        env=env,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        start_new_session=True,
        close_fds=True,
    )
    deadline = time.monotonic() + timeout
    stdout = bytearray()
    stderr = bytearray()
    caps = {"stdout": stdout_cap, "stderr": stderr_cap}
    streams = {
        process.stdout.fileno(): ("stdout", process.stdout, stdout),
        process.stderr.fileno(): ("stderr", process.stderr, stderr),
    }
    for _, stream, _ in streams.values():
        os.set_blocking(stream.fileno(), False)
    open_fds = set(streams)
    timed_out = False
    cap_exceeded = False
    cleanup = "not_needed"
    while open_fds:
        if time.monotonic() >= deadline:
            timed_out = True
            cleanup = cleanup_group(process, process.pid, time.monotonic() + 6.0)
            break
        readable, _, _ = select.select(list(open_fds), [], [], 0.1)
        for descriptor in readable:
            name, _, buffer = streams[descriptor]
            try:
                chunk = os.read(descriptor, 4096)
            except BlockingIOError:
                continue
            if not chunk:
                open_fds.discard(descriptor)
                continue
            buffer.extend(chunk)
            if len(buffer) > caps[name]:
                cap_exceeded = True
                del buffer[caps[name] :]
                cleanup = cleanup_group(process, process.pid, time.monotonic() + 6.0)
                open_fds.clear()
                break
        if process.poll() is not None and not readable:
            for descriptor in list(open_fds):
                try:
                    chunk = os.read(descriptor, 4096)
                except (BlockingIOError, OSError):
                    chunk = b""
                if chunk:
                    name, _, buffer = streams[descriptor]
                    buffer.extend(chunk)
                    if len(buffer) > caps[name]:
                        cap_exceeded = True
                        del buffer[caps[name] :]
                else:
                    open_fds.discard(descriptor)
    if process.poll() is None or process_group_exists(process.pid):
        cleanup = cleanup_group(process, process.pid, time.monotonic() + 6.0)
    for stream in (process.stdout, process.stderr):
        if stream is not None and not stream.closed:
            stream.close()
    return {
        "stdout": bytes(stdout),
        "stderr": bytes(stderr),
        "exitCode": process.poll(),
        "timedOut": timed_out,
        "outputCapExceeded": cap_exceeded,
        "cleanup": cleanup,
    }


def auth_projection(raw: bytes) -> dict[str, Any]:
    projection = {
        "loggedIn": False,
        "authMethod": "unexpected",
        "apiProvider": "unexpected",
        "subscriptionType": "unexpected",
        "parseSucceeded": False,
    }
    try:
        value = json.loads(raw.decode("utf-8"))
        if not isinstance(value, dict):
            return projection
        projection = {
            "loggedIn": value.get("loggedIn") is True,
            "authMethod": (
                "claude.ai" if value.get("authMethod") == "claude.ai" else "unexpected"
            ),
            "apiProvider": (
                "firstParty"
                if value.get("apiProvider") == "firstParty"
                else "unexpected"
            ),
            "subscriptionType": (
                "team" if value.get("subscriptionType") == "team" else "unexpected"
            ),
            "parseSucceeded": True,
        }
    except (UnicodeDecodeError, json.JSONDecodeError):
        pass
    return projection


def expected_auth(projection: dict[str, Any]) -> bool:
    return projection == {
        "loggedIn": True,
        "authMethod": "claude.ai",
        "apiProvider": "firstParty",
        "subscriptionType": "team",
        "parseSucceeded": True,
    }


def strip_terminal(value: bytes) -> bytes:
    value = re.sub(rb"\x1b\[[0-?]*[ -/]*[@-~]", b"", value)
    value = re.sub(rb"\x1b\][^\x07]*(?:\x07|\x1b\\)", b"", value)
    return re.sub(rb"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", b"", value)


def detect_block_class(value: bytes) -> str | None:
    lowered = strip_terminal(value).lower()
    for classification, patterns in BLOCK_PATTERNS.items():
        if any(pattern in lowered for pattern in patterns):
            return classification
    return None


def public_fixture() -> tuple[str, bytes, str]:
    prompt = (
        "Join the four public test tokens TEAM, SUBSCRIPTION, PTY, and OK with "
        "underscores. Output only the joined string."
    )
    marker = b"TEAM_SUBSCRIPTION_PTY_OK"
    return prompt, marker, sha256_bytes(prompt.encode("ascii"))


def system_prompt() -> str:
    return (
        "This is a transport acceptance probe. Use no tools. Return only the "
        "requested synthetic marker."
    )


def live_argv(binary: Path) -> list[str]:
    prompt, _, _ = public_fixture()
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
        MODEL,
        "--system-prompt",
        system_prompt(),
        "--ax-screen-reader",
        prompt,
    ]


def argv_is_safe(argv: list[str]) -> bool:
    if any(flag in argv for flag in BANNED_LIVE_FLAGS):
        return False
    required = {
        "--safe-mode",
        "--no-chrome",
        "--disable-slash-commands",
        "--strict-mcp-config",
        "--tools",
        "--disallowedTools",
        "--permission-mode",
        "--model",
        "--system-prompt",
        "--ax-screen-reader",
    }
    return required.issubset(argv) and argv[-1] == public_fixture()[0]


def binary_tuple(requested: Path) -> tuple[Path, tuple[Any, ...], dict[str, Any]]:
    requested_lstat = requested.lstat()
    requested_target = os.readlink(requested) if stat.S_ISLNK(requested_lstat.st_mode) else None
    resolved = requested.resolve(strict=True)
    info = resolved.lstat()
    digest = sha256_file(resolved)
    tuple_value = (
        requested_lstat.st_dev,
        requested_lstat.st_ino,
        requested_lstat.st_mode,
        requested_lstat.st_uid,
        requested_lstat.st_gid,
        requested_lstat.st_size,
        requested_lstat.st_mtime_ns,
        requested_lstat.st_ctime_ns,
        requested_target,
        resolved,
        info.st_dev,
        info.st_ino,
        info.st_mode,
        info.st_uid,
        info.st_gid,
        info.st_size,
        info.st_mtime_ns,
        info.st_ctime_ns,
        digest,
    )
    owner_ok = info.st_uid in {0, os.getuid()}
    report = {
        "version": None,
        "binarySha256": digest,
        "exactTupleMatched": False,
        "resolvedRegularExecutable": stat.S_ISREG(info.st_mode)
        and os.access(resolved, os.X_OK),
        "ownerAllowed": owner_ok,
        "requestedResolutionStable": True,
    }
    return resolved, tuple_value, report


def collect_preflight(requested: Path) -> tuple[dict[str, Any], tuple[Any, ...], Path]:
    before_state = snapshot_protected()
    before_repo = snapshot_root(REPO_ROOT)
    try:
        resolved, pinned_tuple, claude = binary_tuple(requested)
    except (OSError, RuntimeError, SafeStop):
        return (
            {
                "ready": False,
                "failureClass": "HOST_OR_BINARY_PIN_FAILED",
                "platform": {
                    "system": platform.system(),
                    "osVersion": platform.mac_ver()[0],
                    "architecture": platform.machine(),
                },
            },
            tuple(),
            requested,
        )
    parent_clean = env_is_clean(dict(os.environ))
    child_env = child_environment(dict(os.environ))
    child_exact = set(child_env).issubset(
        set(INHERITED_ENV_ALLOWLIST) | set(SAFE_ENV)
    ) and all(child_env.get(key) == value for key, value in SAFE_ENV.items())
    settings = inspect_settings()
    system = platform.system()
    os_version = platform.mac_ver()[0]
    architecture = platform.machine()
    host_ok = (
        system == EXPECTED_SYSTEM
        and os_version == EXPECTED_OS_VERSION
        and architecture == EXPECTED_ARCH
    )
    binary_ok = (
        claude["binarySha256"] == EXPECTED_BINARY_SHA256
        and claude["resolvedRegularExecutable"]
        and claude["ownerAllowed"]
    )

    version_result = bounded_command(
        [str(resolved), "--version"],
        env=child_env,
        timeout=10.0,
        stdout_cap=4096,
        stderr_cap=4096,
    )
    version_match = re.search(
        rb"\b(\d+\.\d+\.\d+)\b", version_result["stdout"]
    )
    version = version_match.group(1).decode("ascii") if version_match else None
    claude["version"] = version

    help_result = bounded_command(
        [str(resolved), "--help"],
        env=child_env,
        timeout=10.0,
        stdout_cap=128 * 1024,
        stderr_cap=8192,
    )
    help_bytes = help_result["stdout"]
    flags_ok = all(flag.encode() in help_bytes for flag in REQUIRED_HELP_FLAGS)
    positional_ok = b"[prompt]" in help_bytes and b"prompt" in help_bytes.lower()

    auth_result = bounded_command(
        [str(resolved), "auth", "status", "--json"],
        env=child_env,
        timeout=15.0,
        stdout_cap=32 * 1024,
        stderr_cap=8192,
    )
    auth = auth_projection(auth_result["stdout"])

    after_state = snapshot_protected()
    after_repo = snapshot_root(REPO_ROOT)
    state_diff = compare_protected(before_state, after_state)
    repo_unchanged = compare_exact(before_repo, after_repo)
    command_ok = all(
        result["exitCode"] == 0
        and not result["timedOut"]
        and not result["outputCapExceeded"]
        for result in (version_result, help_result, auth_result)
    )
    ready = all(
        (
            host_ok,
            binary_ok,
            parent_clean,
            child_exact,
            settings["parseSucceeded"],
            settings["overrideAbsent"],
            version == EXPECTED_CLAUDE_VERSION,
            command_ok,
            flags_ok,
            positional_ok,
            expected_auth(auth),
            argv_is_safe(live_argv(resolved)),
            state_diff["protectedStateUnchanged"],
            repo_unchanged,
        )
    )
    claude.update(
        {
            "exactTupleMatched": host_ok
            and binary_ok
            and version == EXPECTED_CLAUDE_VERSION,
            "flagSurfaceSupported": flags_ok,
            "positionalPromptSupported": positional_ok,
        }
    )
    return (
        {
            "ready": ready,
            "failureClass": None if ready else "PREFLIGHT_FAILED",
            "platform": {
                "system": system,
                "osVersion": os_version,
                "architecture": architecture,
            },
            "claude": claude,
            "auth": auth,
            "environment": {
                "parentOverrideAbsent": parent_clean,
                "minimalChildEnvironment": child_exact,
            },
            "settings": settings,
            "flags": {
                "requiredSurface": flags_ok,
                "positionalPrompt": positional_ok,
                "liveArgvSafe": argv_is_safe(live_argv(resolved)),
            },
            "state": {
                **state_diff,
                "repositoryUnchanged": repo_unchanged,
            },
        },
        pinned_tuple,
        resolved,
    )


def revalidate_binary_tuple(requested: Path, expected: tuple[Any, ...]) -> bool:
    try:
        _, current, _ = binary_tuple(requested)
        return current == expected
    except (OSError, RuntimeError, SafeStop):
        return False


def set_terminal_size(descriptor: int, rows: int, columns: int) -> None:
    fcntl.ioctl(
        descriptor,
        termios.TIOCSWINSZ,
        struct.pack("HHHH", rows, columns, 0, 0),
    )


def spawn_pty_process(
    argv: list[str], env: dict[str, str], cwd: Path
) -> tuple[subprocess.Popen[bytes], int, int]:
    master, slave = pty.openpty()
    set_terminal_size(slave, 24, 80)
    try:
        process = subprocess.Popen(
            argv,
            cwd=str(cwd),
            env=env,
            stdin=slave,
            stdout=slave,
            stderr=slave,
            preexec_fn=os.setsid,
            close_fds=True,
        )
    except BaseException:
        os.close(master)
        os.close(slave)
        raise
    os.close(slave)
    os.set_blocking(master, False)
    return process, process.pid, master


def tty_fixture() -> dict[str, Any]:
    fixture_code = r'''
import fcntl, json, os, signal, struct, sys, termios, time
def size():
    return list(struct.unpack("HHHH", fcntl.ioctl(0, termios.TIOCGWINSZ, b"\0" * 8))[:2])
stats = [os.fstat(fd) for fd in (0, 1, 2)]
payload = {"tty": [os.isatty(fd) for fd in (0, 1, 2)], "same": len({(s.st_dev, s.st_ino, s.st_rdev) for s in stats}) == 1, "sizes": [size()]}
def resized(_sig, _frame):
    payload["sizes"].append(size())
    sys.stdout.write(json.dumps(payload) + "\n"); sys.stdout.flush()
signal.signal(signal.SIGWINCH, resized)
sys.stdout.write(json.dumps(payload) + "\n"); sys.stdout.flush()
deadline = time.monotonic() + 3
while len(payload["sizes"]) < 4 and time.monotonic() < deadline:
    time.sleep(0.02)
'''
    env = child_environment(dict(os.environ))
    with tempfile.TemporaryDirectory(prefix="mad-pty-v2-fixture-") as directory:
        process, pgid, master = spawn_pty_process(
            [sys.executable, "-B", "-c", fixture_code], env, Path(directory)
        )
        raw = bytearray()
        try:
            def latest_payload() -> dict[str, Any] | None:
                current: dict[str, Any] | None = None
                for line in strip_terminal(bytes(raw)).splitlines():
                    try:
                        candidate = json.loads(line.decode("utf-8"))
                        if isinstance(candidate, dict) and "tty" in candidate:
                            current = candidate
                    except (UnicodeDecodeError, json.JSONDecodeError):
                        continue
                return current

            def read_until_size_count(expected: int, timeout: float) -> bool:
                deadline = time.monotonic() + timeout
                while time.monotonic() < deadline:
                    parsed = latest_payload()
                    if parsed is not None and len(parsed.get("sizes", [])) >= expected:
                        return True
                    readable, _, _ = select.select([master], [], [], 0.05)
                    if readable:
                        try:
                            chunk = os.read(master, 4096)
                        except OSError:
                            chunk = b""
                        raw.extend(chunk)
                    if len(raw) > 16 * 1024:
                        return False
                return False

            ready = read_until_size_count(1, 1.0)
            for rows, columns in RESIZES:
                if not ready:
                    break
                previous = latest_payload()
                previous_count = len(previous.get("sizes", [])) if previous else 0
                set_terminal_size(master, rows, columns)
                signal_group(pgid, signal.SIGWINCH)
                ready = read_until_size_count(previous_count + 1, 0.8)
            deadline = time.monotonic() + 5.0
            while time.monotonic() < deadline and (
                process.poll() is None or len(raw) < 2
            ):
                readable, _, _ = select.select([master], [], [], 0.1)
                if readable:
                    try:
                        chunk = os.read(master, 4096)
                    except OSError:
                        chunk = b""
                    raw.extend(chunk)
                if len(raw) > 16 * 1024:
                    break
            wait_exit(process, 0.5)
            cleanup = (
                "self_exit"
                if not process_group_exists(pgid)
                else cleanup_group(process, pgid, time.monotonic() + 3.0)
            )
        finally:
            try:
                os.close(master)
            except OSError:
                pass
    parsed: dict[str, Any] | None = None
    for line in strip_terminal(bytes(raw)).splitlines():
        try:
            candidate = json.loads(line.decode("utf-8"))
            if isinstance(candidate, dict) and "tty" in candidate:
                parsed = candidate
        except (UnicodeDecodeError, json.JSONDecodeError):
            continue
    expected_sizes = [[24, 80], *[list(item) for item in RESIZES]]
    passed = bool(
        parsed
        and parsed.get("tty") == [True, True, True]
        and parsed.get("same") is True
        and parsed.get("sizes") == expected_sizes
        and cleanup == "self_exit"
    )
    return {
        "passed": passed,
        "stdinIsTty": bool(parsed and parsed.get("tty", [False])[0]),
        "stdoutIsTty": bool(parsed and parsed.get("tty", [False, False])[1]),
        "stderrIsTty": bool(parsed and parsed.get("tty", [False, False, False])[2]),
        "sameSlave": bool(parsed and parsed.get("same")),
        "resizeFixtureCount": len(RESIZES) if passed else 0,
        "processGroupCleanupPrimitive": cleanup == "self_exit",
        "rawContentRetained": False,
    }


def claim_ledger(path: Path = LEDGER_PATH) -> None:
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(path, flags, 0o600)
    payload = json.dumps(
        {
            "schemaVersion": 1,
            "target": TARGET,
            "arm": "pty",
            "attemptClaimed": True,
            "claimedBeforeProcessSpawn": True,
            "modelBearingCliProcessUpperBound": 1,
            "printAllowed": False,
            "retryAllowed": False,
            "billingSource": "claude_ai_team_included_subscription",
            "usageCreditsDisabledOperatorDirected": True,
            "rawContentRetained": False,
        },
        indent=2,
        sort_keys=True,
    ).encode("utf-8") + b"\n"
    try:
        view = memoryview(payload)
        while view:
            written = os.write(descriptor, view)
            if written <= 0:
                raise SafeStop("LEDGER_WRITE_FAILED")
            view = view[written:]
        os.fsync(descriptor)
        os.fchmod(descriptor, 0o600)
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
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600:
        raise SafeStop("LEDGER_MODE_FAILED")


def write_json_atomic(path: Path, payload: dict[str, Any]) -> None:
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    temporary = Path(temporary_name)
    try:
        os.fchmod(descriptor, 0o644)
        encoded = json.dumps(payload, indent=2, sort_keys=True).encode("utf-8") + b"\n"
        view = memoryview(encoded)
        while view:
            written = os.write(descriptor, view)
            view = view[written:]
        os.fsync(descriptor)
        os.close(descriptor)
        descriptor = -1
        os.replace(temporary, path)
        directory_flags = os.O_RDONLY
        if hasattr(os, "O_DIRECTORY"):
            directory_flags |= os.O_DIRECTORY
        directory_descriptor = os.open(path.parent, directory_flags)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        if descriptor >= 0:
            os.close(descriptor)
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def state_fixture_suite() -> bool:
    with tempfile.TemporaryDirectory(prefix="mad-state-v2-") as directory:
        root = Path(directory)
        allowed = root / "allowed"
        allowed.mkdir()
        value = allowed / "value"
        value.write_bytes(b"public fixture")
        before = {"claude_cache_library": snapshot_root(allowed)}
        os.utime(value, None)
        after = {"claude_cache_library": snapshot_root(allowed)}
        if not compare_protected(before, after)["metadataOnlyAllowlistSatisfied"]:
            return False

        before = {"claude_cache_library": snapshot_root(allowed)}
        value.write_bytes(b"changed")
        after = {"claude_cache_library": snapshot_root(allowed)}
        if compare_protected(before, after)["protectedStateUnchanged"]:
            return False

        cases: list[Callable[[Path], None]] = [
            lambda base: (base / "new").write_bytes(b"x"),
            lambda base: os.chmod(base / "value", 0o600),
            lambda base: (base / "link").symlink_to("value"),
            lambda base: os.mkfifo(base / "pipe"),
        ]
        for mutation in cases:
            case = root / f"case-{len(list(root.iterdir()))}"
            case.mkdir()
            (case / "value").write_bytes(b"fixture")
            left = {"claude_cache_library": snapshot_root(case)}
            mutation(case)
            right = {"claude_cache_library": snapshot_root(case)}
            if compare_protected(left, right)["protectedStateUnchanged"]:
                return False

        socket_root = root / "socket-case"
        socket_root.mkdir()
        left = {"claude_cache_library": snapshot_root(socket_root)}
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        try:
            sock.bind(str(socket_root / "socket"))
            right = {"claude_cache_library": snapshot_root(socket_root)}
        finally:
            sock.close()
        if compare_protected(left, right)["protectedStateUnchanged"]:
            return False

        synthetic_device = Entry("device", 0o600, 0, 0, 0, None, None, 0, 0)
        if metadata_touch_allowed("claude_cache_library", "device", synthetic_device):
            return False

        sensitive = root / "sensitive"
        sensitive.mkdir()
        history = sensitive / "history"
        history.mkdir()
        left = {"claude_cache_library": snapshot_root(sensitive)}
        os.utime(history, None)
        right = {"claude_cache_library": snapshot_root(sensitive)}
        if compare_protected(left, right)["protectedStateUnchanged"]:
            return False
    return True


def cleanup_fixture_suite() -> bool:
    process = subprocess.Popen(
        [
            sys.executable,
            "-B",
            "-c",
            (
                "import subprocess,sys,time; "
                "subprocess.Popen([sys.executable,'-B','-c','import time; time.sleep(30)']); "
                "time.sleep(30)"
            ),
        ],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        start_new_session=True,
        close_fds=True,
    )
    result = cleanup_group(process, process.pid, time.monotonic() + 6.0)
    return result in {"interrupted", "terminated", "killed"} and not process_group_exists(
        process.pid
    )


def bounded_command_fixture_suite() -> bool:
    environment = child_environment(dict(os.environ))
    timeout = bounded_command(
        [sys.executable, "-B", "-c", "import time; time.sleep(10)"],
        env=environment,
        timeout=0.1,
        stdout_cap=128,
        stderr_cap=128,
    )
    output = bounded_command(
        [sys.executable, "-B", "-c", "import sys; sys.stdout.write('x' * 4096)"],
        env=environment,
        timeout=2.0,
        stdout_cap=128,
        stderr_cap=128,
    )
    return bool(
        timeout["timedOut"]
        and timeout["cleanup"] in {"interrupted", "terminated", "killed", "already_exited"}
        and output["outputCapExceeded"]
        and len(output["stdout"]) == 128
    )


def repo_ledger_fixture_suite() -> bool:
    with tempfile.TemporaryDirectory(prefix="mad-repo-ledger-v2-") as directory:
        root = Path(directory)
        nested = root / "docs/spikes/fixture"
        nested.mkdir(parents=True)
        (root / "stable").write_bytes(b"stable")
        before = snapshot_root(root)
        ledger = nested / "ledger.json"
        claim_ledger(ledger)
        after = snapshot_root(root)
        relative = ledger.relative_to(root).as_posix()
        if not repo_change_is_only_new_file(before, after, relative):
            return False
        (root / "stable").write_bytes(b"changed")
        changed = snapshot_root(root)
        if repo_change_is_only_new_file(before, changed, relative):
            return False
    return True


def fixture_suite() -> dict[str, Any]:
    env_denials = all(
        not env_is_clean({name: "fixture"}) for name in sorted(AUTH_OVERRIDE_NAMES)
    )
    settings_denials = all(
        object_has_override({key: "fixture"}) for key in sorted(SETTINGS_OVERRIDE_KEYS)
    )
    auth_table = (
        expected_auth(
            auth_projection(
                b'{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"team"}'
            )
        )
        and not expected_auth(auth_projection(b"{}"))
        and not expected_auth(auth_projection(b"not-json"))
    )
    argv_test = argv_is_safe(live_argv(Path("/public/claude")))
    tty = tty_fixture()
    state = state_fixture_suite()
    with tempfile.TemporaryDirectory(prefix="mad-ledger-v2-") as directory:
        ledger = Path(directory) / "ledger.json"
        claim_ledger(ledger)
        mode_ok = stat.S_IMODE(ledger.stat().st_mode) == 0o600
        exclusive = False
        try:
            claim_ledger(ledger)
        except FileExistsError:
            exclusive = True
    sanitizer = (
        detect_block_class(b"\x1b[31mUsage credits required\x1b[0m") == "billing"
        and detect_block_class(b"Please log in") == "auth"
        and detect_block_class(b"Do you trust this folder?") == "trust"
        and detect_block_class(b"Approve tool use") == "tool"
        and detect_block_class(b"Opening browser") == "browser"
        and detect_block_class(b"TEAM_SUBSCRIPTION_PTY_OK") is None
    )
    cleanup = cleanup_fixture_suite()
    bounded = bounded_command_fixture_suite()
    repo_ledger = repo_ledger_fixture_suite()
    tests = {
        "environmentDenialTable": env_denials,
        "settingsDenialTable": settings_denials,
        "authProjectionTable": auth_table,
        "liveArgvContract": argv_test,
        "ttyFixture": tty["passed"],
        "stateClassifierFixtures": state,
        "ledgerExclusiveCreateAndMode": mode_ok and exclusive,
        "sanitizerFixtures": sanitizer,
        "processGroupCleanupFixture": cleanup,
        "timeoutAndOutputCapFixtures": bounded,
        "repositoryLedgerAllowlistFixture": repo_ledger,
        "zeroPrintPath": "--print" not in live_argv(Path("/public/claude")),
        "noRetryPath": True,
    }
    return {
        "passed": all(tests.values()),
        "tests": tests,
        "tty": tty,
        "modelBearingProcessSpawned": False,
        "printProcessCount": 0,
        "retryCount": 0,
        "rawContentRetained": False,
    }


def execute_live(
    binary: Path,
    env: dict[str, str],
    protected_before: dict[str, RootSnapshot],
    repo_before: RootSnapshot,
) -> tuple[dict[str, Any], dict[str, Any]]:
    prompt, marker, fixture_sha = public_fixture()
    del prompt
    started = time.monotonic()
    deadline = started + LIVE_DEADLINE_SECONDS
    process: subprocess.Popen[bytes] | None = None
    pgid: int | None = None
    master: int | None = None
    rolling = bytearray()
    total_bytes = 0
    marker_matched = False
    block_class: str | None = None
    resize_count = 0
    eof_sent = False
    timed_out = False
    output_cap = False
    cleanup = "not_started"
    exit_code: int | None = None
    workspace_unchanged = False

    with tempfile.TemporaryDirectory(prefix="mad-claude-pty-v2-") as directory:
        workspace = Path(directory)
        workspace_before = snapshot_root(workspace)
        try:
            # Exactly one model-bearing spawn site exists in execute_live.
            process, pgid, master = spawn_pty_process(live_argv(binary), env, workspace)

            def read_once(wait: float) -> None:
                nonlocal total_bytes, block_class, output_cap
                assert master is not None
                readable, _, _ = select.select([master], [], [], max(0.0, wait))
                if not readable:
                    return
                try:
                    chunk = os.read(master, 4096)
                except (BlockingIOError, OSError):
                    return
                total_bytes += len(chunk)
                rolling.extend(chunk)
                if len(rolling) > ROLLING_BYTES:
                    del rolling[: len(rolling) - ROLLING_BYTES]
                if total_bytes > OUTPUT_CAP_BYTES:
                    output_cap = True
                detected = detect_block_class(bytes(rolling))
                if detected:
                    block_class = detected

            for rows, columns in RESIZES:
                read_once(0.05)
                if process.poll() is not None or block_class or output_cap:
                    break
                set_terminal_size(master, rows, columns)
                signal_group(pgid, signal.SIGWINCH)
                resize_count += 1
                read_once(0.05)

            while (
                time.monotonic() < deadline - 8.0
                and process.poll() is None
                and not block_class
                and not output_cap
            ):
                read_once(0.15)
                if marker in strip_terminal(bytes(rolling)):
                    marker_matched = True
                    break

            if process.poll() is not None and not marker_matched:
                drain_until = min(deadline - 8.0, time.monotonic() + 0.5)
                while time.monotonic() < drain_until:
                    read_once(0.05)
                marker_matched = marker in strip_terminal(bytes(rolling))

            if time.monotonic() >= deadline - 8.0 and process.poll() is None:
                timed_out = True

            if marker_matched and not block_class and not output_cap and process.poll() is None:
                try:
                    os.write(master, b"\x04")
                    eof_sent = True
                except OSError:
                    eof_sent = False
                clean_until = min(deadline, time.monotonic() + 5.0)
                while time.monotonic() < clean_until and process.poll() is None:
                    read_once(0.1)
                if process.poll() is not None and not process_group_exists(pgid):
                    cleanup = "clean_eof_exit"

            if process.poll() is None or process_group_exists(pgid):
                cleanup = cleanup_group(process, pgid, deadline)
            elif cleanup == "not_started":
                cleanup = "self_exit"
            exit_code = process.poll()
        finally:
            if process is not None and pgid is not None and (
                process.poll() is None or process_group_exists(pgid)
            ):
                cleanup = cleanup_group(process, pgid, deadline)
            if master is not None:
                try:
                    os.close(master)
                except OSError:
                    pass
            workspace_after = snapshot_root(workspace)
            workspace_unchanged = compare_exact(workspace_before, workspace_after)

    protected_after = snapshot_protected()
    repo_after = snapshot_root(REPO_ROOT)
    state_diff = compare_protected(protected_before, protected_after)
    repo_unchanged = compare_exact(repo_before, repo_after)
    process_group_clear = bool(pgid is not None and not process_group_exists(pgid))
    duration_ms = int((time.monotonic() - started) * 1000)

    if block_class == "billing":
        classification = "PROVIDER_LIMIT_NO_RETRY"
    elif block_class is not None:
        classification = "INCONCLUSIVE_SAFE_STOP"
    elif not state_diff["protectedStateUnchanged"] or not workspace_unchanged or not repo_unchanged:
        classification = "LOCAL_STATE_POLICY_FAIL"
    elif timed_out or output_cap or not marker_matched or resize_count != len(RESIZES):
        classification = "PTY_NEGATIVE"
    elif not eof_sent or cleanup not in {"clean_eof_exit", "self_exit"}:
        classification = "PTY_NEGATIVE"
    elif not process_group_clear or exit_code not in {0, None}:
        classification = "PTY_NEGATIVE"
    else:
        classification = "SUPPORTED_EXACT_PTY"

    return (
        {
            "classification": classification,
            "markerMatched": marker_matched,
            "exitClass": "zero" if exit_code == 0 else "nonzero_or_signal",
            "timedOut": timed_out,
            "outputCapExceeded": output_cap,
            "cleanupClass": cleanup,
            "durationMs": duration_ms,
            "totalByteCount": min(total_bytes, OUTPUT_CAP_BYTES + 1),
            "resizeCount": resize_count,
            "positionalDeliveryClass": "initial_positional_argument",
            "blockerClass": block_class,
            "eofSentAfterMarker": eof_sent,
            "processGroupClear": process_group_clear,
            "fixtureSha256": fixture_sha,
        },
        {
            "workspaceUnchanged": workspace_unchanged,
            **state_diff,
            "repositoryUnchanged": repo_unchanged,
        },
    )


def sanitized_evidence(
    preflight: dict[str, Any], fixtures: dict[str, Any], pty_result: dict[str, Any], state: dict[str, Any]
) -> dict[str, Any]:
    result = pty_result["classification"]
    return {
        "schemaVersion": 1,
        "recordedAt": utc_offset_timestamp(),
        "target": TARGET,
        "scope": "direct_official_cli_external_experimental_exact_macos_pty_only",
        "result": result,
        "platform": preflight["platform"],
        "claude": {
            "version": preflight["claude"]["version"],
            "binarySha256": preflight["claude"]["binarySha256"],
            "exactTupleRevalidatedBeforeLedger": True,
            "exactTupleRevalidatedBeforeSpawn": True,
            "flagSurfaceSupported": preflight["claude"]["flagSurfaceSupported"],
        },
        "auth": {
            "loggedIn": preflight["auth"]["loggedIn"],
            "authMethod": preflight["auth"]["authMethod"],
            "apiProvider": preflight["auth"]["apiProvider"],
            "subscriptionType": preflight["auth"]["subscriptionType"],
            "apiKeyOverridePresent": False,
            "authTokenOverridePresent": False,
            "cloudCredentialOverridePresent": False,
            "networkOverridePresent": False,
            "settingsOverridePresent": False,
        },
        "billing": {
            "source": "claude_ai_team_included_subscription",
            "usageCreditsDisabledOperatorDirected": True,
            "dollarBudgetFlagUsed": False,
        },
        "controls": {
            "toolsDisabled": True,
            "mcpDisabled": True,
            "slashCommandsDisabled": True,
            "browserDisabled": True,
            "promptHistoryDisabled": True,
            "sessionReuseDisabled": True,
            "minimalEnvironment": True,
            "rawAuthCaptureRetained": False,
            "rawTerminalCaptureRetained": False,
            "runtimePromptCaptureRetained": False,
            "runtimeResponseCaptureRetained": False,
        },
        "attempt": {
            "ledgerClaimed": True,
            "ledgerMode": "0600",
            "claimedBeforeProcessSpawn": True,
            "processSpawned": True,
            "modelBearingProcessCount": 1,
            "printProcessCount": 0,
            "retryPerformed": False,
            "providerNetworkRequestCount": None,
        },
        "tty": fixtures["tty"],
        "pty": pty_result,
        "stateDiff": {**state, "contentRetained": False},
        "retention": {
            "providerSide": "Anthropic and the official Claude Code CLI may process or retain the public synthetic request under the Team subscription policy.",
            "localRawAuthRetained": False,
            "localRawTerminalRetained": False,
            "localPromptResponseCaptureRetained": False,
            "perPathOrPerFileDigestRetained": False,
        },
        "claims": {
            "exactPlatformVersionEvidence": True,
            "stableManagedSupport": False,
            "phase3Satisfied": False,
            "linuxSupported": False,
            "windowsSupported": False,
            "usageCapability": False,
            "billingCapability": False,
            "accountCapability": False,
        },
        "fallback": "Use the official Claude Code CLI directly outside the MultiAgentDesk managed surface. Any further experiment requires a new lifecycle and fresh ledger.",
    }


def emit(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, indent=2, sort_keys=True))


def main() -> int:
    args = parse_args()
    install_signal_handlers()

    if args.fixtures:
        result = fixture_suite()
        emit({"schemaVersion": 1, "mode": "fixtures", **result})
        return 0 if result["passed"] else 2

    requested = Path(args.claude).expanduser()
    fixtures = fixture_suite()
    if not fixtures["passed"]:
        emit(
            {
                "schemaVersion": 1,
                "mode": "preflight" if args.preflight else "execute",
                "status": "BLOCKED_PRE_REQUEST",
                "failureClass": "ZERO_MODEL_FIXTURE_FAILED",
                "ledgerClaimed": False,
                "modelBearingProcessSpawned": False,
                "printProcessCount": 0,
                "retryCount": 0,
                "rawContentRetained": False,
            }
        )
        return 2

    preflight, pinned_tuple, binary = collect_preflight(requested)
    sanitized_preflight = {
        "schemaVersion": 1,
        "mode": "preflight" if args.preflight else "execute",
        "status": "READY" if preflight.get("ready") else "BLOCKED_PRE_REQUEST",
        "preflight": preflight,
        "fixtures": fixtures,
        "ledgerClaimed": False,
        "modelBearingProcessSpawned": False,
        "printProcessCount": 0,
        "retryCount": 0,
        "rawContentRetained": False,
    }
    if args.preflight:
        emit(sanitized_preflight)
        return 0 if preflight.get("ready") else 2
    if not preflight.get("ready"):
        emit(sanitized_preflight)
        return 2
    if not args.usage_credits_disabled_operator_directed:
        sanitized_preflight["status"] = "BLOCKED_PRE_REQUEST"
        sanitized_preflight["failureClass"] = "USAGE_CREDITS_DIRECTION_MISSING"
        emit(sanitized_preflight)
        return 2
    if LEDGER_PATH.exists():
        sanitized_preflight["status"] = "BLOCKED_PRE_REQUEST"
        sanitized_preflight["failureClass"] = "ONE_SHOT_LEDGER_ALREADY_EXISTS"
        emit(sanitized_preflight)
        return 2
    if not revalidate_binary_tuple(requested, pinned_tuple):
        sanitized_preflight["status"] = "BLOCKED_PRE_REQUEST"
        sanitized_preflight["failureClass"] = "BINARY_TUPLE_DRIFT_BEFORE_LEDGER"
        emit(sanitized_preflight)
        return 2

    protected_before_ledger = snapshot_protected()
    repo_before_ledger = snapshot_root(REPO_ROOT)
    claim_ledger()

    second_preflight, second_tuple, second_binary = collect_preflight(requested)
    ledger_mode_ok = (
        LEDGER_PATH.is_file() and stat.S_IMODE(LEDGER_PATH.stat().st_mode) == 0o600
    )
    after_ledger_state = snapshot_protected()
    after_ledger_repo = snapshot_root(REPO_ROOT)
    relative_ledger = LEDGER_PATH.relative_to(REPO_ROOT).as_posix()
    ledger_only_repo_change = repo_change_is_only_new_file(
        repo_before_ledger, after_ledger_repo, relative_ledger
    )
    prespawn_state = compare_protected(protected_before_ledger, after_ledger_state)
    prespawn_ready = all(
        (
            second_preflight.get("ready"),
            second_tuple == pinned_tuple,
            second_binary == binary,
            ledger_mode_ok,
            ledger_only_repo_change,
            prespawn_state["protectedStateUnchanged"],
            env_is_clean(dict(os.environ)),
            argv_is_safe(live_argv(binary)),
        )
    )
    if not prespawn_ready:
        sanitized_preflight.update(
            {
                "status": "BLOCKED_PRE_REQUEST",
                "failureClass": "PRESPAWN_REVALIDATION_FAILED_AFTER_LEDGER",
                "ledgerClaimed": True,
                "modelBearingProcessSpawned": False,
            }
        )
        emit(sanitized_preflight)
        return 2

    # Revalidate the executable tuple immediately adjacent to the sole spawn.
    if not revalidate_binary_tuple(requested, pinned_tuple):
        sanitized_preflight.update(
            {
                "status": "BLOCKED_PRE_REQUEST",
                "failureClass": "BINARY_TUPLE_DRIFT_BEFORE_SPAWN",
                "ledgerClaimed": True,
                "modelBearingProcessSpawned": False,
            }
        )
        emit(sanitized_preflight)
        return 2

    live_protected_before = snapshot_protected()
    live_repo_before = snapshot_root(REPO_ROOT)
    pty_result, state = execute_live(
        binary, child_environment(dict(os.environ)), live_protected_before, live_repo_before
    )
    evidence = sanitized_evidence(second_preflight, fixtures, pty_result, state)
    write_json_atomic(EVIDENCE_JSON_PATH, evidence)
    emit(
        {
            "schemaVersion": 1,
            "status": evidence["result"],
            "ledgerClaimed": True,
            "modelBearingProcessSpawned": True,
            "modelBearingProcessCount": 1,
            "printProcessCount": 0,
            "retryCount": 0,
            "evidenceWritten": True,
            "rawContentRetained": False,
        }
    )
    return 0 if evidence["result"] == "SUPPORTED_EXACT_PTY" else 1


def guarded_main() -> int:
    try:
        return main()
    except Interrupted:
        emit(
            {
                "schemaVersion": 1,
                "status": "INCONCLUSIVE_SAFE_STOP",
                "ledgerClaimed": LEDGER_PATH.exists(),
                "modelBearingProcessUpperBound": 1 if LEDGER_PATH.exists() else 0,
                "printProcessCount": 0,
                "retryCount": 0,
                "rawContentRetained": False,
            }
        )
        return 1
    except FileExistsError:
        emit(
            {
                "schemaVersion": 1,
                "status": "BLOCKED_PRE_REQUEST",
                "failureClass": "ONE_SHOT_LEDGER_ALREADY_EXISTS",
                "ledgerClaimed": LEDGER_PATH.exists(),
                "modelBearingProcessSpawned": False,
                "printProcessCount": 0,
                "retryCount": 0,
                "rawContentRetained": False,
            }
        )
        return 2
    except (Exception, KeyboardInterrupt):
        emit(
            {
                "schemaVersion": 1,
                "status": "INCONCLUSIVE_SAFE_STOP" if LEDGER_PATH.exists() else "BLOCKED_PRE_REQUEST",
                "failureClass": "HARNESS_INTERNAL_ERROR",
                "ledgerClaimed": LEDGER_PATH.exists(),
                "modelBearingProcessUpperBound": 1 if LEDGER_PATH.exists() else 0,
                "printProcessCount": 0,
                "retryCount": 0,
                "rawContentRetained": False,
            }
        )
        return 1 if LEDGER_PATH.exists() else 2


if __name__ == "__main__":
    raise SystemExit(guarded_main())
