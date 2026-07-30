package ui

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/processor"
	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// Раннер тестов уровня конфигурации (план 108). Находит тест-обработки
// (kind: test), гоняет каждую процедуру Выполнить() офлайн (как procrun),
// инжектируя объект «Утверждать», и собирает результаты проверок и ошибок.

// TestCaseResult — итог одного тест-процессора.
type TestCaseResult struct {
	Name     string
	Title    string
	Asserts  []interpreter.AssertOutcome
	Passed   int      // сколько проверок прошло
	Failed   int      // сколько проверок провалено
	Err      error    // ошибка выполнения (ВызватьИсключение, unknown function, …)
	Messages []string // вывод Сообщить
	Duration time.Duration
}

// OK — тест успешен: не упал с ошибкой и все проверки прошли. Тест без единой
// проверки считается неуспешным (пустой тест — почти всегда ошибка автора).
func (c TestCaseResult) OK() bool {
	return c.Err == nil && c.Failed == 0 && c.Passed > 0
}

// TestRunResult — итог всего прогона.
type TestRunResult struct {
	Cases []TestCaseResult
}

// OK — весь прогон успешен.
func (r TestRunResult) OK() bool {
	for _, c := range r.Cases {
		if !c.OK() {
			return false
		}
	}
	return true
}

// Totals агрегирует счётчики прогона.
func (r TestRunResult) Totals() (tests, passedTests, asserts, failedAsserts int) {
	for _, c := range r.Cases {
		tests++
		if c.OK() {
			passedTests++
		}
		asserts += c.Passed + c.Failed
		failedAsserts += c.Failed
	}
	return
}

// testRecorder реализует interpreter.AssertRecorder для одного теста.
type testRecorder struct{ outcomes []interpreter.AssertOutcome }

func (t *testRecorder) RecordAssert(o interpreter.AssertOutcome) {
	t.outcomes = append(t.outcomes, o)
}

// RunTests находит тест-обработки, гоняет каждую и собирает итоги. filter —
// необязательная маска по имени (регистронезависимая подстрока); пусто — все.
// Ошибка возвращается только при сбое окружения (сборка сервера, отсутствие
// обработки); провалы самих тестов — в TestRunResult.
func RunTests(ctx context.Context, proj *project.Project, db *storage.DB, filter string) (TestRunResult, error) {
	s, reg, err := NewOfflineServer(proj, db)
	if err != nil {
		return TestRunResult{}, err
	}

	var res TestRunResult
	for _, proc := range selectTests(proj, filter) {
		rec := &testRecorder{}
		assert := interpreter.NewAssertRoot(rec)
		extra := map[string]any{"Утверждать": assert, "Assert": assert}

		start := time.Now()
		msgs, runErr, envErr := s.RunProcessor(ctx, reg, proc.Name, nil, nil, extra)
		dur := time.Since(start)
		if envErr != nil {
			return res, envErr
		}

		c := TestCaseResult{
			Name:     proc.Name,
			Title:    proc.DisplayName(""),
			Asserts:  rec.outcomes,
			Err:      runErr,
			Messages: msgs,
			Duration: dur,
		}
		for _, o := range rec.outcomes {
			if o.Passed {
				c.Passed++
			} else {
				c.Failed++
			}
		}
		res.Cases = append(res.Cases, c)
	}
	return res, nil
}

// selectTests возвращает тест-обработки, отсортированные по имени (для
// детерминированного вывода), с учётом фильтра-подстроки.
func selectTests(proj *project.Project, filter string) []*processor.Processor {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out []*processor.Processor
	for _, p := range proj.Processors {
		if !p.IsTest() {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(p.Name), filter) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
