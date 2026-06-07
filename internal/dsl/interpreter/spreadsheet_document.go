package interpreter

import (
	"fmt"
	"strconv"
	"strings"
)

// ─── SpreadsheetDocumentCell (ЯчейкаТабличногоДокумента) ─────────────────────

// SpreadsheetDocumentCell represents a single cell in a spreadsheet document.
type SpreadsheetDocumentCell struct {
	text       string
	value      any
	align      string // left, center, right, justify
	vertical   string // top, center, bottom
	width      float64
	height     float64
	bold       bool
	italic     bool
	fontSize   int
	fontFamily string
	backColor  string
	textColor  string
	border     string
	fill       string
	picture    string
	colSpan    int
	rowSpan    int
	parameterName string // for макет: which named parameter fills this cell
}

// NewSpreadsheetDocumentCell creates a new cell with default formatting.
func NewSpreadsheetDocumentCell(text string) *SpreadsheetDocumentCell {
	return &SpreadsheetDocumentCell{
		text:       text,
		value:      text,
		align:      "left",
		vertical:   "top",
		fontSize:   10,
		fontFamily: "Times New Roman",
		border:     "all",
	}
}

// ─── SpreadsheetDocumentArea (ОбластьТабличногоДокумента) ─────────────────────

// SpreadsheetDocumentArea represents a rectangular area template with its own cells.
// Each area has independent cell storage — parameters are set in relative coordinates
// (R1C1 = top-left of area) and copied to the document during Вывести/Присоединить.
type SpreadsheetDocumentArea struct {
	document *SpreadsheetDocument
	cells    map[string]*SpreadsheetDocumentCell // own cell storage (relative coords)
	top      int
	left     int
	bottom   int
	right    int
	name     string // optional name for named areas
}

// areaRows returns the number of rows in the area.
func (a *SpreadsheetDocumentArea) areaRows() int {
	return a.bottom - a.top + 1
}

// areaCols returns the number of columns in the area.
func (a *SpreadsheetDocumentArea) areaCols() int {
	return a.right - a.left + 1
}

func (a *SpreadsheetDocumentArea) CallMethod(name string, args []any) any {
	switch name {
	case "параметры", "parameters":
		return a.parameters(args)
	case "параметр", "parameter":
		if len(args) > 0 {
			return a.getParameter(strArg(args, 0))
		}
	case "установитьпараметр", "setparameter":
		if len(args) >= 2 {
			a.setParameter(strArg(args, 0), args[1])
		}
	case "ширина", "width":
		return float64(a.areaCols())
	case "высота", "height":
		return float64(a.areaRows())
	case "очистить", "clear":
		a.clear()
	case "объединить", "merge":
		a.mergeArea()
	}
	return nil
}

// parameters returns an AreaParameters object for accessing area cells via dot notation.
func (a *SpreadsheetDocumentArea) parameters(args []any) any {
	return &AreaParameters{area: a}
}

// getParameter returns the value of a parameter by name (e.g., "R1C1") — relative to area.
func (a *SpreadsheetDocumentArea) getParameter(name string) any {
	row, col, ok := parseRC(name)
	if !ok {
		return ""
	}
	key := fmt.Sprintf("%d,%d", row-1, col-1)
	if cell, exists := a.cells[key]; exists {
		return cell.text
	}
	return ""
}

// setParameter sets the value of a parameter (cell) by name — relative to area.
func (a *SpreadsheetDocumentArea) setParameter(name string, value any) {
	row, col, ok := parseRC(name)
	if !ok {
		return
	}
	cell := a.areaGetOrCreateCell(row-1, col-1)
	cell.text = strArg([]any{value}, 0)
	cell.value = value
}

// clear removes all content in the area.
func (a *SpreadsheetDocumentArea) clear() {
	a.cells = make(map[string]*SpreadsheetDocumentCell)
}

// mergeArea merges all cells in the area into one.
func (a *SpreadsheetDocumentArea) mergeArea() {
	cols := a.areaCols()
	rows := a.areaRows()
	if rows == 1 && cols == 1 {
		return
	}
	cell := a.areaGetOrCreateCell(0, 0)
	cell.colSpan = cols
	cell.rowSpan = rows
}

