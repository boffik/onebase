package storage

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	sqlite "modernc.org/sqlite"
)

// ErrForeignKeyViolation возвращается из путей записи (upsert), когда драйвер БД
// отклонил INSERT/UPDATE из-за нарушения внешнего ключа — например, поле-ссылка
// указывает на UUID, которого нет в целевой таблице.
//
// Нарушение распознаётся по коду ошибки драйвера, а не по тексту:
//   - SQLite (modernc.org/sqlite): расширенный код 787
//     (SQLITE_CONSTRAINT_FOREIGNKEY);
//   - PostgreSQL (pgconn): SQLSTATE 23503 (foreign_key_violation).
//
// Вызывающий HTTP-код перехватывает через errors.Is и отдаёт структурированный
// 422 вместо сырого 500 с текстом драйвера. Ср. ErrVersionConflict.
var ErrForeignKeyViolation = errors.New("storage: нарушение внешнего ключа (ссылка на несуществующий объект)")

// sqliteConstraintForeignKey — расширенный result-код SQLite для нарушения FK
// (SQLITE_CONSTRAINT | (3<<8)). modernc.org/sqlite отдаёт именно расширенный код.
const sqliteConstraintForeignKey = 787

// pgForeignKeyViolation — SQLSTATE PostgreSQL для foreign_key_violation.
const pgForeignKeyViolation = "23503"

// classifyConstraintErr оборачивает ошибку драйвера в типизированную
// ErrForeignKeyViolation, если это нарушение внешнего ключа. Остальные ошибки
// (включая другие нарушения ограничений) возвращаются без изменений — их текст
// не утекает в API-ответ по этому пути.
func classifyConstraintErr(err error) error {
	if err == nil {
		return nil
	}
	var sqErr *sqlite.Error
	if errors.As(err, &sqErr) && sqErr.Code() == sqliteConstraintForeignKey {
		return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
	}
	return err
}
