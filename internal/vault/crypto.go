// Package vault é o cofre local: SQLite com conteúdo cifrado por linha (research R3, R4).
package vault

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	KeySize   = chacha20poly1305.KeySize
	NonceSize = chacha20poly1305.NonceSizeX
	SaltSize  = 16
)

// KDFParams: Argon2id com paralelismo 1, como o crypto_pwhash do libsodium.
type KDFParams struct {
	Time      uint32
	MemoryKiB uint32
	Salt      []byte
}

// DefaultKDF equivale ao MODERATE do libsodium usado pela 001: 3 passes, 256 MiB.
func DefaultKDF(salt []byte) KDFParams {
	return KDFParams{Time: 3, MemoryKiB: 256 * 1024, Salt: salt}
}

// TestKDF é o custo mínimo, só para testes (o mesmo TEST_KDF da 001: 1 passe, 8 KiB).
func TestKDF(salt []byte) KDFParams {
	return KDFParams{Time: 1, MemoryKiB: 8, Salt: salt}
}

// LibsodiumKDF converte opsLimit/memLimit (bytes) do libsodium nos parâmetros do Argon2id.
func LibsodiumKDF(opsLimit uint32, memLimitBytes uint64, salt []byte) KDFParams {
	return KDFParams{Time: opsLimit, MemoryKiB: uint32(memLimitBytes / 1024), Salt: salt}
}

func DeriveKEK(password []byte, p KDFParams) []byte {
	return argon2.IDKey(password, p.Salt, p.Time, p.MemoryKiB, 1, KeySize)
}

func RandomKey() []byte {
	return randomBytes(KeySize)
}

func RandomSalt() []byte {
	return randomBytes(SaltSize)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand só falha se o SO não tiver fonte de entropia; seguir seria inseguro.
		panic("crypto/rand indisponível: " + err.Error())
	}
	return b
}

var errOpen = errors.New("conteúdo cifrado inválido")

// Seal cifra com XChaCha20-Poly1305 e devolve nonce(24) ‖ ciphertext, como a 001.
func Seal(key, plaintext, ad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, NonceSize, NonceSize+len(plaintext)+aead.Overhead())
	copy(out, randomBytes(NonceSize))
	return aead.Seal(out, out[:NonceSize], plaintext, ad), nil
}

// Unseal falha se a chave, o AD ou qualquer byte não conferirem.
func Unseal(key, sealed, ad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < NonceSize+aead.Overhead() {
		return nil, errOpen
	}
	pt, err := aead.Open(nil, sealed[:NonceSize], sealed[NonceSize:], ad)
	if err != nil {
		return nil, errOpen
	}
	return pt, nil
}

// Zero apaga uma chave ou texto em claro da memória.
func Zero(b []byte) {
	clear(b)
}
