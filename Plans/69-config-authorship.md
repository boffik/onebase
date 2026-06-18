# План 69: Авторство и лицензирование конфигураций

> **Для агентов-исполнителей:** ОБЯЗАТЕЛЬНЫЙ САБ-НАВЫК — используйте
> `superpowers:subagent-driven-development` (рекомендуется) или
> `superpowers:executing-plans` для пошагового исполнения. Шаги размечены
> чекбоксами (`- [ ]`).

**Цель:** Дать конфигурациям OneBase явные поля авторства и лицензии
(`author` / `copyright` / `license` в `config/app.yaml`), показать их вместе с
правообладателем платформы на экране «О программе» и в конфигураторе, чтобы
форк/поставка клиенту имели юридически определённое авторство.

**Архитектура:** Поля добавляются в `project.AppConfig` (парсинг YAML),
пробрасываются в `ui.Config` (через `run.go`/`dev.go`) и в `configuratorData`
(конфигуратор). Отображение — существующий шаблон `tplAbout` и существующая форма
свойств конфигурации в конфигураторе. Правообладатель платформы выносится в
константы пакета `version`. Новых страниц/роутов не создаётся — расширяются
существующие.

**Стек:** Go 1.23+, `gopkg.in/yaml.v3`, `html/template`, chi-роутинг,
i18n-бандл (`internal/i18n`).

---

## Что уже есть (проверено при составлении плана — НЕ переделывать)

- `LICENSE` — MIT с правообладателем: `Copyright (c) 2026 Иван Титов`.
- `README.md` — секция «## Лицензия» с `© 2026 Иван Титов` и дисклеймером 1С.
- Экран «О программе» — `internal/ui/templates.go`, шаблон `tplAbout`, роут
  `/ui/about` (`internal/ui/server.go:321`, хендлер `internal/ui/handlers_home.go:18`).
  Показывает: версию платформы, имя/версию конфигурации, БД, счётчики метаданных.

## Чего нет (закрывает этот план)

1. Поля `author`/`copyright`/`license` в `config/app.yaml` (`AppConfig` имеет
   только `name`/`version`/`lang`/`logo`/`email`/`attachments`/`demo`/`llm`/`webhooks`).
2. Правообладатель платформы на экране «О программе» (сейчас только `onebase <ver>`).
3. Поля авторства в редакторе свойств конфигурации (конфигуратор).

## Грабли проекта (учесть при исполнении)

- **i18ncheck**: каждый новый `{{t $.Lang "..."}}` обязан иметь перевод в
  `internal/i18n/locales/en.json`, иначе pre-commit и CI (job build) падают.
- **Windows-блокировка бинаря**: перед `go build` остановить сервер —
  `taskkill /IM onebase.exe /F` (иначе сборка молча не обновит exe).
- **gofmt + CRLF**: `gofmt -l` ложно срабатывает на CRLF; проверять `gofmt -d`.

---

## Карта файлов

| Файл | Что меняется |
|---|---|
| `internal/project/loader.go` | +3 поля в `AppConfig` |
| `internal/project/author_test.go` | **новый** — тест парсинга полей |
| `internal/version/version.go` | +константы `Author`, `License`, `Year` |
| `internal/ui/server.go` | +поля в `ui.Config` |
| `internal/cli/run.go` | проброс полей в `uiCfg` |
| `internal/cli/dev.go` | проброс полей в `uiCfg` |
| `internal/ui/templates.go` | строки авторства в `tplAbout` |
| `internal/launcher/configurator.go` | `configuratorData` + `loadCfgData` + `configuratorSaveApp` + `saveAppConfig` |
| `internal/launcher/configurator_tmpl.go` | поля формы свойств конфигурации |
| `internal/i18n/locales/en.json` | переводы новых ключей |
| `README.md` | секция «## Авторы» |
| `docs/features.md` | секция о возможности (status: testing) |
| `examples/tasks/config/app.yaml` | демонстрация полей |
| `CLAUDE.md` | (опц., Задача 8) формат git-trailer |

---

## Задача 1: Поля авторства в `AppConfig`

**Файлы:**
- Modify: `internal/project/loader.go:90-107` (struct `AppConfig`)
- Test: `internal/project/author_test.go` (создать)

- [ ] **Шаг 1: Написать падающий тест**

Создать `internal/project/author_test.go`:

