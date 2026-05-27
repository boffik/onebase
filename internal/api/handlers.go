package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/entityservice"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

type handler struct {
	reg       *runtime.Registry
	store     *storage.DB
	interp    *interpreter.Interpreter
	entitySvc *entityservice.Service // разделяем с ui — см. ui.Server.EntitySvc()
}

// createUpdateBody — JSON-контракт для POST/PUT. Шапка — flat-поля (как раньше),
// плюс опциональные ТЧ через __tableparts. Action позволяет явно провести
// документ в одном вызове (раньше нужно было два запроса: PUT + кнопка UI).
//
// Пример:
//
//	{
//	  "Номер": "100", "Контрагент": "...",
//	  "__tableparts": {"Товары": [{"Номенклатура":"...","Количество":3}]},
//	  "__action": "post"
//	}
type createUpdateBody struct {
	Fields        map[string]any                  `json:"-"`
	TablePartRows map[string][]map[string]any     `json:"__tableparts,omitempty"`
	Action        string                          `json:"__action,omitempty"`
}

// decodeBody парсит JSON в createUpdateBody, отделяя служебные ключи (__tableparts,
// __action) от собственных полей сущности. Делаем это вручную через generic map,
// чтобы пользователю не нужно было оборачивать поля в "fields": {...}.
func decodeBody(r *http.Request) (createUpdateBody, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return createUpdateBody{}, err
	}
	body := createUpdateBody{Fields: make(map[string]any, len(raw))}
	if v, ok := raw["__tableparts"]; ok {
		if err := json.Unmarshal(v, &body.TablePartRows); err != nil {
			return createUpdateBody{}, err
		}
		delete(raw, "__tableparts")
	}
	if v, ok := raw["__action"]; ok {
		_ = json.Unmarshal(v, &body.Action)
		delete(raw, "__action")
	}
	for k, v := range raw {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			return createUpdateBody{}, err
		}
		body.Fields[k] = val
	}
	return body, nil
}

func (h *handler) createObject(kind metadata.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := chi.URLParam(r, "entity")
		if len(entityName) > 0 {
			entityName = capitalize(entityName)
		}
		entity := h.reg.GetEntity(entityName)
		if entity == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "", 0)
			return
		}

		// Auto-number для документов: если поле Номер пустое, генерируем.
		// Раньше API создавал документ с пустым номером — UI это делал, API нет.
		if kind == metadata.KindDocument {
			for _, f := range entity.Fields {
				if f.Name == "Номер" && f.Type == metadata.FieldTypeString {
					if v, _ := body.Fields["Номер"].(string); strings.TrimSpace(v) == "" {
						body.Fields["Номер"] = generateAutoNumber(r.Context(), h.store, entity, body.Fields)
					}
					break
				}
			}
		}

		result, err := h.entitySvc.Save(r.Context(), entityservice.SaveRequest{
			Entity:        entity,
			ID:            uuid.New(),
			IsNew:         true,
			Fields:        body.Fields,
			TablePartRows: body.TablePartRows,
			Action:        body.Action,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "", 0)
			return
		}
		if result.DSLError != "" {
			writeError(w, http.StatusUnprocessableEntity, result.DSLError, "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       result.ID.String(),
			"messages": result.DSLMessages,
		})
	}
}

func (h *handler) getObject(kind metadata.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := capitalize(chi.URLParam(r, "entity"))
		entity := h.reg.GetEntity(entityName)
		if entity == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id", "", 0)
			return
		}
		result, err := h.store.GetByID(r.Context(), entityName, id, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}

func (h *handler) listObjects(kind metadata.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := capitalize(chi.URLParam(r, "entity"))
		entity := h.reg.GetEntity(entityName)
		if entity == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		params := storage.ListParams{Filters: parseRestFilters(r)}
		if s := r.URL.Query().Get("sort"); s != "" {
			params.Sort = s
		}
		if d := r.URL.Query().Get("dir"); d != "" {
			params.Dir = d
		}
		rows, err := h.store.List(r.Context(), entityName, entity, params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	}
}

func (h *handler) updateObject(kind metadata.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := capitalize(chi.URLParam(r, "entity"))
		entity := h.reg.GetEntity(entityName)
		if entity == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id", "", 0)
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "", 0)
			return
		}

		// If-Match — optimistic locking. Клиент шлёт версию которую он видел
		// при чтении; если в БД сейчас другая — 409 Conflict вместо тихого
		// перетирания чужих правок. Без заголовка проверка не делается
		// (обратная совместимость для клиентов которые её ещё не используют).
		var expectedVersion *int64
		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
			if v, perr := strconv.ParseInt(strings.Trim(ifMatch, `"`), 10, 64); perr == nil {
				expectedVersion = &v
			}
		}

		result, err := h.entitySvc.Save(r.Context(), entityservice.SaveRequest{
			Entity:          entity,
			ID:              id,
			IsNew:           false,
			Fields:          body.Fields,
			TablePartRows:   body.TablePartRows,
			Action:          body.Action,
			ExpectedVersion: expectedVersion,
		})
		if err != nil {
			if errors.Is(err, storage.ErrVersionConflict) {
				writeError(w, http.StatusConflict, "version conflict: object was modified by another client", "", 0)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error(), "", 0)
			return
		}
		if result.DSLError != "" {
			writeError(w, http.StatusUnprocessableEntity, result.DSLError, "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       id.String(),
			"messages": result.DSLMessages,
		})
	}
}

