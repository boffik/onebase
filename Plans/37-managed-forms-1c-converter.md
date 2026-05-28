# План 37: Управляемые формы OneBase + двусторонний конвертер 1С ↔ OneBase + UI-редактор

## Контекст

Сейчас в OneBase формы — это массивы `list_form`/`item_form` в YAML объекта (только видимость и порядок полей, см. `internal/metadata/types.go:80-81`). Авто-генерируемый рендер живёт в `internal/ui/templates.go` (define `page-form`, строки 790-990).

Инфраструктура «управляемых форм» уже частично заложена:
- `internal/metadata/form_module.go` — `FormModule`, `FormElement`, 13 типов элементов, 15+ событий, дерево `Children`, `Handlers`, `Procedures`.
- `internal/dsl/loader/form_loader.go` — загрузчик `.form.os`.
- `internal/dsl/interpreter/form_interpreter.go` — интерпретатор с готовым `ExecuteEventHandler`.

Чего нет: UI-редактора, родного декларативного описания структуры формы (только модуль с процедурами), демо-форм в `examples/`, и интероперабельности с 1С-форматом.

Цель — добавить полноценный объект метаданных «управляемая форма» (декларативный YAML рядом с DSL-модулем), UI-редактор по паттерну существующего редактора печатных форм (split-pane Monaco + live preview) и двусторонний конвертер с `Form.xml` + `Module.bsl` + `Items/*` управляемых форм 1С. Текущая авто-форма остаётся в роли fallback'а.

**Решения, зафиксированные с пользователем:**
1. Родной формат — расширенный YAML (`.form.yaml`).
2. Редактор — split-pane Monaco + live preview (по паттерну редактора печатных форм).
3. Конвертер двусторонний с самого начала.
4. Покрытие полное — все типы элементов (включая `PictureField` с бинарными ресурсами), локализация `v8:item`, уникальные `id` для round-trip.

**Юридический контекст:** интероперабельность с открытым текстовым форматом данных юридически безопасна (стандарт практики). Риски снимаются собственной YAML-схемой как канонической, парсингом 1С-формата по образцам без включения чужих XSD, дисклеймером (уже принят в проекте) и формулировкой «совместимость с XML управляемой формы Enterprise-системы» вместо «формат 1С».

## Архитектура

### 1. Новый пакет `internal/onec_forms/` — изолированный конвертер через IR

Двусторонний конвертер реализован через нейтральное **промежуточное представление**: `Form.xml ↔ IR ↔ .form.yaml`. Пакет зависит только от `internal/metadata`, `gopkg.in/yaml.v3`, `encoding/xml`.

| Файл | Роль |
|---|---|
| `ir.go` | `IRForm`, `IRElement`, `IRAttribute` (+ `IRAttributeColumn` для ValueTable), `IRCommand`, `IRTitle` (map[lang]content), `IRType`, `IRResource` |
| `reader_xml.go` | `ReadFormXML(path) (*IRForm, []Warning, error)` — DOM-парсер через `encoding/xml` с `xml:",any"` для сохранения порядка children и неизвестных узлов |
| `writer_xml.go` | `WriteFormXML(*IRForm, dst)` — восстановление namespaces, `version="2.20"`, использует `OriginalID` если есть, иначе генерирует id ≥ 1000 |
| `reader_yaml.go`, `writer_yaml.go` | `.form.yaml` ↔ IR; writer на `yaml.Node` для стабильного порядка ключей |
| `mapping_in.go`, `mapping_out.go` | Нормализация IR при импорте/экспорте (русификация имён, маппинг событий, восстановление namespace-префиксов) |
| `module_bsl.go` | State-machine лексер `Module.bsl`: режет на процедуры, выделяет директивы `&НаСервере`/`&НаКлиенте`/…, сохраняет параметры с типами и тело «как есть». Не строит AST |
| `module_bsl_writer.go` | `.form.os` → `Module.bsl`: восстанавливает директиву из аннотации `// @directive=...` (или дефолт по семантике события) |
| `resources.go` | Копирование `Items/<ElementName>/Picture.png|ValuesPicture.png|*.gif` в проект и обратно; стандартные иконки `<xr:Ref>StdPicture.X</xr:Ref>` ↔ `picture: stdpic:X` без копирования |
| `diagnostics.go` | `Warning { Severity, Code, Element, Field, Message, Suggest }`; коды `W001..W050` |
| `types_map.go`, `events_map.go`, `elements_map.go` | Единые таблицы соответствий (см. ниже) |
| `onec_forms.go` | Публичный фасад: `ImportFromOneC(...)`, `ExportToOneC(...)`, `Validate(yamlPath)` |
| `roundtrip_test.go`, `fixtures/` | Golden round-trip + мини-выборки реальных Form.xml (50-100 строк) и эталонные YAML |

