package auth

// Одноразовые bootstrap-коды (план 53, этап 1). Сессионный токен живёт 24 часа
// и не должен появляться в URL (логи, Referer, история браузера). Вместо него
// конфигуратор получает короткоживущий single-use код через POST
// /auth/one-time-code и открывает /auth/bootstrap?code=... — код гаснет после
// первого обмена, утечка его значения через лог почти бесполезна.

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type otcEntry struct {
	token   string
	expires time.Time
}

// OneTimeCodes — in-memory хранилище одноразовых кодов с TTL.
// Потокобезопасно; протухшие записи вычищаются лениво при Issue/Exchange.
type OneTimeCodes struct {
	mu    sync.Mutex
	ttl   time.Duration
	codes map[string]otcEntry
}

func NewOneTimeCodes(ttl time.Duration) *OneTimeCodes {
	return &OneTimeCodes{ttl: ttl, codes: make(map[string]otcEntry)}
}

// Issue выдаёт одноразовый код для сессионного токена.
func (o *OneTimeCodes) Issue(token string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf)

	o.mu.Lock()
	defer o.mu.Unlock()
	o.purgeLocked()
	o.codes[code] = otcEntry{token: token, expires: time.Now().Add(o.ttl)}
	return code, nil
}

// Exchange обменивает код на токен сессии. Код одноразовый: удаляется при
// первом обращении независимо от результата.
func (o *OneTimeCodes) Exchange(code string) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.purgeLocked()
	e, ok := o.codes[code]
	if !ok {
		return "", false
	}
	delete(o.codes, code)
	if time.Now().After(e.expires) {
		return "", false
	}
	return e.token, true
}

func (o *OneTimeCodes) purgeLocked() {
	now := time.Now()
	for c, e := range o.codes {
		if now.After(e.expires) {
			delete(o.codes, c)
		}
	}
}
