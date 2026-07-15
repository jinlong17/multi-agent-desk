package domain

import (
	"errors"
	"fmt"
)

// ErrorCode is a stable, machine-readable application error code.
type ErrorCode string

const (
	CodeInvalidArgument         ErrorCode = "invalid_argument"
	CodeNotFound                ErrorCode = "not_found"
	CodeAlreadyExists           ErrorCode = "already_exists"
	CodeConflict                ErrorCode = "conflict"
	CodeUnauthenticated         ErrorCode = "unauthenticated"
	CodePermissionDenied        ErrorCode = "permission_denied"
	CodeUnsupportedVersion      ErrorCode = "unsupported_version"
	CodeMethodNotFound          ErrorCode = "method_not_found"
	CodeFrameTooLarge           ErrorCode = "frame_too_large"
	CodeDeadlineExceeded        ErrorCode = "deadline_exceeded"
	CodeResourceExhausted       ErrorCode = "resource_exhausted"
	CodeDaemonUnavailable       ErrorCode = "daemon_unavailable"
	CodeSchemaIncompatible      ErrorCode = "schema_incompatible"
	CodeInvalidTransition       ErrorCode = "invalid_transition"
	CodeLeaseRequired           ErrorCode = "lease_required"
	CodeLeaseHeld               ErrorCode = "lease_held"
	CodeStaleLease              ErrorCode = "stale_lease"
	CodeReplayUnavailable       ErrorCode = "replay_unavailable"
	CodeVaultLocked             ErrorCode = "vault_locked"
	CodeProviderFailed          ErrorCode = "provider_failed"
	CodeUnsupportedPlatform     ErrorCode = "unsupported_platform"
	CodeMaterializationConflict ErrorCode = "materialization_conflict"
	CodeQuarantined             ErrorCode = "quarantined"
)

// Error is safe to return across a public command or protocol boundary.
// Cause is intentionally excluded from Error so storage or payload details do
// not leak through ordinary formatting.
type Error struct {
	Code    ErrorCode
	Message string
	cause   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.cause
}

// NewError creates a stable safe error.
func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError preserves an internal cause while retaining a bounded public
// message.
func WrapError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// CodeOf extracts a stable code without exposing an internal cause.
func CodeOf(err error) ErrorCode {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
