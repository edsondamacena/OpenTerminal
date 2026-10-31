package fonts

import "testing"

func TestListIncludesMonospaceAndBundledFonts(t *testing.T) {
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	byFamily := make(map[string]Font, len(items))
	for _, item := range items {
		byFamily[item.Family] = item
	}
	if got, ok := byFamily["Consolas"]; !ok || got.Source != SourceSystem {
		t.Fatalf("Consolas não encontrada como fonte do sistema: %+v", got)
	}
	if _, ok := byFamily["Arial"]; ok {
		t.Fatal("Arial não é monoespaçada e não deve aparecer")
	}
	for _, family := range []string{"JetBrains Mono", "Cascadia Code"} {
		got, ok := byFamily[family]
		if !ok || got.Source != SourceBundled || !got.Ligatures {
			t.Errorf("fonte embutida ausente ou incorreta: %s %+v", family, got)
		}
	}
}

func TestListIsCachedButReturnsIndependentSlice(t *testing.T) {
	first, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("lista de fontes vazia")
	}
	want := first[0]
	first[0].Family = "alterada"
	second, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if second[0] != want {
		t.Fatalf("cache exposto para mutação: got %+v want %+v", second[0], want)
	}
}
