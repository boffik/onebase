package ui

import (
	"net/http"
	"strings"

	"github.com/ivantit66/onebase/internal/metadata"
)

// pickManagedForm возвращает первую managed-форму нужного Kind из Entity.Forms
// или nil, если такой нет. Используется в рантайме для опционального
// переключения с авто-генерации на ручную форму из forms/<entity>/*.form.yaml.
//
// kind: "object" — карточка элемента/документа, "list" — форма списка,
// "choice" — форма выбора, "" — любая (берётся первая managed).
func pickManagedForm(entity *metadata.Entity, kind string) *metadata.FormModule {
	if entity == nil {
		return nil
	}
	kindLower := strings.ToLower(kind)
	for _, fm := range entity.Forms {
		if fm == nil || !fm.IsManaged() {
			continue
		}
		if kindLower == "" || strings.ToLower(fm.Kind) == kindLower {
			return fm
		}
	}
	return nil
}

// renderEntityForm — единая точка рендера формы объекта/документа.
// Если для Entity есть managed-форма с подходящим Kind — рендерит
// page-managed-form с теми же data + "Form": managed-форма.
// Иначе — текущий page-form (auto-generated).
//
// Это даёт пользователю опциональность: создание .form.yaml в проекте
// автоматически активирует managed-рендер для выбранной сущности; без
// .form.yaml продолжает работать существующая авто-форма без изменений.
func (s *Server) renderEntityForm(w http.ResponseWriter, r *http.Request, kind string, data map[string]any) {
	entity, _ := data["Entity"].(*metadata.Entity)
	managed := pickManagedForm(entity, kind)
	if managed != nil {
		data["Form"] = managed
		s.render(w, r, "page-managed-form", data)
		return
	}
	s.render(w, r, "page-form", data)
}
