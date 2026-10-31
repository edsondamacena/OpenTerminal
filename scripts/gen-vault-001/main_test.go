package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"openterminal/internal/vault"
	"openterminal/internal/vault/migrate001"
)

func TestRunGeneratesMigratableVault(t *testing.T) {
	source := filepath.Join("..", "..", "tests", "fixtures", "vault-001")
	output := t.TempDir()
	if err := run(source, output); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(source, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected fixture
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	store, err := vault.Open(output, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if state := store.Status().State; state != vault.StateMigratable {
		t.Fatalf("estado antes da migração = %s", state)
	}
	if err := migrate001.Migrate(store, expected.Password); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(content.Hosts) != 4 || len(content.Groups) != 2 || len(content.Credentials) != 3 {
		t.Fatalf("conteúdo migrado inesperado: %d hosts, %d grupos, %d credenciais",
			len(content.Hosts), len(content.Groups), len(content.Credentials))
	}
	if _, err := os.Stat(filepath.Join(output, "vault.svlt.migrated")); err != nil {
		t.Fatal(err)
	}
}
