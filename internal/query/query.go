package query

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/ivantit66/onebase/internal/metadata"
)

// CompileOpts holds options for query compilation including register metadata
// needed to resolve virtual table references (Остатки, Обороты, СрезПоследних, …).
type CompileOpts struct {
	Params      map[string]any
	Registers   []*metadata.Register
	InfoRegs    []*metadata.InfoRegister
	AccountRegs []*metadata.AccountRegister
}

// Result holds compiled PostgreSQL SQL and positional arguments.
type Result struct {
	SQL  string
	Args []any
}

// Compile translates a 1C-style query to PostgreSQL SQL.
func Compile(src string, opts CompileOpts) (Result, error) {
	return translate(tokenize(src), opts)
}

// --- tokenizer ---

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tDot
	tComma
	tLParen
	tRParen
	tParam
	tStr
	tNum
	tOp
	tStar
)

type tok struct {
	kind tokKind
	val  string
}

func tokenize(src string) []tok {
	var out []tok
	runes := []rune(src)
	n := len(runes)
	i := 0
	for i < n {
		ch := runes[i]
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		switch {
		case ch == '&':
			i++
			j := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			out = append(out, tok{tParam, string(runes[j:i])})
		case ch == '"':
			i++
			j := i
			for i < n && runes[i] != '"' {
				i++
			}
			out = append(out, tok{tStr, string(runes[j:i])})
			if i < n {
				i++
			}
		case ch == '\'':
			i++
			j := i
			for i < n && runes[i] != '\'' {
				i++
			}
			out = append(out, tok{tStr, string(runes[j:i])})
			if i < n {
				i++
			}
		case ch == '.':
			out = append(out, tok{tDot, "."})
			i++
		case ch == ',':
			out = append(out, tok{tComma, ","})
			i++
		case ch == '(':
			out = append(out, tok{tLParen, "("})
			i++
		case ch == ')':
			out = append(out, tok{tRParen, ")"})
			i++
		case ch == '*':
			out = append(out, tok{tStar, "*"})
			i++
		case ch == '<':
			if i+1 < n && runes[i+1] == '>' {
				out = append(out, tok{tOp, "<>"})
				i += 2
			} else if i+1 < n && runes[i+1] == '=' {
				out = append(out, tok{tOp, "<="})
				i += 2
			} else {
				out = append(out, tok{tOp, "<"})
				i++
			}
		case ch == '>':
			if i+1 < n && runes[i+1] == '=' {
				out = append(out, tok{tOp, ">="})
				i += 2
			} else {
				out = append(out, tok{tOp, ">"})
				i++
			}
		case ch == '!' && i+1 < n && runes[i+1] == '=':
			out = append(out, tok{tOp, "<>"})
			i += 2
		case ch == '=' || ch == '+' || ch == '-' || ch == '/':
			out = append(out, tok{tOp, string(ch)})
			i++
		case unicode.IsLetter(ch) || ch == '_':
			j := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			out = append(out, tok{tIdent, string(runes[j:i])})
		case unicode.IsDigit(ch):
			j := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			out = append(out, tok{tNum, string(runes[j:i])})
		default:
			i++
		}
	}
	out = append(out, tok{tEOF, ""})
	return out
}

// --- source type mapping ---

var sourcePrefix = map[string]string{
	"РЕГИСТРНАКОПЛЕНИЯ":    "рег_",
	"ACCUMULATIONREGISTER": "рег_",
	"РЕГИСТРСВЕДЕНИЙ":      "инфо_",
	"INFORMATIONREGISTER":  "инфо_",
	"РЕГИСТРБУХГАЛТЕРИИ":   "акк_",
	"ACCOUNTINGREGISTER":   "акк_",
	"СПРАВОЧНИК":           "",
	"CATALOG":              "",
	"ДОКУМЕНТ":             "",
	"DOCUMENT":             "",
}

func isSourceType(upper string) bool {
	_, ok := sourcePrefix[upper]
	return ok
}

