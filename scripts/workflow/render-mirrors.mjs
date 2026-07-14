function quote(value) {
  return JSON.stringify(String(value));
}

export function codexAgent(agent) {
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

export function claudeAgent(agent) {
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
