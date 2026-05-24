package onec_forms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExport_FullRoundTrip — полный конвейер:
//   Form.xml (фикстура) → IR → нормализация-импорт → YAML на диск →
//   YAML → IR → нормализация-экспорт → Form.xml' → IR' → диффы по
//   ключевым семантическим полям.
//
// Этот тест ловит регрессии в любой из 4 точек конвертера (reader_xml /
// writer_yaml / reader_yaml / writer_xml). Он не делает побайтового
// сравнения — XML-сериализация неизбежно отличается порядком атрибутов
// и форматированием.
func TestExport_FullRoundTrip(t *testing.T) {
	xmlPath := writeFixture(t, "Form.xml", minForm)

	// Цикл 1: XML → YAML.
	form1, _, err := ReadFormXML(xmlPath)
	if err != nil {
		t.Fatalf("read xml: %v", err)
	}
	NormalizeForImport(form1)
	form1.Entity = "РеализацияТоваров"
	form1.Name = "ФормаОбъекта"
	form1.Kind = "object"

	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "form.yaml")
	if err := WriteFormYAML(form1, yamlPath); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Цикл 2: YAML → IR → нормализация-экспорт → XML'.
	form2, err := ReadFormYAML(yamlPath)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	NormalizeForExport(form2)

	xmlOut := filepath.Join(tmp, "Form.out.xml")
	if err := WriteFormXML(form2, xmlOut); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	// Цикл 3: XML' → IR''
	form3, _, err := ReadFormXML(xmlOut)
	if err != nil {
		t.Fatalf("re-read xml: %v", err)
	}

	// Семантические сравнения:
	if form3.Version != form1.Version && form3.Version != "" && form1.Version != "" {
		t.Errorf("Version: исходно %q, после round-trip %q", form1.Version, form3.Version)
	}
	if form3.AutoSaveDataInSettings != form1.AutoSaveDataInSettings {
		t.Errorf("AutoSaveDataInSettings: %v → %v", form1.AutoSaveDataInSettings, form3.AutoSaveDataInSettings)
	}
	if form3.VerticalScroll != form1.VerticalScroll {
		t.Errorf("VerticalScroll: %q → %q", form1.VerticalScroll, form3.VerticalScroll)
	}

	// События: после нормализации-экспорта имена 1С (OnOpen), после
	// re-read через reader_xml — те же 1С-имена.
	if got := form3.Events["OnOpen"]; got != "ПриОткрытии" {
		t.Errorf("OnOpen handler: %q", got)
	}
	if got := form3.Events["OnCreateAtServer"]; got != "ПриСозданииНаСервере" {
		t.Errorf("OnCreateAtServer handler: %q", got)
	}

	// Реквизиты: количество, имена, типы.
	if len(form3.Attributes) != len(form1.Attributes) {
		t.Fatalf("Attributes count: %d → %d", len(form1.Attributes), len(form3.Attributes))
	}
	for i, a1 := range form1.Attributes {
		a3 := form3.Attributes[i]
		if a3.Name != a1.Name {
			t.Errorf("Attributes[%d].Name: %q → %q", i, a1.Name, a3.Name)
		}
		// Тип после round-trip: исходно "DocumentRef.X" (OneBase-вид),
		// после re-read — снова нормализован → "DocumentRef.X".
		if a3.TypeRef != a1.TypeRef {
			t.Errorf("Attributes[%d].TypeRef: %q → %q", i, a1.TypeRef, a3.TypeRef)
		}
		if a3.OriginalID != a1.OriginalID {
			t.Errorf("Attributes[%d].OriginalID: %q → %q", i, a1.OriginalID, a3.OriginalID)
		}
	}

	// Колонки ValueTable должны сохраниться вместе с типами.
	if len(form3.Attributes[1].Columns) != 2 {
		t.Fatalf("Товары.Columns: %d (ожидалось 2)", len(form3.Attributes[1].Columns))
	}
	if form3.Attributes[1].Columns[1].TypeRef != "decimal(15,2)" {
		t.Errorf("Цена.TypeRef = %q", form3.Attributes[1].Columns[1].TypeRef)
	}

	// Команды.
	if len(form3.Commands) != 1 || form3.Commands[0].Action != "ПровестиКоманда" {
		t.Errorf("Commands после round-trip: %+v", form3.Commands)
	}

	// Дерево: Pages → Page → UsualGroup → 2 ребёнка (InputField, CheckBoxField).
	if len(form3.Elements) != 1 {
		t.Fatalf("Elements: %d", len(form3.Elements))
	}
	pages := form3.Elements[0]
	if pages.Kind != "Pages" {
		t.Errorf("корень после export = %q, ожидался Pages", pages.Kind)
	}
	if len(pages.Children) != 1 || pages.Children[0].Kind != "Page" {
		t.Fatalf("Pages.Children: %+v", pages.Children)
	}
	group := pages.Children[0].Children[0]
	if group.Kind != "UsualGroup" {
		t.Errorf("group kind = %q", group.Kind)
	}
	if len(group.Children) != 2 {
		t.Fatalf("group.Children: %d", len(group.Children))
	}
	if group.Children[0].Kind != "InputField" || group.Children[0].DataPath != "Объект.Номер" {
		t.Errorf("child[0]: %+v", group.Children[0])
	}
	if group.Children[1].Kind != "CheckBoxField" {
		t.Errorf("child[1]: %+v", group.Children[1])
	}

	// AutoCommandBar с одной кнопкой "КнПровести".
	if form3.AutoCommandBar == nil {
		t.Fatal("AutoCommandBar отсутствует")
	}
	if len(form3.AutoCommandBar.Buttons) != 1 {
		t.Fatalf("buttons: %d", len(form3.AutoCommandBar.Buttons))
	}
	btn := form3.AutoCommandBar.Buttons[0]
	if btn.Name != "КнПровести" || btn.CommandName != "Form.Command.Провести" {
		t.Errorf("button: %+v", btn)
	}

	// Локализация: ru-заголовок поля сохранился через round-trip.
	if group.Title.Get("ru") != "Шапка" {
		t.Errorf("group title: %q", group.Title.Get("ru"))
	}
	if group.Children[0].Title.Get("ru") != "Номер" {
		t.Errorf("input title: %q", group.Children[0].Title.Get("ru"))
	}
}

