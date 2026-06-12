package printform

import "html/template"

// PrintForm describes a declarative print form loaded from YAML.
type PrintForm struct {
	Name     string        `yaml:"name"`
	Document string        `yaml:"document"`
	Title    string        `yaml:"title"`
	Header   string        `yaml:"header"`
	Table    *TableSection `yaml:"table"`
	Footer   string        `yaml:"footer"`

	// External помечает форму, пришедшую из внешнего контура (таблица
	// _ext_printforms), а не из конфигурации проекта. Заполняется
	// программно при загрузке; в YAML не сериализуется. UI показывает у
	// таких форм пометку «(внешняя)».
	External bool `yaml:"-"`
}

type TableSection struct {
	Source  string      `yaml:"source"`
	Columns []Column    `yaml:"columns"`
	Totals  []TotalSpec `yaml:"totals"`
}

type Column struct {
	Field  string `yaml:"field"`
	Label  string `yaml:"label"`
	Width  string `yaml:"width"`
	Align  string `yaml:"align"`
	Format string `yaml:"format"`
}

type TotalSpec struct {
	Field string `yaml:"field"`
	Sum   bool   `yaml:"sum"`
	Label string `yaml:"label"`
}

// RenderContext holds all data needed to render a print form.
type RenderContext struct {
	Document   map[string]any              // fields of the document/catalog record
	TableParts map[string][]map[string]any // table part name → rows
	Constants  map[string]any              // global constants
	Refs       map[string]map[string]any   // field name → expanded reference fields

	// sumCache мемоизирует Итог.<ТЧ>.<Поле> в пределах одного рендера: при
	// использовании Итог внутри repeat-строки наивный sumColumn пересчитывал бы
	// сумму на каждой строке (O(N²)). Лениво инициализируется в sumColumn;
	// контекст создаётся заново на каждый рендер, поэтому глобального состояния нет.
	sumCache map[string]float64
}

// RenderedForm is the final HTML ready to be written to the response.
type RenderedForm = template.HTML
