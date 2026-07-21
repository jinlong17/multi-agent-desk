package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

type PasskeyRegistrationCommit struct {
	CeremonyID              string
	UserID                  string
	CurrentSessionID        string
	ExpectedSessionRevision int64
	Passkey                 StoredPasskey
	ReplacementSession      BrowserSessionCreate
	RevokeAllOtherSessions  bool
	At                      time.Time
}

func (s *Store) CommitPasskeyRegistration(ctx context.Context, input PasskeyRegistrationCommit) error {
	if input.UserID == "" || input.CurrentSessionID == "" || input.ExpectedSessionRevision < 1 || input.Passkey.ID == "" || input.Passkey.UserID != input.UserID || input.At.IsZero() {
		return fmt.Errorf("passkey registration commit is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return fmt.Errorf("passkey registration ceremony is invalid")
	}
	if err := validateBrowserSessionCreate(input.ReplacementSession, input.At); err != nil {
		return err
	}
	credentialJSON, err := json.Marshal(input.Passkey.Credential)
	if err != nil || len(credentialJSON) > 1<<20 {
		return fmt.Errorf("registered passkey is invalid")
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
	if err := consumeWebAuthnCeremony(ctx, conn, input.CeremonyID, ceremonyPasskeyRegistration); err != nil {
		return err
	}
	var sessionUser string
	var sessionRevision int64
	if err := conn.QueryRowContext(ctx, `SELECT user_id,revision FROM browser_sessions WHERE id=? AND revoked_at IS NULL AND expires_at>? AND idle_expires_at>?`, input.CurrentSessionID, formatServerTime(input.At), formatServerTime(input.At)).Scan(&sessionUser, &sessionRevision); err != nil || sessionUser != input.UserID || sessionRevision != input.ExpectedSessionRevision {
		return ErrRevisionChanged
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO passkeys(id,user_id,credential_id,credential_json,name,transports_json,sign_count,credential_revision,active,created_at,last_used_at,updated_at) VALUES(?,?,?,?,?,?,?,1,1,?,NULL,?)`, input.Passkey.ID, input.UserID, input.Passkey.Credential.ID, credentialJSON, input.Passkey.Name, string(input.Passkey.TransportsJSON), input.Passkey.SignCount, formatServerTime(input.At), formatServerTime(input.At)); err != nil {
		return fmt.Errorf("create registered passkey: %w", err)
	}
	if input.RevokeAllOtherSessions {
		if _, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE user_id=? AND revoked_at IS NULL`, formatServerTime(input.At), formatServerTime(input.At), input.UserID); err != nil {
			return err
		}
	} else {
		if _, err := conn.ExecContext(ctx, `UPDATE browser_sessions SET revoked_at=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND revoked_at IS NULL`, formatServerTime(input.At), formatServerTime(input.At), input.CurrentSessionID, input.ExpectedSessionRevision); err != nil {
			return err
		}
	}
	if err := insertBrowserSession(ctx, conn, input.ReplacementSession, input.At); err != nil {
		return err
	}
	if err := insertAuthAudit(ctx, conn, "browser", "passkey.register", "allowed", "passkey_registered", input.Passkey.ID, input.UserID, input.At); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