func isAccumRegType(upper string) bool {
	return upper == "РЕГИСТРНАКОПЛЕНИЯ" || upper == "ACCUMULATIONREGISTER"
}

func isInfoRegType(upper string) bool {
	return upper == "РЕГИСТРСВЕДЕНИЙ" || upper == "INFORMATIONREGISTER"
}

func isAccountRegType(upper string) bool {
	return upper == "РЕГИСТРБУХГАЛТЕРИИ" || upper == "ACCOUNTINGREGISTER"
}

func sourceToTable(typeUpper, entityName string) string {
	return sourcePrefix[typeUpper] + strings.ToLower(entityName)
}

// --- virtual table kind maps ---

var accumVTKinds = map[string]string{
	"ОСТАТКИ":               "balances",
	"BALANCES":              "balances",
	"ОБОРОТЫ":               "turnovers",
	"TURNOVERS":             "turnovers",
	"ОСТАТКИИОБОРОТЫ":       "balances_turnovers",
	"BALANCESANDTURNOVERS":  "balances_turnovers",
}

var infoVTKinds = map[string]string{
	"СРЕЗПОСЛЕДНИХ": "last_slice",
	"LASTSLICE":     "last_slice",
	"СРЕЗПЕРВЫХ":    "first_slice",
	"FIRSTSLICE":    "first_slice",
}

// --- keyword mapping ---

var kwMap = map[string]string{
	// Russian structural keywords
	"ВЫБРАТЬ":       "SELECT",
	"РАЗЛИЧНЫЕ":     "DISTINCT",
	"ИЗ":            "FROM",
	"ГДЕ":           "WHERE",
	"СГРУППИРОВАТЬ": "GROUP",
	"УПОРЯДОЧИТЬ":   "ORDER",
	"ПО":            "ON", // standalone ПО without СГРУППИРОВАТЬ/УПОРЯДОЧИТЬ is always JOIN ON
	"ИМЕЯ":          "HAVING",
	"КАК":           "AS",
	"И":             "AND",
	"ИЛИ":           "OR",
	"НЕ":            "NOT",
	"ВЫБОР":         "CASE",
	"КОГДА":         "WHEN",
	"ТОГДА":         "THEN",
	"ИНАЧЕ":         "ELSE",
	"КОНЕЦ":         "END",
	"УБЫВ":          "DESC",
	"ВОЗР":          "ASC",
	"ЕСТЬ":          "IS",
	"ПУСТО":         "NULL",
	"В":             "IN",
	"ОБЪЕДИНИТЬ":    "UNION",
	"ВСЕ":           "ALL",
	// JOIN keywords (Russian)
	"ВНУТРЕННЕЕ": "INNER",
	"ЛЕВОЕ":      "LEFT",
	"ПРАВОЕ":     "RIGHT",
	"ПОЛНОЕ":     "FULL",
	"СОЕДИНЕНИЕ": "JOIN",
	// English pass-through
	"SELECT":   "SELECT",
	"DISTINCT": "DISTINCT",
	"FROM":     "FROM",
	"WHERE":    "WHERE",
	"GROUP":    "GROUP",
	"ORDER":    "ORDER",
	"BY":       "BY",
	"ON":       "ON",
	"HAVING":   "HAVING",
	"AS":       "AS",
	"AND":      "AND",
	"OR":       "OR",
	"NOT":      "NOT",
	"CASE":     "CASE",
	"WHEN":     "WHEN",
	"THEN":     "THEN",
	"ELSE":     "ELSE",
	"END":      "END",
	"DESC":     "DESC",
	"ASC":      "ASC",
	"IS":       "IS",
	"NULL":     "NULL",
	"IN":       "IN",
	"UNION":    "UNION",
	"ALL":      "ALL",
	// JOIN keywords (English pass-through)
	"INNER": "INNER",
	"LEFT":  "LEFT",
	"RIGHT": "RIGHT",
	"FULL":  "FULL",
	"OUTER": "OUTER",
	"JOIN":  "JOIN",
	"CROSS": "CROSS",
}

