package metadata

// Endpoint — входящий REST-эндпоинт (план 58): внешняя система зовёт
// /api/hooks/<path> и попадает в DSL-обработчик конфигурации. Вместе с
// исходящими веб-хуками (план 29) образует «живую» интеграционную платформу.

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Endpoint описывает один входящий эндпоинт (endpoints/<имя>.yaml).
type Endpoint struct {
	Name string `yaml:"name"`
	// Path — путь под /api: "/hooks/telegram" → URL /api/hooks/telegram.
	Path   string `yaml:"path"`
	Method string `yaml:"method"` // по умолчанию POST
	// Auth: none | token (заголовок X-Webhook-Token) | hmac (X-Webhook-Signature =
	// hex(HMAC-SHA256(тело, secret))).
	Auth   string `yaml:"auth"`
	Secret string `yaml:"secret"` // задавайте через ${env:VAR} — секрет живёт в окружении
	// Handler — имя DSL-модуля без суффикса: src/<handler>.endpoint.os с
	// процедурой Обработать(Запрос, Ответ). По умолчанию — имя эндпоинта
	// в нижнем регистре.
	Handler   string `yaml:"handler"`
	RateLimit int    `yaml:"rate_limit"` // запросов/мин на эндпоинт; 0 = без лимита
}

// LoadEndpointFile читает и нормализует endpoints/<имя>.yaml.
func LoadEndpointFile(path string) (*Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ep Endpoint
	if err := yaml.Unmarshal(data, &ep); err != nil {
		return nil, fmt.Errorf("endpoint %s: %w", path, err)
	}
	ep.normalize()
	return &ep, nil
}

func (e *Endpoint) normalize() {
	e.Method = strings.ToUpper(strings.TrimSpace(e.Method))
	if e.Method == "" {
		e.Method = "POST"
	}
	e.Auth = strings.ToLower(strings.TrimSpace(e.Auth))
	if e.Auth == "" {
		e.Auth = "none"
	}
	e.Path = strings.TrimSpace(e.Path)
	if e.Path != "" && !strings.HasPrefix(e.Path, "/") {
		e.Path = "/" + e.Path
	}
	if e.Handler == "" {
		e.Handler = strings.ToLower(e.Name)
	}
}

// Validate проверяет согласованность эндпоинта (вызывается onebase check).
func (e *Endpoint) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("endpoint: не задано имя (name)")
	}
	if strings.TrimSpace(e.Path) == "" {
		return fmt.Errorf("endpoint %s: не задан путь (path)", e.Name)
	}
	switch e.Method {
	case "POST", "GET", "PUT", "DELETE", "PATCH", "":
	default:
		return fmt.Errorf("endpoint %s: неподдерживаемый метод %q", e.Name, e.Method)
	}
	switch e.Auth {
	case "none", "":
	case "token", "hmac":
		if strings.TrimSpace(e.Secret) == "" {
			return fmt.Errorf("endpoint %s: auth %q требует secret (используйте ${env:VAR})", e.Name, e.Auth)
		}
	default:
		return fmt.Errorf("endpoint %s: неизвестный auth %q (none|token|hmac)", e.Name, e.Auth)
	}
	return nil
}
