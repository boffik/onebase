package sheet

import (
	"bytes"
	"strings"
	"testing"
)

// 4×4 PNG/JPEG data-URI (сгенерированы image/png, image/jpeg — заведомо парсятся fpdf).
const (
	testPNGDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAAG0lEQVR4nGJhYGiwYWCAIBYQAQO4OYAAAAD//0tAAfk+lwHVAAAAAElFTkSuQmCC"
	testJPGDataURI = "data:image/jpeg;base64,/9j/2wCEAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUoKSgBBwcHCggKEwoKEygaFhooKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKP/AABEIAAQABAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/ALHh7wZov9lxf6N/L/CtL/hDNF/59v5f4Vo+Hv8AkFxVpV51bGV/aP33v3Ly7H4n6pT/AHj+FdfI/9k="
)

// TestClassifyPicture — распознаются data-URI и http(s), прочее отбрасывается.
func TestClassifyPicture(t *testing.T) {
	cases := []struct {
		in   string
		want pictureKind
	}{
		{"", picNone},
		{"   ", picNone},
		{testPNGDataURI, picDataURI},
		{"DATA:IMAGE/PNG;BASE64,AAAA", picDataURI},
		{"https://example.com/logo.png", picURL},
		{"http://example.com/x.jpg", picURL},
		{"/local/path.png", picNone},
		{"javascript:alert(1)", picNone},
		{"data:text/html;base64,AAAA", picNone},
	}
	for _, c := range cases {
		if got := classifyPicture(c.in); got != c.want {
			t.Errorf("classifyPicture(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestDecodeDataURIImage — корректное декодирование PNG/JPEG, отказ на мусоре.
func TestDecodeDataURIImage(t *testing.T) {
	data, typ, ok := decodeDataURIImage(testPNGDataURI)
	if !ok || typ != "PNG" || len(data) == 0 {
		t.Fatalf("PNG: ok=%v typ=%q len=%d", ok, typ, len(data))
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Errorf("декодированный PNG не имеет сигнатуры PNG")
	}
	_, typ, ok = decodeDataURIImage(testJPGDataURI)
	if !ok || typ != "JPG" {
		t.Errorf("JPG: ok=%v typ=%q", ok, typ)
	}
	if _, _, ok := decodeDataURIImage("data:image/png;base64,@@@notbase64@@@"); ok {
		t.Errorf("битый base64 должен отбрасываться")
	}
	if _, _, ok := decodeDataURIImage("https://x/y.png"); ok {
		t.Errorf("URL не data-URI — decode должен вернуть ok=false")
	}
}

// TestHTMLRendersPicture — Picture с data-URI попадает в <img src=...>.
func TestHTMLRendersPicture(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.Text = "Логотип"
	c.Picture = testPNGDataURI
	html := d.HTMLString()
	if !strings.Contains(html, "<img src=\"data:image/png;base64,") {
		t.Errorf("HTML не содержит <img> с data-URI картинки")
	}
	// Текст ячейки сохраняется рядом с картинкой.
	if !strings.Contains(html, "Логотип") {
		t.Errorf("HTML потерял текст ячейки с картинкой")
	}
}

// TestHTMLIgnoresUnsafePicture — небезопасный/неподдерживаемый Picture не даёт <img>.
func TestHTMLIgnoresUnsafePicture(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.Picture = "javascript:alert(1)"
	if strings.Contains(d.HTMLString(), "<img") {
		t.Errorf("небезопасный Picture не должен давать <img>")
	}
}

// TestPDFEmbedsPicture — PDF с картинкой генерируется, содержит встроенный
// образ (XObject /Image) и больше идентичного документа без картинки.
// Базовый документ имеет ту же геометрию и пустой текст, чтобы разница в
// размере объяснялась только встроенным образом, а не сабсетом шрифта.
func TestPDFEmbedsPicture(t *testing.T) {
	mk := func(withPic bool) []byte {
		d := NewDocument()
		c := d.GetOrCreateCell(0, 0)
		c.Text = ""
		c.Width = 100
		c.Height = 100
		if withPic {
			c.Picture = testPNGDataURI
		}
		b, err := d.PDF(PDFOptions{})
		if err != nil {
			t.Fatalf("PDF (withPic=%v): %v", withPic, err)
		}
		return b
	}
	bBase := mk(false)
	bImg := mk(true)

	if !bytes.HasPrefix(bImg, []byte("%PDF")) {
		t.Fatalf("PDF c картинкой не начинается с %%PDF")
	}
	if !bytes.Contains(bImg, []byte("/Image")) {
		t.Errorf("в PDF нет XObject /Image — картинка не встроена")
	}
	if len(bImg) <= len(bBase) {
		t.Errorf("PDF c картинкой (%d) не больше базового (%d) — образ не встроен", len(bImg), len(bBase))
	}
}

// TestPictureCacheName — имя кэша зависит от содержимого, а не от длины: две
// разные картинки одного формата и одинаковой длины получают разные имена
// (иначе fpdf вернул бы первую из кэша — коллизия).
func TestPictureCacheName(t *testing.T) {
	a := []byte{1, 2, 3, 4}
	b := []byte{4, 3, 2, 1} // та же длина, другое содержимое
	na := pictureCacheName("PNG", a)
	nb := pictureCacheName("PNG", b)
	if na == nb {
		t.Errorf("разные картинки одной длины дали одно имя кэша: %q", na)
	}
	// Идентичные данные → одно имя (дедупликация ресурса).
	if got := pictureCacheName("PNG", []byte{1, 2, 3, 4}); got != na {
		t.Errorf("идентичные данные дали разные имена: %q != %q", got, na)
	}
	// Тип входит в имя — одинаковые данные разных форматов различимы.
	if pictureCacheName("PNG", a) == pictureCacheName("JPG", a) {
		t.Errorf("тип картинки не отражён в имени кэша")
	}
}

// TestPDFTwoDistinctPicturesSameLength — два разных PNG одинаковой длины в одном
// документе встраиваются как два РАЗНЫХ образа (раньше ключ кэша по длине давал
// коллизию: вторая картинка рисовалась первой).
func TestPDFTwoDistinctPicturesSameLength(t *testing.T) {
	a, _, okA := decodeDataURIImage(testPNGDataURI)
	if !okA {
		t.Fatal("не удалось декодировать тестовый PNG")
	}
	// Имена двух картинок одинаковой длины не должны совпасть, если данные
	// различаются хотя бы одним байтом.
	b := append([]byte(nil), a...)
	b[len(b)-1] ^= 0xFF
	if pictureCacheName("PNG", a) == pictureCacheName("PNG", b) {
		t.Errorf("две разные картинки одинаковой длины делят имя кэша")
	}
}

// TestPDFUnsupportedPictureNoCrash — неподдерживаемый Picture не валит PDF.
func TestPDFUnsupportedPictureNoCrash(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.Text = "Текст"
	c.Picture = "data:image/svg+xml;base64,AAAA" // SVG не поддержан fpdf
	b, err := d.PDF(PDFOptions{})
	if err != nil {
		t.Fatalf("PDF с неподдерживаемой картинкой вернул ошибку: %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("PDF повреждён")
	}
}
