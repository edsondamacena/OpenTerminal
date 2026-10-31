package appwin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDir(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, legacyDirName)
	for name, body := range map[string]string{
		legacyDBName:               "db",
		legacyDBName + "-wal":      "wal",
		"settings.json":            "{}",
		"webview/cache/blob":       "x",
		"profiles/sub/arquivo.txt": "p",
	} {
		p := filepath.Join(old, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(base, "OpenTerminal")
	if err := migrateLegacyDir(base, dir); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		dbName:                     "db",
		dbName + "-wal":            "wal",
		"settings.json":            "{}",
		"profiles/sub/arquivo.txt": "p",
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "webview")); !os.IsNotExist(err) {
		t.Fatal("cache do WebView não devia ser copiado")
	}
	if _, err := os.Stat(filepath.Join(old, legacyDBName)); err != nil {
		t.Fatal("pasta antiga foi alterada")
	}
	// Segunda execução: dir já existe, nada muda.
	if err := os.WriteFile(filepath.Join(dir, dbName), []byte("novo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyDir(base, dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, dbName)); string(got) != "novo" {
		t.Fatal("migração rodou de novo")
	}
}