```go
package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_Authorship проверяет, что author/copyright/license из app.yaml
// разбираются в AppConfig (план 69).
func TestLoadConfig_Authorship(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appYAML := `name: Demo
version: "1.0"
author: Иван Титов
copyright: © 2026 ООО «Ромашка»
license: MIT
`
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte(appYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author != "Иван Титов" {
		t.Errorf("Author = %q, want %q", cfg.Author, "Иван Титов")
	}
	if cfg.Copyright != "© 2026 ООО «Ромашка»" {
		t.Errorf("Copyright = %q", cfg.Copyright)
	}
	if cfg.License != "MIT" {
		t.Errorf("License = %q, want MIT", cfg.License)
	}
}

// TestLoadConfig_AuthorshipOptional — поля необязательны: без них пусто, без ошибок.
func TestLoadConfig_AuthorshipOptional(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte("name: Bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author != "" || cfg.Copyright != "" || cfg.License != "" {
		t.Errorf("ожидались пустые поля авторства, получено: %q/%q/%q", cfg.Author, cfg.Copyright, cfg.License)
	}
}
```

- [ ] **Шаг 2: Запустить тест — убедиться, что не компилируется/падает**

Run: `go test ./internal/project/ -run TestLoadConfig_Authorship`
Expected: FAIL — `cfg.Author undefined (type *AppConfig has no field Author)`

- [ ] **Шаг 3: Добавить поля в `AppConfig`**

В `internal/project/loader.go` в struct `AppConfig` после строки
`Version     string             ` + "`yaml:\"version\"`" добавить:

```go
	// Авторство и лицензия конфигурации (план 69). Необязательны. Едут вместе
	// с конфигурацией (app.yaml попадает в файл/в _onebase_config/в .obz) —
	// чтобы форк/поставка клиенту имели определённого правообладателя.
	Author    string `yaml:"author,omitempty"`
	Copyright string `yaml:"copyright,omitempty"`
	License   string `yaml:"license,omitempty"`
```

- [ ] **Шаг 4: Запустить тест — должен пройти**

Run: `go test ./internal/project/ -run TestLoadConfig_Authorship`
Expected: PASS (оба теста — `_Authorship` и `_AuthorshipOptional`)

- [ ] **Шаг 5: Коммит**

```bash
git add internal/project/loader.go internal/project/author_test.go
git commit -m "feat(config): поля author/copyright/license в app.yaml"
```

---

## Задача 2: Константы правообладателя платформы

**Файлы:**
- Modify: `internal/version/version.go`

- [ ] **Шаг 1: Прочитать текущий файл**

Run: открыть `internal/version/version.go` (там объявлен `Build` и `func String()`).

- [ ] **Шаг 2: Добавить константы**

В `internal/version/version.go` добавить после объявления `Build`:

```go
// Правообладатель и лицензия платформы (план 69). Единый источник для экрана
// «О программе». Совпадают с файлом LICENSE в корне репозитория.
const (
	Author  = "Иван Титов"
	License = "MIT"
	Year    = "2026"
)
```

- [ ] **Шаг 3: Проверить компиляцию**

Run: `go build ./internal/version/`
Expected: без ошибок.

- [ ] **Шаг 4: Коммит**

```bash
git add internal/version/version.go
git commit -m "feat(version): константы правообладателя платформы"
```

---

## Задача 3: Проброс полей в `ui.Config`

**Файлы:**
- Modify: `internal/ui/server.go:27-51` (struct `Config`)
- Modify: `internal/cli/run.go:231-245`
- Modify: `internal/cli/dev.go:201-208`

- [ ] **Шаг 1: Добавить поля в `ui.Config`**

В `internal/ui/server.go` в struct `Config` после `AppVersion    string` добавить:

```go
	AppAuthor     string // автор конфигурации (app.yaml: author)
	AppCopyright  string // правообладатель конфигурации (app.yaml: copyright)
	AppLicense    string // лицензия конфигурации (app.yaml: license)
	PlatAuthor    string // правообладатель платформы (version.Author)
	PlatLicense   string // лицензия платформы (version.License)
```

- [ ] **Шаг 2: Заполнить в `run.go`**

В `internal/cli/run.go` блок `uiCfg := ui.Config{...}` (строка ~231) дополнить
полем платформы, а блок `if appCfg != nil {` (строка ~235) — полями конфигурации:

