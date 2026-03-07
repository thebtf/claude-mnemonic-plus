// Package instincts provides parsing and importing of ECC instinct files.
package instincts

// Instinct represents a parsed ECC instinct file.
type Instinct struct {
	ID         string  `yaml:"id"`
	Trigger    string  `yaml:"trigger"`
	Confidence float64 `yaml:"confidence"`
	Domain     string  `yaml:"domain"`
	Source     string  `yaml:"source"`
	Body       string  `yaml:"-"`
	FilePath   string  `yaml:"-"`
}

// ImportResult summarizes the outcome of an import operation.
type ImportResult struct {
	Total    int      `json:"total"`
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}