// areaGetOrCreateCell gets or creates a cell at relative (row, col) within the area.
func (a *SpreadsheetDocumentArea) areaGetOrCreateCell(row, col int) *SpreadsheetDocumentCell {
	key := fmt.Sprintf("%d,%d", row, col)
	if cell, ok := a.cells[key]; ok {
		return cell
	}
	cell := NewSpreadsheetDocumentCell("")
	a.cells[key] = cell
	return cell
}

// Get allows accessing cells via dot notation (Area.R1C1) or properties (Area.Параметры).
func (a *SpreadsheetDocumentArea) Get(field string) any {
	switch strings.ToLower(field) {
	case "текст", "text":
		if cell, ok := a.cells["0,0"]; ok {
			return cell.text
		}
		return ""
	case "параметры", "parameters":
		return &AreaParameters{area: a}
	}
	return a.getParameter(field)
}

// Set allows setting cells via dot notation (Area.R1C1 = "value") or area properties.
func (a *SpreadsheetDocumentArea) Set(field string, v any) {
	a.setProperty(field, v)
}

// setProperty handles both R1C1 parameters and named properties on all area cells.
func (a *SpreadsheetDocumentArea) setProperty(field string, v any) {
	rows := a.areaRows()
	cols := a.areaCols()
	switch strings.ToLower(field) {
	case "текст", "text":
		text := strArg([]any{v}, 0)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				cell := a.areaGetOrCreateCell(r, c)
				cell.text = text
				cell.value = text
			}
		}
	case "шрифтжирный", "bold":
		bold := truthy(v)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).bold = bold
			}
		}
	case "размершрифта", "fontsize":
		size := int(toFloatOr0(v))
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).fontSize = size
			}
		}
	case "курсив", "italic":
		italic := truthy(v)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).italic = italic
			}
		}
	case "горизонтальноеположение", "horizontalalign", "halign":
		align := strings.ToLower(strArg([]any{v}, 0))
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).align = align
			}
		}
	case "вертикальноеположение", "verticalalign", "valign":
		vAlign := strings.ToLower(strArg([]any{v}, 0))
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).vertical = vAlign
			}
		}
	case "цветфона", "backcolor", "backgroundcolor":
		color := strArg([]any{v}, 0)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).backColor = color
			}
		}
	case "цветтекста", "textcolor", "fontcolor":
		color := strArg([]any{v}, 0)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				a.areaGetOrCreateCell(r, c).textColor = color
			}
		}
	default:
		// Try R1C1 format
		a.setParameter(field, v)
	}
}

// ─── AreaParameters ────────────────────────────────────────────────────────────

// AreaParameters provides dot-notation access to area cell values (R1C1, R1C2, etc.)
// Used when DSL code accesses Обл.Параметры.R1C1 = "value".
type AreaParameters struct {
	area *SpreadsheetDocumentArea
}

func (p *AreaParameters) Get(field string) any {
	// First try named parameter (from макет)
	for _, cell := range p.area.cells {
		if cell.parameterName != "" && strings.EqualFold(cell.parameterName, field) {
			return cell.text
		}
	}
	// Then try R1C1 format
	return p.area.getParameter(field)
}

func (p *AreaParameters) Set(field string, v any) {
	// First try named parameter (from макет)
	found := false
	for _, cell := range p.area.cells {
		if cell.parameterName != "" && strings.EqualFold(cell.parameterName, field) {
			cell.text = strArg([]any{v}, 0)
			cell.value = v
			found = true
		}
	}
	if found {
		return
	}
	// Then try R1C1 format
	p.area.setParameter(field, v)
}

func (p *AreaParameters) CallMethod(name string, args []any) any {
	return nil
}

// ─── parseRC helper ────────────────────────────────────────────────────────────

