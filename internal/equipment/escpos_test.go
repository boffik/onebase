package equipment

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// captureServer поднимает TCP-листенер (эмулятор сетевого принтера) и отдаёт
// его адрес и канал, в который попадут принятые байты — железо не требуется.
func captureServer(t *testing.T) (string, <-chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan []byte, 1)
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			out <- nil
			return
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(time.Second))
		data, _ := io.ReadAll(conn)
		out <- data
	}()
	return ln.Addr().String(), out
}

func TestESCPOS_PrintReceipt(t *testing.T) {
	addr, received := captureServer(t)

	dev, err := Open("escpos_tcp", map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	printer, ok := dev.(ReceiptPrinter)
	if !ok {
		t.Fatal("устройство не реализует ReceiptPrinter")
	}

	r := Receipt{
		Header:  []string{"ООО Ромашка"},
		Items:   []ReceiptItem{{Name: "Хлеб", Qty: 2, Price: 30, Sum: 60}},
		Total:   60,
		Payment: "Наличные",
		Footer:  []string{"Спасибо за покупку"},
	}
	if err := printer.PrintReceipt(r); err != nil {
		t.Fatalf("PrintReceipt: %v", err)
	}
	dev.Disconnect() // закрываем — сервер получит EOF и вернёт накопленное

	got := <-received
	if !bytes.HasPrefix(got, escInit) {
		t.Error("поток не начинается с ESC @ (инициализация)")
	}
	if !bytes.Contains(got, escCutFull) {
		t.Error("нет команды реза бумаги (GS V 0)")
	}
	for _, want := range []string{"ООО Ромашка", "Хлеб", "ИТОГО:", "Наличные", "Спасибо за покупку"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("в чеке отсутствует %q", want)
		}
	}
}

func TestESCPOS_OpenDrawer(t *testing.T) {
	addr, received := captureServer(t)
	dev, err := Open("escpos_tcp", map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	dev.(ReceiptPrinter).OpenDrawer()
	dev.Disconnect()

	if got := <-received; !bytes.Equal(got, escDrawer) {
		t.Errorf("импульс ящика = % x, ожидался % x", got, escDrawer)
	}
}

func TestDrivers_RegistersESCPOS(t *testing.T) {
	for _, d := range Drivers() {
		if d == "escpos_tcp" {
			return
		}
	}
	t.Errorf("драйвер escpos_tcp не зарегистрирован, доступны: %v", Drivers())
}

func TestOpen_UnknownDriver(t *testing.T) {
	if _, err := Open("нет_такого", nil); err == nil {
		t.Error("ожидалась ошибка для неизвестного драйвера")
	}
}

func TestConnect_MissingPort(t *testing.T) {
	if _, err := Open("escpos_tcp", nil); err == nil {
		t.Error("ожидалась ошибка при отсутствии параметра Порт")
	}
}
