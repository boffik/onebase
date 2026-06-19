package equipment

import "testing"

func TestIsSerialAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:9100":     false,
		"localhost:9100":     false,
		"COM3":               true,
		"/dev/ttyUSB0":       true,
		"/dev/tty.usbserial": true,
	}
	for addr, want := range cases {
		if got := isSerialAddr(addr); got != want {
			t.Errorf("isSerialAddr(%q) = %v, ожидалось %v", addr, got, want)
		}
	}
}

func TestOpenWriteTransport_TCP(t *testing.T) {
	addr, _ := captureServer(t)
	wc, err := openWriteTransport(map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("openWriteTransport TCP: %v", err)
	}
	wc.Close()
}

// serial-путь выбирается по адресу без двоеточия; несуществующий порт даёт ошибку
// (полная проверка реальной печати по COM требует железа).
func TestOpenWriteTransport_SerialError(t *testing.T) {
	if _, err := openWriteTransport(map[string]string{"порт": "/dev/onebase_nonexistent_tty"}); err == nil {
		t.Error("ожидалась ошибка открытия несуществующего serial-порта")
	}
}

func TestOpenWriteTransport_NoPort(t *testing.T) {
	if _, err := openWriteTransport(map[string]string{}); err == nil {
		t.Error("ожидалась ошибка при отсутствии параметра Порт")
	}
}

func TestNeutralDriverNames(t *testing.T) {
	for _, name := range []string{"escpos", "display"} {
		found := false
		for _, d := range Drivers() {
			if d == name {
				found = true
			}
		}
		if !found {
			t.Errorf("драйвер %q не зарегистрирован: %v", name, Drivers())
		}
	}
}

// Запрос-ответ драйвер (весы) теперь тоже выбирает serial по адресу без двоеточия.
func TestRWTransport_SerialError(t *testing.T) {
	if _, err := Open("scale_tcp", map[string]string{"порт": "/dev/onebase_nonexistent_tty"}); err == nil {
		t.Error("ожидалась ошибка serial для весов на несуществующем порту")
	}
}

func TestRWTransport_NoPort(t *testing.T) {
	if _, err := Open("scanner_tcp", map[string]string{}); err == nil {
		t.Error("ожидалась ошибка при отсутствии параметра Порт")
	}
}