### 2. Расширение `internal/metadata/form_module.go`

Существующие `FormModule`, `FormElement`, `FormElementType`, `FormEventType` остаются — добавляются поля.

**`FormModule` — новые поля:**
- `Title map[string]string` — локализованный заголовок.
- `OriginalID string` — id корневого узла из 1С.
- `Attributes []*FormAttribute` — реквизиты формы (отдельны от полей сущности).
- `Commands []*FormCommand`.
- `AutoCommandBar *FormCommandBar`.
- `AutoSaveDataInSettings bool`, `VerticalScroll string`.
- `LayoutKind string` — `"managed"` (новый) или `"autogen"` (текущее поведение; пустое = `"autogen"` для backward-compat).

**Новые типы:**
- `FormAttribute { ID, Name, Title (map[string]string), TypeRef string, Length, Precision int, AllowedLength string, Save bool, FillingValue string, Columns []*FormAttributeColumn, Props }`.
- `FormAttributeColumn { ID, Name, Title, TypeRef, Length, Precision, Props }` — для ValueTable-реквизита.
- `FormCommand { ID, Name, Title, Group, Picture, Action /* имя процедуры */, Props }`.
- `FormCommandBar`, `FormCommandBarButton`.

**`FormElement` — новые поля:** `OriginalID`, `TitleMap`, `DataPath`, `Picture`, `Width`, `Height`, `HorizontalAlign`, `VerticalAlign`, `ReadOnly`, `Hint`, `Mask`, `Choice`, `UnknownXML []byte` (для round-trip без потерь).

**Новые `FormElementType`:** `Колонка` (колонка Table), `КоманднаяПанель`, `ПолеКартинки` (PictureField), `КнопкаКП` (кнопка командной панели).

**Новые `FormEventType`:** `Нажатие` (OnClick), `ПередДобавлениемСтроки`, `ПослеДобавленияСтроки`, `ПередУдалениемСтроки`, `НачалоВыбораИзСписка`, `АвтоПодбор`, `ВыполнитьКоманду`.

**Утилиты:** `(*FormModule) GenerateID()` (счётчик ≥ 10000, чтобы не пересекаться с 1С), `FindByID`, `Walk`, `(*FormElement) IsContainer()`.

### 3. Родной формат `.form.yaml` — расширенный YAML

Файлы лежат в `<project>/forms/<entity>/<form_name>.form.yaml` рядом с соседним `<form_name>.form.os` (DSL-модуль с процедурами-обработчиками) и опциональной папкой `<form_name>/_resources/` с бинарными ресурсами. Сосуществует с существующей привычкой хранить `*.form.os` в `src/` (fallback).

Минимальный пример (полный — в `docs/forms.md`):

