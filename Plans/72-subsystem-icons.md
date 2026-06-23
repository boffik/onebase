# 72 — Иконки подсистем (и страниц): довести до рендера

## Проблема

Поле `icon:` у подсистем уже есть в модели, редактируется в конфигураторе и
сохраняется в YAML, но **нигде не отображается** в пользовательском интерфейсе.
Фича «наполовину»: модель и редактор есть, рендера нет. Нужно отрисовать иконку
в навигации, с фолбэком на неизвестное/пустое имя, и улучшить ввод.

## Текущее состояние (проверенные точки в коде)

- **Модель.** `internal/metadata/subsystem.go` — `Subsystem.Icon` (yaml `icon`).
  Страницы тоже имеют иконку: `internal/page/page.go` — `Page.Icon`
  (round-trip проверяется в `internal/page/page_test.go:103`).
- **Ввод (конфигуратор).** Свободный текстовый инпут:
  `internal/launcher/configurator_tmpl.go:1597` — подсистема, placeholder
  `shopping-cart`; `:1740` — страница/главная, placeholder `layout-dashboard`.
  Метка `{{t $.Lang "Иконка"}}` уже есть.
- **Сохранение.** `internal/launcher/configurator.go:940` (`sub.Icon`),
  `:923` (`pg.Icon`).
- **Разрыв в рендере.** `internal/ui/templates.go:850-854` — панель подсистем
  `.subsys-bar` выводит только `{{.DisplayName $.Lang}}`, иконку игнорирует.
  `grep '\.Icon' **/*.go` → ноль потребления в рендере (только модель, конфигуратор,
  тесты).
- **Исходный замысел.** `Plans/10-subsystems.md:115` — `<i class="icon-{{.Icon}}">`,
  в реальную навигацию не подключён. Связанное: `Plans/39-subsystem-home-pages.md`.
- **Ассеты.** `internal/webassets/assets.go` — паттерн вендоринга: `//go:embed` +
  `XxxHandler()` (Monaco/ECharts/SlickGrid/Quill), монтируются под `/vendor/xxx/`
  через `http.StripPrefix` в ui- и launcher-серверах. Тест-эталон —
  `internal/webassets/assets_test.go`.

## Набор иконок

**Lucide** (`https://lucide.dev/icons`) — плейсхолдеры `shopping-cart` /
`layout-dashboard` уже из него. Имена — kebab-case.

## Решение: способ рендера (выбрать перед стартом)

- **(A) Сервер-сайд курируемый набор (рекомендуется для старта).**
  `map[string]template.HTML` с инлайн-SVG ~30–50 ходовых иконок + helper
  `lucideIcon(name)` с фолбэком. Без JS, без клиентских ассетов; шипятся только
  включённые иконки. Минус — список ограничен (расширяется по мере надобности).
- **(B) Полный Lucide как SVG-спрайт.** `webassets/lucide/sprite.svg` + `//go:embed`
  + `LucideHandler()` под `/vendor/lucide/`, рендер
  `<svg><use href="/vendor/lucide/sprite.svg#{{.Icon}}"/></svg>`. Работает любое
  имя из набора. Минус — размер спрайта (весь Lucide) и нюансы `<use>`
  (стилизация/кэш).
- **(C) Lucide JS** (`data-lucide` + `lucide.createIcons()`) — не рекомендую
  (JS-зависимость, FOUC).

Рекомендация: начать с **(A)**; неизвестное/пустое имя → дефолт (напр. `square`)
или ничего. При необходимости полного набора позже мигрировать на (B).

## Где рендерить

1. **Панель подсистем** `.subsys-bar` (`internal/ui/templates.go:853`) — иконка
   перед заголовком. Основное.
2. (Опц.) Свёрнутая навигация `ob-nav` (`templates.go:857+`), если показывает
   подсистемы.
3. (Опц.) Иконка **страницы** в навигации — сначала найти место рендера
   навигации страниц (план 66 `Plans/66-pages.md`).
4. Фолбэк: пустое/неизвестное имя → без иконки или дефолт, без битой разметки.

## Шаги

1. Зафиксировать способ (A/B).
2. (A) `internal/ui/icons.go`: `var lucideIcons map[string]template.HTML` +
   `LucideIcon(name) template.HTML` (фолбэк). Регистрация в funcMap шаблонов ui
   (и launcher — для превью в конфигураторе).
   (B) вендор `webassets/lucide/`, `//go:embed`, `LucideHandler()`, монтаж
   `/vendor/lucide/` в ui- и launcher-серверах; helper для `<svg><use>`.
3. Отрисовать в `.subsys-bar`: `{{lucideIcon .Icon}}{{.DisplayName $.Lang}}` +
   CSS `.subsys-icon` (≈16px, выравнивание, отступ).
4. (Опц.) `ob-nav` и навигация страниц.
5. **Конфигуратор**: рядом с инпутом `name="icon"` (`configurator_tmpl.go:1597`
   и `:1740`) — живой превью иконки (onchange), ссылка на lucide.dev, нормализация
   имени (lower/kebab) при сохранении (`configurator.go`). Опц. `datalist` с
   курируемым списком.
6. Доки: обновить `docs/features.md` секцию «Подсистемы (разделы навигации)»
   (или новую) — как заполнять иконку и где взять имена (Lucide). status: testing.

## Тесты

- Рендер `.subsys-bar` с `Icon="shopping-cart"` → в HTML есть SVG/`<use>` этой
  иконки.
- Пустой `Icon` → нет битой разметки (фолбэк/ничего).
- `Icon="nonexistent-xyz"` → фолбэк, без паники и без пустого `href`.
- (A) юнит helper: `LucideIcon("shopping-cart")` непустой; `""`/неизвестное →
  дефолт.
- (B) webassets-тест по аналогии с `assets_test.go`: `/vendor/lucide/sprite.svg`
  отдаётся, 404 на несуществующее.

## Вне области

- Загрузка произвольных пользовательских SVG.
- Иконки у отдельных объектов (документов/справочников) — только подсистемы
  (и опц. страницы).

## Ветка / PR

- Ветка `feature/72-subsystem-icons` от `upstream/main` (не от текущей ветки
  переводов).
- Коммиты по-русски, трейлер `Generated-with: Claude Code`.
- PR в `upstream/main` (ivanarama/onebase) через `gh`.
