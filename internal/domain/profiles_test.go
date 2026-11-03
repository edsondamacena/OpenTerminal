package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/vault"
)

func testProfiles(t *testing.T) (*vault.Store, *Profiles) {
	t.Helper()
	store, err := vault.Open(t.TempDir(), vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	return store, NewProfiles(store)
}

func TestTerminalProfileRanges(t *testing.T) {
	_, profiles := testProfiles(t)
	base, err := profiles.GetGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if base.FontSize != 14 || base.LineHeight != 1.2 || base.Scrollback != 10_000 ||
		base.BackgroundOpacity != 1 || base.Blur != 0 || base.Padding != 12 ||
		base.FontWeight != 400 || base.FontWeightBold != 700 {
		t.Fatalf("defaults: %+v", base)
	}

	cases := []struct {
		name string
		bad  func(*model.TerminalProfile)
	}{
		{"fontSize", func(p *model.TerminalProfile) { p.FontSize = 7.9 }},
		{"fontWeight", func(p *model.TerminalProfile) { p.FontWeight = 99 }},
		{"fontWeightBold", func(p *model.TerminalProfile) { p.FontWeightBold = 901 }},
		{"lineHeight", func(p *model.TerminalProfile) { p.LineHeight = 2.1 }},
		{"letterSpacing", func(p *model.TerminalProfile) { p.LetterSpacing = -2.1 }},
		{"scrollback", func(p *model.TerminalProfile) { p.Scrollback = 99 }},
		{"backgroundOpacity", func(p *model.TerminalProfile) { p.BackgroundOpacity = .49 }},
		{"blur", func(p *model.TerminalProfile) { p.Blur = 41 }},
		{"padding", func(p *model.TerminalProfile) { p.Padding = 33 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := base
			tc.bad(&bad)
			_, err := profiles.UpdateGlobal(bad)
			wantCode(t, err, apperr.InvalidInput)
		})
	}
}

func TestExactlyOneGlobalAndHostOverrideLifecycle(t *testing.T) {
	store, profiles := testProfiles(t)
	all, err := store.TerminalProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || !all[0].IsGlobal {
		t.Fatalf("initial profiles: %+v", all)
	}

	input := baseInput()
	input.AuthMethod, input.CredentialID = model.AuthAsk, nil
	host, err := CreateHost(mustContent(t, store), input, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Mutate(func(c *model.Content) error {
		c.Hosts = append(c.Hosts, host)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	global, _ := profiles.GetGlobal()
	global.FontSize = 18
	override, err := profiles.SetHostOverride(host.ID, &global)
	if err != nil {
		t.Fatal(err)
	}
	if override == nil || override.IsGlobal || override.ID == global.ID {
		t.Fatalf("override: %+v", override)
	}
	want := global
	want.ID, want.Name, want.IsGlobal = override.ID, override.Name, false
	if !reflect.DeepEqual(*override, want) {
		t.Fatalf("override is not a full copy:\n got %+v\nwant %+v", *override, want)
	}
	content, _ := store.Read()
	stored, _ := FindHost(content, host.ID)
	if stored.ProfileID == nil || *stored.ProfileID != override.ID {
		t.Fatalf("host profileId: %+v", stored.ProfileID)
	}

	removed, err := profiles.SetHostOverride(host.ID, nil)
	if err != nil || removed != nil {
		t.Fatalf("remove override: %+v %v", removed, err)
	}
	content, _ = store.Read()
	stored, _ = FindHost(content, host.ID)
	if stored.ProfileID != nil {
		t.Fatalf("profileId not cleared: %v", *stored.ProfileID)
	}
}

const brandID = "00000000-0000-4000-8000-000000000001"

func TestBuiltinSchemeIsImmutableAndSchemeInUseCannotBeRemoved(t *testing.T) {
	_, profiles := testProfiles(t)
	schemes, err := profiles.ListSchemes()
	if err != nil || len(schemes) == 0 || schemes[0].Origin != model.SchemeBuiltin {
		t.Fatalf("builtin schemes: %+v %v", schemes, err)
	}
	builtin := schemes[0]
	builtin.Name = "editado"
	_, err = profiles.SaveScheme(builtin)
	wantCode(t, err, apperr.InvalidInput)
	wantCode(t, profiles.RemoveScheme(schemes[0].ID), apperr.InvalidInput)

	custom, err := profiles.DuplicateScheme(schemes[0].ID, "Meu esquema")
	if err != nil {
		t.Fatal(err)
	}
	global, _ := profiles.GetGlobal()
	global.ColorSchemeID = custom.ID
	if _, err := profiles.UpdateGlobal(global); err != nil {
		t.Fatal(err)
	}
	// Aplicado só no global: exclui e o global volta para o esquema da marca.
	if err := profiles.RemoveScheme(custom.ID); err != nil {
		t.Fatalf("excluir o esquema aplicado no global = %v", err)
	}
	global, _ = profiles.GetGlobal()
	if global.ColorSchemeID != brandID {
		t.Fatalf("global ficou com %q, esperava o da marca %q", global.ColorSchemeID, brandID)
	}
}

func TestImportedSchemeGetsUniqueNameAndUserDuplicatesFail(t *testing.T) {
	_, profiles := testProfiles(t)
	items, err := profiles.ListSchemes()
	if err != nil {
		t.Fatal(err)
	}
	imported := items[0]
	imported.ID, imported.Name, imported.Origin = "", "Fixture WT", model.SchemeImported
	first, err := profiles.SaveScheme(imported)
	if err != nil || first.Name != "Fixture WT" {
		t.Fatalf("primeira importação = %+v, %v", first, err)
	}
	imported.ID = ""
	second, err := profiles.SaveScheme(imported)
	if err != nil || second.Name != "Fixture WT (2)" || second.ID == first.ID {
		t.Fatalf("segunda importação = %+v, %v", second, err)
	}
	imported.ID = ""
	third, err := profiles.SaveScheme(imported)
	if err != nil || third.Name != "Fixture WT (3)" {
		t.Fatalf("terceira importação = %+v, %v", third, err)
	}

	custom, err := profiles.DuplicateScheme(items[0].ID, "Nome exclusivo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = profiles.DuplicateScheme(items[0].ID, custom.Name)
	if apperr.CodeOf(err) != apperr.AlreadyExists {
		t.Fatalf("duplicação com nome repetido = %v", err)
	}
	custom.Name = first.Name
	_, err = profiles.SaveScheme(custom)
	if apperr.CodeOf(err) != apperr.AlreadyExists {
		t.Fatalf("renomeação para nome repetido = %v", err)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || !strings.Contains(appErr.Detail, first.Name) {
		t.Fatalf("detalhe do conflito = %v", err)
	}
}

func TestProfilesPersistAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := vault.Open(dir, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	profiles := NewProfiles(store)
	global, _ := profiles.GetGlobal()
	global.FontFamily, global.FontSize = "Cascadia Code", 16
	if _, err := profiles.UpdateGlobal(global); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := vault.Open(dir, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := NewProfiles(reopened).GetGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if got.FontFamily != "Cascadia Code" || got.FontSize != 16 {
		t.Fatalf("persisted global: %+v", got)
	}
	all, err := reopened.TerminalProfiles()
	if err != nil || len(all) != 1 || !all[0].IsGlobal {
		t.Fatalf("profiles after reopen: %+v %v", all, err)
	}
}

func mustContent(t *testing.T, store *vault.Store) *model.Content {
	t.Helper()
	c, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
