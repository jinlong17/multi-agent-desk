#!/usr/bin/env python3
"""Compile a tiny WKWebView app and verify key persistence across app processes."""

from __future__ import annotations

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tempfile
import threading
from typing import Any


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--swiftc", default="xcrun")
    parser.add_argument("--output", type=Path)
    parser.add_argument(
        "--poc-dir",
        type=Path,
        default=Path(__file__).with_name("poc"),
    )
    parser.add_argument(
        "--harness-dir",
        type=Path,
        default=Path(__file__).with_name("webkit-probe"),
    )
    return parser.parse_args()


def compile_app(swiftc: str, source: Path, plist: Path, root: Path) -> Path:
    contents = root / "BrowserKeyProbe.app" / "Contents"
    executable = contents / "MacOS" / "BrowserKeyProbe"
    executable.parent.mkdir(parents=True)
    shutil.copy2(plist, contents / "Info.plist")
    compiler = [swiftc, "swiftc"] if Path(swiftc).name == "xcrun" else [swiftc]
    process = subprocess.run(
        [
            *compiler,
            str(source),
            "-o",
            str(executable),
            "-framework",
            "AppKit",
            "-framework",
            "WebKit",
        ],
        capture_output=True,
        text=True,
        timeout=120,
    )
    if process.returncode != 0:
        raise RuntimeError(f"Swift compile failed: {process.stderr.strip()}")
    return executable


def run_phase(executable: Path, base_url: str, phase: str) -> dict[str, Any]:
    process = subprocess.run(
        [str(executable), f"{base_url}?phase={phase}"],
        check=True,
        capture_output=True,
        text=True,
        timeout=60,
        env={**os.environ},
    )
    lines = [line for line in process.stdout.splitlines() if line.strip()]
    if not lines:
        raise RuntimeError(f"WebKit harness returned no result for {phase}")
    return json.loads(lines[-1])


def main() -> int:
    args = parse_args()
    handler = partial(QuietHandler, directory=str(args.poc_dir.resolve()))
    server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base_url = f"http://127.0.0.1:{server.server_port}/index.html"

    try:
        with tempfile.TemporaryDirectory(prefix="mad-webkit-key-probe-") as temporary:
            executable = compile_app(
                args.swiftc,
                (args.harness_dir / "main.swift").resolve(),
                (args.harness_dir / "Info.plist").resolve(),
                Path(temporary),
            )
            write = run_phase(executable, base_url, "write")
            read = run_phase(executable, base_url, "read")
            cleanup = run_phase(executable, base_url, "cleanup")
    finally:
        server.shutdown()
        server.server_close()

    output = {
        "schemaVersion": 1,
        "browser": "WKWebView on macOS",
        "osVersion": platform.mac_ver()[0],
        "processRestarted": True,
        "writeComplete": bool(write.get("writeComplete")),
        "cleanupComplete": bool(cleanup.get("cleanupComplete")),
        "probe": read,
    }
    rendered = json.dumps(output, indent=2, sort_keys=True)
    print(rendered)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(f"{rendered}\n", encoding="utf-8")
    return 0 if output["writeComplete"] and read.get("e2eeEligible") else 3


if __name__ == "__main__":
    raise SystemExit(main())
