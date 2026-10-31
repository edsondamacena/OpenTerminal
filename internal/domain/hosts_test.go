package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"openterminal/internal/model"
	"openterminal/internal/validate"
)

func TestCreateHost(t *testing.T) {
	c := testContent()
	in := baseInput()
	in.Label = "  Servidor  "
	h, err := CreateHost(c, in, t0)
	mustOK(t, err)
	if h.Label != "Servidor" || h.CreatedAt != t0 || h.UpdatedAt != t0 || validate.UUID(h.ID) != nil {
		t.Fatalf("host: %+v", h)
	}
	if h.LegacyAlgorithms || h.ProfileID != nil {
		t.Fatal("campos novos devem vir com padrão")
	}
	if len(c.Hosts) != 1 {
		t.Fatal("não adicionou")
	}
}

func TestCreateHostRejects(t *testing.T) {
	cases := map[string]func(*HostInput){
		"label vazio":          func(i *HostInput) { i.Label = "   " },
		"label > 100":          func(i *HostInput) { i.Label = strings.Repeat("x", 101) },
		"endereço inválido":    func(i *HostInput) { i.Address = "não é host!" },
		"endereço com espaço":  func(i *HostInput) { i.Address = "a b" },
		"porta 0":              func(i *HostInput) { i.Port = 0 },
		"porta 65536":          func(i *HostInput) { i.Port = 65536 },
		"usuário vazio":        func(i *HostInput) { i.Username = "" },
		"usuário com espaço":   func(i *HostInput) { i.Username = "de ploy" },
		"usuário com controle": func(i *HostInput) { i.Username = "a\u0007b" },
		"usuário > 255":        func(i *HostInput) { i.Username = strings.Repeat("u", 256) },
		"método desconhecido":  func(i *HostInput) { i.AuthMethod = "kerberos" },
	}
	for name, patch := range cases {
		in := baseInput()
		patch(&in)
		if _, err := CreateHost(testContent(), in, t0); err == nil {
			t.Errorf("%s: aceito", name)
		} else {
			wantCode(t, err, "INVALID_INPUT")
		}
	}
}

func TestAcceptsAddresses(t *testing.T) {
	for _, a := range []string{"10.0.0.1", "::1", "[2001:db8::1]", "fe80::1", "a-b.c", "localhost", "SRV01"} {
		in := baseInput()
		in.Address = a
		if _, err := CreateHost(testContent(), in, t0); err != nil {
			t.Errorf("%s rejeitado: %v", a, err)
		}
	}
}

func TestNormalizeIPv6(t *testing.T) {
	if got := NormalizeAddress("[2001:DB8::1]"); got != "2001:db8::1" {
		t.Fatalf("got %s", got)
	}
	in := baseInput()
	in.Address = "[::1]"
	h, _ := CreateHost(testContent(), in, t0)
	if h.Address != "::1" {
		t.Fatalf("got %s", h.Address)
	}
}

func TestCredentialRules(t *testing.T) {
	in := baseInput()
	in.CredentialID = nil
	_, err := CreateHost(testContent(), in, t0)
	wantCode(t, err, "INVALID_INPUT")

	in = baseInput()
	in.AuthMethod = model.AuthAsk
	h, _ := CreateHost(testContent(), in, t0)
	if h.CredentialID != nil {
		t.Fatal("ask deveria zerar a credencial")
	}

	// "agent" não usa credentialId (MUST ser null).
	in = baseInput()
	in.AuthMethod = model.AuthAgent
	h, err = CreateHost(testContent(), in, t0)
	mustOK(t, err)
	if h.CredentialID != nil {
		t.Fatal("agent deveria zerar a credencial")
	}

	in = baseInput()
	in.CredentialID = model.Ptr(keyCred)
	_, err = CreateHost(testContent(), in, t0)
	wantCode(t, err, "INVALID_INPUT")

	in = baseInput()
	in.AuthMethod = model.AuthKey
	_, err = CreateHost(testContent(), in, t0)
	wantCode(t, err, "INVALID_INPUT")
	in.CredentialID = model.Ptr(keyCred)
	_, err = CreateHost(testContent(), in, t0)
	mustOK(t, err)

	in = baseInput()
	in.CredentialID = model.Ptr(missing)
	_, err = CreateHost(testContent(), in, t0)
	wantCode(t, err, "INVALID_INPUT")
	in = baseInput()
	in.GroupID = model.Ptr(missing)
	_, err = CreateHost(testContent(), in, t0)
	wantCode(t, err, "INVALID_INPUT")
}

func TestLegacyAlgorithmsDefaultAndSet(t *testing.T) {
	in := baseInput()
	in.LegacyAlgorithms = true
	h, _ := CreateHost(testContent(), in, t0)
	if !h.LegacyAlgorithms {
		t.Fatal("opção não gravada")
	}
}

