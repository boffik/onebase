package cli

import (
	"context"
	"fmt"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// applyAppAISettings applies deploy-time AI settings from app.yaml to _settings.
// app.yaml is authoritative for these fields when present, matching the llm:
// behavior used by demos and production deployments.
func applyAppAISettings(ctx context.Context, db *storage.DB, appCfg *project.AppConfig) []error {
	if appCfg == nil {
		return nil
	}
	var errs []error
	if appCfg.LLM != nil {
		if err := db.SaveLLMConfig(ctx, *appCfg.LLM); err != nil {
			errs = append(errs, fmt.Errorf("llm: %w", err))
		}
	}
	if appCfg.AI != nil {
		if appCfg.AI.DataScope != "" {
			if err := db.SaveAIDataScope(ctx, appCfg.AI.DataScope); err != nil {
				errs = append(errs, fmt.Errorf("ai.data_scope: %w", err))
			}
		}
		if appCfg.AI.DailyTokenCap != nil {
			if err := db.SaveAIDailyTokenCap(ctx, *appCfg.AI.DailyTokenCap); err != nil {
				errs = append(errs, fmt.Errorf("ai.daily_token_cap: %w", err))
			}
		}
	}
	return errs
}
