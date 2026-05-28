# План 40: Управляемые формы для справочников и обработок

## Контекст

Управляемые формы (managed forms) сейчас работают только для документов. Нужно опционально распространить на **справочники** (catalogs) и **обработки** (processors): если `forms/{name}/формаобъекта.form.yaml` существует — рендерится managed форма, иначе — стандартный UI.

### Текущее состояние

| Компонент | Документы | Справочники | Обработки |
|-----------|-----------|-------------|-----------|
| Entity struct | `Entity{Forms[]}` | `Entity{Forms[]}` (тот же!) | `Processor{Params[]}` — нет Forms |
| Загрузчик форм | `loadFormModules()` — все Entities | Уже загружает! | Нет загрузки |
| Рендеринг | `renderEntityForm()` | `form()` → `renderEntityForm()` — работает! | `processorForm()` → `page-processor` |
| Маршруты | `/ui/{kind}/{entity}/new`, form-event | `/ui/{kind}/{entity}/new` — те же | `/ui/processor/{name}` — отдельные |
| Сохранение | `store.Create/Update` | `store.Create/Update` | `interp.Run(procDecl)` |

**Ключевое открытие:** справочники уже поддерживают managed forms на уровне платформы. Загрузчик итерирует ВСЕ entities, `renderEntityForm()` работает для любого entity.

---

## Часть 1: Справочники — верификация

Инфраструктура уже есть. Проверить:
- `form()` handler → `renderEntityForm("object", data)` — работает для catalogs
- `submit()` / `form-event` — работают для `kind=catalog`
- `loadRefOptions` / `loadTPRefOptions` — работают для catalog entities

---

## Часть 2: Обработки — добавление managed форм

### Шаг 2.1. Processor struct + Forms
**Файл:** `internal/processor/processor.go`
- Добавить `Forms []*FormModule` (yaml:"-")

### Шаг 2.2. Загрузка форм обработок
**Файл:** `internal/project/loader.go`
- После `loadProcessors()` — загрузить managed формы для каждого процессора
- Формы в `forms/{processor_name}/формаобъекта.form.yaml`

### Шаг 2.3. Виртуальная Entity из Params
**Файл:** `internal/ui/handlers.go`
- `processorVirtualEntity(proc)` — создаёт Entity с Fields из Params
- Типы: string/text → string, number → number, date → date, bool → bool, choice → enum, reference:XXX → reference

### Шаг 2.4. Handler: рендеринг managed формы обработки
**Файл:** `internal/ui/handlers.go`
- `processorForm()`: если managed форма есть → `page-managed-form` с виртуальной entity
- `processorRun()`: собрать paramValues → `interp.Run` → результаты

### Шаг 2.5. Route: form-event для обработок
**Файл:** `internal/ui/server.go`
- `r.Post("/ui/processor/{name}/form-event", s.handleProcessorFormEvent)`

### Шаг 2.6. Шаблон: text/choice в managed form
**Файл:** `internal/ui/templates_managed.go`
- `text` → `<textarea>`
- `choice` → `<select>` с options

---

## Файлы платформы

| # | Файл | Изменение |
|---|------|-----------|
| 1 | `internal/processor/processor.go` | + Forms |
| 2 | `internal/project/loader.go` | + загрузка форм обработок |
| 3 | `internal/ui/handlers.go` | + processorVirtualEntity, managed form rendering |
| 4 | `internal/ui/server.go` | + route processor form-event |
| 5 | `internal/ui/templates_managed.go` | + text/choice типы |

## Файлы конфигурации (демо)

| # | Файл | Описание |
|---|------|----------|
| 1 | `forms/загрузкавыписки/формаобъекта.form.yaml` | Managed форма ЗагрузкаВыписки |
| 2 | `forms/загрузкавыписки/формаобъекта.form.os` | Обработчики формы |

---

## Проверка

1. Справочник: создать `forms/номенклатура/формаобъекта.form.yaml` → карточка с managed формой
2. Обработка без формы: старый UI не сломан
3. Обработка с формой: managed форма с вкладками/кнопками
4. Submit: заполнить → Выполнить → результат ниже
5. Form event: onchange → server handler → обновлённые values