Заменить:
```go
	uiCfg := ui.Config{
		DSN:         dsn,
		PlatVersion: version.String(),
	}
	if appCfg != nil {
		uiCfg.AppName = appCfg.Name
		uiCfg.AppVersion = appCfg.Version
		uiCfg.Lang = appCfg.Lang
```
на:
```go
	uiCfg := ui.Config{
		DSN:         dsn,
		PlatVersion: version.String(),
		PlatAuthor:  version.Author,
		PlatLicense: version.License,
	}
	if appCfg != nil {
		uiCfg.AppName = appCfg.Name
		uiCfg.AppVersion = appCfg.Version
		uiCfg.AppAuthor = appCfg.Author
		uiCfg.AppCopyright = appCfg.Copyright
		uiCfg.AppLicense = appCfg.License
		uiCfg.Lang = appCfg.Lang
```

- [ ] **Шаг 3: Заполнить в `dev.go`**

В `internal/cli/dev.go` (строка ~201) заменить:
```go
	uiCfg := ui.Config{DSN: dsn, PlatVersion: version.String()}
	if appCfg != nil {
		uiCfg.AppName = appCfg.Name
		uiCfg.AppVersion = appCfg.Version
		uiCfg.Lang = appCfg.Lang
```
на:
```go
	uiCfg := ui.Config{DSN: dsn, PlatVersion: version.String(), PlatAuthor: version.Author, PlatLicense: version.License}
	if appCfg != nil {
		uiCfg.AppName = appCfg.Name
		uiCfg.AppVersion = appCfg.Version
		uiCfg.AppAuthor = appCfg.Author
		uiCfg.AppCopyright = appCfg.Copyright
		uiCfg.AppLicense = appCfg.License
		uiCfg.Lang = appCfg.Lang
```

- [ ] **Шаг 4: Проверить сборку**

Run (Windows — сначала остановить сервер): `taskkill /IM onebase.exe /F 2>NUL & go build -o onebase.exe ./cmd/onebase`
Expected: сборка успешна, ошибок нет.

- [ ] **Шаг 5: Коммит**

```bash
git add internal/ui/server.go internal/cli/run.go internal/cli/dev.go
git commit -m "feat(ui): проброс авторства конфигурации и платформы в Config"
```

---

## Задача 4: Отображение авторства на экране «О программе»

**Файлы:**
- Modify: `internal/ui/templates.go` (шаблон `tplAbout`, строки ~2631-2646)
- Modify: `internal/i18n/locales/en.json`

- [ ] **Шаг 1: Добавить строку правообладателя платформы**

В `internal/ui/templates.go` найти строку «Версия платформы» (в `tplAbout`):
```html
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;width:180px;font-size:14px">{{t $.Lang "Версия платформы"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-weight:600;font-size:14px">onebase {{if .Cfg.PlatVersion}}{{.Cfg.PlatVersion}}{{else}}dev{{end}}</td>
    </tr>
```
и сразу ПОСЛЕ этого `</tr>` вставить:
```html
    {{if .Cfg.PlatAuthor}}
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;font-size:14px">{{t $.Lang "Правообладатель платформы"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-size:14px">{{.Cfg.PlatAuthor}}{{if .Cfg.PlatLicense}} · {{.Cfg.PlatLicense}}{{end}}</td>
    </tr>
    {{end}}
```

- [ ] **Шаг 2: Добавить строки авторства конфигурации**

В том же шаблоне найти блок «Версия конфигурации»:
```html
    {{if .Cfg.AppVersion}}
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;font-size:14px">{{t $.Lang "Версия конфигурации"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-size:14px">{{.Cfg.AppVersion}}</td>
    </tr>
    {{end}}
```
и сразу ПОСЛЕ `{{end}}` вставить:
```html
    {{if .Cfg.AppAuthor}}
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;font-size:14px">{{t $.Lang "Автор конфигурации"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-size:14px">{{.Cfg.AppAuthor}}</td>
    </tr>
    {{end}}
    {{if .Cfg.AppCopyright}}
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;font-size:14px">{{t $.Lang "Правообладатель"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-size:14px">{{.Cfg.AppCopyright}}</td>
    </tr>
    {{end}}
    {{if .Cfg.AppLicense}}
    <tr>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;color:#64748b;font-size:14px">{{t $.Lang "Лицензия конфигурации"}}</td>
      <td style="padding:14px 0;border-bottom:1px solid #f1f5f9;font-size:14px">{{.Cfg.AppLicense}}</td>
    </tr>
    {{end}}
```

