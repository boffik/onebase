package ui

// Rate-limit ИИ-чата (план 54, этап 3): один запрос чата — до 12 раундов
// tool-use × вызовы LLM, т.е. неконтролируемый расход денег на провайдера
// (cost-DoS). Лимитер окна считает запросы на пользователя в минуту.

import (
	"sync"
	"time"
)

type aiWindowLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newAIWindowLimiter(max int, window time.Duration) *aiWindowLimiter {
	return &aiWindowLimiter{max: max, window: window, hits: make(map[string][]time.Time)}
}

// Allow регистрирует запрос и сообщает, укладывается ли он в лимит окна.
func (l *aiWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
