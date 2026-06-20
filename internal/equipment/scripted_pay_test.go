package equipment

import "testing"

// Декларативный эквайринг: тот же результат, что у зашитого acquiring_tcp, но
// протокол (шаблон запроса, признак одобрения, разбор RRN/карты) — параметры.
// Здесь — русскоязычный терминал, чтобы показать, что признак одобрения тоже
// настраивается данными.
func TestScriptedPay_Approved(t *testing.T) {
	addr := terminalServer(t, "ОДОБРЕНО RRN=987654321098 PAN=****5678\r\n")

	dev, err := Open("scripted_pay", map[string]string{
		"порт":             addr,
		"шаблонзапроса":    "PAY {amount}",
		"признакодобрения": "ОДОБРЕНО",
		"шаблонrrn":        `RRN=(\d+)`,
		"шаблонкарты":      `PAN=(\S+)`,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Disconnect()
	term, ok := dev.(PaymentTerminal)
	if !ok {
		t.Fatal("scripted_pay не реализует PaymentTerminal")
	}
	res, err := term.Pay(150)
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if !res.Approved {
		t.Error("ожидалось одобрение (ОДОБРЕНО)")
	}
	if res.RRN != "987654321098" {
		t.Errorf("RRN = %q, ожидался 987654321098", res.RRN)
	}
	if res.Card != "****5678" {
		t.Errorf("Card = %q, ожидался ****5678", res.Card)
	}
}

func TestScriptedPay_Declined(t *testing.T) {
	addr := terminalServer(t, "DECLINED code=51\r\n")
	dev, err := Open("scripted_pay", map[string]string{"порт": addr})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Disconnect()
	res, err := dev.(PaymentTerminal).Pay(100)
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if res.Approved {
		t.Error("DECLINED не должно быть одобрено")
	}
}
