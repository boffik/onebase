// Package deviceagent реализует локальный агент подключаемого оборудования:
// HTTP-сервер на localhost машины кассира, принимающий команды печати/ящика
// и исполняющий их через пакет equipment.
//
// Это «расширение работы с оборудованием» в стиле веб-клиента 1С: сервер или
// браузер кассы шлёт агенту JSON-команду, а агент уже говорит с железом.
// Драйверы переиспользуются из internal/equipment без изменений — это и есть
// клиент-серверный сценарий B, для которого локальный режим A (сервер=касса) —
// частный случай (агент на 127.0.0.1).
package deviceagent

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/equipment"
)

// Agent — HTTP-обработчик команд оборудования. Команды защищены общим токеном:
// без него любая открытая в браузере вкладка могла бы печатать чеки.
type Agent struct {
	token string
}

// New создаёт агент с общим токеном. Пустой токен отключает проверку
// (допустимо лишь для локальной отладки).
func New(token string) *Agent { return &Agent{token: token} }

// Handler возвращает маршрутизатор агента: / (страница кассы) и /health открыты,
// команды /print, /drawer, /display, /weight и /pay требуют заголовок X-Agent-Token.
func (a *Agent) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(cors)
	r.Get("/", a.page)
	r.Get("/health", a.health)
	r.Get("/events", a.events) // SSE; токен через query (EventSource не шлёт заголовки)
	r.Group(func(r chi.Router) {
		r.Use(a.auth)
		r.Post("/print", a.print)
		r.Post("/drawer", a.drawer)
		r.Post("/display", a.display)
		r.Post("/weight", a.weight)
		r.Post("/pay", a.pay)
		r.Post("/fiscal", a.fiscal)
	})
	return r
}

// ─── DTO ──────────────────────────────────────────────────────────────────────

type deviceRef struct {
	Driver string            `json:"driver"`
	Params map[string]string `json:"params"`
}

type printRequest struct {
	deviceRef
	Receipt receiptDTO `json:"receipt"`
}

type displayRequest struct {
	deviceRef
	Lines []string `json:"lines"`
}

type payRequest struct {
	deviceRef
	Amount float64 `json:"amount"`
}

type fiscalRequest struct {
	deviceRef
	Receipt fiscalReceiptDTO `json:"receipt"`
}

type fiscalReceiptDTO struct {
	Type     string             `json:"type"`
	Taxation string             `json:"taxation"`
	Email    string             `json:"email"`
	Phone    string             `json:"phone"`
	Items    []fiscalItemDTO    `json:"items"`
	Payments []fiscalPaymentDTO `json:"payments"`
}

