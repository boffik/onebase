package report

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Param struct {
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`    // string, date, number, bool, select, reference:Entity
	Label   string            `yaml:"label"`   // display label; falls back to Name
	Labels  map[string]string `yaml:"labels"`  // per-language labels (lang code → translation)
	Options []string          `yaml:"options"` // for type: select
}

type Report struct {
	Name      string            `yaml:"name"`
	Title     string            `yaml:"title"`
	Titles    map[string]string `yaml:"titles"`
	Params    []Param           `yaml:"params"`
	Query     string            `yaml:"query"`
	ChartProc string            `yaml:"chart_proc"`
}

// DisplayLabel возвращает подпись параметра с учётом языка.
func (p *Param) DisplayLabel(lang string) string {
	if lang != "" {
		if v, ok := p.Labels[lang]; ok && v != "" {
			return v
		}
	}
	if p.Label != "" {
		return p.Label
	}
	return p.Name
}

// DisplayName возвращает заголовок отчёта с учётом языка.
func (r *Report) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := r.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

func LoadFile(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
