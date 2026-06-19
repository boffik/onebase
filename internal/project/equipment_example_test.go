package project_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
)

// TestLoad_TradeEquipment проверяет, что метаданные торгового оборудования и
// РМК-обработка из examples/trade загружаются, а DSL-модуль печати чека
// парсится и привязывается к обработке ПечатьЧека. БД не требуется.
func TestLoad_TradeEquipment(t *testing.T) {
	p, err := project.Load("../../examples/trade")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	contains := func(name string, names []string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}

	var entities, enums, procs []string
	for _, e := range p.Entities {
		entities = append(entities, e.Name)
	}
	for _, e := range p.Enums {
		enums = append(enums, e.Name)
	}
	for _, pr := range p.Processors {
		procs = append(procs, pr.Name)
	}

	if !contains("ПодключаемоеОборудование", entities) {
		t.Errorf("справочник ПодключаемоеОборудование не загружен; есть: %v", entities)
	}
	if !contains("ТипОборудования", enums) {
		t.Errorf("перечисление ТипОборудования не загружено; есть: %v", enums)
	}
	if !contains("ПечатьЧека", procs) {
		t.Errorf("обработка ПечатьЧека не загружена; есть: %v", procs)
	}

	prog, ok := p.Programs["ПечатьЧека"]
	if !ok || prog == nil {
		keys := make([]string, 0, len(p.Programs))
		for k := range p.Programs {
			keys = append(keys, k)
		}
		t.Fatalf("DSL-модуль ПечатьЧека не привязан; ключи Programs: %v", keys)
	}

	var execFound bool
	for _, proc := range prog.Procedures {
		if strings.EqualFold(proc.Name.Literal, "Выполнить") {
			execFound = true
		}
	}
	if !execFound {
		t.Error("процедура Выполнить() не найдена в модуле ПечатьЧека")
	}
}

// TestLoad_ScriptedScaleConfig проверяет, что протокол устройства переехал в
// конфигурацию: у справочника появились поля декларативного драйвера, а
// РМК-обработка взвешивания загрузилась и привязалась. БД не требуется.
func TestLoad_ScriptedScaleConfig(t *testing.T) {
	p, err := project.Load("../../examples/trade")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	var fields []string
	for _, e := range p.Entities {
		if e.Name == "ПодключаемоеОборудование" {
			for _, f := range e.Fields {
				fields = append(fields, f.Name)
			}
		}
	}
	for _, want := range []string{"КомандаЗапроса", "Шаблон", "Множитель"} {
		found := false
		for _, f := range fields {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("в справочнике нет поля %q; есть: %v", want, fields)
		}
	}

	hasProc := false
	for _, pr := range p.Processors {
		if pr.Name == "ВзвеситьТовар" {
			hasProc = true
		}
	}
	if !hasProc {
		t.Error("обработка ВзвеситьТовар не загружена")
	}

	prog, ok := p.Programs["ВзвеситьТовар"]
	if !ok || prog == nil {
		t.Fatal("DSL-модуль ВзвеситьТовар не привязан")
	}
	execFound := false
	for _, proc := range prog.Procedures {
		if strings.EqualFold(proc.Name.Literal, "Выполнить") {
			execFound = true
		}
	}
	if !execFound {
		t.Error("процедура Выполнить() не найдена в ВзвеситьТовар")
	}
}

// TestLoad_FiscalConfig проверяет метаданные фискального чека (54-ФЗ): появились
// перечисления ставок НДС/СНО/признака предмета, у номенклатуры — поля тегов
// ФФД, константа СНО, а РМК-обработка ПробитьЧек загрузилась и привязалась.
func TestLoad_FiscalConfig(t *testing.T) {
	p, err := project.Load("../../examples/trade")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer p.Close()

	has := func(name string, names []string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}

	var enums []string
	for _, e := range p.Enums {
		enums = append(enums, e.Name)
	}
	for _, want := range []string{"СтавкаНДС", "СистемаНалогообложения", "ПризнакПредмета"} {
		if !has(want, enums) {
			t.Errorf("перечисление %q не загружено; есть: %v", want, enums)
		}
	}

	var nomFields []string
	for _, e := range p.Entities {
		if e.Name == "Номенклатура" {
			for _, f := range e.Fields {
				nomFields = append(nomFields, f.Name)
			}
		}
	}
	for _, want := range []string{"СтавкаНДС", "ПризнакПредмета"} {
		if !has(want, nomFields) {
			t.Errorf("в номенклатуре нет поля %q; есть: %v", want, nomFields)
		}
	}

	var procs []string
	for _, pr := range p.Processors {
		procs = append(procs, pr.Name)
	}
	if !has("ПробитьЧек", procs) {
		t.Errorf("обработка ПробитьЧек не загружена; есть: %v", procs)
	}

	prog, ok := p.Programs["ПробитьЧек"]
	if !ok || prog == nil {
		t.Fatal("DSL-модуль ПробитьЧек не привязан")
	}
	execFound := false
	for _, proc := range prog.Procedures {
		if strings.EqualFold(proc.Name.Literal, "Выполнить") {
			execFound = true
		}
	}
	if !execFound {
		t.Error("процедура Выполнить() не найдена в ПробитьЧек")
	}
}
