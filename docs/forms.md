# Управляемые формы OneBase — спецификация `.form.yaml`

OneBase описывает форму объекта декларативно: дерево элементов и обработчики
событий лежат в `<projectDir>/forms/<entity>/<name>.form.yaml`. Авто-генерация
по полям сущности по-прежнему доступна для тех объектов, у которых
управляемой формы нет — выбор за пользователем (см. раздел «Опциональность»).

## Файлы и каталоги

```
<projectDir>/
  forms/
    <entityLower>/
      <formLower>.form.yaml           # описание элементов
      <formLower>.form.os             # обработчики событий (опционально)
      <formLower>/
        _resources/
          <ElementName>/
            Picture.png               # бинарные ресурсы PictureField
            ValuesPicture.png
```

- Имена каталогов и файлов — в нижнем регистре, для совместимости с
  case-insensitive ФС.
- `.form.os` хранит DSL-процедуры, привязанные к события дерева
  (`ПриОткрытии`, `ПриИзменении`, и т. д.). Связь задаётся в YAML через
  поле `events`.
- `_resources/` нужны только если в форме есть `ПолеКартинки` со ссылкой
  на файл (для `stdpic:X` копии не нужны).

В режиме «конфигурация в БД» (см. `onebase migrate`) те же пути
адресуют записи в таблице `_onebase_config(path, content)`.

## Опциональность

Каждая сущность независимо выбирает режим формы:

| Состояние `forms/<entity>/` | Поведение в пользовательском режиме |
|---|---|
| **Каталог отсутствует**          | Старая авто-форма по полям сущности — без изменений. |
| **Есть `.form.yaml` объектной формы** | Карточка рендерится по YAML (маркер `◇ managed`). Авто-форма становится fallback. |
| **Несколько форм одного `kind`** | Берётся первая найденная. |

Это позволяет постепенно мигрировать большой проект: сначала навешиваем
managed-форму на один справочник, остальное продолжает работать.

Приоритет загрузчика реализован в `internal/project/loader.go:loadFormModules`.

## Структура `.form.yaml`

```yaml
schema: onebase.form/v1       # ОБЯЗАТЕЛЬНО. Других схем пока нет.

form:                         # Общие свойства формы.
  name: ФормаОбъекта          # ObjectForm | ListForm | ChoiceForm | произвольное имя.
  kind: object                # object | list | choice | folder | custom.
  entity: Контрагент          # имя сущности, к которой привязана форма.
  title:                      # локализованный заголовок (lang → текст).
    ru: "Карточка контрагента"
  original_id: "0"            # id корневого узла из 1С (опционально, для round-trip).
  auto_save_settings: true    # сохранять данные в настройки пользователя.
  vertical_scroll: auto       # auto | never | always.

attributes:                   # реквизиты формы (живут отдельно от полей сущности).
  - name: Объект
    type: CatalogRef.Контрагент
    save: true
    main: true                # эквивалент <MainAttribute>true</MainAttribute>.
  - name: Товары
    type: ValueTable
    columns:
      - { name: Номенклатура, type: CatalogRef.Номенклатура }
      - { name: Цена,         type: "decimal(15,2)" }

commands:                     # команды формы.
  - name: Провести
    title: { ru: "Провести" }
    action: ПровестиКоманду   # имя процедуры в .form.os
    picture: stdpic:Post

command_bar:                  # авто-командная панель (необязательно).
  name: ФормаКоманднаяПанель
  visible: true
  buttons:
    - { name: КнПровести, command: Провести, representation: PictureAndText }

elements:                     # дерево UI. Порядок значим.
  - kind: ГруппаФормы
    name: Реквизиты
    title: { ru: "Реквизиты" }
    children:
      - kind: ПолеВвода
        name: ПолеНаименование
        title: { ru: "Наименование" }
        data_path: Объект.Наименование
        required: true
        hint: "Полное юридическое наименование"
        events:
          ПриИзменении: НаименованиеПриИзменении

events:                       # form-level handlers (имя события → имя процедуры).
  ПриОткрытии: ПриОткрытииФормы
  ПередЗаписью: ПередЗаписьюФормы

resources:                    # ресурсы (заполняется автоматически при импорте).
  - { path: _resources/Логотип/Picture.png, element: Логотип, original_name: Picture.png }

oneC_meta:                    # служебный блок, рантайм игнорирует.
  version: "2.20"
  unknown_xml: []             # base64-узлы XML без аналога в IR (для round-trip).
```

## Типы элементов

| `kind`             | 1С эквивалент      | Описание                                                  |
|--------------------|--------------------|-----------------------------------------------------------|
| `ПолеВвода`        | `InputField`       | Поле ввода значения. Тип ввода зависит от `metadata.Field` |
| `Надпись`          | `LabelField`       | Декоративная подпись                                       |
| `Кнопка`           | `Button`           | Кнопка действия                                            |
| `Флажок`           | `CheckBoxField`    | Чекбокс                                                    |
| `Переключатель`    | `RadioButtonField` | Radio-группа                                               |
| `ПолеКартинки`     | `PictureField`     | Поле с изображением (`picture: stdpic:X` или путь)        |
| `ТабличнаяЧасть`   | `Table`            | Табличная часть с колонками                                |
| `Колонка`          | `ColumnGroup`      | Колонка табличной части                                    |
| `ГруппаФормы`      | `UsualGroup`       | Группа (`<fieldset>`)                                      |
| `СтраницыФормы`    | `Pages`            | Закладки                                                   |
| `Страница`         | `Page`             | Одна закладка внутри `СтраницыФормы`                       |
| `КоманднаяПанель`  | `AutoCommandBar`   | Командная панель                                           |
| `КнопкаКП`         | `Button (CommandBarButton)` | Кнопка в командной панели                         |

