package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// RollupRecorderType — синтетический тип регистратора опорных движений, которые
// создаёт свёртка базы (план 74). Не соответствует никакой сущности
// конфигурации: остатки на дату свёртки самодостаточны и не зависят от
// документов. OrphanMovements/DeleteOrphanMovements обязаны его пропускать —
// иначе «Очистка регистров» удалила бы опорные остатки как «осиротевшие».
const RollupRecorderType = "_СвёрткаБазы"

// RollupOptions — параметры свёртки информационной базы.
type RollupOptions struct {
	// Date — дата свёртки D. Сворачиваются движения строго ДО начала дня D
	// (period < D 00:00); опорные остатки пишутся на момент D 00:00; движения
	// с period >= D 00:00 остаются нетронутыми.
	Date time.Time
	// Registers — имена регистров накопления, которые сворачиваем. Пустой
	// список = ничего не сворачивать (оборотные регистры админ исключает,
	// просто не включая их сюда).
	Registers []string
	// DeleteDocuments — true: физически удалить документы с датой < D (вместе с
	// табличными частями); false: снять у них проведение (движения всё равно
	// свёрнуты, а дата запрета проведения защищает от перепроведения).
	DeleteDocuments bool
}

// RollupRegReport — итог свёртки по одному регистру.
type RollupRegReport struct {
	Name            string
	FoldedMovements int // движений (period < D), свёрнуто/будет свёрнуто
	OpeningRows     int // опорных строк (ненулевых остатков), создано/создастся
}

// RollupReport — сводка операции свёртки (или её предпросмотра).
type RollupReport struct {
	Cutoff       time.Time
	Preview      bool
	Registers    []RollupRegReport
	DeletedDocs  int // документов: удалено (run) или под удаление (preview); 0 при keep-режиме
	DanglingRefs int // ссылок на удаляемые документы из других записей (только preview, delete-режим)
}