var aggFuncs = map[string]string{
	"СУММА":      "SUM",
	"КОЛИЧЕСТВО": "COUNT",
	"МИНИМУМ":    "MIN",
	"МАКСИМУМ":   "MAX",
	"СРЕДНЕЕ":    "AVG",
	"SUM":        "SUM",
	"COUNT":      "COUNT",
	"MIN":        "MIN",
	"MAX":        "MAX",
	"AVG":        "AVG",
}

func sqlKW(ident string) (string, bool) {
	kw, ok := kwMap[strings.ToUpper(ident)]
	return kw, ok
}

func sqlAgg(ident string) (string, bool) {
	kw, ok := aggFuncs[strings.ToUpper(ident)]
	return kw, ok
}

// --- translator ---

type translator struct {
	tokens      []tok
	pos         int
	args        []any
	params      map[string]int // param name → 1-based index in args (0 = NULL sentinel)
	paramValues map[string]any
	opts        CompileOpts
	parts       []string
	prevWasDot  bool // true after emitting "." — used to resolve .Ссылка → .id
}

func (tr *translator) peek(offset int) tok {
	i := tr.pos + offset
	if i >= len(tr.tokens) {
		return tok{tEOF, ""}
	}
	return tr.tokens[i]
}

func (tr *translator) advance() tok {
	t := tr.tokens[tr.pos]
	tr.pos++
	return t
}

func (tr *translator) emit(s string) {
	tr.parts = append(tr.parts, s)
}