```yaml
schema: onebase.form/v1
form:
  name: ФормаОбъекта
  kind: object              # object | list | choice | folder | custom
  entity: Контрагенты
  title: { ru: "Контрагенты (карточка)" }
  original_id: "0"

attributes:                 # реквизиты формы (включая ValueTable)
  - name: Объект
    type: DocumentRef.РеализацияТоваров
    save: true
    original_id: "1"
  - name: ТаблицаТоваров
    type: ValueTable
    columns:
      - { name: Номенклатура, type: CatalogRef.Номенклатура }
      - { name: Цена,         type: "decimal(15,2)" }

commands:
  - { name: Провести, title: { ru: "Провести" }, action: ПровестиКоманду, picture: stdpic:Post }

elements:                   # дерево; порядок значим
  - kind: ГруппаФормы
    name: Шапка
    title: { ru: "Шапка" }
    children:
      - kind: ПолеВвода
        name: ПолеКонтрагент
        data_path: Объект.Контрагент
        original_id: "132"
        choice: true
        events: { ПриИзменении: КонтрагентПриИзменении }
      - kind: Флажок
        name: ПолеПроведен
        data_path: Объект.Проведен
  - kind: СтраницыФормы
    children:
      - kind: Страница
        title: { ru: "Товары" }
        children:
          - kind: ТабличнаяЧасть
            name: Товары
            data_path: Объект.Товары
            children:
              - { kind: Колонка, name: КолНоменклатура, data_path: Объект.Товары.Номенклатура }

events:
  ПриОткрытии: ПриОткрытииФормы

oneC_meta:                  # служебно, только для конвертера, рантайм игнорирует
  version: "2.20"
  unknown_xml: []           # base64-куски экзотических узлов для round-trip
```

Приоритеты загрузки (новая функция в `internal/dsl/loader/form_loader.go`):
1. `forms/<entity>/<form>.form.yaml` → managed.
2. `src/<entity>_<form>.form.os` → существующая логика (autogen).
3. Авто-генерация по полям Entity.

### 4. Маппинги

**Элементы:** `InputField ↔ ПолеВвода`, `LabelField ↔ Надпись`, `Button ↔ Кнопка`, `CheckBoxField ↔ Флажок`, `RadioButtonField ↔ Переключатель`, `PictureField ↔ ПолеКартинки`, `Table ↔ ТабличнаяЧасть`, `ColumnGroup ↔ Колонка`, `UsualGroup ↔ ГруппаФормы`, `Pages ↔ СтраницыФормы`, `Page ↔ Страница`, `AutoCommandBar ↔ КоманднаяПанель`, `Decoration ↔ Надпись (props.decoration=true)`. Неизвестные → `Props + UnknownXML` + warning `W010`.

**Типы реквизитов:** `xs:string + StringQualifiers ↔ string(N)`, `xs:decimal + NumberQualifiers ↔ decimal(P,S)`, `xs:dateTime ↔ dateTime/date`, `xs:boolean ↔ bool`, `cfg:CatalogRef.X ↔ CatalogRef.X` (имя кириллицей), `cfg:DocumentRef.X ↔ DocumentRef.X`, `cfg:EnumRef.X ↔ EnumRef.X`, `v8:ValueTable ↔ ValueTable + columns:`, композитные типы → `string` + `W021`.

**События:** `OnOpen ↔ ПриОткрытии`, `BeforeWrite ↔ ПередЗаписью`, `OnWrite ↔ ПриЗаписи`, `AfterWrite ↔ ПослеЗаписи`, `OnCreateAtServer ↔ ПриСоздании`, `OnChange ↔ ПриИзменении`, `Choice ↔ ОбработкаВыбора`, `StartChoice ↔ НачалоВыбора`, `AfterDeleteRow ↔ ПослеУдаленияСтроки`, `OnClick ↔ Нажатие`, `Command ↔ ВыполнитьКоманду`. Без 1:1 аналога → `props.events_unmapped: {…}` + `W030`.

### 5. Module.bsl ↔ .form.os

**Импорт.** Лексер `module_bsl.go` через state-машину режет файл на «директива + процедура/функция»:
- Директивы `&НаСервере`, `&НаКлиенте`, `&НаСервереБезКонтекста`, `&НаКлиентеНаСервереБезКонтекста` сохраняются в `BSLProcedure.Directive` и при записи в `.form.os` выводятся как комментарии-аннотации `// @directive=&НаСервере` над процедурой (OneBase не различает клиент/сервер).
- Тело копируется как есть (DSL OneBase близок к BSL, но не идентичен).
- Сканер `W040` ищет 20–30 типичных BSL-конструкций без аналога (`Новый Запрос`, `НСтр(...)`, `СтрШаблон(...)`, `ОбщегоНазначения.…`, `Метаданные.…`) и выдаёт по одному предупреждению с указанием строки.
- Параметры со `Знач` и типизированными аннотациями сохраняются.

