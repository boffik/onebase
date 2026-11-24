package launcher

import (
	"net/http"

	"github.com/boffik/onebase/internal/dsl/langref"
	"github.com/go-chi/chi/v5"
)

// configuratorLangref отдаёт справочник встроенного языка (статический, из
// реестра langref) для автодополнения/hover/окна-справочника в конфигураторе.
// База по id не нужна для данных, но cfgAuthMiddleware уже валидирует id;
// guard ниже даёт чистый 404 при прямом вызове без middleware.
func (h *handler) configuratorLangref(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.Get(chi.URLParam(r, "id")); err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, langref.All())
}
