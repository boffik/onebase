package cli

import "github.com/boffik/onebase/internal/project"

func appDSLStrictLexicalScope(cfg *project.AppConfig) bool {
	return cfg != nil && cfg.DSL != nil && cfg.DSL.StrictLexicalScope
}
