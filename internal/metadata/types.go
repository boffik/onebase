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
	Name      string
	Type      FieldType
	RefEntity string // non-empty when Type starts with "reference:"
	EnumName  string // non-empty when Type starts with "enum:"
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
	Fields []Field
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
	Name       string
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
