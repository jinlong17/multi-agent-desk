export interface RevokeSessionTarget {
  readonly id: string;
  readonly revision: number;
}

export interface SessionRevokeDependencies<TTarget extends RevokeSessionTarget, TResult> {
  confirm(target: TTarget, refreshed: boolean): boolean;
  createIdempotencyKey(): string;
  refresh(): Promise<readonly TTarget[]>;
  revoke(target: TTarget, idempotencyKey: string): Promise<TResult>;
}

export type SessionRevokeOutcome<TTarget, TResult> =
  | { readonly status: "cancelled"; readonly refreshed: boolean }
  | { readonly status: "missing_after_refresh" }
  | { readonly status: "revoked"; readonly target: TTarget; readonly result: TResult; readonly refreshed: boolean };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// isMatchingSessionRevisionConflict rejects malformed or cross-target details.
// The server-provided current revision must advance the exact attempted target;
// anything else is surfaced as an error rather than treated as retry authority.
export function isMatchingSessionRevisionConflict(error: unknown, target: RevokeSessionTarget): boolean {
  if (!isRecord(error) || error.code !== "session_revision_conflict" || !isRecord(error.details)) return false;
  const details = error.details;
  const keys = Object.keys(details).sort();
  if (keys.length !== 3 || keys[0] !== "currentRevision" || keys[1] !== "expectedRevision" || keys[2] !== "sessionId") return false;
  return details.sessionId === target.id &&
    details.expectedRevision === target.revision &&
    Number.isSafeInteger(details.currentRevision) &&
    (details.currentRevision as number) > target.revision;
}

// revokeSessionWithExplicitReconfirmation performs at most two mutations. A
// revision conflict always refreshes authority and requires a second explicit
// confirmation plus a fresh idempotency key; a second conflict is returned to
// the caller rather than becoming a retry loop.
export async function revokeSessionWithExplicitReconfirmation<TTarget extends RevokeSessionTarget, TResult>(
  initial: TTarget,
  dependencies: SessionRevokeDependencies<TTarget, TResult>,
): Promise<SessionRevokeOutcome<TTarget, TResult>> {
  if (!dependencies.confirm(initial, false)) return { status: "cancelled", refreshed: false };

  const firstIdempotencyKey = dependencies.createIdempotencyKey();
  try {
    const result = await dependencies.revoke(initial, firstIdempotencyKey);
    return { status: "revoked", target: initial, result, refreshed: false };
  } catch (error) {
    if (!isMatchingSessionRevisionConflict(error, initial)) throw error;
  }

  const refreshed = await dependencies.refresh();
  const current = refreshed.find((candidate) => candidate.id === initial.id);
  if (!current) return { status: "missing_after_refresh" };
  if (!dependencies.confirm(current, true)) return { status: "cancelled", refreshed: true };

  const secondIdempotencyKey = dependencies.createIdempotencyKey();
  if (secondIdempotencyKey === firstIdempotencyKey) {
    throw new Error("session revoke retry requires a distinct idempotency key");
  }
  const result = await dependencies.revoke(current, secondIdempotencyKey);
  return { status: "revoked", target: current, result, refreshed: true };
}
