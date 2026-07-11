# Contract: Phase 0 monorepo scaffold

This feature defines repository/build contracts only.

## Required physical paths

The tracked tree must contain every path from Plan v0.2 §17:
`cmd/{multidesk,multidesk-server}`, all listed `internal/*` packages including
`providers/{codex,claude}`, `apps/{web,desktop/src-tauri}`,
`packages/{ui,protocol,config}`, `api/{openapi,events}`,
`migrations/{device,server}`, and `deploy/docker` plus
`deploy/docker-compose.yml`. The §17 document set additionally requires
`docs/{ARCHITECTURE,DATA_MODEL,PROVIDER_ADAPTER,ROADMAP}.md` alongside the
already present implementation plan, threat model, compatibility, research,
ADR, and review documents.

Forbidden paths include `apps/daemon`, `apps/cli`, `apps/server`, and any
`packages/provider-*`. A deterministic structure check must fail if a required
path is absent or a forbidden path appears.

## Build contracts

- `go build ./cmd/...` and `go test ./...` cover both empty Go commands and
  compile-safe internal placeholders.
- pnpm workspace scripts build/check Web and shared packages from the root.
- `pnpm --filter @multi-agent-desk/desktop tauri build --no-bundle` proves the
  Tauri CLI/config/frontend/Rust integration without installer bundling.
- `npm run scaffold:verify` composes structure, format, check, build, and test
  commands and fails when any required tool/command is missing.
- `just check` and `just build` are wrappers, not separate logic.

Exact root script graph:

| Script | Command responsibility |
|---|---|
| `scaffold:structure` | `node scripts/scaffold/verify-layout.mjs` |
| `go:fmt-check` | fail when `gofmt -l` returns any Go file |
| `go:build` / `go:test` | `go build ./cmd/...` / `go test ./...` |
| `web:check` / `web:build` | recursive pnpm check/build for Web and shared packages, excluding Desktop Tauri wrapper |
| `desktop:fmt-check` | `cargo fmt --manifest-path ... --check` |
| `desktop:check` | `cargo check --locked --manifest-path ...` |
| `desktop:build` | pnpm-filtered `tauri build --no-bundle` |
| `scaffold:check` | structure + Go fmt/test + Web check + Cargo fmt/check |
| `scaffold:build` | Go build + Web build + Tauri build |
| `scaffold:verify` | `scaffold:check` then `scaffold:build` |

## Toolchain contract

Version files and manifest engines agree on `.go-version=1.26.5`, `go 1.26`,
`.node-version=24`, Node engines `>=24 <25`, `packageManager=pnpm@10.23.0`,
and `rust-toolchain.toml channel=1.91.1`. Tauri stays on major 2; lockfiles
supply exact dependency versions.

## Exact manifest set

- root: `go.mod`, `package.json`, `pnpm-workspace.yaml`, `.go-version`,
  `.node-version`, `rust-toolchain.toml`, `justfile`;
- Go: two `cmd/*/main.go` files and compile-safe `doc.go` placeholders in each
  owned internal directory;
- TypeScript: `apps/web/package.json`, `tsconfig.json`, `index.html`, and
  `src/main.ts`; each `packages/{ui,protocol,config}` has package.json,
  tsconfig.json, and src/index.ts;
- Desktop: `apps/desktop/package.json` and
  `src-tauri/{Cargo.toml,Cargo.lock,build.rs,tauri.conf.json,capabilities/default.json,src/main.rs}`;
- validation/deploy/docs: `scripts/scaffold/verify-layout.mjs`, ownership
  READMEs for API/migrations/deploy placeholders, empty compose service map,
  and the four truthful §17 document placeholders.

## Placeholder contract

Command output may identify an empty scaffold and exit successfully. It must
not expose a durable public API, open a socket, read credentials, initialize a
Vault/database, invoke Codex/Claude, or freeze Provider/crypto/Windows details.
