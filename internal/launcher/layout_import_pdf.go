package launcher

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/pdfimport"
)

// ── Импорт макета из PDF (план 64, этап 6, фаза 2) ──────────────────────────
//
// «Создать макет из PDF» рядом с «+ Печатная форма (макет)»: пользователь
// выбирает PDF (выгрузка 1С/Excel с текстовым слоем), задаёт имя формы и номер
// страницы. ImportPage извлекает черновик-скелет (сетка, тексты, спаны, границы),
// который сохраняется как printforms/<имя>.layout.yaml и открывается в редакторе.
//
// Запись переиспользует механику layout_new.go (file-mode + configdb).
// Недоверенный ввод: MaxBytesReader на 10МБ + запас; парсер pdfimport сам под
// recover/таймаутом/лимитом.

// maxPDFUpload — верхняя граница тела запроса (лимит файла 10МБ + запас на
// multipart-обёртку и поля формы).
const maxPDFUpload = pdfimport.MaxFileSize + (1 << 20)

// configuratorImportPDFLayout обрабатывает POST .../configurator/layout/import-pdf.
// Поля multipart-формы: file (PDF), name (имя макета), page (номер страницы, 1+).
func (h *handler) configuratorImportPDFLayout(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	lang := resolveLang(r)

	r.Body = http.MaxBytesReader(w, r.Body, maxPDFUpload)
	if err := r.ParseMultipartForm(maxPDFUpload); err != nil {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Файл слишком большой или форма повреждена"))
		return
	}

	layoutName := strings.TrimSpace(r.FormValue("name"))
	if layoutName == "" {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Имя макета обязательно"))
		return
	}
	if !validLayoutName(layoutName) {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Недопустимое имя файла"))
		return
	}

	page := 1
	if p := strings.TrimSpace(r.FormValue("page")); p != "" {
		if n, perr := strconv.Atoi(p); perr == nil && n >= 1 {
			page = n
		}
	}

	file, _, ferr := r.FormFile("file")
	if ferr != nil {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Выберите PDF-файл"))
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, cerr := io.Copy(&buf, file); cerr != nil {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Не удалось прочитать файл"))
		return
	}

	lt, ierr := pdfimport.ImportBytes(buf.Bytes(), page)
	if ierr != nil {
		h.layoutCreateError(w, r, b, lang, importPDFErrorMessage(lang, ierr))
		return
	}
	lt.Name = layoutName

	src, merr := marshalLayout(lt)
	if merr != nil {
		h.layoutCreateError(w, r, b, lang, tr(lang, "Ошибка создания макета")+": "+merr.Error())
		return
	}

	filename := layoutName + ".layout.yaml"
	relPath := "printforms/" + filename

	if b.ConfigSource == "database" {
		db, derr := OpenDB(r.Context(), b)
		if derr != nil {
			h.layoutCreateError(w, r, b, lang, tr(lang, "Ошибка создания макета")+": "+derr.Error())
			return
		}
		defer db.Close()
		var exists bool
		if rows, qerr := db.Query(r.Context(), `SELECT 1 FROM _onebase_config WHERE path=$1`, relPath); qerr == nil {
			exists = rows.Next()
			rows.Close()
		}
		if exists {
			h.layoutCreateError(w, r, b, lang, tr(lang, "Макет уже существует"))
			return
		}
		if _, werr := db.Exec(r.Context(), `
			INSERT INTO _onebase_config (path, content, updated_at)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (path) DO NOTHING
		`, relPath, src); werr != nil {
			h.layoutCreateError(w, r, b, lang, tr(lang, "Ошибка создания макета")+": "+werr.Error())
			return
		}
	} else {
		dir := filepath.Join(b.Path, "printforms")
		os.MkdirAll(dir, 0o755)
		fullPath := filepath.Join(dir, filename)
		if _, statErr := os.Stat(fullPath); statErr == nil {
			h.layoutCreateError(w, r, b, lang, tr(lang, "Макет уже существует"))
			return
		}
		if werr := os.WriteFile(fullPath, src, 0o644); werr != nil {
			h.layoutCreateError(w, r, b, lang, tr(lang, "Ошибка создания макета")+": "+werr.Error())
			return
		}
	}

	data := h.loadCfgData(r.Context(), b, "tree")
	data.FieldsSaved = true
	data.FieldsSavedEntity = layoutName
	data.SelectedTreeID = "mkt-" + layoutName
	renderCfg(w, r, data)
}

// importPDFErrorMessage переводит ошибку pdfimport в понятное пользователю
// сообщение (t-ключи en+de) с сохранением деталей парсера.
func importPDFErrorMessage(lang string, err error) string {
	switch {
	case errors.Is(err, pdfimport.ErrNoTextLayer):
		return tr(lang, "В PDF не найден текстовый слой: похоже, это скан или изображение. Импорт макета возможен только для PDF с текстом (выгрузка из 1С/Excel).")
	case errors.Is(err, pdfimport.ErrPageNotFound):
		return tr(lang, "Указанной страницы нет в документе.")
	case errors.Is(err, pdfimport.ErrFileTooLarge):
		return tr(lang, "Файл больше 10 МБ — слишком большой для импорта.")
	case errors.Is(err, pdfimport.ErrParse):
		return tr(lang, "Не удалось разобрать PDF (возможно, файл повреждён или зашифрован).")
	default:
		return tr(lang, "Ошибка импорта PDF") + ": " + err.Error()
	}
}
