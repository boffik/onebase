package launcher

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/configdb"
	"github.com/ivantit66/onebase/internal/metadata"
	"gopkg.in/yaml.v3"
)

// configuratorSaveWidget upserts a single widgets/<name>.yaml entry. The body
// is taken verbatim from the textarea, then validated by re-parsing through
// metadata.LoadWidgetFile so users see syntax errors instead of broken pages.
func (h *handler) configuratorSaveWidget(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	lang := resolveLang(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("widget_name"))
	body := r.FormValue("yaml")
	if name == "" {
		data := h.loadCfgData(r.Context(), b, "tree")
		data.Error = tr(lang, "Имя виджета не задано")
		renderCfg(w, r, data)
		return
	}

	// Validate: parse without writing to disk first so a malformed YAML never
	// replaces a working widget definition.
	tmp, err := os.CreateTemp("", "widget-*.yaml")
	if err == nil {
		tmp.WriteString(body)
		tmp.Close()
		defer os.Remove(tmp.Name())
		if _, perr := metadata.LoadWidgetFile(tmp.Name()); perr != nil {
			data := h.loadCfgData(r.Context(), b, "tree")
			data.Error = tr(lang, "Ошибка YAML") + ": " + perr.Error()
			renderCfg(w, r, data)
			return
		}
	}

	relPath := "widgets/" + nameToFilename(name) + ".yaml"
	saveErr := saveConfigFile(r, h, b, relPath, []byte(body))

	data := h.loadCfgData(r.Context(), b, "tree")
	if saveErr != nil {
		data.Error = tr(lang, "Ошибка сохранения") + ": " + saveErr.Error()
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = name
	}
	renderCfg(w, r, data)
}

// configuratorDeleteWidget removes widgets/<name>.yaml from the configuration.
func (h *handler) configuratorDeleteWidget(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	lang := resolveLang(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("widget_name"))
	if name == "" {
		data := h.loadCfgData(r.Context(), b, "tree")
		data.Error = tr(lang, "Имя виджета не задано")
		renderCfg(w, r, data)
		return
	}
	relPath := "widgets/" + nameToFilename(name) + ".yaml"
	delErr := deleteConfigFile(r, h, b, relPath)
	data := h.loadCfgData(r.Context(), b, "tree")
	if delErr != nil {
		data.Error = tr(lang, "Ошибка удаления") + ": " + delErr.Error()
	}
	renderCfg(w, r, data)
}

// configuratorSaveHomePage writes config/home_page.yaml verbatim. Validation
// is YAML-only — empty layout means "use defaults" which is supported by the
// runtime.
func (h *handler) configuratorSaveHomePage(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	lang := resolveLang(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body := r.FormValue("yaml")

	if strings.TrimSpace(body) != "" {
		var probe map[string]any
		if perr := yaml.Unmarshal([]byte(body), &probe); perr != nil {
			data := h.loadCfgData(r.Context(), b, "tree")
			data.Error = tr(lang, "Ошибка YAML") + ": " + perr.Error()
			renderCfg(w, r, data)
			return
		}
	}

	saveErr := saveConfigFile(r, h, b, "config/home_page.yaml", []byte(body))
	data := h.loadCfgData(r.Context(), b, "tree")
	if saveErr != nil {
		data.Error = tr(lang, "Ошибка сохранения") + ": " + saveErr.Error()
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = "home-page"
	}
	renderCfg(w, r, data)
}

// saveConfigFile is a small wrapper used by widget/homepage handlers. It writes
// the given relative path either to the database-backed config table or to the
// file-based project directory, matching whichever storage mode the base uses.
func saveConfigFile(r *http.Request, h *handler, b *Base, relPath string, content []byte) error {
	if b.ConfigSource == "database" {
		db, err := OpenDB(r.Context(), b)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := configdb.New(db)
		if err := repo.EnsureSchema(r.Context()); err != nil {
			return err
		}
		return repo.SaveFile(r.Context(), relPath, content)
	}
	full := filepath.Join(b.Path, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0o644)
}

func deleteConfigFile(r *http.Request, h *handler, b *Base, relPath string) error {
	if b.ConfigSource == "database" {
		db, err := OpenDB(r.Context(), b)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := configdb.New(db)
		return repo.DeleteFile(r.Context(), relPath)
	}
	full := filepath.Join(b.Path, filepath.FromSlash(relPath))
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