// TestExport_BSLDirectiveRestore — проверка восстановления &НаСервере /
// &НаКлиенте через аннотации // @directive=... из .form.os.
func TestExport_BSLDirectiveRestore(t *testing.T) {
	dsl := `// Этот файл сгенерирован конвертером onebase forms convert-from-1c.

// @directive=&НаКлиенте
Процедура НомерПриИзменении(Элемент)
	Сообщить("test");
КонецПроцедуры

// @directive=&НаСервере
Функция РассчитатьСумму(Знач Х) Экспорт
	Возврат Х * 2;
КонецФункции
`
	bsl, warns := EmitBSLFromDSL(dsl)

	must := []string{
		"&НаКлиенте\nПроцедура НомерПриИзменении",
		"&НаСервере\nФункция РассчитатьСумму",
		"КонецПроцедуры",
		"КонецФункции",
	}
	for _, s := range must {
		if !strings.Contains(bsl, s) {
			t.Errorf("BSL не содержит %q\nFull output:\n%s", s, bsl)
		}
	}
	// W042 (директива по умолчанию) не должен сработать — мы явно указали обе.
	for _, w := range warns {
		if w.Code == W042_DirectiveMissing {
			t.Errorf("неожиданный W042: %s", w)
		}
	}
}

// TestExport_BSLDefaultDirective — без аннотации @directive ставится
// &НаСервере и эмитится W042.
func TestExport_BSLDefaultDirective(t *testing.T) {
	dsl := `Процедура X()
	Возврат;
КонецПроцедуры
`
	bsl, warns := EmitBSLFromDSL(dsl)
	if !strings.Contains(bsl, "&НаСервере\nПроцедура X") {
		t.Errorf("дефолтная директива не добавлена:\n%s", bsl)
	}
	seenW042 := false
	for _, w := range warns {
		if w.Code == W042_DirectiveMissing {
			seenW042 = true
		}
	}
	if !seenW042 {
		t.Error("W042 не сгенерирован для процедуры без @directive")
	}
}

