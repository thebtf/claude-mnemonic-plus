package auth

import "context"

// Context-key types. Both are unexported as type definitions but exported as
// VALUE singletons (RoleKey, SourceKey) so other packages can read context
// values without taking a dependency on internal/worker's authRoleKey{}
// (which historically lives in handlers_auth.go and predates this package).
//
// Two packages need to read these keys:
//   - internal/grpcserver — sets identity from gRPC interceptor
//   - internal/worker     — sets identity from HTTP middleware AND reads it
//                           from request handlers (issuance gate per FR-6)
//
// To preserve backwards compatibility with existing internal/worker code that
// reads the literal authRoleKey{} struct value, internal/worker/middleware.go
// MAY alias auth.RoleKey to its own authRoleKey{} (same Type Identity is not
// required — both consumers will be migrated together in Phase 3).

type roleKeyType struct{}
type sourceKeyType struct{}

// RoleKey is the context value key under which Identity.Role is stored. Read
// it via IdentityFrom or directly with ctx.Value(auth.RoleKey).
var RoleKey = roleKeyType{}

// SourceKey is the context value key under which Identity.Source is stored.
// Issuance endpoints (FR-6 / Clarification C4) MUST gate on this value being
// SourceSession; bearer-based callers (master OR client) MUST be rejected.
var SourceKey = sourceKeyType{}

// WithIdentity returns a new context carrying id under the canonical Role and
// Source keys. Intended to be called once per request after Validator.Validate
// (or after a session-cookie path resolves the user role).
func WithIdentity(ctx context.Context, id Identity) context.Context {
	ctx = context.WithValue(ctx, RoleKey, id.Role)
	ctx = context.WithValue(ctx, SourceKey, id.Source)
	return ctx
}

// IdentityFrom extracts an Identity from ctx. The second return is false when
// no role has been set on the context — callers should treat that as
// "unauthenticated" and respond accordingly. KeycardID is NOT round-tripped
// through context (issuance/audit handlers that need it should consult their
// own state).
func IdentityFrom(ctx context.Context) (Identity, bool) {
	role, ok := ctx.Value(RoleKey).(Role)
	if !ok {
		return Identity{}, false
	}
	src, _ := ctx.Value(SourceKey).(Source)
	return Identity{Role: role, Source: src}, true
}

// RoleFrom returns the role string for backward-compat with handlers that
// previously read worker.authRoleKey{}. Empty string if no role.
func RoleFrom(ctx context.Context) string {
	if role, ok := ctx.Value(RoleKey).(Role); ok {
		return string(role)
	}
	return ""
}

// SourceFrom returns the auth source string. Empty string if no source set
// (e.g., authentication was disabled).
func SourceFrom(ctx context.Context) string {
	if src, ok := ctx.Value(SourceKey).(Source); ok {
		return string(src)
	}
	return ""
}
