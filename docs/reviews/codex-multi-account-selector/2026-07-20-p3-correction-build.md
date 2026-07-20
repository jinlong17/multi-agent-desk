# P3 correction build: selector enrollment platform gate

- Date: 2026-07-20
- Executor: `feature-build / P3 correction`
- Clearing: P1 finding in `2026-07-20-feature-verify-p3.md`
- Base commit: `4ba0090`
- Result: `READY_FOR_VERIFY`

## Correction

All official selector enrollment discovery points now use the same platform
gate as preview and reserved runtime startup.

- `auth.begin` rejects before enrollment ID persistence,
  CredentialInstance placeholder creation, staging-root/Home creation, or
  official login launch.
- `auth.complete` repeats the gate after claiming the enrollment. Platform
  drift transitions it to `failed`, removes the unknown placeholder
  CredentialInstance, and deletes staging before validation.
- `auth.confirm` repeats the gate after attestation and before validator/Vault
  seal. Drift performs the same terminal cleanup and cannot bind the Profile.
- The production default is `codex.RequireSelectorPlatform`; a narrow injected
  gate exists only to model accepted Linux and pending-platform transitions in
  application tests.

## Direct regression evidence

The auth lifecycle test now proves:

1. pending-platform `auth.begin --profile @A` creates zero Credentials and no
   enrollment staging root;
2. platform drift before complete leaves a failed enrollment with a null
   Credential reference and no staging directory;
3. platform drift before confirm leaves no sealed Vault item/Profile binding,
   removes the placeholder Credential/staging, and returns the typed pending
   error;
4. resetting the test gate to accepted Linux preserves the complete logout and
   re-login lifecycle.

## Writer verification

- targeted application/Provider tests and vet — pass;
- `go test -count=1 ./...` and `go vet ./...` — pass;
- `go test -race -count=1 ./...` — pass;
- auth enrollment and selector platform/reservation tests with
  `-race -count=10` — pass;
- Darwin arm64, Linux amd64, Windows amd64 builds — pass; compile evidence only
  for macOS/Windows;
- workflow mirrors, Go format, local links, dashboard refresh/verify, and
  `project:verify` — pass at restored `READY_FOR_VERIFY`;
- `git diff --check` — pass before documentation/status refresh.

No login, credential, remote, push, merge, Security Gate, ship, or release
action was performed by this correction.