- [ ] **Шаг 3: Добавить переводы в `en.json`**

В `internal/i18n/locales/en.json` добавить ключи (рядом с существующими
`"Версия конфигурации"`):
```json
  "Правообладатель платформы": "Platform copyright holder",
  "Автор конфигурации": "Configuration author",
  "Правообладатель": "Copyright holder",
  "Лицензия конфигурации": "Configuration license",
```
> Перед вставкой проверить грепом, что ключей ещё нет, и не сломать запятые JSON.

- [ ] **Шаг 4: Проверить переводы и сборку**

Run: `go build -o onebase.exe ./cmd/onebase` (после `taskkill /IM onebase.exe /F`)
Run: `go run ./cmd/onebase i18n check` *(если такой команды нет — пропустить; pre-commit i18ncheck отчитается при коммите)*
Expected: сборка успешна; новые ключи присутствуют в en.json.

- [ ] **Шаг 5: Ручная проверка отображения**

Run: `onebase dev --project ./examples/tasks --sqlite ./_tmp69.db --port 8099`
Открыть `http://localhost:8099/ui/about` → системное меню → «О программе».
Expected: строка «Правообладатель платформы: Иван Титов · MIT» видна.
(Авторство конфигурации появится после Задачи 6, когда в примере заданы поля.)
Остановить сервер, удалить `_tmp69.db`.

- [ ] **Шаг 6: Коммит**

```bash
git add internal/ui/templates.go internal/i18n/locales/en.json
git commit -m "feat(ui): авторство и лицензия на экране «О программе»"
```

---

## Задача 5: Поля авторства в конфигураторе

**Файлы:**
- Modify: `internal/launcher/configurator.go:306-311` (`configuratorData`)
- Modify: `internal/launcher/configurator.go:571-576` (`loadCfgData`)
- Modify: `internal/launcher/configurator.go:3179-3181` (`configuratorSaveApp`)
- Modify: `internal/launcher/configurator.go:3253-3259` (`saveAppConfig`)
- Modify: `internal/launcher/configurator_tmpl.go:4983-4994` (форма)
- Modify: `internal/i18n/locales/en.json`

> **КРИТИЧНО:** `saveAppConfig` маршалит `app.yaml` из своих полей и перезаписывает
> файл. Если не добавить туда новые поля — конфигуратор будет **затирать**
> авторство при каждом сохранении свойств. (То же ограничение уже касается
> секций `llm`/`email`/`webhooks` — они вне scope этого плана, лишь отмечаем.)

- [ ] **Шаг 1: Поля в `configuratorData`**

В `internal/launcher/configurator.go` в struct `configuratorData` после
`AppLang          string` добавить:
```go
	AppAuthor        string
	AppCopyright     string
	AppLicense       string
```

- [ ] **Шаг 2: Заполнение в `loadCfgData`**

В `internal/launcher/configurator.go` блок (строка ~571):
```go
	if appCfg, _ := project.LoadConfig(proj.Dir); appCfg != nil {
		data.AppName = appCfg.Name
		data.AppVersion = appCfg.Version
		data.AppLogo = appCfg.Logo
		data.AppLang = appCfg.Lang
	}
```
дополнить тремя присваиваниями внутри `if`:
```go
		data.AppAuthor = appCfg.Author
		data.AppCopyright = appCfg.Copyright
		data.AppLicense = appCfg.License
```

- [ ] **Шаг 3: Чтение формы в `configuratorSaveApp`**

В `internal/launcher/configurator.go` после строки (~3181)
`newLang := strings.TrimSpace(r.FormValue("app_lang"))` добавить:
```go
	newAuthor := strings.TrimSpace(r.FormValue("app_author"))
	newCopyright := strings.TrimSpace(r.FormValue("app_copyright"))
	newLicense := strings.TrimSpace(r.FormValue("app_license"))
```

- [ ] **Шаг 4: Сохранение в `saveAppConfig`**

В `internal/launcher/configurator.go` (строка ~3253) заменить:
```go
	type saveAppConfig struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version,omitempty"`
		Lang    string `yaml:"lang,omitempty"`
		Logo    string `yaml:"logo,omitempty"`
	}
	out, _ := yaml.Marshal(saveAppConfig{Name: newName, Version: newVersion, Lang: newLang, Logo: logoPath})
