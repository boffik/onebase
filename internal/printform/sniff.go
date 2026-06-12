package printform

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// sniff.go — определение формата печатной формы по содержимому (план 64, этап 4).
// Внешние формы (_ext_printforms) и бандлы могут нести либо макет v2
// (LayoutTemplate с top-level areas:), либо устаревший плоский YAML (PrintForm с
// title/header/table/footer). IsLayoutV2 различает их, чтобы загрузчик выбрал
// правильный парсер.

// IsLayoutV2 сообщает, что content — макет v2: YAML-mapping верхнего уровня с
// ключом areas. Устаревший плоский формат (title/header/table/footer) ключа
// areas не содержит. Битый/пустой YAML трактуется как НЕ v2 (откат на legacy-
// парсер, который вернёт понятную ошибку).
func IsLayoutV2(content []byte) bool {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return false
	}
	doc := &root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if strings.EqualFold(doc.Content[i].Value, "areas") {
			return true
		}
	}
	return false
}
