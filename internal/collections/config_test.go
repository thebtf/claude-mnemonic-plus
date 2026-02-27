package collections

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingFile(t *testing.T) {
	r, err := Load("/nonexistent/path/that/does/not/exist.yml")
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Empty(t, r.All())
	assert.Empty(t, r.Names())
}

func TestLoadValidYAML(t *testing.T) {
	const yamlContent = `
collections:
  - name: vault
    description: Main knowledge vault
    path_context:
      "/": "global context"
  - name: journal
    description: Daily journal entries
    path_context:
      "/journal": "journal context"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "collections.yml")
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0600))

	r, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, r)

	all := r.All()
	assert.Len(t, all, 2)
	assert.Equal(t, "vault", all[0].Name)
	assert.Equal(t, "journal", all[1].Name)

	c, ok := r.Get("vault")
	require.True(t, ok)
	assert.Equal(t, "vault", c.Name)
	assert.Equal(t, "Main knowledge vault", c.Description)

	c, ok = r.Get("journal")
	require.True(t, ok)
	assert.Equal(t, "journal", c.Name)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid:\tyaml:\t[unclosed"), 0600))

	r, err := Load(path)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNames(t *testing.T) {
	const yamlContent = `
collections:
  - name: zebra
    description: Last alphabetically
  - name: alpha
    description: First alphabetically
  - name: mango
    description: Middle alphabetically
`
	dir := t.TempDir()
	path := filepath.Join(dir, "collections.yml")
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0600))

	r, err := Load(path)
	require.NoError(t, err)

	names := r.Names()
	assert.Equal(t, []string{"alpha", "mango", "zebra"}, names)

	// All() preserves definition order (zebra, alpha, mango)
	all := r.All()
	assert.Equal(t, "zebra", all[0].Name)
	assert.Equal(t, "alpha", all[1].Name)
	assert.Equal(t, "mango", all[2].Name)
}

func TestResolveContext(t *testing.T) {
	c := &Collection{
		Name: "test",
		PathContext: map[string]string{
			"/":        "vault",
			"/2024":    "daily 2024",
			"/2024/01": "january",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "all three prefixes match",
			path:     "/2024/01/15.md",
			expected: "vault\n\ndaily 2024\n\njanuary",
		},
		{
			name:     "two prefixes match",
			path:     "/2024/02/01.md",
			expected: "vault\n\ndaily 2024",
		},
		{
			name:     "only root matches",
			path:     "/other/file.md",
			expected: "vault",
		},
		{
			name:     "no prefix matches",
			path:     "relative/path.md",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ResolveContext(tt.path)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveContextNilCollection(t *testing.T) {
	var c *Collection
	assert.Equal(t, "", c.ResolveContext("/any/path"))
}

func TestResolveContextEmptyPathContext(t *testing.T) {
	c := &Collection{
		Name:        "empty",
		PathContext: map[string]string{},
	}
	assert.Equal(t, "", c.ResolveContext("/any/path"))
}
