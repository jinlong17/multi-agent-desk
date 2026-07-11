# SOP: Bugfix

1. Classify the owning module and create/reuse a bug feature-state directory.
2. Run `bug-diagnose`; require a stable reproduction and root-cause evidence.
3. Run `bug-fix`; keep the patch minimal and add a regression test.
4. Run independent `bug-verify`; do not let the verifier repair the patch.
5. Return `BLOCKED` failures to `bug-fix` with reproduction evidence.
6. Run `ship` only after `READY_TO_SHIP` and explicit authorization.
7. Preserve the original failure and its resolution in Work Log.
