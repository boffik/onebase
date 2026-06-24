package cli

import (
	"sort"
	"strings"
	"testing"
)

func TestExampleSnippet_Aliases(t *testing.T) {
	got, ok := exampleSnippet(" справочник ")
	if !ok {
		t.Fatal("справочник alias not found")
	}
	if !strings.Contains(got, "name: Контрагент") || !strings.Contains(got, "fields:") {
		t.Fatalf("catalog example looks wrong:\n%s", got)
	}

	got, ok = exampleSnippet("QUERY")
	if !ok {
		t.Fatal("QUERY alias not found")
	}
	if !strings.Contains(got, "РегистрНакопления.ОстаткиТоваров.Остатки") {
		t.Fatalf("query example does not show accumulation-register virtual table:\n%s", got)
	}

	got, ok = exampleSnippet("роль")
	if !ok {
		t.Fatal("роль alias not found")
	}
	if !strings.Contains(got, "permissions:") || !strings.Contains(got, "processors:") {
		t.Fatalf("role example does not match roles/*.yaml format:\n%s", got)
	}
}

func TestExampleKindsSorted(t *testing.T) {
	kinds := exampleKinds()
	if len(kinds) == 0 {
		t.Fatal("empty example kind list")
	}
	if !sort.StringsAreSorted(kinds) {
		t.Fatalf("example kinds are not sorted: %v", kinds)
	}
	for _, want := range []string{"catalog", "document", "query", "service"} {
		var found bool
		for _, got := range kinds {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("example kind %q not listed in %v", want, kinds)
		}
	}
}
