import { readFileSync, writeFileSync, mkdirSync, cpSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const registryPath = resolve(repoRoot, ".agents/registry.json");
const registry = JSON.parse(readFileSync(registryPath, "utf8"));

function quote(value) {
  return JSON.stringify(String(value));
}

function codexAgent(agent) {
  return `# GENERATED from .agents/registry.json and .agents/roles/${agent.name}.md.\n` +
    `# Edit the source files, then run npm run workflow:generate.\n` +
    `name = ${quote(agent.name)}\n` +
    `description = ${quote(agent.description)}\n` +
    `sandbox_mode = ${quote(agent.sandbox)}\n` +
    `model_reasoning_effort = "high"\n` +
    `developer_instructions = '''\n` +
    `You are the MultiAgentDesk ${agent.name} role. Before acting, read AGENTS.md, CLAUDE.md, docs/workflow/project/workflow.md, and .agents/roles/${agent.name}.md completely. Follow the role boundaries, state transitions, and exact Handoff contract in that role file. Re-read repository state instead of relying on chat memory.\n` +
    `'''\n`;
}

function claudeAgent(agent) {
  return `---\n` +
    `name: ${agent.name}\n` +
    `description: ${agent.description}\n` +
    `tools: ${agent.tools}\n` +
    `---\n\n` +
    `<!-- GENERATED from .agents/registry.json. Edit the source and regenerate. -->\n\n` +
    `Read \`.agents/roles/${agent.name}.md\`, \`AGENTS.md\`, \`CLAUDE.md\`, and ` +
    `\`docs/workflow/project/workflow.md\` completely. Execute the role exactly, ` +
    `including its write/read-only boundary, state transition, and final \`## Handoff\` contract. ` +
    `Re-read repository state instead of relying on chat memory.\n`;
}

const codexDir = resolve(repoRoot, ".codex/agents");
const claudeDir = resolve(repoRoot, ".claude/agents");
mkdirSync(codexDir, { recursive: true });
mkdirSync(claudeDir, { recursive: true });

for (const agent of registry.agents) {
  const rolePath = resolve(repoRoot, `.agents/roles/${agent.name}.md`);
  readFileSync(rolePath, "utf8");
  writeFileSync(resolve(codexDir, `${agent.name}.toml`), codexAgent(agent));
  writeFileSync(resolve(claudeDir, `${agent.name}.md`), claudeAgent(agent));
}

for (const skill of registry.skills) {
  const sourceDir = dirname(resolve(repoRoot, skill.source));
  const targetDir = resolve(repoRoot, `.claude/skills/${skill.name}`);
  mkdirSync(targetDir, { recursive: true });
  cpSync(resolve(sourceDir, "SKILL.md"), resolve(targetDir, "SKILL.md"));
}

console.log(`generated ${registry.agents.length} Codex agents, ${registry.agents.length} Claude agents, and ${registry.skills.length} Claude skill mirrors`);