func (tr *translator) build() string {
	var sb strings.Builder
	for i, p := range tr.parts {
		if i > 0 {
			prev := tr.parts[i-1]
			noBefore := p == "," || p == ")" || p == "." || p == "("
			noAfter := prev == "(" || prev == "."
			if !noBefore && !noAfter {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(p)
	}
	return sb.String()
}

// addParam registers a named parameter and returns its SQL placeholder.
func (tr *translator) addParam(name string) string {
	if _, exists := tr.params[name]; !exists {
		v := tr.paramValues[name]
		if v == nil {
			tr.params[name] = 0
		} else {
			tr.args = append(tr.args, v)
			tr.params[name] = len(tr.args)
		}
	}
	if tr.params[name] == 0 {
		return "NULL"
	}
	return fmt.Sprintf("$%d%s", tr.params[name], pgCast(tr.paramValues[name]))
}

// parseVTArgs collects argument groups from a virtual-table call.
// The opening "(" has already been consumed; this method consumes until the matching ")".
func (tr *translator) parseVTArgs() [][]tok {
	var groups [][]tok
	var current []tok
	depth := 0
	for {
		t := tr.advance()
		if t.kind == tEOF {
			break
		}
		switch {
		case t.kind == tLParen:
			depth++
			current = append(current, t)
		case t.kind == tRParen && depth > 0:
			depth--
			current = append(current, t)
		case t.kind == tRParen: // depth == 0, closing paren
			groups = append(groups, current)
			return groups
		case t.kind == tComma && depth == 0:
			groups = append(groups, current)
			current = nil
		default:
			current = append(current, t)
		}
	}
	return groups
}

// translateFilterTokens translates a token slice to a SQL expression fragment,
// resolving &params through the translator's shared state.
func (tr *translator) translateFilterTokens(tokens []tok) string {
	var parts []string
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.kind {
		case tParam:
			parts = append(parts, tr.addParam(t.val))
		case tIdent:
			upper := strings.ToUpper(t.val)
			if kw, ok := kwMap[upper]; ok {
				parts = append(parts, kw)
			} else if agg, ok := aggFuncs[upper]; ok && i+1 < len(tokens) && tokens[i+1].kind == tLParen {
				parts = append(parts, agg)
			} else {
				parts = append(parts, strings.ToLower(t.val))
			}
		case tStr:
			parts = append(parts, "'"+strings.ReplaceAll(t.val, "'", "''")+"'")
		case tNum, tOp, tStar:
			parts = append(parts, t.val)
		case tComma:
			parts = append(parts, ",")
		case tLParen:
			parts = append(parts, "(")
		case tRParen:
			parts = append(parts, ")")
		case tDot:
			parts = append(parts, ".")
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (tr *translator) findRegister(name string) *metadata.Register {
	nl := strings.ToLower(name)
	for _, r := range tr.opts.Registers {
		if strings.ToLower(r.Name) == nl {
			return r
		}
	}
	return nil
}

func (tr *translator) findInfoRegister(name string) *metadata.InfoRegister {
	nl := strings.ToLower(name)
	for _, r := range tr.opts.InfoRegs {
		if strings.ToLower(r.Name) == nl {
			return r
		}
	}
	return nil
}

func dimCols(dims []metadata.Field) []string {
	names := make([]string, len(dims))
	for i, d := range dims {
		names[i] = strings.ToLower(d.Name)
	}
	return names
}

// buildAccumVT generates a SQL subquery for an accumulation register virtual table.
func (tr *translator) buildAccumVT(vtKind, regName string, args [][]tok) (subq, alias string, err error) {
	reg := tr.findRegister(regName)
	if reg == nil {
		return "", "", fmt.Errorf("accumulation register %q not found; pass Registers in CompileOpts", regName)
	}
	switch vtKind {
	case "balances":
		return tr.genBalances(reg, args)
	case "turnovers":
		return tr.genTurnovers(reg, args)
	case "balances_turnovers":
		return tr.genBalancesAndTurnovers(reg, args)
	}
	return "", "", fmt.Errorf("unknown accumulation virtual table: %s", vtKind)
}

// buildInfoVT generates a SQL subquery for an information register virtual table.
func (tr *translator) buildInfoVT(vtKind, regName string, args [][]tok) (subq, alias string, err error) {
	ir := tr.findInfoRegister(regName)
	if ir == nil {
		return "", "", fmt.Errorf("information register %q not found; pass InfoRegs in CompileOpts", regName)
	}
	switch vtKind {
	case "last_slice":
		return tr.genLastSlice(ir, args)
	case "first_slice":
		return tr.genFirstSlice(ir, args)
	}
	return "", "", fmt.Errorf("unknown information virtual table: %s", vtKind)
}

func (tr *translator) findAccountRegister(name string) *metadata.AccountRegister {
	nl := strings.ToLower(name)
	for _, r := range tr.opts.AccountRegs {
		if strings.ToLower(r.Name) == nl {
			return r
		}
	}
	return nil
}

// buildAccountVT generates a SQL subquery for an accounting register virtual table.
func (tr *translator) buildAccountVT(vtKind, regName string, args [][]tok) (subq, alias string, err error) {
	ar := tr.findAccountRegister(regName)
	if ar == nil {
		return "", "", fmt.Errorf("accounting register %q not found; pass AccountRegs in CompileOpts", regName)
	}
	switch vtKind {
	case "balances":
		return tr.genAccountBalances(ar, args)
	case "turnovers":
		return tr.genAccountTurnovers(ar, args)
	}
	return "", "", fmt.Errorf("unknown accounting virtual table: %s", vtKind)
}

func (tr *translator) genAccountBalances(ar *metadata.AccountRegister, args [][]tok) (string, string, error) {
	table := metadata.AccountRegTableName(ar.Name)
	alias := "остатки_" + strings.ToLower(ar.Name)

	var resCols []string
	for _, r := range ar.Resources {
		col := strings.ToLower(r.Name)
		resCols = append(resCols,
			"COALESCE(SUM(CASE WHEN r.счётдт = a.code THEN r."+col+" ELSE 0 END),0) AS "+col+"_дт",
			"COALESCE(SUM(CASE WHEN r.счёткт = a.code THEN r."+col+" ELSE 0 END),0) AS "+col+"_кт",
			"COALESCE(SUM(CASE WHEN r.счётдт = a.code THEN r."+col+" ELSE -r."+col+" END),0) AS "+col+"остаток",
		)
	}

	selectList := "a.code AS счёт, a.name AS наименование"
	if len(resCols) > 0 {
		selectList += ", " + strings.Join(resCols, ", ")
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(selectList)
	sb.WriteString(" FROM _accounts a LEFT JOIN ")
	sb.WriteString(table)
	sb.WriteString(" r ON (r.счётдт = a.code OR r.счёткт = a.code)")

	var conds []string
	if len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "r.period <= "+s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" AND ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if len(args) > 1 && len(args[1]) > 0 {
		if s := tr.translateFilterTokens(args[1]); s != "" {
			sb.WriteString(" WHERE (")
			sb.WriteString(s)
			sb.WriteString(")")
		}
	}
	sb.WriteString(" GROUP BY a.code, a.name")

	return sb.String(), alias, nil
}

func (tr *translator) genAccountTurnovers(ar *metadata.AccountRegister, args [][]tok) (string, string, error) {
	table := metadata.AccountRegTableName(ar.Name)
	alias := "обороты_" + strings.ToLower(ar.Name)

	var resCols []string
	for _, r := range ar.Resources {
		col := strings.ToLower(r.Name)
		resCols = append(resCols,
			"COALESCE(SUM(CASE WHEN r.счётдт = a.code THEN r."+col+" ELSE 0 END),0) AS "+col+"_дт",
			"COALESCE(SUM(CASE WHEN r.счёткт = a.code THEN r."+col+" ELSE 0 END),0) AS "+col+"_кт",
		)
	}

	selectList := "a.code AS счёт, a.name AS наименование"
	if len(resCols) > 0 {
		selectList += ", " + strings.Join(resCols, ", ")
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(selectList)
	sb.WriteString(" FROM _accounts a LEFT JOIN ")
	sb.WriteString(table)
	sb.WriteString(" r ON (r.счётдт = a.code OR r.счёткт = a.code)")

	var conds []string
	if len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "r.period >= "+s)
		}
	}
	if len(args) > 1 && len(args[1]) > 0 {
		if s := tr.translateFilterTokens(args[1]); s != "" && s != "NULL" {
			conds = append(conds, "r.period <= "+s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" AND ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	sb.WriteString(" GROUP BY a.code, a.name HAVING SUM(CASE WHEN r.id IS NOT NULL THEN 1 ELSE 0 END) > 0")

	return sb.String(), alias, nil
}

func (tr *translator) genBalances(reg *metadata.Register, args [][]tok) (string, string, error) {
	tableName := metadata.RegisterTableName(reg.Name)
	alias := "остатки_" + strings.ToLower(reg.Name)
	dims := dimCols(reg.Dimensions)

	var cols []string
	cols = append(cols, dims...)
	for _, r := range reg.Resources {
		col := strings.ToLower(r.Name)
		cols = append(cols,
			"SUM(CASE WHEN вид_движения = 'Приход' THEN "+col+" ELSE -"+col+" END) AS "+col+"остаток")
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(cols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(tableName)

	var conds []string
	if len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "period <= "+s)
		}
	}
	if len(args) > 1 && len(args[1]) > 0 {
		if s := tr.translateFilterTokens(args[1]); s != "" {
			conds = append(conds, s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if len(dims) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(dims, ", "))
	}

	return sb.String(), alias, nil
}

func (tr *translator) genTurnovers(reg *metadata.Register, args [][]tok) (string, string, error) {
	tableName := metadata.RegisterTableName(reg.Name)
	alias := "обороты_" + strings.ToLower(reg.Name)
	dims := dimCols(reg.Dimensions)

	var cols []string
	cols = append(cols, dims...)
	for _, r := range reg.Resources {
		col := strings.ToLower(r.Name)
		cols = append(cols,
			"SUM(CASE WHEN вид_движения = 'Приход' THEN "+col+" ELSE 0 END) AS "+col+"приход",
			"SUM(CASE WHEN вид_движения = 'Расход' THEN "+col+" ELSE 0 END) AS "+col+"расход",
			"SUM(CASE WHEN вид_движения = 'Приход' THEN "+col+" ELSE -"+col+" END) AS "+col+"оборот",
		)
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(cols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(tableName)

	var conds []string
	if len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "period >= "+s)
		}
	}
	if len(args) > 1 && len(args[1]) > 0 {
		if s := tr.translateFilterTokens(args[1]); s != "" && s != "NULL" {
			conds = append(conds, "period <= "+s)
		}
	}
	if len(args) > 2 && len(args[2]) > 0 {
		if s := tr.translateFilterTokens(args[2]); s != "" {
			conds = append(conds, s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if len(dims) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(dims, ", "))
	}

	return sb.String(), alias, nil
}

func (tr *translator) genBalancesAndTurnovers(reg *metadata.Register, args [][]tok) (string, string, error) {
	tableName := metadata.RegisterTableName(reg.Name)
	alias := "остаткиоборотов_" + strings.ToLower(reg.Name)
	dims := dimCols(reg.Dimensions)

	var startSQL, endSQL, filterSQL string
	if len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "NULL" {
			startSQL = s
		}
	}
	if len(args) > 1 && len(args[1]) > 0 {
		if s := tr.translateFilterTokens(args[1]); s != "NULL" {
			endSQL = s
		}
	}
	if len(args) > 2 && len(args[2]) > 0 {
		filterSQL = tr.translateFilterTokens(args[2])
	}

	var cols []string
	cols = append(cols, dims...)
	for _, r := range reg.Resources {
		col := strings.ToLower(r.Name)
		if startSQL != "" {
			cols = append(cols,
				"SUM(CASE WHEN вид_движения = 'Приход' AND period < "+startSQL+
					" THEN "+col+" WHEN вид_движения = 'Расход' AND period < "+startSQL+
					" THEN -"+col+" ELSE 0 END) AS "+col+"начальный")
		}
		periodCond := ""
		if startSQL != "" && endSQL != "" {
			periodCond = " AND period >= " + startSQL + " AND period <= " + endSQL
		} else if startSQL != "" {
			periodCond = " AND period >= " + startSQL
		} else if endSQL != "" {
			periodCond = " AND period <= " + endSQL
		}
		cols = append(cols,
			"SUM(CASE WHEN вид_движения = 'Приход'"+periodCond+" THEN "+col+" ELSE 0 END) AS "+col+"приход",
			"SUM(CASE WHEN вид_движения = 'Расход'"+periodCond+" THEN "+col+" ELSE 0 END) AS "+col+"расход",
		)
		if endSQL != "" {
			cols = append(cols,
				"SUM(CASE WHEN вид_движения = 'Приход' AND period <= "+endSQL+
					" THEN "+col+" WHEN вид_движения = 'Расход' AND period <= "+endSQL+
					" THEN -"+col+" ELSE 0 END) AS "+col+"конечный")
		}
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(cols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(tableName)

	var conds []string
	if endSQL != "" {
		conds = append(conds, "period <= "+endSQL)
	}
	if filterSQL != "" {
		conds = append(conds, filterSQL)
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if len(dims) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(dims, ", "))
	}

	return sb.String(), alias, nil
}

func (tr *translator) genLastSlice(ir *metadata.InfoRegister, args [][]tok) (string, string, error) {
	tableName := metadata.InfoRegTableName(ir.Name)
	alias := "срезпоследних_" + strings.ToLower(ir.Name)
	dims := dimCols(ir.Dimensions)

	var resCols []string
	for _, r := range ir.Resources {
		resCols = append(resCols, strings.ToLower(r.Name))
	}

	var sb strings.Builder
	if ir.Periodic && len(dims) > 0 {
		sb.WriteString("SELECT DISTINCT ON (")
		sb.WriteString(strings.Join(dims, ", "))
		sb.WriteString(") period, ")
		sb.WriteString(strings.Join(append(dims, resCols...), ", "))
	} else {
		var allCols []string
		allCols = append(allCols, dims...)
		allCols = append(allCols, resCols...)
		sb.WriteString("SELECT ")
		sb.WriteString(strings.Join(allCols, ", "))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(tableName)

	var conds []string
	if ir.Periodic && len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "period <= "+s)
		}
	}
	filterIdx := 1
	if !ir.Periodic {
		filterIdx = 0
	}
	if len(args) > filterIdx && len(args[filterIdx]) > 0 {
		if s := tr.translateFilterTokens(args[filterIdx]); s != "" {
			conds = append(conds, s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if ir.Periodic && len(dims) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(dims, ", "))
		sb.WriteString(", period DESC")
	}

	return sb.String(), alias, nil
}

func (tr *translator) genFirstSlice(ir *metadata.InfoRegister, args [][]tok) (string, string, error) {
	tableName := metadata.InfoRegTableName(ir.Name)
	alias := "срезпервых_" + strings.ToLower(ir.Name)
	dims := dimCols(ir.Dimensions)

	var resCols []string
	for _, r := range ir.Resources {
		resCols = append(resCols, strings.ToLower(r.Name))
	}

	var sb strings.Builder
	if ir.Periodic && len(dims) > 0 {
		sb.WriteString("SELECT DISTINCT ON (")
		sb.WriteString(strings.Join(dims, ", "))
		sb.WriteString(") period, ")
		sb.WriteString(strings.Join(append(dims, resCols...), ", "))
	} else {
		var allCols []string
		allCols = append(allCols, dims...)
		allCols = append(allCols, resCols...)
		sb.WriteString("SELECT ")
		sb.WriteString(strings.Join(allCols, ", "))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(tableName)

	var conds []string
	if ir.Periodic && len(args) > 0 && len(args[0]) > 0 {
		if s := tr.translateFilterTokens(args[0]); s != "" && s != "NULL" {
			conds = append(conds, "period >= "+s)
		}
	}
	filterIdx := 1
	if !ir.Periodic {
		filterIdx = 0
	}
	if len(args) > filterIdx && len(args[filterIdx]) > 0 {
		if s := tr.translateFilterTokens(args[filterIdx]); s != "" {
			conds = append(conds, s)
		}
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	if ir.Periodic && len(dims) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(dims, ", "))
		sb.WriteString(", period ASC")
	}

	return sb.String(), alias, nil
}

// --- main translator loop ---

func translate(tokens []tok, opts CompileOpts) (Result, error) {
	if opts.Params == nil {
		opts.Params = map[string]any{}
	}
	tr := &translator{
		tokens:      tokens,
		params:      map[string]int{},
		paramValues: opts.Params,
		opts:        opts,
	}
	for {
		t := tr.peek(0)
		if t.kind == tEOF {
			break
		}
		upper := strings.ToUpper(t.val)

		// Source type: TypeName.EntityName[.VirtualTable(args)] → table or subquery
		if t.kind == tIdent && isSourceType(upper) &&
			tr.peek(1).kind == tDot && tr.peek(2).kind == tIdent {

			// Check for virtual table: TypeName.EntityName.VTName(...)
			if tr.peek(3).kind == tDot && tr.peek(4).kind == tIdent &&
				tr.peek(5).kind == tLParen {
				vt4Upper := strings.ToUpper(tr.peek(4).val)

				if vtKind, ok := accumVTKinds[vt4Upper]; ok && isAccumRegType(upper) {
					tr.advance() // TypeName
					tr.advance() // .
					regName := tr.advance().val
					tr.advance() // .
					tr.advance() // VTName
					tr.advance() // (
					vtArgs := tr.parseVTArgs()
					subq, alias, err := tr.buildAccumVT(vtKind, regName, vtArgs)
					if err != nil {
						return Result{}, err
					}
					tr.emit("(" + subq + ") AS " + alias)
					continue
				}

				if vtKind, ok := infoVTKinds[vt4Upper]; ok && isInfoRegType(upper) {
					tr.advance() // TypeName
					tr.advance() // .
					regName := tr.advance().val
					tr.advance() // .
					tr.advance() // VTName
					tr.advance() // (
					vtArgs := tr.parseVTArgs()
					subq, alias, err := tr.buildInfoVT(vtKind, regName, vtArgs)
					if err != nil {
						return Result{}, err
					}
					tr.emit("(" + subq + ") AS " + alias)
					continue
				}

				if vtKind, ok := accumVTKinds[vt4Upper]; ok && isAccountRegType(upper) {
					tr.advance() // TypeName
					tr.advance() // .
					regName := tr.advance().val
					tr.advance() // .
					tr.advance() // VTName
					tr.advance() // (
					vtArgs := tr.parseVTArgs()
					subq, alias, err := tr.buildAccountVT(vtKind, regName, vtArgs)
					if err != nil {
						return Result{}, err
					}
					tr.emit("(" + subq + ") AS " + alias)
					continue
				}
			}

			// Regular source: TypeName.EntityName → table_name
			tr.advance()
			tr.advance()
			entity := tr.advance()
			tr.emit(sourceToTable(upper, entity.val))
			continue
		}

		// Multi-word: СГРУППИРОВАТЬ ПО / УПОРЯДОЧИТЬ ПО
		if t.kind == tIdent && (upper == "СГРУППИРОВАТЬ" || upper == "УПОРЯДОЧИТЬ") {
			tr.advance()
			kw := "GROUP BY"
			if upper == "УПОРЯДОЧИТЬ" {
				kw = "ORDER BY"
			}
			if tr.peek(0).kind == tIdent && strings.ToUpper(tr.peek(0).val) == "ПО" {
				tr.advance()
			}
			tr.emit(kw)
			continue
		}

		// Parameter: &Name → $N or NULL
		if t.kind == tParam {
			tr.prevWasDot = false
			tr.advance()
			tr.emit(tr.addParam(t.val))
			continue
		}

		// String literal
		if t.kind == tStr {
			tr.prevWasDot = false
			tr.advance()
			tr.emit("'" + strings.ReplaceAll(t.val, "'", "''") + "'")
			continue
		}

		// Number / star / operator
		if t.kind == tNum || t.kind == tStar || t.kind == tOp {
			tr.prevWasDot = false
			tr.advance()
			tr.emit(t.val)
			continue
		}

		// Punctuation
		if t.kind == tComma || t.kind == tLParen || t.kind == tRParen {
			tr.prevWasDot = false
			tr.advance()
			tr.emit(t.val)
			continue
		}

		if t.kind == tDot {
			tr.advance()
			tr.emit(".")
			tr.prevWasDot = true
			continue
		}

		// Identifiers: aggregate function (only before "("), keyword, or lowercase field name
		if t.kind == tIdent {
			tr.advance()
			prevDot := tr.prevWasDot
			tr.prevWasDot = false
			// .Ссылка / .Reference → .id (virtual primary-key field, like 1C)
			if prevDot && (strings.ToUpper(t.val) == "ССЫЛКА" || strings.ToUpper(t.val) == "REFERENCE" || strings.ToUpper(t.val) == "REF") {
				tr.emit("id")
				continue
			}
			if agg, ok := sqlAgg(t.val); ok && tr.peek(0).kind == tLParen {
				tr.emit(agg)
			} else if kw, ok := sqlKW(t.val); ok {
				tr.emit(kw)
			} else {
				tr.emit(strings.ToLower(t.val))
			}
			continue
		}

		tr.advance()
	}
	return Result{SQL: tr.build(), Args: tr.args}, nil
}

// pgCast returns a PostgreSQL explicit cast suffix for v.
func pgCast(v any) string {
	switch v.(type) {
	case time.Time:
		return "::timestamptz"
	case string:
		return "::text"
	case float64, float32:
		return "::numeric"
	case int, int32, int64, uint, uint32, uint64:
		return "::bigint"
	case bool:
		return "::boolean"
	}
	return ""
}
