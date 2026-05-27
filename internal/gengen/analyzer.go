package gengen

import (
	"os"
	"strings"
)

// domainRules maps domain names to their keyword sets and template paths.
// Keywords are stored in lowercase; matching is case-insensitive.
var domainRules = map[string]DomainRule{
	"trade": {
		Keywords: []string{
			"продаж", "опт", "розниц", "клиент", "контрагент",
			"отгрузк", "реализаци", "счет-фактур", "заказ", "склад", "остатк",
			"товар", "номенклатур", "поставщик", "закупк",
		},
		Templates: []string{"examples/trade", "templates/trade"},
	},
	"warehouse": {
		Keywords: []string{
			"склад", "остатк", "товар", "ячейк", "адресн",
			"инвентаризаци", "перемещен", "приход", "расход",
		},
		Templates: []string{"templates/warehouse"},
	},
	"crm": {
		Keywords: []string{
			"клиент", "сделк", "воронк", "лид", "звонк",
			"менеджер", "контакт", "коммерческ",
		},
		Templates: []string{"templates/crm"},
	},
	"finance": {
		Keywords: []string{
			"финанс", "бюджет", "счет", "доход", "расход",
			"категор", "долг", "прибыл", "убыток",
		},
		Templates: []string{"templates/finance"},
	},
	"tasks": {
		Keywords: []string{
			"задач", "проект", "исполнител", "дедлайн",
			"приоритет", "статус",
		},
		Templates: []string{"templates/tasks"},
	},
	"accounting": {
		Keywords: []string{
			"бухгалтер", "план счетов", "проводк", "дебет",
			"кредит", "оборотн", "сальдо", "основн средств", "амортизаци",
		},
		Templates: []string{"examples/accounting", "templates/accounting"},
	},
	"texts": {
		Keywords: []string{
			"текст", "перевод", "язык", "событие",
			"перевод текст", "текст и перевод",
		},
		Templates: []string{"templates/texts"},
	},
}

// Analyze parses a natural-language prompt and returns an AnalyzeResult.
// It uses keyword matching against known domain rules.
//
// Algorithm:
//  1. Lowercase the prompt.
//  2. For each domain, count keyword matches (substring search).
//  3. Pick the domain with the highest count.
//  4. If tied, set Confident=false and list ambiguous domains.
//  5. Resolve the template path (first existing dir from Templates list).
func Analyze(prompt string) *AnalyzeResult {
	prompt = strings.ToLower(prompt)

	type score struct {
		domain string
		count  int
	}
	var best score
	var ties []string

	for name, rule := range domainRules {
		count := 0
		for _, kw := range rule.Keywords {
			if strings.Contains(prompt, kw) {
				count++
			}
		}
		if count > best.count {
			best.count = count
			best.domain = name
			ties = nil
		} else if count == best.count && count > 0 {
			ties = append(ties, name)
		}
	}

	if best.count == 0 {
		return &AnalyzeResult{Domain: "unknown"}
	}

	result := &AnalyzeResult{
		Domain:    best.domain,
		Confident: len(ties) == 0,
		Ambiguous: ties,
	}

	// Resolve template path
	rule := domainRules[best.domain]
	for _, t := range rule.Templates {
		if dirExists(t) {
			result.Template = t
			break
		}
	}

	return result
}

// AvailableDomains returns all known domain names and their keyword counts.
func AvailableDomains() map[string][]string {
	out := make(map[string][]string, len(domainRules))
	for name, rule := range domainRules {
		out[name] = append([]string(nil), rule.Keywords...)
	}
	return out
}

// dirExists checks if a directory exists on the filesystem.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
