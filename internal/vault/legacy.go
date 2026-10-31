package vault

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
)

// Prefixo do vault.svlt da 001: "SVLT" | formatVersion uint16 BE | headerLength uint32 BE.
var legacyMagic = []byte("SVLT")

// LegacySupportedVersion é o único formatVersion da 001 que a migração lê.
const LegacySupportedVersion = 1

// probeLegacy decide o estado quando só existe o arquivo da 001, lendo apenas o prefixo.
func probeLegacy(dir string) Status {
	f, err := os.Open(filepath.Join(dir, LegacyFile))
	if err != nil {
		return Status{State: StateNonexistent}
	}
	defer f.Close()
	prefix := make([]byte, 10)
	if _, err := io.ReadFull(f, prefix); err != nil || !bytes.Equal(prefix[:4], legacyMagic) {
		r := ReasonCorrupted
		return Status{State: StateUnreadable, Reason: &r}
	}
	if v := binary.BigEndian.Uint16(prefix[4:6]); v < 1 || v > LegacySupportedVersion {
		r := ReasonUnsupportedVersion
		return Status{State: StateUnreadable, Reason: &r}
	}
	return Status{State: StateMigratable}
}
