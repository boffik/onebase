package api

// Логирование HTTP-запросов с редактированием чувствительных query-параметров.
// middleware.Logger пишет полный RequestURI в stdout — значения сессионных
// токенов и одноразовых кодов в логе недопустимы (план 53, анализ §2.2).

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// sensitiveParams — query-параметры, значения которых вырезаются из лога.
var sensitiveParams = map[string]bool{
	"_tk":      true,
	"token":    true,
	"code":     true,
	"api_key":  true,
	"apikey":   true,
	"password": true,
	"secret":   true,
}

// redactURI заменяет значения чувствительных параметров на ***, сохраняя
// порядок и кодировку остальных (работает по сырой строке, без перекодирования).
func redactURI(uri string) string {
	q := strings.IndexByte(uri, '?')
	if q < 0 {
		return uri
	}
	path, query := uri[:q], uri[q+1:]
	parts := strings.Split(query, "&")
	changed := false
	for idx, p := range parts {
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			continue
		}
		if sensitiveParams[strings.ToLower(p[:eq])] {
			parts[idx] = p[:eq] + "=***"
			changed = true
		}
	}
	if !changed {
		return uri
	}
	return path + "?" + strings.Join(parts, "&")
}

// redactingFormatter оборачивает стандартный LogFormatter chi, подменяя
// RequestURI на отредактированный перед форматированием строки лога.
type redactingFormatter struct{ inner middleware.LogFormatter }

func (f redactingFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	if red := redactURI(r.RequestURI); red != r.RequestURI {
		r2 := r.Clone(r.Context())
		r2.RequestURI = red
		r = r2
	}
	return f.inner.NewLogEntry(r)
}

// requestLogger — замена middleware.Logger с редактированием секретов.
func requestLogger() func(http.Handler) http.Handler {
	return middleware.RequestLogger(redactingFormatter{
		inner: &middleware.DefaultLogFormatter{Logger: log.New(os.Stdout, "", log.LstdFlags)},
	})
}
