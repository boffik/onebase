# Печатные формы v2 — рефакторинг механизма + PDF-импорт

## Context

Запрос пользователя: «Полный рефакторинг механизма печатных форм. Я должен быть способен конфигурировать такие формы [как УПД] + конвертер из PDF, чтобы получалась готовая печатная форма». Бенчмарк — реальный УПД (`c:\Users\ibrog\Desktop\1с счета\УПД (статус 2)...pdf`): многоуровневая шапка с rowspan/colspan, границы по сторонам ячеек, шрифты 6–8pt, ландшафт, повтор строк по данным, итоги, подписи.

Текущее состояние (разведка подтверждена):
- **Два конфликтующих механизма**: YAML-формы (`printform.PrintForm` — одна таблица, `Итог.Поле` сломан, нет картинок) и DSL-формы (`.os` + `LayoutTemplate` + `ТабличныйДокумент`); коллизия имён разрешается молча (registry.go:319).
- **PDF фактически сломан**: fpdf транслитерирует кириллицу в латиницу (`latinize`, internal/printform/pdf.go); DSL-формы серверного PDF не имеют вовсе.
- **Макет ограничен**: `Areas` — map (порядок не гарантирован), границы только пресетами, нет page setup; картинки/разрывы страниц/повтор шапки хранятся, но не рендерятся (spreadsheet_document.go).
- **Визуальный редактор есть** (split-pane YAML↔визуал в конфигураторе), но не умеет ширины колонок/высоты строк, границы по сторонам, создание макета с нуля, привязку данных.

