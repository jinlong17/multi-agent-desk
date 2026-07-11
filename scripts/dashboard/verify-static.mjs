import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const dashboardDir = resolve(repoRoot, "docs/prototypes/dev-dashboard");
const htmlPath = resolve(dashboardDir, "index.html");
const statePath = resolve(dashboardDir, "state.generated.js");
const stateRelative = relative(repoRoot, statePath).replaceAll("\\", "/");

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function git(args) {
  return execFileSync("git", args, { cwd: repoRoot, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
}

function dirtyCount() {
  const output = git(["status", "--porcelain=v1"]);
  return output ? output.split(/\r?\n/).filter(Boolean).filter(line => !line.endsWith(stateRelative)).length : 0;
}

assert(existsSync(htmlPath), "dashboard index.html is missing");
assert(existsSync(statePath), "state.generated.js is missing; run npm run dashboard");
const html = readFileSync(htmlPath, "utf8");
const generated = readFileSync(statePath, "utf8");
const match = generated.match(/^window\.MULTI_AGENT_DESK_DASHBOARD_STATE\s*=\s*([\s\S]*);\s*$/);
assert(match, "generated state must assign window.MULTI_AGENT_DESK_DASHBOARD_STATE");
const state = JSON.parse(match[1]);

assert(html.includes('<script src="./state.generated.js"></script>'), "dashboard must load state.generated.js");
["repoValue", "branchValue", "statusValue", "snapshotValue", "railStatus", "railStatusNote", "timeline", "workflowFlow", "skillRegistry", "phaseTable"].forEach(id => {
  assert(new RegExp(`id=["']${id}["']`).test(html), `dashboard missing required id ${id}`);
});
assert(state.schema_version === 1, "unsupported dashboard state schema");
assert(state.state_owner === "project-system", "dashboard state owner must be project-system");
assert(state.git.branch === git(["branch", "--show-current"]), "generated branch is stale");
assert(state.git.head === git(["rev-parse", "HEAD"]), "generated commit is stale");
assert(state.git.dirty.total === dirtyCount(), "generated dirty count is stale; run npm run dashboard");
assert(state.manual?.current_phase, "manual current phase is missing");
assert(Array.isArray(state.manual?.phases) && state.manual.phases.length >= 8, "manual phase registry is incomplete");
assert(Array.isArray(state.modules) && state.modules.length === 7, "module registry must contain seven owning modules");
assert(state.skill_agent_registry.summary.complete_agent_mirrors === state.skill_agent_registry.summary.agents, "agent runtime mirrors are incomplete");
assert(state.skill_agent_registry.summary.complete_skill_mirrors === state.skill_agent_registry.summary.skills, "skill runtime mirrors are incomplete");
assert(state.required_docs.every(doc => doc.exists), "one or more required dashboard documents are missing");
assert(Array.isArray(state.feature_logs), "feature_logs must be an array");
for (const log of state.feature_logs) {
  assert(log.status !== "MISSING_DEV_LOG", `feature directory ${log.slug} has no dev_log.md`);
  ["title", "owner_module", "phase", "status", "executor", "updated", "suggested_next"].forEach(field => {
    assert(log[field] && log[field] !== "unknown", `feature ${log.slug} dev_log.md is missing Status Panel field ${field}`);
  });
}
// Static fallback must use canonical workflow semantics.
["FIX_READY", "spike-intake", "只读"].forEach(token => {
  assert(!html.includes(token), `dashboard static fallback contains stale token "${token}"`);
});
["DIAGNOSED", "READY_FOR_VERIFY", "SPIKE_READY", "GATE_RESOLVED", "ACCEPTED"].forEach(token => {
  assert(html.includes(token), `dashboard static fallback missing canonical status ${token}`);
});
const focus = state.manual?.focus;
assert(Array.isArray(focus) && focus.length > 0, "manual focus must bind at least one {slug, expected_status} target");
for (const target of focus) {
  const log = state.feature_logs.find(item => item.slug === target.slug);
  assert(log, `manual focus names unknown feature ${target.slug}`);
  assert(log.status === target.expected_status,
    `manual focus for ${target.slug} expects ${target.expected_status} but its dev_log says ${log.status}; refresh dashboard-state.json (operator or next writer role)`);
}
assert(!/token_value|access_token|refresh_token|cookie_value|session_secret/i.test(generated), "generated dashboard state may contain a secret-bearing field");

console.log(`verified dashboard: branch=${state.git.branch}, dirty=${state.git.dirty.total}, phases=${state.manual.phases.length}, agents=${state.skill_agent_registry.summary.agents}, skills=${state.skill_agent_registry.summary.skills}`);
