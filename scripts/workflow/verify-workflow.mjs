import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { codexAgent, claudeAgent } from "./render-mirrors.mjs";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const registry = JSON.parse(readFileSync(resolve(repoRoot, ".agents/registry.json"), "utf8"));
const moduleRegistry = JSON.parse(readFileSync(resolve(repoRoot, "docs/workflow/project/module-registry.json"), "utf8"));
const moduleKeys = new Set(moduleRegistry.modules.map(mod => mod.key));
const agentNames = registry.agents.map(agent => agent.name);

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function setEqual(a, b) {
  return a.size === b.size && [...a].every(item => b.has(item));
}

const requiredDocs = [
  "AGENTS.md",
  "CLAUDE.md",
  "docs/IMPLEMENTATION_PLAN.md",
  "docs/adr/README.md",
  "docs/adr/0009-repository-layout-authority.md",
  "docs/workflow/project/workflow.md",
  "docs/workflow/project/dev-dashboard.md",
  "docs/workflow/project/dashboard-state.json",
  "docs/workflow/project/module-registry.json",
  "docs/workflow/FILE_STRUCTURE.md",
  "docs/workflow/templates/feature-brief.md",
  "docs/workflow/templates/design.md",
  "docs/workflow/templates/api.md",
  "docs/workflow/templates/test.md",
  "docs/workflow/templates/dev_log.md",
  "docs/workflow/templates/bug_log.md",
  "docs/workflow/templates/spike_log.md"
];

requiredDocs.forEach(path => assert(existsSync(resolve(repoRoot, path)), `missing required workflow file: ${path}`));
assert(registry.schema_version === 1, "unsupported agent registry schema");
assert(new Set(agentNames).size === registry.agents.length, "agent names must be unique");

for (const [workflow, names] of Object.entries(registry.workflows)) {
  names.forEach(name => assert(agentNames.includes(name), `${workflow} references unknown agent ${name}`));
}
assert(registry.workflows.spike.includes("feature-plan"), "spike workflow must include feature-plan for intake and decision");

const canonicalStatuses = [
  "DRAFT", "NEEDS_REVIEW", "APPROVED", "REVISE", "READY_FOR_VERIFY", "VERIFIED",
  "READY_TO_SHIP", "SHIPPED", "DIAGNOSED", "SPIKE_READY", "EVIDENCE_READY",
  "INCONCLUSIVE", "ACCEPTED", "GATE_RESOLVED", "BLOCKED"
];

