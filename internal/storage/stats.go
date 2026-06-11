package storage

import "time"

// PoolStats — снимок состояния пула соединений PostgreSQL. Для SQLite не
// применимо (PoolStats возвращает nil). Поля повторяют pgxpool.Stat, но в виде
// простого DTO, чтобы вызывающий код (например, /metrics в internal/api) не
// импортировал pgxpool напрямую.
type PoolStats struct {
	AcquiredConns        int32         // соединения, занятые прямо сейчас
	ConstructingConns    int32         // соединения в процессе установки
	IdleConns            int32         // свободные соединения в пуле
	MaxConns             int32         // максимум соединений
	TotalConns           int32         // всего соединений (idle + acquired + constructing)
	NewConnsCount        int64         // сколько соединений создано за всё время
	AcquireCount         int64         // успешные Acquire
	EmptyAcquireCount    int64         // Acquire, ждавшие свободного соединения
	CanceledAcquireCount int64         // Acquire, отменённые контекстом
	AcquireDuration      time.Duration // суммарное время ожидания Acquire
}

// PoolStats возвращает статистику пула соединений или nil для SQLite-подключения.
func (db *DB) PoolStats() *PoolStats {
	if db.pool == nil {
		return nil
	}
	s := db.pool.Stat()
	return &PoolStats{
		AcquiredConns:        s.AcquiredConns(),
		ConstructingConns:    s.ConstructingConns(),
		IdleConns:            s.IdleConns(),
		MaxConns:             s.MaxConns(),
		TotalConns:           s.TotalConns(),
		NewConnsCount:        s.NewConnsCount(),
		AcquireCount:         s.AcquireCount(),
		EmptyAcquireCount:    s.EmptyAcquireCount(),
		CanceledAcquireCount: s.CanceledAcquireCount(),
		AcquireDuration:      s.AcquireDuration(),
	}
}
