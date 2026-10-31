// Package cloudsync mantém uma cópia cifrada do cofre na pasta oculta do app no Google Drive
// (appDataFolder) e a reconcilia entre aparelhos. O Google só recebe bytes cifrados: a chave sai
// da senha mestra (Argon2id) e o conteúdo vai com XChaCha20-Poly1305, como no cofre local.
package cloudsync

import (
	"encoding/json"
	"errors"
	"fmt"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

const (
	format  = "openterminal-sync"
	version = 1
	// Um arquivo forjado na nuvem não pode pedir um Argon2 absurdo a este aparelho.
	maxTime      = 10
	maxMemoryKiB = 1024 * 1024
)

var (
	ErrNotSync     = errors.New("o arquivo da nuvem não é do OpenTerminal")
	ErrWrongSecret = errors.New("a senha mestra deste aparelho não abre o cofre da nuvem")
)

// Snapshot é o conteúdo sincronizado: o cofre inteiro, inclusive as configurações dele.
type Snapshot struct {
	Settings     model.VaultSettings `json:"settings"`
	Groups       []model.Group       `json:"groups"`
	Hosts        []model.Host        `json:"hosts"`
	Credentials  []model.Credential  `json:"credentials"`
	KnownHosts   []model.KnownHost   `json:"knownHosts"`
	ForwardRules []model.ForwardRule `json:"forwardRules"`
}

func SnapshotOf(c *model.Content) Snapshot {
	return Snapshot{Settings: c.Settings, Groups: c.Groups, Hosts: c.Hosts, Credentials: c.Credentials,
		KnownHosts: c.KnownHosts, ForwardRules: c.ForwardRules}
}

// Apply troca o conteúdo pelo da nuvem (listas nil viram vazias).
func (s Snapshot) Apply(c *model.Content) {
	c.Settings = s.Settings
	c.Groups = nonNil(s.Groups)
	c.Hosts = nonNil(s.Hosts)
	c.Credentials = nonNil(s.Credentials)
	c.KnownHosts = nonNil(s.KnownHosts)
	c.ForwardRules = nonNil(s.ForwardRules)
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// Header é a parte aberta do arquivo. Gen sobe a cada envio; Device diz quem enviou.
type Header struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	Gen       int64  `json:"gen"`
	Device    string `json:"device"`
	UpdatedAt string `json:"updatedAt"`
	KDF       struct {
		Alg       string `json:"alg"`
		Time      uint32 `json:"time"`
		MemoryKiB uint32 `json:"memoryKiB"`
		Salt      []byte `json:"salt"`
	} `json:"kdf"`
}

type envelope struct {
	Header
	Data []byte `json:"data"`
}

// Params devolve os parâmetros do KDF do cabeçalho.
func (h Header) Params() vault.KDFParams {
	return vault.KDFParams{Time: h.KDF.Time, MemoryKiB: h.KDF.MemoryKiB, Salt: h.KDF.Salt}
}

// ad amarra cabeçalho e conteúdo: mexer em geração, aparelho ou KDF faz a abertura falhar.
func (h Header) ad() []byte {
	return fmt.Appendf(nil, "%s:v%d|%d|%s|%s|%s|%d|%d|%x", h.Format, h.Version, h.Gen, h.Device,
		h.UpdatedAt, h.KDF.Alg, h.KDF.Time, h.KDF.MemoryKiB, h.KDF.Salt)
}

// ReadHeader lê e valida só o cabeçalho, sem decifrar.
func ReadHeader(data []byte) (Header, error) {
	var e envelope
	if err := json.Unmarshal(data, &e); err != nil || e.Format != format {
		return Header{}, ErrNotSync
	}
	h := e.Header
	if h.Version != version || h.KDF.Alg != "argon2id" || h.KDF.Time < 1 || h.KDF.Time > maxTime ||
		h.KDF.MemoryKiB < 8 || h.KDF.MemoryKiB > maxMemoryKiB || len(h.KDF.Salt) < 16 || h.Gen < 1 {
		return Header{}, ErrNotSync
	}
	return h, nil
}

// Seal cifra o snapshot com a chave já derivada; h precisa trazer KDF, Gen e Device.
func Seal(s Snapshot, key []byte, h Header) ([]byte, error) {
	h.Format, h.Version, h.KDF.Alg = format, version, "argon2id"
	plain, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	defer vault.Zero(plain)
	e := envelope{Header: h}
	if e.Data, err = vault.Seal(key, plain, h.ad()); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

// Open decifra; ErrWrongSecret quando a chave não confere (senha diferente ou arquivo alterado).
func Open(data, key []byte) (Header, Snapshot, error) {
	h, err := ReadHeader(data)
	if err != nil {
		return Header{}, Snapshot{}, err
	}
	var e envelope
	_ = json.Unmarshal(data, &e)
	plain, err := vault.Unseal(key, e.Data, h.ad())
	if err != nil {
		return Header{}, Snapshot{}, ErrWrongSecret
	}
	defer vault.Zero(plain)
	var s Snapshot
	if err := json.Unmarshal(plain, &s); err != nil {
		return Header{}, Snapshot{}, ErrNotSync
	}
	return h, s, nil
}
