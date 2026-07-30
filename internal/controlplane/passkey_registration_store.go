package controlplane

import (
	"context"
	"database/sql"
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
	credentialJSON, err := validatePasskeyRegistrationCommit(input)
	if err != nil {
		return err
	}
	_, err = withImmediateConn(ctx, s.db, "passkey registration", func(conn *sql.Conn) (struct{}, error) {
		return struct{}{}, commitPasskeyRegistrationTx(ctx, conn, input, credentialJSON)
	})
	return err
}

func validatePasskeyRegistrationCommit(input PasskeyRegistrationCommit) ([]byte, error) {
	if input.UserID == "" || input.CurrentSessionID == "" || input.ExpectedSessionRevision < 1 || input.Passkey.ID == "" || input.Passkey.UserID != input.UserID || input.At.IsZero() {
		return nil, fmt.Errorf("passkey registration commit is invalid")
	}
	if _, err := transport.ParseUUIDv7(input.CeremonyID); err != nil {
		return nil, fmt.Errorf("passkey registration ceremony is invalid")
	}
	if err := validateBrowserSessionCreate(input.ReplacementSession, input.At); err != nil {
		return nil, err
	}
	credentialJSON, err := json.Marshal(input.Passkey.Credential)
	if err != nil || len(credentialJSON) > 1<<20 {
		return nil, fmt.Errorf("registered passkey is invalid")
	}
	return credentialJSON, nil
}

func commitPasskeyRegistrationTx(ctx context.Context, conn *sql.Conn, input PasskeyRegistrationCommit, credentialJSON []byte) error {
	if conn == nil {
		return fmt.Errorf("passkey registration transaction is required")
	}
	validatedJSON, err := validatePasskeyRegistrationCommit(input)
	if err != nil {
		return err
	}
	if credentialJSON == nil {
		credentialJSON = validatedJSON
	}
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
	return nil
}
