# План 71 — Оформление вывода отчёта (линии сетки, зебра)

Статус: черновик. Ветка: `feature/71-report-appearance` от `main`.
Связан с: план 59 (компоновка), план 70 (рантайм-настройки отчёта).

## Зачем

Пользователь заметил, что в кросс-таблице (и вообще в скомпонованных отчётах) **нет
вертикальных линий** — «дело привычки». Сейчас вид зашит в CSS `.report-composed`
(`internal/ui/templates.go:506-511`): у ячеек только `border-bottom`, боковых границ
нет, и это не настраивается ниоткуда.

В 1С СКД для этого есть отдельная вкладка оформления. В OneBase **условное
оформление уже есть** (`Composition.Conditional` → `CellStyle{Color,Background,
Bold,Italic}`), но линии сетки — это свойство **всей таблицы**, а не правило «по
условию». Поэтому им нужен отдельный блок `Appearance`, а не новое поле в `CellStyle`.

## Дизайн-решения

1. **Линии — табличное свойство, не условное.** Новый блок `Composition.Appearance`,
   отдельный от `Conditional`.
2. **Одно поле-перечисление `lines`** вместо набора булевых флагов: покрывает все
   комбинации одним контролом и решает проблему «дефолт = текущий вид» без инверсий
   (нулевое значение `""` трактуется как `horizontal`).
   - `""`/`horizontal` — как сейчас (только нижние границы). **Обратная совместимость.**
   - `vertical` — только вертикальные линии.
   - `both` — и горизонтальные, и вертикальные (полная сетка).
   - `none` — без линий.
3. **`zebra`** (чередование фона строк) — дёшево и часто просят; добавляем заодно.
4. **Два слоя (как в плане 70):**
   - **Дизайн-тайм:** секция «Оформление» в построителе отчёта конфигуратора →
     пишется в YAML, дефолт отчёта. Это и есть аналог «вкладки оформления» 1С.
   - **Рантайм, на пользователя:** тот же контрол в панели «Настройки» формы отчёта.
     **Безопасно:** `Appearance` — чистая презентация без исполняемых выражений,
     поэтому его МОЖНО пропускать из пользовательского ввода (в отличие от
     `Conditional[].When` / `Measures[].Expr`). Один любит линии — включил себе.
5. **Рендер через CSS-класс**, не инлайн-стили: `Appearance` → класс на `<table>`,
   правила в `templates.go`. Значение `lines` маппится `switch`'ем на фиксированные
   имена классов, поэтому любой мусор из недоверенного ввода вырождается в дефолт.

## НЕ-цели (возможные продолжения, не в этом плане)

- Границы в условном оформлении (`CellStyle.Border…`) — отдельная задача, если попросят.
- Линии в Excel-выгрузке — опциональный Task 8 ниже (HTML-вид — приоритет жалобы).
- Шрифт/размер/плотность таблицы целиком — позже, если будет спрос.

## Модель данных

```yaml
# documents/.../<отчёт>.yaml
composition:
  appearance:
    lines: both      # ""|horizontal(умолч.)|vertical|both|none
    zebra: false
```

```go
// internal/report/report.go

// Appearance — общее оформление вывода компоновки (план 71): свойства всей
// таблицы, не зависящие от условий (в отличие от Conditional). Нулевое значение
// = текущий исторический вид (горизонтальные линии, без зебры) — обратная
// совместимость со всеми существующими отчётами.
type Appearance struct {
	Lines string `yaml:"lines" json:"lines,omitempty"` // ""|horizontal|vertical|both|none
	Zebra bool   `yaml:"zebra" json:"zebra,omitempty"`
}

// + в Composition:
//   Appearance Appearance `yaml:"appearance" json:"appearance,omitempty"`
```

## CSS (templates.go, рядом со строками 506-511)

```css
/* План 71: оформление вывода. По умолчанию (нет класса) — только нижние границы. */
.report-composed.rep-lines-v  td,.report-composed.rep-lines-v  th{border-bottom:none;border-right:1px solid #eef2f7}
.report-composed.rep-lines-v  td:last-child,.report-composed.rep-lines-v th:last-child{border-right:none}
.report-composed.rep-lines-both td,.report-composed.rep-lines-both th{border-right:1px solid #eef2f7}
.report-composed.rep-lines-both td:last-child,.report-composed.rep-lines-both th:last-child{border-right:none}
.report-composed.rep-lines-none td,.report-composed.rep-lines-none th{border-bottom:none}
.report-composed.rep-zebra tbody tr:nth-child(even) td{background:#fafbfc}
```

> `nth-child` имеет низкую специфичность → инлайновый фон условного оформления
> (`cssOf`) всегда перекрывает зебру. Проверить тестом рендера.

## Задачи

### Task 1 — модель `Appearance`
- `internal/report/report.go` — **изменить.** Тип `Appearance` + поле в `Composition`.
- `internal/report/report_composition_test.go` — **изменить.** Round-trip YAML с `appearance`.
- [ ] Step 1.1 Добавить тип и поле.
- [ ] Step 1.2 Тест разбора YAML `appearance: {lines: both, zebra: true}`.
- [ ] Commit: `feat(report): блок appearance в компоновке (линии сетки, зебра)`

### Task 2 — разбор формы (общий для конфигуратора и рантайма)
- `internal/report/compform/compform.go` — **изменить.** В `Parse` читать
  `comp.appearance.lines` (валидировать по белому списку `{horizontal,vertical,both,none}`,
  иначе `""`) и `comp.appearance.zebra`. **Не** включать `Appearance` в проверку
  «пусто» на строке ~122 (оформление само по себе не делает компоновку непустой).
- `internal/report/compform/compform_test.go` — **изменить.** Разбор + отбраковка
  мусорного `lines`.
