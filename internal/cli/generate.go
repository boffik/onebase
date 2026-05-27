package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ivantit66/onebase/internal/gengen"
)

var (
	genPrompt   string
	genOutput   string
	genDomain   string
	genList     bool
	genAddons   []string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new project from a natural-language prompt",
	Long: `Создаёт новый проект OneBase по описанию на естественном языке.

Примеры:
  onebase generate --prompt "оптовые продажи, склад, контрагенты"
  onebase generate --prompt "учёт задач и проектов" --output ./my-tasks
  onebase generate --prompt "тексты и переводы" --domain texts
  onebase generate --list   # показать доступные домены`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().StringVar(&genPrompt, "prompt", "", "описание проекта на естественном языке")
	generateCmd.Flags().StringVar(&genOutput, "output", "", "директория для вывода (по умолчанию = имя домена)")
	generateCmd.Flags().StringVar(&genDomain, "domain", "", "явно указать домен (trade, crm, tasks, ...)")
	generateCmd.Flags().StringSliceVar(&genAddons, "addon", nil, "дополнительные модули (через запятую)")
	generateCmd.Flags().BoolVar(&genList, "list", false, "показать доступные домены и выйти")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	if genList {
		fmt.Fprintln(os.Stdout, "Доступные домены:")
		domains := gengen.AvailableDomains()
		for name, keywords := range domains {
			fmt.Fprintf(os.Stdout, "  %-14s %s\n", name, keywords[0]+", "+keywords[1])
		}
		return nil
	}

	if genPrompt == "" && genDomain == "" {
		return fmt.Errorf("укажите --prompt \"описание\" или --domain <имя>\n\nИспользуйте --list для просмотра доступных доменов")
	}

	// 1. Analyze
	var result *gengen.AnalyzeResult
	if genDomain != "" {
		// Direct domain override
		result = &gengen.AnalyzeResult{
			Domain:    genDomain,
			Confident: true,
		}
	} else {
		result = gengen.Analyze(genPrompt)
	}

	if result.Domain == "unknown" {
		fmt.Fprintf(os.Stderr, "⚠ Домен не определён по промпту.\n\n")
		fmt.Fprintf(os.Stderr, "Попробуйте:\n")
		fmt.Fprintf(os.Stderr, "  1. Указать домен явно: --domain trade\n")
		fmt.Fprintf(os.Stderr, "  2. Использовать другие ключевые слова\n\n")
		fmt.Fprintf(os.Stderr, "Доступные домены:\n")
		for name := range gengen.AvailableDomains() {
			fmt.Fprintf(os.Stderr, "  %s\n", name)
		}
		return fmt.Errorf("domain detection failed")
	}

	if !result.Confident {
		fmt.Fprintf(os.Stderr, "⚠ Амбигуозный результат, возможны варианты: %v\n", result.Ambiguous)
		fmt.Fprintf(os.Stderr, "  Выбран: %s (уточните --domain для явного выбора)\n\n", result.Domain)
	}

	// 2. Resolve template
	if result.Template == "" {
		// Try to find template by domain name
		candidates := []string{
			"examples/" + result.Domain,
			"templates/" + result.Domain,
		}
		for _, t := range candidates {
			if dirExists(t) {
				result.Template = t
				break
			}
		}
	}

	if result.Template == "" {
		return fmt.Errorf("нет шаблона для домена %q (искали: examples/%s, templates/%s)",
			result.Domain, result.Domain, result.Domain)
	}

	// 3. Determine output directory
	outDir := genOutput
	if outDir == "" {
		outDir = gengen.SanitizePrompt(genPrompt)
		if outDir == "" {
			outDir = result.Domain
		}
	}

	// 4. Generate
	gen := &gengen.Generator{OutputDir: outDir}
	if err := gen.Generate(result.Template, genAddons); err != nil {
		return err
	}

	// 5. Success
	fmt.Fprintf(os.Stdout, "✓ Проект сгенерирован в %s\n", outDir)
	fmt.Fprintf(os.Stdout, "  Домен: %s\n", result.Domain)
	fmt.Fprintf(os.Stdout, "  Шаблон: %s\n", result.Template)
	if len(genAddons) > 0 {
		fmt.Fprintf(os.Stdout, "  Аддоны: %v\n", genAddons)
	}
	fmt.Fprintf(os.Stdout, "\nЗапуск:\n")
	absPath, _ := filepath.Abs(outDir)
	fmt.Fprintf(os.Stdout, "  onebase run --project %s --sqlite %s.db\n", absPath, result.Domain)

	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
