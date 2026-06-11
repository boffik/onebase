package gengen

import (
	"os"
	"sort"
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
			"crm", "клиент", "сделк", "воронк", "лид", "звонк",
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
//  2. For each domain, count keyword matches (substring search). A keyword
//     mentioned with negation («без продаж») is not counted.
//  3. Pick the domain with the highest count. Ties are broken
//     deterministically in favour of the more specific domain: higher share
//     of its keyword set matched, then longer matched keywords, then name.
//  4. On a tie by count, set Confident=false and list ambiguous domains.
//  5. Resolve the template path (first existing dir from Templates list).
func Analyze(prompt string) *AnalyzeResult {
	prompt = strings.ToLower(prompt)

	type score struct {
		domain  string
		count   int
		ratio   float64 // count / len(keywords) — мера специфичности домена
		matched int     // суммарная длина совпавших ключей в рунах
	}
	var scores []score

	for name, rule := range domainRules {
		count, matched := 0, 0
		for _, kw := range rule.Keywords {
			if !strings.Contains(prompt, kw) {
				continue
			}
			// «складской учёт без продаж»: упоминание с отрицанием — это
			// исключение возможности, а не запрос на неё.
			if strings.Contains(prompt, "без "+kw) {
				continue
			}
			count++
			matched += len([]rune(kw))
		}
		if count > 0 {
			scores = append(scores, score{name, count, float64(count) / float64(len(rule.Keywords)), matched})
		}
	}

	if len(scores) == 0 {
		return &AnalyzeResult{Domain: "unknown"}
	}

	sort.Slice(scores, func(i, j int) bool {
		a, b := scores[i], scores[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if a.ratio != b.ratio {
			return a.ratio > b.ratio
		}
		if a.matched != b.matched {
			return a.matched > b.matched
		}
		return a.domain < b.domain
	})

	best := scores[0]
	var ties []string
	for _, s := range scores[1:] {
		if s.count == best.count {
			ties = append(ties, s.domain)
		}
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
