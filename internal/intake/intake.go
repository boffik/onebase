// Package intake — платформенный примитив надёжной приёмки входящих событий
// (план 90, пробел G6). Даёт две гарантии, недостижимые прикладным паттерном:
//
//   - идемпотентность через атомарный insert-if-new ключа (storage: UNIQUE +
//     ON CONFLICT DO NOTHING) — конкурентные дубли не создают двух бизнес-
//     объектов, нет гонки «проверил-потом-создал»;
//   - обработчик и отметка «обработано» в одной транзакции (storage.WithTx) —
//     сбой обработчика откатывает бизнес-объект целиком, полу-записи невозможны;
//     упавшее событие уходит в карантин (DLQ) для ручного replay.
//
// Транспорт остаётся за швом MessageSource: HTTP — эталон (план 61), нативный
// AMQP-consumer (G4/путь Б) подключается позже, не трогая это ядро.
package intake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

// Status — бизнес-исход приёмки (в отличие от транспортного ack).
type Status string

const (
	StatusAccepted    Status = "Принято"   // новое событие обработано, бизнес-объект создан
	StatusDuplicate   Status = "Дубль"     // событие уже принято ранее, повторной обработки нет
	StatusQuarantined Status = "Карантин"  // событие отложено в DLQ (сбой/несовпадение), ждёт replay
	StatusRejected    Status = "Отклонено" // событие отвергнуто без карантина (например нет ключа)
)

// Result — результат Ingest/Replay.
type Result struct {
	Status         Status
	ResultRef      string         // ссылка на созданный бизнес-объект (при Принято/Дубль)
	BusinessResult map[string]any // произвольный бизнес-результат обработчика
	Reason         string         // причина карантина/отклонения (dlq reason или тех. причина)
	DLQID          string         // id записи карантина, если создана
}

// HandlerResult — то, что обработчик возвращает движку.
type HandlerResult struct {
	Ref            string         // ссылка на созданный бизнес-объект (например UUID Заявки)
	BusinessResult map[string]any // произвольный бизнес-результат (для business_result и ответа)
}

// Handler — шов обработчика. Handle создаёт бизнес-объект, используя ctx (в нём
// уже открыта транзакция приёмки — все записи через storage к ней присоединятся,
// storage.WithTxIfNeeded). Ошибка Handle откатывает транзакцию и отправляет
// событие в карантин. Прикладная реализация запускает DSL-процедуру Обработать.
type Handler interface {
	Handle(ctx context.Context, env Envelope) (HandlerResult, error)
}

// HandlerFunc адаптирует функцию к интерфейсу Handler.
type HandlerFunc func(ctx context.Context, env Envelope) (HandlerResult, error)

// Handle реализует Handler.
func (f HandlerFunc) Handle(ctx context.Context, env Envelope) (HandlerResult, error) {
	return f(ctx, env)
}

// MessageSource — шов транспорта. Реализация принимает сырые сообщения из своего
// канала (HTTP-запрос, AMQP-доставка) и для каждого зовёт Engine.Ingest, затем
// ack/nack по Result. Ядро приёмки от транспорта не зависит; http реализуется
// первым (план 61), amqp (G4/путь Б) — позже, без правки Ingest.
type MessageSource interface {
	// Name — имя шлюза, чьи сообщения поставляет источник.
	Name() string
}

// Envelope — разобранный единый конверт входящего события.
type Envelope struct {
	Top     map[string]any // весь конверт (event_id, source, aggregate, payload, …)
	Payload map[string]any // тело события (конверт.payload)
	Raw     []byte         // исходные байты — сохраняются в карантин как есть
}

// ParseEnvelope разбирает сырой JSON-конверт.
func ParseEnvelope(raw []byte) (Envelope, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return Envelope{}, fmt.Errorf("intake: разбор конверта: %w", err)
	}
	env := Envelope{Top: top, Raw: raw}
	if p, ok := top["payload"].(map[string]any); ok {
		env.Payload = p
	} else {
		env.Payload = map[string]any{}
	}
	return env, nil
}