// TestExport_BSLDSLOnlyConstructs — конструкции OneBase без аналога в BSL
// дают W041 с указанием строки.
func TestExport_BSLDSLOnlyConstructs(t *testing.T) {
	dsl := `// @directive=&НаСервере
Процедура Y()
	Транзакция.Начать();
	JSON.Decode("[]");
КонецПроцедуры
`
	_, warns := EmitBSLFromDSL(dsl)
	var foundTr, foundJSON bool
	for _, w := range warns {
		if w.Code != W041_DSLNotInBSL {
			continue
		}
		if w.Field == "Транзакция.Начать" {
			foundTr = true
		}
		if w.Field == "JSON.Decode" {
			foundJSON = true
		}
	}
	if !foundTr {
		t.Error("W041 не сработал на Транзакция.Начать")
	}
	if !foundJSON {
		t.Error("W041 не сработал на JSON.Decode")
	}
}

// TestExportToOneC_Smoke — фасадный smoke-тест: написать YAML, прогнать
// ExportToOneC, проверить что Form.xml и Module.bsl созданы и корректны.
func TestExportToOneC_Smoke(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "minimal.form.yaml")
	osPath := filepath.Join(tmp, "minimal.form.os")

	// Импортируем фикстуру, чтобы получить валидный YAML/OS.
	xmlPath := writeFixture(t, "Form.xml", minForm)
	bslPath := filepath.Join(t.TempDir(), "Module.bsl")
	os.WriteFile(bslPath, []byte(`// @directive=&НаСервере
Процедура ПриСозданииНаСервере(Отмена, СтандартнаяОбработка)
	ЭтаФорма.Заголовок = "test";
КонецПроцедуры
`), 0o644)

	if _, err := ImportFromOneC(ImportOptions{
		XMLPath:     xmlPath,
		BSLPath:     bslPath,
		EntityName:  "РеализацияТоваров",
		FormName:    "ФормаОбъекта",
		FormKind:    "object",
		DstYAMLPath: yamlPath,
		DstOSPath:   osPath,
	}); err != nil {
		t.Fatalf("ImportFromOneC: %v", err)
	}

	// Теперь экспортируем обратно.
	dstDir := filepath.Join(tmp, "1c-export", "Ext")
	report, err := ExportToOneC(ExportOptions{
		YAMLPath:   yamlPath,
		OSPath:     osPath,
		DstFormDir: dstDir,
	})
	if err != nil {
		t.Fatalf("ExportToOneC: %v", err)
	}
	if report.FormDir == "" {
		t.Error("FormDir пустой")
	}

	// Проверим что файлы созданы.
	outXML := filepath.Join(dstDir, "Form.xml")
	outBSL := filepath.Join(dstDir, "Form", "Module.bsl")
	xb, err := os.ReadFile(outXML)
	if err != nil {
		t.Fatalf("Form.xml не создан: %v", err)
	}
	bb, err := os.ReadFile(outBSL)
	if err != nil {
		t.Fatalf("Module.bsl не создан: %v", err)
	}

	if !strings.Contains(string(xb), `version="2.20"`) {
		t.Error("в выходном Form.xml нет version")
	}
	if !strings.Contains(string(xb), "<InputField") {
		t.Error("в выходном Form.xml нет InputField")
	}
	if !strings.Contains(string(xb), "<Attribute name=\"Объект\"") {
		t.Error("в выходном Form.xml нет реквизита Объект")
	}
	if !strings.Contains(string(bb), "&НаСервере\nПроцедура ПриСозданииНаСервере") {
		t.Errorf("в Module.bsl нет директивы &НаСервере:\n%s", string(bb))
	}

	// Финальная сверка — XML можно перечитать через тот же reader.
	form, _, err := ReadFormXML(outXML)
	if err != nil {
		t.Fatalf("re-read exported xml: %v", err)
	}
	if form.Version != "2.20" {
		t.Errorf("re-read Version = %q", form.Version)
	}
	if len(form.Attributes) != 2 {
		t.Errorf("re-read Attributes: %d", len(form.Attributes))
	}
}
