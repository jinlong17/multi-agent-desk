import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const manualPath = resolve(repoRoot, "docs/workflow/project/dashboard-state.json");
const modulePath = resolve(repoRoot, "docs/workflow/project/module-registry.json");
const agentPath = resolve(repoRoot, ".agents/registry.json");
const outputPath = resolve(repoRoot, "docs/prototypes/dev-dashboard/state.generated.js");
const outputRelative = relative(repoRoot, outputPath).replaceAll("\\", "/");

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function git(args, fallback = "unknown") {
  try {
    return execFileSync("git", args, { cwd: repoRoot, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  } catch {
    return fallback;
  }
}

function dirtyFiles() {
  const raw = git(["status", "--porcelain=v1"], "");
  return raw ? raw.split(/\r?\n/).filter(Boolean).filter(line => !line.endsWith(outputRelative)) : [];
}

function requiredDoc(path, label) {
  const absolute = resolve(repoRoot, path);
  return { path, label, exists: existsSync(absolute), bytes: existsSync(absolute) ? readFileSync(absolute).byteLength : 0 };
}

function featureLogs() {
  const root = resolve(repoRoot, "docs/workflow/features");
  if (!existsSync(root)) return [];
  return readdirSync(root, { withFileTypes: true }).filter(entry => entry.isDirectory()).map(entry => {
    const path = resolve(root, entry.name, "dev_log.md");
    if (!existsSync(path)) return { slug: entry.name, status: "MISSING_DEV_LOG", path: relative(repoRoot, path) };
    const text = readFileSync(path, "utf8");
    const field = name => text.match(new RegExp("\\|\\s*" + name + "\\s*\\|\\s*`?([^|`]+)`?\\s*\\|", "i"))?.[1]?.trim() || "unknown";
    return {
      slug: entry.name,
      title: field("Title"),
      owner_module: field("Owner Module"),
      phase: field("Current Phase"),
      status: field("Status"),
      executor: field("Executor"),
      updated: field("Updated"),
      suggested_next: field("Suggested Next"),
      path: relative(repoRoot, path)
    };
  });
}

const manual = readJson(manualPath);
const modules = readJson(modulePath);
const agentRegistry = readJson(agentPath);
const dirty = dirtyFiles();
const requiredDocs = [
  requiredDoc("docs/IMPLEMENTATION_PLAN.md", "Implementation Plan"),
  requiredDoc("docs/USER_GUIDE.md", "User operations guide"),
  requiredDoc("docs/workflow/project/workflow.md", "Workflow policy"),
  requiredDoc("docs/workflow/project/dev-dashboard.md", "Dashboard contract"),
  requiredDoc("docs/workflow/project/module-registry.json", "Module registry"),
  requiredDoc("docs/workflow/FILE_STRUCTURE.md", "Workflow file structure"),
  requiredDoc(".agents/registry.json", "Agent registry"),
  requiredDoc("AGENTS.md", "Codex rules"),
  requiredDoc("CLAUDE.md", "Shared project rules")
];

const agentEntries = agentRegistry.agents.map(agent => ({
  ...agent,
  role_source: `.agents/roles/${agent.name}.md`,
  codex_path: `.codex/agents/${agent.name}.toml`,
  claude_path: `.claude/agents/${agent.name}.md`,
  role_exists: existsSync(resolve(repoRoot, `.agents/roles/${agent.name}.md`)),
  codex_exists: existsSync(resolve(repoRoot, `.codex/agents/${agent.name}.toml`)),
  claude_exists: existsSync(resolve(repoRoot, `.claude/agents/${agent.name}.md`))
}));

const skillEntries = agentRegistry.skills.map(skill => ({
  ...skill,
  kind: "skill",
  codex_exists: existsSync(resolve(repoRoot, skill.source)),
  claude_path: `.claude/skills/${skill.name}/SKILL.md`,
  claude_exists: existsSync(resolve(repoRoot, `.claude/skills/${skill.name}/SKILL.md`))
}));

const state = {
  schema_version: 1,
  state_owner: "project-system",
  generated_at: new Date().toISOString(),
  repo_root: repoRoot,
  git: {
    branch: git(["branch", "--show-current"]),
    latest_commit: git(["log", "-1", "--format=%h %s"]),
    head: git(["rev-parse", "HEAD"]),
    dirty: { total: dirty.length, files: dirty },
    remote: git(["remote", "get-url", "origin"], "not-configured")
  },
  manual,
  modules: modules.modules,
  workflows: agentRegistry.workflows,
  skill_agent_registry: {
    agents: agentEntries,
    skills: skillEntries,
    summary: {
      agents: agentEntries.length,
      skills: skillEntries.length,
      complete_agent_mirrors: agentEntries.filter(item => item.role_exists && item.codex_exists && item.claude_exists).length,
      complete_skill_mirrors: skillEntries.filter(item => item.codex_exists && item.claude_exists).length
    }
  },
  feature_logs: featureLogs(),
  required_docs: requiredDocs,
  commands: {
    refresh: "npm run dashboard",
    verify: "npm run project:verify",
    serve: "npm run dashboard:serve"
  }
};

writeFileSync(outputPath, `window.MULTI_AGENT_DESK_DASHBOARD_STATE = ${JSON.stringify(state, null, 2)};\n`);
console.log(`generated ${outputRelative}: branch=${state.git.branch}, dirty=${dirty.length}, agents=${agentEntries.length}, skills=${skillEntries.length}`);
