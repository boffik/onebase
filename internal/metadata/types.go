package metadata

import "strings"

type Kind string

const (
	KindCatalog  Kind = "catalog"
	KindDocument Kind = "document"
)

type FieldType string

const (
	FieldTypeString FieldType = "string"
	FieldTypeDate   FieldType = "date"
	FieldTypeNumber FieldType = "number"
	FieldTypeBool   FieldType = "bool"
)

type Field struct {
	Name string
	// Title — синоним поля по умолчанию (показывается в UI). Пустой Title →
	// в интерфейсе используется Name.
	Title string
	// Titles — переводы синонима по языкам (lang code → перевод).
	Titles    map[string]string
	Type      FieldType
	RefEntity string // non-empty when Type starts with "reference:"
	EnumName  string // non-empty when Type starts with "enum:"
}

// DisplayName возвращает представление поля для интерфейса: Titles[lang] →
// Title → Name. Name всегда остаётся идентификатором (БД, URL, форма).
func (f Field) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := f.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if f.Title != "" {
		return f.Title
	}
	return f.Name
}

type Enum struct {
	Name   string
	Values []string
}

type Constant struct {
	Name      string
	Type      FieldType
	RefEntity string
	EnumName  string
	Default   string
	Label     string
}

type TablePart struct {
	Name   string
	Title  string
	Titles map[string]string
	Fields []Field
}

// DisplayName возвращает представление табличной части для интерфейса.
func (tp TablePart) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := tp.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if tp.Title != "" {
		return tp.Title
	}
	return tp.Name
}

// Numerator describes automatic document numbering.
type Numerator struct {
	Prefix string // e.g. "ПОС-"
	Length int    // digits in numeric part, padded with leading zeros
	Period string // "year" | "month" | "none"
	// Scope — имя поля документа, значение которого включается в ключ
	// нумерации. Например, scope: "Организация" даст отдельные счётчики
	// для каждой организации.
	Scope string
}

// PredefinedItem describes a catalog record that is always present in the DB
// and cannot be deleted. Synced from YAML on every startup.
type PredefinedItem struct {
	Name   string         // identifier used in DSL: ПредопределённыеЗначения.Валюта.Рубль
	Fields map[string]any // initial field values
}

type Entity struct {
	Name string
	// Title — человекочитаемое представление (аналог «Синонима» в 1С).
	// Если пусто, в интерфейсе показывается Name.
	Title string
	// Titles — переводы синонима по языкам (lang code → перевод). Если для
	// активного языка есть запись, используется она; иначе откатываемся на
	// Title и затем на Name. Пустой map допустим.
	Titles     map[string]string
	Kind       Kind
	Fields     []Field
	TableParts []TablePart
	// Posting enables 1C-style posting semantics: movements are written only
	// when the document is explicitly posted, not on every save.
	Posting       bool
	Numerator     *Numerator        // nil if auto-numbering is disabled
	Predefined    []*PredefinedItem // nil for most entities; populated from YAML
	Hierarchical  bool              // catalog with parent_id / is_folder tree support
	HierarchyKind string            // "folders_and_items" (default) | "items_only"
	ListForm      []string          // visible fields in list form (nil = all)
	ItemForm      []string          // visible fields in item form (nil = all)
	Forms         []*FormModule     // form modules (object form, list form, custom forms)
}

type Register struct {
	Name       string
	Dimensions []Field // form the grouping key for balances
	Resources  []Field // accumulated (summed with sign based on movement type)
	Attributes []Field // extra data, stored but not aggregated
}

type InfoRegister struct {
	Name       string
	Periodic   bool    // if true, (period, dim...) is PK; otherwise just (dim...)
	Dimensions []Field // key fields
	Resources  []Field // value fields
}

func RegisterTableName(regName string) string {
	return "рег_" + strings.ToLower(regName)
}

func InfoRegTableName(regName string) string {
	return "инфо_" + strings.ToLower(regName)
}

func TablePartTableName(entityName, tpName string) string {
	return strings.ToLower(entityName) + "_" + strings.ToLower(tpName)
}

// DisplayName возвращает представление объекта для интерфейса с учётом языка:
// сначала пробуется Titles[lang], затем Title (синоним по умолчанию), затем
// Name. Пустой lang пропускает первый шаг — используется как Name всегда
// остаётся идентификатором (URL, DSL).
func (e *Entity) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := e.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if e.Title != "" {
		return e.Title
	}
	return e.Name
}

func IsReference(ft FieldType) bool {
	return strings.HasPrefix(string(ft), "reference:")
}

func RefName(ft FieldType) string {
	return strings.TrimPrefix(string(ft), "reference:")
}

func IsEnum(ft FieldType) bool {
	return strings.HasPrefix(string(ft), "enum:")
}

func EnumTypeName(ft FieldType) string {
	return strings.TrimPrefix(string(ft), "enum:")
}

func TableName(entityName string) string {
	return strings.ToLower(entityName)
}

func ColumnName(f Field) string {
	col := strings.ToLower(f.Name)
	if f.RefEntity != "" {
		return col + "_id"
	}
	return col
}
