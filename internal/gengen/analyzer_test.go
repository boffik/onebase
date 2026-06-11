package gengen

import "testing"

// ─── Trade domain ─────────────────────────────────────────────────────────────

func TestAnalyze_Trade(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"хочу склад и продажи"},
		{"система оптовых продаж"},
		{"клиенты, контрагенты, отгрузки"},
		{"нужна реализация товаров со склада"},
		{"учёт продаж и закупок"},
		{"оптовая торговля с контрагентами"},
		{"розничные продажи номенклатуры"},
		{"заказ поставщика и отгрузка товара"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "trade" {
				t.Errorf("Analyze(%q) = %q, want trade", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── Warehouse domain ─────────────────────────────────────────────────────────

func TestAnalyze_Warehouse(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"складской учёт без продаж"},
		{"адресное хранение товаров"},
		{"инвентаризация склада"},
		{"приход и расход товаров"},
		{"перемещение между ячейками"},
		{"учёт остатков на складах"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "warehouse" {
				t.Errorf("Analyze(%q) = %q, want warehouse", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── CRM domain ───────────────────────────────────────────────────────────────

func TestAnalyze_CRM(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"управление клиентами и сделками"},
		{"воронка продаж с лидами"},
		{"менеджер звонит контактам"},
		{"коммерческие предложения клиентам"},
		{"crm система для отдела продаж"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "crm" {
				t.Errorf("Analyze(%q) = %q, want crm", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── Finance domain ───────────────────────────────────────────────────────────

func TestAnalyze_Finance(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"домашние финансы и бюджет"},
		{"учёт доходов и расходов"},
		{"категории расходов"},
		{"долги и прибыль"},
		{"финансовый учёт по счетам"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "finance" {
				t.Errorf("Analyze(%q) = %q, want finance", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── Tasks domain ─────────────────────────────────────────────────────────────

func TestAnalyze_Tasks(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"трекер задач и проектов"},
		{"исполнители и дедлайны"},
		{"приоритет и статус задачи"},
		{"управление проектами"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "tasks" {
				t.Errorf("Analyze(%q) = %q, want tasks", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── Accounting domain ────────────────────────────────────────────────────────

func TestAnalyze_Accounting(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"бухгалтерия с планом счетов"},
		{"проводки дебет кредит"},
		{"оборотная ведомость и сальдо"},
		{"учёт основных средств и амортизация"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "accounting" {
				t.Errorf("Analyze(%q) = %q, want accounting", tt.prompt, r.Domain)
			}
		})
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

func TestAnalyze_Unknown(t *testing.T) {
	r := Analyze("абракадабра ничего не понятно")
	if r.Domain != "unknown" {
		t.Errorf("Analyze(unknown) = %q, want unknown", r.Domain)
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze("")
	if r.Domain != "unknown" {
		t.Errorf("Analyze(empty) = %q, want unknown", r.Domain)
	}
}

func TestAnalyze_Confidence(t *testing.T) {
	// Clear single match → confident
	r := Analyze("хочу склад и продажи товаров")
	if !r.Confident {
		t.Error("expected Confident=true for clear trade match")
	}
	if len(r.Ambiguous) != 0 {
		t.Errorf("expected no Ambiguous, got %v", r.Ambiguous)
	}
}

func TestAnalyze_AvailableDomains(t *testing.T) {
	domains := AvailableDomains()
	expected := []string{"trade", "warehouse", "crm", "finance", "tasks", "accounting", "texts"}
	for _, d := range expected {
		if _, ok := domains[d]; !ok {
			t.Errorf("AvailableDomains() missing %q", d)
		}
	}
}

// ─── Texts domain ─────────────────────────────────────────────────────────────

func TestAnalyze_Texts(t *testing.T) {
	tests := []struct {
		prompt string
	}{
		{"тексты и переводы"},
		{"учёт текстов и переводов"},
		{"текст содержит ссылку на событие, перевод содержит язык"},
		{"перевод текста на разные языки"},
		{"хранение текстов событий и их переводов"},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			r := Analyze(tt.prompt)
			if r.Domain != "texts" {
				t.Errorf("Analyze(%q) = %q, want texts", tt.prompt, r.Domain)
			}
		})
	}
}