// EnsureRollupTable создаёт служебный журнал свёрток _rollup. Времена хранятся
// строками (RFC3339 / дата) — это дёшево и не зависит от диалекта (логика свёртки
// берёт дату из RollupOptions, а не из этой таблицы; таблица — только аудит).
func (db *DB) EnsureRollupTable(ctx context.Context) error {
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS _rollup (
    id                TEXT PRIMARY KEY,
    cutoff            TEXT NOT NULL,
    created_at        TEXT NOT NULL,
    created_by        TEXT NOT NULL DEFAULT '',
    registers         TEXT NOT NULL DEFAULT '',
    folded_movements  INTEGER NOT NULL DEFAULT 0,
    opening_rows      INTEGER NOT NULL DEFAULT 0,
    deleted_documents INTEGER NOT NULL DEFAULT 0,
    documents_deleted INTEGER NOT NULL DEFAULT 0
)`)
	return err
}

// dayStart обрезает момент до начала дня (00:00 в той же зоне) — граница свёртки.
func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// selectRollupRegs отбирает из всех регистров только включённые в свёртку
// (по имени, регистронезависимо).
func selectRollupRegs(all []*metadata.Register, names []string) []*metadata.Register {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[strings.ToLower(n)] = true
	}
	var out []*metadata.Register
	for _, reg := range all {
		if want[strings.ToLower(reg.Name)] {
			out = append(out, reg)
		}
	}
	return out
}

// RollupPreview считает, что сделает свёртка, ничего не записывая (для UI-мастера
// и CLI --dry-run).
func (db *DB) RollupPreview(ctx context.Context, regs []*metadata.Register, ents []*metadata.Entity, opts RollupOptions) (RollupReport, error) {
	cutoff := dayStart(opts.Date)
	rep := RollupReport{Cutoff: cutoff, Preview: true}
	for _, reg := range selectRollupRegs(regs, opts.Registers) {
		folded, err := db.countMovementsBefore(ctx, reg.Name, cutoff)
		if err != nil {
			return rep, err
		}
		open, err := db.balancesBefore(ctx, reg, cutoff)
		if err != nil {
			return rep, err
		}
		rep.Registers = append(rep.Registers, RollupRegReport{
			Name: reg.Name, FoldedMovements: folded, OpeningRows: len(open),
		})
	}
	if opts.DeleteDocuments {
		n, err := db.countDocumentsBefore(ctx, ents, cutoff)
		if err != nil {
			return rep, err
		}
		rep.DeletedDocs = n
		dangling, err := db.countDanglingRefs(ctx, ents, cutoff)
		if err != nil {
			return rep, err
		}
		rep.DanglingRefs = dangling
	}
	return rep, nil
}

// countDanglingRefs оценивает, сколько ссылок повиснет при удалении документов до
// cutoff: считает записи (в шапках и ТЧ любых сущностей), чьё ссылочное поле
// указывает на удаляемый документ. Сигнал для предупреждения; слегка завышает
// (учитывает и ссылки от других удаляемых документов), что безопасно.
func (db *DB) countDanglingRefs(ctx context.Context, ents []*metadata.Entity, cutoff time.Time) (int, error) {
	d := db.dialect
	docDateCol := make(map[string]string)
	for _, e := range ents {
		if e.Kind == metadata.KindDocument {
			if c := documentDateColumn(e); c != "" {
				docDateCol[e.Name] = c
			}
		}
	}
	if len(docDateCol) == 0 {
		return 0, nil
	}
	total := 0
	scan := func(refEntity, table, col string) error {
		dateCol, ok := docDateCol[refEntity]
		if !ok {
			return nil // ссылка не на удаляемый по дате документ
		}
		var n int
		q := fmt.Sprintf(
			"SELECT COUNT(*) FROM %s WHERE %s IN (SELECT id FROM %s WHERE %s < %s)",
			table, col, metadata.TableName(refEntity), dateCol, d.Placeholder(1))
		if err := db.QueryRow(ctx, q, cutoff).Scan(&n); err != nil {
			return nil // нет колонки/таблицы — пропускаем, это лишь оценка
		}
		total += n
		return nil
	}
	for _, e := range ents {
		for _, f := range e.Fields {
			if f.RefEntity != "" {
				scan(f.RefEntity, metadata.TableName(e.Name), metadata.ColumnName(f))
			}
		}
		for _, tp := range e.TableParts {
			for _, f := range tp.Fields {
				if f.RefEntity != "" {
					scan(f.RefEntity, metadata.TablePartTableName(e.Name, tp.Name), metadata.ColumnName(f))
				}
			}
		}
	}
	return total, nil
}

// Rollup выполняет свёртку базы в одной транзакции с пост-чеком «остатки до ==
// остатки после»: при расхождении — откат и ошибка.
func (db *DB) Rollup(ctx context.Context, regs []*metadata.Register, ents []*metadata.Entity, opts RollupOptions) (RollupReport, error) {
	if err := db.EnsureRollupTable(ctx); err != nil {
		return RollupReport{}, err
	}
	cutoff := dayStart(opts.Date)
	included := selectRollupRegs(regs, opts.Registers)
	rep := RollupReport{Cutoff: cutoff}

	err := db.WithTx(ctx, func(ctx context.Context) error {
		// Снимок остатков ДО (полные, без фильтра) — для пост-чека.
		before := make(map[string][]map[string]any, len(included))
		for _, reg := range included {
			b, err := db.GetBalances(ctx, reg.Name, reg, RegFilter{})
			if err != nil {
				return err
			}
			before[reg.Name] = b
		}

		var totalFolded, totalOpening int
		for _, reg := range included {
			folded, err := db.countMovementsBefore(ctx, reg.Name, cutoff)
			if err != nil {
				return err
			}
			open, err := db.balancesBefore(ctx, reg, cutoff)
			if err != nil {
				return err
			}
			// Опорные движения на момент cutoff (recorder = новый uuid операции).
			if len(open) > 0 {
				if err := db.WriteMovements(ctx, reg.Name, RollupRecorderType, uuid.New(), open, reg, &cutoff); err != nil {
					return err
				}
			}
			// Удаляем всё свёрнутое: period < cutoff (включая опорные строки
			// прежних свёрток — они станут частью нового опорного остатка).
			// Только что вставленные опорные на period == cutoff не попадают.
			if _, err := db.Exec(ctx, fmt.Sprintf(
				"DELETE FROM %s WHERE period < %s",
				metadata.RegisterTableName(reg.Name), db.dialect.Placeholder(1)), cutoff); err != nil {
				return err
			}
			rep.Registers = append(rep.Registers, RollupRegReport{
				Name: reg.Name, FoldedMovements: folded, OpeningRows: len(open),
			})
			totalFolded += folded
			totalOpening += len(open)
		}

		// Пост-чек: полные остатки должны совпасть с зафиксированными ДО.
		for _, reg := range included {
			after, err := db.GetBalances(ctx, reg.Name, reg, RegFilter{})
			if err != nil {
				return err
			}
			if !balancesEqual(before[reg.Name], after, reg) {
				return fmt.Errorf("свёртка регистра %s: остатки до и после не совпали — откат", reg.Name)
			}
		}

		// Документы: удалить или снять проведение.
		deleted, err := db.applyRollupDocPolicy(ctx, ents, cutoff, opts.DeleteDocuments)
		if err != nil {
			return err
		}
		rep.DeletedDocs = deleted

		// Дата запрета проведения = cutoff (защищает опорные остатки и keep-режим).
		if err := db.SavePostingLockDate(ctx, cutoff); err != nil {
			return err
		}

		return db.logRollup(ctx, cutoff, included, totalFolded, totalOpening, deleted, opts.DeleteDocuments)
	})
	if err != nil {
		return RollupReport{}, err
	}
	return rep, nil
}

// countMovementsBefore — число движений регистра с period < cutoff.
func (db *DB) countMovementsBefore(ctx context.Context, regName string, cutoff time.Time) (int, error) {
	var n int
	err := db.QueryRow(ctx, fmt.Sprintf(
		"SELECT COUNT(*) FROM %s WHERE period < %s",
		metadata.RegisterTableName(regName), db.dialect.Placeholder(1)), cutoff).Scan(&n)
	return n, err
}

// balancesBefore считает остатки регистра по движениям строго ДО cutoff
// (period < cutoff), сгруппированные по измерениям. Знаковая сумма ресурсов —
// идентично GetBalances, чтобы пост-чек и UI считали остатки одинаково. Нулевые
// комбинации (все ресурсы ≈ 0) пропускаются — опорная строка им не нужна.
func (db *DB) balancesBefore(ctx context.Context, reg *metadata.Register, cutoff time.Time) ([]map[string]any, error) {
	d := db.dialect
	table := metadata.RegisterTableName(reg.Name)

	var selectParts, groupBy, dimNames, resNames []string
	for _, f := range reg.Dimensions {
		col := metadata.ColumnName(f)
		selectParts = append(selectParts, col)
		groupBy = append(groupBy, col)
		dimNames = append(dimNames, f.Name)
	}
	for _, f := range reg.Resources {
		col := metadata.ColumnName(f)
		selectParts = append(selectParts, fmt.Sprintf(
			"SUM(CASE WHEN вид_движения = 'Приход' THEN %s ELSE -%s END) AS %s", col, col, col))
		resNames = append(resNames, f.Name)
	}

	q := fmt.Sprintf("SELECT %s FROM %s WHERE period < %s",
		strings.Join(selectParts, ", "), table, d.Placeholder(1))
	if len(groupBy) > 0 {
		q += " GROUP BY " + strings.Join(groupBy, ", ")
	}
	rows, err := db.Query(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("rollup balances %s: %w", reg.Name, err)
	}
	defer rows.Close()

	total := len(reg.Dimensions) + len(reg.Resources)
	var result []map[string]any
	for rows.Next() {
		dest := make([]any, total)
		ptrs := make([]any, total)
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, total)
		for i, name := range dimNames {
			row[name] = normalizeValue(dest[i])
		}
		nonZero := false
		for i, name := range resNames {
			v := normalizeNumber(dest[len(reg.Dimensions)+i])
			row[name] = v
			if absFloat(toFloat(v)) > 1e-9 {
				nonZero = true
			}
		}
		if !nonZero {
			continue
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// balancesEqual сравнивает два набора остатков (как их возвращает GetBalances) по
// комбинации измерений с допуском по ресурсам. Используется пост-чеком свёртки.
func balancesEqual(before, after []map[string]any, reg *metadata.Register) bool {
	key := func(row map[string]any) string {
		var parts []string
		for _, f := range reg.Dimensions {
			parts = append(parts, fmt.Sprintf("%v", row[f.Name]))
		}
		return strings.Join(parts, "\x1f")
	}
	index := func(rows []map[string]any) map[string]map[string]float64 {
		m := make(map[string]map[string]float64, len(rows))
		for _, row := range rows {
			res := make(map[string]float64, len(reg.Resources))
			for _, f := range reg.Resources {
				res[f.Name] = toFloat(row[f.Name])
			}
			m[key(row)] = res
		}
		return m
	}
	bi, ai := index(before), index(after)
	if len(bi) != len(ai) {
		return false
	}
	for k, bres := range bi {
		ares, ok := ai[k]
		if !ok {
			return false
		}
		for _, f := range reg.Resources {
			if absFloat(bres[f.Name]-ares[f.Name]) > 1e-6 {
				return false
			}
		}
	}
	return true
}

// applyRollupDocPolicy удаляет (или снимает проведение) документы с датой < cutoff.
// Возвращает число удалённых документов (0 при keep-режиме). Дата документа —
// первое поле типа date; сущности без даты пропускаются.
func (db *DB) applyRollupDocPolicy(ctx context.Context, ents []*metadata.Entity, cutoff time.Time, del bool) (int, error) {
	d := db.dialect
	deleted := 0
	for _, e := range ents {
		if e.Kind != metadata.KindDocument {
			continue
		}
		dateCol := documentDateColumn(e)
		if dateCol == "" {
			continue
		}
		table := metadata.TableName(e.Name)
		if del {
			rows, err := db.Query(ctx, fmt.Sprintf(
				"SELECT id FROM %s WHERE %s < %s", table, dateCol, d.Placeholder(1)), cutoff)
			if err != nil {
				return deleted, err
			}
			var ids []uuid.UUID
			for rows.Next() {
				var raw any
				if err := rows.Scan(&raw); err != nil {
					rows.Close()
					return deleted, err
				}
				if id, ok := parseUUIDValue(raw); ok {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				if err := db.Delete(ctx, e.Name, id); err != nil {
					return deleted, fmt.Errorf("свёртка: удаление %s %s: %w", e.Name, id, err)
				}
				deleted++
			}
		} else if e.Posting {
			// Снять проведение у старых документов: их движения уже свёрнуты,
			// «проведён» без движений — противоречивое состояние.
			if _, err := db.Exec(ctx, fmt.Sprintf(
				"UPDATE %s SET posted = %s WHERE %s < %s AND posted = %s",
				table, boolFalseLit(d), dateCol, d.Placeholder(1), boolTrueLitS(d)), cutoff); err != nil {
				return deleted, err
			}
		}
	}
	return deleted, nil
}

// countDocumentsBefore — число документов с датой < cutoff (для предпросмотра
// удаления).
func (db *DB) countDocumentsBefore(ctx context.Context, ents []*metadata.Entity, cutoff time.Time) (int, error) {
	d := db.dialect
	total := 0
	for _, e := range ents {
		if e.Kind != metadata.KindDocument {
			continue
		}
		dateCol := documentDateColumn(e)
		if dateCol == "" {
			continue
		}
		var n int
		if err := db.QueryRow(ctx, fmt.Sprintf(
			"SELECT COUNT(*) FROM %s WHERE %s < %s",
			metadata.TableName(e.Name), dateCol, d.Placeholder(1)), cutoff).Scan(&n); err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// documentDateColumn возвращает имя колонки первого date-поля документа (его
// «даты»), либо "" если такого поля нет.
func documentDateColumn(e *metadata.Entity) string {
	for _, f := range e.Fields {
		if f.Type == metadata.FieldTypeDate {
			return metadata.ColumnName(f)
		}
	}
	return ""
}

// logRollup пишет запись в журнал _rollup.
func (db *DB) logRollup(ctx context.Context, cutoff time.Time, regs []*metadata.Register, folded, opening, deletedDocs int, docsDeleted bool) error {
	names := make([]string, 0, len(regs))
	for _, r := range regs {
		names = append(names, r.Name)
	}
	d := db.dialect
	docsDel := 0
	if docsDeleted {
		docsDel = 1
	}
	user, _ := auditUserFromCtx(ctx)
	_, err := db.Exec(ctx, fmt.Sprintf(
		`INSERT INTO _rollup (id, cutoff, created_at, created_by, registers, folded_movements, opening_rows, deleted_documents, documents_deleted)
		 VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s)`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4),
		d.Placeholder(5), d.Placeholder(6), d.Placeholder(7), d.Placeholder(8), d.Placeholder(9)),
		uuid.New().String(), cutoff.Format("2006-01-02"), time.Now().Format(time.RFC3339),
		user.UserLogin, strings.Join(names, ", "), folded, opening, deletedDocs, docsDel)
	return err
}

// ── Дата запрета проведения (план 74) ────────────────────────────────────────

const postingLockKey = "rollup.posting_lock_date"

// GetPostingLockDate возвращает дату запрета проведения (документы с датой <=
// этой даты нельзя проводить/перепроводить) и признак её наличия. Отсутствие
// ключа/таблицы — (нулевое время, false).
func (db *DB) GetPostingLockDate(ctx context.Context) (time.Time, bool) {
	d := db.dialect
	var v string
	if err := db.QueryRow(ctx,
		`SELECT value FROM _settings WHERE key = `+d.Placeholder(1), postingLockKey).Scan(&v); err != nil {
		return time.Time{}, false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// SavePostingLockDate сохраняет дату запрета проведения. Нулевое время очищает
// настройку (запрет снят).
func (db *DB) SavePostingLockDate(ctx context.Context, t time.Time) error {
	if err := db.EnsureSettingsSchema(ctx); err != nil {
		return err
	}
	d := db.dialect
	if t.IsZero() {
		_, err := db.Exec(ctx, `DELETE FROM _settings WHERE key = `+d.Placeholder(1), postingLockKey)
		return err
	}
	q := fmt.Sprintf(
		`INSERT INTO _settings (key, value) VALUES (%s, %s)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		d.Placeholder(1), d.Placeholder(2))
	_, err := db.Exec(ctx, q, postingLockKey, dayStart(t).Format("2006-01-02"))
	return err
}

