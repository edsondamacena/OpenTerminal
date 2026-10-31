// Package domain tem as regras de negócio do cofre (data-model.md da 001 e da 002). As funções
// trabalham sobre o rascunho de model.Content que vault.Store.Mutate entrega.
package domain

import (
	"crypto/rand"
	"fmt"
	"time"

	"openterminal/internal/apperr"
)

// NewID gera um UUID v4.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Now é o instante atual no formato ISO 8601 UTC usado pela 001.
func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

func invalid() error  { return apperr.New(apperr.InvalidInput) }
func notFound() error { return apperr.New(apperr.NotFound) }
