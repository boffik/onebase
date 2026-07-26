package ui

// Транспорт входных шлюзов (план 90, заход 1/90C, вариант «intake владеет
// маршрутом»). Приёмник делит префикс /hs/ с HTTP-сервисами (план 61), поэтому
// диспетчер /hs/* сперва пытается сопоставить путь с endpoint шлюза, и лишь затем
// уходит в сервисы. Транспортные хелперы (net-gate, чтение тела, constant-time
// сравнение секрета) переиспользуются из services.go.
//
// Обработчик шлюза — DSL-процедура Обработать(Конверт) в модуле handler. Она
// запускается ВНУТРИ транзакции Ingest (ctx уже несёт транзакцию), поэтому записи
// справочников/документов вливаются в неё: сбой обработчика откатывает и
// бизнес-объект, и отметку идемпотентности разом.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/intake"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

// dispatchIntake обслуживает запрос как входной шлюз, если его путь совпадает с
// endpoint какого-либо http-шлюза. true — запрос обслужен (диспетчер /hs/* не
// идёт дальше). Реестр читается вживую → --watch подхватывает новые шлюзы.
func (s *Server) dispatchIntake(w http.ResponseWriter, r *http.Request) bool {
	var in *metadata.Intake
	for _, cand := range s.reg.Intakes() {
		if cand.Transport == metadata.IntakeTransportHTTP && cand.Endpoint == r.URL.Path {
			in = cand
			break
		}
	}
	if in == nil {
		return false
	}
	// Событие постится в тело — принимаем только POST.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeServiceError(w, http.StatusMethodNotAllowed, "приёмник принимает только POST")
		return true
	}

	// Тело читаем целиком ДО аутентификации (hmac подписывает тело), с лимитом.
	var body []byte
	if r.Body != nil {
		var rerr error
		body, rerr = io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxFileSizeBytes))
		r.Body.Close()
		if rerr != nil {
			var maxErr *http.MaxBytesError
			if errors.As(rerr, &maxErr) {
				writeServiceError(w, http.StatusRequestEntityTooLarge, "тело запроса превышает допустимый размер")
			} else {
				writeServiceError(w, http.StatusBadRequest, "не удалось прочитать тело запроса")
			}
			return true
		}
	}

	if !s.resolveIntakeAuth(in, w, r, body) {
		return true // 401 уже отправлен
	}

	env, err := intake.ParseEnvelope(body)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err.Error())
		return true
	}

	proc := s.reg.GetModuleNamespacedProc(in.Handler, "Обработать")
	if proc == nil {
		writeServiceError(w, http.StatusInternalServerError,
			"обработчик Обработать не найден в модуле "+in.Handler)
		return true
	}

	opCtx, finish, ok := s.beginOperation(r, opHTTPServiceRun, in.Name+".Обработать")
	if !ok {
		writeServiceError(w, http.StatusTooManyRequests, "слишком много одновременно выполняемых приёмников, повторите позже")
		return true
	}
	opStatus := "ok"
	defer func() { finish(opStatus, 0, false) }()

	eng := intake.New(s.store)
	handler := &dslIntakeHandler{s: s, proc: proc, timeout: s.operationTimeout(opHTTPServiceRun)}
	res, err := eng.Ingest(opCtx, in, handler, env)
	if err != nil {
		opStatus = operationStatus(opCtx, err)
		writeServiceError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	s.writeIntakeResult(w, res)
	return true
}

// resolveIntakeAuth проверяет подлинность отправителя по auth шлюза. body нужен
// режиму hmac. false — запрос отклонён (401 уже отправлен). Логика зеркалит
// resolveServiceAuth (token/hmac), но берёт секрет из объявления шлюза.
func (s *Server) resolveIntakeAuth(in *metadata.Intake, w http.ResponseWriter, r *http.Request, body []byte) bool {
	switch in.Auth {
	case metadata.IntakeAuthNone, "":
		return true
	case metadata.IntakeAuthToken:
		if in.Secret == "" {
			writeServiceError(w, http.StatusUnauthorized, "секрет приёмника не задан")
			return false
		}
		got := r.Header.Get("X-Webhook-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(in.Secret)) != 1 {
			writeServiceError(w, http.StatusUnauthorized, "неверный токен")
			return false
		}
		return true
	case metadata.IntakeAuthHMAC:
		if in.Secret == "" {
			writeServiceError(w, http.StatusUnauthorized, "секрет приёмника не задан")
			return false
		}
		mac := hmac.New(sha256.New, []byte(in.Secret))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		got := strings.TrimPrefix(strings.ToLower(r.Header.Get("X-Webhook-Signature")), "sha256=")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeServiceError(w, http.StatusUnauthorized, "неверная подпись")
			return false
		}
		return true
	default:
		writeServiceError(w, http.StatusInternalServerError, "неизвестный режим аутентификации приёмника: "+in.Auth)
		return false
	}
}

