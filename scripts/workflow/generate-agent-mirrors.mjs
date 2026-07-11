import { readFileSync, writeFileSync, mkdirSync, cpSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { codexAgent, claudeAgent } from "./render-mirrors.mjs";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const registryPath = resolve(repoRoot, ".agents/registry.json");
const registry = JSON.parse(readFileSync(registryPath, "utf8"));

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
