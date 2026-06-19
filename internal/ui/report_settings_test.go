package ui

import (
	"testing"

	reportpkg "github.com/ivantit66/onebase/internal/report"
)

func TestEffectiveComposition(t *testing.T) {
	main := &reportpkg.Composition{Groupings: []string{"Основной"}}
	variantComp := &reportpkg.Composition{Groupings: []string{"Вариант"}}
	override := &reportpkg.Composition{Groupings: []string{"Override"}}
	rep := &reportpkg.Report{
		Composition: main,
		Variants:    []reportpkg.ReportVariant{{Name: "V", Composition: variantComp}},
	}

	// 1) override (settings.Composition) перекрывает и вариант, и основной.
	if got := effectiveComposition(rep, &reportpkg.UserReportSettings{Variant: "V", Composition: override}); got != override {
		t.Fatalf("override: %+v", got)
	}
	// 2) settings без Composition → активный вариант по имени.
	if got := effectiveComposition(rep, &reportpkg.UserReportSettings{Variant: "V"}); got != variantComp {
		t.Fatalf("variant: %+v", got)
	}
	// 3) settings == nil → основной composition.
	if got := effectiveComposition(rep, nil); got != main {
		t.Fatalf("main: %+v", got)
	}
}
