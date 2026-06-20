package equipment

import "testing"

// Декларативный драйвер: тот же результат, что у зашитого scale_tcp, но протокол
// (запрос ENQ, шаблон разбора, перевод граммы→кг) задан ПАРАМЕТРАМИ, не Go-кодом.
func TestScripted_Weight(t *testing.T) {
	addr := scaleServer(t, "ST,GS,+001250 g\r\n") // ответ весов в граммах

	dev, err := Open("scripted", map[string]string{
		"порт":       addr,
		"запрос_hex": "05", // ENQ
		"шаблон":     `[-+]?[0-9]+(?:[.,][0-9]+)?`,
		"множитель":  "0.001", // граммы → кг
		"тип":        "весы",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Disconnect()

	if dev.Kind() != "весы" {
		t.Errorf("Kind = %q, ожидался «весы» (из параметра)", dev.Kind())
	}
	scale, ok := dev.(Scale)
	if !ok {
		t.Fatal("scripted не реализует Scale")
	}
	w, err := scale.Weight()
	if err != nil {
		t.Fatalf("Weight: %v", err)
	}
	if w != 1.25 {
		t.Errorf("вес = %v, ожидался 1.25 (1250 г × 0.001)", w)
	}
}

func TestScripted_BadHex(t *testing.T) {
	if _, err := Open("scripted", map[string]string{"порт": "127.0.0.1:9100", "запрос_hex": "zz"}); err == nil {
		t.Error("ожидалась ошибка неверного Запрос_hex")
	}
}
