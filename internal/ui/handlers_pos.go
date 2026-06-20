package ui

import "net/http"

// agentSettings — настройки рабочего места: адрес и токен локального
// device-agent. Серверу хранить нечего (значения per-машина живут в localStorage
// браузера), поэтому страница статична — только рендер формы.
func (s *Server) agentSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "page-agent-settings", map[string]any{})
}

// posPage — рабочее место кассира в основном UI: кнопки печати/веса/оплаты/
// фискализации и поле сканера обращаются к локальному агенту из браузера через
// onebaseDevice. Сервер onebase к агенту не ходит — страница статична.
func (s *Server) posPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "page-pos", map[string]any{})
}
