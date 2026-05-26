package processor

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Param struct {
	Name   string            `yaml:"name"`
	Type   string            `yaml:"type"`   // string, number, date, bool, reference:Entity
	Label  string            `yaml:"label"`  // подпись поля; по умолчанию совпадает с Name
	Labels map[string]string `yaml:"labels"` // переводы подписи по языкам (lang code → перевод)
	// Default — значение по умолчанию, подставляется при первом открытии формы.
	// Для type: bool допустимы true/false (флажок), для остальных — строка/число.
	Default any `yaml:"default"`
}

type Processor struct {
	Name   string            `yaml:"name"`
	Title  string            `yaml:"title"`
	Titles map[string]string `yaml:"titles"`
	Params []Param           `yaml:"params"`
}

// DisplayLabel возвращает подпись параметра с учётом языка.
func (p Param) DisplayLabel(lang string) string {
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

// DisplayName возвращает заголовок обработки с учётом языка.
func (p *Processor) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := p.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if p.Title != "" {
		return p.Title
	}
	return p.Name
}

func LoadFile(path string) (*Processor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var proc Processor
	if err := yaml.Unmarshal(data, &proc); err != nil {
		return nil, err
	}
	return &proc, nil
}

func LoadDir(dir string) ([]*Processor, error) {
	items, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var procs []*Processor
	for _, item := range items {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".yaml") {
			continue
		}
		proc, err := LoadFile(filepath.Join(dir, item.Name()))
		if err != nil {
			return nil, err
		}
		procs = append(procs, proc)
	}
	return procs, nil
}
