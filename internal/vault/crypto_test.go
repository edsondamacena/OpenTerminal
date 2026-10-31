package vault

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type kdfVector struct {
	Password string `json:"password"`
	SaltHex  string `json:"saltHex"`
	OpsLimit uint32 `json:"opsLimit"`
	MemLimit uint64 `json:"memLimit"`
	KekHex   string `json:"kekHex"`
}

func loadVector(t *testing.T) kdfVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "vault-001", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		KdfVector kdfVector `json:"kdfVector"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.KdfVector
}

// A KEK derivada em Go precisa ser a mesma do libsodium (crypto_pwhash, Argon2id13) da 001.
func TestDeriveKEKMatchesLibsodium(t *testing.T) {
	v := loadVector(t)
	salt, _ := hex.DecodeString(v.SaltHex)
	want, _ := hex.DecodeString(v.KekHex)
	got := DeriveKEK([]byte(v.Password), LibsodiumKDF(v.OpsLimit, v.MemLimit, salt))
	if !bytes.Equal(got, want) {
		t.Fatalf("KEK diverge do libsodium:\n got %x\nwant %x", got, want)
	}
}

func TestLibsodiumKDFConversion(t *testing.T) {
	// MODERATE do libsodium: 3 passes, 256 MiB.
	p := LibsodiumKDF(3, 268435456, make([]byte, 16))
	if p.Time != 3 || p.MemoryKiB != 262144 {
		t.Fatalf("conversão errada: %+v", p)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := RandomKey()
	ad := []byte("hosts:4ce66518-3068-4bef-ad0b-eb3f0141443d")
	sealed, err := Seal(key, []byte("conteúdo"), ad)
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) != NonceSize+len("conteúdo")+16 {
		t.Fatalf("tamanho inesperado: %d", len(sealed))
	}
	got, err := Unseal(key, sealed, ad)
	if err != nil || string(got) != "conteúdo" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	key := RandomKey()
	ad := []byte("hosts:a")
	sealed, _ := Seal(key, []byte("segredo"), ad)

	if _, err := Unseal(key, sealed, []byte("hosts:b")); err == nil {
		t.Error("AD trocado foi aceito")
	}
	for _, i := range []int{0, NonceSize, len(sealed) - 1} {
		bad := bytes.Clone(sealed)
		bad[i] ^= 1
		if _, err := Unseal(key, bad, ad); err == nil {
			t.Errorf("byte %d adulterado foi aceito", i)
		}
	}
	if _, err := Unseal(RandomKey(), sealed, ad); err == nil {
		t.Error("chave errada foi aceita")
	}
	if _, err := Unseal(key, sealed[:NonceSize], ad); err == nil {
		t.Error("conteúdo truncado foi aceito")
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	key := RandomKey()
	a, _ := Seal(key, []byte("x"), nil)
	b, _ := Seal(key, []byte("x"), nil)
	if bytes.Equal(a[:NonceSize], b[:NonceSize]) {
		t.Fatal("nonce repetido")
	}
}

func TestZero(t *testing.T) {
	b := []byte{1, 2, 3}
	Zero(b)
	if !bytes.Equal(b, []byte{0, 0, 0}) {
		t.Fatal("não zerou")
	}
}
