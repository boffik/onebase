package sheet

import (
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// PDF-рендер табличного документа (план 64, этап 2). Рисуем по координатам:
// сетка колонок (px→мм/%/mm), авто-высоты строк по SplitText, спаны через карту
// covered (как в html.go), границы legacy-пресетом, фоны, выравнивания H/V,
// авто- и явные разрывы страниц с повтором HeaderArea. Кириллица — через
// встроенные PT-шрифты (fonts.go), без транслитерации.

// Единицы и константы геометрии.
const (
	pxPerInch = 96.0
	mmPerInch = 25.4
	ptToMM    = mmPerInch / 72.0 // 1pt = 0.352777… мм
	lineGap   = 1.2              // межстрочный коэффициент
	cellPadMM = 1.0              // горизонтальный/вертикальный паддинг ячейки
	minRowMM  = 4.0              // минимальная высота строки
)

// pxToMM конвертирует пиксели (CSS, 96dpi) в миллиметры.
func pxToMM(px float64) float64 { return px * mmPerInch / pxPerInch }

// PDFOptions — параметры PDF-рендера. Title уходит в метаданные документа.
// Геометрия страницы берётся из Document.Page (PageSetup).
type PDFOptions struct {
	Title string
}

// orientFlag нормализует ориентацию в флаг fpdf ("P"/"L").
func orientFlag(orientation string) string {
	switch strings.ToLower(strings.TrimSpace(orientation)) {
	case "landscape", "ландшафт", "альбомная", "l":
		return "L"
	default:
		return "P"
	}
}

// pageFormat нормализует формат страницы для fpdf (по умолчанию A4).
func pageFormat(format string) string {
	f := strings.TrimSpace(format)
	if f == "" {
		return "A4"
	}
	return f
}

// orientLandscape сообщает, ландшафтная ли ориентация.
func orientLandscape(orientation string) bool { return orientFlag(orientation) == "L" }

// formatSizeMM возвращает портретные размеры (ширина, высота) известных
// форматов в мм. Для неизвестного формата — A4. Используется для пагинации
// DSL-вывода без создания fpdf-документа.
func formatSizeMM(format string) (w, h float64) {
	switch strings.ToUpper(strings.TrimSpace(format)) {
	case "A3":
		return 297, 420
	case "A5":
		return 148, 210
	case "LETTER":
		return 215.9, 279.4
	case "LEGAL":
		return 215.9, 355.6
	default: // A4
		return 210, 297
	}
}

// resolveFont выбирает семейство и начертание PT-шрифта по стилю ячейки.
// FontFamily с признаком serif/Times → PT Serif, иначе PT Sans.
// PT Sans Italic в комплекте нет: курсив-sans честно падает на PT Serif Italic.
func resolveFont(family string, bold, italic bool) (fontFamily, style string) {
	lf := strings.ToLower(family)
	serif := strings.Contains(lf, "serif") || strings.Contains(lf, "times") ||
		strings.Contains(lf, "georgia") || family == ""
	if italic && !serif {
		// PT Sans Italic отсутствует → используем PT Serif Italic.
		serif = true
	}
	if serif {
		fontFamily = "PTSerif"
	} else {
		fontFamily = "PTSans"
	}
	if bold {
		style += "B"
	}
	if italic {
		style += "I"
	}
	return fontFamily, style
}

// registerFonts регистрирует все 6 начертаний из встроенных байтов.
// fpdf субсетит шрифты при выводе, так что регистрация всех начертаний дёшева
// (см. спайк: ~70КБ на весь набор при реальном использовании одного).
func registerFonts(pdf *fpdf.Fpdf) {
	pdf.AddUTF8FontFromBytes("PTSerif", "", PTSerifRegular)
	pdf.AddUTF8FontFromBytes("PTSerif", "B", PTSerifBold)
	pdf.AddUTF8FontFromBytes("PTSerif", "I", PTSerifItalic)
	pdf.AddUTF8FontFromBytes("PTSerif", "BI", PTSerifBoldItalic)
	pdf.AddUTF8FontFromBytes("PTSans", "", PTSansRegular)
	pdf.AddUTF8FontFromBytes("PTSans", "B", PTSansBold)
}

// resolveColumnWidthsMM вычисляет ширины всех колонок документа в мм по доступной
// ширине usable (мм). Источник — Document.ColWidths (px, как из DSL ШиринаКолонки
// и HTML-рендера); незаданные колонки делят остаток поровну. Это обобщение
// computeColWidths из printform/pdf.go (там работали строки "%"/"mm"; здесь —
// числовые px из модели). colCount — число колонок (0-based maxCol+1).
func (d *Document) resolveColumnWidthsMM(usable float64, colCount int) []float64 {
	widths := make([]float64, colCount)
	used := 0.0
	free := make([]int, 0, colCount)
	for c := 0; c < colCount; c++ {
		px := d.ColWidths[c+1] // 1-based
		if px <= 0 {
			// fallback на индивидуальную ширину ячейки в этой колонке (px).
			if w := d.maxCellWidthPx(c); w > 0 {
				px = w
			}
		}
		if px > 0 {
			mm := pxToMM(px)
			widths[c] = mm
			used += mm
		} else {
			free = append(free, c)
		}
	}
	if len(free) > 0 {
		remaining := usable - used
		if remaining < float64(len(free))*5 {
			// Слишком тесно — дать каждой свободной колонке разумный минимум.
			remaining = float64(len(free)) * 20
		}
		each := remaining / float64(len(free))
		for _, c := range free {
			widths[c] = each
		}
	}
	return widths
}

// maxCellWidthPx возвращает максимальную индивидуальную ширину ячейки в колонке
// col (0-based), если ширина колонки не задана на документе.
func (d *Document) maxCellWidthPx(col int) float64 {
	max := 0.0
	for key, cell := range d.Cells {
		if key.Col == col && cell != nil && cell.Width > max {
			max = cell.Width
		}
	}
	return max
}

// fontSizeOr возвращает размер шрифта ячейки или дефолт (10pt).
func fontSizeOr(cell *Cell) float64 {
	if cell != nil && cell.FontSize > 0 {
		return float64(cell.FontSize)
	}
	return 10
}

// PDF рендерит документ в PDF и возвращает байты. Кириллица — встроенными
// PT-шрифтами без транслитерации.
func (d *Document) PDF(opts PDFOptions) ([]byte, error) {
	page := d.Page
	if page.Format == "" {
		page = DefaultPageSetup()
	}
	pdf := fpdf.New(orientFlag(page.Orientation), "mm", pageFormat(page.Format), "")
	registerFonts(pdf)

	m := page.MarginsMM
	pdf.SetMargins(m.Left, m.Top, m.Right)
	pdf.SetAutoPageBreak(false, m.Bottom)
	if opts.Title != "" {
		pdf.SetTitle(opts.Title, true)
	}
	pdf.AddPage()

	pageW, pageH := pdf.GetPageSize()
	usable := pageW - m.Left - m.Right
	pageBottom := pageH - m.Bottom

	maxRow, maxCol := d.ContentBounds()
	colCount := maxCol + 1
	colWidths := d.resolveColumnWidthsMM(usable, colCount)

	// Левый край колонки (мм) для быстрого позиционирования.
	colX := make([]float64, colCount+1)
	colX[0] = m.Left
	for c := 0; c < colCount; c++ {
		colX[c+1] = colX[c] + colWidths[c]
	}

	rowHeights := d.computeRowHeightsMM(maxRow, colWidths)

	// Множество строк с явным разрывом страницы ПЕРЕД ними.
	breakBefore := make(map[int]bool, len(d.PageBreaks))
	for _, r := range d.PageBreaks {
		breakBefore[r] = true
	}

	covered := make(map[CellKey]bool)

	// headerHeights — высоты строк повторяемой шапки (если задана).
	var headerRows int
	var headerHeights []float64
	if d.RepeatHeader && d.HeaderArea != nil {
		headerRows = d.HeaderArea.Rows()
		headerHeights = make([]float64, headerRows)
		for hr := 0; hr < headerRows; hr++ {
			headerHeights[hr] = minRowMM
		}
	}

	y := m.Top

	drawHeader := func() float64 {
		if d.HeaderArea == nil {
			return y
		}
		hy := m.Top
		for hr := 0; hr < headerRows; hr++ {
			d.drawAreaRow(pdf, d.HeaderArea, hr, colX, colWidths, colCount, hy, headerHeights[hr])
			hy += headerHeights[hr]
		}
		return hy
	}

	for row := 0; row <= maxRow; row++ {
		rh := rowHeights[row]

		// Явный разрыв страницы перед строкой.
		if breakBefore[row] && row > 0 {
			pdf.AddPage()
			y = m.Top
			if d.RepeatHeader && d.HeaderArea != nil {
				y = drawHeader()
			}
		}

		// Авто-разрыв: строка не помещается на текущей странице.
		if y+rh > pageBottom && row > 0 {
			pdf.AddPage()
			y = m.Top
			if d.RepeatHeader && d.HeaderArea != nil {
				y = drawHeader()
			}
		}

		d.drawDocRow(pdf, row, maxCol, colX, colWidths, colCount, y, rh, covered)
		y += rh
	}

	var w byteWriter
	if err := pdf.Output(&w); err != nil {
		return nil, err
	}
	return w.data, nil
}

// computeRowHeightsMM вычисляет высоты всех строк документа в мм. Заданная
// строчная высота (RowHeights, px) приоритетна; иначе авто = max по ячейкам
// строки: число строк текста при переносе по ширине ячейки × line height +
// паддинг.
func (d *Document) computeRowHeightsMM(maxRow int, colWidths []float64) []float64 {
	heights := make([]float64, maxRow+1)
	for row := 0; row <= maxRow; row++ {
		if px := d.RowHeights[row+1]; px > 0 {
			heights[row] = pxToMM(px)
			if heights[row] < minRowMM {
				heights[row] = minRowMM
			}
			continue
		}
		h := minRowMM
		for col := 0; col < len(colWidths); col++ {
			cell := d.GetCell(row, col)
			if cell == nil || cell.Text == "" {
				continue
			}
			// Ширина для переноса с учётом colspan.
			cw := colWidths[col]
			for cs := 1; cs < cell.ColSpan && col+cs < len(colWidths); cs++ {
				cw += colWidths[col+cs]
			}
			fs := fontSizeOr(cell)
			lineH := fs * ptToMM * lineGap
			avail := cw - 2*cellPadMM
			if avail <= 0 {
				avail = cw
			}
			lines := countWrappedLines(cell.Text, avail, fs)
			needed := float64(lines)*lineH + 2*cellPadMM
			// rowspan распределяет высоту на несколько строк — здесь упрощённо
			// учитываем как высоту одной строки (известное ограничение MVP).
			if cell.RowSpan > 1 {
				needed = needed / float64(cell.RowSpan)
			}
			if needed > h {
				h = needed
			}
		}
		heights[row] = h
	}
	return heights
}

// drawDocRow рисует одну строку документа: фон, границы, текст для каждой не
// перекрытой ячейки. covered обновляется colspan/rowspan.
func (d *Document) drawDocRow(pdf *fpdf.Fpdf, row, maxCol int, colX, colWidths []float64, colCount int, y, rh float64, covered map[CellKey]bool) {
	for col := 0; col <= maxCol && col < colCount; col++ {
		if covered[CellKey{row, col}] {
			continue
		}
		cell := d.GetCell(row, col)
		x := colX[col]

		// Ширина с учётом colspan.
		w := colWidths[col]
		if cell != nil && cell.ColSpan > 1 {
			for cs := 1; cs < cell.ColSpan && col+cs < colCount; cs++ {
				w += colWidths[col+cs]
			}
			for c := col + 1; c < col+cell.ColSpan; c++ {
				covered[CellKey{row, c}] = true
			}
		}
		// Высота с учётом rowspan (суммой высот строк документа недоступна
		// здесь — высоты строк ниже могут быть ещё не посчитаны; в MVP
		// rowspan-высота приближается высотой текущей строки × rowspan).
		h := rh
		if cell != nil && cell.RowSpan > 1 {
			h = rh * float64(cell.RowSpan)
			for r := row + 1; r < row+cell.RowSpan; r++ {
				for c := col; c < col+maxInt(cell.ColSpan, 1) && c < colCount; c++ {
					covered[CellKey{r, c}] = true
				}
			}
		}
		drawCell(pdf, cell, x, y, w, h)
	}
}

// drawAreaRow рисует строку области (HeaderArea) с относительными координатами.
func (d *Document) drawAreaRow(pdf *fpdf.Fpdf, area *Area, relRow int, colX, colWidths []float64, colCount int, y, rh float64) {
	cols := area.Cols()
	for c := 0; c < cols && c < colCount; c++ {
		key := fmt.Sprintf("%d,%d", relRow, c)
		cell := area.Cells[key]
		x := colX[c]
		w := colWidths[c]
		if cell != nil && cell.ColSpan > 1 {
			for cs := 1; cs < cell.ColSpan && c+cs < colCount; cs++ {
				w += colWidths[c+cs]
			}
		}
		drawCell(pdf, cell, x, y, w, rh)
	}
}

// drawCell рисует одну ячейку: фон, текст (с выравниванием H/V), границы.
func drawCell(pdf *fpdf.Fpdf, cell *Cell, x, y, w, h float64) {
	if cell == nil {
		return
	}

	// Фон.
	if cell.BackColor != "" {
		if r, g, b, ok := parseHexColor(cell.BackColor); ok {
			pdf.SetFillColor(r, g, b)
			pdf.Rect(x, y, w, h, "F")
		}
	}

	// Текст.
	if cell.Text != "" {
		family, style := resolveFont(cell.FontFamily, cell.Bold, cell.Italic)
		fs := fontSizeOr(cell)
		pdf.SetFont(family, style, fs)
		if cell.TextColor != "" {
			if r, g, b, ok := parseHexColor(cell.TextColor); ok {
				pdf.SetTextColor(r, g, b)
			}
		} else {
			pdf.SetTextColor(0, 0, 0)
		}

		lineH := fs * ptToMM * lineGap
		avail := w - 2*cellPadMM
		if avail <= 0 {
			avail = w
		}
		lines := wrapText(pdf, cell.Text, avail)
		textH := float64(len(lines)) * lineH

		// Вертикальное выравнивание блока текста.
		ty := y + cellPadMM
		switch strings.ToLower(cell.Vertical) {
		case "center", "middle", "центр":
			ty = y + (h-textH)/2
		case "bottom", "низ":
			ty = y + h - textH - cellPadMM
		}
		if ty < y {
			ty = y
		}

		align := pdfAlign(cell.Align)
		for _, line := range lines {
			pdf.SetXY(x+cellPadMM, ty)
			pdf.CellFormat(avail, lineH, line, "", 0, align+"M", false, 0, "")
			ty += lineH
		}
	}

	// Границы (legacy-пресет).
	drawCellBorder(pdf, cell, x, y, w, h)
}

// drawCellBorder рисует рамку ячейки по legacy-пресету Border ("all"/"thin"/
// "thick"/"none"/""). thin=0.2мм, thick=0.5мм. Цвет — чёрный.
func drawCellBorder(pdf *fpdf.Fpdf, cell *Cell, x, y, w, h float64) {
	preset := strings.ToLower(strings.TrimSpace(cell.Border))
	var lw float64
	switch preset {
	case "", "none":
		return
	case "thick":
		lw = 0.5
	case "thin", "all":
		lw = 0.2
	default:
		lw = 0.2
	}
	pdf.SetLineWidth(lw)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Rect(x, y, w, h, "D")
}

// pdfAlign конвертирует выравнивание ячейки в горизонтальный флаг fpdf.
func pdfAlign(align string) string {
	switch strings.ToLower(align) {
	case "center", "центр":
		return "C"
	case "right", "право":
		return "R"
	default:
		return "L"
	}
}

// wrapText разбивает текст по ширине avail (мм) текущим установленным шрифтом
// pdf, дополнительно уважая явные переводы строк.
func wrapText(pdf *fpdf.Fpdf, text string, avail float64) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		split := pdf.SplitText(para, avail)
		if len(split) == 0 {
			out = append(out, "")
			continue
		}
		out = append(out, split...)
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// countWrappedLines оценивает число строк текста при переносе по ширине avail
// (мм) для шрифта размера fs (pt) — для расчёта высоты строки до выбора шрифта
// на конкретной ячейке. Эвристика по средней ширине символа (≈0.5×fs в pt).
func countWrappedLines(text string, avail float64, fs float64) int {
	if avail <= 0 {
		return 1
	}
	total := 0
	avgCharMM := fs * 0.5 * ptToMM
	if avgCharMM <= 0 {
		avgCharMM = 1
	}
	perLine := int(avail / avgCharMM)
	if perLine < 1 {
		perLine = 1
	}
	for _, para := range strings.Split(text, "\n") {
		runes := len([]rune(para))
		if runes == 0 {
			total++
			continue
		}
		n := (runes + perLine - 1) / perLine
		if n < 1 {
			n = 1
		}
		total += n
	}
	if total < 1 {
		total = 1
	}
	return total
}

// parseHexColor парсит "#rrggbb"/"#rgb" в компоненты RGB.
func parseHexColor(s string) (r, g, b int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
		// ok
	default:
		return 0, 0, 0, false
	}
	var rv, gv, bv int
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &rv, &gv, &bv); err != nil {
		return 0, 0, 0, false
	}
	return rv, gv, bv, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// byteWriter — io.Writer, собирающий вывод fpdf в []byte.
type byteWriter struct{ data []byte }

func (w *byteWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}