type fiscalItemDTO struct {
	Name        string  `json:"name"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Sum         float64 `json:"sum"`
	VAT         string  `json:"vat"`
	ItemType    string  `json:"itemType"`
	PaymentType string  `json:"paymentType"`
}

type fiscalPaymentDTO struct {
	Type string  `json:"type"`
	Sum  float64 `json:"sum"`
}

func (r fiscalReceiptDTO) toFiscalReceipt() equipment.FiscalReceipt {
	rec := equipment.FiscalReceipt{
		Type:     r.Type,
		Taxation: r.Taxation,
		Email:    r.Email,
		Phone:    r.Phone,
	}
	for _, it := range r.Items {
		sum := it.Sum
		if sum == 0 {
			sum = it.Qty * it.Price
		}
		rec.Items = append(rec.Items, equipment.FiscalItem{
			Name: it.Name, Qty: it.Qty, Price: it.Price, Sum: sum,
			VAT: it.VAT, ItemType: it.ItemType, PaymentType: it.PaymentType,
		})
	}
	for _, p := range r.Payments {
		rec.Payments = append(rec.Payments, equipment.FiscalPayment{Type: p.Type, Sum: p.Sum})
	}
	return rec
}

type receiptDTO struct {
	Header  []string  `json:"header"`
	Items   []itemDTO `json:"items"`
	Total   float64   `json:"total"`
	Payment string    `json:"payment"`
	Footer  []string  `json:"footer"`
}

type itemDTO struct {
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Price float64 `json:"price"`
	Sum   float64 `json:"sum"`
}

func (r receiptDTO) toReceipt() equipment.Receipt {
	rec := equipment.Receipt{
		Header:  r.Header,
		Footer:  r.Footer,
		Total:   r.Total,
		Payment: r.Payment,
	}
	for _, it := range r.Items {
		sum := it.Sum
		if sum == 0 {
			sum = it.Qty * it.Price
		}
		rec.Items = append(rec.Items, equipment.ReceiptItem{
			Name: it.Name, Qty: it.Qty, Price: it.Price, Sum: sum,
		})
	}
	return rec
}

// ─── обработчики ────────────────────────────────────────────────────────────

func (a *Agent) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"drivers": equipment.Drivers(),
	})
}

func (a *Agent) print(w http.ResponseWriter, r *http.Request) {
	var req printRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	printer, ok := dev.(equipment.ReceiptPrinter)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не печатает чеки")
		return
	}
	if err := printer.PrintReceipt(req.Receipt.toReceipt()); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *Agent) drawer(w http.ResponseWriter, r *http.Request) {
	var req deviceRef
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	printer, ok := dev.(equipment.ReceiptPrinter)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не поддерживает денежный ящик")
		return
	}
	if err := printer.OpenDrawer(); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *Agent) display(w http.ResponseWriter, r *http.Request) {
	var req displayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	disp, ok := dev.(equipment.CustomerDisplay)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не является дисплеем")
		return
	}
	if err := disp.ShowLines(req.Lines); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *Agent) weight(w http.ResponseWriter, r *http.Request) {
	var req deviceRef
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	scale, ok := dev.(equipment.Scale)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не является весами")
		return
	}
	val, err := scale.Weight()
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "weight": val})
}

// requireToken запрещает денежные/фискальные операции, когда агент запущен без
// токена: при пустом токене auth-middleware пропускает всё, и без этой проверки
// любая вкладка в браузере кассы могла бы пробить оплату/фискальный чек.
func (a *Agent) requireToken(w http.ResponseWriter) bool {
	if a.token == "" {
		writeErr(w, http.StatusForbidden, "операция запрещена: агент запущен без токена (--token)")
		return false
	}
	return true
}

func (a *Agent) pay(w http.ResponseWriter, r *http.Request) {
	if !a.requireToken(w) {
		return
	}
	var req payRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	terminal, ok := dev.(equipment.PaymentTerminal)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не является терминалом эквайринга")
		return
	}
	res, err := terminal.Pay(req.Amount)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "approved": res.Approved, "rrn": res.RRN, "card": res.Card, "message": res.Message,
	})
}

func (a *Agent) fiscal(w http.ResponseWriter, r *http.Request) {
	if !a.requireToken(w) {
		return
	}
	var req fiscalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	dev, err := equipment.Open(req.Driver, req.Params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	kkt, ok := dev.(equipment.FiscalRegistrar)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не является фискальным регистратором")
		return
	}
	res, err := kkt.RegisterReceipt(req.Receipt.toFiscalReceipt())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "fn": res.FN, "fd": res.FD, "fp": res.FP, "qr": res.QR,
	})
}

// events — SSE-поток событий устройства (сканер ШК). EventSource в браузере не
// умеет слать заголовки, поэтому токен и параметры устройства идут через query.
func (a *Agent) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if a.token != "" && q.Get("token") != a.token {
		writeErr(w, http.StatusUnauthorized, "неверный или отсутствующий token")
		return
	}
	params := map[string]string{}
	for k := range q {
		switch k {
		case "driver", "token":
		default:
			params[k] = q.Get(k)
		}
	}
	dev, err := equipment.Open(q.Get("driver"), params)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer dev.Disconnect()
	src, ok := dev.(equipment.EventSource)
	if !ok {
		writeErr(w, http.StatusBadRequest, "устройство «"+dev.Kind()+"» не выдаёт события")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "стриминг не поддерживается сервером")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	src.Stream(r.Context(), func(code string) {
		fmt.Fprintf(w, "data: %s\n\n", code)
		flusher.Flush()
	})
}

// auth проверяет общий токен (заголовок X-Agent-Token). Пустой токен агента
// означает «проверка отключена».
func (a *Agent) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.token != "" && r.Header.Get("X-Agent-Token") != a.token {
			writeErr(w, http.StatusUnauthorized, "неверный или отсутствующий X-Agent-Token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}
