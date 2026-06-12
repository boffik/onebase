package printform

import "testing"

func TestIsLayoutV2(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantV2  bool
	}{
		{
			name: "v2 с top-level areas",
			content: `name: Накладная
document: Реализация
areas:
  - name: Заголовок
    rows: []
`,
			wantV2: true,
		},
		{
			name: "legacy с table/title",
			content: `name: Накладная
document: Реализация
title: "Накладная № {{Номер}}"
table:
  source: Товары
  columns: []
`,
			wantV2: false,
		},
		{
			name: "legacy mapping-areas (НЕ top-level v2-маркер) всё равно areas → v2",
			content: `name: X
areas:
  Заголовок:
    rows: []
`,
			wantV2: true,
		},
		{
			name:    "пустой",
			content: ``,
			wantV2:  false,
		},
		{
			name: "areas как значение строки, не ключ — не v2",
			content: `name: X
title: "areas: тут просто текст"
`,
			wantV2: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLayoutV2([]byte(tc.content))
			if got != tc.wantV2 {
				t.Errorf("IsLayoutV2 = %v, ожидалось %v", got, tc.wantV2)
			}
		})
	}
}
