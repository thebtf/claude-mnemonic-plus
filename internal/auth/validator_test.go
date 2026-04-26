package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/thebtf/engram/internal/auth"
	gormdb "github.com/thebtf/engram/internal/db/gorm"
)

// stubStore is a TokenStoreReader fake. It records FindByPrefix invocations
// for the absence-of-extra-IO assertion (NFR-2) and returns a fixed candidate
// slice keyed by prefix.
type stubStore struct {
	byPrefix     map[string][]gormdb.APIToken
	prefixCalls  int
	returnErr    error
}

func (s *stubStore) FindByPrefix(_ context.Context, prefix string) ([]gormdb.APIToken, error) {
	s.prefixCalls++
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	return append([]gormdb.APIToken(nil), s.byPrefix[prefix]...), nil
}

// makeKeycard hashes raw using bcrypt.MinCost (test-only) and returns an
// APIToken row with the conventional engram_<prefix><tail> shape.
func makeKeycard(t *testing.T, id, raw, scope string, revoked bool) gormdb.APIToken {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.MinCost)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(raw, "engram_"), "test fixture: raw must start with engram_")
	require.GreaterOrEqual(t, len(raw), 15, "test fixture: raw must be ≥ 15 chars")
	return gormdb.APIToken{
		ID:          id,
		Name:        "test-" + id,
		TokenHash:   string(hash),
		TokenPrefix: raw[7:15],
		Scope:       scope,
		Revoked:     revoked,
	}
}

func TestValidate_EmptyToken(t *testing.T) {
	t.Parallel()
	v := auth.NewValidator("master-secret", &stubStore{})

	id, err := v.Validate(context.Background(), "")

	assert.True(t, errors.Is(err, auth.ErrEmptyToken), "expected ErrEmptyToken")
	assert.Equal(t, auth.Identity{}, id)
}

func TestValidate_MasterMatch(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	v := auth.NewValidator("master-secret", store)

	id, err := v.Validate(context.Background(), "master-secret")

	require.NoError(t, err)
	assert.Equal(t, auth.RoleAdmin, id.Role)
	assert.Equal(t, auth.SourceMaster, id.Source)
	assert.Equal(t, "", id.KeycardID)
	assert.Equal(t, 0, store.prefixCalls,
		"NFR-2: master path MUST NOT touch token store")
}

func TestValidate_MasterEmpty_FallthroughToTier2(t *testing.T) {
	t.Parallel()
	// Server with auth disabled (master == "") should NOT match an empty
	// bearer; FR-1 storage disjointness is preserved by ErrEmptyToken
	// short-circuiting before tier-1.
	v := auth.NewValidator("", &stubStore{})

	_, err := v.Validate(context.Background(), "")

	assert.True(t, errors.Is(err, auth.ErrEmptyToken))
}

func TestValidate_RawGarbage_NoTier2Lookup(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	v := auth.NewValidator("master-secret", store)

	_, err := v.Validate(context.Background(), "not-a-token")

	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
	assert.Equal(t, 0, store.prefixCalls,
		"prefix lookup MUST be skipped for tokens that don't match engram_<8hex>... shape")
}

func TestValidate_TooShortToken_NoTier2Lookup(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	v := auth.NewValidator("master-secret", store)

	_, err := v.Validate(context.Background(), "engram_abc") // 10 chars; need ≥ 15

	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
	assert.Equal(t, 0, store.prefixCalls)
}

func TestValidate_ValidPrefix_NoMatchInStore(t *testing.T) {
	t.Parallel()
	store := &stubStore{
		byPrefix: map[string][]gormdb.APIToken{
			"abcd1234": {}, // empty candidate set for the prefix
		},
	}
	v := auth.NewValidator("master-secret", store)

	_, err := v.Validate(context.Background(), "engram_abcd1234ZZZZZZZZZZZZ")

	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
	assert.Equal(t, 1, store.prefixCalls)
}

func TestValidate_ValidPrefix_NonRevokedMatch(t *testing.T) {
	t.Parallel()
	raw := "engram_deadbeef0000000000000001"
	keycard := makeKeycard(t, "uuid-1", raw, "read-write", false)
	store := &stubStore{
		byPrefix: map[string][]gormdb.APIToken{
			"deadbeef": {keycard},
		},
	}
	v := auth.NewValidator("master-secret", store)

	id, err := v.Validate(context.Background(), raw)

	require.NoError(t, err)
	assert.Equal(t, auth.RoleReadWrite, id.Role)
	assert.Equal(t, auth.SourceClient, id.Source)
	assert.Equal(t, "uuid-1", id.KeycardID)
	assert.Equal(t, 1, store.prefixCalls,
		"NFR-2: exactly one prefix lookup")
}

