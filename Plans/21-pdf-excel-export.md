# Этап 21 — Печать в PDF и экспорт в Excel

**Статус:** ⬜ Не начато

## Контекст

Без PDF и Excel нельзя передать накладную клиенту по email или выгрузить отчёт бухгалтеру. Это базовая потребность любого учётного приложения. Сейчас печатные формы доступны только как HTML в браузере.

## Синтаксис / UX

### PDF

На карточке документа рядом с «Печать» появляется «Скачать PDF». Кнопка отправляет GET-запрос:
```
GET /ui/documents/{name}/{id}/printform/{form}/pdf
→ Content-Type: application/pdf
→ Content-Disposition: attachment; filename="накладная_ПРД-2026-00001.pdf"
```

### Excel

На любом списке и отчёте появляется кнопка «Excel»:
```
GET /ui/catalog/{name}/excel
GET /ui/report/{name}/excel?param1=...
→ Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
→ Content-Disposition: attachment; filename="номенклатура_2026-05-06.xlsx"
```

Из DSL (для обработок):
```
Данные = ... // Массив строк
ФайлExcel = ВыгрузитьВExcel(Данные, "Отчёт");
// возвращает base64 содержимое xlsx-файла или путь к tmp-файлу
```

## Реализация PDF

**Вариант A — chromedp (headless Chrome):**
- Берём готовый HTML печатной формы → рендерим в PDF через CDP
- Плюс: идеальная печать, CSS работает
- Минус: нужен Chrome/Chromium в системе (~130 МБ зависимость)

**Вариант B — gofpdf / go-pdf:**
- Генерируем PDF из структуры печатной формы напрямую
- Плюс: нет зависимости от браузера, работает на сервере без UI
- Минус: ограниченный CSS

**Решение:** Вариант B для сервера (go-pdf), опциональный Вариант A через флаг `--pdf-engine=chrome`.

**Библиотека:** `github.com/jung-kurt/gofpdf` или `github.com/signintech/gopdf`.

## Реализация Excel

**Библиотека:** `github.com/xuri/excelize/v2`

**Новый файл:** `internal/excel/excel.go`
- `func ExportList(cols []string, rows [][]any) ([]byte, error)` — экспорт произвольной таблицы
- Форматирование заголовков (жирный шрифт, фон)
- Автоширина колонок
- Заморозка первой строки

## Изменения в коде

**`internal/ui/handlers.go`:**
- `GET /ui/documents/{name}/{id}/printform/{form}/pdf` → handler
- `GET /ui/catalog/{name}/excel` → handler
- `GET /ui/report/{name}/excel` → handler
- `GET /ui/journal/{name}/excel` → handler

**`internal/printform/render.go`:**
- функция `RenderPDF(form, doc) ([]byte, error)`

**`internal/excel/excel.go`:**
- новый пакет

## Тесты

- Юнит: ExportList → проверить что xlsx парсится и содержит правильные данные
- Интеграционный: endpoint возвращает Content-Type: application/pdf

## Эстимейт

4–5 дней.