**Решения пользователя** (зафиксированы): (1) печатные формы сначала, richtext (#43) — следующим отдельным планом; (2) PDF-импорт — «черновик + доводка в редакторе»; (3) серверный PDF со встроенным кириллическим шрифтом — да; (4) единый механизм на макетах, YAML-формы → legacy с автомиграцией.

## Целевая архитектура

Все три пути формирования сходятся в одной модели и одной паре рендереров:

```
.layout.yaml (макет v2 + binding) ──┐ BuildSheet (декларативный движок)
.os + .layout.yaml (DSL) ───────────┤ RunWithResult (как сейчас)
legacy *.yaml ──(автоконвертер)─────┘
                  ▼
        internal/sheet.Document   ← НОВЫЙ нейтральный пакет (модель ячеек,
                  │                  спаны, границы per-side, страницы)
        ┌─────────┴─────────┐        Решает цикл импортов interpreter↔printform
    sheet.HTML()        sheet.PDF()  (interpreter импортирует printform — maket.go:7)
  (перенос toHTML)   (fpdf + PT-шрифты, go:embed)
```

`interpreter.SpreadsheetDocument` становится тонкой DSL-обвязкой над `sheet.Document` (весь CallMethod/Get/Set остаётся, данные — в sheet).

### Макет v2 (internal/printform/layout.go)
- `Areas []*LayoutArea` (упорядоченный slice; имя — поле `Name`). **Обратная совместимость**: кастомный `UnmarshalYAML` через `yaml.Node` принимает и старый mapping (yaml.v3 сохраняет порядок ключей), и новый sequence. `MarshalYAML` пишет sequence.
- `LayoutCell.Borders *CellBorders{Left,Top,Right,Bottom}` (per-side, приоритет над legacy-пресетом `Border`).
- `Page *PageSetup{Orientation, Format, Margins, Scale}`.
- **`Binding`** — декларативные формы без кода:

```yaml
binding:
  sequence: [Шапка, ШапкаТаблицы, Строка, Итоги]   # default = порядок areas
  repeat_header: ШапкаТаблицы                       # повтор на каждой странице PDF
  parameters:                  # параметр ← выражение (язык renderer.go + новый Итог.<ТЧ>.<Поле>)
    НомерУПД: "Номер"
    Покупатель: "Покупатель.Наименование"
    ИтогоСумма: "Итог.Товары.Сумма | number:2"
  repeat:
    - area: Строка
      source: Товары           # табличная часть; параметры ← колонки, @row
```

Автопривязка по имени (параметр без записи = одноимённое поле), интерполяция `{{выражение|формат}}` в `text:` ячеек (только в декларативном пути; DSL-путь не меняется). Резолвер выражений переезжает из renderer.go в `internal/printform/binding.go`.

### Унификация
- `LoadDir`: standalone `*.layout.yaml` без парного `.os` = декларативная форма.
- Registry: единый `PrintFormRef{Kind: Declarative|DSL|Legacy, ...}`, `GetAllPrintForms(entity)`; приоритет коллизий Declarative > DSL > Legacy с warning.
- Маршруты: `/print/{form}` (HTML) и `/print/{form}/pdf` — для ВСЕХ видов; `/print-dsl/` → redirect. Кнопка «Печать ▾» — один цикл по GetAllPrintForms (templates.go:1199, templates_managed.go:349).

### PDF (internal/sheet/pdf.go)
- Шрифты **PT Serif + PT Sans** (SIL OFL 1.1, кириллица родная, визуально совместимы с Times New Roman/Arial; ~1.5–2 МБ на 6 начертаний; DejaVu — запасной). `go:embed` в `internal/sheet/fonts/` + OFL.txt. fpdf `AddUTF8FontFromBytes` (сабсеттинг — проверить спайком).
- Рисуем сами по координатам: сетка колонок (mm/px/% → мм; обобщить computeColWidths из pdf.go перед удалением), авто-высоты строк (SplitText), спаны через карту covered (как в toHTML), границы по сторонам отдельными Line (thin=0.2/medium=0.4/thick=0.8мм), фоны, выравнивания.
- Разрывы страниц: авто по высоте + явные `РазделительСтраниц` (оживает) + повтор шапки (`repeat_header` / `ПовторитьПриПечати` — оживает). `ПроверитьВывод` — по реальной высоте страницы вместо «50 строк». RowSpan через границу страницы в MVP запрещён (предупреждение в configcheck).
- Картинки: рендер в PDF (`RegisterImageOptionsReader`) и в HTML (`<img>` — сейчас игнорируется).
- DSL: свойства `ОриентацияСтраницы`/`РазмерСтраницы`/`ПоляПечати`; `Записать(имя,"pdf")` → base64 (как ВыгрузитьВExcel); кнопка «PDF» в тулбаре → серверный endpoint. `latinize` и pdf.go умирают.

### Legacy-миграция
- `ConvertLegacy(pf *PrintForm) (*LayoutTemplate, error)`: Title→область-заголовок; Header/Footer→области (markdown-подмножество → строки ячеек); Table→ШапкаТаблицы+Строка+Итоги+binding. Вызывается на загрузке (in-memory) — renderer.go и pdf.go удаляются (выживают formatters.go и binding.go).
- Команда `onebase printforms migrate --project <dir>` — переписывает файлы; прогнать по examples/* (21 форма), визуально сверить trade/accounting.
- `_ext_printforms`: сниффинг формата content (`areas:` → v2, иначе ConvertLegacy); схему БД не трогаем.
- configcheck (cross_refs.go:152): валидация binding (document, source-ТЧ, поля выражений, имена областей, непривязанные параметры); legacy-файлы — warning «выполните printforms migrate».

### Дизайнер (internal/launcher/configurator_tmpl.go ~1598-1931, 4437-4471; configurator.go :640-690, :2810-2868)
Минимум для «УПД руками» = 6.1–6.4:
1. **v2-модель в JS**: areas-массив, порядок областей (вверх/вниз), round-trip page:/binding: без потерь.
2. **Ширины колонок / высоты строк**: линейка над таблицей (клик → инпут), ручки строк.
3. **Границы per-side**: 4 тоггла Л/В/П/Н + толщина + пресеты «все/нет/сетка».
4. **Создание с нуля**: «+ Печатная форма (макет)» у сущности (заготовка с binding по первой ТЧ); «Создать макет» у .os без layout (снять ограничение HasLayout, configurator.go:679).
5. **Привязка данных**: панель «Данные» — дерево реквизитов/ТЧ/констант (метаданные уже в cfgEntity), клик → parameter + binding; чекбокс «строка ТЧ» у области; select форматтера.
6. **Предпросмотр**: POST `/configurator/layout/preview` (YAML + сущность → BuildSheet по последнему документу или синтетике → HTML/PDF).

### PDF-импорт (internal/pdfimport/)
- **Библиотека**: вендорить форк rsc.io/pdf (BSD-3, `dslipak/pdf`) в `internal/pdfimport/pdfparse/` + два расширения: path-операторы `m/l/re+S/f/B` → отрезки линий сетки; парсер ToUnicode CMap (bfchar/bfrange) — без него кириллица из сабсет-шрифтов 1С/Excel приходит мусором (~300 строк). **Спайк в начале этапа**: 3–5 реальных УПД-PDF — go/no-go.
- **Алгоритм** (`grid.go`, `ImportPage(f, size, page) (*LayoutTemplate, error)`): MediaBox→PageSetup; слияние коллинеарных линий (ε≈0.7pt); X-cuts/Y-cuts → мелкая сетка; регионы между линиями → ячейки/спаны; линии по сторонам → Borders; текстовые runs внутри региона → text (объединение по Y,X), FontSize/Bold/Italic из шрифта, выравнивание эвристикой; fallback без линий — кластеризация текста по Y/гистограмме X-зазоров. Выход: одна область «Страница1» → дизайнер; операция «Разрезать область перед строкой» в дизайнере (нужна для выделения области-повтора).
- **UI**: «Создать макет из PDF» (upload + № страницы) → POST `/configurator/layout/import-pdf` → черновик в редакторе.
- **Безопасность недоверенного PDF**: recover вокруг парсера (rsc.io паникует — это его API), таймаут 10с, лимит 10МБ + лимит распакованных стримов, admin-only зона.
- **Ожидания** (зафиксировать в доке): пригодный черновик только для векторных PDF с текстовым слоем; сканы → честная ошибка; точность ±1мм; выравнивания/многострочность — доводка руками.

## Этапы (каждый = рабочий PR от main; план в репо: Plans/64-printforms-v2.md)

| # | Этап | Зависит | Размер |
|---|------|---------|--------|
| 1 | Пакет `internal/sheet`: вынос модели+toHTML из spreadsheet_document.go, interpreter — обвязка; golden-тесты HTML до/после (бит-в-бит) | — | M |
| 2 | PDF-рендер: шрифты PT, `sheet.PDF()`, PDF для DSL-форм, `Записать("pdf")`, PageSetup в DSL, смерть latinize | 1 | M |
| 3 | Макет v2 + binding + декларативный движок `BuildSheet`; standalone .layout.yaml; единый PrintFormRef/маршруты/кнопка Печать | 1,2 | L |
| 4 | Legacy: ConvertLegacy на загрузке, команда `printforms migrate`, перевод examples/*, сниффинг _ext_printforms, удаление renderer.go/pdf.go, configcheck binding | 3 | M |
| 5 | Дизайнер: 6.1–6.4 («УПД руками»), затем 6.5–6.6 (привязка+preview) — можно двумя PR | 3 | L |
| 6 | PDF-импорт: спайк → vendored pdfparse + ToUnicode + линии; grid.go; endpoint+UI; acceptance на реальном УПД пользователя | 3,5 | L–XL |
| 7 | Полировка: картинки HTML+PDF, rowspan через страницу, DEVELOPER.md/ai-guide, перф (накладная 1000 строк, кэш SplitText) | 2–4 | S–M |

**MVP = этапы 1–3** (любая форма печатается в PDF с кириллицей; простые формы — без кода).

## Критические файлы

- `internal/dsl/interpreter/spreadsheet_document.go` — вынос модели в `internal/sheet` (этап 1)
- `internal/printform/layout.go` — макет v2 (areas slice + совместимый Unmarshal, Borders, Page, Binding) (этап 3)
- `internal/printform/renderer.go`, `pdf.go` — источники для binding.go/computeColWidths, затем удаляются (этап 4)
- `internal/ui/handlers_print.go`, `internal/ui/server.go:276` — единые маршруты (этапы 2–3)
- `internal/runtime/registry.go:248-361` — PrintFormRef (этап 3)
- `internal/launcher/configurator_tmpl.go`, `configurator.go` — дизайнер (этапы 5–6)
- `internal/extform/repo.go`, `internal/configcheck/cross_refs.go` — legacy/валидация (этап 4)

## Верификация

- Этап 1: golden-тесты — HTML всех DSL-форм examples/ до и после выноса идентичен; `go test ./...`.
- Этап 2: PDF УПД-подобного макета открывается с читаемой кириллицей (ручная проверка + тест на наличие UTF-8 шрифта в байтах PDF); `onebase check` examples.
- Этап 3–4: `printforms migrate` на examples → рендер всех 21 формы визуально эквивалентен (golden HTML); конфигурации со старыми map-areas макетами грузятся.
- Этап 5: acceptance — УПД собирается в дизайнере руками (воспроизвести шапку таблицы УПД с границами и спанами), сохраняется, печатается в PDF.
- Этап 6: импорт пользовательского УПД-PDF даёт черновик с сеткой/текстами, доводимый в дизайнере; битый/скан PDF → ошибка без падения сервера.
- Везде: go test ./..., i18ncheck (новые t-ключи в en+de), сборка, конвенции проекта (TDD, коммиты по-русски).

## Риски

- Цикл импортов interpreter↔printform — решён пакетом sheet (sheet ни от чего не зависит).
- Совместимость: legacy-парсер не удаляется (удаляется только рендерер); map-areas читаются вечно; golden-тесты examples.
- +~2 МБ шрифтов в бинаре — приемлемо (Monaco 4.2 МБ); сабсеттинг fpdf проверяется спайком.
- PDF-парсер на недоверенном вводе — recover/таймаут/лимиты; не копировать структуру mxl 1С (юр. паттерн проекта).

## Follow-up (вне этого плана)

- **Richtext (#43)** — отдельный план после: `FieldTypeRichText` + vendored Quill (~350КБ) + bluemonday-санитайзер + inline-эндпоинт вложений для `<img>` + вывод richtext-контента в печатные формы (ляжет на новый механизм). Ответить тестировщику в #43 о принятом направлении и порядке (печатные формы → richtext) при старте исполнения.
