package appwin

import (
	"os"
	"path/filepath"
	"testing"
)

// Segunda instância com a primeira aberta: o Web Data travado pelo WebView2 não impede a abertura.
func TestClearWebViewAutofillDataSkipsFileInUse(t *testing.T) {
	dataDir := t.TempDir()
	profile := filepath.Join(WebViewDir(dataDir), "EBWebView", "Default")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(profile, "Web Data")
	if err := os.WriteFile(locked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := os.Open(locked)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := os.Remove(locked); !fileInUse(err) {
		t.Skipf("o sistema permitiu apagar arquivo aberto (%v); cenário não se aplica", err)
	}
	if err := ClearWebViewAutofillData(dataDir); err != nil {
		t.Fatalf("arquivo em uso deveria ser ignorado: %v", err)
	}
}