// PostingLockViolation сообщает, запрещено ли проведение документа из-за даты
// запрета: true, если дата документа <= даты запрета. Сущности без date-поля и
// отсутствие запрета → false.
func (db *DB) PostingLockViolation(ctx context.Context, entity *metadata.Entity, id uuid.UUID) (bool, time.Time, error) {
	lock, ok := db.GetPostingLockDate(ctx)
	if !ok {
		return false, time.Time{}, nil
	}
	dateCol := documentDateColumn(entity)
	if dateCol == "" {
		return false, lock, nil
	}
	d := db.dialect
	var raw any
	err := db.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = %s",
		dateCol, metadata.TableName(entity.Name), d.Placeholder(1)), idArg(d, id)).Scan(&raw)
	if err != nil {
		return false, lock, nil
	}
	docDate, ok := parseTimeValue(raw)
	if !ok {
		return false, lock, nil
	}
	// Запрет включает саму дату запрета: документ этого дня и ранее «заморожен».
	return !dayStart(docDate).After(lock), lock, nil
}

// PostingFrozen сообщает, попадает ли дата документа в свёрнутый («заморожённый»)
// период: true, если date по дню <= даты запрета проведения. Используется
// guard'ами проведения (план 74) в ui/entityservice.
func PostingFrozen(lock, date time.Time) bool {
	return !dayStart(date).After(lock)
}

