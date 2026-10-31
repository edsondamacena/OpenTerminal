// Package migrate001 lê o vault.svlt da versão Electron (001) e o importa no cofre SQLite
// (FR-102, FR-103, research R5). O arquivo original só é renomeado depois de o cofre novo ser
// gravado e conferido; qualquer falha antes disso não altera nada.
package migrate001

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/vault"
)

const (
	migratedSuffix = ".migrated"
	settingsFile   = "settings.json"
	maxHeaderBytes = 64 * 1024
	// PrefLegacyTerminalFont guarda fonte e tamanho da 001 até o perfil global ser criado (T069).
	PrefLegacyTerminalFont = "legacyTerminalFont"
	migratedFrom           = "vault.svlt v1"
)

type TerminalFont struct {
	Family string `json:"family"`
	Size   int    `json:"size"`
}

// verifyHook só é usado pelos testes para simular falha depois da gravação.
var verifyHook func() error

type kdfHeader struct {
	Alg      string `json:"alg"`
	OpsLimit uint32 `json:"opsLimit"`
	MemLimit uint64 `json:"memLimit"`
	Salt     string `json:"salt"`
}

type header struct {
	Kdf        kdfHeader `json:"kdf"`
	WrappedDek string    `json:"wrappedDek"`
	CreatedAt  string    `json:"createdAt"`
}

// legacyContent é o VaultContent da 001 (schemaVersion 1).
type legacyContent struct {
	SchemaVersion int `json:"schemaVersion"`
	Settings      struct {
		AutoLockMinutes       *int `json:"autoLockMinutes"`
		ConnectTimeoutSeconds int  `json:"connectTimeoutSeconds"`
	} `json:"settings"`
	Groups        []model.Group       `json:"groups"`
	Hosts         []model.Host        `json:"hosts"`
	Credentials   []model.Credential  `json:"credentials"`
	KnownHostKeys []model.KnownHost   `json:"knownHostKeys"`
	ForwardRules  []model.ForwardRule `json:"forwardRules"`
}

// jsonText serializa como o JSON.stringify do JavaScript (sem escapar <, > e &).
func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return string(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
}

// wrapAD reproduz byte a byte o AD da 001:
// JSON.stringify({ v: 1, kdf: { alg, opsLimit, memLimit, salt }, createdAt }).
func wrapAD(h header) []byte {
	return fmt.Appendf(nil, `{"v":1,"kdf":{"alg":%s,"opsLimit":%d,"memLimit":%d,"salt":%s},"createdAt":%s}`,
		jsonText(h.Kdf.Alg), h.Kdf.OpsLimit, h.Kdf.MemLimit, jsonText(h.Kdf.Salt), jsonText(h.CreatedAt))
}

// decode separa cabeçalho (tudo antes do payload é o AD do payload) e payload.
func decode(raw []byte) (header, []byte, []byte, error) {
	var h header
	if len(raw) < 10 || !bytes.Equal(raw[:4], []byte("SVLT")) {
		return h, nil, nil, apperr.New(apperr.VaultUnreadable)
	}
	if v := binary.BigEndian.Uint16(raw[4:6]); v != vault.LegacySupportedVersion {
		return h, nil, nil, apperr.New(apperr.VaultUnsupportedVersion)
	}
	n := binary.BigEndian.Uint32(raw[6:10])
	end := 10 + uint64(n)
	if n == 0 || n > maxHeaderBytes || end >= uint64(len(raw)) {
		return h, nil, nil, apperr.New(apperr.VaultUnreadable)
	}
	if err := json.Unmarshal(raw[10:end], &h); err != nil || h.Kdf.Alg != "argon2id13" {
		return h, nil, nil, apperr.New(apperr.VaultUnreadable)
	}
	return h, raw[:end], raw[end:], nil
}

