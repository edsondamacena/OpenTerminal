package bridge

import (
	"testing"

	"openterminal/internal/model"
	"openterminal/internal/tabby"
)

func TestImportEntries(t *testing.T) {
	c := &model.Content{}
	entries := []tabby.Entry{
		{Name: "OLT 1", Protocol: "telnet", Address: "10.0.0.1", Port: 23, Username: "noc", Password: "a", Group: []string{"Provedor", "OLTs"}},
		{Name: "OLT 2", Protocol: "telnet", Address: "10.0.0.2", Port: 23, Password: "a", Group: []string{"Provedor", "OLTs"}},
		{Name: "BNG", Protocol: "ssh", Address: "10.0.0.3", Port: 22, Username: "noc", Password: "b", Legacy: true, Group: []string{"Provedor"}},
		{Name: "Sem senha", Protocol: "ssh", Address: "10.0.0.4", Port: 2222, Username: "root"},
		{Name: "Inválido", Protocol: "ssh", Address: "não é host", Port: 22, Username: "x", Password: "c"},
		{Name: "SSH sem usuário", Protocol: "ssh", Address: "10.0.0.5", Port: 22, Password: "d"},
	}
	r := importEntries(c, entries, "2026-10-27T00:00:00Z")
	want := TabbyImportResult{Hosts: 4, Telnet: 2, Groups: 2, Credentials: 2, NoPassword: 1, Invalid: 2}
	if r != want {
		t.Fatalf("1ª: %+v", r)
	}
	if len(c.Credentials) != 2 {
		t.Fatalf("credencial órfã: %d", len(c.Credentials))
	}
	byLabel := map[string]model.Host{}
	for _, h := range c.Hosts {
		byLabel[h.Label] = h
	}
	olt1, olt2, bng := byLabel["OLT 1"], byLabel["OLT 2"], byLabel["BNG"]
	if !olt1.IsTelnet() || olt1.AuthMethod != model.AuthPassword || *olt1.CredentialID != *olt2.CredentialID {
		t.Fatalf("telnet: %+v %+v", olt1, olt2)
	}
	if olt2.Username != "" || *olt1.GroupID != *olt2.GroupID || bng.IsTelnet() || !bng.LegacyAlgorithms {
		t.Fatalf("campos: %+v %+v", olt2, bng)
	}
	if byLabel["Sem senha"].AuthMethod != model.AuthAsk {
		t.Fatal("sem senha devia perguntar")
	}
	var oltGroup model.Group
	for _, g := range c.Groups {
		if g.ID == *olt1.GroupID {
			oltGroup = g
		}
	}
	if oltGroup.Name != "OLTs" || oltGroup.ParentID == nil || *oltGroup.ParentID != *bng.GroupID {
		t.Fatalf("hierarquia: %+v", oltGroup)
	}

	r = importEntries(c, entries, "2026-10-28T00:00:00Z")
	if r != (TabbyImportResult{Duplicates: 4, Invalid: 2}) || len(c.Hosts) != 4 || len(c.Groups) != 2 || len(c.Credentials) != 2 {
		t.Fatalf("2ª duplicou: %+v", r)
	}
}
