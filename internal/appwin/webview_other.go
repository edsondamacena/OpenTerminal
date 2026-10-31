//go:build !windows

package appwin

// ConfigureWebView não faz nada fora do Windows (plataforma alvo única desta fase).
func ConfigureWebView(extra []string) {}

func fileInUse(error) bool { return false }
