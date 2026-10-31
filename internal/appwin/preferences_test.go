package appwin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateShortcuts(t *testing.T) {
	t.Parallel()
	valid := []map[string]string{
		{},
		{"closeTab": "Ctrl+Shift+W", "palette": ""},
		{"newTab": "Ctrl+Alt+T", "nextTab": "Ctrl+Tab", "search": "F5"},
		{"zoomIn": "Alt+Equal", "zoomOut": "Ctrl+Minus", "zoomReset": "Ctrl+Digit0"},
	}
	for _, shortcuts := range valid {
		if err := ValidateShortcuts(shortcuts); err != nil {
			t.Errorf("atalhos válidos %v: %v", shortcuts, err)
		}
	}

	invalid := []struct {
		name      string
		shortcuts map[string]string
		want      string
	}{
		{"ação desconhecida", map[string]string{"deploy": "Ctrl+D"}, "desconhecida"},
		{"sem modificador", map[string]string{"newTab": "T"}, "Ctrl ou Alt"},
		{"ordem", map[string]string{"newTab": "Shift+Ctrl+T"}, "ordem"},
		{"prefixo Key", map[string]string{"newTab": "Ctrl+KeyT"}, "código"},
		{"duplicata", map[string]string{"newTab": "Ctrl+T", "closeTab": "Ctrl+T"}, "duplicada"},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateShortcuts(test.shortcuts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("erro = %v, esperado motivo %q", err, test.want)
			}
		})
	}
}

func TestNormalizedPreferencesBell(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.Bell = "beep"
	got := normalizedPreferences(preferences)
	if got.Bell != "visual" {
		t.Fatalf("sino normalizado = %q", got.Bell)
	}
	for _, bell := range []string{"off", "sound", "visual"} {
		preferences.Bell = bell
		if got := normalizedPreferences(preferences); got.Bell != bell {
			t.Errorf("sino %q virou %q", bell, got.Bell)
		}
	}
}

func TestPreferencesWithoutTerminalOptionsKeepDefaults(t *testing.T) {
	preferences := DefaultPreferences()
	if err := json.Unmarshal([]byte(`{"theme":"dark"}`), &preferences); err != nil {
		t.Fatal(err)
	}
	preferences = normalizedPreferences(preferences)
	if preferences.CopyOnSelect || !preferences.RightClickPaste || preferences.Bell != "visual" || !preferences.BoldBright || preferences.AutoReconnect {
		t.Fatalf("preferências migradas = %+v", preferences)
	}
}
