package pdfimport

import (
	"testing"
	"time"
)

// TestGridFeasibleBounds проверяет краевые значения решения grid/fallback:
// граница maxGridLines включительна, превышение хотя бы по одной оси → fallback,
// и ниже minGridLines тоже fallback.
func TestGridFeasibleBounds(t *testing.T) {
	cases := []struct {
		name   string
		nH, nV int
		want   bool
	}{
		{"оба ровно maxGridLines — сетка", maxGridLines, maxGridLines, true},
		{"H превышает maxGridLines — fallback", maxGridLines + 1, maxGridLines, false},
		{"V превышает maxGridLines — fallback", maxGridLines, maxGridLines + 1, false},
		{"оба превышают — fallback", maxGridLines + 1, maxGridLines + 1, false},
		{"ниже minGridLines — fallback", minGridLines - 1, maxGridLines, false},
		{"нормальная сетка", 5, 5, true},
	}
	for _, c := range cases {
		if got := gridFeasible(c.nH, c.nV); got != c.want {
			t.Errorf("%s: gridFeasible(%d,%d)=%v, ожидалось %v", c.name, c.nH, c.nV, got, c.want)
		}
	}
}

// TestBuildLayoutHugeGridFallsBack — главный анти-OOM тест: вход с числом линий
// много больше maxGridLines не должен паниковать, не должен выделять
// O(nRows×nCols) памяти (это десятки ГБ) и не должен делать span-детект
// O(nRows×nCols×(nRows+nCols)). Мы строим синтетический extractedPage с >
// maxGridLines различными горизонтальными и вертикальными линиями и проверяем,
// что buildLayout быстро завершается (уходит в fallback), а не зависает/падает.
func TestBuildLayoutHugeGridFallsBack(t *testing.T) {
	const n = maxGridLines + 500 // заведомо больше предела по обеим осям

	ep := &extractedPage{
		Geom: pageGeom{MediaX1: 100000, MediaY1: 100000},
	}
	// n горизонтальных линий на разных Y и n вертикальных на разных X (шаг 3pt,
	// больше snapEps — линии не схлопнутся, дадут ~n различных cut'ов каждая).
	for i := 0; i < n; i++ {
		y := float64(i) * 3
		x := float64(i) * 3
		ep.Lines = append(ep.Lines,
			lineSeg{X1: 0, Y1: y, X2: 90000, Y2: y, Width: 0.5, Horizontal: true},
			lineSeg{X1: x, Y1: 0, X2: x, Y2: 90000, Width: 0.5, Horizontal: false},
		)
	}
	// Немного текста, чтобы fallback что-то вернул.
	ep.Runs = []textRun{
		{X: 10, Y: 99000, W: 20, FontSize: 10, S: "Текст"},
	}

	done := make(chan *struct{ panicked bool }, 1)
	go func() {
		res := &struct{ panicked bool }{}
		defer func() {
			if r := recover(); r != nil {
				res.panicked = true
				t.Errorf("buildLayout запаниковал на огромной сетке: %v", r)
			}
			done <- res
		}()
		tpl := buildLayout(ep)
		if tpl == nil {
			t.Errorf("buildLayout вернул nil")
		}
	}()

	select {
	case <-done:
		// Быстро завершился — значит ушёл в fallback, гигантский make не делался.
	case <-time.After(15 * time.Second):
		// Если бы выбрался grid-путь, span-детект на ~n×n ячейках работал бы
		// несоизмеримо дольше (а make[][]bool сожрал бы память до OOM).
		t.Fatal("buildLayout не завершился за 15с — похоже, выбран grid-путь вместо fallback (OOM-вектор не закрыт)")
	}
}
