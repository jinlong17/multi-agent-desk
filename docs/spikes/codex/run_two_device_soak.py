#!/usr/bin/env python3
"""Run a sanitized two-device Codex auth/app-server soak.

The harness compares account identifiers only in memory. Its JSONL output never
contains email addresses, account identifiers, tokens, rate-limit values, usage
values, auth-file hashes, or stderr text.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import select
import subprocess
import time
from typing import Any


CLIENT_INFO = {
    "name": "multi_agent_desk_spike",
    "title": "MultiAgentDesk Provider Spike",
    "version": "0.1.0",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--local-codex", required=True)
    parser.add_argument("--remote-host", required=True)
    parser.add_argument("--remote-codex", required=True)
    parser.add_argument("--remote-home", required=True)
    parser.add_argument("--duration-hours", type=float, default=48.0)
    parser.add_argument("--interval-seconds", type=float, default=3600.0)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args()


def send(process: subprocess.Popen[str], payload: dict[str, Any]) -> None:
    assert process.stdin is not None
    process.stdin.write(json.dumps(payload, separators=(",", ":")) + "\n")
    process.stdin.flush()


def receive_id(
    process: subprocess.Popen[str], request_id: int, timeout: float = 60.0
) -> dict[str, Any]:
    assert process.stdout is not None
    deadline = time.time() + timeout
    while time.time() < deadline:
        ready, _, _ = select.select(
            [process.stdout], [], [], max(0.0, deadline - time.time())
        )
        if not ready:
            break
        line = process.stdout.readline()
        if not line:
            break
        try:
            message = json.loads(line)
        except json.JSONDecodeError:
            continue
        if message.get("id") == request_id:
            return message
    return {"error": {"code": "timeout"}}


def response_ok(message: dict[str, Any]) -> bool:
    return "error" not in message


def error_code(message: dict[str, Any]) -> str | int | None:
    error = message.get("error")
    return error.get("code") if isinstance(error, dict) else None


def local_auth_digest() -> str:
    auth_path = Path.home() / ".codex" / "auth.json"
    return hashlib.sha256(auth_path.read_bytes()).hexdigest()


def remote_auth_digest(ssh: list[str], remote_home: str) -> str:
    output = subprocess.check_output(
        ssh + [f"sha256sum {remote_home}/.codex/auth.json"],
        text=True,
        stderr=subprocess.DEVNULL,
        timeout=30,
    )
    return output.split()[0]


def account_email(message: dict[str, Any]) -> str | None:
    result = message.get("result")
    if not isinstance(result, dict):
        return None
    account = result.get("account")
    if not isinstance(account, dict):
        return None
    email = account.get("email")
    return email if isinstance(email, str) else None


def start_clients(args: argparse.Namespace) -> tuple[
    subprocess.Popen[str], subprocess.Popen[str], list[str]
]:
    ssh = [
        "ssh",
        "-o",
        "BatchMode=yes",
        "-o",
        "ConnectTimeout=10",
        args.remote_host,
    ]
    local = subprocess.Popen(
        [args.local_codex, "app-server", "--stdio"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        bufsize=1,
    )
    remote = subprocess.Popen(
        ssh + [f"{args.remote_codex} app-server --stdio"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        bufsize=1,
    )
    return local, remote, ssh


def stop_clients(*processes: subprocess.Popen[str]) -> None:
    for process in processes:
        try:
            process.terminate()
        except ProcessLookupError:
            pass
    for process in processes:
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()


def run_sample(
    args: argparse.Namespace, sequence: int, elapsed_seconds: float, refresh: bool
) -> dict[str, Any]:
    local, remote, ssh = start_clients(args)
    try:
        for process in (local, remote):
            send(
                process,
                {"id": 0, "method": "initialize", "params": {"clientInfo": CLIENT_INFO}},
            )
        local_init = receive_id(local, 0)
        remote_init = receive_id(remote, 0)

        for process in (local, remote):
            send(process, {"method": "initialized", "params": {}})
            send(
                process,
                {"id": 1, "method": "account/read", "params": {"refreshToken": False}},
            )
        local_account = receive_id(local, 1)
        remote_account = receive_id(remote, 1)
        local_email = account_email(local_account)
        remote_email = account_email(remote_account)

        event: dict[str, Any] = {
            "kind": "sample",
            "sequence": sequence,
            "recordedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "elapsedSeconds": round(elapsed_seconds, 3),
            "localInitializeOk": response_ok(local_init),
            "remoteInitializeOk": response_ok(remote_init),
            "localAccountReadOk": response_ok(local_account),
            "remoteAccountReadOk": response_ok(remote_account),
            "sameAccount": bool(local_email and remote_email and local_email == remote_email),
            "refreshRequested": refresh,
        }

        if refresh and event["sameAccount"]:
            local_before = local_auth_digest()
            remote_before = remote_auth_digest(ssh, args.remote_home)
            started = time.time()
            send(
                local,
                {"id": 2, "method": "account/read", "params": {"refreshToken": True}},
            )
            send(
                remote,
                {"id": 2, "method": "account/read", "params": {"refreshToken": True}},
            )
            local_refresh = receive_id(local, 2)
            remote_refresh = receive_id(remote, 2)
            event["concurrentRefresh"] = {
                "localOk": response_ok(local_refresh),
                "remoteOk": response_ok(remote_refresh),
                "localErrorCode": error_code(local_refresh),
                "remoteErrorCode": error_code(remote_refresh),
                "elapsedSeconds": round(time.time() - started, 3),
                "localAuthFileChanged": local_auth_digest() != local_before,
                "remoteAuthFileChanged": remote_auth_digest(ssh, args.remote_home)
                != remote_before,
            }

        request_id = 3
        for method in ("account/rateLimits/read", "account/usage/read"):
            send(local, {"id": request_id, "method": method})
            send(remote, {"id": request_id, "method": method})
            local_response = receive_id(local, request_id)
            remote_response = receive_id(remote, request_id)
            key = "rateLimits" if method.endswith("rateLimits/read") else "usage"
            event[key] = {
                "localOk": response_ok(local_response),
                "remoteOk": response_ok(remote_response),
                "localErrorCode": error_code(local_response),
                "remoteErrorCode": error_code(remote_response),
            }
            request_id += 1
        return event
    finally:
        stop_clients(local, remote)


def append_event(path: Path, event: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(event, sort_keys=True) + "\n")
        handle.flush()
        os.fsync(handle.fileno())


def main() -> int:
    args = parse_args()
    started = time.time()
    duration_seconds = args.duration_hours * 3600.0
    append_event(
        args.output,
        {
            "kind": "started",
            "recordedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "durationHours": args.duration_hours,
            "intervalSeconds": args.interval_seconds,
            "sanitized": True,
        },
    )
    sequence = 0
    while True:
        elapsed = time.time() - started
        final_sample = elapsed >= duration_seconds
        try:
            event = run_sample(
                args,
                sequence=sequence,
                elapsed_seconds=elapsed,
                refresh=(sequence == 0 or final_sample),
            )
        except Exception as error:  # evidence must record failures without raw text
            event = {
                "kind": "sample-error",
                "sequence": sequence,
                "recordedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                "elapsedSeconds": round(elapsed, 3),
                "errorType": type(error).__name__,
            }
        append_event(args.output, event)
        sequence += 1
        if final_sample:
            break
        remaining = duration_seconds - (time.time() - started)
        time.sleep(max(0.0, min(args.interval_seconds, remaining)))

    append_event(
        args.output,
        {
            "kind": "completed",
            "recordedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "elapsedSeconds": round(time.time() - started, 3),
            "sampleCount": sequence,
        },
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
