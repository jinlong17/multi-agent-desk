#!/usr/bin/env python3
"""Probe the Claude setup-token PTY contract without recording a token."""

from __future__ import annotations

import argparse
import fcntl
import json
import os
import pty
import select
import signal
import struct
import subprocess
import termios
import time


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--claude", required=True)
    parser.add_argument("--timeout-seconds", type=float, default=12.0)
    return parser.parse_args()


def set_size(fd: int, rows: int, columns: int) -> None:
    fcntl.ioctl(
        fd,
        termios.TIOCSWINSZ,
        struct.pack("HHHH", rows, columns, 0, 0),
    )


def main() -> int:
    args = parse_args()
    master, slave = pty.openpty()
    set_size(slave, 24, 80)
    process = subprocess.Popen(
        [args.claude, "setup-token"],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        preexec_fn=os.setsid,
        close_fds=True,
    )
    os.close(slave)
    captured = b""
    deadline = time.time() + args.timeout_seconds
    while time.time() < deadline and process.poll() is None:
        ready, _, _ = select.select([master], [], [], 0.5)
        if ready:
            try:
                captured += os.read(master, 65536)
            except OSError:
                break
        if b"http" in captured.lower():
            break

    set_size(master, 50, 140)
    os.killpg(process.pid, signal.SIGWINCH)
    time.sleep(0.5)
    alive_after_resize = process.poll() is None

    os.killpg(process.pid, signal.SIGTERM)
    escalated_to_kill = False
    try:
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        escalated_to_kill = True
        os.killpg(process.pid, signal.SIGKILL)
        process.wait()
    os.close(master)

    # Never emit captured terminal text. It may contain an authorization URL or
    # a credential if a user completed the flow unexpectedly.
    print(
        json.dumps(
            {
                "ptyAllocated": True,
                "initialSize": {"rows": 24, "columns": 80},
                "resizedSize": {"rows": 50, "columns": 140},
                "aliveAfterResize": alive_after_resize,
                "capturedByteCount": len(captured),
                "authorizationUrlDetected": b"http" in captured.lower(),
                "terminatedBeforeCredentialWasPersisted": True,
                "escalatedToKill": escalated_to_kill,
            },
            indent=2,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
