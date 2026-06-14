package sheet

import (
	"strings"
	"testing"
)

// Валидный 8x8 PNG в data-URI (для проверки встраивания картинки richtext).
// 1x1-PNG не годится: вендорный fpdf паникует на минимальном PNG без IDAT-данных
// (его и должен гасить recover в drawCell), а нам тут нужна реально встраиваемая
// картинка для проверки роста размера PDF.
const tinyPNGDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAHElEQVR4nGJhYKiQY2DARCwgAhsYnBKAAAAA//9VCQI9fAHmsQAAAABJRU5ErkJggg=="

// TestHTMLRichHTMLNotEscaped — RichHTML выводится как HTML-блок (теги НЕ
// экранированы), картинка-img присутствует; обычная ячейка не затрагивается.
func TestHTMLRichHTMLNotEscaped(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.RichHTML = `<p><b>Готово</b> <i>выполнено</i></p><img src="` + tinyPNGDataURI + `" alt="">`

	htmlOut := d.HTML(HTMLOptions{})

	for _, want := range []string{"<b>Готово</b>", "<i>выполнено</i>", "<img src=", "class=\"rich\""} {
		if !strings.Contains(htmlOut, want) {
			t.Errorf("HTML не содержит %q\n%s", want, htmlOut)
		}
	}
	// Теги richtext НЕ должны быть экранированы.
	if strings.Contains(htmlOut, "&lt;b&gt;") {
		t.Errorf("RichHTML экранирован, а не выведен как HTML")
	}
	// richtext-CSS подключается.
	if !strings.Contains(htmlOut, ".rich img") {
		t.Errorf("richtext-CSS не добавлен при наличии RichHTML")
	}
}

// TestHTMLNoRichCSSWithoutRich — без richtext .rich-CSS НЕ добавляется (golden).
func TestHTMLNoRichCSSWithoutRich(t *testing.T) {
	d := NewDocument()
	d.SetCell(0, 0, "Обычный текст")
	htmlOut := d.HTML(HTMLOptions{})
	if strings.Contains(htmlOut, ".rich") {
		t.Errorf("richtext-CSS добавлен без richtext-ячеек (ломает golden)")
	}
}

// TestPDFRichHTMLDoesNotPanic — PDF с richtext-ячейкой (форматирование +
// картинка + список) формируется без паники и не пустой.
func TestPDFRichHTMLDoesNotPanic(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.RichHTML = `<h2>Отчёт</h2><p>Текст <b>жирный</b> и <i>курсив</i>.</p>` +
		`<ul><li>первый</li><li>второй</li></ul>` +
		`<ol><li>раз</li><li>два</li></ol>` +
		`<img src="` + tinyPNGDataURI + `" alt="">`
	d.SetColumnWidth(1, 400)

	pdfBytes, err := d.PDF(PDFOptions{Title: "test"})
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Errorf("PDF слишком мал (%d байт)", len(pdfBytes))
	}
	if !strings.HasPrefix(string(pdfBytes[:5]), "%PDF-") {
		t.Errorf("выходные байты не похожи на PDF")
	}
}

// TestPDFRichWithImageLargerThanWithout — тот же richtext, но с встроенной
// картинкой, даёт больший PDF, чем без неё (картинка реально встроена в поток).
// Сравниваем одинаковый текст ±картинка, чтобы исключить влияние субсеттинга шрифтов.
func TestPDFRichWithImageLargerThanWithout(t *testing.T) {
	const text = `<p>С картинкой одинаковый текст</p>`

	noImg := NewDocument()
	noImg.GetOrCreateCell(0, 0).RichHTML = text
	noImg.SetColumnWidth(1, 300)
	noImgBytes, err := noImg.PDF(PDFOptions{})
	if err != nil {
		t.Fatalf("no-img PDF: %v", err)
	}

	withImg := NewDocument()
	withImg.GetOrCreateCell(0, 0).RichHTML = text + `<img src="` + tinyPNGDataURI + `" alt="">`
	withImg.SetColumnWidth(1, 300)
	withImgBytes, err := withImg.PDF(PDFOptions{})
	if err != nil {
		t.Fatalf("with-img PDF: %v", err)
	}

	if len(withImgBytes) <= len(noImgBytes) {
		t.Errorf("PDF с картинкой (%d) не больше PDF без неё (%d)", len(withImgBytes), len(noImgBytes))
	}
}

// TestPDFRichBrokenHTMLSurvives — битый/сложный HTML не валит PDF (устойчивость).
func TestPDFRichBrokenHTMLSurvives(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.RichHTML = `<p>незакрытый <b>жирный <ul><li>вложенный<table><tr><td>мусор` +
		`<img src="data:image/png;base64,НЕВАЛИДНЫЙ"><script>alert(1)</script>`
	d.SetColumnWidth(1, 200)

	if _, err := d.PDF(PDFOptions{}); err != nil {
		t.Fatalf("PDF на битом HTML вернул ошибку: %v", err)
	}
}

// TestParseRichHTMLBlocks — базовый разбор: абзацы, начертания, списки.
func TestParseRichHTMLBlocks(t *testing.T) {
	blocks := parseRichHTML(`<p>Привет <b>мир</b></p><ul><li>a</li><li>b</li></ul><ol><li>x</li></ol>`)
	if len(blocks) != 4 {
		t.Fatalf("ожидалось 4 блока, получено %d: %+v", len(blocks), blocks)
	}
	// Первый блок — абзац с двумя сегментами (обычный + жирный).
	if len(blocks[0].segs) < 2 {
		t.Errorf("абзац: ожидалось >=2 сегмента, получено %d", len(blocks[0].segs))
	}
	if !hasBoldSeg(blocks[0].segs) {
		t.Errorf("абзац: нет жирного сегмента")
	}
	// Элементы маркированного списка получают «• ».
	if blocks[1].marker != "• " || blocks[2].marker != "• " {
		t.Errorf("ul-маркеры неверны: %q %q", blocks[1].marker, blocks[2].marker)
	}
	// Нумерованный список — «1. ».
	if blocks[3].marker != "1. " {
		t.Errorf("ol-маркер неверен: %q", blocks[3].marker)
	}
}

// TestParseRichHTMLImage — img-сегмент извлекается из проекции.
func TestParseRichHTMLImage(t *testing.T) {
	blocks := parseRichHTML(`<p>текст</p><img src="` + tinyPNGDataURI + `" alt="">`)
	found := false
	for _, blk := range blocks {
		for _, s := range blk.segs {
			if s.img != "" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("img-сегмент не найден в блоках: %+v", blocks)
	}
}

func hasBoldSeg(segs []richSegment) bool {
	for _, s := range segs {
		if s.bold {
			return true
		}
	}
	return false
}
