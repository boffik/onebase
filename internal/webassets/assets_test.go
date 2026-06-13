package webassets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMonacoHandlerServesCriticalFiles guards the offline self-hosting: every
// file the templates load from /vendor/monaco/ must be embedded and served.
func TestMonacoHandlerServesCriticalFiles(t *testing.T) {
	h := http.StripPrefix("/vendor/monaco/", MonacoHandler())
	files := []string{
		"vs/loader.js",
		"vs/editor/editor.main.js",
		"vs/editor/editor.main.css",
		"vs/base/worker/workerMain.js",
		"vs/base/browser/ui/codicons/codicon/codicon.ttf",
		"vs/basic-languages/yaml/yaml.js",
	}
	for _, f := range files {
		req := httptest.NewRequest(http.MethodGet, "/vendor/monaco/"+f, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", f, rec.Code)
			continue
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty body", f)
		}
	}

	// A path outside the embedded tree must 404, not leak the filesystem.
	req := httptest.NewRequest(http.MethodGet, "/vendor/monaco/vs/language/typescript/tsMode.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("absent file: status = %d, want 404", rec.Code)
	}

	// Cache header must be set for long-lived versioned assets.
	req = httptest.NewRequest(http.MethodGet, "/vendor/monaco/vs/loader.js", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age") {
		t.Errorf("Cache-Control = %q, want max-age", cc)
	}
}

// TestEChartsHandlerServesBundle guards that the shared ECharts bundle is
// embedded and served — both the base UI and the configurator load it from
// /vendor/echarts/echarts.min.js.
func TestEChartsHandlerServesBundle(t *testing.T) {
	h := http.StripPrefix("/vendor/echarts/", EChartsHandler())
	req := httptest.NewRequest(http.MethodGet, "/vendor/echarts/echarts.min.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("echarts.min.js: status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("echarts.min.js: empty body")
	}
}

// TestSlickGridHandlerServesCriticalFiles guards that all required SlickGrid
// assets are embedded and served. Managed forms load them for editable table
// parts from /vendor/slickgrid/.
func TestSlickGridHandlerServesCriticalFiles(t *testing.T) {
	h := http.StripPrefix("/vendor/slickgrid/", SlickGridHandler())
	files := []string{
		"slick.core.js",
		"slick.interactions.js",
		"slick.grid.js",
		"slick.dataview.js",
		"slick.editors.js",
		"slick.formatters.js",
		"slick.grid.css",
		"slick-default-theme.css",
	}
	for _, f := range files {
		req := httptest.NewRequest(http.MethodGet, "/vendor/slickgrid/"+f, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", f, rec.Code)
			continue
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty body", f)
		}
	}

	// A path outside the embedded tree must 404.
	req := httptest.NewRequest(http.MethodGet, "/vendor/slickgrid/nonexistent.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("absent file: status = %d, want 404", rec.Code)
	}

	// Cache header must be set for long-lived versioned assets.
	req = httptest.NewRequest(http.MethodGet, "/vendor/slickgrid/slick.grid.js", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age") {
		t.Errorf("Cache-Control = %q, want max-age", cc)
	}
}

// TestQuillHandlerServesBundle guards that the Quill WYSIWYG editor is embedded
// and served offline — richtext-fields load quill.js and quill.snow.css from
// /vendor/quill/.
func TestQuillHandlerServesBundle(t *testing.T) {
	h := http.StripPrefix("/vendor/quill/", QuillHandler())
	for _, f := range []string{"quill.js", "quill.snow.css"} {
		req := httptest.NewRequest(http.MethodGet, "/vendor/quill/"+f, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", f, rec.Code)
			continue
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty body", f)
		}
	}

	// A path outside the embedded tree must 404, not leak the filesystem.
	req := httptest.NewRequest(http.MethodGet, "/vendor/quill/nonexistent.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("absent file: status = %d, want 404", rec.Code)
	}

	// Cache header must be set for long-lived versioned assets.
	req = httptest.NewRequest(http.MethodGet, "/vendor/quill/quill.js", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age") {
		t.Errorf("Cache-Control = %q, want max-age", cc)
	}
}
