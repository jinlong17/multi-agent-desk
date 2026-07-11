import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const registry = JSON.parse(readFileSync(resolve(repoRoot, ".agents/registry.json"), "utf8"));

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

const requiredDocs = [
  "AGENTS.md",
  "CLAUDE.md",
  "docs/IMPLEMENTATION_PLAN.md",
  "docs/workflow/project/workflow.md",
  "docs/workflow/project/dev-dashboard.md",
  "docs/workflow/project/dashboard-state.json",
  "docs/workflow/project/module-registry.json",
  "docs/workflow/FILE_STRUCTURE.md",
  "docs/workflow/templates/feature-brief.md",
  "docs/workflow/templates/design.md",
  "docs/workflow/templates/api.md",
  "docs/workflow/templates/test.md",
  "docs/workflow/templates/dev_log.md"
];

requiredDocs.forEach(path => assert(existsSync(resolve(repoRoot, path)), `missing required workflow file: ${path}`));
assert(registry.schema_version === 1, "unsupported agent registry schema");
assert(new Set(registry.agents.map(agent => agent.name)).size === registry.agents.length, "agent names must be unique");

for (const [workflow, names] of Object.entries(registry.workflows)) {
  names.forEach(name => assert(registry.agents.some(agent => agent.name === name), `${workflow} references unknown agent ${name}`));
}

for (const agent of registry.agents) {
  const role = `.agents/roles/${agent.name}.md`;
  const codex = `.codex/agents/${agent.name}.toml`;
  const claude = `.claude/agents/${agent.name}.md`;
  [role, codex, claude].forEach(path => assert(existsSync(resolve(repoRoot, path)), `missing agent file: ${path}`));
  const roleText = readFileSync(resolve(repoRoot, role), "utf8");
  const codexText = readFileSync(resolve(repoRoot, codex), "utf8");
  const claudeText = readFileSync(resolve(repoRoot, claude), "utf8");
  assert(roleText.includes("## Handoff"), `${role} missing Handoff contract`);
  assert(codexText.includes(role), `${codex} does not point to its authority role`);
  assert(claudeText.includes(role), `${claude} does not point to its authority role`);
  assert(codexText.startsWith("# GENERATED"), `${codex} missing generated marker`);
  assert(claudeText.includes("GENERATED"), `${claude} missing generated marker`);
}

for (const skill of registry.skills) {
  const source = resolve(repoRoot, skill.source);
  const mirror = resolve(repoRoot, `.claude/skills/${skill.name}/SKILL.md`);
  assert(existsSync(source), `missing source skill: ${skill.source}`);
  assert(existsSync(mirror), `missing Claude skill mirror: ${skill.name}`);
  const sourceText = readFileSync(source, "utf8");
  assert(!sourceText.includes("TODO"), `${skill.source} still contains TODO`);
  assert(sourceText === readFileSync(mirror, "utf8"), `Claude skill mirror drift: ${skill.name}`);
}

const workflowText = readFileSync(resolve(repoRoot, "docs/workflow/project/workflow.md"), "utf8");
["NEEDS_REVIEW", "APPROVED", "READY_FOR_VERIFY", "READY_TO_SHIP", "SHIPPED", "BLOCKED"].forEach(status => {
  assert(workflowText.includes(status), `workflow policy missing status ${status}`);
});

console.log(`verified workflow: agents=${registry.agents.length}, skills=${registry.skills.length}, docs=${requiredDocs.length}`);
