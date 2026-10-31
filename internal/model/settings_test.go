package model

import "testing"

func TestNormalizeVaultSettingsFillsFieldsMissingFromOlderVault(t *testing.T) {
	settings := NormalizeVaultSettings(VaultSettings{})
	if settings.ConnectTimeoutSeconds != 15 || settings.KeepaliveSeconds != 30 ||
		settings.LegacyDefault || settings.Term != DefaultTerm {
		t.Fatalf("configurações normalizadas = %+v", settings)
	}
	for _, term := range []string{"xterm-256color", "xterm", "vt100", "linux"} {
		if !ValidTerm(term) {
			t.Errorf("TERM válido rejeitado: %s", term)
		}
	}
	if ValidTerm("screen") {
		t.Fatal("TERM fora do contrato foi aceito")
	}
}
