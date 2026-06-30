package configcheck

import (
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/report"
)

func projWithReport(rep *report.Report) *project.Project {
	return &project.Project{Reports: []*report.Report{rep}}
}

func TestCheckReportOutputFormat_AcceptsKnown(t *testing.T) {
	for _, of := range []string{"", "html", "pdf", "excel", "PDF", "Excel"} {
		rep := &report.Report{Name: "R", OutputFormat: of}
		if iss := CheckReportOutputFormat(projWithReport(rep)); len(iss) != 0 {
			t.Errorf("output_format=%q должен приниматься, получили %d ошибок", of, len(iss))
		}
	}
}

func TestCheckReportOutputFormat_RejectsUnknown(t *testing.T) {
	rep := &report.Report{Name: "ЖурналPDF", OutputFormat: "pdfx"}
	iss := CheckReportOutputFormat(projWithReport(rep))
	if len(iss) != 1 {
		t.Fatalf("ожидалась 1 ошибка для неизвестного output_format, получили %d", len(iss))
	}
	if iss[0].Code != "report.bad-output-format" {
		t.Errorf("Code = %q, ожидался report.bad-output-format", iss[0].Code)
	}
	if iss[0].Object != "ЖурналPDF" {
		t.Errorf("Object = %q", iss[0].Object)
	}
}
