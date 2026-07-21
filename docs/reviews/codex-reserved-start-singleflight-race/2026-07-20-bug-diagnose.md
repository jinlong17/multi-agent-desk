# Bug diagnosis: Codex reserved start singleflight race

## Verdict

`DIAGNOSED` — PR #28 exposed a pre-existing production idempotency race in
Codex reserved Session start. It is not introduced by the docs-only branch and
must not be waived by rerunning the failed macOS job.

## Classification

```text
Owner: provider
Confidence: high
Why: the defect is in the Codex RuntimeManager exact-once Provider-start contract
Impacts: core (durable Session lifecycle), project-system (required macOS CI)
Branch: codex/provider/codex-reserved-start-singleflight-race
Workflow: bugfix
Gates: Provider compatibility remains resolved; Security Gate none
Docs: docs/workflow/features/codex-reserved-start-singleflight-race/dev_log.md
```

## Environment and reproduction

- Base: `origin/main@2d3b4162b72bff26d203c55bb63782b725464f87`.
- Triggering branch: PR #28 head
  `aa9b52639dddaa3c1125335a298febf1b2beeb26`; its changed files are Claude
  compatibility/spike documents and contain no Codex runtime or storage diff.
- Remote: macOS arm64, Go `1.26.5`, CI run `29805832734`,
  [job 88556037636](https://github.com/jinlong17/multi-agent-desk/actions/runs/29805832734/job/88556037636).
- Remote symptom: the test failed in `0.05s` at `runtime_test.go:399` with
  `conflict: provider session identity changed`. That timing excludes the
  independent five-second fixture-deadline defect already fixed on main.
- Local exact-head reproduction on Darwin arm64, Go `1.26.5`:

  ```text
  GOMAXPROCS=1 go test -count=100 \
    -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' \
    ./internal/providers/codex
  ```

  Result: `59/100` executions failed with the same line and error. Ordinary
  target count/race runs and the full suite can pass, which shows the schedule
  sensitivity and explains why earlier checks did not detect the race.

## Causal trace

```text
caller A reads durable Session = starting
caller B reads durable Session = starting, then is descheduled
caller A installs m.starting gate
  -> starts Provider thread
  -> writes provider_session_id
  -> transitions Session to running
  -> deletes/closes gate
caller B resumes and sees no gate
  -> becomes a second leader using stale starting snapshot
  -> starts a second Provider thread
  -> provider_session_id CAS changes zero rows
  -> returns "provider session identity changed"
  -> releaseBinding may fail Session and finalize runtime
```

The causal window is `internal/providers/codex/runtime.go:245-268`: the Store
read and `starting` decision precede the singleflight lock. The exact error
comes from `internal/storage/repository.go:828-837`, whose CAS correctly
requires one `starting` Session with no Provider identity. The CAS is exposing
the duplicate leader; weakening it would conceal the bug.

The failure cleanup is materially unsafe: `runtime.go:415-417` calls
`releaseBinding`, and `runtime.go:618-650` can remove the successful binding,
kill/finalize the credential runtime, and transition the previously running
Session to `failed`. This is therefore not merely a flaky assertion.

## Scope and history

The race entered with the reserved-start singleflight implementation in
`2bbd444`; the Session read and gate ordering remain unchanged in PR #28 and
`origin/main@2d3b416`. Main commits `a4c3f21` and `621a4a2` fixed and verified
the separate test-fixture context-budget issue. They correctly left production
runtime code unchanged, so they neither caused nor repaired this recurrence.

## Minimum repair

1. In `internal/providers/codex/runtime.go`, validate the reserved request and
   acquire/wait on `m.starting[reserved.SessionID]` before the durable Session
   read.
2. After becoming gate owner, read and validate a fresh Session. If it is
   already `running` or terminal, return the durable state without Provider
   discovery, materialization, spawn, `thread/start`, or cleanup.
3. Preserve wait cancellation and close/delete the per-Session gate exactly
   once on every elected-owner exit.
4. Add a focused `runtime_test.go` regression that exercises the late-contender
   schedule and proves two successful identical results, one spawn, one
   Provider thread, a running durable Session, and an intact runtime/binding.
5. Re-run the single-P stress reproduction, race tests, the full Go/vet suite,
   and native macOS plus Windows protected checks before verification.

Do not modify the Store CAS, SQLite settings, Provider capability matrix,
platform support, the earlier fixture deadline, Named Pipe code, or unrelated
Device tests.
