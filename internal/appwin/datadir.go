// Package appwin reúne o que é da janela e do processo: diretório de dados, argumentos do
// WebView2, aparência e confirmação de fechamento.
package appwin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DataDir devolve o diretório de dados do app, criando-o se preciso.
// OPENTERMINAL_DATA_DIR sobrescreve o padrão %APPDATA%\OpenTerminal (usado pelos testes).
func DataDir() (string, error) {
	dir := os.Getenv("OPENTERMINAL_DATA_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "OpenTerminal")
		if err := migrateLegacyDir(base, dir); err != nil {
			return "", fmt.Errorf("migrar dados do OpenHost: %w", err)
		}
	}
	if !filepath.IsAbs(dir) {
		return "", errors.New("diretório de dados precisa ser absoluto")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// WebViewDir é onde o WebView2 guarda o perfil (cache, storage), dentro do diretório de dados.
func WebViewDir(dataDir string) string {
	return filepath.Join(dataDir, "webview")
}

// ClearWebViewAutofillData remove dados que versões anteriores do WebView2 possam ter salvo em
// claro. Deve rodar antes de criar o WebView; falhar fechado evita abrir o app mantendo segredos.
// Arquivo em uso significa outra instância aberta, que já limpou ao iniciar: a trava de instância
// única do Wails vai só trazer aquela janela para a frente.
func ClearWebViewAutofillData(dataDir string) error {
	profile := filepath.Join(WebViewDir(dataDir), "EBWebView", "Default")
	for _, name := range []string{"Web Data", "Web Data-journal", "Web Data-wal", "Web Data-shm"} {
		err := os.Remove(filepath.Join(profile, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) && !fileInUse(err) {
			return err
		}
	}
	return nil
}

// baseWebViewArgs desliga o envio e a gravação de crash dumps do WebView2 (FR-030, R17).
var baseWebViewArgs = []string{"--disable-breakpad", "--disable-crash-reporter"}

// WebViewArgs são os argumentos do processo do navegador: os fixos mais os extras do build de
// testes (tag e2e).
func WebViewArgs(extra []string) []string {
	return append(append([]string{}, baseWebViewArgs...), extra...)
}
