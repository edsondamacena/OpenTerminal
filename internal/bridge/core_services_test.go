package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/localpty"
	"openterminal/internal/model"
	"openterminal/internal/vault"
)

func testServices(t *testing.T) *Services {
	t.Helper()
	services, err := newServices(t.TempDir(), vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { services.Shutdown(nil) })
	return services
}

func TestAppServicePreferencesAndExternalURL(t *testing.T) {
	services := testServices(t)
	app := services.appService

	preferences, err := app.GetPreferences()
	if err != nil || preferences.DefaultShell != "powershell" || !preferences.GlassEnabled || preferences.SidebarCollapsed ||
		preferences.CopyOnSelect || !preferences.RightClickPaste || preferences.Bell != "visual" || !preferences.BoldBright || preferences.AutoReconnect {
		t.Fatalf("preferências %+v, %v", preferences, err)
	}
	groupID := "4ce66518-3068-4bef-ad0b-eb3f0141443d"
	updated, err := app.UpdatePreferences(PreferencesPatch{
		"theme":            "light",
		"glassEnabled":     false,
		"sidebarCollapsed": true,
		"accent":           "brand-violet",
		"defaultShell":     "cmd",
		"collapsedGroups":  []any{groupID},
		"shortcuts":        map[string]any{"closeTab": "Ctrl+Alt+W", "palette": ""},
		"copyOnSelect":     true,
		"rightClickPaste":  false,
		"bell":             "sound",
		"boldBright":       false,
		"autoReconnect":    true,
	})
	if err != nil || updated.Theme != "light" || updated.GlassEnabled || !updated.SidebarCollapsed || updated.Accent != "brand-violet" || updated.DefaultShell != "cmd" || len(updated.CollapsedGroups) != 1 || updated.Shortcuts["closeTab"] != "Ctrl+Alt+W" ||
		!updated.CopyOnSelect || updated.RightClickPaste || updated.Bell != "sound" || updated.BoldBright || !updated.AutoReconnect {
		t.Fatalf("preferências atualizadas %+v, %v", updated, err)
	}
	persisted, err := app.GetPreferences()
	if err != nil || !persisted.SidebarCollapsed {
		t.Fatalf("sidebarCollapsed não persistiu: %+v, %v", persisted, err)
	}
	if len(persisted.Shortcuts) != 2 || persisted.Shortcuts["palette"] != "" {
		t.Fatalf("shortcuts não persistiram: %+v", persisted.Shortcuts)
	}
	replaced, err := app.UpdatePreferences(PreferencesPatch{"shortcuts": map[string]string{"search": "F5"}})
	if err != nil || len(replaced.Shortcuts) != 1 || replaced.Shortcuts["search"] != "F5" {
		t.Fatalf("substituição de shortcuts = %+v, %v", replaced.Shortcuts, err)
	}
	_, err = app.UpdatePreferences(PreferencesPatch{
		"shortcuts": map[string]any{"newTab": "Ctrl+T", "closeTab": "Ctrl+T"},
	})
	var shortcutError *apperr.Error
	if !errors.As(err, &shortcutError) || shortcutError.Code != apperr.InvalidInput || !strings.Contains(shortcutError.Detail, "duplicada") {
		t.Fatalf("erro de shortcut = %+v", err)
	}
	wantBridgeCode(t, func() error {
		_, err := app.UpdatePreferences(PreferencesPatch{"defaultShell": "bash"})
		return err
	}(), apperr.InvalidInput)
	wantBridgeCode(t, func() error {
		_, err := app.UpdatePreferences(PreferencesPatch{"bell": "beep"})
		return err
	}(), apperr.InvalidInput)
	wantBridgeCode(t, func() error {
		_, err := app.UpdatePreferences(PreferencesPatch{"copyOnSelect": "sim"})
		return err
	}(), apperr.InvalidInput)

	var opened string
	app.openURL = func(_ context.Context, url string) { opened = url }
	if err := app.OpenExternalUrl("https://example.com/docs"); err != nil || opened == "" {
		t.Fatalf("url %q, %v", opened, err)
	}
	wantBridgeCode(t, app.OpenExternalUrl("file:///C:/segredo"), apperr.InvalidInput)
}

func TestAppServiceLocalShellsAndMissingDefaultFallback(t *testing.T) {
	services := testServices(t)
	app := services.appService
	app.localShells = func() []localpty.LocalShell {
		return []localpty.LocalShell{
			{ID: "powershell", Label: "Windows PowerShell", Kind: localpty.KindPowerShell},
			{ID: "gitbash", Label: "Git Bash", Kind: localpty.KindGitBash},
			{ID: "wsl:Ubuntu", Label: "Ubuntu", Kind: localpty.KindWSL},
		}
	}

	shells, err := app.ListLocalShells()
	if err != nil || len(shells) != 3 || shells[1].Kind != localpty.KindGitBash {
		t.Fatalf("shells = %+v, %v", shells, err)
	}
	updated, err := app.UpdatePreferences(PreferencesPatch{"defaultShell": "wsl:Ubuntu"})
	if err != nil || updated.DefaultShell != "wsl:Ubuntu" {
		t.Fatalf("preferência = %+v, %v", updated, err)
	}
	wantBridgeCode(t, func() error {
		_, err := app.UpdatePreferences(PreferencesPatch{"defaultShell": "pwsh"})
		return err
	}(), apperr.InvalidInput)

	app.localShells = func() []localpty.LocalShell {
		return []localpty.LocalShell{{ID: "powershell", Label: "Windows PowerShell", Kind: localpty.KindPowerShell}}
	}
	got, err := app.GetPreferences()
	if err != nil || got.DefaultShell != "powershell" {
		t.Fatalf("fallback = %+v, %v", got, err)
	}
	persisted, err := app.preferences.Get()
	if err != nil || persisted.DefaultShell != "powershell" {
		t.Fatalf("fallback persistido = %+v, %v", persisted, err)
	}
}

func wantBridgeCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if got := apperr.CodeOf(err); got != code {
		t.Fatalf("erro %v (%s), esperado %s", err, got, code)
	}
	if _, ok := err.(*apperr.Error); err != nil && !ok {
		t.Fatalf("erro não sanitizado: %T", err)
	}
}

func TestCoreServicesVaultAndDomain(t *testing.T) {
	services := testServices(t)
	vaultService := services.vaultService
	hostService := services.hostService
	credentialService := services.credentialService

	status, err := vaultService.GetStatus()
	if err != nil || status.State != vault.StateNonexistent {
		t.Fatalf("status %+v, %v", status, err)
	}
	if err := vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	autoLock := 7
	settings, err := vaultService.UpdateSettings(VaultSettingsPatch{
		"autoLockMinutes":       autoLock,
		"connectTimeoutSeconds": 20,
		"keepaliveSeconds":      45,
	})
	if err != nil || settings.AutoLockMinutes == nil || *settings.AutoLockMinutes != 7 || settings.ConnectTimeoutSeconds != 20 || settings.KeepaliveSeconds != 45 {
		t.Fatalf("settings %+v, %v", settings, err)
	}

	credential, err := credentialService.CreatePassword("Senha NOC", "segredo")
	if err != nil {
		t.Fatal(err)
	}
	group, err := hostService.CreateGroup("Datacenter", nil)
	if err != nil {
		t.Fatal(err)
	}
	host, err := hostService.Create(domain.HostInput{
		Label: "Roteador", Address: "127.0.0.1", Port: 22, Username: "noc",
		AuthMethod: model.AuthPassword, CredentialID: &credential.ID, GroupID: &group.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if host.CredentialName == nil || *host.CredentialName != "Senha NOC" {
		t.Fatalf("host %+v", host)
	}
	listed, err := hostService.List()
	if err != nil || len(listed) != 1 || listed[0].ID != host.ID {
		t.Fatalf("hosts %+v, %v", listed, err)
	}

	wantBridgeCode(t, credentialService.Remove(credential.ID), apperr.InvalidInput)
	wantBridgeCode(t, hostService.Remove("id-inválido"), apperr.InvalidInput)
	if err := hostService.Remove(host.ID); err != nil {
		t.Fatal(err)
	}
	if err := credentialService.Remove(credential.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSessionServiceLocalLifecycle(t *testing.T) {
	services := testServices(t)
	sessions := services.sessionService

	opened, err := sessions.OpenLocal("cmd")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := sessions.ActiveSummary()
	if err != nil || summary.Sessions != 1 || summary.Transfers != 0 || summary.Forwards != 0 {
		t.Fatalf("resumo %+v, %v", summary, err)
	}
	if err := sessions.Close(opened.SessionID); err != nil {
		t.Fatal(err)
	}
	if services.sessions.Count() != 0 {
		t.Fatal("sessão local não foi removida")
	}
	_, err = sessions.OpenLocal("bash")
	wantBridgeCode(t, err, apperr.InvalidInput)
	wantBridgeCode(t, sessions.SendKeys(opened.SessionID, "ctrl+x"), apperr.InvalidInput)
}

func TestConnectionSettingsTermAndLegacyDefault(t *testing.T) {
	services := testServices(t)
	if err := services.vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	settings, err := services.vaultService.UpdateSettings(VaultSettingsPatch{
		"connectTimeoutSeconds": 42,
		"legacyDefault":         true,
		"term":                  "linux",
	})
	if err != nil || settings.ConnectTimeoutSeconds != 42 || !settings.LegacyDefault || settings.Term != "linux" {
		t.Fatalf("configurações = %+v, %v", settings, err)
	}

	var omitted domain.HostInput
	if err := json.Unmarshal([]byte(`{"label":"Padrão","address":"localhost","port":22,"username":"noc","authMethod":"ask","credentialId":null,"groupId":null}`), &omitted); err != nil {
		t.Fatal(err)
	}
	host, err := services.hostService.Create(omitted)
	if err != nil || !host.LegacyAlgorithms {
		t.Fatalf("host com padrão = %+v, %v", host, err)
	}
	options, err := services.sessionService.optionsForHost(host.ID)
	if err != nil || options.ConnectTimeout != 42*time.Second || options.Term != "linux" || !options.LegacyAlgorithms {
		t.Fatalf("opções SSH = %+v, %v", options, err)
	}

	var explicit domain.HostInput
	if err := json.Unmarshal([]byte(`{"label":"Explícito","address":"localhost","port":22,"username":"noc","authMethod":"ask","credentialId":null,"groupId":null,"legacyAlgorithms":false}`), &explicit); err != nil {
		t.Fatal(err)
	}
	host, err = services.hostService.Create(explicit)
	if err != nil || host.LegacyAlgorithms {
		t.Fatalf("host explícito = %+v, %v", host, err)
	}

	_, err = services.vaultService.UpdateSettings(VaultSettingsPatch{"term": "screen"})
	wantBridgeCode(t, err, apperr.InvalidInput)
}