// parseRC parses "R<row>C<col>" format and returns 1-based row, col.
func parseRC(name string) (int, int, bool) {
	if !strings.HasPrefix(strings.ToUpper(name), "R") {
		return 0, 0, false
	}
	parts := strings.Split(strings.ToUpper(name), "C")
	if len(parts) != 2 {
		return 0, 0, false
	}
	row, err := strconv.Atoi(parts[0][1:])
	if err != nil {
		return 0, 0, false
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return row, col, true
}

// ─── SpreadsheetDocument (ТабличныйДокумент) ─────────────────────────────────

// SpreadsheetDocument represents a spreadsheet document for print forms.
type SpreadsheetDocument struct {
	cells        map[string]*SpreadsheetDocumentCell // "row,col" -> cell
	namedAreas   map[string]*SpreadsheetDocumentArea
	rowCount     int
	colCount     int
	currentRow   int
	currentCol   int
	pageBreaks   []int
	showMode     bool
	fileName     string
	repeatHeader bool
	headerArea   *SpreadsheetDocumentArea
	BackURL      string // URL for the "Назад" button
}

// NewSpreadsheetDocument creates a new empty spreadsheet document.
func NewSpreadsheetDocument() *SpreadsheetDocument {
	return &SpreadsheetDocument{
		cells:      make(map[string]*SpreadsheetDocumentCell),
		namedAreas: make(map[string]*SpreadsheetDocumentArea),
		rowCount:   100,
		colCount:   50,
		currentRow: 0,
		currentCol: 0,
	}
}

func (d *SpreadsheetDocument) CallMethod(name string, args []any) any {
	switch name {
	case "вывести", "put":
		return d.put(args)
	case "присоединить", "append":
		return d.append(args)
	case "область", "area":
		return d.area(args)
	case "получитьобласть", "getarea":
		if len(args) > 0 {
			return d.getNamedArea(strArg(args, 0))
		}
	case "показать", "show":
		d.show()
	case "записать", "write":
		if len(args) > 0 {
			return d.write(strArg(args, 0), args)
		}
	case "очистить", "clear":
		d.clear()
	case "удалитьобласть", "removearea":
		if len(args) > 0 {
			d.removeArea(args[0])
		}
	case "разделительстраниц", "pagebreak":
		d.pageBreak()
	case "проверитьвывод", "checkoutput":
		return d.checkOutput(args)
	case "закончитьстраницу", "endpage":
		d.endPage()
	case "повторитьприпечати", "repeatonprint":
		if len(args) >= 2 {
			d.repeatOnPrint(args[0], args[1])
		}
	case "нарисовать", "draw":
		if len(args) > 0 {
			d.draw(args[0])
		}
	case "получитьрисунок", "getpicture":
		if len(args) > 0 {
			return d.getPicture(args[0])
		}
	case "установитьимяобласти", "setareaname":
		if len(args) >= 2 {
			d.setAreaName(args[0], strArg(args, 1))
		}
	case "ширинаколонки", "columnwidth":
		if len(args) >= 2 {
			d.setColumnWidth(int(floatArg(args, 0)), args[1])
		}
	case "высотастроки", "rowheight":
		if len(args) >= 2 {
			d.setRowHeight(int(floatArg(args, 0)), args[1])
		}
	case "выровнять", "align":
		if len(args) >= 3 {
			d.setAlign(args[0], strArg(args, 1), strArg(args, 2))
		}
	case "объединить", "merge":
		if len(args) >= 4 {
			d.merge(int(floatArg(args, 0)), int(floatArg(args, 1)),
				int(floatArg(args, 2)), int(floatArg(args, 3)))
		}
	case "ячейка", "cell":
		if len(args) >= 2 {
			return d.getCellObj(int(floatArg(args, 0)), int(floatArg(args, 1)))
		}
	case "ширина", "width":
		return float64(d.colCount)
	case "высота", "height":
		return float64(d.rowCount)
	case "текущаястрока", "currentrow":
		return float64(d.currentRow + 1)
	case "текущаяколонка", "currentcol":
		return float64(d.currentCol + 1)
	}
	return nil
}

// put outputs an area at the current position and moves to the next line.
func (d *SpreadsheetDocument) put(args []any) any {
	if len(args) == 0 {
		return nil
	}
	area, ok := args[0].(*SpreadsheetDocumentArea)
	if !ok {
		return nil
	}

	rows := area.areaRows()
	cols := area.areaCols()

	// Copy area's own cells to document at current position
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			key := fmt.Sprintf("%d,%d", r, c)
			if srcCell, exists := area.cells[key]; exists {
				targetRow := d.currentRow + r
				targetCol := d.currentCol + c
				destCell := d.getOrCreateCell(targetRow, targetCol)
				*destCell = *srcCell
			}
		}
	}

	// Move to next line after the area
	d.currentRow += rows
	d.currentCol = 0
	return nil
}