В YAML можно указывать и канонические 1С-имена — обе формы понимаются
загрузчиком; mapping_out при экспорте обратно нормализует в 1С-канон.

## Свойства элемента

| Поле                  | Тип              | Назначение                                    |
|-----------------------|------------------|-----------------------------------------------|
| `kind`                | string           | Тип элемента (обязателен)                     |
| `name`                | string           | Имя элемента для адресации в `.form.os`       |
| `title`               | `{lang:text}`    | Локализованный заголовок                       |
| `data_path`           | string           | `Объект.Поле`, `Список.Цена`. Обязателен для `ПолеВвода`, `Флажок`, `Переключатель`, `ПолеДаты`, `ПолеСписка` |
| `original_id`         | string           | id из 1С, нужен только для round-trip          |
| `picture`             | string           | `stdpic:X` или относительный путь к ресурсу   |
| `values_picture`      | string           | Палитра выбора (`PictureField`)               |
| `readonly`            | bool             | Только чтение                                  |
| `required`            | bool             | Обязательно к заполнению                      |
| `choice`              | bool             | Включена кнопка выбора (для `ПолеВвода` со ссылочным типом) |
| `width`, `height`     | int              | Размер в условных единицах                    |
| `halign`, `valign`    | string           | Выравнивание                                  |
| `hint`                | string           | Всплывающая подсказка                         |
| `mask`                | string           | Маска ввода (regex)                            |
| `events`              | `{evt: proc}`    | Обработчики событий (имя процедуры в `.form.os`) |
| `props`               | `{key: any}`     | Прочие свойства, специфичные для типа         |
| `children`            | list             | Вложенные элементы                            |

## События

| OneBase                      | 1С                  |
|------------------------------|---------------------|
| `ПриОткрытии`                | `OnOpen`            |
| `ПередЗаписью`               | `BeforeWrite`       |
| `ПриЗаписи`                  | `OnWrite`           |
| `ПослеЗаписи`                | `AfterWrite`        |
| `ПередЗакрытием`             | `BeforeClose`       |
| `ПриЗакрытии`                | `OnClose`           |
| `ПриСоздании`                | `OnCreateAtServer`  |
| `ПриИзменении`               | `OnChange`          |
| `ОбработкаВыбора`            | `Choice` / `Choose` |
| `НачалоВыбора`               | `StartChoice`       |
| `Нажатие`                    | `OnClick`           |
| `ВыполнитьКоманду`           | `Command`           |
| `ПриДобавленииСтроки`        | (нет точного, см. `AfterAddRow`) |
| `ПриУдаленииСтроки`          | `AfterDeleteRow`    |

Полная таблица в [`docs/forms-1c-converter.md`](forms-1c-converter.md).

## Типы реквизитов

| OneBase                    | 1С `<v8:Type>`              | Качества                          |
|----------------------------|-----------------------------|-----------------------------------|
| `string`, `string(N)`      | `xs:string`                 | `<StringQualifiers><Length>N</Length></StringQualifiers>` |
| `number`, `decimal(P,S)`   | `xs:decimal`                | `<NumberQualifiers><Digits>P</Digits><FractionDigits>S</FractionDigits></NumberQualifiers>` |
| `date`, `dateTime`         | `xs:date` / `xs:dateTime`   | —                                 |
| `bool`                     | `xs:boolean`                | —                                 |
| `CatalogRef.X`             | `cfg:CatalogRef.X`          | —                                 |
| `DocumentRef.X`            | `cfg:DocumentRef.X`         | —                                 |
| `EnumRef.X`                | `cfg:EnumRef.X`             | —                                 |
| `ChartOfAccountsRef.X`     | `cfg:ChartOfAccountsRef.X`  | —                                 |
| `AnyRef`                   | `cfg:AnyRef`                | импорт даёт `W021`                |
| `ValueTable` + `columns:`  | `v8:ValueTable` + `<Columns>` | —                               |

При экспорте составные типы (`composite`) сворачиваются до строки + `W020`.

## Связка с `.form.os`

`.form.os` — это файл DSL OneBase с процедурами-обработчиками. Перед каждой
процедурой может стоять аннотация `// @directive=&НаКлиенте` (или
`&НаСервере`); конвертер использует её при экспорте обратно в `Module.bsl`.

Связь YAML ↔ `.form.os` по именам процедур:

```yaml
events:
  ПриОткрытии: ПриОткрытииФормы   # ← имя процедуры в .form.os
```

```bsl
// @directive=&НаСервере
Процедура ПриОткрытииФормы()
  ...
КонецПроцедуры
```

## Конструктор и редактор

В UI конфигуратора (узел дерева **◇ Управляемые формы**) есть split-pane
Monaco-редактор со следующими кнопками:

- **Сохранить** — пишет YAML и `.form.os` (в файлы или БД, по режиму базы).
- **Просмотр** — показывает упрощённую HTML-форму в iframe (без обращения к БД).
- **Проверить** — вызывает `Validate` и подсвечивает предупреждения.
- **Удалить** — удаляет YAML/OS/ресурсы.

Импорт из 1С (`onebase forms convert-from-1c`) автоматически создаёт
правильную структуру каталогов; результат сразу видно в редакторе.

## CLI

```
onebase forms convert-from-1c --src ... --entity X --dst <project>
onebase forms convert-to-1c   --src <yaml> --dst <Ext>
onebase forms validate         --src <yaml>
```

Подробнее в [`docs/forms-1c-converter.md`](forms-1c-converter.md).
