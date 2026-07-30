# План 110 — Внешнее (S3-совместимое) хранилище: бэкап и бинарники

**Статус:** ✅ Этап 1 (S3-выгрузка автобэкапа, PR #486). ✅ Этап 2a (S3-бэкенд
image-блобов, PR #487). ✅ Этап 2b (S3-бэкенд вложений, PR #488). ✅ Этап 2c
(опциональный S3-Range-стриминг раздачи вложений, ветка
`feature/110-s3-stream-attachments`). План 110 закрыт.
**Ветки:** `feature/110-s3-external-storage` (1), `feature/110-s3-blob-backend`
(2a), `feature/110-s3-attachments` (2b), `feature/110-s3-stream-attachments` (2c).
**Дата проектирования:** 2026-07-30.

## Контекст и мотив

Предложение из обсуждения (#393): выносить «тяжёлые» бинарники (картинки и т.п.)
на внешнее хранение; использовать каталог как файл-хранилище вместо СУБД;
автобэкап тоже отправлять на S3.

Разбор текущего состояния движка показал, что **два из трёх пунктов уже закрыты**:

- **Каталог вместо СУБД — это режим по умолчанию.** Картинки поля `image`
  хранятся ссылкой-UUID, а содержимое — `internal/storage/blobs.go`, режим `disk`
  (по умолчанию): файл на диске `filesDir/_blobs/<id>`, в `_blobs` только
  метаданные. Режим `db` (BLOB-колонка) — опция. Вложения к документам/справочникам
  (`internal/storage/attachments.go`) лежат на диске **всегда**, безусловно.
- **Бинарники уже вынесены из строк БД** — то самое «внешнее» относительно записи.

Реально новое во всём предложении — **объектное (S3-совместимое) хранилище** как
опциональный бэкенд. Оно нужно в двух независимых местах:

1. **Бэкап → S3** (off-site sink к уже работающему автобэкапу). ← этап 1, этот PR.
2. **S3-бэкенд блобов/вложений** (третий режим хранения). ← этап 2, отдельный PR.

Начинаем с (1): лучший баланс ценность/риск. Off-site-копия — то, ради чего S3
вообще нужен для DR; горячий транзакционный путь записи не затрагивается.

## Принципы (важны для обоих этапов)

- **Ноль новых зависимостей.** Идентичность OneBase — один самодостаточный бинарь,
  без CGo, с минимумом зависимостей. S3 реализуем на stdlib: подпись **AWS
  Signature V4** поверх `net/http` + `crypto/{hmac,sha256}`. Не тянем AWS SDK и
  minio-go (вес + supply-chain). Совместимо с AWS S3, MinIO, Ceph RGW, Backblaze
  B2/Wasabi и прочими S3-совместимыми.
- **Опциональность и офлайн.** Дефолтный путь (`disk`, локальный бэкап) не должен
  требовать сети никогда. S3 включается только явной секцией конфига.
- **Секреты — через окружение.** Ключи S3 задаются `${env:VAR}` (единый механизм,
  как SMTP-пароль и ключи ИИ — план 83), чтобы не жить в `app.yaml`/git/`.obz`.
  Секреты не едут в дамп конфигурации.
- **Переиспользуемый leaf-пакет.** S3-клиент — `internal/objstore`, без зависимостей
  на другие пакеты OneBase, чтобы им могли пользоваться и `backup` (этап 1), и
  `storage` (этап 2) без циклов импорта.

## Этап 1 — S3-выгрузка автобэкапа (этот PR)

### Пакет `internal/objstore` (новый, leaf)

Минимальный S3-клиент на stdlib с подписью SigV4:

- `New(cfg Config) (*Client, error)` — конфиг: `Endpoint`, `Region`, `Bucket`,
  `AccessKey`, `SecretKey`, `UseSSL`, `PathStyle`.
- `PutObject(ctx, key string, r io.Reader, size int64, contentType string) error`
  — одиночный PUT, `x-amz-content-sha256: UNSIGNED-PAYLOAD` (потоковая заливка без
  двойного чтения файла; допустимо всеми крупными S3-хранилищами). Лимит одиночного
  PUT — 5 ГБ (multipart — за рамками этапа 1, для типичного дампа хватает).
- `ListKeys(ctx, prefix string) ([]string, error)` — `ListObjectsV2`, с
  пагинацией по `IsTruncated`/`NextContinuationToken`.
- `DeleteObject(ctx, key string) error`.

Стиль URL: `PathStyle=true` (по умолчанию) → `scheme://endpoint/bucket/key`
(совместимо с MinIO/кастомными эндпойнтами и bucket-именами с точками);
`PathStyle=false` → virtual-host `scheme://bucket.endpoint/key`.

Корректность SigV4 проверяется юнит-тестом против документированного AWS-вектора
(GET Object, `AKIDEXAMPLE`/…, ожидаемая сигнатура из доки AWS).

### Конфиг (`internal/project/loader.go`)

`BackupConfig` получает опциональную секцию `s3`:

```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"
  keep_last: 7
  directory: ./backups      # локальная копия остаётся
  s3:
    endpoint: s3.amazonaws.com      # или minio.local:9000
    region: us-east-1
    bucket: my-onebase-backups
    prefix: prod/                    # ключ-префикс в бакете
    access_key: ${env:S3_ACCESS_KEY}
    secret_key: ${env:S3_SECRET_KEY}
    use_ssl: true
    path_style: true
    keep_last: 30                    # ротация в бакете (0 = не ротировать)
```

`${env:VAR}` в `endpoint`/`access_key`/`secret_key` раскрывается в `LoadConfig`
(новая `expandBackupEnv`, по образцу `expandWebhookEnv`).

### Встраивание (`internal/backup/auto.go`)

После успешного локального дампа и ротации: если `cfg.S3 != nil` — залить файл
ключом `prefix + basename(path)`, затем (при `S3.KeepLast > 0`) отротировать
объекты в бакете. Ошибка S3 не удаляет локальную копию, но возвращается наверх
(планировщик логирует) — падение off-site-выгрузки должно быть заметно.

Тестовый шов — как у существующего `autoDumper`: `createAutoBackup` принимает
фабрику `ObjectStore` (интерфейс `PutObject/ListKeys/DeleteObject`), в тестах —
фейковый стор, в проде — `objstore` поверх `project.S3Config`.

### Пользовательская видимость

`docs/features.md` — секция «Бэкап в S3 (off-site)», `status: testing`.

## Этап 2a — S3-бэкенд image-блобов (реализовано)

Третий режим `FileStorageS3 = "s3"`. `objstore` получил `GetObject`. В `DB` —
инжектируемый `BlobObjectStore` (интерфейс определён в `storage`; `objstore.Client`
удовлетворяет структурно — без импорта `storage → objstore`) + `SetBlobStore`.

- **Маршрутизация по колонке `loc`** (`disk`/`db`/`s3`; пусто = легаси).
  `PutBlob`/`OpenBlob`/`DeleteBlob` роутят по `loc`, а не по текущему режиму —
  смена режима не осиротит уже загруженные блобы. Каждый блоб знает своё место.
- **Раздача — проксированием.** Выбран вариант (а): `imageServe` без изменений
  стримит `OpenBlob`'s `io.ReadCloser` с прежней IDOR-проверкой. Публичные URL
  бакета не используются, presigned — отложено.
- **GC — без отдельного `Walk`.** `SweepOrphanBlobs` уже DB-driven и удаляет через
  `DeleteBlob`; достаточно было сделать `DeleteBlob` S3-aware. Инжекция клиента —
  в `gc-blobs` (иначе удаление s3-блоба вернёт понятную ошибку, а не осиротит объект).
- **Конфиг.** Режим — в `_settings` (`ui.file_storage=s3`); креды — `app.yaml`
  `file_storage.s3` (переиспользует `S3Config`), ключи через `${env:VAR}`, вне БД.
  Инжекция в `run`/`dev`/`procrun`/`gc-blobs`; невалидный конфиг фатален на старте.
- **Компенсация.** Заливка буферизуется в память (как db-режим) для Content-Length;
  при сбое INSERT после PUT объект удаляется. Известный зазор (паритет с диском):
  откат внешней DSL-транзакции осиротит объект в бакете (нет строки → GC не видит).

## Этап 2b — S3-бэкенд вложений (реализовано)

Тот же режим `ui.file_storage=s3` и тот же инжектируемый `BlobObjectStore`, что и
у блобов; ключи `[<prefix>/]attachments/<owner>/<id>`; колонка `_attachments.loc`
('' = легаси disk). Как решены файловые допущения:

- **Раздача через `http.ServeContent`** (нужен seekable): выбрана **материализация
  во временный файл** (по умолчанию) — `OpenAttachment` возвращает
  `io.ReadSeekCloser`; для s3 объект скачивается в temp-файл (seekable, Range
  работает), `Close` удаляет его. Опциональный стриминг — этап 2c ниже.
- **DSL `ПутьКВложению`** (нужен путь на диске): новый метод `MaterializeAttachment`
  — disk отдаёт реальный путь (cleanup=nil), s3 скачивает в temp-каталог с
  basename=ИД (чтобы demo-песочница восстановила id) и возвращает `cleanup`.
  Жизненный цикл — `context.AfterFunc(ctx, cleanup)`: temp живёт до конца
  запроса/прогона. `emailAttachmentPathResolver` для s3 авторизует по RLS и отдаёт
  свежую материализацию (path-match — только для disk).
- **Загрузка** — стейджинг во временный файл (ограничение памяти), потом PUT;
  компенсация при откате DSL-транзакции удаляет объект (паритет с диском).
- **Раздатчики не изменились**: `attachmentDownload`/v2 используют `f` как
  `io.ReadSeeker`+`Close` — новый тип подошёл без правок.

## Этап 2c — опциональный S3-Range-стриминг раздачи вложений (реализовано)

Флаг `file_storage.s3.stream: true` (по умолчанию false) переключает раздачу
S3-вложений с temp-файла на **ленивый Range-ридер** прямо из S3 — без локальной
копии. Оставлен опцией, т.к. у temp-файла есть своя польза (развязывает скорость
S3 от медленного клиента; полнота фич ServeContent).

- `objstore.OpenReadSeeker(ctx, key, size) io.ReadSeekCloser` + `rangeReader`:
  `Seek` только двигает оффсет (сети нет — `Seek(0,End)` отдаёт известный размер),
  `Read` открывает `GET Range: bytes=<pos>-`. Тело закрывается/переоткрывается при
  прыжке. `getFrom` принимает 200 и 206.
- Интеграция без правок раздатчиков: `BlobObjectStore` += `OpenReadSeeker`;
  `DB.blobStream` + `SetBlobStreaming`; `OpenAttachment` при `stream` отдаёт ридер
  вместо temp — `http.ServeContent` работает как раньше (Range → 206).
- Не влияет на картинки (уже стримятся через `io.Copy`) и на `ПутьКВложению`
  (всегда нужен реальный путь → temp).
- Тесты: `objstore` (full/range через Seek; **e2e через `ServeContent` с реальным
  206 Range-ответом**); `storage` (streaming `OpenAttachment`, temp не создаётся);
  парсинг `stream`.

## Definition of done

- **Этап 1:** ✅ `internal/objstore` (SigV4-вектор + round-trip httptest); `backup.s3`
  + `${env:}`; выгрузка/ротация в `auto.go`; `docs/features.md`; зелёные.
- **Этап 2a:** ✅ `objstore.GetObject`; `BlobObjectStore`+`loc` в `storage`;
  `file_storage.s3` + инжекция; тесты (роутинг, mode-switch, not-configured,
  put-error, real-client e2e через httptest); `docs/features.md`; зелёные.
- **Этап 2b:** ✅ `_attachments.loc` + маршрутизация Upload/Open/Delete;
  `MaterializeAttachment` (temp + `context.AfterFunc`) для DSL `ПутьКВложению` и
  email-резолвера; `OpenAttachment → io.ReadSeekCloser`; тесты (роутинг,
  mode-switch, not-configured, real-client e2e); `docs/features.md`; зелёные.
- **Этап 2c:** ✅ `objstore.OpenReadSeeker`/`rangeReader`; `file_storage.s3.stream`
  + `SetBlobStreaming`; стриминг в `OpenAttachment`; тесты (ServeContent+206 e2e,
  streaming open, config); `docs/features.md`; зелёные.
