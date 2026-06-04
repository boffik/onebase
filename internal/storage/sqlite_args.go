package storage

import "time"

// sqliteTimeLayout — каноничный формат записи time.Time в SQLite (по стенным
// часам, без перевода в UTC).
//
// Зачем: драйвер modernc.org/sqlite по умолчанию биндит time.Time как Go-строку
// `2006-01-02 15:04:05 -0700 MST` (с именем зоны, напр. `… +0300 MSK`). Функции
// дат SQLite (`date`, `strftime`) такой формат распарсить НЕ могут и молча
// возвращают NULL. Из-за этого ломалась группировка по периоду:
// `.Обороты(, , Месяц)`, `НАЧАЛОПЕРИОДА`, `Год/Месяц/День` сваливали все периоды
// в одну пустую корзину, а виджеты/отчёты «по месяцам» считали неверно — при
// зелёном `onebase check`.
//
// Стенные часы сохраняем как есть (без UTC), иначе полночь по локали уехала бы
// на предыдущий день. Формат совпадает с тем, как SQLite хранит `datetime('now')`.
const sqliteTimeLayout = "2006-01-02 15:04:05"

// normalizeSQLiteArgs возвращает копию args, где каждый time.Time (и непустой
// *time.Time) приведён к strftime-совместимой строке; остальные значения
// проходят как есть. Применяется только на SQLite-пути — pgx биндит time.Time
// нативно как timestamptz, там нормализация не нужна и вредна.
func normalizeSQLiteArgs(args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case time.Time:
			out[i] = v.Format(sqliteTimeLayout)
		case *time.Time:
			if v == nil {
				out[i] = nil
			} else {
				out[i] = v.Format(sqliteTimeLayout)
			}
		default:
			out[i] = a
		}
	}
	return out
}
