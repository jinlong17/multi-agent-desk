#!/usr/bin/env python3
"""Run the browser-key PoC through a real Chrome, Edge, Firefox, or Safari."""

from __future__ import annotations

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import tempfile
import threading
import time
from typing import Any

from selenium import webdriver
from selenium.common.exceptions import TimeoutException
from selenium.webdriver.support.ui import WebDriverWait


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--browser",
        required=True,
        choices=("chrome", "edge", "firefox", "safari"),
    )
    parser.add_argument("--browser-name", required=True)
    parser.add_argument("--binary")
    parser.add_argument("--output", type=Path)
    parser.add_argument(
        "--poc-dir",
        type=Path,
        default=Path(__file__).with_name("poc"),
    )
    return parser.parse_args()


def create_driver(browser: str, profile: str, binary: str | None) -> Any:
    if browser == "chrome":
        options = webdriver.ChromeOptions()
        options.add_argument("--headless=new")
        options.add_argument(f"--user-data-dir={profile}")
        if binary:
            options.binary_location = binary
        return webdriver.Chrome(options=options)
    if browser == "edge":
        options = webdriver.EdgeOptions()
        options.add_argument("--headless=new")
        options.add_argument(f"--user-data-dir={profile}")
        if binary:
            options.binary_location = binary
        return webdriver.Edge(options=options)
    if browser == "firefox":
        options = webdriver.FirefoxOptions()
        options.add_argument("-headless")
        options.add_argument("-profile")
        options.add_argument(profile)
        if binary:
            options.binary_location = binary
        return webdriver.Firefox(options=options)
    return webdriver.Safari(options=webdriver.SafariOptions())


def run_phase(
    browser: str,
    profile: str,
    binary: str | None,
    base_url: str,
    phase: str,
) -> tuple[str, dict[str, Any]]:
    driver = create_driver(browser, profile, binary)
    try:
        driver.set_page_load_timeout(20)
        driver.get(f"{base_url}?phase={phase}")
        try:
            WebDriverWait(driver, 45).until(
                lambda current: current.execute_script(
                    "return document.documentElement.dataset.probeComplete === 'true'"
                )
            )
        except TimeoutException as error:
            stage = driver.execute_script(
                "return document.documentElement.dataset.probeStage || 'unknown'"
            )
            raise RuntimeError(f"browser probe timed out at {stage}") from error
        result = json.loads(driver.find_element("id", "result").text)
        version = driver.capabilities.get(
            "browserVersion", driver.capabilities.get("version", "unknown")
        )
        return str(version), result
    finally:
        driver.quit()


def read_result(driver: Any, url: str) -> dict[str, Any]:
    driver.set_page_load_timeout(20)
    driver.get(url)
    try:
        WebDriverWait(driver, 45).until(
            lambda current: current.execute_script(
                "return document.documentElement.dataset.probeComplete === 'true'"
            )
        )
    except TimeoutException as error:
        stage = driver.execute_script(
            "return document.documentElement.dataset.probeStage || 'unknown'"
        )
        raise RuntimeError(f"browser probe timed out at {stage}") from error
    return json.loads(driver.find_element("id", "result").text)


def run_isolated_safari_session(
    base_url: str,
) -> tuple[str, dict[str, Any], dict[str, Any]]:
    """Use one session because Safari isolates WebDriver runs from one another."""
    driver = create_driver("safari", "", None)
    try:
        write = read_result(driver, f"{base_url}?phase=write")
        driver.get("about:blank")
        read = read_result(driver, f"{base_url}?phase=read")
        version = driver.capabilities.get(
            "browserVersion", driver.capabilities.get("version", "unknown")
        )
        return str(version), write, read
    finally:
        driver.quit()


def main() -> int:
    args = parse_args()
    handler = partial(QuietHandler, directory=str(args.poc_dir.resolve()))
    server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base_url = f"http://127.0.0.1:{server.server_port}/index.html"

    try:
        if args.browser == "safari":
            read_version, write, read = run_isolated_safari_session(base_url)
            write_version = read_version
            process_restarted = False
        else:
            with tempfile.TemporaryDirectory(prefix="mad-browser-key-profile-") as profile:
                write_version, write = run_phase(
                    args.browser, profile, args.binary, base_url, "write"
                )
                time.sleep(1)
                read_version, read = run_phase(
                    args.browser, profile, args.binary, base_url, "read"
                )
            process_restarted = True
    finally:
        server.shutdown()
        server.server_close()

    output = {
        "schemaVersion": 1,
        "browser": args.browser_name,
        "browserVersion": read_version,
        "versionStableAcrossRestart": write_version == read_version,
        "processRestarted": process_restarted,
        "writeComplete": bool(write.get("writeComplete")),
        "probe": read,
    }
    if args.browser == "safari":
        output["webdriverSessionIsolated"] = True
        output["restartEvidence"] = "separate WebKit process probe required"
    rendered = json.dumps(output, indent=2, sort_keys=True)
    print(rendered)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(f"{rendered}\n", encoding="utf-8")
    return 0 if output["writeComplete"] and read.get("e2eeEligible") else 3


if __name__ == "__main__":
    raise SystemExit(main())
