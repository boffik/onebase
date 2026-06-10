package ui

// HTTP-сервисы (план 52) — серверная сторона по аналогии с «HTTPСервис» 1С.
// Конфигурация публикует собственные REST-эндпоинты под /hs/<корень>/…, а их
// обработчики пишутся на DSL (src/<имя>.service.os). Здесь — маршрутизация,
// аутентификация по сервису и запуск обработчика с полным набором DSL-
// переменных (тот же, что у обработок: Запрос/Документы/Справочники/…).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/httpservice"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

// MountServices монтирует поверхность HTTP-сервисов. Регистрируется на верхнем
// уровне роутера (вне session-middleware веб-интерфейса): каждый сервис сам
// объявляет свою аутентификацию (none/basic/session), поэтому публичные
// сервисы-приёмники вебхуков работают без cookie. Диспетчер читает реестр
// «вживую», поэтому --watch подхватывает новые сервисы без рестарта.
func (s *Server) MountServices(r chi.Router) {
	r.Handle("/hs", http.HandlerFunc(s.serviceIndex))
	r.Handle("/hs/", http.HandlerFunc(s.serviceIndex))
	// Статические маршруты приоритетнее catch-all /hs/* в chi.
	r.Get("/hs/docs", s.serviceDocs)
	r.Get("/hs/docs/rapidoc-min.js", s.serviceDocsAsset)
	r.Handle("/hs/*", http.HandlerFunc(s.serviceDispatch))
}

// serviceIndex — GET /hs: машиночитаемый список опубликованных сервисов
// (имя, заголовок, корневой URL, шаблоны и методы). Помогает интеграторам и
// заменяет ?wsdl-дискавери из мира SOAP.
func (s *Server) serviceIndex(w http.ResponseWriter, r *http.Request) {
	type tmplInfo struct {
		Template string   `json:"template"`
		Methods  []string `json:"methods"`
	}
	type svcInfo struct {
		Name      string     `json:"name"`
		Title     string     `json:"title,omitempty"`
		RootURL   string     `json:"root_url"`
		BaseURL   string     `json:"base_url"`
		Auth      string     `json:"auth"`
		Roles     []string   `json:"roles,omitempty"`
		Templates []tmplInfo `json:"templates"`
	}
	services := s.reg.HTTPServices()
	out := make([]svcInfo, 0, len(services))
	for _, svc := range services {
		info := svcInfo{Name: svc.Name, Title: svc.Title, RootURL: svc.RootURL, BaseURL: "/hs/" + svc.RootURL, Auth: svc.Auth, Roles: svc.Roles}
		for _, t := range svc.Templates {
			info.Templates = append(info.Templates, tmplInfo{Template: t.Template, Methods: sortedMethods(t.Methods)})
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RootURL < out[j].RootURL })
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"services": out})
}

