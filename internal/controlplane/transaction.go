package controlplane

import (
	"context"
	"database/sql"
	"fmt"
)

// withImmediateConn is used by legacy/direct Store entry points. P2 HTTP
// mutations call the corresponding *Tx helper from WithAuthIdempotency so the
// global gate, security transition, audit, and receipt share one transaction.
func withImmediateConn[T any](ctx context.Context, db *sql.DB, label string, action func(*sql.Conn) (T, error)) (result T, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("acquire %s connection: %w", label, err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return result, fmt.Errorf("begin %s: %w", label, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	result, err = action(conn)
	if err != nil {
		return result, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return result, fmt.Errorf("commit %s: %w", label, err)
	}
	committed = true
	return result, nil
}
