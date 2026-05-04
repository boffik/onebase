# Этап 10 — Подсистемы (разделы навигации)

## Контекст

В 1С — горизонтальная панель «Продажи / Закупки / Склад / Финансы» с группировкой объектов по бизнес-разделам. Сейчас в onebase плоская левая nav-панель — у крупного приложения (десятки справочников и документов) всё в одном списке станет неудобно.

Эта фича добавляет конфигурационный объект «Подсистема» и двухуровневую навигацию: верхний таб-бар разделов + левая nav в рамках выбранного раздела.

---

## YAML

`subsystems/продажи.yaml`:

```yaml
name: Продажи
title: Продажи
icon: shopping-cart      # имя из feather/material-icons
order: 10                # порядок в верхнем таб-баре
contents:
  documents: [Реализация, ВозвратОтПокупателя]
  catalogs:  [Контрагент, Номенклатура]
  reports:   [ОстаткиТоваров, Продажи]
  inforegs:  [ЦеныНоменклатуры]
  registers: []
  processors: [ПересчётЦен]
```

Один объект может входить в несколько подсистем (как в 1С: «Контрагент» — и в «Продажах», и в «Закупках»).

---

## Связь с RBAC

Пользователь видит подсистему, если есть `read` хотя бы на одну сущность внутри. Иначе подсистема скрыта.

Внутри подсистемы при формировании nav — фильтрация по правам пользователя (объект с `read=false` в подсистеме не отображается).

---

## Изменения в коде

### `internal/metadata/subsystem.go` (новый)

```go
type Subsystem struct {
    Name, Title, Icon string
    Order             int
    Contents          SubsystemContents
}

type SubsystemContents struct {
    Documents  []string
    Catalogs   []string
    Reports    []string
    InfoRegs   []string
    Registers  []string
    Processors []string
}

func LoadSubsystemFile(path string) (*Subsystem, error)
func LoadSubsystemDir(dir string) ([]*Subsystem, error)
```

### `internal/project/loader.go`

```go
type Project struct {
    ...
    Subsystems []*metadata.Subsystem
}

func (p *Project) loadSubsystems() error {
    dir := filepath.Join(p.Dir, "subsystems")
    ...
}
```

### `internal/ui/server.go` (`buildNav`)

```go
func (s *Server) buildNav(user *auth.User, currentSubsystem string) []NavGroup {
    if len(s.reg.Subsystems()) == 0 {
        return s.buildFlatNav(user)  // фолбэк — текущее поведение
    }
    
    var nav []NavGroup
    sub := s.reg.GetSubsystem(currentSubsystem)
    if sub == nil {
        return s.buildFlatNav(user)
    }
    
    // Фильтрация по contents подсистемы и правам пользователя
    nav = append(nav, NavGroup{
        Title: "Справочники",
        Items: filterByPerm(sub.Contents.Catalogs, "catalog", user),
    })
    // ... аналогично для остальных kind
    return nav
}
```

URL: `/ui/?subsystem=Продажи` сохраняет выбор. Если параметра нет — открывается первая подсистема (по `order`).

### `internal/ui/templates.go`

В шаблоне layout — горизонтальный таб-бар сверху:

```html
{{if .Subsystems}}
<nav class="subsystems-bar">
  {{range .Subsystems}}
  <a class="subsystem {{if eq .Name $.CurrentSubsystem}}active{{end}}"
     href="/ui/?subsystem={{.Name | urlquery}}">
    <i class="icon-{{.Icon}}"></i> {{.Title}}
  </a>
  {{end}}
</nav>
{{end}}
```

### `internal/launcher/configurator.go`

Секция «Подсистемы» в дереве. В правой панели:
- Имя, заголовок, иконка, порядок
- Список contents с возможностью добавлять/удалять (drag-n-drop из других разделов дерева — опционально)

### `internal/runtime/registry.go`

```go
func (r *Registry) GetSubsystem(name string) *metadata.Subsystem
func (r *Registry) Subsystems() []*metadata.Subsystem  // отсортировано по Order
```

---

## Тесты

### `internal/ui/subsystem_test.go`

```go
func TestBuildNav_FilteredBySubsystem(t *testing.T) {
    s := &Subsystem{
        Name: "Продажи",
        Contents: SubsystemContents{
            Catalogs:  []string{"Контрагент"},
            Documents: []string{"Реализация"},
        },
    }
    user := &auth.User{IsAdmin: true}
    nav := buildNavForSubsystem(s, user, allEntities)
    
    catNames := navItemNames(nav, "Справочники")
    assert.Equal(t, []string{"Контрагент"}, catNames)
    assert.NotContains(t, catNames, "Номенклатура")  // не входит в Продажи
}

func TestBuildNav_HiddenIfNoPermissions(t *testing.T) { /* ... */ }
```

---

## Verification

1. В `examples/trade/subsystems/` создать 3 подсистемы: «Продажи», «Закупки», «Склад».
2. Запуск `onebase dev` → в UI появляется верхний таб-бар с 3 пунктами.
3. Клик на «Продажи» → левая nav сужена до Реализаций, Контрагентов, отчёта Продажи.
4. Клик на «Склад» → видны только Поступления, Списания, Остатки.
5. Без подсистем (удалить папку) — UI как раньше, плоская nav.
6. Логин как менеджер с правами только на Контрагенты — подсистему «Склад» не видит вообще, в «Продажах» видит только Контрагентов.
7. `DEVELOPER.md` — раздел «Подсистемы».

---

## Эстимейт: 2–3 дня

- Метаданные + загрузка: 0.5 дня
- UI (таб-бар + фильтрация nav): 1.5 дня
- Конфигуратор: 0.5 дня
- Тесты + пример: 0.5 дня
