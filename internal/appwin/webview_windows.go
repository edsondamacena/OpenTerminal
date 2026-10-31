package appwin

import (
	"errors"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// ConfigureWebView entrega os argumentos ao WebView2 antes de wails.Run. Usa o patch local do
// go-webview2 (third_party/go-webview2/PATCH.md): o loader ignora a variável de ambiente.
func ConfigureWebView(extra []string) {
	edge.ExtraBrowserArgs = WebViewArgs(extra)
	edge.DisableAutofill = true
}

// fileInUse diz se o arquivo está aberto por outro processo (o WebView2 de uma instância já
// aberta do app).
func fileInUse(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
