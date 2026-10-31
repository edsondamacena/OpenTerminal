package tabby

import (
	"os"
	"testing"
)

// Conta o que o import traria do Tabby desta máquina, sem mostrar nenhum valor.
// OPENTERMINAL_TABBY_SMOKE=1 go test ./internal/tabby -run Smoke -v
func TestSmokeRealConfig(t *testing.T) {
	if os.Getenv("OPENTERMINAL_TABBY_SMOKE") != "1" {
		t.Skip("só sob demanda")
	}
	path, _ := DefaultPath()
	lookup, done := KeyringLookup()
	defer done()
	entries, err := Read(path, lookup)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, e := range entries {
		count[e.Protocol]++
		if e.Password != "" {
			count[e.Protocol+" com senha"]++
		}
		if e.Username == "" {
			count[e.Protocol+" sem usuário"]++
		}
		if e.Legacy {
			count["ssh legado"]++
		}
		if len(e.Group) > 0 {
			count["em grupo"]++
		}
	}
	t.Logf("%d acessos: %v", len(entries), count)
}
