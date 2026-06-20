# Этап 32 — Декларативный дисплей покупателя

## Контекст

В подсистеме оборудования уже есть **декларативные драйверы**, чей протокол задан данными, а не Go-кодом:
- `scripted` (`internal/equipment/scripted.go`) — весы: запрос (hex) + regexp разбора ответа → число. Реализует `Scale`.
- `scripted_pay` (`internal/equipment/scripted_pay.go`) — эквайринг: текстовый шаблон запроса + признак одобрения + regexp полей. Реализует `PaymentTerminal`.

Оба работают через существующие DSL-методы и эндпоинты агента **без их изменения**, потому что реализуют стандартные интерфейсы.

Чего не хватает: **дисплей** покупателя пока только зашитый `display`/`display_tcp` (`internal/equipment/display.go`, протокол CD5220). Хочется так же описывать дисплей данными в справочнике (`Драйвер=scripted_display`, шаблоны команд).

## Почему отложили (и в чём сложность)

В отличие от весов/оплаты (чистый текст), у дисплея команды **бинарные** (`ESC Q A …`, `0x0C`) и **перемешаны с текстом**, плюс **кодировка** кириллицы (часто CP866, не UTF-8). Текстовое декларирование regexp'ом тут не подходит — нужен формат «hex-команды + плейсхолдер текста».

## Подход: драйвер `scripted_display`

`internal/equipment/scripted_display.go`, реализует `CustomerDisplay` (`ShowLines`, `Clear`). Протокол — из параметров:

- `КомандаИниц` — hex (напр. `1B40` = ESC @)
- `КомандаОчистки` — hex (напр. `0C`)
- `ШаблонСтроки1`, `ШаблонСтроки2` — строка вида `1B5141{text}0D` (hex-префикс + `{text}` + hex-суффикс)
- `Ширина` — число (дополнение/обрезка строки)
- `Кодировка` — `cp866` | `utf8` (по умолчанию `cp866` для VFD)

### Формат шаблона строки

Разбирается на сегменты: hex-байты вне `{text}` декодируются как есть, `{text}` заменяется на байты строки в нужной кодировке.

```
"1B5141{text}0D"  →  [0x1B 0x51 0x41] + encode(text, cp866) + [0x0D]
```

### Реализация (эскиз)

```go
func init() { Register("scripted_display", func() Device { return &scriptedDisplayDevice{width: 20} }) }

type scriptedDisplayDevice struct {
	conn    io.WriteCloser   // openWriteTransport — TCP или serial
	width   int
	init    []byte
	clear   []byte
	lineTpl [][2][]byte      // для каждой строки: {префикс, суффикс}
	encode  func(string) []byte
}

func (d *scriptedDisplayDevice) ShowLines(lines []string) error {
	var buf bytes.Buffer
	buf.Write(d.init)
	buf.Write(d.clear)
	for i, tpl := range d.lineTpl {
		text := ""
		if i < len(lines) { text = lines[i] }
		buf.Write(tpl[0])
		buf.Write(d.encode(fit(text, d.width)))
		buf.Write(tpl[1])
	}
	return write(d.conn, buf.Bytes())
}
```

Транспорт — существующий `openWriteTransport` (TCP/serial), как у `escpos`/`display`. Парсинг шаблона `1B5141{text}0D` на префикс/суффикс — в `Connect`.

## Кодировка CP866

Дисплеи/принтеры РФ обычно ждут CP866 для кириллицы. Нужна функция `utf8 → cp866`:
- Таблица соответствия (диапазоны А-Я, а-я, Ё/ё) или `golang.org/x/text/encoding/charmap.CodePage866`.
- `golang.org/x/text` — проверить, есть ли в графе (вероятно через зависимости); иначе `go get`.

Это же пригодится зашитым `escpos`/`display` (сейчас они шлют UTF-8, что на реальном железе даст кракозябры) — стоит вынести `encode` в общий helper и применить везде.

## DSL и агент — без изменений

`scripted_display` реализует `CustomerDisplay` → автоматически работает через:
- DSL `Дисплей.Показать(стр1, стр2)` (`asDisplay` в `equipment_builtins.go`),
- агент `POST /display`.

Ничего в этих слоях менять не нужно — как и с `scripted`/`scripted_pay`.

## Тесты (без железа)

- Пакетный: эмулятор на TCP-сокете (как `display_test.go`), `Open("scripted_display", {шаблоны…})`, `ShowLines(["А","Б"])`, проверить, что в потоке есть `init`, `clear`, hex-префиксы строк и **байты текста в CP866**.
- Тест парсинга шаблона `1B5141{text}0D` → префикс `[1B 51 41]`, суффикс `[0D]`.
- Тест кодировки: `encode("Привет")` == ожидаемые CP866-байты.
- DSL-тест: `ПодключитьОборудование("scripted_display", …).Показать(...)` через интерпретатор.

## Файлы

- `internal/equipment/scripted_display.go` (+ `_test.go`)
- `internal/equipment/encoding.go` — общий `cp866`-энкодер (применить и к `escpos`/`display`)
- Опционально: пример в `examples/trade` — карточка дисплея с `Драйвер=scripted_display` в справочнике.

## Решения

- Объём небольшой (один драйвер + энкодер + тесты), главная тонкость — кодировка и парсинг hex-шаблона.
- Бонус: вынос CP866-энкодера чинит кириллицу у уже существующих принтера/дисплея на реальном железе.