// Field возвращает строковое значение top-level поля конверта ("" если нет).
func (e Envelope) Field(name string) string {
	v, ok := e.Top[name]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// CorrelationID — сквозной идентификатор трассировки из конверта.
func (e Envelope) CorrelationID() string { return e.Field("correlation_id") }

// Engine исполняет приёмку поверх хранилища.
type Engine struct {
	Store *storage.DB
	Now   func() time.Time // инъекция часов для тестов; nil → time.Now
}

// New создаёт движок приёмки.
func New(store *storage.DB) *Engine { return &Engine{Store: store} }

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Ingest — приём одного события. Машина состояний:
//  1. канонизировать payload → payload_hash;
//  2. атомарный insert-if-new строки идемпотентности по (intake, scope, key);
//  3. строка новая → в одной транзакции запустить обработчик и отметить
//     processed; ошибка обработчика → откат + карантин;
//  4. строка есть, hash совпал → прежний результат (Дубль, без обработки);
//  5. строка есть, hash не совпал → карантин (schema_mismatch).
func (e *Engine) Ingest(ctx context.Context, in *metadata.Intake, h Handler, env Envelope) (Result, error) {
	key := env.Field(in.Idempotency.Key)
	if key == "" {
		// Без ключа дедупликация невозможна — отвергаем, чтобы отправитель починил
		// конверт (а не молча схлопывать все события в одну строку).
		return Result{Status: StatusRejected, Reason: "missing_idempotency_key"}, nil
	}
	scope := e.resolveScope(in, env)
	hash, err := payloadHash(env.Payload)
	if err != nil {
		return Result{}, err
	}

	ttlSec, err := in.TTLSeconds()
	if err != nil {
		return Result{}, err
	}
	now := e.now().Unix()
	ttlExpires := int64(0)
	if ttlSec > 0 {
		ttlExpires = now + ttlSec
	}

	inserted, err := e.Store.InsertIntakeLogIfNew(ctx, storage.IntakeLogRow{
		Intake:        in.Name,
		Scope:         scope,
		Key:           key,
		PayloadHash:   hash,
		Status:        storage.IntakeStatusReceived,
		CorrelationID: env.CorrelationID(),
		ReceivedAt:    now,
		TTLExpiresAt:  ttlExpires,
	})
	if err != nil {
		return Result{}, err
	}

	if !inserted {
		return e.onExisting(ctx, in, env, key, scope, hash)
	}
	return e.process(ctx, in, h, env, key, scope)
}

// onExisting разбирает случай «ключ уже занят»: дубль, карантин или несовпадение.
func (e *Engine) onExisting(ctx context.Context, in *metadata.Intake, env Envelope, key, scope, hash string) (Result, error) {
	row, found, err := e.Store.GetIntakeLog(ctx, in.Name, scope, key)
	if err != nil {
		return Result{}, err
	}
	if !found {
		// Гонка с TTL-очисткой между insert-конфликтом и чтением — редкий случай,
		// сообщаем ошибкой (транспорт переспросит).
		return Result{}, fmt.Errorf("intake %q: строка идемпотентности исчезла после конфликта", in.Name)
	}
	if row.PayloadHash != hash {
		// Тот же ключ, другое тело — несовпадение. Прежняя строка остаётся как есть,
		// оскорбительное тело — в карантин на разбор.
		return e.quarantine(ctx, in, env, key, scope, metadata.DLQSchemaMismatch,
			fmt.Sprintf("payload_hash: было %s, стало %s", shortHash(row.PayloadHash), shortHash(hash)), false)
	}
	if row.Status == storage.IntakeStatusQuarantined {
		return Result{Status: StatusQuarantined, Reason: "already_quarantined", ResultRef: row.ResultRef}, nil
	}
	// processed или received (in-flight) с тем же телом → Дубль без повторной обработки.
	var br map[string]any
	if row.BusinessResult != "" {
		_ = json.Unmarshal([]byte(row.BusinessResult), &br)
	}
	return Result{Status: StatusDuplicate, ResultRef: row.ResultRef, BusinessResult: br}, nil
}

// process запускает обработчик и отметку processed в одной транзакции. Ошибка
// обработчика откатывает бизнес-объект и отправляет событие в карантин.
func (e *Engine) process(ctx context.Context, in *metadata.Intake, h Handler, env Envelope, key, scope string) (Result, error) {
	var hr HandlerResult
	txErr := e.Store.WithTx(ctx, func(txctx context.Context) error {
		r, herr := h.Handle(txctx, env)
		if herr != nil {
			return herr
		}
		hr = r
		return e.Store.SetIntakeLogProcessed(txctx, in.Name, scope, key, r.Ref, marshalResult(r.BusinessResult))
	})
	if txErr != nil {
		return e.quarantine(ctx, in, env, key, scope, metadata.DLQHandlerError, txErr.Error(), true)
	}
	return Result{Status: StatusAccepted, ResultRef: hr.Ref, BusinessResult: hr.BusinessResult}, nil
}

// quarantine отправляет событие в карантин. ourRow=true — сбой на нашей свежей
// received-строке (её надо перевести в quarantined); false — несовпадение по
// чужой строке (её не трогаем). Если причина не входит в dlq.on — событие
// отклоняется без карантина, а осиротевшая received-строка снимается.
func (e *Engine) quarantine(ctx context.Context, in *metadata.Intake, env Envelope, key, scope, reason, errMsg string, ourRow bool) (Result, error) {
	if !in.QuarantineOn(reason) {
		if ourRow {
			if err := e.Store.DeleteIntakeLog(ctx, in.Name, scope, key); err != nil {
				return Result{}, err
			}
		}
		return Result{Status: StatusRejected, Reason: reason}, nil
	}
	var dlqID string
	err := e.Store.WithTx(ctx, func(txctx context.Context) error {
		id, err := e.Store.InsertIntakeDLQ(txctx, storage.IntakeDLQEntry{
			Intake:        in.Name,
			Key:           key,
			Scope:         scope,
			Payload:       string(env.Raw),
			Reason:        reason,
			Error:         errMsg,
			CorrelationID: env.CorrelationID(),
			QuarantinedAt: e.now().Unix(),
		})
		if err != nil {
			return err
		}
		dlqID = id
		if ourRow {
			return e.Store.SetIntakeLogStatus(txctx, in.Name, scope, key, storage.IntakeStatusQuarantined)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: StatusQuarantined, Reason: reason, DLQID: dlqID}, nil
}

// Replay переигрывает открытую запись карантина по ключу. Идемпотентность
// соблюдается: строка идемпотентности уже существует (quarantined), при успехе
// переходит в processed. Предыдущий сбойный проход ничего не закоммитил, поэтому
// дубля бизнес-объекта не возникает.
func (e *Engine) Replay(ctx context.Context, in *metadata.Intake, h Handler, key string) (Result, error) {
	entry, found, err := e.Store.GetOpenIntakeDLQ(ctx, in.Name, key)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{}, fmt.Errorf("intake %q: нет открытой записи карантина по ключу %q", in.Name, key)
	}
	env, err := ParseEnvelope([]byte(entry.Payload))
	if err != nil {
		return Result{}, err
	}
	scope := entry.Scope

	var hr HandlerResult
	txErr := e.Store.WithTx(ctx, func(txctx context.Context) error {
		r, herr := h.Handle(txctx, env)
		if herr != nil {
			return herr
		}
		hr = r
		return e.Store.SetIntakeLogProcessed(txctx, in.Name, scope, key, r.Ref, marshalResult(r.BusinessResult))
	})
	if txErr != nil {
		if err := e.Store.BumpIntakeDLQAttempts(ctx, entry.ID, txErr.Error()); err != nil {
			return Result{}, err
		}
		return Result{Status: StatusQuarantined, Reason: metadata.DLQHandlerError, DLQID: entry.ID}, nil
	}
	if err := e.Store.SetIntakeDLQState(ctx, entry.ID, storage.DLQStateReplayed); err != nil {
		return Result{}, err
	}
	return Result{Status: StatusAccepted, ResultRef: hr.Ref, BusinessResult: hr.BusinessResult}, nil
}

// resolveScope собирает пространство ключа из полей конверта, перечисленных в
// idempotency.scope, соединяя значения через "|".
func (e *Engine) resolveScope(in *metadata.Intake, env Envelope) string {
	if len(in.Idempotency.Scope) == 0 {
		return ""
	}
	parts := make([]string, 0, len(in.Idempotency.Scope))
	for _, s := range in.Idempotency.Scope {
		parts = append(parts, env.Field(s))
	}
	return strings.Join(parts, "|")
}

// payloadHash канонизирует payload (JSON с отсортированными ключами —
// encoding/json сортирует ключи map детерминированно) и берёт sha256.
func payloadHash(payload map[string]any) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("intake: канонизация payload: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func marshalResult(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// RejectError — обработчик может обернуть в неё бизнес-отказ, чтобы отличить
// «событие отвергнуто по правилам» от технического сбоя. Зарезервировано для
// стыка с валидацией (план 89) на транспортном уровне (маппинг в HTTP 422).
type RejectError struct{ Reason string }

func (e *RejectError) Error() string { return "intake reject: " + e.Reason }

// AsReject сообщает, что ошибка — бизнес-отказ RejectError.
func AsReject(err error) (*RejectError, bool) {
	var re *RejectError
	if errors.As(err, &re) {
		return re, true
	}
	return nil, false
}
