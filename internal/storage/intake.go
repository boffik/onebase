package storage

// Хранилище входного шлюза (план 90, пробел G6):
//   _intake_log — стор идемпотентности. UNIQUE(intake, scope, key) + атомарный
//     INSERT … ON CONFLICT DO NOTHING даёт «insert-if-new» без гонки TOCTOU:
//     из N конкурентных дублей ровно один получает inserted=true (тот же приём,
//     что _numerators и _exchange_*).
//   _intake_dlq — карантин: сырое тело + причина + попытки, для ручного replay.
//
// Время хранится как BIGINT-эпоха (секунды), как changed_at обмена, — чтобы TTL
// считался арифметикой в Go и не зависел от разбора dialect-таймстампов.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Статусы строки идемпотентности _intake_log.
const (
	IntakeStatusReceived    = "received"    // строка занята, обработчик ещё не завершён
	IntakeStatusProcessed   = "processed"   // бизнес-объект создан, результат сохранён
	IntakeStatusQuarantined = "quarantined" // обработчик упал, событие в карантине
)

// Состояния записи карантина _intake_dlq.
const (
	DLQStateOpen      = "open"      // ждёт разбора/повтора
	DLQStateReplayed  = "replayed"  // успешно переигран
	DLQStateDiscarded = "discarded" // отброшен вручную
)

// IntakeLogRow — строка стора идемпотентности.
type IntakeLogRow struct {
	Intake         string
	Scope          string
	Key            string
	PayloadHash    string
	Status         string
	ResultRef      string
	BusinessResult string
	CorrelationID  string
	ReceivedAt     int64
	TTLExpiresAt   int64 // 0 — бессрочно
}

// IntakeDLQEntry — запись карантина.
type IntakeDLQEntry struct {
	ID            string
	Intake        string
	Key           string
	Scope         string
	Payload       string // сырое тело конверта (JSON)
	Reason        string
	Error         string
	Attempts      int
	CorrelationID string
	QuarantinedAt int64
	ReplayState   string
}

// EnsureIntakeSchema создаёт служебные таблицы приёмки. Идемпотентно.
func (db *DB) EnsureIntakeSchema(ctx context.Context) error {
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _intake_log (
			intake          TEXT   NOT NULL,
			scope           TEXT   NOT NULL DEFAULT '',
			key             TEXT   NOT NULL,
			payload_hash    TEXT   NOT NULL DEFAULT '',
			status          TEXT   NOT NULL DEFAULT 'received',
			result_ref      TEXT   NOT NULL DEFAULT '',
			business_result TEXT   NOT NULL DEFAULT '',
			correlation_id  TEXT   NOT NULL DEFAULT '',
			received_at     BIGINT NOT NULL DEFAULT 0,
			ttl_expires_at  BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (intake, scope, key)
		)`); err != nil {
		return fmt.Errorf("intake: create _intake_log: %w", err)
	}
	if _, err := db.Exec(ctx,
		`CREATE INDEX IF NOT EXISTS idx_intake_log_ttl ON _intake_log (ttl_expires_at)`); err != nil {
		return fmt.Errorf("intake: index _intake_log ttl: %w", err)
	}
	d := db.dialect
	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS _intake_dlq (
			id             %s PRIMARY KEY,
			intake         TEXT   NOT NULL,
			key            TEXT   NOT NULL DEFAULT '',
			scope          TEXT   NOT NULL DEFAULT '',
			payload        TEXT   NOT NULL DEFAULT '',
			reason         TEXT   NOT NULL DEFAULT '',
			error          TEXT   NOT NULL DEFAULT '',
			attempts       INTEGER NOT NULL DEFAULT 0,
			correlation_id TEXT   NOT NULL DEFAULT '',
			quarantined_at BIGINT NOT NULL DEFAULT 0,
			replay_state   TEXT   NOT NULL DEFAULT 'open'
		)`, d.TypeUUID())
	if _, err := db.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("intake: create _intake_dlq: %w", err)
	}
	if _, err := db.Exec(ctx,
		`CREATE INDEX IF NOT EXISTS idx_intake_dlq_open ON _intake_dlq (intake, replay_state)`); err != nil {
		return fmt.Errorf("intake: index _intake_dlq open: %w", err)
	}
	return nil
}

