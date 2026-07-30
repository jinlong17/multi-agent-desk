import assert from "node:assert/strict";
import test from "node:test";

import { isMatchingSessionRevisionConflict, revokeSessionWithExplicitReconfirmation } from "../src/session-revoke.ts";

const initial = Object.freeze({ id: "session-a", revision: 1 });

function fixture(overrides = {}) {
  const calls = { confirmations: [], keys: [], refreshes: 0, revokes: [] };
  let key = 0;
  const dependencies = {
    confirm(target, refreshed) {
      calls.confirmations.push({ target, refreshed });
      return true;
    },
    createIdempotencyKey() {
      const value = `idempotency-key-${++key}`;
      calls.keys.push(value);
      return value;
    },
    async refresh() {
      calls.refreshes += 1;
      return [{ id: initial.id, revision: 2 }];
    },
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      return { sessionId: target.id, revision: target.revision + 1 };
    },
    ...overrides,
  };
  return { calls, dependencies };
}

test("an initial cancellation performs no request", async () => {
  const { calls, dependencies } = fixture({ confirm: () => false });
  const outcome = await revokeSessionWithExplicitReconfirmation(initial, dependencies);
  assert.deepEqual(outcome, { status: "cancelled", refreshed: false });
  assert.deepEqual(calls.keys, []);
  assert.deepEqual(calls.revokes, []);
  assert.equal(calls.refreshes, 0);
});

test("a current revision uses one explicit confirmation and one fresh key", async () => {
  const { calls, dependencies } = fixture();
  const outcome = await revokeSessionWithExplicitReconfirmation(initial, dependencies);
  assert.equal(outcome.status, "revoked");
  assert.equal(outcome.refreshed, false);
  assert.deepEqual(calls.confirmations, [{ target: initial, refreshed: false }]);
  assert.deepEqual(calls.keys, ["idempotency-key-1"]);
  assert.deepEqual(calls.revokes, [{ target: initial, idempotencyKey: "idempotency-key-1" }]);
  assert.equal(calls.refreshes, 0);
});

test("a revision conflict refetches, reconfirms the new revision, and uses a new key", async () => {
  const { calls, dependencies } = fixture({
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      if (calls.revokes.length === 1) throw {
        code: "session_revision_conflict",
        details: { sessionId: target.id, expectedRevision: target.revision, currentRevision: 2 },
      };
      return { sessionId: target.id, revision: target.revision + 1 };
    },
  });
  const outcome = await revokeSessionWithExplicitReconfirmation(initial, dependencies);
  assert.equal(outcome.status, "revoked");
  assert.equal(outcome.refreshed, true);
  assert.equal(outcome.target.revision, 2);
  assert.equal(calls.refreshes, 1);
  assert.deepEqual(calls.confirmations.map(({ target, refreshed }) => [target.revision, refreshed]), [[1, false], [2, true]]);
  assert.deepEqual(calls.keys, ["idempotency-key-1", "idempotency-key-2"]);
  assert.notEqual(calls.revokes[0].idempotencyKey, calls.revokes[1].idempotencyKey);
  assert.deepEqual(calls.revokes.map(({ target }) => target.revision), [1, 2]);
});

test("declining the refreshed revision never sends a second mutation", async () => {
  const { calls, dependencies } = fixture({
    confirm(_target, refreshed) {
      calls.confirmations.push({ refreshed });
      return !refreshed;
    },
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      throw {
        code: "session_revision_conflict",
        details: { sessionId: target.id, expectedRevision: target.revision, currentRevision: 2 },
      };
    },
  });
  const outcome = await revokeSessionWithExplicitReconfirmation(initial, dependencies);
  assert.deepEqual(outcome, { status: "cancelled", refreshed: true });
  assert.equal(calls.refreshes, 1);
  assert.equal(calls.revokes.length, 1);
  assert.deepEqual(calls.keys, ["idempotency-key-1"]);
});

test("a target missing from the refreshed authority is not retried", async () => {
  const { calls, dependencies } = fixture({
    async refresh() {
      calls.refreshes += 1;
      return [];
    },
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      throw {
        code: "session_revision_conflict",
        details: { sessionId: target.id, expectedRevision: target.revision, currentRevision: 2 },
      };
    },
  });
  const outcome = await revokeSessionWithExplicitReconfirmation(initial, dependencies);
  assert.deepEqual(outcome, { status: "missing_after_refresh" });
  assert.equal(calls.confirmations.length, 1);
  assert.equal(calls.revokes.length, 1);
});

test("a second conflict is surfaced without a third confirmation or request", async () => {
  const { calls, dependencies } = fixture({
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      throw {
        code: "session_revision_conflict",
        details: { sessionId: target.id, expectedRevision: target.revision, currentRevision: target.revision + 1 },
      };
    },
  });
  await assert.rejects(() => revokeSessionWithExplicitReconfirmation(initial, dependencies), (error) => error.code === "session_revision_conflict");
  assert.equal(calls.refreshes, 1);
  assert.equal(calls.confirmations.length, 2);
  assert.equal(calls.revokes.length, 2);
  assert.equal(calls.keys.length, 2);
});

test("a duplicate retry idempotency key fails closed before the second DELETE", async () => {
  const { calls, dependencies } = fixture({
    createIdempotencyKey() {
      calls.keys.push("duplicate-key");
      return "duplicate-key";
    },
    async revoke(target, idempotencyKey) {
      calls.revokes.push({ target, idempotencyKey });
      throw {
        code: "session_revision_conflict",
        details: { sessionId: target.id, expectedRevision: target.revision, currentRevision: target.revision + 1 },
      };
    },
  });
  await assert.rejects(
    () => revokeSessionWithExplicitReconfirmation(initial, dependencies),
    /requires a distinct idempotency key/u,
  );
  assert.equal(calls.refreshes, 1);
  assert.equal(calls.confirmations.length, 2);
  assert.deepEqual(calls.keys, ["duplicate-key", "duplicate-key"]);
  assert.equal(calls.revokes.length, 1);
});

test("only exact advancing conflict details authorize a refresh", () => {
  const valid = { code: "session_revision_conflict", details: { sessionId: initial.id, expectedRevision: 1, currentRevision: 2 } };
  assert.equal(isMatchingSessionRevisionConflict(valid, initial), true);
  for (const malformed of [
    { code: "session_revision_conflict" },
    { code: "session_revision_conflict", details: null },
    { code: "session_revision_conflict", details: { sessionId: "session-b", expectedRevision: 1, currentRevision: 2 } },
    { code: "session_revision_conflict", details: { sessionId: initial.id, expectedRevision: 2, currentRevision: 3 } },
    { code: "session_revision_conflict", details: { sessionId: initial.id, expectedRevision: 1, currentRevision: 1 } },
    { code: "session_revision_conflict", details: { sessionId: initial.id, expectedRevision: 1, currentRevision: 1.5 } },
    { code: "session_revision_conflict", details: { sessionId: initial.id, expectedRevision: 1, currentRevision: 2, extra: true } },
    { code: "conflict", details: valid.details },
  ]) {
    assert.equal(isMatchingSessionRevisionConflict(malformed, initial), false, JSON.stringify(malformed));
  }
});
