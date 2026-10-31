//go:build e2e

package main

import (
	"os"

	"openterminal/internal/bridge"
	"openterminal/internal/bridge/testsvc"
)

// Build de testes: abre o DevTools Protocol do WebView2 para o Playwright (connectOverCDP) e
// liga o TestService. Nada disto existe no binário de release (scripts/check-release.mjs).
func init() {
	port := os.Getenv("OPENTERMINAL_E2E_CDP_PORT")
	if port == "" {
		port = "9333"
	}
	extraWebViewArgs = append(extraWebViewArgs, "--remote-debugging-port="+port)
	if scale := os.Getenv("OPENTERMINAL_E2E_SCALE"); scale != "" {
		extraWebViewArgs = append(extraWebViewArgs, "--force-device-scale-factor="+scale)
	}
	extraBindings = func(s *bridge.Services) []any {
		sessions := func() testsvc.Sessions {
			if st := s.TestSessions(); st != nil {
				return st
			}
			return nil
		}
		return []any{testsvc.New(s.Context, sessions)}
	}
}