// serviceDispatch разбирает /hs/<корень>/<путь>, находит сервис и шаблон,
// проверяет метод и аутентификацию, затем выполняет DSL-обработчик.
func (s *Server) serviceDispatch(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/hs/")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		s.serviceIndex(w, r)
		return
	}
	if rest == "openapi.json" {
		s.serviceOpenAPI(w, r)
		return
	}
	root, remainder, _ := strings.Cut(rest, "/")
	svc := s.reg.GetHTTPServiceByRoot(root)
	if svc == nil {
		writeServiceError(w, http.StatusNotFound, "HTTP-сервис не найден: "+root)
		return
	}

	// CORS уровня сервиса. Заголовки Allow-Origin ставим на все ответы сервиса,
	// а preflight (OPTIONS) обрабатываем здесь же, не запуская обработчик.
	if svc.CORS != nil {
		s.setCORSHeaders(w, r, svc)
		if r.Method == http.MethodOptions {
			s.writeCORSPreflight(w, r, svc, remainder)
			return
		}
	}

	tmpl, pathParams, ok := svc.Match("/" + remainder)
	if !ok {
		writeServiceError(w, http.StatusNotFound, "ресурс не найден: /"+remainder)
		return
	}
	handlerName, ok := tmpl.Methods[strings.ToUpper(r.Method)]
	if !ok {
		w.Header().Set("Allow", strings.Join(sortedMethods(tmpl.Methods), ", "))
		writeServiceError(w, http.StatusMethodNotAllowed, "метод не поддержан: "+r.Method)
		return
	}

	ctx, ok := s.resolveServiceAuth(svc, w, r)
	if !ok {
		return // 401 уже отправлен
	}

	// Авторизация по ролям (план 52). Непустой roles: требует
	// аутентифицированного пользователя с одной из ролей (админ — всегда).
	if len(svc.Roles) > 0 {
		u := auth.UserFromContext(ctx)
		if u == nil || !u.HasAnyRole(svc.Roles) {
			writeServiceError(w, http.StatusForbidden, "доступ запрещён: требуется роль "+strings.Join(svc.Roles, "/"))
			return
		}
	}

	// Тело читаем целиком (ограничено maxFileSizeBytes) — обработчик получит его
	// как байты/строку без возни с потоком.
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxFileSizeBytes))
		r.Body.Close()
	}

	procDecl := s.reg.GetProcedure(svc.Name, handlerName)
	if procDecl == nil {
		writeServiceError(w, http.StatusInternalServerError,
			"обработчик "+handlerName+" не найден в src/"+strings.ToLower(svc.Name)+".service.os")
		return
	}

	reqObj := interpreter.NewServiceRequest(r.Method, svc.RootURL, r.URL.Path, pathParams, r.URL.Query(), r.Header, body)

	var msgs []string
	mc := runtime.NewMovementsCollector("service", uuid.Nil)
	dslVars := s.buildDSLVarsWithMessages(ctx, mc, &msgs)
	// Запрос доступен и как параметр обработчика, и как глобальная переменная —
	// чтобы работали оба стиля: Функция H(Запрос) и Функция H() с чтением Запрос.
	dslVars["Запрос"] = reqObj
	dslVars["Request"] = reqObj

	result, err := s.interp.Call(procDecl, reqObj, []any{reqObj}, dslVars)
	if err != nil {
		writeServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeServiceResult(w, result)
}

// resolveServiceAuth применяет аутентификацию конкретного сервиса. Возвращает
// контекст (с вложенным пользователем при успехе) и ok=false, если запрос уже
// отклонён (401 отправлен).
func (s *Server) resolveServiceAuth(svc *httpservice.Service, w http.ResponseWriter, r *http.Request) (ctxOut context.Context, ok bool) {
	switch strings.ToLower(strings.TrimSpace(svc.Auth)) {
	case "", "none":
		return r.Context(), true

	case "basic":
		login, pass, has := r.BasicAuth()
		if !has || s.authRepo == nil {
			return s.denyBasic(w)
		}
		u, err := s.authRepo.Authenticate(r.Context(), login, pass)
		if err != nil {
			return s.denyBasic(w)
		}
		return s.serviceUserCtx(r.Context(), u), true

	case "session":
		if s.authRepo == nil {
			writeServiceError(w, http.StatusUnauthorized, "требуется аутентификация")
			return nil, false
		}
		token := sessionToken(r)
		if token == "" {
			writeServiceError(w, http.StatusUnauthorized, "требуется аутентификация")
			return nil, false
		}
		u, err := s.authRepo.LookupSession(r.Context(), token)
		if err != nil {
			writeServiceError(w, http.StatusUnauthorized, "недействительная сессия")
			return nil, false
		}
		return s.serviceUserCtx(r.Context(), u), true

	default:
		// Неизвестный режим — это ошибка конфигурации; не открываем доступ молча.
		writeServiceError(w, http.StatusInternalServerError, "неизвестный режим аутентификации сервиса: "+svc.Auth)
		return nil, false
	}
}

func (s *Server) denyBasic(w http.ResponseWriter) (context.Context, bool) {
	w.Header().Set("WWW-Authenticate", `Basic realm="onebase"`)
	writeServiceError(w, http.StatusUnauthorized, "требуется аутентификация")
	return nil, false
}

