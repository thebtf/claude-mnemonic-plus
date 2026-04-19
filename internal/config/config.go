// Package config provides configuration management for engram.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultWorkerPort is the default HTTP port for the worker service.
	DefaultWorkerPort = 37777

	// DefaultModel for SDK agent (use "haiku" for cost-efficient processing).
	// Claude Code CLI accepts aliases: haiku, sonnet, opus (always latest versions)
	DefaultModel = "haiku"
)

// DefaultObservationTypes are the observation types to include in context.
var DefaultObservationTypes = []string{
	"bugfix", "feature", "refactor", "change", "discovery", "decision",
}

// DefaultObservationConcepts are the concept tags to include in context.
var DefaultObservationConcepts = []string{
	"how-it-works", "why-it-exists", "what-changed",
	"problem-solution", "gotcha", "pattern", "trade-off",
}

// CriticalConcepts are concepts that indicate "must know" information.
// Observations with these concepts are prioritized in context injection.
var CriticalConcepts = []string{
	"gotcha", "pattern", "problem-solution", "trade-off",
}

// Config holds the application configuration.
// Field order optimized for memory alignment (fieldalignment).
type Config struct {
	ContextFullField          string `json:"context_full_field"`
	DBPath                    string `json:"db_path"`
	Model                     string `json:"model"`
	VectorStorageStrategy     string `json:"vector_storage_strategy"`
	EmbeddingProvider         string `json:"embedding_provider"`
	EmbeddingBaseURL          string `json:"embedding_base_url"`
	EmbeddingModelName        string `json:"embedding_model_name"`
	EmbeddingDimensions       int    `json:"embedding_dimensions"`
	EmbeddingAPIKey           string
	DatabaseDSN               string   `json:"-"`                  // env-only: DATABASE_DSN (contains password, never JSON)
	DatabaseMaxConns          int      `json:"database_max_conns"` // PostgreSQL pool size (default: 10)
	ContextObsConcepts        []string `json:"context_obs_concepts"`
	ContextObsTypes           []string `json:"context_obs_types"`
	ContextFullCount          int      `json:"context_full_count"`
	ContextRelevanceThreshold float64  `json:"context_relevance_threshold"`
	WorkerPort                int      `json:"worker_port"`
	ContextObservations       int      `json:"context_observations"`
	ContextMaxPromptResults   int      `json:"context_max_prompt_results"`
	ContextSessionCount       int      `json:"context_session_count"`
	MaxConns                  int      `json:"max_conns"`
	HubThreshold              int      `json:"hub_threshold"`
	WorkerHost                string   // env-only
	WorkerToken               string   // env-only
	CollectionConfigPath      string   // env-only
	WorkstationID             string   // env-only: WORKSTATION_ID
	TelemetryEnabled          bool     `json:"telemetry_enabled"`
	LogBufferSize             int      `json:"log_buffer_size"`    // Ring buffer capacity for /api/logs (default: 10000)
	ContextMaxTokens          int      `json:"context_max_tokens"` // Token budget for context injection (default: 8000, 0=unlimited)
	StoreMemoryHardLimit      int      `json:"store_memory_hard_limit"`      // Max chars for store_memory content (default: 10000)
	StoreMemorySoftLimit      int      `json:"store_memory_soft_limit"`      // Chars above which content is truncated (default: 1000)
	StoreMemoryDedupThreshold float64  `json:"store_memory_dedup_threshold"` // Cosine similarity for dedup (default: 0.92)
	StoreMemorySummarize      bool     `json:"store_memory_summarize"`       // Use LLM to summarize long content (default: false)
	EncryptionKeyFile         string   `json:"-"`                            // env-only: ENGRAM_ENCRYPTION_KEY_FILE (path to vault.key)
	EncryptionKey             string   `json:"-"`                            // env-only: ENGRAM_ENCRYPTION_KEY (hex-encoded 256-bit key)
	AlwaysInjectLimit         int      `json:"always_inject_limit"` // ENGRAM_ALWAYS_INJECT_LIMIT (default: 20)
	ProjectInjectLimit        int      `json:"project_inject_limit"` // ENGRAM_PROJECT_INJECT_LIMIT (default: 15)
	InjectUnified             bool     `json:"inject_unified"`      // ENGRAM_INJECT_UNIFIED (default: true) — emergency rollback flag; removed after two release cycles
	EnforceSourceProject      bool     `json:"enforce_source_project"` // ENGRAM_ENFORCE_SOURCE_PROJECT (default: true)

	// Signal weights for reward computation (closed-loop learning FR-7)
	SignalWeights map[string]float64 `json:"signal_weights"`

	// OutcomeRecorderIntervalMinutes controls how often the periodic outcome recorder runs.
	// It records outcomes for sessions that have injection records but no outcome yet.
	// Env: ENGRAM_OUTCOME_RECORDER_INTERVAL_MINUTES (default: 15)
	OutcomeRecorderIntervalMinutes int `json:"outcome_recorder_interval_minutes"`

	// Authentik SSO forward-auth integration
	// ENGRAM_AUTHENTIK_ENABLED: enable Authentik header detection (default: false)
	// ENGRAM_AUTHENTIK_AUTO_PROVISION: auto-create users from Authentik headers (default: false)
	// ENGRAM_AUTHENTIK_TRUSTED_PROXIES: comma-separated list of trusted proxy IPs (default: empty)
	AuthentikEnabled        bool     `json:"authentik_enabled"`
	AuthentikAutoProvision  bool     `json:"authentik_auto_provision"`
	AuthentikTrustedProxies []string `json:"authentik_trusted_proxies"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configMu     sync.RWMutex
)

// DataDir returns the data directory path (~/.engram).
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".engram")
}

// DBPath returns the database file path.
func DBPath() string {
	if v := strings.TrimSpace(os.Getenv("ENGRAM_DB_PATH")); v != "" {
		return v
	}
	return filepath.Join(DataDir(), "engram.db")
}

// SettingsPath returns the settings file path.
func SettingsPath() string {
	return filepath.Join(DataDir(), "settings.json")
}

// EnsureDataDir creates the data directory if it doesn't exist.
// Uses 0700 permissions (owner-only) for security.
func EnsureDataDir() error {
	return os.MkdirAll(DataDir(), 0700)
}

// EnsureSettings creates a default settings file if it doesn't exist.
func EnsureSettings() error {
	path := SettingsPath()

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists
	}

	// Create default settings file with comments
	defaultSettings := `{
  "ENGRAM_WORKER_PORT": 37777,
  "ENGRAM_MODEL": "haiku",
  "ENGRAM_CONTEXT_OBSERVATIONS": 100,
  "ENGRAM_CONTEXT_FULL_COUNT": 25,
  "ENGRAM_CONTEXT_SESSION_COUNT": 10
}
`
	return os.WriteFile(path, []byte(defaultSettings), 0600)
}

// EnsureAll ensures all required directories and files exist.
func EnsureAll() error {
	if err := EnsureDataDir(); err != nil {
		return err
	}
	if err := EnsureSettings(); err != nil {
		return err
	}
	return nil
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		WorkerPort:                     DefaultWorkerPort,
		DBPath:                         DBPath(),
		MaxConns:                       4,
		Model:                          DefaultModel,
		VectorStorageStrategy:          "hub", // Hub storage strategy (LEANN-inspired)
		EmbeddingProvider:              "openai",
		EmbeddingBaseURL:               "https://api.openai.com/v1",
		EmbeddingModelName:             "text-embedding-3-small",
		EmbeddingDimensions:            1536,
		HubThreshold:                   5, // Require 5+ accesses to store embedding
		ContextObservations:            100,
		ContextFullCount:               25,
		ContextSessionCount:            10,
		ContextFullField:               "narrative",
		ContextObsTypes:                DefaultObservationTypes,
		ContextObsConcepts:             DefaultObservationConcepts,
		ContextRelevanceThreshold:      0.3,  // Minimum 30% similarity to include
		ContextMaxPromptResults:        10,   // Cap at 10 results max (0 = no cap, threshold only)
		ContextMaxTokens:               8000, // ~8K tokens default budget for context injection
		WorkerHost:                     "127.0.0.1",
		DatabaseMaxConns:               10,
		TelemetryEnabled:               true,
		StoreMemoryHardLimit:           10000,
		StoreMemorySoftLimit:           1000,
		StoreMemorySummarize:           false,
		StoreMemoryDedupThreshold:      0.92,
		AlwaysInjectLimit:              20,   // Inject up to 20 always-inject observations per session
		ProjectInjectLimit:             15,   // Inject up to 15 project-scoped observations per session
		InjectUnified:                  true, // Use unified RetrieveRelevant path for inject (FR-3). Set ENGRAM_INJECT_UNIFIED=false for emergency rollback.
		EnforceSourceProject:           true, // Enforce source/project scoping on store/recall (T010)
		OutcomeRecorderIntervalMinutes: 15,
		SignalWeights: map[string]float64{
			"git_commit":   1.0,
			"pr_created":   2.0,
			"pr_merged":    3.0,
			"test_passed":  0.5,
			"error_streak": -0.5,
		},
	}
}

// Load loads configuration from the settings file, merging with defaults.
func Load() (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(SettingsPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load settings from JSON file (skip if file doesn't exist)
	if err == nil {
		var settings map[string]interface{}
		if err := json.Unmarshal(data, &settings); err == nil {
			if v, ok := settings["ENGRAM_WORKER_PORT"].(float64); ok {
				cfg.WorkerPort = int(v)
			}
			// WorkerHost and WorkerToken are env-only (not settable via JSON file).
			if v, ok := settings["ENGRAM_DB_PATH"].(string); ok && v != "" {
				cfg.DBPath = v
			}
			if v, ok := settings["ENGRAM_MODEL"].(string); ok {
				cfg.Model = v
			}
			if v, ok := settings["ENGRAM_CONTEXT_OBSERVATIONS"].(float64); ok {
				cfg.ContextObservations = int(v)
			}
			if v, ok := settings["ENGRAM_CONTEXT_FULL_COUNT"].(float64); ok {
				cfg.ContextFullCount = int(v)
			}
			if v, ok := settings["ENGRAM_CONTEXT_SESSION_COUNT"].(float64); ok {
				cfg.ContextSessionCount = int(v)
			}
			if v, ok := settings["ENGRAM_CONTEXT_OBS_TYPES"].(string); ok && v != "" {
				cfg.ContextObsTypes = splitTrim(v)
			}
			if v, ok := settings["ENGRAM_CONTEXT_OBS_CONCEPTS"].(string); ok && v != "" {
				cfg.ContextObsConcepts = splitTrim(v)
			}
			if v, ok := settings["ENGRAM_CONTEXT_RELEVANCE_THRESHOLD"].(float64); ok && v >= 0 && v <= 1 {
				cfg.ContextRelevanceThreshold = v
			}
			if v, ok := settings["ENGRAM_CONTEXT_MAX_PROMPT_RESULTS"].(float64); ok && v >= 0 {
				cfg.ContextMaxPromptResults = int(v)
			}
			if v, ok := settings["ENGRAM_VECTOR_STORAGE_STRATEGY"].(string); ok && v != "" {
				cfg.VectorStorageStrategy = v
			}
			if v, ok := settings["EMBEDDING_PROVIDER"].(string); ok && v != "" {
				cfg.EmbeddingProvider = v
			}
			if v, ok := settings["EMBEDDING_BASE_URL"].(string); ok && v != "" {
				cfg.EmbeddingBaseURL = v
			}
			// EMBEDDING_API_KEY is env-only, NOT loaded from JSON file.
			if v, ok := settings["EMBEDDING_MODEL_NAME"].(string); ok && v != "" {
				cfg.EmbeddingModelName = v
			}
			if v, ok := settings["EMBEDDING_DIMENSIONS"].(float64); ok && v > 0 {
				cfg.EmbeddingDimensions = int(v)
			}
			if v, ok := settings["ENGRAM_HUB_THRESHOLD"].(float64); ok && v > 0 {
				cfg.HubThreshold = int(v)
			}
			if v, ok := settings["ENGRAM_ENFORCE_SOURCE_PROJECT"].(bool); ok {
				cfg.EnforceSourceProject = v
			}
		}
	}

	// Environment variable overrides (take precedence over JSON settings)
	if v := strings.TrimSpace(os.Getenv("ENGRAM_DB_PATH")); v != "" {
		cfg.DBPath = v
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_WORKER_HOST")); v != "" {
		cfg.WorkerHost = v
	}
	// Auth admin token: new name takes priority, old name is deprecated alias
	if v := strings.TrimSpace(os.Getenv("ENGRAM_AUTH_ADMIN_TOKEN")); v != "" {
		cfg.WorkerToken = v
	} else if v := strings.TrimSpace(os.Getenv("ENGRAM_API_TOKEN")); v != "" {
		cfg.WorkerToken = v
	}
	if v := envFirstOf("ENGRAM_EMBEDDING_PROVIDER", "EMBEDDING_PROVIDER"); v != "" {
		cfg.EmbeddingProvider = v
	}
	if v := envFirstOf("ENGRAM_EMBEDDING_BASE_URL", "EMBEDDING_BASE_URL"); v != "" {
		cfg.EmbeddingBaseURL = v
	}
	if v := envFirstOf("ENGRAM_EMBEDDING_API_KEY", "EMBEDDING_API_KEY"); v != "" {
		cfg.EmbeddingAPIKey = v
	}
	if v := envFirstOf("ENGRAM_EMBEDDING_MODEL_NAME", "EMBEDDING_MODEL_NAME"); v != "" {
		cfg.EmbeddingModelName = v
	}
	if v := envFirstOf("ENGRAM_EMBEDDING_DIMENSIONS", "EMBEDDING_DIMENSIONS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			cfg.EmbeddingDimensions = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_CONTEXT_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.ContextMaxTokens = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_DSN")); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := strings.TrimSpace(os.Getenv("DATABASE_MAX_CONNS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DatabaseMaxConns = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("COLLECTION_CONFIG")); v != "" {
		cfg.CollectionConfigPath = v
	}
	if v := strings.TrimSpace(os.Getenv("WORKSTATION_ID")); v != "" {
		cfg.WorkstationID = v
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_TELEMETRY_ENABLED")); v == "false" || v == "0" {
		cfg.TelemetryEnabled = false
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_LOG_BUFFER_SIZE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LogBufferSize = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_ENCRYPTION_KEY_FILE")); v != "" {
		cfg.EncryptionKeyFile = v
	}
	// ENGRAM_VAULT_KEY is the primary name; ENGRAM_ENCRYPTION_KEY is accepted as alias.
	if v := strings.TrimSpace(os.Getenv("ENGRAM_VAULT_KEY")); v != "" {
		cfg.EncryptionKey = v
	} else if v := strings.TrimSpace(os.Getenv("ENGRAM_ENCRYPTION_KEY")); v != "" {
		cfg.EncryptionKey = v
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_ALWAYS_INJECT_LIMIT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AlwaysInjectLimit = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_PROJECT_INJECT_LIMIT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ProjectInjectLimit = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_INJECT_UNIFIED")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.InjectUnified = b
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_ENFORCE_SOURCE_PROJECT")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnforceSourceProject = b
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_OUTCOME_RECORDER_INTERVAL_MINUTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.OutcomeRecorderIntervalMinutes = n
		}
	}

	// Authentik SSO forward-auth integration
	if v := strings.TrimSpace(os.Getenv("ENGRAM_AUTHENTIK_ENABLED")); v == "true" || v == "1" {
		cfg.AuthentikEnabled = true
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_AUTHENTIK_AUTO_PROVISION")); v == "true" || v == "1" {
		cfg.AuthentikAutoProvision = true
	}
	if v := strings.TrimSpace(os.Getenv("ENGRAM_AUTHENTIK_TRUSTED_PROXIES")); v != "" {
		cfg.AuthentikTrustedProxies = splitTrim(v)
	}

	return cfg, nil
}

// envFirstOf returns the first non-empty env var value from the given keys.
// Allows ENGRAM_-prefixed vars to take priority over legacy unprefixed vars.
func envFirstOf(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// splitTrim splits a comma-separated string and trims whitespace.
func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Get returns the global configuration, loading it if necessary.
func Get() *Config {
	configOnce.Do(func() {
		var err error
		globalConfig, err = Load()
		if err != nil {
			globalConfig = Default()
		}
	})

	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// Reload re-reads configuration from disk and updates the global config atomically.
// Returns the new config and any fields that changed.
func Reload() (*Config, []string, error) {
	newCfg, err := Load()
	if err != nil {
		return nil, nil, err
	}

	configMu.Lock()
	old := globalConfig
	globalConfig = newCfg
	configMu.Unlock()

	// Detect changed fields for logging
	var changed []string
	if old != nil {
		if old.Model != newCfg.Model {
			changed = append(changed, "model")
		}
		if old.EmbeddingBaseURL != newCfg.EmbeddingBaseURL {
			changed = append(changed, "embedding_base_url")
		}
		if old.EmbeddingModelName != newCfg.EmbeddingModelName {
			changed = append(changed, "embedding_model_name")
		}
		if old.ContextMaxTokens != newCfg.ContextMaxTokens {
			changed = append(changed, "context_max_tokens")
		}
		if old.ContextObservations != newCfg.ContextObservations {
			changed = append(changed, "context_observations")
		}
		if old.WorkerPort != newCfg.WorkerPort {
			changed = append(changed, "worker_port (requires restart)")
		}
		if old.WorkerToken != newCfg.WorkerToken {
			changed = append(changed, "worker_token (requires restart)")
		}
	}

	return newCfg, changed, nil
}

// GetWorkerPort returns the worker port from environment or config.
func GetWorkerPort() int {
	if port := os.Getenv("ENGRAM_WORKER_PORT"); port != "" {
		var p int
		if err := json.Unmarshal([]byte(port), &p); err == nil && p > 0 {
			return p
		}
	}
	return Get().WorkerPort
}

// GetWorkerHost returns the worker host from environment, config, or fallback.
func GetWorkerHost() string {
	host := strings.TrimSpace(os.Getenv("ENGRAM_WORKER_HOST"))
	if host != "" {
		return host
	}
	if cfgHost := strings.TrimSpace(Get().WorkerHost); cfgHost != "" {
		return cfgHost
	}
	return "127.0.0.1"
}

// GetWorkerToken returns the worker authentication token.
// GetWorkerToken returns the admin authentication token.
// Checks ENGRAM_AUTH_ADMIN_TOKEN first (preferred), falls back to ENGRAM_API_TOKEN (deprecated).
func GetWorkerToken() string {
	if v := strings.TrimSpace(os.Getenv("ENGRAM_AUTH_ADMIN_TOKEN")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("ENGRAM_API_TOKEN"))
}

// GetEmbeddingProvider returns the embedding provider (e.g., "openai").
func GetEmbeddingProvider() string {
	if v := envFirstOf("ENGRAM_EMBEDDING_PROVIDER", "EMBEDDING_PROVIDER"); v != "" {
		return v
	}
	return Get().EmbeddingProvider
}

// GetEmbeddingBaseURL returns the OpenAI-compatible API base URL.
func GetEmbeddingBaseURL() string {
	if v := envFirstOf("ENGRAM_EMBEDDING_BASE_URL", "EMBEDDING_BASE_URL"); v != "" {
		return v
	}
	return Get().EmbeddingBaseURL
}

// GetEmbeddingAPIKey returns the embedding API key (env-only, never from config file).
func GetEmbeddingAPIKey() string {
	return envFirstOf("ENGRAM_EMBEDDING_API_KEY", "EMBEDDING_API_KEY")
}

// GetDatabaseDSN returns the PostgreSQL DSN.
// env DATABASE_DSN takes priority (contains password, never stored in config file).
// Returns empty string if not configured.
func GetDatabaseDSN() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_DSN")); v != "" {
		return v
	}
	return Get().DatabaseDSN
}

// GetEmbeddingModelName returns the embedding model name for external providers.
func GetEmbeddingModelName() string {
	if v := envFirstOf("ENGRAM_EMBEDDING_MODEL_NAME", "EMBEDDING_MODEL_NAME"); v != "" {
		return v
	}
	if cfg := Get(); cfg.EmbeddingModelName != "" {
		return cfg.EmbeddingModelName
	}
	return "text-embedding-3-small"
}

// GetCollectionConfigPath returns the path to the collections YAML config.
// Falls back to ~/.config/engram/collections.yml if env is unset.
func GetCollectionConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("COLLECTION_CONFIG")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "engram", "collections.yml")
}

// GetWorkstationID returns the workstation identifier from environment.
// Returns empty string if not set; caller should fall back to sessions.WorkstationID().
func GetWorkstationID() string {
	return strings.TrimSpace(os.Getenv("WORKSTATION_ID"))
}

// GetEmbeddingDimensions returns the embedding vector dimensions for external providers.
func GetEmbeddingDimensions() int {
	if v := envFirstOf("ENGRAM_EMBEDDING_DIMENSIONS", "EMBEDDING_DIMENSIONS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			return d
		}
	}
	if cfg := Get(); cfg.EmbeddingDimensions > 0 {
		return cfg.EmbeddingDimensions
	}
	return 1536
}

// GetEmbeddingTruncate returns whether client-side MRL truncation is enabled.
// When true, vectors with more dimensions than configured are truncated and
// L2-normalized. Requires a Matryoshka-trained model for quality preservation.
// When false (default), a dimension mismatch causes an error (fail-fast).
func GetEmbeddingTruncate() bool {
	v := envFirstOf("ENGRAM_EMBEDDING_TRUNCATE", "EMBEDDING_TRUNCATE")
	return v == "true" || v == "1" || v == "yes"
}
