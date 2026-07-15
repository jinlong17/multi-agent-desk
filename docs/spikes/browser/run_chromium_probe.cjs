#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const http = require("node:http");
const os = require("node:os");
const path = require("node:path");
const {chromium} = require("playwright");

function parseArgs(argv) {
  const values = {};
  for (let index = 2; index < argv.length; index += 2) {
    const name = argv[index];
    const value = argv[index + 1];
    if (!name?.startsWith("--") || value === undefined) {
      throw new Error(`invalid argument near ${name ?? "<end>"}`);
    }
    values[name.slice(2)] = value;
  }
  for (const required of ["browser-name", "binary"]) {
    if (!values[required]) {
      throw new Error(`missing --${required}`);
    }
  }
  values["poc-dir"] ??= path.join(__dirname, "poc");
  return values;
}

function startServer(pocDirectory) {
  const server = http.createServer((request, response) => {
    const url = new URL(request.url, "http://127.0.0.1");
    if (url.pathname === "/favicon.ico") {
      response.writeHead(204).end();
      return;
    }
    if (url.pathname !== "/" && url.pathname !== "/index.html") {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
    });
    fs.createReadStream(path.join(pocDirectory, "index.html")).pipe(response);
  });
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => resolve(server));
  });
}

async function runPhase({binary, profile, url, phase}) {
  const diagnostics = [];
  const context = await chromium.launchPersistentContext(profile, {
    executablePath: binary,
    headless: true,
    args: [
      "--disable-background-networking",
      "--disable-component-update",
      "--no-first-run",
      "--no-default-browser-check",
      "--no-proxy-server",
    ],
  });
  try {
    const page = context.pages()[0] ?? await context.newPage();
    page.on("console", (message) => {
      if (message.type() === "error") diagnostics.push(`console:${message.text()}`);
    });
    page.on("pageerror", (error) => diagnostics.push(`page:${error.name}`));
    await page.goto(`${url}?phase=${phase}`, {
      waitUntil: "domcontentloaded",
      timeout: 15_000,
    });
    await page.waitForFunction(
      () => document.documentElement.dataset.probeComplete === "true",
      undefined,
      {timeout: 45_000},
    );
    const result = JSON.parse(await page.locator("#result").textContent());
    return {
      browserVersion: context.browser()?.version() ?? "unknown",
      diagnostics,
      result,
    };
  } catch (error) {
    const page = context.pages()[0];
    const stage = page
      ? await page.evaluate(() => document.documentElement.dataset.probeStage ?? "unknown")
          .catch(() => "unavailable")
      : "no-page";
    throw new Error(`${error.name}: ${error.message}; stage=${stage}`);
  } finally {
    await context.close();
  }
}

async function main() {
  const args = parseArgs(process.argv);
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), "mad-browser-key-profile-"));
  const server = await startServer(path.resolve(args["poc-dir"]));
  const address = server.address();
  const url = `http://127.0.0.1:${address.port}/index.html`;
  try {
    const write = await runPhase({
      binary: args.binary,
      profile,
      url,
      phase: "write",
    });
    const read = await runPhase({
      binary: args.binary,
      profile,
      url,
      phase: "read",
    });
    const output = {
      schemaVersion: 1,
      browser: args["browser-name"],
      browserVersion: read.browserVersion,
      processRestarted: true,
      writeComplete: Boolean(write.result.writeComplete),
      probe: read.result,
      diagnostics: [...write.diagnostics, ...read.diagnostics],
    };
    process.stdout.write(`${JSON.stringify(output, null, 2)}\n`);
    process.exitCode = output.writeComplete && output.probe.e2eeEligible ? 0 : 3;
  } finally {
    await new Promise((resolve) => server.close(resolve));
    fs.rmSync(profile, {recursive: true, force: true});
  }
}

main().catch((error) => {
  process.stderr.write(`${error.stack ?? error}\n`);
  process.exitCode = 2;
});