// serviceUserCtx вкладывает пользователя в контекст так, чтобы ТекущийПользователь()
// и аудит записи видели вызывающего (как делает session-middleware веб-интерфейса).
func (s *Server) serviceUserCtx(ctx context.Context, u *auth.User) context.Context {
	if roles, err := s.authRepo.GetRolesForUser(ctx, u.ID); err == nil {
		u.Roles = roles
	}
	ctx = auth.ContextWithUser(ctx, u)
	ctx = storage.WithAuditUser(ctx, u.ID, u.Login)
	return ctx
}

// writeServiceResult сериализует результат обработчика в HTTP-ответ. Кроме
// явного HTTPСервисОтвет поддержаны «голые» возвраты (удобство сверх 1С):
// строка → text/plain, Структура/Соответствие/Массив/число → JSON, Неопределено → 204.
func (s *Server) writeServiceResult(w http.ResponseWriter, result any) {
	switch v := result.(type) {
	case *interpreter.DSLServiceResponse:
		for k, val := range v.HeadersMap() {
			w.Header().Set(k, val)
		}
		code := v.StatusCodeValue()
		if code == 0 {
			code = http.StatusOK
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.WriteHeader(code)
		_, _ = w.Write(v.BodyBytes())
	case nil:
		w.WriteHeader(http.StatusNoContent)
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(v))
	default:
		data, err := interpreter.MarshalDSLValue(v)
		if err != nil {
			writeServiceError(w, http.StatusInternalServerError, "сериализация ответа: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func writeServiceError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// sessionToken повторяет логику session-middleware: cookie onebase_session либо
// query-параметр _tk (для конфигуратора на другом порту).
func sessionToken(r *http.Request) string {
	if c, err := r.Cookie("onebase_session"); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("_tk")
}

// setCORSHeaders ставит Access-Control-Allow-Origin (+ credentials) согласно
// политике сервиса. С credentials нельзя отвечать «*» — эхо конкретного источника.
func (s *Server) setCORSHeaders(w http.ResponseWriter, r *http.Request, svc *httpservice.Service) {
	origin := r.Header.Get("Origin")
	allow, ok := matchOrigin(svc.CORS.Origins, origin)
	if !ok {
		return
	}
	if svc.CORS.Credentials {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", allow)
	}
	w.Header().Add("Vary", "Origin")
}

// writeCORSPreflight отвечает на OPTIONS-preflight: разрешённые методы (из
// шаблона ресурса либо объединение всех), заголовки и время кэша.
func (s *Server) writeCORSPreflight(w http.ResponseWriter, r *http.Request, svc *httpservice.Service, remainder string) {
	var methods []string
	if t, _, ok := svc.Match("/" + remainder); ok {
		methods = sortedMethods(t.Methods)
	} else {
		set := map[string]string{}
		for _, t := range svc.Templates {
			for m := range t.Methods {
				set[m] = ""
			}
		}
		methods = sortedMethods(set)
	}
	methods = append(methods, http.MethodOptions)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

	if len(svc.CORS.Headers) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(svc.CORS.Headers, ", "))
	} else if reqH := r.Header.Get("Access-Control-Request-Headers"); reqH != "" {
		w.Header().Set("Access-Control-Allow-Headers", reqH)
	}
	if svc.CORS.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(svc.CORS.MaxAge))
	}
	w.WriteHeader(http.StatusNoContent)
}

// matchOrigin возвращает значение для Allow-Origin и признак совпадения.
// «*» среди источников разрешает любой (возвращает "*"); иначе ищется точное
// совпадение с заголовком Origin.
func matchOrigin(origins []string, origin string) (string, bool) {
	for _, o := range origins {
		if o == "*" {
			return "*", true
		}
		if origin != "" && strings.EqualFold(o, origin) {
			return origin, true
		}
	}
	return "", false
}

func sortedMethods(methods map[string]string) []string {
	out := make([]string, 0, len(methods))
	for m := range methods {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
