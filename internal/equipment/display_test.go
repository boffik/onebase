package equipment

import (
	"bytes"
	"testing"
)

func TestDisplay_ShowLines(t *testing.T) {
	addr, received := captureServer(t)

	dev, err := Open("display_tcp", map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	disp, ok := dev.(CustomerDisplay)
	if !ok {
		t.Fatal("устройство не реализует CustomerDisplay")
	}
	if err := disp.ShowLines([]string{"Молоко 3 шт", "ИТОГО: 150"}); err != nil {
		t.Fatalf("ShowLines: %v", err)
	}
	dev.Disconnect()

	got := <-received
	if !bytes.HasPrefix(got, dispInit) {
		t.Error("поток не начинается с ESC @ (инициализация)")
	}
	if !bytes.Contains(got, dispClear) {
		t.Error("нет команды очистки CLR")
	}
	if !bytes.Contains(got, dispUpper) || !bytes.Contains(got, dispLower) {
		t.Error("нет команд верхней/нижней строки CD5220")
	}
	for _, want := range []string{"Молоко 3 шт", "ИТОГО: 150"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("на дисплее нет %q", want)
		}
	}
}

// Дисплей и принтер — разные типы устройств: дисплей не должен приводиться к
// ReceiptPrinter (проверка изоляции категорий оборудования).
func TestDisplay_IsNotPrinter(t *testing.T) {
	addr, _ := captureServer(t)
	dev, err := Open("display_tcp", map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Disconnect()
	if _, ok := dev.(ReceiptPrinter); ok {
		t.Error("дисплей покупателя не должен быть ReceiptPrinter")
	}
}