// ---- Parse the state-machine edge table: (Workflow, Current, Writer, Next) ----
const workflowText = readFileSync(resolve(repoRoot, "docs/workflow/project/workflow.md"), "utf8");
const smSection = workflowText.split("## 3. State machine")[1]?.split(/\n## /)[0];
assert(smSection, "workflow.md missing '## 3. State machine' section");

const edges = smSection
  .split("\n")
  .filter(line => line.trim().startsWith("|"))
  .slice(2) // header + separator
  .map(line => {
    const cells = line.split("|").slice(1, -1).map(cell => cell.trim());
    assert(cells.length === 4, `state machine row is not four columns: ${line.trim()}`);
    const writers = agentNames.filter(name => cells[2].includes(name));
    assert(writers.length === 1, `state machine row must name exactly one writer role: ${line.trim()}`);
    return {
      workflow: cells[0].replaceAll("`", ""),
      current: cells[1].replaceAll("`", ""),
      writer: writers[0],
      next: cells[3].match(/[A-Z][A-Z_]{2,}/g) ?? []
    };
  });
assert(edges.length > 0, "state machine table has no edges");

const workflows = new Set(edges.map(edge => edge.workflow));
["FEATURE_DEV", "BUGFIX", "SPIKE"].forEach(wf => assert(workflows.has(wf), `state machine missing workflow ${wf}`));

// Canonical status set equals the set of statuses used by the table (both directions).
const tableStatuses = new Set(edges.flatMap(edge => [edge.current, ...edge.next]));
assert(setEqual(tableStatuses, new Set(canonicalStatuses)),
  `state machine statuses != canonical set; table-only: [${[...tableStatuses].filter(s => !canonicalStatuses.includes(s))}], missing: [${canonicalStatuses.filter(s => !tableStatuses.has(s))}]`);
assert(!edges.some(edge => edge.current === "BLOCKED"), "BLOCKED must not appear as a Current row; recovery is prose");

// Required entry edges per workflow.
const hasEdge = (wf, current, writer, next) =>
  edges.some(edge => edge.workflow === wf && edge.current === current && edge.writer === writer && edge.next.includes(next));
assert(hasEdge("FEATURE_DEV", "DRAFT", "feature-plan", "NEEDS_REVIEW"), "missing entry edge (FEATURE_DEV, DRAFT, feature-plan -> NEEDS_REVIEW)");
assert(hasEdge("BUGFIX", "DRAFT", "bug-diagnose", "DIAGNOSED"), "missing entry edge (BUGFIX, DRAFT, bug-diagnose -> DIAGNOSED)");
assert(hasEdge("SPIKE", "DRAFT", "feature-plan", "SPIKE_READY"), "missing entry edge (SPIKE, DRAFT, feature-plan -> SPIKE_READY)");

// Spike REVISE must loop back into the spike pipeline, never the feature pipeline.
const spikeRevise = edges.filter(edge => edge.workflow === "SPIKE" && edge.current === "REVISE");
assert(spikeRevise.length > 0, "missing (SPIKE, REVISE) edge");
assert(spikeRevise.every(edge => edge.next.includes("SPIKE_READY") && !edge.next.includes("NEEDS_REVIEW")),
  "(SPIKE, REVISE) must re-enter SPIKE_READY and never NEEDS_REVIEW");

// Per-workflow legal resting statuses: Current statuses plus terminal Next
// statuses (a Next that is no row's Current within the same workflow).
const legalStatuses = {};
const currentsByWorkflow = {};
for (const wf of workflows) {
  const wfEdges = edges.filter(edge => edge.workflow === wf);
  const currents = new Set(wfEdges.map(edge => edge.current));
  const terminals = new Set(wfEdges.flatMap(edge => edge.next).filter(status => !currents.has(status)));
  currentsByWorkflow[wf] = currents;
  legalStatuses[wf] = new Set([...currents, ...terminals]);
}

// Gate-scoped writers: with an open Security Gate, only security-review may
// write from these states; without one, only the ungated writer may.
const gatedStates = {
  "SPIKE|EVIDENCE_READY": { gated: "security-review", ungated: "feature-plan" },
  "FEATURE_DEV|READY_TO_SHIP": { gated: "security-review", ungated: "ship" }
};

// ---- Agents: mirrors, handoff/registry/state-machine consistency ----
for (const agent of registry.agents) {
  const role = `.agents/roles/${agent.name}.md`;
  const codex = `.codex/agents/${agent.name}.toml`;
  const claude = `.claude/agents/${agent.name}.md`;
  [role, codex, claude].forEach(path => assert(existsSync(resolve(repoRoot, path)), `missing agent file: ${path}`));
  const roleText = readFileSync(resolve(repoRoot, role), "utf8");
  assert(roleText.includes("## Handoff"), `${role} missing Handoff contract`);
  assert(readFileSync(resolve(repoRoot, codex), "utf8") === codexAgent(agent), `${codex} drifted from generator output; run npm run workflow:generate`);
  assert(readFileSync(resolve(repoRoot, claude), "utf8") === claudeAgent(agent), `${claude} drifted from generator output; run npm run workflow:generate`);

  const statusLine = roleText.split("\n").find(line => line.startsWith("**Status**:") || line.startsWith("**Verdict**:"));
  assert(statusLine, `${role} handoff missing a Status or Verdict line`);
  const emitted = new Set(statusLine.match(/[A-Z][A-Z_]{2,}/g) ?? []);
  assert(emitted.size > 0, `${role} handoff Status/Verdict line declares no statuses`);
  const registryTokens = new Set(agent.output.match(/[A-Z][A-Z_]{2,}/g) ?? []);
  assert(setEqual(emitted, registryTokens),
    `${agent.name}: registry output statuses [${[...registryTokens]}] != role handoff statuses [${[...emitted]}]`);
  for (const status of emitted) {
    assert(canonicalStatuses.includes(status), `${role} emits non-canonical status ${status}`);
    assert(edges.some(edge => edge.writer === agent.name && edge.next.includes(status)),
      `no state-machine edge lets ${agent.name} produce ${status}`);
  }
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

// ---- Templates carry every Status Panel field the dashboard reads ----
const statusPanelFields = ["Workflow", "Target", "Title", "Owner Module", "Current Phase", "Status", "Executor", "Updated", "Suggested Next"];
const templateWorkflows = {
  "docs/workflow/templates/dev_log.md": "FEATURE_DEV",
  "docs/workflow/templates/bug_log.md": "BUGFIX",
  "docs/workflow/templates/spike_log.md": "SPIKE"
};
const panelField = (text, name) => text.match(new RegExp("\\|\\s*" + name + "\\s*\\|\\s*`?([^|`]+)`?\\s*\\|", "i"))?.[1]?.trim();
for (const [template, wf] of Object.entries(templateWorkflows)) {
  const text = readFileSync(resolve(repoRoot, template), "utf8");
  statusPanelFields.forEach(field => {
    assert(new RegExp("\\|\\s*" + field + "\\s*\\|").test(text), `${template} missing Status Panel field ${field}`);
  });
  assert(panelField(text, "Workflow") === wf, `${template} Workflow field must be ${wf}`);
  assert(new RegExp("\\|\\s*Security Gate\\s*\\|").test(text), `${template} missing Security Gate field`);
}

// ---- Feature/bug/spike state records ----
const gateKeywords = /credential|auth|key|token|secret|keychain|e2ee|remote[ -]control|trust boundar/i;
const featuresRoot = resolve(repoRoot, "docs/workflow/features");
for (const entry of readdirSync(featuresRoot, { withFileTypes: true }).filter(item => item.isDirectory())) {
  const slug = entry.name;
  const logPath = resolve(featuresRoot, slug, "dev_log.md");
  assert(existsSync(logPath), `docs/workflow/features/${slug} is missing dev_log.md`);
  const text = readFileSync(logPath, "utf8");
  statusPanelFields.forEach(field => {
    const value = panelField(text, field);
    assert(value && value !== "unknown", `${slug}/dev_log.md Status Panel field ${field} is missing or empty`);
  });
  assert(/## Work Log/.test(text), `${slug}/dev_log.md missing Work Log section`);

  const workflow = panelField(text, "Workflow");
  const status = panelField(text, "Status");
  const owner = panelField(text, "Owner Module");
  assert(workflows.has(workflow), `${slug}/dev_log.md has unknown workflow ${workflow}`);
  assert(legalStatuses[workflow].has(status), `${slug}/dev_log.md status ${status} is not a legal ${workflow} state`);
  assert(moduleKeys.has(owner), `${slug}/dev_log.md Owner Module ${owner} is not a module-registry key`);

  // Security-gate rules: keyword heuristic on Title+Hypothesis for spikes;
  // any security-owned unit must carry an open/resolved gate.
  const gate = panelField(text, "Security Gate") ?? "";
  if (workflow === "SPIKE") {
    assert(gate, `${slug}/dev_log.md (spike) missing Security Gate field`);
    const surface = `${panelField(text, "Title") ?? ""} ${panelField(text, "Hypothesis") ?? ""}`;
    if (gateKeywords.test(surface)) {
      assert(!/^none/i.test(gate), `${slug}/dev_log.md mentions credentials/keys/auth/remote control/trust boundaries but Security Gate is none (SOP_SPIKE rule 5)`);
    }
  }
  if (owner === "security") {
    assert(gate && !/^none/i.test(gate), `${slug}/dev_log.md is owned by security but Security Gate is none`);
  }

  // Suggested Next must name only legal writers for the current state, and
  // gate-scoped states must route to the gate-selected writer.
  if (currentsByWorkflow[workflow].has(status)) {
    const suggested = panelField(text, "Suggested Next") ?? "";
    const named = agentNames.filter(name => suggested.includes(name));
    assert(named.length > 0, `${slug}/dev_log.md Suggested Next names no workflow agent at non-terminal (${workflow}, ${status})`);
    const writers = new Set(edges.filter(edge => edge.workflow === workflow && edge.current === status).map(edge => edge.writer));
    named.forEach(name => assert(writers.has(name),
      `${slug}/dev_log.md Suggested Next names ${name}, which is not a legal writer from (${workflow}, ${status})`));
    const gateRule = gatedStates[`${workflow}|${status}`];
    if (gateRule) {
      const gateOpen = /^open/i.test(gate);
      assert(gateOpen || /^(none|resolved)/i.test(gate),
        `${slug}/dev_log.md Security Gate must start with open, none, or resolved at (${workflow}, ${status})`);
      if (gateOpen) {
        assert(named.includes(gateRule.gated) && !named.includes(gateRule.ungated),
          `${slug}/dev_log.md has an open Security Gate at (${workflow}, ${status}); Suggested Next must be ${gateRule.gated}, not ${gateRule.ungated}`);
      } else {
        assert(named.includes(gateRule.ungated) && !named.includes(gateRule.gated),
          `${slug}/dev_log.md has no open Security Gate at (${workflow}, ${status}); Suggested Next must be ${gateRule.ungated}`);
      }
    }
  }
}

console.log(`verified workflow: agents=${registry.agents.length}, skills=${registry.skills.length}, docs=${requiredDocs.length}, edges=${edges.length}, statuses=${canonicalStatuses.length}`);
