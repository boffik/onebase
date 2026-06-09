package langref

import "testing"

func TestByName_CaseInsensitiveAndAlias(t *testing.T) {
	saved := functionDescriptors
	defer func() { functionDescriptors = saved }()
	functionDescriptors = []Descriptor{{
		Name: "сообщить", Display: "Сообщить", Aliases: []string{"Message"},
		Kind: KindFunc, Signature: "Сообщить(Текст)", Doc: "Выводит текст.",
	}}
	if _, ok := ByName("СООБЩИТЬ"); !ok {
		t.Error("ByName должен находить по имени регистронезависимо")
	}
	if _, ok := ByName("message"); !ok {
		t.Error("ByName должен находить по англоязычному алиасу")
	}
	if _, ok := ByName("неттакого"); ok {
		t.Error("ByName не должен находить несуществующее имя")
	}
}

func TestObjectsAndGroups_UniqueSorted(t *testing.T) {
	savedF, savedM := functionDescriptors, methodDescriptors
	defer func() { functionDescriptors, methodDescriptors = savedF, savedM }()
	functionDescriptors = []Descriptor{
		{Name: "b", Display: "B", Kind: KindFunc, Group: "Строки", Signature: "B()", Doc: "d"},
		{Name: "a", Display: "A", Kind: KindFunc, Group: "Даты", Signature: "A()", Doc: "d"},
	}
	methodDescriptors = []Descriptor{
		{Name: "добавить", Display: "Добавить", Kind: KindMethod, Object: "Массив", Signature: "Массив.Добавить(З)", Doc: "d"},
	}
	g := Groups()
	if len(g) != 2 || g[0] != "Даты" || g[1] != "Строки" {
		t.Errorf("Groups должен быть уникален и отсортирован, got %v", g)
	}
	o := Objects()
	if len(o) != 1 || o[0] != "Массив" {
		t.Errorf("Objects: got %v", o)
	}
}