func TestUpdateHost(t *testing.T) {
	c := testContent()
	h, _ := CreateHost(c, baseInput(), t0)
	c.Hosts[0].ProfileID = model.Ptr("perfil-do-host")
	in := baseInput()
	in.Port = 2222
	u, err := UpdateHost(c, h.ID, in, t1)
	mustOK(t, err)
	if u.ID != h.ID || u.Port != 2222 || u.CreatedAt != t0 || u.UpdatedAt != t1 {
		t.Fatalf("update: %+v", u)
	}
	// O override de perfil é do ProfileService; editar o host não o perde.
	if u.ProfileID == nil || *u.ProfileID != "perfil-do-host" {
		t.Fatal("update perdeu o profileId")
	}
	_, err = UpdateHost(c, missing, baseInput(), t1)
	wantCode(t, err, "NOT_FOUND")
}

func TestRemoveHostDropsRulesKeepsCredentials(t *testing.T) {
	c := testContent()
	h, _ := CreateHost(c, baseInput(), t0)
	c.ForwardRules = append(c.ForwardRules, model.ForwardRule{ID: "00000000-0000-4000-8000-0000000000b1",
		HostID: h.ID, BindAddress: "127.0.0.1", BindPort: 15432, DestHost: "db", DestPort: 5432})
	mustOK(t, RemoveHost(c, h.ID))
	if len(c.Hosts) != 0 || len(c.ForwardRules) != 0 || len(c.Credentials) != 2 {
		t.Fatalf("depois de remover: %d hosts, %d regras, %d credenciais", len(c.Hosts), len(c.ForwardRules), len(c.Credentials))
	}
}

func TestHostViewsHaveNoSecrets(t *testing.T) {
	c := testContent()
	_, _ = CreateHost(c, baseInput(), t0)
	views := ListHostViews(c)
	if views[0].CredentialName == nil || *views[0].CredentialName != "senha" {
		t.Fatal("nome da credencial ausente")
	}
	b, _ := json.Marshal(views)
	if strings.Contains(string(b), `"password":"x"`) {
		t.Fatal("view contém segredo")
	}
}

func TestDuplicateLabelsAllowed(t *testing.T) {
	c := testContent()
	_, _ = CreateHost(c, baseInput(), t0)
	in := baseInput()
	in.Address = "10.0.0.2"
	_, err := CreateHost(c, in, t0)
	mustOK(t, err)
}

func TestMoveHost(t *testing.T) {
	c := testContent()
	g, _ := CreateGroup(c, "A", nil)
	h, _ := CreateHost(c, baseInput(), t0)
	mustOK(t, MoveHost(c, h.ID, &g.ID, t1))
	if *c.Hosts[0].GroupID != g.ID || c.Hosts[0].UpdatedAt != t1 {
		t.Fatal("não moveu")
	}
	mustOK(t, MoveHost(c, h.ID, nil, t1))
	if c.Hosts[0].GroupID != nil {
		t.Fatal("não voltou à raiz")
	}
	wantCode(t, MoveHost(c, h.ID, model.Ptr(missing), t1), "INVALID_INPUT")
	wantCode(t, MoveHost(c, missing, nil, t1), "NOT_FOUND")
}

func TestDuplicateHost(t *testing.T) {
	c := testContent()
	g, _ := CreateGroup(c, "A", nil)
	in := baseInput()
	in.GroupID = &g.ID
	in.LegacyAlgorithms = true
	h, _ := CreateHost(c, in, t0)
	c.Hosts[0].ProfileID = model.Ptr("perfil")
	c.ForwardRules = append(c.ForwardRules, model.ForwardRule{ID: "00000000-0000-4000-8000-0000000000b1",
		HostID: h.ID, Label: model.Ptr("db"), BindAddress: "127.0.0.1", BindPort: 15432,
		DestHost: "db.internal", DestPort: 5432})
	cp, err := DuplicateHost(c, h.ID, t1)
	mustOK(t, err)
	if cp.ID == h.ID || cp.Label != "Servidor (cópia)" || *cp.CredentialID != pwCred ||
		*cp.GroupID != g.ID || cp.CreatedAt != t1 || !cp.LegacyAlgorithms || cp.ProfileID == nil {
		t.Fatalf("cópia: %+v", cp)
	}
	var copied []model.ForwardRule
	for _, r := range c.ForwardRules {
		if r.HostID == cp.ID {
			copied = append(copied, r)
		}
	}
	if len(copied) != 1 || copied[0].ID == "00000000-0000-4000-8000-0000000000b1" ||
		copied[0].DestHost != "db.internal" || copied[0].BindPort != 15433 {
		t.Fatalf("regras copiadas: %+v", copied)
	}
}

func TestDuplicateLabelLimit(t *testing.T) {
	c := testContent()
	in := baseInput()
	in.Label = strings.Repeat("ç", 100)
	h, _ := CreateHost(c, in, t0)
	cp, _ := DuplicateHost(c, h.ID, t1)
	if utf8.RuneCountInString(cp.Label) > 100 || !strings.HasSuffix(cp.Label, " (cópia)") {
		t.Fatalf("rótulo: %q", cp.Label)
	}
}