// InsertIntakeLogIfNew атомарно вставляет строку идемпотентности, если её ещё
// нет. inserted=true — строка новая (вызывающий владеет обработкой); false —
// ключ уже занят (дубль или конфликт), актуальную строку читай GetIntakeLog.
//
// Гарантия опирается на UNIQUE(intake, scope, key) + ON CONFLICT DO NOTHING:
// под конкуренцией уникальный индекс сериализует вставки, ровно одна побеждает.
func (db *DB) InsertIntakeLogIfNew(ctx context.Context, row IntakeLogRow) (bool, error) {
	if row.Status == "" {
		row.Status = IntakeStatusReceived
	}
	d := db.dialect
	q := fmt.Sprintf(`
		INSERT INTO _intake_log
			(intake, scope, key, payload_hash, status, result_ref, business_result, correlation_id, received_at, ttl_expires_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT (intake, scope, key) DO NOTHING`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4), d.Placeholder(5),
		d.Placeholder(6), d.Placeholder(7), d.Placeholder(8), d.Placeholder(9), d.Placeholder(10))
	tag, err := db.Exec(ctx, q,
		row.Intake, row.Scope, row.Key, row.PayloadHash, row.Status,
		row.ResultRef, row.BusinessResult, row.CorrelationID, row.ReceivedAt, row.TTLExpiresAt)
	if err != nil {
		return false, fmt.Errorf("intake: insert-if-new: %w", err)
	}
	return tag.RowsAffected == 1, nil
}

// GetIntakeLog читает строку идемпотентности. found=false — строки нет.
func (db *DB) GetIntakeLog(ctx context.Context, intake, scope, key string) (IntakeLogRow, bool, error) {
	d := db.dialect
	q := fmt.Sprintf(`
		SELECT intake, scope, key, payload_hash, status, result_ref, business_result, correlation_id, received_at, ttl_expires_at
		FROM _intake_log WHERE intake = %s AND scope = %s AND key = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3))
	var r IntakeLogRow
	err := db.QueryRow(ctx, q, intake, scope, key).Scan(
		&r.Intake, &r.Scope, &r.Key, &r.PayloadHash, &r.Status,
		&r.ResultRef, &r.BusinessResult, &r.CorrelationID, &r.ReceivedAt, &r.TTLExpiresAt)
	if IsNotFound(err) {
		return IntakeLogRow{}, false, nil
	}
	if err != nil {
		return IntakeLogRow{}, false, fmt.Errorf("intake: get log: %w", err)
	}
	return r, true, nil
}

// SetIntakeLogProcessed помечает строку обработанной и сохраняет результат.
// Вызывать в той же транзакции, что и создание бизнес-объекта (storage.WithTx).
func (db *DB) SetIntakeLogProcessed(ctx context.Context, intake, scope, key, resultRef, businessResult string) error {
	d := db.dialect
	q := fmt.Sprintf(`
		UPDATE _intake_log SET status = %s, result_ref = %s, business_result = %s
		WHERE intake = %s AND scope = %s AND key = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3),
		d.Placeholder(4), d.Placeholder(5), d.Placeholder(6))
	if _, err := db.Exec(ctx, q, IntakeStatusProcessed, resultRef, businessResult, intake, scope, key); err != nil {
		return fmt.Errorf("intake: mark processed: %w", err)
	}
	return nil
}

// DeleteIntakeLog удаляет строку идемпотентности. Нужен, когда причина сбоя не
// входит в dlq.on: событие отклоняется без карантина, а осиротевшую received-
// строку надо снять, чтобы корректный повтор обработался заново.
func (db *DB) DeleteIntakeLog(ctx context.Context, intake, scope, key string) error {
	d := db.dialect
	q := fmt.Sprintf(`DELETE FROM _intake_log WHERE intake = %s AND scope = %s AND key = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3))
	if _, err := db.Exec(ctx, q, intake, scope, key); err != nil {
		return fmt.Errorf("intake: delete log: %w", err)
	}
	return nil
}

// SetIntakeLogStatus меняет статус строки идемпотентности.
func (db *DB) SetIntakeLogStatus(ctx context.Context, intake, scope, key, status string) error {
	d := db.dialect
	q := fmt.Sprintf(`UPDATE _intake_log SET status = %s WHERE intake = %s AND scope = %s AND key = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4))
	if _, err := db.Exec(ctx, q, status, intake, scope, key); err != nil {
		return fmt.Errorf("intake: set status: %w", err)
	}
	return nil
}

