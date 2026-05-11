package launcher

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/backup"
	"github.com/ivantit66/onebase/internal/storage"
	"gopkg.in/yaml.v3"
)

func (h *handler) backupDir(b *Base) string {
	if b.Path != "" {
		return filepath.Join(b.Path, "backups")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".onebase", "backups", b.ID)
}

func (h *handler) backupCreate(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	dir := h.backupDir(b)
	outPath, dumpErr := backup.Dump(r.Context(), b.DB, dir)
	data := h.loadCfgData(r.Context(), b, "backup")
	if dumpErr != nil {
		data.Error = "Ошибка бэкапа: " + dumpErr.Error()
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = "panel-backup"
		data.BackupMessage = "Бэкап создан: " + filepath.Base(outPath)
	}
	renderCfg(w, data)
}

func (h *handler) backupDownload(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file := chi.URLParam(r, "file")
	dir := h.backupDir(b)
	fp := filepath.Join(dir, file)
	if _, err := os.Stat(fp); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+file)
	http.ServeFile(w, r, fp)
}

func (h *handler) backupDelete(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file := chi.URLParam(r, "file")
	os.Remove(filepath.Join(h.backupDir(b), file))
	data := h.loadCfgData(r.Context(), b, "backup")
	data.FieldsSaved = true
	data.FieldsSavedEntity = "panel-backup"
	renderCfg(w, data)
}

func (h *handler) backupSettings(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	type backupCfg struct {
		Enabled   bool   `yaml:"enabled"`
		Schedule  string `yaml:"schedule"`
		KeepLast  int    `yaml:"keep_last"`
		Directory string `yaml:"directory"`
	}
	type appCfgWithBackup struct {
		Name    string    `yaml:"name"`
		Version string    `yaml:"version,omitempty"`
		Backup  backupCfg `yaml:"backup,omitempty"`
	}
	keepLast, _ := strconv.Atoi(r.FormValue("backup_keep"))
	cfg := backupCfg{
		Enabled:   r.FormValue("backup_enabled") == "on",
		Schedule:  strings.TrimSpace(r.FormValue("backup_schedule")),
		KeepLast:  keepLast,
		Directory: strings.TrimSpace(r.FormValue("backup_dir")),
	}
	out, _ := yaml.Marshal(appCfgWithBackup{Name: b.Name, Backup: cfg})
	var saveErr error
	if b.ConfigSource == "database" {
		db, cerr := storage.Connect(r.Context(), b.DB)
		if cerr != nil {
			saveErr = cerr
		} else {
			defer db.Close()
			_, saveErr = db.Pool().Exec(r.Context(), `
				INSERT INTO _onebase_config (path, content, updated_at)
				VALUES ($1, $2, now())
				ON CONFLICT (path) DO UPDATE SET content=EXCLUDED.content, updated_at=now()
			`, "config/app.yaml", out)
		}
	} else {
		dir := filepath.Join(b.Path, "config")
		os.MkdirAll(dir, 0o755)
		saveErr = os.WriteFile(filepath.Join(dir, "app.yaml"), out, 0o644)
	}
	data := h.loadCfgData(r.Context(), b, "backup")
	if saveErr != nil {
		data.Error = fmt.Sprintf("Ошибка сохранения: %s", saveErr.Error())
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = "panel-backup"
		data.BackupMessage = "Настройки бэкапа сохранены"
	}
	renderCfg(w, data)
}
