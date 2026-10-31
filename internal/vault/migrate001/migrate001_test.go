package migrate001

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/vault"
)

const fixtureDir = "../../../tests/fixtures/vault-001"

type expected struct {
	Password string `json:"password"`
	Content  struct {
		Settings struct {
			AutoLockMinutes       *int `json:"autoLockMinutes"`
			ConnectTimeoutSeconds int  `json:"connectTimeoutSeconds"`
		} `json:"settings"`
		Groups        []model.Group       `json:"groups"`
		Hosts         []model.Host        `json:"hosts"`
		Credentials   []model.Credential  `json:"credentials"`
		KnownHostKeys []model.KnownHost   `json:"knownHostKeys"`
		ForwardRules  []model.ForwardRule `json:"forwardRules"`
	} `json:"content"`
}

func setup(t *testing.T) (string, expected) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"vault.svlt", "settings.json"} {
		b, err := os.ReadFile(filepath.Join(fixtureDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exp expected
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatal(err)
	}
	return dir, exp
}

func openStore(t *testing.T, dir string) *vault.Store {
	t.Helper()
	s, err := vault.Open(dir, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigratePreservesEverything(t *testing.T) {
	dir, exp := setup(t)
	s := openStore(t, dir)
	if s.Status().State != vault.StateMigratable {
		t.Fatalf("estado inicial: %s", s.Status().State)
	}
	if err := Migrate(s, exp.Password); err != nil {
		t.Fatal(err)
	}
	if s.Status().State != vault.StateUnlocked {
		t.Fatalf("após migrar: %s", s.Status().State)
	}
	c, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	e := exp.Content
	if c.Settings.ConnectTimeoutSeconds != e.Settings.ConnectTimeoutSeconds ||
		*c.Settings.AutoLockMinutes != *e.Settings.AutoLockMinutes || c.Settings.KeepaliveSeconds != 30 {
		t.Errorf("configurações: %+v", c.Settings)
	}
	if !jsonEqual(c.Groups, e.Groups) {
		t.Errorf("grupos:\n got %+v\nwant %+v", c.Groups, e.Groups)
	}
	if len(c.Hosts) != len(e.Hosts) {
		t.Fatalf("hosts: %d de %d", len(c.Hosts), len(e.Hosts))
	}
	for i, h := range c.Hosts {
		w := e.Hosts[i]
		if h.LegacyAlgorithms || h.ProfileID != nil {
			t.Errorf("host %s: campos novos deveriam vir com padrão", h.ID)
		}
		if !jsonEqual(h, w) {
			t.Errorf("host %d:\n got %+v\nwant %+v", i, h, w)
		}
	}
	if !jsonEqual(c.Credentials, e.Credentials) {
		t.Errorf("credenciais divergem")
	}
	if !jsonEqual(c.ForwardRules, e.ForwardRules) {
		t.Errorf("regras:\n got %+v\nwant %+v", c.ForwardRules, e.ForwardRules)
	}
	if len(c.KnownHosts) != len(e.KnownHostKeys) {
		t.Fatalf("host keys: %d de %d", len(c.KnownHosts), len(e.KnownHostKeys))
	}
	for i, k := range c.KnownHosts {
		w := e.KnownHostKeys[i]
		if k.ID == "" || k.Endpoint != w.Endpoint || k.KeyType != w.KeyType || k.Fingerprint != w.Fingerprint || k.AcceptedAt != w.AcceptedAt {
			t.Errorf("host key %d: %+v", i, k)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, "vault.svlt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("vault.svlt deveria ter sido renomeado")
	}
	if _, err := os.Stat(filepath.Join(dir, "vault.svlt.migrated")); err != nil {
		t.Error("vault.svlt.migrated ausente")
	}

	var theme string
	if ok, _ := s.Preference("theme", &theme); !ok || theme != "dark" {
		t.Errorf("tema não migrou: %q", theme)
	}
	var font TerminalFont
	if ok, _ := s.Preference(PrefLegacyTerminalFont, &font); ok {
		t.Errorf("preferência temporária da fonte não foi removida: %+v", font)
	}
	profile, err := s.GlobalProfile()
	if err != nil || profile.FontFamily != "Consolas, monospace" || profile.FontSize != 15 {
		t.Errorf("fonte não migrou para o perfil global: %+v, %v", profile, err)
	}

	// Reabrir: cofre novo, sem pedir migração de novo.
	s.Lock()
	_ = s.Close()
	s2 := openStore(t, dir)
	if s2.Status().State != vault.StateLocked {
		t.Fatalf("reaberto: %s", s2.Status().State)
	}
	if err := s2.Unlock(exp.Password); err != nil {
		t.Fatal(err)
	}
}

func jsonEqual(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return bytes.Equal(ja, jb)
}

func assertUntouched(t *testing.T, dir string, s *vault.Store) {
	t.Helper()
	orig, _ := os.ReadFile(filepath.Join(fixtureDir, "vault.svlt"))
	now, err := os.ReadFile(filepath.Join(dir, "vault.svlt"))
	if err != nil || !bytes.Equal(orig, now) {
		t.Fatal("vault.svlt foi alterado")
	}
	if s.Status().State != vault.StateMigratable {
		t.Fatalf("estado após falha: %s", s.Status().State)
	}
}

func TestWrongPasswordChangesNothing(t *testing.T) {
	dir, _ := setup(t)
	s := openStore(t, dir)
	err := Migrate(s, "senha-errada-1")
	if apperr.CodeOf(err) != apperr.VaultBadPassword {
		t.Fatalf("esperava VAULT_BAD_PASSWORD, veio %v", err)
	}
	assertUntouched(t, dir, s)
}

func TestFailureAfterImportRollsBack(t *testing.T) {
	dir, exp := setup(t)
	s := openStore(t, dir)
	verifyHook = func() error { return errors.New("falha simulada") }
	t.Cleanup(func() { verifyHook = nil })
	err := Migrate(s, exp.Password)
	if apperr.CodeOf(err) != apperr.VaultMigrationFailed {
		t.Fatalf("esperava VAULT_MIGRATION_FAILED, veio %v", err)
	}
	assertUntouched(t, dir, s)
	// Depois da falha, uma nova tentativa funciona.
	verifyHook = nil
	if err := Migrate(s, exp.Password); err != nil {
		t.Fatal(err)
	}
}

func TestNewerFormatIsUnreadable(t *testing.T) {
	dir, _ := setup(t)
	path := filepath.Join(dir, "vault.svlt")
	b, _ := os.ReadFile(path)
	binary.BigEndian.PutUint16(b[4:6], 2)
	_ = os.WriteFile(path, b, 0o600)
	s := openStore(t, dir)
	st := s.Status()
	if st.State != vault.StateUnreadable || st.Reason == nil || *st.Reason != vault.ReasonUnsupportedVersion {
		t.Fatalf("estado: %+v", st)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(b, after) {
		t.Fatal("arquivo foi tocado")
	}
}