func (h *handler) deleteObject(kind metadata.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := capitalize(chi.URLParam(r, "entity"))
		if h.reg.GetEntity(entityName) == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id", "", 0)
			return
		}
		if err := h.store.WithTx(r.Context(), func(ctx context.Context) error {
			// Clear movements for documents before deleting
			if kind == metadata.KindDocument {
				for _, reg := range h.reg.Registers() {
					if err := h.store.WriteMovements(ctx, reg.Name, entityName, id, nil, reg, nil); err != nil {
						return err
					}
				}
			}
			return h.store.Delete(ctx, entityName, id)
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "", 0)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// postDocument — POST /documents/{entity}/{id}/post. Проводит существующий
// документ: запускает OnPost, пишет движения, ставит posted=true. Это
// функциональная дыра API: раньше провести документ через REST было
// невозможно (только через UI-кнопку).
//
// Тело может быть пустым (id берётся из URL) либо содержать обновлённые
// поля шапки/ТЧ (тогда сначала применятся изменения, потом проведение —
// аналогично UI «Записать и провести»).
func (h *handler) postDocument() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityName := capitalize(chi.URLParam(r, "entity"))
		entity := h.reg.GetEntity(entityName)
		if entity == nil {
			writeError(w, http.StatusNotFound, "unknown entity: "+entityName, "", 0)
			return
		}
		if !entity.Posting {
			writeError(w, http.StatusBadRequest, "entity is not postable: "+entityName, "", 0)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id", "", 0)
			return
		}

		// Тело опционально. Если есть — используем как обновление перед проведением;
		// если пусто — читаем текущее состояние из БД, чтобы OnPost увидел актуальные
		// данные документа.
		var fields map[string]any
		var tpRows map[string][]map[string]any
		if r.ContentLength > 0 {
			body, decErr := decodeBody(r)
			if decErr != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON: "+decErr.Error(), "", 0)
				return
			}
			fields = body.Fields
			tpRows = body.TablePartRows
		} else {
			existing, gerr := h.store.GetByID(r.Context(), entityName, id, entity)
			if gerr != nil {
				writeError(w, http.StatusNotFound, gerr.Error(), "", 0)
				return
			}
			fields = existing
		}

		result, err := h.entitySvc.Save(r.Context(), entityservice.SaveRequest{
			Entity:        entity,
			ID:            id,
			IsNew:         false,
			Fields:        fields,
			TablePartRows: tpRows,
			Action:        "post",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "", 0)
			return
		}
		if result.DSLError != "" {
			writeError(w, http.StatusUnprocessableEntity, result.DSLError, "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       id.String(),
			"posted":   true,
			"messages": result.DSLMessages,
		})
	}
}

// generateAutoNumber — упрощённый аналог ui.Server.generateNumber. Не пытается
// использовать Numerator-конфиг (это требует доступа к storage.ComputePeriodKey),
// просто берёт NextNum для базы. Для API этого достаточно — клиенты, которым
// нужна сложная нумерация, могут передавать Номер сами.
func generateAutoNumber(ctx context.Context, store *storage.DB, entity *metadata.Entity, fields map[string]any) string {
	if entity.Numerator != nil {
		num := entity.Numerator
		periodKey := storage.ComputePeriodKey(num, fields)
		if n, err := store.NextNumber(ctx, entity.Name, periodKey); err == nil {
			return storage.FormatNumber(num.Prefix, num.Length, n)
		}
	}
	if n, err := store.NextNum(ctx, entity.Name); err == nil {
		return formatLegacy(n)
	}
	return ""
}

func formatLegacy(n int64) string {
	s := strconv.FormatInt(n, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

func parseRestFilters(r *http.Request) map[string]storage.FilterValue {
	filters := make(map[string]storage.FilterValue)
	for k, vals := range r.URL.Query() {
		if strings.HasPrefix(k, "f.") && len(vals) > 0 {
			filters[strings.TrimPrefix(k, "f.")] = storage.FilterValue{Value: vals[0]}
		}
	}
	return filters
}

type errorResponse struct {
	Error string `json:"error"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

func writeError(w http.ResponseWriter, code int, msg, file string, line int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg, File: file, Line: line})
}

func capitalize(s string) string {
	if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