```
на:
```go
	type saveAppConfig struct {
		Name      string `yaml:"name"`
		Version   string `yaml:"version,omitempty"`
		Lang      string `yaml:"lang,omitempty"`
		Logo      string `yaml:"logo,omitempty"`
		Author    string `yaml:"author,omitempty"`
		Copyright string `yaml:"copyright,omitempty"`
		License   string `yaml:"license,omitempty"`
	}
	out, _ := yaml.Marshal(saveAppConfig{
		Name: newName, Version: newVersion, Lang: newLang, Logo: logoPath,
		Author: newAuthor, Copyright: newCopyright, License: newLicense,
	})
```

- [ ] **Шаг 5: Поля в форме конфигуратора**

В `internal/launcher/configurator_tmpl.go` после блока «Версия» (заканчивается
на `<input type="text" name="app_version" ...>` и его `</div>`, строка ~4986)
вставить:
```html
      <div class="fg" style="margin-top:10px">
        <label>{{t $.Lang "Автор"}}</label>
        <input type="text" name="app_author" value="{{.AppAuthor}}" placeholder="{{t $.Lang "ФИО или организация"}}">
        <div class="hint">{{t $.Lang "Указывается на экране «О программе» в пользовательском режиме"}}</div>
      </div>
      <div class="fg" style="margin-top:10px">
        <label>{{t $.Lang "Правообладатель"}}</label>
        <input type="text" name="app_copyright" value="{{.AppCopyright}}" placeholder="© 2026 ...">
      </div>
      <div class="fg" style="margin-top:10px">
        <label>{{t $.Lang "Лицензия конфигурации"}}</label>
        <input type="text" name="app_license" value="{{.AppLicense}}" placeholder="MIT / проприетарная / ...">
      </div>
```

- [ ] **Шаг 6: Переводы в `en.json`**

Добавить отсутствующие ключи (проверить грепом, `"Правообладатель"` и
`"Лицензия конфигурации"` уже добавлены в Задаче 4 — не дублировать):
```json
  "Автор": "Author",
  "ФИО или организация": "Name or organization",
  "Указывается на экране «О программе» в пользовательском режиме": "Shown on the About screen in user mode",
```

- [ ] **Шаг 7: Сборка**

Run: `taskkill /IM onebase.exe /F 2>NUL & go build -tags webview -ldflags="-H windowsgui" -o onebase-gui.exe ./cmd/onebase`
Expected: GUI-бинарь собирается без ошибок.

- [ ] **Шаг 8: Ручная проверка цикла сохранения**

Запустить лаунчер, открыть конфигуратор базы → панель «Конфигурация» →
заполнить «Автор», «Правообладатель», «Лицензия» → «Сохранить». Затем открыть
панель ещё раз — значения должны сохраниться (а не обнулиться). Проверить, что
`config/app.yaml` (или запись в `_onebase_config`) содержит `author:`/
`copyright:`/`license:` и при этом `name:`/`version:` не потеряны.

- [ ] **Шаг 9: Коммит**

```bash
git add internal/launcher/configurator.go internal/launcher/configurator_tmpl.go internal/i18n/locales/en.json
git commit -m "feat(configurator): редактирование автора/правообладателя/лицензии конфигурации"
```

---

## Задача 6: Демонстрация в примере + документация фичи

**Файлы:**
- Modify: `examples/tasks/config/app.yaml`
- Modify: `docs/features.md`

- [ ] **Шаг 1: Заполнить поля в примере `tasks`**

Заменить содержимое `examples/tasks/config/app.yaml`:
```yaml
name: Tasks — Таск-трекер
version: "1.0"
```
на:
```yaml
name: Tasks — Таск-трекер
version: "1.0"
author: Иван Титов
copyright: © 2026 Иван Титов
license: MIT
```

- [ ] **Шаг 2: Проверить пример**

Run: `onebase check --project ./examples/tasks`
Expected: валидация без ошибок (поля авторства не влияют на компиляцию).

- [ ] **Шаг 3: Секция в `docs/features.md`**

Добавить в `docs/features.md`:
```markdown
## Авторство и лицензия конфигурации
<!-- status: testing -->
<!-- since: build-69 -->
<!-- date: 2026-06-18 -->

