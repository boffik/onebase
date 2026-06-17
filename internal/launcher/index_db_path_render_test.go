package launcher

import (
	"bytes"
	"strings"
	"testing"
)

// renderIndex рендерит страницу списка баз ("page-index") с заданными базами
// и выбранной базой — для проверки того, что попадает в HTML списка и info-панель.
func renderIndex(t *testing.T, bases []*baseVM, selected *baseVM) string {
	t.Helper()
	var buf bytes.Buffer
	data := map[string]any{
		"Title":    "test",
		"Lang":     "ru",
		"Bases":    bases,
		"Selected": selected,
		"BaseURL":  "",
	}
	if err := tmpl.ExecuteTemplate(&buf, "page-index", data); err != nil {
		t.Fatalf("ExecuteTemplate page-index: %v", err)
	}
	return buf.String()
}

// Для базы с файловой БД (SQLite) в списке должен быть виден путь к файлу .db,
// а не пустой DSN — иначе неясно, где лежат данные.
func TestIndex_SQLiteListShowsDBFilePath(t *testing.T) {
	vm := &baseVM{Base: &Base{
		ID: "s1", Name: "Файловая", ConfigSource: "file",
		Path: `C:\proj\app`, DBType: "sqlite", DBPath: `C:\onebase\app.db`, Port: 8080,
	}}
	html := renderIndex(t, []*baseVM{vm}, vm)
	if !strings.Contains(html, `C:\onebase\app.db`) {
		t.Errorf("в списке нет пути к файлу SQLite (DBPath)")
	}
	if !strings.Contains(html, "💾") {
		t.Errorf("у строки данных SQLite нет иконки-маркера 💾")
	}
}

// В info-панели справа для выбранной SQLite-базы должна быть строка с путём
// к файлу базы под отдельной меткой «Файл базы».
func TestIndex_SQLiteInfoPanelShowsDBFile(t *testing.T) {
	vm := &baseVM{Base: &Base{
		ID: "s1", Name: "Файловая", ConfigSource: "file",
		Path: `C:\proj\app`, DBType: "sqlite", DBPath: `C:\onebase\app.db`, Port: 8080,
	}}
	html := renderIndex(t, []*baseVM{vm}, vm)
	idx := strings.Index(html, `class="info-panel"`)
	if idx < 0 {
		t.Fatalf("info-панель не отрендерилась")
	}
	panel := html[idx:]
	if !strings.Contains(panel, "Файл базы") {
		t.Errorf("в info-панели нет метки «Файл базы»")
	}
	if !strings.Contains(panel, `C:\onebase\app.db`) {
		t.Errorf("в info-панели нет пути к файлу SQLite")
	}
}

// Для PostgreSQL поведение не меняется: в списке видна маскированная строка
// подключения, иконка-маркер файла 💾 не добавляется.
func TestIndex_PostgresListShowsMaskedDSN(t *testing.T) {
	vm := &baseVM{Base: &Base{
		ID: "p1", Name: "Серверная", ConfigSource: "database",
		DBType: "postgres", DB: "postgres://user:secret@host/db", Port: 8081,
	}}
	html := renderIndex(t, []*baseVM{vm}, vm)
	if !strings.Contains(html, "postgres://user:***@host/db") {
		t.Errorf("в списке нет маскированной строки подключения PostgreSQL")
	}
	if strings.Contains(html, "💾") {
		t.Errorf("для PostgreSQL не должно быть иконки-маркера файла 💾")
	}
}
