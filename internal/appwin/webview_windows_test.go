//go:build windows

package appwin

import (
	"testing"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

func TestConfigureWebViewDisablesAutofill(t *testing.T) {
	edge.DisableAutofill = false
	ConfigureWebView(nil)
	if !edge.DisableAutofill {
		t.Fatal("autofill do WebView2 permaneceu habilitado")
	}
}