В свойствах конфигурации (Конфигуратор → панель «Конфигурация») появились поля
«Автор», «Правообладатель» и «Лицензия конфигурации». Они хранятся в
`config/app.yaml` (`author` / `copyright` / `license`), едут вместе с
конфигурацией при выгрузке/загрузке и показываются пользователю на экране
«О программе» рядом с правообладателем платформы. Нужно, чтобы форк или
поставка конфигурации клиенту имели юридически определённого правообладателя.
```

- [ ] **Шаг 4: Коммит**

```bash
git add examples/tasks/config/app.yaml docs/features.md
git commit -m "docs(config): пример авторства в tasks + features.md"
```

---

## Задача 7: Секция «Авторы» в README

**Файлы:**
- Modify: `README.md` (перед/рядом с секцией «## Лицензия», строка ~285)

- [ ] **Шаг 1: Добавить секцию**

В `README.md` перед `## Лицензия` вставить:
```markdown
## Авторы

Платформа OneBase — © 2026 Иван Титов. Код написан Иваном Титовым, в том числе
с использованием инструментов разработки на базе ИИ (Claude Code). Согласно
позиции разработчиков таких инструментов и применимому законодательству
(в РФ — ст. 1257 ГК РФ), ИИ-ассистент является инструментом, а права на
созданный по заданиям человека код принадлежат автору задания. Записи вида
`Co-Authored-By` / `Generated-with` в истории git — пометки об использованном
инструменте, а не передача авторских прав.

Авторство и лицензия конкретной конфигурации задаются в её свойствах
(`config/app.yaml`: `author` / `copyright` / `license`) и видны в
пользовательском режиме на экране «О программе».

---
```

- [ ] **Шаг 2: Коммит**

```bash
git add README.md
git commit -m "docs: секция «Авторы» в README"
```

---

## Задача 8 (опциональная — требует явного согласия пользователя): git-trailer

> Меняет шаблон будущих коммитов всего проекта. **Историю не переписываем**
> (force-push теряет аннотации и ломает чужие клоны). MIT-лицензия от Ивана
> Титова уже покрывает весь код независимо от trailer — это косметика/гигиена,
> не юридическая необходимость.

**Файлы:**
- Modify: `CLAUDE.md`

- [ ] **Шаг 1: Добавить правило о trailer**

В `CLAUDE.md` в раздел «## Соглашения» добавить пункт:
```markdown
- **Авторство коммитов:** ИИ-ассистент — инструмент, не соавтор. В трейлере
  коммита используйте `Generated-with: Claude Code`, а не `Co-Authored-By: ...`.
  Правообладатель кода — автор задания (Иван Титов).
```

- [ ] **Шаг 2: Коммит**

```bash
git add CLAUDE.md
git commit -m "docs: формат git-trailer — инструмент, а не соавтор"
```

---

## Финальная проверка (после всех задач)

- [ ] `go build -o onebase.exe ./cmd/onebase` — успешно (сервер остановлен).
- [ ] `go build -tags webview -ldflags="-H windowsgui" -o onebase-gui.exe ./cmd/onebase` — успешно.
- [ ] `go test ./internal/project/ ./internal/ui/ ./internal/launcher/` — зелёные.
- [ ] `onebase check --project ./examples/tasks` — без ошибок.
- [ ] `gofmt -d internal/project internal/ui internal/cli internal/launcher internal/version` — пусто (помнить про CRLF-ложные срабатывания).
- [ ] Экран «О программе» в примере `tasks` показывает: правообладатель платформы
      (Иван Титов · MIT) и авторство конфигурации (Иван Титов / © 2026 Иван Титов / MIT).
- [ ] Конфигуратор сохраняет и перечитывает поля авторства без потери name/version.

## Самопроверка плана (выполнено при составлении)

- **Покрытие предложенных пунктов:** поле авторства конфигурации (Задачи 1,5) ✓;
  экран «О программе» (Задача 4) ✓; git-trailer (Задача 8) ✓; README (Задача 7) ✓.
- **Консистентность имён:** поля YAML `author/copyright/license`; поля
  `ui.Config` — `AppAuthor/AppCopyright/AppLicense` + `PlatAuthor/PlatLicense`;
  поля `configuratorData` — `AppAuthor/AppCopyright/AppLicense`; имена форм —
  `app_author/app_copyright/app_license`. Совпадают между задачами.
- **i18n:** все новые `t`-ключи имеют перевод в `en.json` (Задачи 4 и 5);
  `"Правообладатель"` и `"Лицензия конфигурации"` вводятся один раз (Задача 4),
  переиспользуются в Задаче 5 — дублей в JSON быть не должно.
