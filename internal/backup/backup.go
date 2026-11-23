// Package backup gera e abre o arquivo .otbackup: acessos, grupos, credenciais, hosts
// conhecidos e encaminhamentos, cifrados com uma senha escolhida na exportação. Mesma
// criptografia do cofre: Argon2id para a chave e XChaCha20-Poly1305 para o conteúdo.
package backup

import (
	"encoding/json"
	"errors"
	"fmt"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

const (
	format  = "openterminal-backup"
	version = 1
	// Limites do KDF aceitos na importação: um arquivo forjado não pode pedir 100 GiB ao Argon2.
	maxTime      = 10
	maxMemoryKiB = 1024 * 1024
	minPassword  = 8
)

var (
	ErrNotBackup     = errors.New("o arquivo não é um backup do OpenTerminal")
	ErrBadPassword   = errors.New("senha do backup incorreta ou arquivo alterado")
	ErrWeak          = fmt.Errorf("a senha do backup precisa de pelo menos %d caracteres", minPassword)
	ErrNeedsPassword = errors.New("este backup foi exportado com senha: digite a senha para importar")
)

// algNone marca o backup exportado sem senha: o conteúdo vai aberto em Plain.
const algNone = "none"

// Payload é o que vai cifrado. Configurações e perfis de terminal são da máquina e ficam fora.
type Payload struct {
	CreatedAt    string              `json:"createdAt"`
	Groups       []model.Group       `json:"groups"`
	Hosts        []model.Host        `json:"hosts"`
	Credentials  []model.Credential  `json:"credentials"`
	KnownHosts   []model.KnownHost   `json:"knownHosts"`
	ForwardRules []model.ForwardRule `json:"forwardRules"`
}

type file struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	KDF     struct {
		Alg       string `json:"alg"`
		Time      uint32 `json:"time"`
		MemoryKiB uint32 `json:"memoryKiB"`
		Salt      []byte `json:"salt"`
	} `json:"kdf"`
	Data  []byte   `json:"data,omitempty"`
	Plain *Payload `json:"plain,omitempty"`
}

// ad amarra os parâmetros do KDF ao conteúdo: trocar um deles faz o Unseal falhar.
func ad(f *file) []byte {
	return fmt.Appendf(nil, "%s:v%d|%s|%d|%d|%x", format, f.Version, f.KDF.Alg, f.KDF.Time, f.KDF.MemoryKiB, f.KDF.Salt)
}

// EncodePlain gera o backup sem senha: qualquer um que tiver o arquivo lê as senhas dos acessos.
func EncodePlain(p Payload) ([]byte, error) {
	f := &file{Format: format, Version: version, Plain: &p}
	f.KDF.Alg = algNone
	return json.MarshalIndent(f, "", "  ")
}

// IsEncrypted diz se o arquivo é um backup com senha (falso para sem senha ou arquivo inválido).
func IsEncrypted(data []byte) bool {
	var f file
	return json.Unmarshal(data, &f) == nil && f.Format == format && f.KDF.Alg != algNone
}

func Encode(p Payload, password string, kdf func(salt []byte) vault.KDFParams) ([]byte, error) {
	if len([]rune(password)) < minPassword {
		return nil, ErrWeak
	}
	params := kdf(vault.RandomSalt())
	f := &file{Format: format, Version: version}
	f.KDF.Alg, f.KDF.Time, f.KDF.MemoryKiB, f.KDF.Salt = "argon2id", params.Time, params.MemoryKiB, params.Salt
	plain, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	key := vault.DeriveKEK([]byte(password), params)
	if f.Data, err = vault.Seal(key, plain, ad(f)); err != nil {
		return nil, err
	}
	return json.MarshalIndent(f, "", "  ")
}

func Decode(data []byte, password string) (Payload, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil || f.Format != format {
		return Payload{}, ErrNotBackup
	}
	if f.Version == version && f.KDF.Alg == algNone {
		if f.Plain == nil {
			return Payload{}, ErrNotBackup
		}
		return *f.Plain, nil
	}
	if password == "" {
		return Payload{}, ErrNeedsPassword
	}
	if f.Version != version || f.KDF.Alg != "argon2id" || f.KDF.Time < 1 || f.KDF.Time > maxTime ||
		f.KDF.MemoryKiB < 8 || f.KDF.MemoryKiB > maxMemoryKiB || len(f.KDF.Salt) < 16 {
		return Payload{}, ErrNotBackup
	}
	key := vault.DeriveKEK([]byte(password), vault.KDFParams{Time: f.KDF.Time, MemoryKiB: f.KDF.MemoryKiB, Salt: f.KDF.Salt})
	plain, err := vault.Unseal(key, f.Data, ad(&f))
	if err != nil {
		return Payload{}, ErrBadPassword
	}
	var p Payload
	if err := json.Unmarshal(plain, &p); err != nil {
		return Payload{}, ErrNotBackup
	}
	return p, nil
}
