package ui

// HTTP-обработчики печатных форм (HTML/PDF/DSL-печать).
// Выделено из handlers.go (план 55, этап 1) — перенос as-is.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/printform"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/sheet"
)

// printDocument — единый HTML-маршрут печати (/print/{form}) для всех видов
// форм (план 64, этап 3). Находит PrintFormRef и диспетчеризует:
//   - Declarative → BuildSheet → sheet.HTML;
//   - DSL → существующий buildDSLPF-путь → sheet.HTML;
//   - Legacy → прежний RenderWithPDFURL (этап 4 заменит конверсией).
func (s *Server) printDocument(w http.ResponseWriter, r *http.Request) {
	entity := s.getEntity(w, r)
	if entity == nil {
		return
	}
	if !s.requirePerm(w, r, string(entity.Kind), entity.Name, "read") {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	formName := printFormParam(r, "form")

	ref, ok := s.reg.GetPrintFormRef(entity.Name, formName)
	if !ok {
		// Fallback: модульная процедура «Печать» (form == "_module") или иной
		// DSL-вход — buildDSLPF сам найдёт процедуру модуля.
		sd, ok := s.buildDSLPF(w, r, entity, id, formName)
		if !ok {
			return
		}
		backPath := fmt.Sprintf("/ui/%s/%s/%s", strings.ToLower(string(entity.Kind)), strings.ToLower(entity.Name), id.String())
		sd.SetBackURL(backPath)
		html := sd.Doc.HTML(sheet.HTMLOptions{BackURL: sd.Doc.BackURL, PDFURL: r.URL.Path + "/pdf"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	switch ref.Kind {
	case runtime.PrintFormDeclarative:
		doc, err := s.buildDeclarativeSheet(r, entity, id, ref.Decl)
		if err != nil {
			http.Error(w, s.errText(r, err), 500)
			return
		}
		backPath := fmt.Sprintf("/ui/%s/%s/%s", strings.ToLower(string(entity.Kind)), strings.ToLower(entity.Name), id.String())
		html := doc.HTML(sheet.HTMLOptions{BackURL: backPath, PDFURL: r.URL.Path + "/pdf"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))

	case runtime.PrintFormDSL:
		sd, ok := s.buildDSLPF(w, r, entity, id, ref.Name)
		if !ok {
			return
		}
		backPath := fmt.Sprintf("/ui/%s/%s/%s", strings.ToLower(string(entity.Kind)), strings.ToLower(entity.Name), id.String())
		sd.SetBackURL(backPath)
		html := sd.Doc.HTML(sheet.HTMLOptions{BackURL: sd.Doc.BackURL, PDFURL: r.URL.Path + "/pdf"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))

	default: // Legacy
		ctx, err := s.loadPrintContext(r, entity, id)
		if err != nil {
			http.Error(w, s.errText(r, err), 404)
			return
		}
		html, err := printform.RenderWithPDFURL(ref.Legacy, ctx, r.URL.Path+"/pdf")
		if err != nil {
			http.Error(w, s.errText(r, err), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}
}

// printFormParam извлекает и url-декодирует параметр маршрута печати.
func printFormParam(r *http.Request, key string) string {
	v := chi.URLParam(r, key)
	if dec, err := url.PathUnescape(v); err == nil {
		return dec
	}
	return v
}

// loadPrintContext загружает запись, табличные части, ссылки и константы в
// RenderContext (для legacy- и декларативного путей). Ссылки НЕ оборачиваются в
// MapThis (это нужно только DSL-пути).
func (s *Server) loadPrintContext(r *http.Request, entity *metadata.Entity, id uuid.UUID) (*printform.RenderContext, error) {
	row, err := s.store.GetByID(r.Context(), entity.Name, id, entity)
	if err != nil {
		return nil, err
	}
	tpRows := make(map[string][]map[string]any)
	for _, tp := range entity.TableParts {
		rows, _ := s.store.GetTablePartRows(r.Context(), entity.Name, tp.Name, id, tp)
		tpRows[tp.Name] = rows
	}
	refs := s.buildPrintRefs(r.Context(), row, entity, tpRows)
	constants, _ := s.store.ListConstants(r.Context())
	return &printform.RenderContext{
		Document:   row,
		TableParts: tpRows,
		Constants:  constants,
		Refs:       refs,
	}, nil
}

// buildDeclarativeSheet строит sheet.Document по декларативной форме (макет +
// binding) и данным записи.
func (s *Server) buildDeclarativeSheet(r *http.Request, entity *metadata.Entity, id uuid.UUID, lf *printform.LayoutForm) (*sheet.Document, error) {
	ctx, err := s.loadPrintContext(r, entity, id)
	if err != nil {
		return nil, err
	}
	return printform.BuildSheet(lf.Layout, ctx)
}

// buildPrintRefs returns a map of UUID → {fields...} for all reference fields in the entity and table parts.
func (s *Server) buildPrintRefs(ctx context.Context, row map[string]any, entity *metadata.Entity, tpRows map[string][]map[string]any) map[string]map[string]any {
	refs := make(map[string]map[string]any)
	resolveRef := func(refEntityName, idStr string) {
		if idStr == "" {
			return
		}
		if _, dup := refs[idStr]; dup {
			return
		}
		refEntity := s.reg.GetEntity(refEntityName)
		if refEntity == nil {
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			return
		}
		refRow, err := s.store.GetByID(ctx, refEntity.Name, id, refEntity)
		if err != nil {
			return
		}
		refs[idStr] = refRow
	}
	for _, f := range entity.Fields {
		if f.RefEntity == "" {
			continue
		}
		idStr, _ := row[f.Name].(string)
		resolveRef(f.RefEntity, idStr)
	}
	for _, tp := range entity.TableParts {
		rows := tpRows[tp.Name]
		for _, f := range tp.Fields {
			if f.RefEntity == "" {
				continue
			}
			for _, r := range rows {
				idStr, _ := r[f.Name].(string)
				resolveRef(f.RefEntity, idStr)
			}
		}
	}
	return refs
}

// resolveDSLRefs replaces reference UUID strings in row with MapThis objects
// so that DSL dot-notation like Документ.Организация.Наименование works.
func (s *Server) resolveDSLRefs(row map[string]any, fields []metadata.Field, refs map[string]map[string]any) {
	for _, f := range fields {
		if f.RefEntity == "" {
			continue
		}
		v, ok := row[f.Name]
		if !ok {
			continue
		}
		idStr, ok := v.(string)
		if !ok || idStr == "" {
			continue
		}
		refData, ok := refs[idStr]
		if !ok {
			continue
		}
		// Wrap ref data as MapThis for DSL dot-notation access
		row[f.Name] = &interpreter.MapThis{M: refData}
	}
}

// printDocumentPDF — единый PDF-маршрут печати (/print/{form}/pdf) для всех
// видов форм (план 64, этап 3). Dispatch по Kind:
//   - Declarative → BuildSheet → sheet.PDF;
//   - DSL → buildDSLPF → sheet.PDF;
//   - Legacy → прежний printform.RenderPDF (этап 4 заменит конверсией).
func (s *Server) printDocumentPDF(w http.ResponseWriter, r *http.Request) {
	entity := s.getEntity(w, r)
	if entity == nil {
		return
	}
	if !s.requirePerm(w, r, string(entity.Kind), entity.Name, "read") {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	formName := printFormParam(r, "form")

	ref, ok := s.reg.GetPrintFormRef(entity.Name, formName)
	if !ok {
		// Fallback: модульная процедура «Печать» (form == "_module") → PDF.
		sd, ok := s.buildDSLPF(w, r, entity, id, formName)
		if !ok {
			return
		}
		pdfBytes, err := sd.Doc.PDF(sheet.PDFOptions{Title: formName})
		if err != nil {
			http.Error(w, "PDF error: "+s.errText(r, err), 500)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", contentDisposition(formName+".pdf"))
		w.Write(pdfBytes)
		return
	}

	var pdfBytes []byte
	switch ref.Kind {
	case runtime.PrintFormDeclarative:
		doc, err := s.buildDeclarativeSheet(r, entity, id, ref.Decl)
		if err != nil {
			http.Error(w, s.errText(r, err), 500)
			return
		}
		pdfBytes, err = doc.PDF(sheet.PDFOptions{Title: ref.Name})
		if err != nil {
			http.Error(w, "PDF error: "+s.errText(r, err), 500)
			return
		}

	case runtime.PrintFormDSL:
		sd, ok := s.buildDSLPF(w, r, entity, id, ref.Name)
		if !ok {
			return
		}
		pdfBytes, err = sd.Doc.PDF(sheet.PDFOptions{Title: ref.Name})
		if err != nil {
			http.Error(w, "PDF error: "+s.errText(r, err), 500)
			return
		}

	default: // Legacy
		ctx, err := s.loadPrintContext(r, entity, id)
		if err != nil {
			http.Error(w, s.errText(r, err), 404)
			return
		}
		pdfBytes, err = printform.RenderPDF(ref.Legacy, ctx)
		if err != nil {
			http.Error(w, "PDF error: "+s.errText(r, err), 500)
			return
		}
	}

	origName := ref.Name + ".pdf"
	if row, err := s.store.GetByID(r.Context(), entity.Name, id, entity); err == nil {
		if num, ok := row["Номер"].(string); ok && num != "" {
			origName = ref.Name + "_" + num + ".pdf"
		}
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", contentDisposition(origName))
	w.Write(pdfBytes)
}

// buildDSLPF выполняет общую часть DSL-печати: находит форму/процедуру,
// загружает запись, резолвит ссылки, исполняет DSL-функцию и возвращает
// готовый ТабличныйДокумент. При ошибке сам пишет HTTP-ответ и возвращает
// ok=false. Общий код для HTML- и PDF-маршрутов DSL-форм (план 64, этап 2).
func (s *Server) buildDSLPF(w http.ResponseWriter, r *http.Request, entity *metadata.Entity, id uuid.UUID, pfName string) (*interpreter.SpreadsheetDocument, bool) {
	// 1. Find DSL print form in registry
	dslForm := s.reg.GetDSLPrintForm(entity.Name, pfName)

	// 2. Also check entity module for a "Печать" procedure
	var procDecl *ast.ProcedureDecl
	var source string

	if dslForm != nil {
		source = dslForm.Source
	} else {
		// Try module procedure: entity module → "Печать"
		procDecl = s.reg.GetProcedure(entity.Name, "Печать")
		if procDecl == nil {
			procDecl = s.reg.GetProcedure(entity.Name, "Print")
		}
		if procDecl == nil {
			http.Error(w, "DSL print form not found: "+pfName, 404)
			return nil, false
		}
	}

	// 3. Parse .os source if needed (for standalone print form files)
	if procDecl == nil && source != "" {
		l := lexer.New(source, "printforms/"+pfName+".os")
		p := parser.New(l)
		prog, parseErr := p.ParseProgram()
		if parseErr != nil {
			http.Error(w, "parse error: "+s.errText(r, parseErr), 500)
			return nil, false
		}
		for _, proc := range prog.Procedures {
			lower := strings.ToLower(proc.Name.Literal)
			if lower == "сформировать" || lower == "сформироватьпечатнуюформу" || lower == "form" {
				procDecl = proc
				break
			}
		}
		if procDecl == nil {
			http.Error(w, fmt.Sprintf(s.tr(s.resolveLang(r), "Функция Сформировать() не найдена в %s.os"), pfName), 404)
			return nil, false
		}
	}

	// 4. Load record data
	row, err := s.store.GetByID(r.Context(), entity.Name, id, entity)
	if err != nil {
		http.Error(w, s.errText(r, err), 404)
		return nil, false
	}

	tpRows := make(map[string][]map[string]any)
	for _, tp := range entity.TableParts {
		rows, _ := s.store.GetTablePartRows(r.Context(), entity.Name, tp.Name, id, tp)
		tpRows[tp.Name] = rows
	}

	// 5. Resolve references so DSL can access Документ.Организация.Наименование etc.
	refs := s.buildPrintRefs(r.Context(), row, entity, tpRows)
	s.resolveDSLRefs(row, entity.Fields, refs)
	for _, tp := range entity.TableParts {
		for _, tpRow := range tpRows[tp.Name] {
			s.resolveDSLRefs(tpRow, tp.Fields, refs)
		}
	}

	// 6. Build DSL environment
	mc := runtime.NewMovementsCollector(entity.Name, id)
	dslVars := s.buildDSLVars(r.Context(), mc)

	// Embed table parts into document row for Документ.Товары access
	for tpName, rows := range tpRows {
		row[tpName] = rows
	}

	// Convert row + table parts into a DSL object
	docData := &interpreter.MapThis{M: row}
	dslVars["Документ"] = docData
	dslVars["Document"] = docData

	// Pass макет layout as DSL variable (if available)
	if dslForm != nil && dslForm.Layout != nil {
		dslVars["Макет"] = interpreter.NewMaket(dslForm.Layout)
	}

	// 7. Execute the DSL function
	var result any
	if err := s.interp.RunWithResult(procDecl, docData, &result, dslVars); err != nil {
		http.Error(w, "DSL error: "+s.errText(r, err), 500)
		return nil, false
	}

	// 8. Render result
	sd, ok := result.(*interpreter.SpreadsheetDocument)
	if !ok {
		http.Error(w, s.tr(s.resolveLang(r), "Процедура должна возвращать ТабличныйДокумент"), 500)
		return nil, false
	}
	return sd, true
}

// redirectDSLPrint — обратная совместимость: старый /print-dsl/{pfName}[/pdf]
// отвечает 301 на единый /print/{pfName}[/pdf] (план 64, этап 3). Маршруты
// печати объединены; буква пути сохраняется (pfName и хвост /pdf).
func (s *Server) redirectDSLPrint(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	ent := chi.URLParam(r, "entity")
	id := chi.URLParam(r, "id")
	pfName := chi.URLParam(r, "pfName") // уже %-encoded в исходном пути
	target := fmt.Sprintf("/ui/%s/%s/%s/print/%s", kind, ent, id, pfName)
	if strings.HasSuffix(r.URL.Path, "/pdf") {
		target += "/pdf"
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}
