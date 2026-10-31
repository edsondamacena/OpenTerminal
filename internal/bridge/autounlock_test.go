package bridge

import (
	"context"
	"sync"
	"testing"

	"openterminal/internal/devicekey"
	"openterminal/internal/vault"
)

type memKeeper struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *memKeeper) Get(n string) ([]byte, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[n]
	return append([]byte(nil), v...), ok
}

func (k *memKeeper) Set(n string, v []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[n] = append([]byte(nil), v...)
	return nil
}

func (k *memKeeper) Delete(n string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, n)
	return nil
}

const autoPW = "senha mestra de teste 123"

func reopen(t *testing.T, dir string) *Services {
	t.Helper()
	s, err := newServices(dir, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	// Só o passo de abertura que importa aqui (Startup inteiro precisa do runtime do Wails).
	s.autoUnlock()
	return s
}

func TestAutoUnlockRemembersLastDecision(t *testing.T) {
	keeper := &memKeeper{m: map[string][]byte{}}
	SetDeviceKeeper(keeper)
	defer SetDeviceKeeper(nil)
	dir := t.TempDir()

	// Cria (destrava) e fecha o app com o cofre aberto: reabre destravado.
	s := reopen(t, dir)
	if err := s.vaultService.Create(autoPW, autoPW); err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
	s = reopen(t, dir)
	if st := s.store.Status().State; st != vault.StateUnlocked {
		t.Fatalf("esperava destravado ao reabrir, veio %s", st)
	}

	// Cadeado e fecha: reabre pedindo senha.
	_ = s.vaultService.Lock()
	s.Shutdown(context.Background())
	s = reopen(t, dir)
	if st := s.store.Status().State; st != vault.StateLocked {
		t.Fatalf("esperava bloqueado ao reabrir, veio %s", st)
	}

	// Destrava com a senha e fecha: volta a abrir sozinho.
	if err := s.vaultService.Unlock(autoPW); err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
	s = reopen(t, dir)
	if st := s.store.Status().State; st != vault.StateUnlocked {
		t.Fatalf("esperava destravado de novo, veio %s", st)
	}
	s.Shutdown(context.Background())
}

func TestAutoUnlockRejectsForeignKey(t *testing.T) {
	keeper := &memKeeper{m: map[string][]byte{}}
	SetDeviceKeeper(keeper)
	defer SetDeviceKeeper(nil)
	dir := t.TempDir()
	s := reopen(t, dir)
	if err := s.vaultService.Create(autoPW, autoPW); err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
	keeper.m[s.keeperName()] = make([]byte, 32) // chave de outro cofre
	s = reopen(t, dir)
	if st := s.store.Status().State; st != vault.StateLocked {
		t.Fatalf("chave errada não pode destravar: %s", st)
	}
	if _, ok := keeper.Get(s.keeperName()); ok {
		t.Fatal("chave errada deveria ser esquecida")
	}
	s.Shutdown(context.Background())
}

func TestAutoUnlockPedeSenhaDepoisDeReiniciarOPC(t *testing.T) {
	keeper := &memKeeper{m: map[string][]byte{}}
	SetDeviceKeeper(keeper)
	defer SetDeviceKeeper(nil)
	boot := "boot-1"
	bootID = func() string { return boot }
	defer func() { bootID = devicekey.BootID }()
	dir := t.TempDir()

	s := reopen(t, dir)
	if err := s.vaultService.Create(autoPW, autoPW); err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
	s = reopen(t, dir) // mesmo boot: abre sozinho
	if st := s.store.Status().State; st != vault.StateUnlocked {
		t.Fatalf("mesmo boot: esperava destravado, veio %s", st)
	}
	s.Shutdown(context.Background())

	boot = "boot-2" // PC reiniciou: pede a senha uma vez e esquece a chave antiga
	s = reopen(t, dir)
	if st := s.store.Status().State; st != vault.StateLocked {
		t.Fatalf("depois de reiniciar: esperava bloqueado, veio %s", st)
	}
	if len(keeper.m) != 0 {
		t.Fatalf("a chave antiga devia ter sido apagada: %v", keeper.m)
	}
	if err := s.vaultService.Unlock(autoPW); err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
	s = reopen(t, dir) // destravou neste boot: volta a abrir sozinho
	if st := s.store.Status().State; st != vault.StateUnlocked {
		t.Fatalf("depois da senha no novo boot: esperava destravado, veio %s", st)
	}
}
