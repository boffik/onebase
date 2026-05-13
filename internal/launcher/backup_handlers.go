package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/backup"
	"github.com/ivantit66/onebase/internal/configdb"
	"github.com/ivantit66/onebase/internal/storage"
	"gopkg.in/yaml.v3"
)

func (h *handler) backupDir(b *Base) string {
	custom := h.loadBackupDirSetting(b)
	if custom != "" {
		return custom
	}
	if b.Path != "" {
		return filepath.Join(b.Path, "backups")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".onebase", "backups", b.ID)
}

func (h *handler) loadBackupDirSetting(b *Base) string {
	if b.ConfigSource == "database" {
		db, err := storage.Connect(context.Background(), b.DB)
		if err != nil {
			return ""
		}
		defer db.Close()
		var content []byte
		if err := db.Pool().QueryRow(context.Background(),
			"SELECT content FROM _onebase_config WHERE path='config/app.yaml'").Scan(&content); err != nil {
			return ""
		}
		var tmp struct {
			Backup struct {
				Directory string `yaml:"directory"`
			} `yaml:"backup"`
		}
		yaml.Unmarshal(content, &tmp)
		return tmp.Backup.Directory
	}
	cfgPath := filepath.Join(b.Path, "config", "app.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return ""
	}
	var tmp struct {
		Backup struct {
			Directory string `yaml:"directory"`
		} `yaml:"backup"`
	}
	yaml.Unmarshal(raw, &tmp)
	return tmp.Backup.Directory
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

func (h *handler) backupUpload(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	dir := h.backupDir(b)
	os.MkdirAll(dir, 0o755)

	file, header, err := r.FormFile("backup_file")
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Ошибка загрузки: " + err.Error()
		renderCfg(w, data)
		return
	}
	defer file.Close()

	name := header.Filename
	outPath := filepath.Join(dir, name)
	f, err := os.Create(outPath)
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Ошибка сохранения: " + err.Error()
		renderCfg(w, data)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	data := h.loadCfgData(r.Context(), b, "backup")
	data.FieldsSaved = true
	data.FieldsSavedEntity = "panel-backup"
	data.BackupMessage = "Файл загружен: " + name
	renderCfg(w, data)
}

func (h *handler) backupRestore(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file := chi.URLParam(r, "file")
	dir := h.backupDir(b)
	fp := filepath.Join(dir, file)
	if _, err := os.Stat(fp); err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Файл не найден: " + file
		renderCfg(w, data)
		return
	}

	restoreErr := backup.Restore(r.Context(), b.DB, fp)
	data := h.loadCfgData(r.Context(), b, "backup")
	if restoreErr != nil {
		data.Error = "Ошибка восстановления: " + restoreErr.Error()
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = "panel-backup"
		data.BackupMessage = "База данных восстановлена из: " + file
	}
	renderCfg(w, data)
}

