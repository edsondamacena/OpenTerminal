package appwin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDirOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "dados")
	t.Setenv("OPENTERMINAL_DATA_DIR", want)
	got, err := DataDir()
	if err != nil || got != want {
		t.Fatalf("got %q, %v", got, err)
	}
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatal("diretório não foi criado")
	}
}

func TestDataDirRejectsRelative(t *testing.T) {
	t.Setenv("OPENTERMINAL_DATA_DIR", "relativo")
	if _, err := DataDir(); err == nil {
		t.Fatal("aceitou caminho relativo")
	}
}

func TestWebViewArgsDisableCrashDumps(t *testing.T) {
	got := strings.Join(WebViewArgs([]string{"--x"}), " ")
	for _, a := range []string{"--disable-breakpad", "--disable-crash-reporter", "--x"} {
		if !strings.Contains(got, a) {
			t.Errorf("faltou %s em %q", a, got)
		}
	}
}

func TestClearWebViewAutofillData(t *testing.T) {
	dataDir := t.TempDir()
	profile := filepath.Join(WebViewDir(dataDir), "EBWebView", "Default")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Web Data", "Web Data-journal", "Web Data-wal", "Web Data-shm", "Cookies"} {
		if err := os.WriteFile(filepath.Join(profile, name), []byte("segredo"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ClearWebViewAutofillData(dataDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Web Data", "Web Data-journal", "Web Data-wal", "Web Data-shm"} {
		if _, err := os.Stat(filepath.Join(profile, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s não foi removido: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(profile, "Cookies")); err != nil {
		t.Fatalf("arquivo não relacionado foi removido: %v", err)
	}
}