// writeIntakeResult отображает бизнес-результат приёмки в HTTP-ответ (ack-политика):
// Принято/Дубль → 200, Отклонено → 422, Карантин → 202 (принято в карантин, штормом
// не ретраить). Тело — JSON со статусом и деталями.
func (s *Server) writeIntakeResult(w http.ResponseWriter, res intake.Result) {
	body := map[string]any{"status": string(res.Status)}
	if res.ResultRef != "" {
		body["ref"] = res.ResultRef
	}
	if res.BusinessResult != nil {
		body["result"] = res.BusinessResult
	}
	if res.Reason != "" {
		body["reason"] = res.Reason
	}
	if res.DLQID != "" {
		body["dlq_id"] = res.DLQID
	}
	code := http.StatusOK
	switch res.Status {
	case intake.StatusRejected:
		code = http.StatusUnprocessableEntity
	case intake.StatusQuarantined:
		code = http.StatusAccepted
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// dslIntakeHandler запускает DSL-процедуру Обработать(Конверт) как обработчик
// шлюза. ctx уже несёт транзакцию Ingest — записи процедуры вливаются в неё.
type dslIntakeHandler struct {
	s       *Server
	proc    *ast.ProcedureDecl
	timeout time.Duration
}

// Handle реализует intake.Handler.
func (h *dslIntakeHandler) Handle(ctx context.Context, env intake.Envelope) (intake.HandlerResult, error) {
	conv := interpreter.JSONValueToDSL(env.Top) // конверт как *Map (как результат ПрочитатьJSON)
	var msgs []string
	mc := runtime.NewMovementsCollector("intake", uuid.Nil)
	dslVars := h.s.buildDSLVarsWithMessages(ctx, mc, &msgs)
	dslVars["Конверт"] = conv
	dslVars["Envelope"] = conv

	var (
		result any
		err    error
	)
	if h.timeout > 0 {
		result, err = h.s.interp.CallSandboxed(h.proc, nil, []any{conv},
			interpreter.SandboxProfile{MaxWallClock: h.timeout}, dslVars)
	} else {
		result, err = h.s.interp.Call(h.proc, nil, []any{conv}, dslVars)
	}
	if err != nil {
		return intake.HandlerResult{}, err
	}
	return mapHandlerResult(result), nil
}

// mapHandlerResult приводит возврат DSL-обработчика к intake.HandlerResult.
// Поддержаны: ссылка (Документы.X.Записать()/.Ссылка) → Ref; строка → Ref;
// Структура/Соответствие → бизнес-результат (+ поле ссылка/ref/id как Ref).
func mapHandlerResult(result any) intake.HandlerResult {
	if result == nil {
		return intake.HandlerResult{}
	}
	if ref, ok := result.(interface{ GetRefUUID() string }); ok {
		return intake.HandlerResult{Ref: ref.GetRefUUID()}
	}
	if s, ok := result.(string); ok {
		return intake.HandlerResult{Ref: s}
	}
	var br map[string]any
	if data, err := interpreter.MarshalDSLValue(result); err == nil {
		_ = json.Unmarshal(data, &br) // объект → map; иначе br = nil
	}
	ref := ""
	for _, k := range []string{"ссылка", "ref", "id"} {
		if v, ok := br[k]; ok {
			if sv, ok := v.(string); ok && sv != "" {
				ref = sv
				break
			}
		}
	}
	return intake.HandlerResult{Ref: ref, BusinessResult: br}
}