// backupFullExport creates a single .obz file containing both database dump and configuration.
func (h *handler) backupFullExport(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Database dump
	tmpDir, err := os.MkdirTemp("", "onebase-obz-dump-*")
	if err != nil {
		http.Error(w, "Temp dir error: "+err.Error(), 500)
		return
	}
	defer os.RemoveAll(tmpDir)

	dumpPath, dumpErr := backup.Dump(r.Context(), b.DB, tmpDir)
	if dumpErr != nil {
		http.Error(w, "Dump error: "+dumpErr.Error(), 500)
		return
	}

	dumpData, err := os.ReadFile(dumpPath)
	if err != nil {
		http.Error(w, "Read dump error: "+err.Error(), 500)
		return
	}
	f, _ := zw.Create("database.sql.gz")
	f.Write(dumpData)

	// Configuration
	if b.ConfigSource == "database" {
		db, cerr := storage.Connect(r.Context(), b.DB)
		if cerr == nil {
			defer db.Close()
			rows, qerr := db.Pool().Query(r.Context(), `SELECT path, content FROM _onebase_config ORDER BY path`)
			if qerr == nil {
				defer rows.Close()
				for rows.Next() {
					var p string
					var content []byte
					if rows.Scan(&p, &content) != nil {
						continue
					}
					cf, _ := zw.Create("config/" + strings.ReplaceAll(p, `\`, "/"))
					cf.Write(content)
				}
			}
		}
	} else {
		srcDir := b.Path
		filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(srcDir, path)
			rel = strings.ReplaceAll(rel, `\`, "/")
			if strings.HasPrefix(rel, "backups/") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			cf, _ := zw.Create("config/" + rel)
			cf.Write(content)
			return nil
		})
	}

	// Metadata
	meta := fmt.Sprintf("onebase_full_export\nversion=1.0\ndate=%s\nbase=%s\nsource=%s\n",
		time.Now().Format("2006-01-02T15:04:05"), b.Name, b.ConfigSource)
	mf, _ := zw.Create("META.txt")
	mf.Write([]byte(meta))

	zw.Close()

	name := b.Name + "_" + time.Now().Format("2006-01-02_15-04") + ".obz"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	w.Write(buf.Bytes())
}

// backupFullImport restores both database and configuration from a .obz file.
func (h *handler) backupFullImport(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	file, _, err := r.FormFile("obz_file")
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Ошибка загрузки файла: " + err.Error()
		renderCfg(w, data)
		return
	}
	defer file.Close()

	dtData, err := io.ReadAll(file)
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Ошибка чтения файла: " + err.Error()
		renderCfg(w, data)
		return
	}

	reader, err := zip.NewReader(bytes.NewReader(dtData), int64(len(dtData)))
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Неверный формат файла .obz: " + err.Error()
		renderCfg(w, data)
		return
	}

	tmpDir, err := os.MkdirTemp("", "onebase-obz-import-*")
	if err != nil {
		data := h.loadCfgData(r.Context(), b, "backup")
		data.Error = "Temp dir error: " + err.Error()
		renderCfg(w, data)
		return
	}
	defer os.RemoveAll(tmpDir)

	var dumpFile string
	var configDir string

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			os.MkdirAll(filepath.Join(tmpDir, f.Name), 0o755)
			continue
		}
		outPath := filepath.Join(tmpDir, f.Name)
		os.MkdirAll(filepath.Dir(outPath), 0o755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if f.Name == "database.sql.gz" {
			dumpFile = outPath
		}
		if strings.HasPrefix(f.Name, "config/") && configDir == "" {
			configDir = filepath.Join(tmpDir, "config")
		}
	}

	// Restore database
	var restoreErr error
	if dumpFile != "" {
		restoreErr = backup.Restore(r.Context(), b.DB, dumpFile)
	} else {
		restoreErr = fmt.Errorf("файл database.sql.gz не найден в архиве")
	}

	// Import configuration
	var configErr error
	if configDir != "" {
		if b.ConfigSource == "database" {
			db, cerr := storage.Connect(r.Context(), b.DB)
			if cerr != nil {
				configErr = cerr
			} else {
				defer db.Close()
				repo := configdb.New(db.Pool())
				configErr = repo.ImportFromDir(r.Context(), configDir)
			}
		} else {
			filepath.WalkDir(configDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(configDir, path)
				dst := filepath.Join(b.Path, rel)
				os.MkdirAll(filepath.Dir(dst), 0o755)
				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				os.WriteFile(dst, content, 0o644)
				return nil
			})
		}
	}

	if configErr == nil {
		h.runner.MigrateBase(r.Context(), b)
	}

	data := h.loadCfgData(r.Context(), b, "backup")
	if restoreErr != nil {
		data.Error = "Ошибка восстановления БД: " + restoreErr.Error()
	} else if configErr != nil {
		data.Error = "Ошибка импорта конфигурации: " + configErr.Error()
	} else {
		data.FieldsSaved = true
		data.FieldsSavedEntity = "panel-backup"
		data.BackupMessage = "Полное восстановление выполнено: база данных + конфигурация"
	}
	renderCfg(w, data)
}