// PostingFrozenError — единый текст ошибки запрета проведения с датой запрета.
func PostingFrozenError(lock time.Time) error {
	return fmt.Errorf("проведение запрещено: документ относится к свёрнутому периоду (дата запрета — %s)", lock.Format("02.01.2006"))
}

// ── мелкие помощники ─────────────────────────────────────────────────────────

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// toFloat приводит значение остатка (float64/строка/json.Number/…) к float64.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", v)), 64)
	return f
}

// boolTrueLitS — строковый литерал true для текущего диалекта (пара к boolFalseLit).
func boolTrueLitS(d Dialect) string {
	if d.Name() == "sqlite" {
		return "1"
	}
	return "TRUE"
}

// parseUUIDValue извлекает uuid из значения колонки id (string/[]byte/uuid.UUID).
func parseUUIDValue(v any) (uuid.UUID, bool) {
	switch x := v.(type) {
	case uuid.UUID:
		return x, true
	case string:
		if id, err := uuid.Parse(strings.TrimSpace(x)); err == nil {
			return id, true
		}
	case []byte:
		if id, err := uuid.Parse(strings.TrimSpace(string(x))); err == nil {
			return id, true
		}
	}
	return uuid.UUID{}, false
}

// parseTimeValue извлекает время из значения колонки даты (time.Time/строка).
func parseTimeValue(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, strings.TrimSpace(x)); err == nil {
				return t, true
			}
		}
	case []byte:
		return parseTimeValue(string(x))
	}
	return time.Time{}, false
}
