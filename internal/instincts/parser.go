package instincts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFile parses a single instinct markdown file with YAML frontmatter.
func ParseFile(path string) (*Instinct, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instinct file: %w", err)
	}

	content := string(data)

	// Split YAML frontmatter from body
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing YAML frontmatter in %s", path)
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed YAML frontmatter in %s", path)
	}

	var inst Instinct
	if err := yaml.Unmarshal([]byte(parts[0]), &inst); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter in %s: %w", path, err)
	}

	inst.Body = strings.TrimSpace(parts[1])
	inst.FilePath = path

	return &inst, nil
}

// ParseDir parses all .md files in a directory (non-recursive).
func ParseDir(dir string) ([]*Instinct, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read instinct directory: %w", err)}
	}

	var instincts []*Instinct
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		inst, err := ParseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		instincts = append(instincts, inst)
	}

	return instincts, errs
}
