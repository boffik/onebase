# Этап 20 — Email-уведомления (SMTP)

**Статус:** ⬜ Не начато

## Контекст

Отсутствие email — блокер для: уведомлений клиентов о заказах, еженедельных отчётов, «забыли пароль», алёртов из регламентных заданий. Один из самых часто запрашиваемых каналов связи в малом бизнесе.

## Синтаксис DSL

```
// Простое письмо
ОтправитьПисьмо("client@example.com", "Ваш заказ принят", "Добрый день, ваш заказ №" + Строка(this.Номер) + " принят.");

// Расширенный вариант
Письмо = Новый ПисьмоEmail;
Письмо.Кому     = "client@example.com";
Письмо.Копия    = "manager@company.ru";
Письмо.Тема     = "Заказ №" + Строка(this.Номер);
Письмо.Текст    = "Добрый день, ...";
Письмо.HTMLТело = "<p>Добрый день, ...</p>";
Письмо.Отправить();
```

## Настройка SMTP

В `config/app.yaml`:
```yaml
email:
  smtp_host: smtp.gmail.com
  smtp_port: 587
  smtp_user: noreply@company.ru
  smtp_password: env:SMTP_PASSWORD     # берётся из переменной окружения
  from_name: "Мой склад"
  from_address: noreply@company.ru
```

Или через **Администрирование → Константы** (если email-константы добавлены в конфигурацию).

## Изменения в коде

**Новый файл:** `internal/dsl/interpreter/email_builtins.go`
- тип `dslEmail` с полями Кому/Копия/Тема/Текст/HTMLТело
- функция `ОтправитьПисьмо(кому, тема, текст)`
- объект `Новый ПисьмоEmail` через dispatch

**Новый файл:** `internal/mailer/mailer.go`
- структура `Config` (SMTP-параметры)
- функция `Send(to, subject, body, htmlBody string) error`
- поддержка TLS (STARTTLS и SSL/465)
- инжекция в интерпретатор через `extraVars`

**`internal/project/scaffold.go`:**
- поле `Email *EmailConfig` в `AppConfig`

## Зависимость

- Библиотека `net/smtp` (stdlib) + `github.com/jordan-wright/email` для HTML/multipart

## Тесты

- Юнит: mock SMTP-сервер (`smtp4dev` или `mailhog` в тесте через `net/smtp`)
- Проверка: письмо с правильными заголовками и телом

## Эстимейт

3 дня.
