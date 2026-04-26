package auth

import "errors"

// Sentinel errors returned by Validator.Validate. Adapters map them to
// transport-specific status codes:
//
//   - HTTP middleware → 401 Unauthorized
//   - gRPC interceptor → codes.Unauthenticated
//
// Use errors.Is to discriminate between cases.
var (
	// ErrEmptyToken signals that the bearer string was empty after Bearer
	// prefix stripping. Transport adapters typically map this to
	// "missing authorization header" (gRPC) or 401 (HTTP).
	ErrEmptyToken = errors.New("auth: empty token")

	// ErrInvalidCredentials signals that the bearer did not match either
	// the master operator key or any non-revoked api_tokens row. The
	// validator deliberately does NOT distinguish "no such token" from
	// "wrong-shape garbage" in the error message — exposing that
	// distinction would help token-fishing attacks.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrRevoked signals that the bearer matched an api_tokens row whose
	// Revoked flag is set. Surfaced to logs (with token id) but transport
	// adapters MAY collapse it into ErrInvalidCredentials at the wire to
	// avoid leaking revocation state to anonymous attackers.
	ErrRevoked = errors.New("auth: token revoked")
)