// append appends an area to the right of the last output area.
func (d *SpreadsheetDocument) append(args []any) any {
	if len(args) == 0 {
		return nil
	}
	area, ok := args[0].(*SpreadsheetDocumentArea)
	if !ok {
		return nil
	}

	// Find current column position (rightmost cell in current row)
	maxCol := 0
	for col := 0; col < d.colCount; col++ {
		if d.getCell(d.currentRow, col) != nil {
			maxCol = col + 1
		}
	}
	d.currentCol = maxCol

	rows := area.areaRows()
	cols := area.areaCols()

	// Copy area's own cells
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			key := fmt.Sprintf("%d,%d", r, c)
			if srcCell, exists := area.cells[key]; exists {
				targetRow := d.currentRow + r
				targetCol := d.currentCol + c
				destCell := d.getOrCreateCell(targetRow, targetCol)
				*destCell = *srcCell
			}
		}
	}

	return nil
}

// area returns an area defined by coordinates (top, left, bottom, right) — 1-based.
func (d *SpreadsheetDocument) area(args []any) any {
	if len(args) < 4 {
		return nil
	}
	top := int(floatArg(args, 0)) - 1 // 1-based to 0-based
	left := int(floatArg(args, 1)) - 1
	bottom := int(floatArg(args, 2)) - 1
	right := int(floatArg(args, 3)) - 1

	return &SpreadsheetDocumentArea{
		document: d,
		cells:    make(map[string]*SpreadsheetDocumentCell),
		top:      top,
		left:     left,
		bottom:   bottom,
		right:    right,
	}
}

// getNamedArea returns a named area.
func (d *SpreadsheetDocument) getNamedArea(name string) *SpreadsheetDocumentArea {
	return d.namedAreas[strings.ToLower(name)]
}

// setAreaName assigns a name to an area.
func (d *SpreadsheetDocument) setAreaName(areaArg any, name string) {
	area, ok := areaArg.(*SpreadsheetDocumentArea)
	if ok {
		area.name = name
		d.namedAreas[strings.ToLower(name)] = area
	}
}

// show displays the document in a dialog (for now, just prints info).
func (d *SpreadsheetDocument) show() {
	d.showMode = true
}

// HTMLString returns the full HTML representation of the document.
func (d *SpreadsheetDocument) HTMLString() string {
	return d.toHTML()
}

// write saves the document to a file.
func (d *SpreadsheetDocument) write(fileName string, args []any) any {
	d.fileName = fileName
	fileType := "html"
	if len(args) > 1 {
		fileType = strings.ToLower(strArg(args, 1))
	}

	switch fileType {
	case "html", "":
		return d.writeHTML(fileName)
	case "pdf":
		return d.writePDF(fileName)
	case "txt":
		return d.writeTXT(fileName)
	}
	return nil
}

// writeHTML exports the document as HTML.
func (d *SpreadsheetDocument) writeHTML(fileName string) any {
	html := d.toHTML()
	fmt.Printf("// Запись файла: %s\n", fileName)
	return html
}

// writePDF exports the document as PDF.
func (d *SpreadsheetDocument) writePDF(fileName string) any {
	html := d.toHTML()
	fmt.Printf("// Запись PDF: %s (используется HTML)\n", fileName)
	return html
}

// writeTXT exports the document as plain text.
func (d *SpreadsheetDocument) writeTXT(fileName string) any {
	var sb strings.Builder
	// Determine actual content bounds
	maxRow, maxCol := d.contentBounds()
	for row := 0; row <= maxRow; row++ {
		for col := 0; col <= maxCol; col++ {
			if cell := d.getCell(row, col); cell != nil {
				sb.WriteString(cell.text)
				sb.WriteString("\t")
			}
		}
		sb.WriteString("\n")
	}
	result := sb.String()
	fmt.Printf("// Запись файла: %s\n", fileName)
	return result
}

// contentBounds returns the max row and col indices that have content.
func (d *SpreadsheetDocument) contentBounds() (int, int) {
	maxRow, maxCol := 0, 0
	for key, cell := range d.cells {
		if cell == nil {
			continue
		}
		var r, c int
		fmt.Sscanf(key, "%d,%d", &r, &c)
		// Consider colspan
		extent := c + cell.colSpan - 1
		if cell.colSpan <= 0 {
			extent = c
		}
		if r > maxRow {
			maxRow = r
		}
		if extent > maxCol {
			maxCol = extent
		}
	}
	return maxRow, maxCol
}

