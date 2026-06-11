package extform

import (
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/report"
	"gopkg.in/yaml.v3"
)

// ParsedReport — результат разбора загруженного отчёта, готовый к сохранению.
type ParsedReport struct {
	Name        string
	Content     []byte // «голый» YAML report.Report (без обёртки manifest/form)
	Author      string
	Version     string
	MinPlatform string
}

// ParseReportUpload разбирает загруженный отчёт: «голый» YAML report.Report
// либо бандл *.obform с секциями manifest/form (kind: report).
func ParseReportUpload(data []byte) (*ParsedReport, error) {
	var bd struct {
		Manifest *Manifest `yaml:"manifest"`
		Form     yaml.Node `yaml:"form"`
	}
	_ = yaml.Unmarshal(data, &bd)

	p := &ParsedReport{}
	hasForm := bd.Form.Kind != 0
	if bd.Manifest != nil || hasForm {
		if !hasForm {
			return nil, fmt.Errorf("бандл без секции form")
		}
		node := &bd.Form
		if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
			node = node.Content[0]
		}
		content, err := yaml.Marshal(node)
		if err != nil {
			return nil, fmt.Errorf("сериализация form: %w", err)
		}
		p.Content = content
		if bd.Manifest != nil {
			p.Name = bd.Manifest.Name
			p.Author = bd.Manifest.Author
			p.Version = bd.Manifest.Version
			p.MinPlatform = bd.Manifest.MinPlatform
		}
	} else {
		p.Content = data
	}

	rep, err := report.ParseBytes(p.Content)
	if err != nil {
		return nil, fmt.Errorf("некорректный YAML отчёта: %w", err)
	}
	if rep.Name != "" {
		p.Name = rep.Name
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("не указано имя отчёта (поле name)")
	}
	if strings.TrimSpace(rep.Query) == "" {
		return nil, fmt.Errorf("у отчёта пустой запрос (поле query)")
	}
	return p, nil
}

// BuildReportBundle собирает переносимый бандл *.obform из записи отчёта.
func BuildReportBundle(rec *ReportRecord, platformVersion string) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(rec.Content, &node); err != nil {
		return nil, fmt.Errorf("разбор содержимого отчёта: %w", err)
	}
	n := &node
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	out := struct {
		Manifest Manifest   `yaml:"manifest"`
		Form     *yaml.Node `yaml:"form"`
	}{
		Manifest: Manifest{
			Kind:        "report",
			Name:        rec.Name,
			Author:      rec.Author,
			Version:     rec.Version,
			MinPlatform: platformVersion,
		},
		Form: n,
	}
	return yaml.Marshal(out)
}
