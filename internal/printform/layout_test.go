package printform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLayoutRoundTrip ensures every new field survives marshal → unmarshal.
func TestLayoutRoundTrip(t *testing.T) {
	src := &LayoutTemplate{
		Name:     "Накладная",
		Document: "РеализацияТоваров",
		Columns: []LayoutColumn{
			{Width: "120px"},
			{Width: "auto"},
		},
		Areas: map[string]*LayoutArea{
			"Шапка": {
				Rows: []LayoutRow{
					{
						Height: "20px",
						Cells: []LayoutCell{
							{
								Text:        "Поставщик",
								Bold:        true,
								Italic:      true,
								Align:       "center",
								VAlign:      "middle",
								FontSize:    12,
								FontFamily:  "Arial",
								BackColor:   "#E8E8E8",
								TextColor:   "#000000",
								ColSpan:     2,
								RowSpan:     1,
								Border:      "thick",
								BorderColor: "#333",
							},
							{
								Parameter: "ИмяПоставщика",
							},
						},
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got LayoutTemplate
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != "Накладная" || got.Document != "РеализацияТоваров" {
		t.Fatalf("name/document lost: %+v", got)
	}
	if len(got.Columns) != 2 || got.Columns[0].Width != "120px" {
		t.Fatalf("columns lost: %+v", got.Columns)
	}
	area, ok := got.Areas["Шапка"]
	if !ok {
		t.Fatalf("area Шапка missing")
	}
	if len(area.Rows) != 1 || area.Rows[0].Height != "20px" {
		t.Fatalf("row height lost: %+v", area.Rows)
	}
	c := area.Rows[0].Cells[0]
	if c.Text != "Поставщик" || !c.Bold || !c.Italic ||
		c.Align != "center" || c.VAlign != "middle" ||
		c.FontSize != 12 || c.FontFamily != "Arial" ||
		c.BackColor != "#E8E8E8" || c.TextColor != "#000000" ||
		c.ColSpan != 2 || c.RowSpan != 1 ||
		c.Border != "thick" || c.BorderColor != "#333" {
		t.Fatalf("cell fields lost: %+v", c)
	}
	if area.Rows[0].Cells[1].Parameter != "ИмяПоставщика" {
		t.Fatalf("parameter cell lost: %+v", area.Rows[0].Cells[1])
	}
}

// TestLayoutBackwardCompat ensures old YAML without new fields still parses.
func TestLayoutBackwardCompat(t *testing.T) {
	src := `
name: Старый
document: Реализация
areas:
  Шапка:
    rows:
      - cells:
          - text: "Заголовок"
            bold: true
            align: center
            colspan: 3
            backColor: "#EEE"
`
	var lt LayoutTemplate
	if err := yaml.Unmarshal([]byte(src), &lt); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	c := lt.Areas["Шапка"].Rows[0].Cells[0]
	if c.Text != "Заголовок" || !c.Bold || c.Align != "center" || c.ColSpan != 3 {
		t.Fatalf("legacy fields lost: %+v", c)
	}
	// New fields are zero values, no panic on PreviewHTML.
	html := lt.PreviewHTML()
	if !strings.Contains(html, "Заголовок") {
		t.Fatalf("PreviewHTML missing text: %s", html)
	}
}

// TestPreviewHTMLNewFields ensures PreviewHTML renders new CSS for new fields.
func TestPreviewHTMLNewFields(t *testing.T) {
	lt := &LayoutTemplate{
		Name: "T",
		Columns: []LayoutColumn{
			{Width: "80px"},
			{Width: "auto"},
		},
		Areas: map[string]*LayoutArea{
			"A": {
				Rows: []LayoutRow{
					{
						Height: "30px",
						Cells: []LayoutCell{
							{
								Text:        "X",
								FontFamily:  "Times New Roman",
								VAlign:      "middle",
								Border:      "thick",
								BorderColor: "#ff0000",
								RowSpan:     2,
							},
							{Text: "Y", Border: "none"},
						},
					},
				},
			},
		},
	}
	out := lt.PreviewHTML()
	checks := []string{
		"<colgroup>",
		"width:80px",
		"font-family:Times New Roman",
		"vertical-align:middle",
		"2px solid #ff0000",
		`rowspan="2"`,
		"height:30px",
		"border:none",
	}
	for _, sub := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("PreviewHTML missing %q\nGot: %s", sub, out)
		}
	}
}

// TestLayoutAcceptsRealExample parses a representative legacy layout (areas /
// rows / cells with a parameter placeholder) and makes sure the model loads it
// cleanly and renders a preview. Inlined into a temp file so the test does not
// depend on example files under examples/ (which get reorganized).
func TestLayoutAcceptsRealExample(t *testing.T) {
	const src = `name: Накладная
document: РеализацияТоваров
columns:
  - width: 120px
  - width: auto
areas:
  Заголовок:
    rows:
      - height: 24px
        cells:
          - text: "Накладная"
            bold: true
            colspan: 4
            align: center
            fontSize: 16
          - parameter: НомерПП
`
	path := filepath.Join(t.TempDir(), "накладная.layout.yaml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	lt, err := LoadLayout(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if lt.Name != "Накладная" || lt.Document != "РеализацияТоваров" {
		t.Fatalf("header: %+v", lt)
	}
	if _, ok := lt.Areas["Заголовок"]; !ok {
		t.Fatalf("Заголовок missing")
	}
	// Preview must include known title text and a parameter placeholder.
	html := lt.PreviewHTML()
	if !strings.Contains(html, "Накладная") {
		t.Errorf("preview missing title: %s", html)
	}
	if !strings.Contains(html, "[НомерПП]") {
		t.Errorf("preview missing parameter placeholder")
	}
	// Round-trip — fields must survive marshal/unmarshal.
	data, err := yaml.Marshal(lt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got LayoutTemplate
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	titleCell := got.Areas["Заголовок"].Rows[0].Cells[0]
	if titleCell.Text != "Накладная" || !titleCell.Bold || titleCell.ColSpan != 4 ||
		titleCell.Align != "center" || titleCell.FontSize != 16 {
		t.Errorf("title cell lost fields after round-trip: %+v", titleCell)
	}
}

// TestPreviewHTMLEscapesText ensures user text/parameter cannot inject HTML.
func TestPreviewHTMLEscapesText(t *testing.T) {
	lt := &LayoutTemplate{
		Areas: map[string]*LayoutArea{
			"A": {Rows: []LayoutRow{{Cells: []LayoutCell{
				{Text: "<script>alert(1)</script>"},
				{Parameter: "<b>bad</b>"},
			}}}},
		},
	}
	out := lt.PreviewHTML()
	if strings.Contains(out, "<script>") {
		t.Errorf("unescaped script tag in preview: %s", out)
	}
	if strings.Contains(out, "<b>bad</b>") {
		t.Errorf("unescaped parameter in preview: %s", out)
	}
}
