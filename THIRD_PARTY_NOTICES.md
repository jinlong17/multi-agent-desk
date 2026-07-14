# Third-party notices

MultiAgentDesk is currently in Phase 0. The empty Web/Desktop scaffold resolves
the following direct build/runtime dependencies; exact transitive versions are
recorded in `pnpm-lock.yaml` and `apps/desktop/src-tauri/Cargo.lock`.

| Component | Locked version | Declared license | Purpose |
|---|---:|---|---|
| TypeScript | 5.9.3 | Apache-2.0 | Type checking and declaration output |
| Vite | 7.3.6 | MIT | Empty Web frontend build |
| `@tauri-apps/cli` | 2.11.4 | Apache-2.0 OR MIT | Tauri build CLI |
| `tauri` | 2.11.5 | Apache-2.0 OR MIT | Empty Desktop shell runtime |
| `tauri-build` | 2.6.3 | Apache-2.0 OR MIT | Tauri build script support |

The Phase 0 scaffold inventory found no dependency with an unknown declared
license. pnpm transitive declarations are MIT, Apache-2.0, Apache-2.0 OR MIT,
ISC, or BSD-3-Clause. Cargo metadata contains no missing license field; its
expressions include permissive/compatible alternatives plus MPL-2.0 components.
This is recorded evidence, not the merge gate: automated allowlist enforcement
and the incompatible-GPL negative fixture belong to `phase0-ci-governance`.

When code or other material is reused, add an entry before merge containing:

- component and version or commit;
- upstream project and source URL;
- license and compatibility conclusion;
- files or concepts used; and
- required copyright or attribution text.

Research is not code reuse, but relevant architectural research must be logged
in `docs/RESEARCH_LOG.md` once that Phase 0 artifact is created. Projects under
AGPL or unclear/restrictive terms may be studied only within the constraints of
the [implementation plan](docs/IMPLEMENTATION_PLAN.md); their source, tests,
protocol constants, and identifiable implementation details must not be copied
into the Apache-2.0 core.
