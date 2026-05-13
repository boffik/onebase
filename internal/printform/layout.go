package printform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LayoutTemplate defines a макет (template) with named areas for print forms.
// Each area contains rows of cells with static text or parameter placeholders.
type LayoutTemplate struct {
	Name     string                  `yaml:"name"`
	Document string                  `yaml:"document"`
	Areas    map[string]*LayoutArea  `yaml:"areas"`
}

// LayoutArea defines a named rectangular area with rows of cells.
type LayoutArea struct {
	Rows []LayoutRow `yaml:"rows"`
}

// LayoutRow is a single row of cells in a layout area.
type LayoutRow struct {
	Cells []LayoutCell `yaml:"cells"`
}

// LayoutCell defines a single cell in a layout template.
// A cell can contain either static text or a named parameter placeholder.
type LayoutCell struct {
	Text      string `yaml:"text,omitempty"`
	Parameter string `yaml:"parameter,omitempty"`
	Bold      bool   `yaml:"bold,omitempty"`
	Italic    bool   `yaml:"italic,omitempty"`
	Align     string `yaml:"align,omitempty"`
	FontSize  int    `yaml:"fontSize,omitempty"`
	BackColor string `yaml:"backColor,omitempty"`
	TextColor string `yaml:"textColor,omitempty"`
	ColSpan   int    `yaml:"colspan,omitempty"`
}

// LoadLayout loads a layout template from a YAML file.
func LoadLayout(path string) (*LayoutTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("layout: read %s: %w", path, err)
	}
	var lt LayoutTemplate
	if err := yaml.Unmarshal(data, &lt); err != nil {
		return nil, fmt.Errorf("layout: parse %s: %w", path, err)
	}
	if lt.Name == "" {
		lt.Name = strings.TrimSuffix(filepath.Base(path), ".layout.yaml")
	}
	return &lt, nil
}

// FindLayoutFile looks for a .layout.yaml file matching the given .os file path.
// For "накладная.os", it looks for "накладная.layout.yaml" in the same directory.
func FindLayoutFile(osPath string) string {
	dir := filepath.Dir(osPath)
	base := strings.TrimSuffix(filepath.Base(osPath), ".os")
	layoutPath := filepath.Join(dir, base+".layout.yaml")
	if _, err := os.Stat(layoutPath); err == nil {
		return layoutPath
	}
	return ""
}

// PreviewHTML returns an HTML preview of the layout template showing all named areas.
func (lt *LayoutTemplate) PreviewHTML() string {
	var sb strings.Builder
	sb.WriteString(`<div style="font-family:Arial,sans-serif;font-size:12px">`)
	for areaName, area := range lt.Areas {
		sb.WriteString(fmt.Sprintf(`<div style="margin-bottom:16px"><div style="font-weight:bold;color:#4a9;margin-bottom:4px">%s</div>`, areaName))
		sb.WriteString(`<table style="border-collapse:collapse;border:1px solid #ccc">`)
		for _, row := range area.Rows {
			sb.WriteString("<tr>")
			for _, cell := range row.Cells {
				style := "border:1px solid #ccc;padding:4px 8px;min-width:40px;"
				if cell.Bold {
					style += "font-weight:bold;"
				}
				if cell.BackColor != "" {
					style += "background-color:" + cell.BackColor + ";"
				}
				if cell.Align != "" {
					style += "text-align:" + cell.Align + ";"
				}
				attrs := ""
				if cell.ColSpan > 1 {
					attrs += fmt.Sprintf(` colspan="%d"`, cell.ColSpan)
				}
				text := cell.Text
				if cell.Parameter != "" {
					text = fmt.Sprintf(`<span style="color:#888">[%s]</span>`, cell.Parameter)
				}
				if text == "" && cell.Parameter == "" {
					text = "&nbsp;"
				}
				sb.WriteString(fmt.Sprintf(`<td style="%s"%s>%s</td>`, style, attrs, text))
			}
			sb.WriteString("</tr>")
		}
		sb.WriteString("</table></div>")
	}
	sb.WriteString("</div>")
	return sb.String()
}