**Экспорт.** Для каждой процедуры в `.form.os`: директива берётся из `// @directive=...`, иначе — дефолт по семантике события (`OnOpen`/`OnCreateAtServer` → `&НаСервере`, `OnChange`/`OnClick` → `&НаКлиенте`). Тело копируется как есть, специфичные для OneBase конструкции помечаются `W041`.

### 6. Round-trip без потерь

- При импорте каждый элемент/реквизит/команда получает `original_id` (сохраняется и в IR, и в YAML).
- Новые элементы из редактора OneBase получают id через `FormModule.GenerateID()`, счётчик стартует с 10000.
- При экспорте: `original_id` если есть, иначе сгенерированный.
- Узлы XML, у которых нет маппинга (включая ДКС, Conditional Appearance), сериализуются base64 в `oneC_meta.unknown_xml[]` с привязкой к имени элемента.

### 7. Бинарные ресурсы (`PictureField`)

**Импорт:** сканируется `Forms/<FormName>/Ext/Form/Items/<ElementName>/`, файлы копируются в `<project>/forms/<entity>/<form_name>/_resources/<ElementName>/<filename>`. В YAML — `picture: _resources/<ElementName>/Picture.png` (или `values_picture: …` для палитры). `<xr:Ref>StdPicture.X</xr:Ref>` → `picture: stdpic:X` без копирования.

**Экспорт:** обратное копирование. Бинарность определяется по расширению, содержимое не декодируется.

**БД-режим (`ConfigSource="database"`):** существующая `_onebase_config(path TEXT, content BYTEA)` уже совместима — записываем по путям `forms/<entity>/<form>.form.yaml`, `.form.os` и `forms/<entity>/<form>/_resources/<Element>/Picture.png`.

### 8. UI-редактор в конфигураторе

В дерево конфигуратора (`internal/launcher/configurator_tmpl.go`) под каждой сущностью добавляется подкатегория «Формы» с дочерними узлами по каждой управляемой форме (иконки: `◇` managed, `▢` autogen).

На странице сущности — две вкладки:
- **«Авто-форма»** — текущий UI (чекбоксы `list_form`/`item_form`).
- **«Управляемая форма»** — split-pane Monaco Editor (YAML слева + live preview справа), по паттерну существующего редактора печатных форм. Кнопки: «Сохранить», «Импорт из 1С», «Экспорт в 1С», «Проверить», «Удалить форму». Если managed-формы нет — кнопка «Создать управляемую форму».

