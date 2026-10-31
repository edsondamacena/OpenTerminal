package cloudsync

import (
	"context"
	"testing"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

const realPW = "senha mestra de teste 123"

func realDevice(t *testing.T, r Remote) (*vault.Store, *Engine) {
	t.Helper()
	s, err := vault.Open(t.TempDir(), vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Create(realPW, realPW); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(s, vault.TestKDF, nil)
	e.debounce = 1 << 62
	s.OnChange(e.Changed)
	if err := e.Connect(context.Background(), r, realPW); err != nil {
		t.Fatal(err)
	}
	return s, e
}

func TestRealStorePullsOnNewDevice(t *testing.T) {
	ctx := context.Background()
	cloud := &memRemote{}
	pc, ePC := realDevice(t, cloud)
	gid, cid := "g1", "c1"
	err := pc.Mutate(func(c *model.Content) error {
		c.Groups = append(c.Groups, model.Group{ID: gid, Name: "OLTs"})
		c.Credentials = append(c.Credentials, model.Credential{ID: cid, Kind: "password", Name: "noc", Password: "x", CreatedAt: "2026-10-27T10:00:00.000Z"})
		c.Hosts = append(c.Hosts, model.Host{ID: "h1", Label: "olt", Address: "10.0.0.1", Port: 22, Username: "admin",
			AuthMethod: "password", CredentialID: &cid, GroupID: &gid,
			CreatedAt: "2026-10-27T10:00:00.000Z", UpdatedAt: "2026-10-27T10:00:00.000Z"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ePC.SyncNow(ctx); err != nil {
		t.Fatal(err)
	}
	cel, eCel := realDevice(t, cloud)
	if st := eCel.Status(); st.State != "idle" {
		t.Fatalf("celular: %+v", st)
	}
	c, _ := cel.Read()
	if len(c.Hosts) != 1 || len(c.Credentials) != 1 || len(c.Groups) != 1 {
		t.Fatalf("celular não puxou: %d hosts", len(c.Hosts))
	}
}

func TestPullBumpsCounter(t *testing.T) {
	cloud := &memRemote{}
	pc, ePC := realDevice(t, cloud)
	_ = pc.Mutate(func(c *model.Content) error {
		c.Groups = append(c.Groups, model.Group{ID: "g"})
		return nil
	})
	if err := ePC.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, eCel := realDevice(t, cloud)
	if eCel.Status().Pulls != 1 || ePC.Status().Pulls != 0 {
		t.Fatalf("pulls: cel %d pc %d", eCel.Status().Pulls, ePC.Status().Pulls)
	}
}
