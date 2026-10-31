package cloudsync

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

// memRemote é a nuvem falsa: um arquivo só.
type memRemote struct {
	mu   sync.Mutex
	data []byte
	ups  int
}

func (r *memRemote) Download(context.Context) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		return nil, ErrNoFile
	}
	return append([]byte(nil), r.data...), nil
}

func (r *memRemote) Upload(_ context.Context, d []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data, r.ups = append([]byte(nil), d...), r.ups+1
	return nil
}

// memStore imita o vault.Store: Mutate avisa o motor, Replace não.
type memStore struct {
	mu    sync.Mutex
	c     *model.Content
	prefs map[string][]byte
	eng   *Engine
}

func newMemStore() *memStore {
	return &memStore{c: model.EmptyContent(), prefs: map[string][]byte{}}
}

func (s *memStore) Read() (*model.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(SnapshotOf(s.c))
	var snap Snapshot
	_ = json.Unmarshal(b, &snap)
	c := model.EmptyContent()
	snap.Apply(c)
	return c, nil
}

func (s *memStore) Replace(fn func(c *model.Content) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.c)
}

func (s *memStore) mutate(fn func(c *model.Content)) {
	s.mu.Lock()
	fn(s.c)
	s.mu.Unlock()
	s.eng.Changed()
}

func (s *memStore) Preference(k string, out any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.prefs[k]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(b, out)
}

func (s *memStore) SetPreference(k string, v any) error {
	b, err := json.Marshal(v)
	s.mu.Lock()
	s.prefs[k] = b
	s.mu.Unlock()
	return err
}

func fastKDF(salt []byte) vault.KDFParams { return vault.KDFParams{Time: 1, MemoryKiB: 64, Salt: salt} }

func device(t *testing.T, r Remote) (*memStore, *Engine) {
	s := newMemStore()
	e := NewEngine(s, fastKDF, nil)
	e.debounce = 1 << 62 // envio só pelo SyncNow do teste
	s.eng = e
	if err := e.Connect(context.Background(), r, "senha-mestra"); err != nil {
		t.Fatal(err)
	}
	return s, e
}

func hostIDs(s *memStore) map[string]string {
	c, _ := s.Read()
	out := map[string]string{}
	for _, h := range c.Hosts {
		out[h.ID] = h.Label
	}
	return out
}

func TestTwoDevicesConverge(t *testing.T) {
	ctx := context.Background()
	cloud := &memRemote{}
	pc, ePC := device(t, cloud)
	pc.mutate(func(c *model.Content) {
		c.Hosts = append(c.Hosts, model.Host{ID: "h1", Label: "olt", UpdatedAt: "2026-10-27T10:00:00Z"})
	})
	if err := ePC.SyncNow(ctx); err != nil {
		t.Fatal(err)
	}

	cel, eCel := device(t, cloud) // celular novo e vazio: puxa tudo
	if got := hostIDs(cel); got["h1"] != "olt" {
		t.Fatalf("celular não puxou o host: %v", got)
	}

	// Os dois mudam ao mesmo tempo: host novo de cada lado e edição concorrente do h1.
	pc.mutate(func(c *model.Content) {
		c.Hosts[0].Label, c.Hosts[0].UpdatedAt = "olt-pc", "2026-10-27T11:00:00Z"
		c.Hosts = append(c.Hosts, model.Host{ID: "h2", Label: "bng"})
	})
	cel.mutate(func(c *model.Content) {
		c.Hosts[0].Label, c.Hosts[0].UpdatedAt = "olt-cel", "2026-10-27T12:00:00Z"
		c.Hosts = append(c.Hosts, model.Host{ID: "h3", Label: "sw"})
	})
	for _, e := range []*Engine{ePC, eCel, ePC} {
		if err := e.SyncNow(ctx); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]string{"h1": "olt-cel", "h2": "bng", "h3": "sw"}
	for name, s := range map[string]*memStore{"pc": pc, "cel": cel} {
		got := hostIDs(s)
		if len(got) != 3 || got["h1"] != want["h1"] || got["h2"] != "bng" || got["h3"] != "sw" {
			t.Fatalf("%s não convergiu: %v", name, got)
		}
	}

	// Sem mudança local, sincronizar de novo não envia nada (sem eco entre aparelhos).
	ups := cloud.ups
	for _, e := range []*Engine{ePC, eCel, ePC, eCel} {
		if err := e.SyncNow(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if cloud.ups != ups {
		t.Fatalf("eco: %d envios sem mudança", cloud.ups-ups)
	}
}

func TestWrongPasswordDoesNotTouchLocal(t *testing.T) {
	cloud := &memRemote{}
	pc, ePC := device(t, cloud)
	pc.mutate(func(c *model.Content) { c.Hosts = append(c.Hosts, model.Host{ID: "h1"}) })
	if err := ePC.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	ups := cloud.ups
	s := newMemStore()
	e := NewEngine(s, fastKDF, nil)
	s.eng = e
	if err := e.Connect(context.Background(), cloud, "outra-senha"); err != ErrWrongSecret {
		t.Fatalf("esperava ErrWrongSecret, veio %v", err)
	}
	if len(hostIDs(s)) != 0 || cloud.ups != ups {
		t.Fatal("senha errada não pode puxar nem enviar")
	}
}

// racyRemote simula outro aparelho enviando entre a leitura e o envio deste.
type racyRemote struct {
	*memRemote
	other func()
	fired bool
}

func (r *racyRemote) Upload(ctx context.Context, d []byte) error {
	return r.memRemote.Upload(ctx, d)
}

func (r *racyRemote) Download(ctx context.Context) ([]byte, error) {
	data, err := r.memRemote.Download(ctx)
	if !r.fired && r.other != nil {
		r.fired = true
		r.other()
	}
	return data, err
}

func TestUploadDoesNotOverwriteConcurrentDevice(t *testing.T) {
	ctx := context.Background()
	cloud := &memRemote{}
	pc, ePC := device(t, cloud)
	cel, eCel := device(t, cloud)
	cel.mutate(func(c *model.Content) { c.Hosts = append(c.Hosts, model.Host{ID: "cel"}) })
	pc.mutate(func(c *model.Content) { c.Hosts = append(c.Hosts, model.Host{ID: "pc"}) })

	// O PC lê a nuvem; o celular envia logo em seguida, antes do envio do PC.
	racy := &racyRemote{memRemote: cloud, other: func() {
		if err := eCel.SyncNow(ctx); err != nil {
			t.Error(err)
		}
	}}
	ePC.SetRemote(racy)
	if err := ePC.SyncNow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := eCel.SyncNow(ctx); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]*memStore{"pc": pc, "cel": cel} {
		if got := hostIDs(s); len(got) != 2 {
			t.Fatalf("%s perdeu host: %v", name, got)
		}
	}
}
