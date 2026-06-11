package langref

import (
	"sort"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
)

// notDocumented — имена из реестра, которые НЕ документируются как функции
// (специальные переменные контекста — не вызываются как функции).
var notDocumented = map[string]bool{
	"this":       true,
	"этотобъект": true,
}

func descriptorNameSet() map[string]bool {
	set := map[string]bool{}
	for _, d := range All() {
		set[strings.ToLower(d.Name)] = true
		for _, a := range d.Aliases {
			set[strings.ToLower(a)] = true
		}
	}
	return set
}

// TestCompleteness_AllBuiltinsDescribed — ЖЁСТКИЙ ГЕЙТ: каждое имя из реестра
// функций имеет дескриптор (кроме спец-переменных из notDocumented).
func TestCompleteness_AllBuiltinsDescribed(t *testing.T) {
	have := descriptorNameSet()
	var missing []string
	for name := range interpreter.KnownBuiltinNames() {
		ln := strings.ToLower(name)
		if notDocumented[ln] || have[ln] {
			continue
		}
		missing = append(missing, ln)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("нет описания (%d): %s", len(missing), strings.Join(missing, ", "))
	}
}

// TestCompleteness_NoOrphanFunctions — ЖЁСТКИЙ ГЕЙТ: дескриптор-функция не
// ссылается на несуществующее имя реестра (висячая ссылка = баг).
func TestCompleteness_NoOrphanFunctions(t *testing.T) {
	known := interpreter.KnownBuiltinNames()
	in := func(s string) bool { _, ok := known[strings.ToLower(s)]; return ok }
	var orphan []string
	for _, d := range All() {
		if d.Kind != KindFunc {
			continue
		}
		if !in(d.Name) {
			orphan = append(orphan, strings.ToLower(d.Name))
		}
		for _, a := range d.Aliases {
			if !in(a) {
				orphan = append(orphan, strings.ToLower(a))
			}
		}
	}
	sort.Strings(orphan)
	if len(orphan) > 0 {
		t.Fatalf("лишний дескриптор-функция (нет в реестре) (%d): %s", len(orphan), strings.Join(orphan, ", "))
	}
}

// TestDescriptors_Structural — дешёвая структурная валидация по ВСЕМ дескрипторам.
func TestDescriptors_Structural(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range All() {
		if d.Name == "" || d.Display == "" || d.Doc == "" {
			t.Errorf("пустое обязательное поле: %+v", d)
		}
		if d.Kind == KindMethod && d.Object == "" {
			t.Errorf("метод без Object: %s", d.Name)
		}
		key := string(d.Kind) + "|" + strings.ToLower(d.Object) + "|" + strings.ToLower(d.Name)
		if seen[key] {
			t.Errorf("дубль дескриптора: %s", key)
		}
		seen[key] = true
	}
}

// TestCoverage_Report — МЯГКИЙ отчёт (не блокирует) по тому, что автоматически
// не сверить: методы объектов и язык запросов.
func TestCoverage_Report(t *testing.T) {
	var fn, method, kw, q int
	objs := map[string]bool{}
	for _, d := range All() {
		switch d.Kind {
		case KindFunc:
			fn++
		case KindMethod:
			method++
			objs[d.Object] = true
		case KindKeyword:
			kw++
		case KindQuery:
			q++
		}
	}
	t.Logf("охват langref: функций=%d, методов=%d по %d объектам, конструкций=%d, слов запросов=%d",
		fn, method, len(objs), kw, q)
}
