// Package gengen implements generative project configuration for OneBase.
// It converts a natural-language prompt into a complete project structure
// (YAML metadata + DSL source files) by matching keywords to domain bundles.
package gengen

// DomainRule describes a project domain: keywords for detection, templates
// to copy, and optional addons (e.g. "edi" adds counterparty bank details).
type DomainRule struct {
	Keywords  []string
	Templates []string          // priority-ordered; first existing dir wins
	Addons    map[string]Addon
}

// Addon is an optional feature pack that extends a domain bundle.
// It contains additional YAML fields, constants, or DSL files.
type Addon struct {
	Name        string
	Description string
	SourceDir   string // relative to project root
}

// AnalyzeResult is the output of the semantic analyzer.
type AnalyzeResult struct {
	Domain    string   // matched domain name
	Template  string   // resolved path to the template directory
	Addons    []string // requested addon names
	Confident bool     // true if a single domain matched clearly
	Ambiguous []string // tied domain names (when Confident == false)
}
