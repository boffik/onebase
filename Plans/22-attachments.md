# Этап 22 — Вложения к документам и справочникам (File Attachments)

**Статус:** ⬜ Не начато

## Контекст

«Прикрепить скан договора», «фото товара в карточке», «акт от поставщика» — типичные сценарии любой CRM и учётной системы. Без вложений платформа не закрывает paperless-офис.

## Синтаксис / UX

На карточке документа/справочника появляется секция **«Вложения»**:
- список файлов (имя, размер, дата, кнопка скачать)
- кнопка «Прикрепить» (drag-and-drop или file picker)
- кнопка удаления вложения

```
GET  /ui/documents/{name}/{id}/attachments           → JSON список
POST /ui/documents/{name}/{id}/attachments           → multipart/form-data, возвращает id
GET  /ui/attachments/{attachment-id}/download        → файл
DELETE /ui/attachments/{attachment-id}              → удаление
```

## Хранение

**Таблица `_attachments`** (создаётся платформой автоматически):
```sql
CREATE TABLE IF NOT EXISTS _attachments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_kind  TEXT NOT NULL,   -- 'catalog' / 'document'
    owner_name  TEXT NOT NULL,   -- 'Контрагент' / 'Реализация'
    owner_id    UUID NOT NULL,
    filename    TEXT NOT NULL,
    mime_type   TEXT NOT NULL,
    size_bytes  BIGINT NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    uploaded_by TEXT
);
```

**Файлы на диске:** `~/.onebase/files/<base-id>/<attachment-id>`

Для облачного хранения (опционально, позже): S3-compatible через `aws-sdk-go`.

## Ограничения

Настройки в `config/app.yaml`:
```yaml
attachments:
  max_file_size_mb: 50
  allowed_types: [pdf, png, jpg, docx, xlsx]   # пустой = все типы
```

## DSL-доступ

```
// Получить список вложений текущего документа
Вложения = this.ПолучитьВложения();
Для Каждого В Из Вложения Цикл
    Сообщить(В.ИмяФайла + " " + Строка(В.Размер) + " байт");
КонецЦикла;
```

## Изменения в коде

**Новый файл:** `internal/storage/attachments.go`
- `CreateTable(ctx, db)`
- `List(ctx, ownerKind, ownerName, ownerID)` → `[]Attachment`
- `Upload(ctx, ownerKind, ownerName, ownerID, filename, mime, reader) (Attachment, error)`
- `Download(ctx, id) (io.ReadCloser, Attachment, error)`
- `Delete(ctx, id) error`

**`internal/ui/handlers.go`:**
- маунт `/ui/documents/{name}/{id}/attachments`
- маунт `/ui/attachments/{id}/download`

**`internal/ui/templates.go`:**
- секция вложений в карточке документа/справочника

## Тесты

- Upload → List → Download → Delete round-trip
- Проверка ограничений по размеру и типу

## Эстимейт

5–7 дней.
