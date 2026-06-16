// Package compose сворачивает плоские строки отчёта в дерево групп с итогами.
// Чистый слой: без БД/HTTP. Условия оформления вычисляются через Evaluator.
package compose

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/ivantit66/onebase/internal/report"
)

type Row = map[string]any

// Evaluator вычисляет булево DSL-выражение при значениях полей строки.
type Evaluator interface {
	EvalBool(expr string, row Row) (bool, error)
}

const DefaultMaxRows = 50000

type Result struct {
	Columns  []string
	Groups   []*Group
	Grand    map[string]any
	RowCount int
	Capped   bool
}

type Group struct {
	Field     string
	Key       any
	Subtotals map[string]any
	Count     int
	Children  []*Group
	Details   []DetailRow
}

type DetailRow struct {
	Values map[string]any
	Styles map[string]report.CellStyle // ключ "" = стиль на всю строку
}

func Compose(rows []Row, spec report.Composition, ev Evaluator) (*Result, error) {
	return ComposeN(rows, spec, ev, DefaultMaxRows)
}

func ComposeN(rows []Row, spec report.Composition, ev Evaluator, maxRows int) (*Result, error) {
	res := &Result{RowCount: len(rows)}
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
		res.Capped = true
		res.RowCount = maxRows
	}
	res.Groups = buildGroups(rows, spec, 0, ev)
	res.Grand = aggregate(rows, spec.Measures)
	return res, nil
}

func buildGroups(rows []Row, spec report.Composition, level int, ev Evaluator) []*Group {
	if level >= len(spec.Groupings) {
		return nil
	}
	field := spec.Groupings[level]
	var order []any
	buckets := map[any][]Row{}
	for _, r := range rows {
		k := normalizeGroupKey(r[field])
		if _, ok := buckets[k]; !ok {
			order = append(order, k)
		}
		buckets[k] = append(buckets[k], r)
	}
	groups := make([]*Group, 0, len(order))
	for _, k := range order {
		gr := &Group{
			Field:     field,
			Key:       k,
			Count:     len(buckets[k]),
			Subtotals: aggregate(buckets[k], spec.Measures),
		}
		if level+1 < len(spec.Groupings) {
			gr.Children = buildGroups(buckets[k], spec, level+1, ev)
		} else if spec.Detail {
			gr.Details = buildDetails(buckets[k], spec, ev)
		}
		groups = append(groups, gr)
	}
	sortGroups(groups, spec)
	return groups
}

func buildDetails(rows []Row, spec report.Composition, ev Evaluator) []DetailRow {
	out := make([]DetailRow, 0, len(rows))
	for _, r := range rows {
		dr := DetailRow{Values: r}
		if len(spec.Conditional) > 0 && ev != nil {
			styles := map[string]report.CellStyle{}
			for _, rule := range spec.Conditional {
				if _, done := styles[rule.Field]; done {
					continue // первое сработавшее правило на целевое поле
				}
				ok, err := ev.EvalBool(rule.When, r)
				if err != nil || !ok {
					continue
				}
				styles[rule.Field] = rule.Style
			}
			if len(styles) > 0 {
				dr.Styles = styles
			}
		}
		out = append(out, dr)
	}
	sortDetails(out, spec)
	return out
}

func measureSet(spec report.Composition) map[string]bool {
	m := map[string]bool{}
	for _, x := range spec.Measures {
		m[x.Field] = true
	}
	return m
}

func sortGroups(groups []*Group, spec report.Composition) {
	if len(spec.Sort) == 0 {
		return
	}
	meas := measureSet(spec)
	sort.SliceStable(groups, func(i, j int) bool {
		for _, sk := range spec.Sort {
			var vi, vj any
			switch {
			case meas[sk.Field]:
				vi, vj = groups[i].Subtotals[sk.Field], groups[j].Subtotals[sk.Field]
			case sk.Field == groups[i].Field:
				vi, vj = groups[i].Key, groups[j].Key
			default:
				continue
			}
			if c := compareVals(vi, vj); c != 0 {
				if sk.Dir == "desc" {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
}

func sortDetails(rows []DetailRow, spec report.Composition) {
	if len(spec.Sort) == 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		for _, sk := range spec.Sort {
			if c := compareVals(rows[i].Values[sk.Field], rows[j].Values[sk.Field]); c != 0 {
				if sk.Dir == "desc" {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
}

// compareVals: -1/0/1. Числа сравниваются как decimal, иначе как строки.
func compareVals(a, b any) int {
	da, oka := toDecimal(a)
	db, okb := toDecimal(b)
	if oka && okb {
		return da.Cmp(db)
	}
	sa, sb := toStr(a), toStr(b)
	switch {
	case sa < sb:
		return -1
	case sa > sb:
		return 1
	default:
		return 0
	}
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// normalizeGroupKey приводит значение группировки к надёжному ключу map.
// decimal.Decimal внутри держит *big.Int — как ключ map он сравнивался бы по
// указателю, а не по значению, поэтому числа группируем по каноничной строке.
func normalizeGroupKey(v any) any {
	if d, ok := v.(decimal.Decimal); ok {
		return d.String()
	}
	return v
}

func aggregate(rows []Row, measures []report.Measure) map[string]any {
	out := map[string]any{}
	for _, m := range measures {
		out[m.Field] = aggMeasure(rows, m)
	}
	return out
}

func aggMeasure(rows []Row, m report.Measure) any {
	if m.Agg == "count" {
		return int64(len(rows))
	}
	var acc, mn, mx decimal.Decimal
	cnt := 0
	first := true
	for _, r := range rows {
		d, ok := toDecimal(r[m.Field])
		if !ok {
			continue
		}
		cnt++
		acc = acc.Add(d)
		if first {
			mn, mx = d, d
			first = false
		} else {
			if d.LessThan(mn) {
				mn = d
			}
			if d.GreaterThan(mx) {
				mx = d
			}
		}
	}
	switch m.Agg {
	case "min":
		if first {
			return nil
		}
		return mn
	case "max":
		if first {
			return nil
		}
		return mx
	case "avg":
		if cnt == 0 {
			return nil
		}
		return acc.Div(decimal.NewFromInt(int64(cnt)))
	default: // sum / ""
		return acc
	}
}

func toDecimal(v any) (decimal.Decimal, bool) {
	switch x := v.(type) {
	case decimal.Decimal:
		return x, true
	case int:
		return decimal.NewFromInt(int64(x)), true
	case int64:
		return decimal.NewFromInt(x), true
	case float64:
		return decimal.NewFromFloat(x), true
	case string:
		d, err := decimal.NewFromString(x)
		if err != nil {
			return decimal.Zero, false
		}
		return d, true
	}
	return decimal.Zero, false
}
