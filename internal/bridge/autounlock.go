package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"openterminal/internal/devicekey"
	"openterminal/internal/vault"
)

// Desbloqueio automático: o app lembra a última decisão do cadeado. Destravar com a senha guarda
// a chave de dados no cofre do sistema; bloquear (cadeado ou inatividade) apaga. Fechar o app
// com o cofre aberto não conta como bloquear: na próxima abertura ele já vem destravado.
// A senha mestra nunca é guardada.
// A chave vale só até o PC desligar ou reiniciar: no primeiro uso depois de ligar, pede a senha
// mestra uma vez; depois disso o cofre só bloqueia no cadeado.

var deviceKeeper = devicekey.Default()

// bootID é trocado nos testes para simular um reinício.
var bootID = devicekey.BootID

// SetDeviceKeeper troca o cofre do sistema (o Android registra o Keystore dele).
func SetDeviceKeeper(k devicekey.Keeper) { deviceKeeper = k }

// keeperName separa pastas de dados diferentes (testes, perfis) no mesmo chaveiro.
func (s *Services) keeperName() string {
	sum := sha256.Sum256([]byte(s.dataDir))
	return "dek-" + hex.EncodeToString(sum[:8])
}

func (s *Services) rememberUnlock() {
	if deviceKeeper == nil {
		return
	}
	dek, err := s.store.RememberDEK()
	if err != nil {
		return
	}
	defer vault.Zero(dek)
	if deviceKeeper.Set(s.keeperName(), dek) == nil {
		_ = deviceKeeper.Set(s.keeperName()+"-boot", []byte(bootID()))
	}
}

func (s *Services) forgetUnlock() {
	if deviceKeeper != nil {
		_ = deviceKeeper.Delete(s.keeperName())
		_ = deviceKeeper.Delete(s.keeperName() + "-boot")
	}
}

// autoUnlock roda na abertura: se a última decisão foi deixar aberto, destrava sem senha.
func (s *Services) autoUnlock() {
	if deviceKeeper == nil || s.store.Status().State != vault.StateLocked {
		return
	}
	dek, ok := deviceKeeper.Get(s.keeperName())
	if !ok {
		return
	}
	if boot, okBoot := deviceKeeper.Get(s.keeperName() + "-boot"); bootID() != "" && (!okBoot || string(boot) != bootID()) {
		// Chave de antes do PC reiniciar (ou de versão sem o boot guardado): esquece e pede a senha.
		vault.Zero(dek)
		s.forgetUnlock()
		return
	}
	defer vault.Zero(dek)
	if err := s.store.UnlockWithDEK(dek); err != nil {
		// Chave de outro cofre (apagado e recriado): esquece e pede a senha.
		s.forgetUnlock()
		return
	}
	s.syncService.unlockedWithKey()
}

const syncKeyPref = "cloudsync.key"

// syncKey é a chave da nuvem, cifrada com a chave de dados, para o sync voltar sem a senha.
type syncKey struct {
	Key       []byte `json:"key"`
	Salt      []byte `json:"salt"`
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memoryKiB"`
}

func (s *SyncService) keepKey(key []byte, p vault.KDFParams) {
	plain, err := json.Marshal(syncKey{Key: key, Salt: p.Salt, Time: p.Time, MemoryKiB: p.MemoryKiB})
	if err != nil {
		return
	}
	defer vault.Zero(plain)
	if sealed, err := s.store.SealSecret(syncKeyPref, plain); err == nil {
		_ = s.store.SetPreference(syncKeyPref, sealed)
	}
}

func (s *SyncService) unlockedWithKey() {
	var link googleLink
	if ok, err := s.store.Preference(googlePref, &link); err != nil || !ok || len(link.Token) == 0 {
		s.engine.SetRemote(nil)
		return
	}
	var sealed []byte
	if ok, _ := s.store.Preference(syncKeyPref, &sealed); !ok {
		return
	}
	plain, err := s.store.OpenSecret(syncKeyPref, sealed)
	if err != nil {
		return
	}
	defer vault.Zero(plain)
	var k syncKey
	if json.Unmarshal(plain, &k) != nil || len(k.Key) == 0 {
		return
	}
	defer vault.Zero(k.Key)
	if !s.connectRemote(link) {
		return
	}
	s.engine.UnlockedWithKey(k.Key, vault.KDFParams{Time: k.Time, MemoryKiB: k.MemoryKiB, Salt: k.Salt})
}