- [ ] Step 2.1 Парсинг + валидация.
- [ ] Step 2.2 Тесты.
- [ ] Commit: `feat(report): compform читает comp.appearance.*`

### Task 3 — рендер (HTML)
- `internal/ui/report_compose_render.go` — **изменить.** `renderComposedTable`:
  собрать класс таблицы из `spec.Appearance` (хелпер `appearanceClass(spec)` →
  `"report-composed rep-lines-v rep-zebra"` и т.п.).
- `internal/ui/report_cross_render.go` — **изменить.** `renderCrossTable`: тот же
  хелпер (сейчас класс жёстко `report-composed report-cross`).
- `internal/ui/templates.go` — **изменить.** Добавить CSS-правила (см. выше).
- `internal/ui/report_compose_render_test.go`, `internal/ui/report_cross_render_test.go`
  — **изменить.** Проверить наличие/отсутствие классов по `lines`/`zebra`; что при
  `lines:""` класс прежний (обратная совместимость).
- [ ] Step 3.1 Хелпер `appearanceClass`.
- [ ] Step 3.2 Подключить в обоих рендерах + CSS.
- [ ] Step 3.3 Тесты рендера.
- [ ] Commit: `feat(ui): линии сетки и зебра в отчётах компоновки`

### Task 4 — безопасность рантайма (пропустить Appearance из __settings)
- `internal/ui/report_settings.go` — **изменить.** В `mergeUserComposition` после
  `out := *base` присвоить `out.Appearance = u.Appearance` (презентация → безопасно).
  Обновить doc-комментарий: `Appearance` — презентационное, берётся из пользователя;
  исполняемое (`Conditional.When`, `Measures.Expr`) по-прежнему только из доверенного.
- `internal/ui/report_settings_test.go` — **изменить.** Тест: пользовательский
  `Appearance` применяется; пользовательский `Conditional`/`Expr` — НЕ применяется
  (регресс на issue #1 не ослаблен).
- [ ] Step 4.1 Правка merge + комментарий.
- [ ] Step 4.2 Тест безопасности.
- [ ] Commit: `feat(report): appearance в пользовательских настройках отчёта`

### Task 5 — UI конфигуратора (секция «Оформление»)
- `internal/launcher/configurator_tmpl.go` — **изменить.** В построителе отчёта рядом
  с условным оформлением добавить fieldset «Оформление»: `<select name="comp.appearance.lines">`
  (Нет / Горизонтальные / Вертикальные / И те и те) + чекбокс `comp.appearance.zebra`.
  Преднаполнять из текущего `composition.Appearance` при редактировании.
- `internal/launcher/static/configurator.js` — **проверить.** Если форма шлётся как
  `FormData(form)` — правок не нужно (именованные поля попадут сами); иначе добавить
  поля в сериализацию.
- `internal/launcher/report_builder_render_test.go` — **изменить.** Контрол есть в
  разметке; преднаполнение из сохранённого `Appearance`.
- [ ] Step 5.1 Разметка fieldset + преднаполнение.
- [ ] Step 5.2 Проверить JS-сериализацию.
- [ ] Step 5.3 Тест рендера построителя.
- [ ] Commit: `feat(configurator): секция «Оформление» в построителе отчёта`

### Task 6 — UI панели «Настройки» формы отчёта
- `internal/ui/templates.go` — **изменить.** В панели рантайм-настроек (та, что пишет
  `__settings`, поля `comp.*`) добавить тот же select+чекбокс, преднаполняемые из
  эффективной компоновки.
- `internal/ui/report_settings_test.go` — **изменить (при необходимости).** Сквозной
  тест: POST `comp.appearance.lines=vertical` → сохранилось → рендер содержит класс.
- [ ] Step 6.1 Контрол в панели.
- [ ] Step 6.2 Сквозной тест save→render.
- [ ] Commit: `feat(ui): переключатель оформления в настройках отчёта`

### Task 7 — документация
- `docs/features.md` — **изменить.** Секция «Оформление отчётов» (`status: testing`,
  `since: build-NNN`, `date: 2026-06-22`) — что это и как попробовать.
- `DEVELOPER.md` — **изменить.** Формат блока `appearance` в справочнике компоновки.
- [ ] Step 7.1 features.md + DEVELOPER.md.
- [ ] Commit: `docs: оформление вывода отчёта (план 71)`

### Task 8 — (опционально) линии в Excel-выгрузке
- `internal/ui/report_cross_excel.go` + путь обычной выгрузки + `internal/excel/excel.go`
  — границы ячеек по `Appearance.Lines`, если `excel.ExportList` это поддерживает.
- Решение по объёму принять при реализации; на жалобу пользователя (экранный вид)
  не влияет. Отдельный коммит, можно отложить.

## План тестов

- `go test ./internal/report/... ./internal/ui/... ./internal/launcher/...`
- `onebase check --project examples/trade` (компоновки с `appearance` валидны;
  `configcheck` оформление игнорирует — исполняемого содержимого нет).
- Ручной прогон `onebase run` по trade: переключить «Линии: и те и те» в построителе
  (виден дефолт) и в панели «Настройки» (виден per-user), проверить кросс и обычный.

## Обратная совместимость и откат

- Нулевой `Appearance` (`lines:""`) = текущий вид → существующие отчёты не меняются;
  тест на это в Task 3.
- Недоверенный `lines` из `__settings` обезврежен `switch`'ем рендера + валидацией в
  `compform` → инъекции в CSS-класс нет.
- Откат — чисто аддитивная фича: revert ветки `feature/71-report-appearance`, данных
  миграций нет (`Appearance` живёт в YAML и в JSON `_settings`).

## Трейлер коммитов

`Generated-with: Claude Code` (правообладатель — Иван Титов; не `Co-Authored-By`).
