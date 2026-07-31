package excel

import (
	"fmt"
	"io"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportList builds an xlsx workbook from headers + rows and returns the raw bytes.
// Cells are formatted: dates → "ДД.ММ.ГГГГ", numbers → right-aligned.
func ExportList(cols []string, rows [][]any) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Лист1"
	f.SetSheetName("Sheet1", sheet)

	boldStyle, cellStyle, numStyle := listStyles(f)

	// Header row
	for ci, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		f.SetCellValue(sheet, cell, col)
		f.SetCellStyle(sheet, cell, cell, boldStyle)
	}
	f.SetRowHeight(sheet, 1, 22)

	// Freeze header row
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Data rows
	for ri, row := range rows {
		rowIdx := ri + 2
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, rowIdx)
			switch v := val.(type) {
			case time.Time:
				f.SetCellValue(sheet, cell, v.Format("02.01.2006"))
				f.SetCellStyle(sheet, cell, cell, cellStyle)
			case float64, float32, int, int32, int64, uint, uint32, uint64:
				f.SetCellValue(sheet, cell, v)
				f.SetCellStyle(sheet, cell, cell, numStyle)
			case nil:
				f.SetCellValue(sheet, cell, "")
				f.SetCellStyle(sheet, cell, cell, cellStyle)
			default:
				f.SetCellValue(sheet, cell, fmt.Sprintf("%v", v))
				f.SetCellStyle(sheet, cell, cell, cellStyle)
			}
		}
		f.SetRowHeight(sheet, rowIdx, 18)
	}

	// Auto column width (approximate: max 40 chars)
	for ci, col := range cols {
		colLetter, _ := excelize.ColumnNumberToName(ci + 1)
		maxLen := len(col)
		for _, row := range rows {
			if ci < len(row) {
				s := fmt.Sprintf("%v", row[ci])
				if len(s) > maxLen {
					maxLen = len(s)
				}
			}
		}
		w := float64(maxLen) * 1.1
		if w < 8 {
			w = 8
		}
		if w > 40 {
			w = 40
		}
		f.SetColWidth(sheet, colLetter, colLetter, w)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// listStyles registers the shared list-export cell styles on f and returns their
// IDs: bold frozen header, bordered data cell, and right-aligned number cell.
// Shared by ExportList and WriteList so both render identically.
func listStyles(f *excelize.File) (bold, cell, num int) {
	bold, _ = f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E2E8F0"}, Pattern: 1},
		Font: &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "CBD5E1", Style: 1},
			{Type: "right", Color: "CBD5E1", Style: 1},
			{Type: "top", Color: "CBD5E1", Style: 1},
			{Type: "bottom", Color: "CBD5E1", Style: 1},
		},
	})
	cell, _ = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: false},
		Border: []excelize.Border{
			{Type: "left", Color: "E2E8F0", Style: 1},
			{Type: "right", Color: "E2E8F0", Style: 1},
			{Type: "top", Color: "E2E8F0", Style: 1},
			{Type: "bottom", Color: "E2E8F0", Style: 1},
		},
	})
	num, _ = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "top"},
		Border: []excelize.Border{
			{Type: "left", Color: "E2E8F0", Style: 1},
			{Type: "right", Color: "E2E8F0", Style: 1},
			{Type: "top", Color: "E2E8F0", Style: 1},
			{Type: "bottom", Color: "E2E8F0", Style: 1},
		},
	})
	return
}

// WriteList streams an xlsx workbook (headers + rows) straight to w with a
// StreamWriter, so neither the full cell model nor the finished file is held in
// memory — unlike ExportList, which returns the whole workbook as a []byte. Same
// visual result as ExportList (bold frozen header, bordered cells, right-aligned
// numbers, dd.mm.yyyy dates, approximate auto column width). Use it for large
// HTTP exports. rows is expected to already be in memory here, so scanning it for
// column widths costs time but not extra memory. План 111 (P2-3).
//
// StreamWriter requires ascending row order and column widths set before any
// row; SetRow errors surface before any bytes reach w, so a caller can still
// send an error status as long as it has not written to w yet.
func WriteList(w io.Writer, cols []string, rows [][]any) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Лист1"
	f.SetSheetName("Sheet1", sheet)
	boldStyle, cellStyle, numStyle := listStyles(f)

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}

	// Column widths must be set before any row is written in stream mode.
	for ci := range cols {
		maxLen := len(cols[ci])
		for _, row := range rows {
			if ci < len(row) {
				if s := fmt.Sprintf("%v", row[ci]); len(s) > maxLen {
					maxLen = len(s)
				}
			}
		}
		width := float64(maxLen) * 1.1
		if width < 8 {
			width = 8
		}
		if width > 40 {
			width = 40
		}
		if err := sw.SetColWidth(ci+1, ci+1, width); err != nil {
			return err
		}
	}

	// Freeze the header row.
	if err := sw.SetPanes(&excelize.Panes{
		Freeze: true, Split: false, YSplit: 1,
		TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return err
	}

	// Header row.
	header := make([]any, len(cols))
	for ci, col := range cols {
		header[ci] = excelize.Cell{StyleID: boldStyle, Value: col}
	}
	if err := sw.SetRow("A1", header, excelize.RowOpts{Height: 22}); err != nil {
		return err
	}

	// Data rows.
	for ri, row := range rows {
		cells := make([]any, len(row))
		for ci, val := range row {
			switch v := val.(type) {
			case time.Time:
				cells[ci] = excelize.Cell{StyleID: cellStyle, Value: v.Format("02.01.2006")}
			case float64, float32, int, int32, int64, uint, uint32, uint64:
				cells[ci] = excelize.Cell{StyleID: numStyle, Value: v}
			case nil:
				cells[ci] = excelize.Cell{StyleID: cellStyle, Value: ""}
			default:
				cells[ci] = excelize.Cell{StyleID: cellStyle, Value: fmt.Sprintf("%v", v)}
			}
		}
		cellRef, _ := excelize.CoordinatesToCellName(1, ri+2)
		if err := sw.SetRow(cellRef, cells, excelize.RowOpts{Height: 18}); err != nil {
			return err
		}
	}

	if err := sw.Flush(); err != nil {
		return err
	}
	return f.Write(w)
}
