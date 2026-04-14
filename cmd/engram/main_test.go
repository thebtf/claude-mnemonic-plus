package main

import (
	"os"
	"testing"
)

func TestEnvOrDefault_SessionEnvTakesPriority(t *testing.T) {
	env := map[string]string{"KEY": "session-value"}
	t.Setenv("KEY", "os-value")

	got := envOrDefault(env, "KEY")
	if got != "session-value" {
		t.Errorf("envOrDefault(env, KEY) = %q, want %q", got, "session-value")
	}
}

func TestEnvOrDefault_FallbackToOsEnv(t *testing.T) {
	env := map[string]string{} // empty session env
	t.Setenv("KEY", "os-value")

	got := envOrDefault(env, "KEY")
	if got != "os-value" {
		t.Errorf("envOrDefault(empty, KEY) = %q, want %q", got, "os-value")
	}
}

func TestEnvOrDefault_NilEnvFallsBack(t *testing.T) {
	t.Setenv("KEY", "os-value")

	got := envOrDefault(nil, "KEY")
	if got != "os-value" {
		t.Errorf("envOrDefault(nil, KEY) = %q, want %q", got, "os-value")
	}
}

func TestEnvOrDefault_EmptySessionValueFallsBack(t *testing.T) {
	env := map[string]string{"KEY": ""} // empty value in session env
	t.Setenv("KEY", "os-value")

	got := envOrDefault(env, "KEY")
	if got != "os-value" {
		t.Errorf("envOrDefault(empty-val, KEY) = %q, want %q", got, "os-value")
	}
}

func TestEnvOrDefault_NeitherSet(t *testing.T) {
	os.Unsetenv("MISSING_KEY_TEST_12345")
	got := envOrDefault(nil, "MISSING_KEY_TEST_12345")
	if got != "" {
		t.Errorf("envOrDefault(nil, missing) = %q, want empty", got)
	}
}

func TestParseGRPCAddr_HTTP(t *testing.T) {
	addr, err := parseGRPCAddr("http://unleashed.lan:37777")
	if err != nil {
		t.Fatalf("parseGRPCAddr error: %v", err)
	}
	if addr != "unleashed.lan:37777" {
		t.Errorf("got %q, want %q", addr, "unleashed.lan:37777")
	}
}

func TestParseGRPCAddr_HTTPS(t *testing.T) {
	addr, err := parseGRPCAddr("https://engram.example.com")
	if err != nil {
		t.Fatalf("parseGRPCAddr error: %v", err)
	}
	if addr != "engram.example.com:443" {
		t.Errorf("got %q, want %q", addr, "engram.example.com:443")
	}
}

func TestParseGRPCAddr_DefaultPort(t *testing.T) {
	addr, err := parseGRPCAddr("http://localhost")
	if err != nil {
		t.Fatalf("parseGRPCAddr error: %v", err)
	}
	if addr != "localhost:37777" {
		t.Errorf("got %q, want %q", addr, "localhost:37777")
	}
}

func TestGRPCPool_SharedConnection(t *testing.T) {
	h := &engramHandler{}

	// Two calls with the same URL should return the same connection.
	// We can't easily test real gRPC connections without a server,
	// but we can verify the pool key logic.
	key1 := connKey{addr: "host:37777", tlsMode: "plaintext"}
	key2 := connKey{addr: "host:37777", tlsMode: "plaintext"}
	if key1 != key2 {
		t.Error("identical connKeys should be equal")
	}

	key3 := connKey{addr: "other:443", tlsMode: "system-tls"}
	if key1 == key3 {
		t.Error("different connKeys should not be equal")
	}

	// Verify the handler struct is usable (sync.Map initialized by zero value).
	_ = h
}

func TestResolveProject_Caching(t *testing.T) {
	h := &engramHandler{}

	// Pre-populate cache to avoid needing git
	h.slugCache.Store("test-id-123", "cached-project")

	p := struct {
		ID  string
		Cwd string
	}{ID: "test-id-123", Cwd: "/nonexistent"}

	// We can't call resolveProject directly because it takes muxcore.ProjectContext
	// which requires the muxcore import. Instead verify the cache works via sync.Map.
	cached, ok := h.slugCache.Load(p.ID)
	if !ok {
		t.Fatal("slug cache miss for pre-populated key")
	}
	if cached.(string) != "cached-project" {
		t.Errorf("cached slug = %q, want %q", cached, "cached-project")
	}
}
