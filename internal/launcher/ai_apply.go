package launcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// applyableSubdirs — подкаталоги метаданных, куда разрешено применять
// сгенерированный каркас. Совпадает с целевыми подкаталогами kindSubdir
// (ai_generate.go): на этапе генерации каркаса создаются только метаданные.
var applyableSubdirs = map[string]bool{
	"catalogs":    true,
	"documents":   true,
	"registers":   true,
	"inforegs":    true,
	"enums":       true,
	"accounts":    true,
	"accountregs": true,
}

// winReservedNames — зарезервированные имена устройств Windows (без расширения,
// регистронезависимо). Файл с таким именем нельзя надёжно создать на Windows.
var winReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// safeConfigPath проверяет относительный slash-путь объекта каркаса перед
// записью в реальную конфигурацию: ровно «подкаталог/имя.yaml», подкаталог из
// белого списка, без обхода каталогов и без проблемных для Windows имён.
func safeConfigPath(rel string) error {
	if strings.TrimSpace(rel) == "" {
		return fmt.Errorf("пустой путь")
	}
	if rel != path.Clean(rel) || strings.Contains(rel, "..") ||
		strings.ContainsRune(rel, '\\') || strings.ContainsRune(rel, 0) {
		return fmt.Errorf("недопустимый путь: %q", rel)
	}
	parts := strings.Split(rel, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("ожидался путь вида «подкаталог/имя.yaml»: %q", rel)
	}
	subdir, fname := parts[0], parts[1]
	if !applyableSubdirs[subdir] {
		return fmt.Errorf("недопустимый подкаталог: %q", subdir)
	}
	if !strings.HasSuffix(strings.ToLower(fname), ".yaml") {
		return fmt.Errorf("ожидался .yaml-файл: %q", fname)
	}
	if strings.ContainsAny(fname, `:*?"<>|`) {
		return fmt.Errorf("недопустимое имя файла: %q", fname)
	}
	stem := strings.ToLower(strings.TrimSuffix(fname, path.Ext(fname)))
	if winReservedNames[stem] {
		return fmt.Errorf("зарезервированное имя файла: %q", fname)
	}
	return nil
}

// cfgAIApply применяет сгенерированный каркас (changes из cfgAIGenerate) в
// конфигурацию базы: проверяет каждый путь и записывает объект в нужный режим
// хранения. Новые объекты появятся в схеме данных только после миграции базы.
func (h *handler) cfgAIApply(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		Changes []GenChange `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if len(req.Changes) == 0 {
		writeJSON(w, 200, map[string]any{"error": "Нет изменений для применения"})
		return
	}
	// Сначала проверяем все пути — чтобы небезопасный путь не оставил
	// частично применённого каркаса.
	for _, ch := range req.Changes {
		if err := safeConfigPath(ch.Path); err != nil {
			writeJSON(w, 200, map[string]any{"error": "недопустимый путь " + ch.Path + ": " + err.Error()})
			return
		}
	}
	applied := 0
	for _, ch := range req.Changes {
		if err := h.writeConfigFileRaw(r.Context(), b, ch.Path, []byte(ch.NewContent)); err != nil {
			writeJSON(w, 200, map[string]any{"error": "не удалось записать " + ch.Path + ": " + err.Error(), "applied": applied})
			return
		}
		applied++
	}
	writeJSON(w, 200, map[string]any{"ok": true, "applied": applied})
}
