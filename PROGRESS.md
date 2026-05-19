# Прогресс по веткe `fix/platform-bugs`

Исходный список: `c:\Project\trade\замечания_по_платформе.md` (22 пункта).
В этой ветке чиним только **явные баги**. Фичи и архитектурные вопросы — обсуждаются отдельным Issue.

## Чек-лист

- [x] **#13** — в `*.proc.os` видна только одна процедура → добавлен `Interpreter.LookupSiblingProc`, резолвит helper-процедуры по файлу. `Registry.GetSiblingProc(currentFile, name)` сканирует `r.procs` с фильтром по `Name.File`. Подключено в `cli/run.go`, `cli/dev.go`. Тесты `TestInterpreter_SiblingProc*` в `interpreter_test.go`.
- [x] **#18** — виртуальное поле `Ссылка` в запросах не работает → проверка `prevDot` в `internal/query/query.go:1238` убрана; теперь `Ссылка`/`Reference`/`Ref` транслируется в `id` и с префиксом (`Н.Ссылка`), и без (`Ссылка`). Тесты `TestCompile_Ssylka_Bare`, `TestCompile_Ssylka_InWhere` в `query_test.go`.
- [x] **#20** — `Справочники.X.ИмяПредопределённой` → теперь возвращает `*Ref`. Добавил `CatalogsRoot/CatalogProxy` в `internal/dsl/interpreter/catalogs_proxy.go`, зарегистрировал как `Справочники`/`Catalogs` в `internal/ui/handlers.go:1454`.
- [x] **#21** — `НайтиПоНаименованию` / `НайтиПоКоду` → реализованы как методы `CatalogProxy`. Storage-метод `FindCatalogByField` в `internal/storage/predefined.go`.
- [x] **#22** — `Если/Иначе` теряет присваивание во внешнем scope → код уже корректен (`internal/dsl/interpreter/env.go:69-79`, fix `feae0205`). Добавлен регресс-тест `scope_test.go` (4 кейса). **Запустить:** `go test ./internal/dsl/interpreter/ -run TestIfElseScope -v`
- [x] **#17** — Ref-типы в измерениях регистров → починено коммитом `1ffa7bf` (resolveRefArg в `internal/storage/register.go:19-35`)

## Стратегия

Каждый баг — отдельный коммит с префиксом `fix:` и пояснением. После того, как все пройдут, PR в `ivanarama/onebase` со ссылкой на исходный Issue.

## Малые фичи (вторая партия)

- [x] **#3** — `ПустаяСсылка(x)` / `IsEmptyRef` — узкий предикат, не путается с `Пустая` (0/Ложь — НЕ пустая ссылка). `internal/dsl/interpreter/builtins_ext.go`.
- [x] **#8** — `ЧислоПрописью(сумма, валюта)` — со склонением рубль/рубля/рублей, поддержка USD/EUR. `internal/dsl/interpreter/amount_in_words.go`.
- [x] **#12** — параметры функций по умолчанию: `Функция X(А, Б = 20)`. AST поле `Defaults`, парсер + интерпретатор.
- [x] **#19а** — служебные имена `period` / `вид_движения` / `recorder` / `line_number` доступны как `Период` / `ВидДвижения` / `Регистратор` / `НомерСтроки`. `systemColAlias()` в `query.go`.

## Не входит в эту ветку (требуют обсуждения)

**🟡 Средние** (~неделя суммарно):
#4 атрибуты в VT, #5 вызов модулей из proc/posting (частично #13 решил),
#6 scope нумераторов, #9 округление в ФИФО, #10 приоритет печатных форм,
#11 demo vs prod, #15 события форм для ТЧ, #16 hot reload `.os`,
#19б трансляция функций `НачалоДня`/`КонецДня` в SQL.

**🔴 Архитектурные** (недели работы каждая):
#1 МоментВремени у VT, #2 управляемые блокировки, #7 «реквизит журнала»,
#14 cross-ref в predefined (deferred FK).
