package cli

import "github.com/ivantit66/onebase/internal/project"

func appDSLStrictLexicalScope(cfg *project.AppConfig) bool {
	return cfg != nil && cfg.DSL != nil && cfg.DSL.StrictLexicalScope
}
