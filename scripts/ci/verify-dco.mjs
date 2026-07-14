import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
const value = flag => {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : undefined;
};
const trailer = /^Signed-off-by:\s+\S(?:.*\S)?\s+<[^<>@\s]+@[^<>\s]+>\s*$/im;
const fullSha = /^[0-9a-f]{40}$/;

function subject(message) {
  return message.trim().split(/\r?\n/, 1)[0];
}

function validateExceptions(exceptions) {
  const seen = new Set();
  for (const exception of exceptions) {
    if (!fullSha.test(exception.sha)) throw new Error(`DCO exception SHA must be full length: ${exception.sha}`);
    if (seen.has(exception.sha)) throw new Error(`duplicate DCO exception: ${exception.sha}`);
    if (!exception.subject?.trim()) throw new Error(`DCO exception subject missing: ${exception.sha}`);
    if (!exception.reason?.trim()) throw new Error(`DCO exception reason missing: ${exception.sha}`);
    seen.add(exception.sha);
  }
  return exceptions;
}

function validateLiveConfig(config, head) {
  if (config.schema_version !== 1) throw new Error(`unsupported DCO exception schema: ${config.schema_version}`);
  if (!fullSha.test(config.policy_effective_commit)) throw new Error("DCO policy effective commit must be a full SHA");
  const exceptions = validateExceptions(config.exceptions ?? []);
  const policyMessage = execFileSync("git", ["show", "-s", "--format=%B", config.policy_effective_commit], { encoding: "utf8" });
  if (!trailer.test(policyMessage)) throw new Error("DCO policy effective commit is not signed off");
  execFileSync("git", ["merge-base", "--is-ancestor", config.policy_effective_commit, head], { stdio: "ignore" });
  for (const exception of exceptions) {
    execFileSync("git", ["merge-base", "--is-ancestor", exception.sha, config.policy_effective_commit], { stdio: "ignore" });
    if (exception.sha === config.policy_effective_commit) throw new Error("DCO policy commit cannot grandfather itself");
    const actualSubject = execFileSync("git", ["show", "-s", "--format=%s", exception.sha], { encoding: "utf8" }).trim();
    if (actualSubject !== exception.subject) throw new Error(`DCO exception subject drift: ${exception.sha}`);
  }
  return exceptions;
}

function verify(commits, exceptions = []) {
  if (!commits.length) throw new Error("DCO range contains no commits");
  const bySha = new Map(validateExceptions(exceptions).map(exception => [exception.sha, exception]));
  const grandfathered = [];
  const failures = commits.filter(commit => {
    if (trailer.test(commit.message)) return false;
    const exception = bySha.get(commit.sha);
    if (exception && exception.subject === subject(commit.message)) {
      grandfathered.push(commit.sha);
      return false;
    }
    return true;
  });
  if (failures.length) {
    throw new Error(`DCO missing or malformed for: ${failures.map(commit => commit.sha).join(", ")}`);
  }
  console.log(`verified DCO: commits=${commits.length} grandfathered=${grandfathered.length}`);
}

const fixture = value("--fixture");
if (fixture) {
  const data = JSON.parse(readFileSync(fixture, "utf8"));
  verify(data.commits, data.grandfathered ?? []);
} else {
  let base = value("--base") || process.env.BASE_SHA;
  const head = value("--head") || process.env.HEAD_SHA || "HEAD";
  if (!base || /^0+$/.test(base)) base = `${head}^`;
  const configPath = value("--grandfathered") || "scripts/ci/dco-grandfathered.json";
  const config = JSON.parse(readFileSync(configPath, "utf8"));
  const exceptions = validateLiveConfig(config, head);
  const output = execFileSync("git", ["log", "--format=%H%x00%B%x00%x1e", `${base}..${head}`], { encoding: "utf8" });
  const commits = output.split("\x1e").map(record => record.trim()).filter(Boolean).map(record => {
    const [sha, message] = record.split("\x00");
    return { sha: sha.trim(), message: message.trim() };
  });
  verify(commits, exceptions);
}