// InsertIntakeDLQ добавляет запись карантина. Возвращает её id.
func (db *DB) InsertIntakeDLQ(ctx context.Context, e IntakeDLQEntry) (string, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.ReplayState == "" {
		e.ReplayState = DLQStateOpen
	}
	d := db.dialect
	q := fmt.Sprintf(`
		INSERT INTO _intake_dlq
			(id, intake, key, scope, payload, reason, error, attempts, correlation_id, quarantined_at, replay_state)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4), d.Placeholder(5), d.Placeholder(6),
		d.Placeholder(7), d.Placeholder(8), d.Placeholder(9), d.Placeholder(10), d.Placeholder(11))
	if _, err := db.Exec(ctx, q,
		e.ID, e.Intake, e.Key, e.Scope, e.Payload, e.Reason, e.Error,
		e.Attempts, e.CorrelationID, e.QuarantinedAt, e.ReplayState); err != nil {
		return "", fmt.Errorf("intake: insert dlq: %w", err)
	}
	return e.ID, nil
}

// ListIntakeDLQ возвращает записи карантина шлюза (новые первыми). onlyOpen —
// только неразобранные (replay_state = open).
func (db *DB) ListIntakeDLQ(ctx context.Context, intake string, onlyOpen bool, limit int) ([]IntakeDLQEntry, error) {
	if limit <= 0 {
		limit = 200
	}
	d := db.dialect
	where := fmt.Sprintf("intake = %s", d.Placeholder(1))
	args := []any{intake}
	if onlyOpen {
		where += fmt.Sprintf(" AND replay_state = %s", d.Placeholder(2))
		args = append(args, DLQStateOpen)
	}
	q := fmt.Sprintf(`
		SELECT id, intake, key, scope, payload, reason, error, attempts, correlation_id, quarantined_at, replay_state
		FROM _intake_dlq WHERE %s ORDER BY quarantined_at DESC, id LIMIT %d`, where, limit)
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("intake: list dlq: %w", err)
	}
	defer rows.Close()
	var out []IntakeDLQEntry
	for rows.Next() {
		var e IntakeDLQEntry
		if err := rows.Scan(&e.ID, &e.Intake, &e.Key, &e.Scope, &e.Payload, &e.Reason,
			&e.Error, &e.Attempts, &e.CorrelationID, &e.QuarantinedAt, &e.ReplayState); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetOpenIntakeDLQ возвращает открытую (replay_state = open) запись карантина по
// ключу — точка входа для replay. found=false — открытой записи нет.
func (db *DB) GetOpenIntakeDLQ(ctx context.Context, intake, key string) (IntakeDLQEntry, bool, error) {
	d := db.dialect
	q := fmt.Sprintf(`
		SELECT id, intake, key, scope, payload, reason, error, attempts, correlation_id, quarantined_at, replay_state
		FROM _intake_dlq WHERE intake = %s AND key = %s AND replay_state = %s
		ORDER BY quarantined_at DESC, id LIMIT 1`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3))
	var e IntakeDLQEntry
	err := db.QueryRow(ctx, q, intake, key, DLQStateOpen).Scan(
		&e.ID, &e.Intake, &e.Key, &e.Scope, &e.Payload, &e.Reason,
		&e.Error, &e.Attempts, &e.CorrelationID, &e.QuarantinedAt, &e.ReplayState)
	if IsNotFound(err) {
		return IntakeDLQEntry{}, false, nil
	}
	if err != nil {
		return IntakeDLQEntry{}, false, fmt.Errorf("intake: get open dlq: %w", err)
	}
	return e, true, nil
}

// SetIntakeDLQState меняет состояние записи карантина (open|replayed|discarded).
func (db *DB) SetIntakeDLQState(ctx context.Context, id, state string) error {
	d := db.dialect
	q := fmt.Sprintf(`UPDATE _intake_dlq SET replay_state = %s WHERE id = %s`, d.Placeholder(1), d.Placeholder(2))
	if _, err := db.Exec(ctx, q, state, id); err != nil {
		return fmt.Errorf("intake: set dlq state: %w", err)
	}
	return nil
}

// BumpIntakeDLQAttempts увеличивает счётчик попыток и записывает последнюю ошибку.
func (db *DB) BumpIntakeDLQAttempts(ctx context.Context, id, lastErr string) error {
	d := db.dialect
	q := fmt.Sprintf(`UPDATE _intake_dlq SET attempts = attempts + 1, error = %s WHERE id = %s`,
		d.Placeholder(1), d.Placeholder(2))
	if _, err := db.Exec(ctx, q, lastErr, id); err != nil {
		return fmt.Errorf("intake: bump dlq attempts: %w", err)
	}
	return nil
}
