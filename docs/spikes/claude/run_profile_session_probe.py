#!/usr/bin/env python3
"""Run a sanitized Claude profile auth and resumable-session probe."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
import subprocess
import time
from typing import Any


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--claude", required=True)
    parser.add_argument("--duration-seconds", type=float, default=60)
    parser.add_argument("--interval-seconds", type=float, default=10)
    return parser.parse_args()


def run_json(
    command: list[str], environment: dict[str, str], timeout: float = 120
) -> tuple[int, dict[str, Any] | None]:
    process = subprocess.run(
        command,
        capture_output=True,
        text=True,
        timeout=timeout,
        env=environment,
    )
    try:
        payload = json.loads(process.stdout)
    except (json.JSONDecodeError, TypeError):
        payload = None
    return process.returncode, payload


def response_ok(payload: dict[str, Any] | None, marker: str) -> bool:
    return bool(
        payload
        and payload.get("is_error") is not True
        and str(payload.get("result", "")).strip() == marker
    )


def response_classification(
    exit_code: int, payload: dict[str, Any] | None, marker: str
) -> dict[str, bool | int]:
    result = str(payload.get("result", "")) if payload else ""
    lowered = result.lower()
    return {
        "exit_code": exit_code,
        "payload_present": payload is not None,
        "is_error": bool(payload and payload.get("is_error") is True),
        "marker_exact": result.strip() == marker,
        "quota_or_limit_message": any(
            value in lowered
            for value in ("usage limit", "session limit", "rate limit", "quota")
        ),
        "authentication_error": any(
            value in lowered
            for value in ("authentication_error", "invalid authentication", "not logged in")
        ),
    }


def main() -> int:
    args = parse_args()
    environment = dict(os.environ)
    environment.pop("ANTHROPIC_API_KEY", None)
    environment.pop("CLAUDE_CODE_OAUTH_TOKEN", None)

    status_code, status = run_json(
        [args.claude, "auth", "status", "--json"], environment
    )
    status_ok = bool(status_code == 0 and status and status.get("loggedIn") is True)

    marker = "MAD_CLAUDE_PROFILE_TURN_1_OK"
    first_code, first = run_json(
        [
            args.claude,
            "-p",
            f"Reply with exactly {marker}",
            "--output-format",
            "json",
        ],
        environment,
    )
    turn_results = [first_code == 0 and response_ok(first, marker)]
    session_id = str(first.get("session_id", "")) if first else ""
    started = time.monotonic()
    turn_index = 2

    while (
        session_id
        and all(turn_results)
        and time.monotonic() - started < args.duration_seconds
    ):
        time.sleep(args.interval_seconds)
        marker = f"MAD_CLAUDE_PROFILE_TURN_{turn_index}_OK"
        code, payload = run_json(
            [
                args.claude,
                "-p",
                f"Reply with exactly {marker}",
                "--resume",
                session_id,
                "--output-format",
                "json",
            ],
            environment,
        )
        turn_results.append(code == 0 and response_ok(payload, marker))
        turn_index += 1

    output = {
        "schema_version": 1,
        "recorded_at": datetime.now(timezone.utc).isoformat(),
        "sanitized": True,
        "secret_values_recorded": False,
        "profile_auth_status": {
            "exit_zero": status_code == 0,
            "logged_in": status_ok,
            "json_keys": sorted(status.keys()) if status else [],
        },
        "resumable_session": {
            "requested_duration_seconds": args.duration_seconds,
            "observed_duration_seconds": round(time.monotonic() - started, 3),
            "session_id_present": bool(session_id),
            "turns_attempted": len(turn_results),
            "turns_succeeded": sum(turn_results),
            "same_session_resumed": len(turn_results) > 1,
            "first_turn": response_classification(first_code, first, marker="MAD_CLAUDE_PROFILE_TURN_1_OK"),
        },
    }
    print(json.dumps(output, indent=2, sort_keys=True))
    return 0 if status_ok and len(turn_results) > 1 and all(turn_results) else 3


if __name__ == "__main__":
    raise SystemExit(main())
