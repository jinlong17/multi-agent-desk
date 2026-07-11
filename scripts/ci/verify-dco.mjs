import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
const value = flag => {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : undefined;
};
const trailer = /^Signed-off-by:\s+\S(?:.*\S)?\s+<[^<>@\s]+@[^<>\s]+>\s*$/im;

function verify(commits) {
  if (!commits.length) throw new Error("DCO range contains no commits");
  const failures = commits.filter(commit => !trailer.test(commit.message));
  if (failures.length) {
    throw new Error(`DCO missing or malformed for: ${failures.map(commit => commit.sha).join(", ")}`);
  }
  console.log(`verified DCO: commits=${commits.length}`);
}

const fixture = value("--fixture");
if (fixture) {
  verify(JSON.parse(readFileSync(fixture, "utf8")).commits);
} else {
  let base = value("--base") || process.env.BASE_SHA;
  const head = value("--head") || process.env.HEAD_SHA || "HEAD";
  if (!base || /^0+$/.test(base)) base = `${head}^`;
  const output = execFileSync("git", ["log", "--format=%H%x00%B%x00%x1e", `${base}..${head}`], { encoding: "utf8" });
  const commits = output.split("\x1e").map(record => record.trim()).filter(Boolean).map(record => {
    const [sha, message] = record.split("\x00");
    return { sha: sha.trim(), message: message.trim() };
  });
  verify(commits);
}