// read decifra o vault.svlt com a senha da 001.
func read(path, password string) (*legacyContent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, apperr.New(apperr.VaultNotFound)
	}
	h, headerBytes, payload, err := decode(raw)
	if err != nil {
		return nil, err
	}
	salt, err1 := base64.StdEncoding.DecodeString(h.Kdf.Salt)
	wrapped, err2 := base64.StdEncoding.DecodeString(h.WrappedDek)
	if err1 != nil || err2 != nil {
		return nil, apperr.New(apperr.VaultUnreadable)
	}
	kek := vault.DeriveKEK([]byte(password), vault.LibsodiumKDF(h.Kdf.OpsLimit, h.Kdf.MemLimit, salt))
	dek, err := vault.Unseal(kek, wrapped, wrapAD(h))
	vault.Zero(kek)
	if err != nil {
		return nil, apperr.New(apperr.VaultBadPassword)
	}
	plain, err := vault.Unseal(dek, payload, headerBytes)
	vault.Zero(dek)
	if err != nil {
		return nil, apperr.New(apperr.VaultBadPassword)
	}
	defer vault.Zero(plain)
	var c legacyContent
	if err := json.Unmarshal(plain, &c); err != nil || c.SchemaVersion != 1 {
		return nil, apperr.New(apperr.VaultUnreadable)
	}
	return &c, nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// convert mapeia o conteúdo da 001 para o modelo da 002 (data-model.md, "Migração").
func convert(lc *legacyContent) *model.Content {
	c := model.EmptyContent()
	c.Settings.AutoLockMinutes = lc.Settings.AutoLockMinutes
	c.Settings.ConnectTimeoutSeconds = lc.Settings.ConnectTimeoutSeconds
	c.Groups = append(c.Groups, lc.Groups...)
	for _, h := range lc.Hosts {
		h.LegacyAlgorithms = false
		h.ProfileID = nil
		c.Hosts = append(c.Hosts, h)
	}
	c.Credentials = append(c.Credentials, lc.Credentials...)
	for _, k := range lc.KnownHostKeys {
		k.ID = newID()
		c.KnownHosts = append(c.KnownHosts, k)
	}
	c.ForwardRules = append(c.ForwardRules, lc.ForwardRules...)
	return c
}

func canonical(c *model.Content) []byte {
	b, _ := json.Marshal(c)
	return b
}

// Migrate importa o cofre da 001 para store (que precisa estar em StateMigratable).
func Migrate(store *vault.Store, password string) error {
	if store.Status().State != vault.StateMigratable {
		return apperr.New(apperr.VaultNotFound)
	}
	dir := store.Dir()
	legacyPath := filepath.Join(dir, vault.LegacyFile)
	lc, err := read(legacyPath, password)
	if err != nil {
		return err
	}
	content := convert(lc)
	if err := store.ImportMigrated(password, content, migratedFrom); err != nil {
		return apperr.New(apperr.VaultMigrationFailed)
	}
	if err := verify(store, content); err != nil {
		// Desfaz: apaga o cofre recém-criado; o vault.svlt continua intacto.
		_ = store.Erase()
		return apperr.New(apperr.VaultMigrationFailed)
	}
	migrateSettings(store, dir)
	// O cofre novo já está gravado e conferido. Se a renomeação falhar, o cofre já existe no
	// banco e o vault.svlt deixa de ser considerado (estado locked).
	_ = os.Rename(legacyPath, legacyPath+migratedSuffix)
	return nil
}

func verify(store *vault.Store, want *model.Content) error {
	if verifyHook != nil {
		if err := verifyHook(); err != nil {
			return err
		}
	}
	got, err := store.Read()
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical(got), canonical(want)) {
		return errors.New("conteúdo gravado difere do lido da 001")
	}
	return nil
}

// migrateSettings leva o settings.json da 001 (sem segredos) para as preferências.
func migrateSettings(store *vault.Store, dir string) {
	raw, err := os.ReadFile(filepath.Join(dir, settingsFile))
	if err != nil {
		return
	}
	var s struct {
		Theme              string          `json:"theme"`
		TerminalFontFamily string          `json:"terminalFontFamily"`
		TerminalFontSize   int             `json:"terminalFontSize"`
		Window             json.RawMessage `json:"window"`
	}
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	switch s.Theme {
	case "system", "light", "dark":
		_ = store.SetPreference("theme", s.Theme)
	}
	if len(s.Window) > 0 {
		_ = store.SetPreference("window", s.Window)
	}
	if s.TerminalFontFamily != "" && s.TerminalFontSize >= 8 && s.TerminalFontSize <= 32 {
		_ = store.SetPreference(PrefLegacyTerminalFont, TerminalFont{Family: s.TerminalFontFamily, Size: s.TerminalFontSize})
		_ = store.ApplyLegacyTerminalFont(s.TerminalFontFamily, s.TerminalFontSize)
	}
	_ = os.Rename(filepath.Join(dir, settingsFile), filepath.Join(dir, settingsFile+migratedSuffix))
}
