package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/fonts"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

type profileTestConn struct{}

func (profileTestConn) Start(int, int, io.Writer) error { return nil }
func (profileTestConn) Wait() error                     { return nil }
func (profileTestConn) Write([]byte) error              { return nil }
func (profileTestConn) Resize(int, int) error           { return nil }
func (profileTestConn) Close() error                    { return nil }

func TestProfileServiceLifecycleDialogsAndEvents(t *testing.T) {
	services := testServices(t)
	if err := services.vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	profileService := services.profileService
	var events []ProfileChangedEvent
	profileService.emit = func(name string, payload any) {
		if name != "profiles:changed" {
			t.Errorf("evento %q", name)
		}
		events = append(events, payload.(ProfileChangedEvent))
	}

	global, err := profileService.GetGlobal()
	if err != nil || global.FontSize != 14 {
		t.Fatalf("perfil global: %+v %v", global, err)
	}
	var patch TerminalProfilePatch
	if err := json.Unmarshal([]byte(`{"fontSize":16,"cursorColor":"#123456"}`), &patch); err != nil {
		t.Fatal(err)
	}
	global, err = profileService.UpdateGlobal(patch)
	if err != nil || global.FontSize != 16 || global.CursorColor == nil || *global.CursorColor != "#123456" {
		t.Fatalf("perfil atualizado: %+v %v", global, err)
	}
	if err := json.Unmarshal([]byte(`{"cursorColor":null}`), &patch); err != nil {
		t.Fatal(err)
	}
	global, err = profileService.UpdateGlobal(patch)
	if err != nil || global.CursorColor != nil || global.FontSize != 16 {
		t.Fatalf("cursor limpo sem perder patch anterior: %+v %v", global, err)
	}

	host, err := services.hostService.Create(domain.HostInput{Label: "Host", Address: "localhost", Port: 22,
		Username: "user", AuthMethod: model.AuthAsk})
	if err != nil {
		t.Fatal(err)
	}
	override := global
	override.FontSize = 18
	stored, err := profileService.SetHostOverride(host.ID, &override)
	if err != nil || stored == nil || stored.FontSize != 18 {
		t.Fatalf("override: %+v %v", stored, err)
	}
	got, err := profileService.GetForHost(host.ID)
	if err != nil || got == nil || got.ID != stored.ID {
		t.Fatalf("get override: %+v %v", got, err)
	}
	sessionID := services.sessions.Open(session.KindSSH, &host.ID, func(context.Context, session.Interactor) (session.Conn, error) {
		return profileTestConn{}, nil
	})
	effective, err := profileService.Effective(sessionID)
	if err != nil || effective.ID != stored.ID {
		t.Fatalf("perfil efetivo: %+v %v", effective, err)
	}
	if err := services.sessions.Close(sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := profileService.SetHostOverride(host.ID, nil); err != nil {
		t.Fatal(err)
	}

	items, err := profileService.ListSchemes()
	if err != nil || len(items) != 8 {
		t.Fatalf("presets: %d %v", len(items), err)
	}
	copy, err := profileService.DuplicateScheme(items[0].ID, "Cópia")
	if err != nil || copy.Origin != model.SchemeCustom {
		t.Fatalf("duplicado: %+v %v", copy, err)
	}
	copy.Foreground = "#abcdef"
	if _, err := profileService.SaveScheme(copy); err != nil {
		t.Fatal(err)
	}
	if err := profileService.RemoveScheme(copy.ID); err != nil {
		t.Fatal(err)
	}

	profileService.pickImport = func(context.Context) (string, error) {
		return filepath.Join("..", "..", "tests", "fixtures", "schemes", "windows-terminal.json"), nil
	}
	imported, err := profileService.ImportScheme()
	if err != nil || imported == nil || len(imported.Imported) != 1 {
		t.Fatalf("importação: %+v %v", imported, err)
	}
	importedAgain, err := profileService.ImportScheme()
	if err != nil || importedAgain == nil || len(importedAgain.Imported) != 1 || importedAgain.Imported[0].Name != imported.Imported[0].Name+" (2)" {
		t.Fatalf("segunda importação: %+v %v", importedAgain, err)
	}
	exportPath := filepath.Join(t.TempDir(), "export.json")
	profileService.pickExport = func(context.Context, string) (string, error) { return exportPath, nil }
	if err := profileService.ExportScheme(imported.Imported[0].ID); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(exportPath); err != nil || !json.Valid(data) {
		t.Fatalf("exportação: %q %v", data, err)
	}

	profileService.listFonts = func() ([]fonts.Font, error) {
		return []fonts.Font{{Family: "Consolas", Source: fonts.SourceSystem}}, nil
	}
	fontList, err := profileService.ListFonts()
	if err != nil || len(fontList) != 1 || fontList[0].Family != "Consolas" {
		t.Fatalf("fontes: %+v %v", fontList, err)
	}
	if len(events) < 7 || events[0].Scope != "global" {
		t.Fatalf("eventos: %+v", events)
	}
	foundHost := false
	for _, event := range events {
		if event.Scope == "host" && event.HostID != nil && *event.HostID == host.ID {
			foundHost = true
		}
	}
	if !foundHost {
		t.Fatalf("evento de host ausente: %+v", events)
	}
}

func TestProfileServiceKeepsSafeSchemeReason(t *testing.T) {
	services := testServices(t)
	profileService := services.profileService
	profileService.pickImport = func(context.Context) (string, error) {
		return filepath.Join("..", "..", "tests", "fixtures", "schemes", "invalid.json"), nil
	}
	_, err := profileService.ImportScheme()
	if apperr.CodeOf(err) != apperr.SchemeInvalid {
		t.Fatalf("código: %v", err)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Message == apperr.Messages[apperr.SchemeInvalid] {
		t.Fatalf("motivo seguro perdido: %v", err)
	}
}
