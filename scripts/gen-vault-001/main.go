// Command gen-vault-001 recria a fixture binária usada para testar a migração do cofre Electron.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"openterminal/internal/vault"
)

type fixture struct {
	Password string          `json:"password"`
	Content  json.RawMessage `json:"content"`
}

type kdfHeader struct {
	Alg      string `json:"alg"`
	OpsLimit uint32 `json:"opsLimit"`
	MemLimit uint64 `json:"memLimit"`
	Salt     string `json:"salt"`
}

type header struct {
	KDF        kdfHeader `json:"kdf"`
	CreatedAt  string    `json:"createdAt"`
	WrappedDEK string    `json:"wrappedDek"`
}

func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return string(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
}

func wrapAD(h header) []byte {
	return fmt.Appendf(nil, `{"v":1,"kdf":{"alg":%s,"opsLimit":%d,"memLimit":%d,"salt":%s},"createdAt":%s}`,
		jsonText(h.KDF.Alg), h.KDF.OpsLimit, h.KDF.MemLimit, jsonText(h.KDF.Salt), jsonText(h.CreatedAt))
}

func run(sourceDir, outputDir string) error {
	expectedRaw, err := os.ReadFile(filepath.Join(sourceDir, "expected.json"))
	if err != nil {
		return err
	}
	settingsRaw, err := os.ReadFile(filepath.Join(sourceDir, "settings.json"))
	if err != nil {
		return err
	}
	var f fixture
	if err := json.Unmarshal(expectedRaw, &f); err != nil {
		return err
	}
	if f.Password == "" || len(f.Content) == 0 {
		return fmt.Errorf("expected.json não contém password/content")
	}
	var content bytes.Buffer
	if err := json.Compact(&content, f.Content); err != nil {
		return err
	}

	salt := vault.RandomSalt()
	params := vault.TestKDF(salt)
	h := header{
		KDF: kdfHeader{
			Alg:      "argon2id13",
			OpsLimit: params.Time,
			MemLimit: uint64(params.MemoryKiB) * 1024,
			Salt:     base64.StdEncoding.EncodeToString(salt),
		},
		CreatedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	}

	dek := vault.RandomKey()
	defer vault.Zero(dek)
	kek := vault.DeriveKEK([]byte(f.Password), params)
	wrapped, err := vault.Seal(kek, dek, wrapAD(h))
	vault.Zero(kek)
	if err != nil {
		return err
	}
	h.WrappedDEK = base64.StdEncoding.EncodeToString(wrapped)
	headerJSON, err := json.Marshal(h)
	if err != nil {
		return err
	}
	prefix := make([]byte, 10)
	copy(prefix, "SVLT")
	binary.BigEndian.PutUint16(prefix[4:6], vault.LegacySupportedVersion)
	binary.BigEndian.PutUint32(prefix[6:10], uint32(len(headerJSON)))
	headerBytes := append(prefix, headerJSON...)
	payload, err := vault.Seal(dek, content.Bytes(), headerBytes)
	if err != nil {
		return err
	}
	raw := append(headerBytes, payload...)

	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"expected.json": expectedRaw,
		"settings.json": settingsRaw,
		"vault.svlt":    raw,
		"vault.bak":     raw,
	} {
		if err := os.WriteFile(filepath.Join(outputDir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	source := flag.String("source", filepath.FromSlash("tests/fixtures/vault-001"), "diretório com expected.json e settings.json")
	output := flag.String("out", "", "diretório de saída; por padrão, o mesmo de -source")
	flag.Parse()
	if *output == "" {
		*output = *source
	}
	if err := run(*source, *output); err != nil {
		fmt.Fprintln(os.Stderr, "falha ao gerar fixture:", err)
		os.Exit(1)
	}
	fmt.Println("Cofre 001 de teste gerado em", *output)
}
