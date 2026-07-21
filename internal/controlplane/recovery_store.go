package controlplane

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func (s *Store) RecoveryCandidates(ctx context.Context) ([]RecoveryCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rc.id,rc.salt,rc.code_hash,rc.status FROM recovery_codes rc JOIN recovery_batches rb ON rb.id=rc.batch_id WHERE rb.status IN ('active','exhausted') ORDER BY rc.ordinal`)
	if err != nil {
		return nil, fmt.Errorf("read recovery candidates: %w", err)
	}
	defer rows.Close()
	var result []RecoveryCandidate
	for rows.Next() {
		var item RecoveryCandidate
		if err := rows.Scan(&item.ID, &item.Salt, &item.Hash, &item.Status); err != nil {
			return nil, fmt.Errorf("read recovery candidate: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ConsumeRecoveryCode(ctx context.Context, codeID string, session BrowserSessionCreate, now time.Time) (BrowserSession, error) {
	if _, err := transport.ParseUUIDv7(codeID); err != nil {
		return BrowserSession{}, ErrRecoveryInvalidOrRateLimited
	}
	if err := validateBrowserSessionCreate(session, now); err != nil || session.AuthenticationMethod != "recovery" {
		return BrowserSession{}, fmt.Errorf("recovery session is invalid")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return BrowserSession{}, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return BrowserSession{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var userID, batchID, status string
	if err := conn.QueryRowContext(ctx, "SELECT user_id,batch_id,status FROM recovery_codes WHERE id=?", codeID).Scan(&userID, &batchID, &status); err != nil || status != "active" || userID != session.UserID {
		return BrowserSession{}, ErrRecoveryConsumed
	}
	result, err := conn.ExecContext(ctx, `UPDATE recovery_codes SET status='consumed',consumed_at=? WHERE id=? AND status='active'`, formatServerTime(now), codeID)
	if err != nil {
		return BrowserSession{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return BrowserSession{}, ErrRecoveryConsumed
	}
	var remaining int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM recovery_codes WHERE batch_id=? AND status='active'", batchID).Scan(&remaining); err != nil {
		return BrowserSession{}, err
	}
	if remaining == 0 {
		if _, err := conn.ExecContext(ctx, `UPDATE recovery_batches SET status='exhausted' WHERE id=? AND status='active'`, batchID); err != nil {
			return BrowserSession{}, err
		}
	}
	if err := insertBrowserSession(ctx, conn, session, now); err != nil {
		return BrowserSession{}, err
	}
	if err := insertAuthAudit(ctx, conn, "recovery", "recovery.verify", "allowed", "recovery_consumed", codeID, userID, now); err != nil {
		return BrowserSession{}, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return BrowserSession{}, err
	}
	committed = true
	digest := sha256.Sum256(session.RawToken)
	return browserSessionByToken(ctx, conn, digest[:], now)
}

type RecoveryRotationInput struct {
	UserID      string
	ScopeDigest [sha256.Size]byte
	BatchID     string
	Codes       []RecoveryCodeHash
	At          time.Time
}

func (s *Store) RotateRecoveryCodes(ctx context.Context, input RecoveryRotationInput) error {
	if _, err := transport.ParseUUIDv7(input.UserID); err != nil {
		return fmt.Errorf("recovery rotation user is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.BatchID); err != nil || len(input.Codes) != recoveryCodeCount || input.At.IsZero() {
		return fmt.Errorf("recovery rotation is invalid")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var marker int
	err = conn.QueryRowContext(ctx, "SELECT 1 FROM one_time_operations WHERE scope_digest=?", input.ScopeDigest[:]).Scan(&marker)
	if err == nil {
		return ErrOneTimeResultUnavailable
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if _, err := conn.ExecContext(ctx, `UPDATE recovery_codes SET status='invalidated' WHERE user_id=? AND status='active'`, input.UserID); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `UPDATE recovery_batches SET status='rotated',invalidated_at=? WHERE user_id=? AND status IN ('active','exhausted')`, formatServerTime(input.At), input.UserID); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO recovery_batches(id,user_id,status,created_at,invalidated_at) VALUES(?,?,'active',?,NULL)`, input.BatchID, input.UserID, formatServerTime(input.At)); err != nil {
		return err
	}
	seen := make(map[int]struct{}, recoveryCodeCount)
	for _, code := range input.Codes {
		if _, err := transport.ParseUUIDv7(code.ID); err != nil || code.Ordinal < 1 || code.Ordinal > recoveryCodeCount || len(code.Salt) != recoverySaltSize || len(code.Hash) != recoveryHashSize {
			return fmt.Errorf("recovery rotation hash is invalid")
		}
		if _, duplicate := seen[code.Ordinal]; duplicate {
			return fmt.Errorf("recovery rotation ordinal is duplicated")
		}
		seen[code.Ordinal] = struct{}{}
		if _, err := conn.ExecContext(ctx, `INSERT INTO recovery_codes(id,batch_id,user_id,ordinal,salt,code_hash,status,created_at,consumed_at) VALUES(?,?,?,?,?,?,'active',?,NULL)`, code.ID, input.BatchID, input.UserID, code.Ordinal, code.Salt, code.Hash, formatServerTime(input.At)); err != nil {
			return err
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO one_time_operations(scope_digest,user_id,operation,created_at) VALUES(?,?,'recovery_rotate',?)`, input.ScopeDigest[:], input.UserID, formatServerTime(input.At)); err != nil {
		return err
	}
	if err := insertAuthAudit(ctx, conn, "browser", "recovery.rotate", "allowed", "recovery_batch_replaced", input.BatchID, input.UserID, input.At); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
