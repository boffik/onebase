package ui

import (
	"net/http"

	"github.com/ivantit66/onebase/internal/storage"
)

// setFormMode пишет персональный режим открытия форм текущего пользователя
// (поле формы mode = pages | tabs | default) и уводит на вход выбранного
// режима. issue #129/#130.
func (s *Server) setFormMode(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue("mode")
	login := currentUserLogin(r)
	if err := s.store.SaveUserFormOpenMode(r.Context(), login, mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	target := "/ui"
	if s.store.EffectiveFormOpenMode(r.Context(), login) == storage.FormModeTabs {
		target = "/ui/app"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
