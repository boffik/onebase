package ui

// Экспорт списков в Excel.
// Выделено из handlers.go (план 55, этап 1) — перенос as-is.

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/ivantit66/onebase/internal/excel"
)

// listExcel exports an entity list (with current filters) as XLSX.
func (s *Server) listExcel(w http.ResponseWriter, r *http.Request) {
	entity := s.getEntity(w, r)
	if entity == nil {
		return
	}
	if !s.requirePerm(w, r, string(entity.Kind), entity.Name, "read") {
		return
	}
	params := parseListParams(r, entity, s.store.GetListPageSize(r.Context()))
	rows, err := s.store.List(r.Context(), entity.Name, entity, params)
	if err != nil {
		http.Error(w, s.errText(r, err), 500)
		return
	}
	s.resolveRefs(r.Context(), entity, rows)

	cols := make([]string, 0, len(entity.Fields))
	for _, f := range entity.Fields {
		cols = append(cols, f.Name)
	}

	xlsRows := make([][]any, len(rows))
	for i, row := range rows {
		cells := make([]any, len(cols))
		for j, col := range cols {
			cells[j] = row[col]
		}
		xlsRows[i] = cells
	}

	data, err := excel.ExportList(cols, xlsRows)
	if err != nil {
		http.Error(w, "Excel error: "+s.errText(r, err), 500)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", contentDisposition(entity.Name+".xlsx"))
	w.Write(data)
}

// contentDisposition собирает заголовок Content-Disposition по RFC 6266:
// ASCII-фолбэк в filename= (для старых клиентов) и полное имя в
// filename*=UTF-8'' (issue #46 — сырой UTF-8 в quoted-string браузеры
// декодируют как latin-1, имя файла превращается в кракозябры).
func contentDisposition(filename string) string {
	fallback := make([]rune, 0, len(filename))
	for _, r := range sanitizeFilename(filename) {
		if r < 0x80 {
			fallback = append(fallback, r)
		} else {
			fallback = append(fallback, '_')
		}
	}
	return "attachment; filename=\"" + string(fallback) + "\"; filename*=UTF-8''" + url.PathEscape(filename)
}

// sanitizeFilename replaces characters unsafe for Content-Disposition filename.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
