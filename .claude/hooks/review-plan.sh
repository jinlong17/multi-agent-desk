#!/usr/bin/env bash
set -euo pipefail

project_dir="${CLAUDE_PROJECT_DIR:-$(pwd)}"
plan_path="${project_dir}/docs/IMPLEMENTATION_PLAN.md"

if [[ ! -f "${plan_path}" ]]; then
  echo "Review failed: ${plan_path} does not exist."
  exit 2
fi

prompt="You are the independent architecture reviewer for MultiAgentDesk. Read ${plan_path} completely. Review it as a production-minded principal engineer and security architect. Do not edit files. Evaluate product scope, trust boundaries, credential transfer, cryptography, local vault behavior, Codex app-server integration, Claude Code CLI and PTY integration, cross-platform feasibility, API and data model completeness, license boundaries, rollout order, testing, failure modes, and whether the plan is decision-complete. Identify contradictions, unsafe assumptions, over-scoped v0.1 items, missing acceptance criteria, and claims that need qualification. Return Markdown in Chinese with exactly these sections: Verdict, Must Fix, Should Fix, Keep, Proposed Edits. Rank each finding P0, P1, or P2 and cite the plan section number. Be concrete enough that another engineer can revise the document without making new architectural decisions."

exec claude \
  --setting-sources user \
  --model fable \
  --effort high \
  --permission-mode dontAsk \
  --tools "Read,Grep,Glob" \
  --add-dir "${project_dir}" \
  --no-session-persistence \
  --output-format text \
  --print "${prompt}"
