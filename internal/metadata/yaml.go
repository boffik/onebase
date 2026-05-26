package metadata

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type rawField struct {
	Name   string            `yaml:"name"`
	Title  string            `yaml:"title"`
	Titles map[string]string `yaml:"titles"`
	Type   string            `yaml:"type"`
}

type rawTablePart struct {
	Name   string            `yaml:"name"`
	Title  string            `yaml:"title"`
	Titles map[string]string `yaml:"titles"`
	Fields []rawField        `yaml:"fields"`
}

type rawNumerator struct {
	Prefix string `yaml:"prefix"`
	Length int    `yaml:"length"`
	Period string `yaml:"period"`
	Scope  string `yaml:"scope"`
}

type rawPredefined struct {
	Name   string                 `yaml:"name"`
	Fields map[string]interface{} `yaml:"fields"`
}

type rawEntity struct {
	Name          string            `yaml:"name"`
	Title         string            `yaml:"title"`
	Titles        map[string]string `yaml:"titles"`
	Fields        []rawField        `yaml:"fields"`
	TableParts    []rawTablePart  `yaml:"tableparts"`
	Posting       bool            `yaml:"posting"`
	Numerator     *rawNumerator   `yaml:"numerator"`
	Predefined    []rawPredefined `yaml:"predefined"`
	Hierarchical  bool            `yaml:"hierarchical"`
	HierarchyKind string          `yaml:"hierarchy_kind"`
	ListForm      []string        `yaml:"list_form"`
	ItemForm      []string        `yaml:"item_form"`
}

func LoadFile(path string, kind Kind) (*Entity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawEntity
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: missing name", path)
	}
	e := &Entity{Name: raw.Name, Title: raw.Title, Titles: raw.Titles, Kind: kind, Posting: raw.Posting, Hierarchical: raw.Hierarchical}
	if raw.Hierarchical {
		e.HierarchyKind = raw.HierarchyKind
		if e.HierarchyKind == "" {
			e.HierarchyKind = "folders_and_items"
		}
	}
	e.ListForm = raw.ListForm
	e.ItemForm = raw.ItemForm
	if raw.Numerator != nil {
		n := &Numerator{
			Prefix: raw.Numerator.Prefix,
			Length: raw.Numerator.Length,
			Period: raw.Numerator.Period,
			Scope:  raw.Numerator.Scope,
		}
		if n.Length <= 0 {
			n.Length = 8
		}
		if n.Period == "" {
			n.Period = "year"
		}
		e.Numerator = n
	}
	for _, rf := range raw.Fields {
		e.Fields = append(e.Fields, parseField(rf))
	}
	for _, rtp := range raw.TableParts {
		tp := TablePart{Name: rtp.Name, Title: rtp.Title, Titles: rtp.Titles}
		for _, rf := range rtp.Fields {
			tp.Fields = append(tp.Fields, parseField(rf))
		}
		e.TableParts = append(e.TableParts, tp)
	}
	for _, rp := range raw.Predefined {
		fields := make(map[string]any, len(rp.Fields))
		for k, v := range rp.Fields {
			fields[k] = v
		}
		e.Predefined = append(e.Predefined, &PredefinedItem{Name: rp.Name, Fields: fields})
	}
	return e, nil
}

type rawRegister struct {
	Name       string     `yaml:"name"`
	Dimensions []rawField `yaml:"dimensions"`
	Resources  []rawField `yaml:"resources"`
	Attributes []rawField `yaml:"attributes"`
}

func LoadRegisterFile(path string) (*Register, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawRegister
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: missing name", path)
	}
	reg := &Register{Name: raw.Name}
	for _, rf := range raw.Dimensions {
		reg.Dimensions = append(reg.Dimensions, parseField(rf))
	}
	for _, rf := range raw.Resources {
		reg.Resources = append(reg.Resources, parseField(rf))
	}
	for _, rf := range raw.Attributes {
		reg.Attributes = append(reg.Attributes, parseField(rf))
	}
	return reg, nil
}

type rawInfoRegister struct {
	Name       string     `yaml:"name"`
	Periodic   bool       `yaml:"periodic"`
	Dimensions []rawField `yaml:"dimensions"`
	Resources  []rawField `yaml:"resources"`
}

func LoadInfoRegisterFile(path string) (*InfoRegister, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawInfoRegister
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: missing name", path)
	}
	ir := &InfoRegister{Name: raw.Name, Periodic: raw.Periodic}
	for _, rf := range raw.Dimensions {
		ir.Dimensions = append(ir.Dimensions, parseField(rf))
	}
	for _, rf := range raw.Resources {
		ir.Resources = append(ir.Resources, parseField(rf))
	}
	return ir, nil
}

type rawEnum struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

func LoadEnumFile(path string) (*Enum, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawEnum
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: missing name", path)
	}
	return &Enum{Name: raw.Name, Values: raw.Values}, nil
}

type rawConstant struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Default string `yaml:"default"`
	Label   string `yaml:"label"`
}

type rawConstantsFile struct {
	Constants []rawConstant `yaml:"constants"`
}

func LoadConstantsFile(path string) ([]*Constant, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawConstantsFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var result []*Constant
	for _, rc := range raw.Constants {
		c := &Constant{
			Name:    rc.Name,
			Type:    FieldType(rc.Type),
			Default: rc.Default,
			Label:   rc.Label,
		}
		if strings.HasPrefix(rc.Type, "reference:") {
			c.RefEntity = strings.TrimPrefix(rc.Type, "reference:")
		} else if strings.HasPrefix(rc.Type, "enum:") {
			c.EnumName = strings.TrimPrefix(rc.Type, "enum:")
		}
		result = append(result, c)
	}
	return result, nil
}

func parseField(rf rawField) Field {
	f := Field{Name: rf.Name, Title: rf.Title, Titles: rf.Titles, Type: FieldType(rf.Type)}
	if strings.HasPrefix(rf.Type, "reference:") {
		f.RefEntity = strings.TrimPrefix(rf.Type, "reference:")
	} else if strings.HasPrefix(rf.Type, "enum:") {
		f.EnumName = strings.TrimPrefix(rf.Type, "enum:")
	}
	return f
}