// toHTML converts the document to HTML representation.
func (d *SpreadsheetDocument) toHTML() string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<title>Табличный документ</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
@page{margin:1cm}
body{font-family:'Times New Roman',Times,serif;font-size:10pt;color:#000;padding:0}
.doc-toolbar{position:sticky;top:0;z-index:10;background:#f5f5f5;border-bottom:1px solid #ddd;padding:8px 16px;display:flex;gap:8px;align-items:center;font-family:Arial,sans-serif;font-size:13px}
.doc-toolbar button,.doc-toolbar a.btn{padding:6px 16px;border:1px solid #bbb;border-radius:4px;background:#fff;cursor:pointer;font-size:13px;text-decoration:none;color:#333;display:inline-block}
.doc-toolbar button:hover,.doc-toolbar a.btn:hover{background:#e8e8e8}
.doc-toolbar .btn-back{margin-right:auto}
.doc-toolbar .btn-print{background:#4a9;color:#fff;border-color:#4a9}
.doc-toolbar .btn-print:hover{background:#398}
.doc-content{padding:20px}
table{border-collapse:collapse;width:auto}
td,th{border:1px solid #cbd5e1;padding:3px 6px;empty-cells:show;font-size:13px}
tr:nth-child(even) td{background:#f8fafc}
@media print{.doc-toolbar{display:none}.doc-content{padding:0}}
</style>
</head>
<body>
<div class="doc-toolbar">
`)
	// Back button: link to document if BackURL is set, otherwise history.back()
	if d.BackURL != "" {
		sb.WriteString(`<a class="btn btn-back" href="` + escapeHTML(d.BackURL) + `">&#8592; Назад</a>`)
	} else {
		sb.WriteString(`<button class="btn-back" onclick="history.back()">&#8592; Назад</button>`)
	}
	sb.WriteString(`
<button class="btn-print" onclick="window.print()">&#128424; Печать</button>
<button onclick="window.print()">&#128196; PDF</button>
</div>
<div class="doc-content">
<table>`)

	maxRow, maxCol := d.contentBounds()

	// Track cells covered by colspan/rowspan
	covered := make(map[string]bool)

	for row := 0; row <= maxRow; row++ {
		sb.WriteString("<tr>")
		for col := 0; col <= maxCol; col++ {
			key := fmt.Sprintf("%d,%d", row, col)
			if covered[key] {
				continue
			}

			cell := d.getCell(row, col)
			var attrs, style string

			if cell != nil {
				style = d.buildCellStyle(cell)
				if cell.colSpan > 1 {
					attrs += fmt.Sprintf(` colspan="%d"`, cell.colSpan)
					for c := col + 1; c < col+cell.colSpan; c++ {
						covered[fmt.Sprintf("%d,%d", row, c)] = true
					}
				}
				if cell.rowSpan > 1 {
					attrs += fmt.Sprintf(` rowspan="%d"`, cell.rowSpan)
					for r := row + 1; r < row+cell.rowSpan; r++ {
						covered[fmt.Sprintf("%d,%d", r, col)] = true
					}
				}
			}

			text := ""
			if cell != nil {
				text = escapeHTML(cell.text)
			}

			if attrs != "" || style != "" {
				sb.WriteString(fmt.Sprintf("<td%s style=\"%s\">%s</td>", attrs, style, text))
			} else {
				sb.WriteString(fmt.Sprintf("<td>%s</td>", text))
			}
		}
		sb.WriteString("</tr>\n")
	}

	sb.WriteString(`</table>
</div>
</body>
</html>`)
	return sb.String()
}

// buildCellStyle builds CSS style string for a cell.
func (d *SpreadsheetDocument) buildCellStyle(cell *SpreadsheetDocumentCell) string {
	var styles []string

	if cell.align != "" && cell.align != "left" {
		styles = append(styles, "text-align:"+cell.align)
	}
	if cell.vertical != "" && cell.vertical != "top" {
		styles = append(styles, "vertical-align:"+cell.vertical)
	}
	if cell.bold {
		styles = append(styles, "font-weight:bold")
	}
	if cell.italic {
		styles = append(styles, "font-style:italic")
	}
	if cell.fontSize > 0 && cell.fontSize != 10 {
		styles = append(styles, fmt.Sprintf("font-size:%dpt", cell.fontSize))
	}
	if cell.fontFamily != "" && cell.fontFamily != "Times New Roman" {
		styles = append(styles, "font-family:"+cell.fontFamily)
	}
	if cell.width > 0 {
		styles = append(styles, fmt.Sprintf("width:%.2fpx", cell.width))
	}
	if cell.height > 0 {
		styles = append(styles, fmt.Sprintf("height:%.2fpx", cell.height))
	}
	if cell.backColor != "" {
		styles = append(styles, "background-color:"+cell.backColor)
	}
	if cell.textColor != "" {
		styles = append(styles, "color:"+cell.textColor)
	}

	return strings.Join(styles, ";")
}

// clear removes all content from the document.
func (d *SpreadsheetDocument) clear() {
	d.cells = make(map[string]*SpreadsheetDocumentCell)
	d.currentRow = 0
	d.currentCol = 0
	d.pageBreaks = nil
}

// removeArea deletes the specified area content.
func (d *SpreadsheetDocument) removeArea(areaArg any) {
	area, ok := areaArg.(*SpreadsheetDocumentArea)
	if !ok {
		return
	}
	for row := area.top; row <= area.bottom; row++ {
		for col := area.left; col <= area.right; col++ {
			key := fmt.Sprintf("%d,%d", row, col)
			delete(d.cells, key)
		}
	}
}

// pageBreak inserts a page break at the current position.
func (d *SpreadsheetDocument) pageBreak() {
	d.pageBreaks = append(d.pageBreaks, d.currentRow)
}

// checkOutput checks if an area will fit on the current page.
func (d *SpreadsheetDocument) checkOutput(args []any) any {
	if len(args) == 0 {
		return true
	}
	area, ok := args[0].(*SpreadsheetDocumentArea)
	if !ok {
		return true
	}
	areaHeight := area.areaRows()
	rowsRemaining := 50 - (d.currentRow % 50)
	return float64(areaHeight) <= float64(rowsRemaining)
}

// endPage finishes the current page and moves to the next.
func (d *SpreadsheetDocument) endPage() {
	nextPage := (d.currentRow/50 + 1) * 50
	d.currentRow = nextPage
	d.currentCol = 0
}

// repeatOnPrint sets an area to repeat on each page.
func (d *SpreadsheetDocument) repeatOnPrint(areaArg any, repeat any) {
	area, ok := areaArg.(*SpreadsheetDocumentArea)
	if ok && truthy(repeat) {
		d.repeatHeader = true
		d.headerArea = area
	}
}

// draw inserts a picture at the current position.
func (d *SpreadsheetDocument) draw(pictureArg any) {
	picture := strArg([]any{pictureArg}, 0)
	cell := d.getOrCreateCell(d.currentRow, d.currentCol)
	cell.picture = picture
	d.currentCol++
}

// getPicture extracts a picture from an area.
func (d *SpreadsheetDocument) getPicture(areaArg any) any {
	area, ok := areaArg.(*SpreadsheetDocumentArea)
	if !ok {
		return ""
	}
	for row := area.top; row <= area.bottom; row++ {
		for col := area.left; col <= area.right; col++ {
			if cell := d.getCell(row, col); cell != nil && cell.picture != "" {
				return cell.picture
			}
		}
	}
	return ""
}

// setColumnWidth sets the width of a column.
func (d *SpreadsheetDocument) setColumnWidth(col int, width any) {
	w := toFloatOr0(width)
	for row := 0; row < d.rowCount; row++ {
		cell := d.getOrCreateCell(row, col-1)
		cell.width = w
	}
}

// setRowHeight sets the height of a row.
func (d *SpreadsheetDocument) setRowHeight(row int, height any) {
	h := toFloatOr0(height)
	for col := 0; col < d.colCount; col++ {
		cell := d.getOrCreateCell(row-1, col)
		cell.height = h
	}
}

// setAlign sets alignment for an area.
func (d *SpreadsheetDocument) setAlign(areaArg any, hAlign, vAlign string) {
	area, ok := areaArg.(*SpreadsheetDocumentArea)
	if !ok {
		return
	}
	for row := area.top; row <= area.bottom; row++ {
		for col := area.left; col <= area.right; col++ {
			if cell := d.getOrCreateCell(row, col); cell != nil {
				cell.align = strings.ToLower(hAlign)
				cell.vertical = strings.ToLower(vAlign)
			}
		}
	}
}

// merge merges cells in an area.
func (d *SpreadsheetDocument) merge(top, left, bottom, right int) {
	if top < 0 || left < 0 || bottom < top || right < left {
		return
	}
	// Convert to 0-based
	top--
	left--
	bottom--
	right--

	if cell := d.getOrCreateCell(top, left); cell != nil {
		cell.colSpan = right - left + 1
		cell.rowSpan = bottom - top + 1
	}
}

// getCellObj returns a cell object for direct manipulation.
func (d *SpreadsheetDocument) getCellObj(row, col int) any {
	row-- // 1-based to 0-based
	col--
	cell := d.getOrCreateCell(row, col)
	return &SpreadsheetDocumentCellWrapper{cell: cell, doc: d, row: row, col: col}
}

// ─── SpreadsheetDocumentCellWrapper ───────────────────────────────────────────

// SpreadsheetDocumentCellWrapper provides direct access to a single cell.
type SpreadsheetDocumentCellWrapper struct {
	cell *SpreadsheetDocumentCell
	doc  *SpreadsheetDocument
	row  int
	col  int
}

func (w *SpreadsheetDocumentCellWrapper) Get(field string) any {
	switch strings.ToLower(field) {
	case "текст", "text":
		return w.cell.text
	case "значение", "value":
		return w.cell.value
	case "ширина", "width":
		return w.cell.width
	case "высота", "height":
		return w.cell.height
	case "выравнивание", "align":
		return w.cell.align
	case "вервыравнивание", "valign":
		return w.cell.vertical
	case "жирный", "bold":
		return w.cell.bold
	case "курсив", "italic":
		return w.cell.italic
	case "размершрифта", "fontsize":
		return float64(w.cell.fontSize)
	case "цветфона", "backcolor":
		return w.cell.backColor
	case "цветтекста", "textcolor":
		return w.cell.textColor
	case "рисунок", "picture":
		return w.cell.picture
	}
	return nil
}

func (w *SpreadsheetDocumentCellWrapper) Set(field string, v any) {
	switch strings.ToLower(field) {
	case "текст", "text":
		w.cell.text = strArg([]any{v}, 0)
	case "значение", "value":
		w.cell.value = v
	case "ширина", "width":
		w.cell.width = toFloatOr0(v)
	case "высота", "height":
		w.cell.height = toFloatOr0(v)
	case "выравнивание", "align":
		w.cell.align = strings.ToLower(strArg([]any{v}, 0))
	case "вервыравнивание", "valign":
		w.cell.vertical = strings.ToLower(strArg([]any{v}, 0))
	case "жирный", "bold":
		w.cell.bold = truthy(v)
	case "курсив", "italic":
		w.cell.italic = truthy(v)
	case "размершрифта", "fontsize":
		w.cell.fontSize = int(toFloatOr0(v))
	case "цветфона", "backcolor":
		w.cell.backColor = strArg([]any{v}, 0)
	case "цветтекста", "textcolor":
		w.cell.textColor = strArg([]any{v}, 0)
	case "рисунок", "picture":
		w.cell.picture = strArg([]any{v}, 0)
	}
}

func (w *SpreadsheetDocumentCellWrapper) CallMethod(name string, args []any) any {
	return nil
}

// ─── Helper methods ───────────────────────────────────────────────────────────

func (d *SpreadsheetDocument) getCell(row, col int) *SpreadsheetDocumentCell {
	key := fmt.Sprintf("%d,%d", row, col)
	return d.cells[key]
}

func (d *SpreadsheetDocument) getOrCreateCell(row, col int) *SpreadsheetDocumentCell {
	key := fmt.Sprintf("%d,%d", row, col)
	if cell, ok := d.cells[key]; ok {
		return cell
	}
	cell := NewSpreadsheetDocumentCell("")
	d.cells[key] = cell
	return cell
}

func (d *SpreadsheetDocument) setCell(row, col int, text string) {
	cell := d.getOrCreateCell(row, col)
	cell.text = text
	cell.value = text
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// ─── Factory function for Новый ТабличныйДокумент ─────────────────────────────

func newSpreadsheetDocument(args []any) any {
	return NewSpreadsheetDocument()
}

// NewSpreadsheetFunctions returns a map of spreadsheet-related functions and factories.
func NewSpreadsheetFunctions() map[string]any {
	return map[string]any{
		"__factory_ТабличныйДокумент":    newSpreadsheetDocument,
		"__factory_SpreadsheetDocument": newSpreadsheetDocument,
	}
}
