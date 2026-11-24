package api

import (
	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/report/expreval"
)

// newReportEvaluator строит evaluator условий/показателей компоновки для REST.
// Реализация вынесена в общий expreval.Evaluator (issue #788): раньше UI и API
// держали две почти одинаковые копии, и UI-копия исполняла формулы без
// песочницы. Профиль лимитов обязателен и передаётся явно.
func newReportEvaluator(interp *interpreter.Interpreter) *expreval.Evaluator {
	return expreval.New(interp, expreval.DefaultProfile())
}