**Новые handlers** в `internal/launcher/forms_handlers.go`:
- `GET /cfg/{baseID}/forms/{entity}/{formName}/edit`
- `POST /cfg/{baseID}/forms/{entity}/{formName}/save`
- `POST /cfg/{baseID}/forms/{entity}/{formName}/delete`
- `POST /cfg/{baseID}/forms/{entity}/import-1c` (multipart с Form.xml + Module.bsl + Items/*)
- `GET /cfg/{baseID}/forms/{entity}/{formName}/export-1c` (отдаёт zip)
- `POST /cfg/{baseID}/forms/{entity}/{formName}/validate` (JSON c warnings)
- `GET /cfg/{baseID}/forms/{entity}/{formName}/preview` (HTML для iframe)

**Рантайм-рендер.** В `internal/ui/templates.go` добавить define `page-managed-form` — рекурсивный рендер дерева `FormModule.Elements` с отдельным HTML-шаблоном на каждый `kind`. Старый `page-form` остаётся для авто-формы; выбор шаблона в `internal/ui/handlers.go` по флагу `Entity.HasManagedForm(kind)`. Превью использует тот же шаблон в режиме `IsPreview=true` (handlers не вызываются, кнопка «Записать» отключена).

### 9. CLI

Cobra-команда `onebase forms` в `internal/cli/forms.go` (зарегистрирована в `internal/cli/root.go`):
- `convert-from-1c --src <1c-config-dir> --entity <Name> --form-name <FormName> --dst <project-dir>` (+ `--all-forms`, `--all-entities`).
- `convert-to-1c --src <project-dir> --entity <Name> --form-name <FormName> --dst <1c-config-dir>`.
- `validate --src <path-to-form.yaml>` — exit-код по errors.
- `init --entity <Name> --form-name ФормаОбъекта --dst <project-dir>` — создать пустой `.form.yaml` по реквизитам сущности.

Warnings/errors с цветовым выводом по аналогии с существующими CLI-командами.

## Список новых файлов

`internal/onec_forms/`: `ir.go`, `reader_xml.go`, `writer_xml.go`, `reader_yaml.go`, `writer_yaml.go`, `mapping_in.go`, `mapping_out.go`, `module_bsl.go`, `module_bsl_writer.go`, `resources.go`, `diagnostics.go`, `types_map.go`, `events_map.go`, `elements_map.go`, `onec_forms.go` (фасад), `onec_forms_test.go`, `roundtrip_test.go`, `fixtures/`.

`internal/launcher/forms_handlers.go`, `internal/launcher/forms_tmpl.go`.

`internal/dsl/loader/managed_form_loader.go`.

`internal/cli/forms.go`.

`docs/forms.md`, `docs/forms-1c-converter.md`.

## Список модифицируемых файлов

- `internal/metadata/form_module.go` — расширение FormModule/FormElement, новые типы, утилиты (`GenerateID`, `FindByID`, `Walk`).
- `internal/dsl/loader/form_loader.go` — попытка managed-загрузки сначала, fallback на autogen.
- `internal/launcher/configurator.go` — регистрация новых маршрутов в `mountConfigurator`.
- `internal/launcher/configurator_tmpl.go` — узлы дерева «Формы», tab-switcher на странице сущности.
- `internal/ui/templates.go` — добавить define `page-managed-form`.
- `internal/ui/handlers.go` — выбор шаблона: managed/autogen.
- `internal/dsl/interpreter/form_interpreter.go` — убедиться, что `data_path` (включая `Объект.X`, `Список.X`, реквизиты формы) корректно резолвится в `ExecuteEventHandler`.
- `internal/cli/root.go` — подключить `formsCmd`.
- `README.md` — пункт о управляемых формах + конвертере + расширенный юр.дисклеймер.

## Этапы реализации

1. **Foundation.** Расширить `metadata/form_module.go`; создать `internal/onec_forms/ir.go` + stub-файлы; `managed_form_loader.go`; ветка managed в `form_loader.go`.
2. **Импорт 1С** (главный value-этап). `reader_xml.go`; `mapping_in.go` + три таблицы; `module_bsl.go`; `resources.go`; `writer_yaml.go`; фасад `ImportFromOneC` + CLI `convert-from-1c`. Round-trip golden test на мини-фикстуре.
3. **Рантайм managed-форм.** `page-managed-form` шаблон + выбор шаблона в `ui/handlers.go`. Ручная проверка: открыть простую managed-форму в пользовательском режиме.
4. **UI-редактор.** `forms_handlers.go` + `forms_tmpl.go` (Monaco split-pane), вкладка и узлы дерева в конфигураторе, регистрация роутов.
5. **Экспорт 1С.** `reader_yaml.go`, `mapping_out.go`, `writer_xml.go`, `module_bsl_writer.go`; фасад `ExportToOneC` + CLI; полный golden round-trip (Form.xml → YAML → Form.xml').
6. **Полировка.** `Validate` фасад + CLI + UI-кнопка; `docs/forms.md` и `docs/forms-1c-converter.md`; дисклеймер в README; опциональный big-test (`//go:build big`) на реальной форме УТ11 из `C:\Projects\АА5БП3\…`.

## Тестирование

- **Round-trip golden test** (`roundtrip_test.go`): мини-Form.xml → IR → YAML → IR' → Form.xml'; diff бизнес-полей, побайтное сравнение `unknown_xml`.
- **Большая реальная форма** (build tag `big`): использует `C:\Projects\АА5БП3\УТ11УТ11\ПереносДанныхУТ11УТ11_52\Forms\Форма\Ext\Form.xml` (5434 строки), проверяет сохранность дерева, копирование 8 папок ресурсов, re-parse результата.
- **Unit-тесты маппингов** (`types_map_test.go`, `events_map_test.go`, `elements_map_test.go`) — по строке таблицы в обе стороны.
- **Тесты лексера BSL** — процедура с директивой, с типизированными параметрами, экспортная, многострочный комментарий, директива без процедуры.
- **Тесты валидатора** (`onec_forms_test.go`) — неизвестный `kind`, неизвестный `data_path`, обработчик без процедуры в `.form.os`.
- **Handlers** (`forms_handlers_test.go`) — POST импорта возвращает HTML с warnings, validate возвращает JSON.

## Верификация (ручная)

1. **Через UI создать managed-форму:** конфигуратор → Контрагенты → «Управляемая форма» → создать; в Monaco описать пару полей и группу; сохранить; в пользовательском режиме открыть карточку — форма по YAML.
2. **Импорт реальной формы:** `onebase forms convert-from-1c --src C:\Projects\АА5БП3\УТ11УТ11\ПереносДанныхУТ11УТ11_52 --entity Контрагенты --form-name Форма --dst C:\tmp\proj` → YAML + .form.os + 8 папок ресурсов + список warnings; открыть в редакторе — live preview.
3. **Round-trip:** импорт фикстуры → правка одного `title` в YAML → экспорт → diff Form.xml.original vs Form.xml.exported показывает только заголовок.
4. **Валидация:** в YAML вписать `data_path: НесуществующееПоле` → кнопка «Проверить» → warnings в нижней панели.
5. **Backward-compat:** `examples/simple-erp`, `examples/trade` без managed-форм продолжают работать на auto-form, никаких изменений.

## Юридический дисклеймер (для README и `docs/forms-1c-converter.md`)

- Конвертер работает с **публичными текстовыми форматами** (XML и BSL) выгрузки конфигурации; форматы открыты по спецификации EDT и используются сторонними инструментами (OScript, EDT-плагины) много лет.
- Все маппинги (типы/события/элементы) — собственные таблицы соответствий по общедоступной документации.
- Лексер `Module.bsl` — минимальный токенизатор без AST; тела процедур копируются как цитаты, без реверс-инжиниринга исполнения.
- Имена ссылочных типов (`CatalogRef.X`, `DocumentRef.X`) — естественные термины русскоязычного бизнес-софта, не охраняемые знаки.
- В маркетинге не используется «формат 1С» / «1С-совместимый» — формулировка: «совместимость с XML управляемой формы Enterprise-системы».
- OneBase не аффилирован с правообладателем формата (главный дисклеймер в README сохраняется).

## Открытые вопросы (не блокируют MVP)

- Поддержка ДКС (Data Composition System) — пока в `oneC_meta.unknown_xml`.
- Conditional Appearance — заглушка `props.conditional_appearance: …`, в рантайме игнорируется.
- Drag-and-drop редактор поверх Monaco — следующая итерация.
- Импорт форм-расширений (Extension Forms) — отдельный план.

## Критичные файлы

- `C:\Projects\onebase\internal\metadata\form_module.go`
- `C:\Projects\onebase\internal\onec_forms\ir.go`
- `C:\Projects\onebase\internal\onec_forms\onec_forms.go`
- `C:\Projects\onebase\internal\dsl\loader\managed_form_loader.go`
- `C:\Projects\onebase\internal\launcher\forms_handlers.go`
- `C:\Projects\onebase\internal\ui\templates.go`