func TestValidate_ValidPrefix_ReadOnlyScope(t *testing.T) {
	t.Parallel()
	raw := "engram_cafef00d00000000000000ro"
	keycard := makeKeycard(t, "uuid-ro", raw, "read-only", false)
	store := &stubStore{byPrefix: map[string][]gormdb.APIToken{"cafef00d": {keycard}}}
	v := auth.NewValidator("master-secret", store)

	id, err := v.Validate(context.Background(), raw)

	require.NoError(t, err)
	assert.Equal(t, auth.RoleReadOnly, id.Role)
	assert.Equal(t, auth.SourceClient, id.Source)
}

func TestValidate_PrefixCollision_TwoCandidates_OneMatches(t *testing.T) {
	t.Parallel()
	rawA := "engram_facade00000000000000000aa"
	rawB := "engram_facade00000000000000000bb"
	cardA := makeKeycard(t, "uuid-a", rawA, "read-write", false)
	cardB := makeKeycard(t, "uuid-b", rawB, "read-only", false)
	store := &stubStore{
		byPrefix: map[string][]gormdb.APIToken{
			"facade00": {cardA, cardB}, // both share prefix
		},
	}
	v := auth.NewValidator("master-secret", store)

	idA, errA := v.Validate(context.Background(), rawA)
	idB, errB := v.Validate(context.Background(), rawB)

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, "uuid-a", idA.KeycardID)
	assert.Equal(t, "uuid-b", idB.KeycardID)
	assert.Equal(t, 2, store.prefixCalls)
}

func TestValidate_PrefixCollision_NeitherMatches(t *testing.T) {
	t.Parallel()
	otherRaw := "engram_facade00000000000000000xx"
	cardA := makeKeycard(t, "uuid-a", "engram_facade00000000000000000aa", "read-write", false)
	cardB := makeKeycard(t, "uuid-b", "engram_facade00000000000000000bb", "read-only", false)
	store := &stubStore{
		byPrefix: map[string][]gormdb.APIToken{"facade00": {cardA, cardB}},
	}
	v := auth.NewValidator("master-secret", store)

	_, err := v.Validate(context.Background(), otherRaw)

	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}

func TestValidate_StoreError_PropagatesAsAuthFailure(t *testing.T) {
	t.Parallel()
	store := &stubStore{returnErr: errors.New("db down")}
	v := auth.NewValidator("master-secret", store)

	_, err := v.Validate(context.Background(), "engram_abcd1234ZZZZZZZZZZZZ")

	require.Error(t, err)
	// Adapters expect ErrInvalidCredentials OR a wrapped store error; the
	// validator MUST NOT silently succeed on store failures.
	assert.False(t, errors.Is(err, auth.ErrEmptyToken))
}

func TestValidate_MasterAndKeycardDistinguishable(t *testing.T) {
	t.Parallel()
	// Verifies FR-1 storage disjointness from the validator's perspective:
	// the master path produces SourceMaster; the keycard path produces
	// SourceClient. There is no fallback that conflates them.
	rawClient := "engram_aaaa1111000000000000bbbb"
	keycard := makeKeycard(t, "uuid-c", rawClient, "read-write", false)
	store := &stubStore{byPrefix: map[string][]gormdb.APIToken{"aaaa1111": {keycard}}}
	v := auth.NewValidator("master-secret", store)

	idMaster, _ := v.Validate(context.Background(), "master-secret")
	idClient, _ := v.Validate(context.Background(), rawClient)

	assert.NotEqual(t, idMaster.Source, idClient.Source,
		"FR-1: master and client identities MUST be distinguishable by Source")
	assert.Equal(t, auth.SourceMaster, idMaster.Source)
	assert.Equal(t, auth.SourceClient, idClient.Source)
}

// Compile-time assertion: gormdb.TokenStore must satisfy auth.TokenStoreReader.
// If gormdb.TokenStore changes its FindByPrefix signature, this file fails to
// compile, surfacing the contract drift immediately.
var _ auth.TokenStoreReader = (*gormdb.TokenStore)(nil)
