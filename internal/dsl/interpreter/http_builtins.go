package interpreter

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ─── dslHTTPConnection ────────────────────────────────────────────────────────

type dslHTTPConnection struct {
	host    string
	port    int
	https   bool
	timeout time.Duration
}

func (c *dslHTTPConnection) CallMethod(name string, args []any) any {
	switch name {
	case "Получить", "Get":
		req := asHTTPRequest(args, 0, "HTTPСоединение.Получить")
		return c.do(req, "GET")
	case "ОтправитьДля", "SendFor":
		req := asHTTPRequest(args, 0, "HTTPСоединение.ОтправитьДля")
		method := "POST"
		if len(args) >= 2 {
			method = strings.ToUpper(fmt.Sprintf("%v", args[1]))
		}
		return c.do(req, method)
	}
	panic(userError{Msg: "HTTPСоединение: неизвестный метод " + name})
}

func (c *dslHTTPConnection) do(req *dslHTTPRequest, method string) *dslHTTPResponse {
	scheme := "http"
	if c.https {
		scheme = "https"
	}
	port := c.port
	var url string
	if port == 0 || (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		url = fmt.Sprintf("%s://%s%s", scheme, c.host, req.resource)
	} else {
		url = fmt.Sprintf("%s://%s:%d%s", scheme, c.host, port, req.resource)
	}

	client := &http.Client{Timeout: c.timeout}
	var bodyReader io.Reader
	if req.body != "" {
		bodyReader = strings.NewReader(req.body)
	}
	httpReq, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		panic(userError{Msg: "HTTPСоединение: ошибка запроса: " + err.Error()})
	}
	for k, v := range req.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		panic(userError{Msg: "HTTPСоединение: " + err.Error()})
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return &dslHTTPResponse{
		statusCode: resp.StatusCode,
		headers:    resp.Header,
		body:       string(bodyBytes),
	}
}

// ─── dslHTTPRequest ───────────────────────────────────────────────────────────

type dslHTTPRequest struct {
	resource string
	headers  map[string]string
	body     string
}

func (r *dslHTTPRequest) CallMethod(name string, args []any) any {
	switch name {
	case "УстановитьЗаголовок", "SetHeader":
		if len(args) >= 2 {
			r.headers[fmt.Sprintf("%v", args[0])] = fmt.Sprintf("%v", args[1])
		}
		return nil
	case "УстановитьТелоИзСтроки", "SetBodyFromString":
		if len(args) > 0 {
			r.body = fmt.Sprintf("%v", args[0])
		}
		return nil
	}
	panic(userError{Msg: "HTTPЗапрос: неизвестный метод " + name})
}

// ─── dslHTTPResponse ──────────────────────────────────────────────────────────

type dslHTTPResponse struct {
	statusCode int
	headers    http.Header
	body       string
}

// Get implements This — allows Ответ.КодСостояния property access.
func (r *dslHTTPResponse) Get(field string) any {
	switch field {
	case "КодСостояния", "StatusCode":
		return float64(r.statusCode)
	}
	return nil
}

func (r *dslHTTPResponse) Set(field string, val any) {}

func (r *dslHTTPResponse) CallMethod(name string, args []any) any {
	switch name {
	case "ПолучитьТелоКакСтроку", "GetBodyAsString":
		return r.body
	case "ПолучитьЗаголовок", "GetHeader":
		if len(args) > 0 {
			return r.headers.Get(fmt.Sprintf("%v", args[0]))
		}
		return ""
	}
	panic(userError{Msg: "HTTPОтвет: неизвестный метод " + name})
}

// ─── NewHTTPFunctions ─────────────────────────────────────────────────────────

// NewHTTPFunctions returns factories and shorthands to inject into DSL extraVars.
func NewHTTPFunctions() map[string]any {
	m := map[string]any{
		"__factory_HTTPСоединение": newHTTPConnFactory(),
		"__factory_HTTPConnection": newHTTPConnFactory(),
		"__factory_HTTPЗапрос":    newHTTPReqFactory(),
		"__factory_HTTPRequest":   newHTTPReqFactory(),
	}

	httpGet := BuiltinFunc(func(args []any, file string, line int) (any, error) {
		url := strArg(args, 0)
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			panic(userError{Msg: "HTTPПолучить: " + err.Error()})
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return &dslHTTPResponse{statusCode: resp.StatusCode, headers: resp.Header, body: string(b)}, nil
	})

	httpPost := BuiltinFunc(func(args []any, file string, line int) (any, error) {
		url := strArg(args, 0)
		body := strArg(args, 1)
		contentType := "application/json"
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Post(url, contentType, strings.NewReader(body))
		if err != nil {
			panic(userError{Msg: "HTTPОтправить: " + err.Error()})
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return &dslHTTPResponse{statusCode: resp.StatusCode, headers: resp.Header, body: string(b)}, nil
	})

	m["HTTPПолучить"] = httpGet
	m["HTTPGet"] = httpGet
	m["HTTPОтправить"] = httpPost
	m["HTTPPost"] = httpPost
	return m
}

func newHTTPConnFactory() func([]any) any {
	return func(args []any) any {
		conn := &dslHTTPConnection{
			host:    strArg(args, 0),
			timeout: 30 * time.Second,
		}
		if len(args) >= 2 {
			conn.port = int(floatArg(args, 1))
		}
		if len(args) >= 3 {
			conn.https = truthy(args[2])
		} else if conn.port == 443 {
			conn.https = true
		}
		if len(args) >= 4 {
			if secs := floatArg(args, 3); secs > 0 {
				conn.timeout = time.Duration(secs) * time.Second
			}
		}
		return conn
	}
}

func newHTTPReqFactory() func([]any) any {
	return func(args []any) any {
		return &dslHTTPRequest{
			resource: strArg(args, 0),
			headers:  make(map[string]string),
		}
	}
}

func asHTTPRequest(args []any, i int, caller string) *dslHTTPRequest {
	if i < len(args) {
		if req, ok := args[i].(*dslHTTPRequest); ok {
			return req
		}
	}
	panic(userError{Msg: caller + ": ожидается HTTPЗапрос"})
}
