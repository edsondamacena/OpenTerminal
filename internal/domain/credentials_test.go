package domain

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/model"
)

const testPassphrase = "kp-test-123"

type testKey struct {
	pem string
	pub string // formato authorized_keys sem comentário
}

func authorized(pub ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

func genKeys(t *testing.T) map[string]testKey {
	t.Helper()
	keys := map[string]testKey{}

	_, ed, _ := ed25519.GenerateKey(rand.Reader)
	blk, _ := ssh.MarshalPrivateKey(ed, "")
	pub, _ := ssh.NewPublicKey(ed.Public())
	keys["ed25519"] = testKey{string(pem.EncodeToMemory(blk)), authorized(pub)}

	blk, _ = ssh.MarshalPrivateKeyWithPassphrase(ed, "", []byte(testPassphrase))
	keys["ed25519-passphrase"] = testKey{string(pem.EncodeToMemory(blk)), authorized(pub)}

	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	blk, _ = ssh.MarshalPrivateKey(ec, "")
	pub, _ = ssh.NewPublicKey(&ec.PublicKey)
	keys["ecdsa-p256"] = testKey{string(pem.EncodeToMemory(blk)), authorized(pub)}

	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	der := x509.MarshalPKCS1PrivateKey(rk)
	pub, _ = ssh.NewPublicKey(&rk.PublicKey)
	keys["rsa-pem"] = testKey{string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})), authorized(pub)}
	return keys
}

func TestCreatePassword(t *testing.T) {
	c := model.EmptyContent()
	cr, err := CreatePassword(c, "prod", "s3nha", t0)
	mustOK(t, err)
	if cr.Kind != model.CredentialPassword || cr.Name != "prod" || cr.Password != "s3nha" {
		t.Fatalf("%+v", cr)
	}
	for _, bad := range [][2]string{{"", "x"}, {"a", ""}, {"a", strings.Repeat("x", 1025)}, {strings.Repeat("n", 101), "x"}} {
		_, err := CreatePassword(c, bad[0], bad[1], t0)
		wantCode(t, err, "INVALID_INPUT")
	}
}

func TestImportKeys(t *testing.T) {
	keys := genKeys(t)
	for file, keyType := range map[string]string{"ed25519": "ssh-ed25519", "ecdsa-p256": "ecdsa-sha2-nistp256", "rsa-pem": "ssh-rsa"} {
		c := model.EmptyContent()
		cr, err := ImportKey(c, file, keys[file].pem, nil, t0)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if cr.KeyType != keyType || cr.Origin != "imported" || cr.Passphrase != nil {
			t.Errorf("%s: %+v", file, cr)
		}
		if cr.PublicKey != keys[file].pub {
			t.Errorf("%s: chave pública %q, esperada %q", file, cr.PublicKey, keys[file].pub)
		}
		if !strings.HasPrefix(cr.Fingerprint, "SHA256:") || strings.Contains(cr.Fingerprint, "=") {
			t.Errorf("%s: fingerprint %q", file, cr.Fingerprint)
		}
	}
}

func TestImportKeyPassphrase(t *testing.T) {
	keys := genKeys(t)
	c := model.EmptyContent()
	cr, err := ImportKey(c, "p", keys["ed25519-passphrase"].pem, model.Ptr(testPassphrase), t0)
	mustOK(t, err)
	if cr.Passphrase == nil || *cr.Passphrase != testPassphrase {
		t.Fatal("passphrase não guardada")
	}

	c = model.EmptyContent()
	_, err = ImportKey(c, "p", keys["ed25519-passphrase"].pem, model.Ptr("errada"), t0)
	wantCode(t, err, "KEY_BAD_PASSPHRASE")
	_, err = ImportKey(c, "p", keys["ed25519-passphrase"].pem, nil, t0)
	wantCode(t, err, "KEY_BAD_PASSPHRASE")
	if len(c.Credentials) != 0 {
		t.Fatal("salvou com erro")
	}
}

func TestImportInvalidKey(t *testing.T) {
	keys := genKeys(t)
	c := model.EmptyContent()
	_, err := ImportKey(c, "x", "-----BEGIN NOTHING-----\nnão é uma chave\n", nil, t0)
	wantCode(t, err, "KEY_UNSUPPORTED")
	_, err = ImportKey(c, "x", keys["ed25519"].pub, nil, t0)
	wantCode(t, err, "KEY_UNSUPPORTED")
	_, err = ImportKey(c, "x", strings.Repeat("a", 64*1024+1), nil, t0)
	wantCode(t, err, "KEY_UNSUPPORTED")
	if len(c.Credentials) != 0 {
		t.Fatal("salvou chave inválida")
	}
}

func TestUnencryptedKeyIgnoresPassphrase(t *testing.T) {
	keys := genKeys(t)
	cr, err := ImportKey(model.EmptyContent(), "x", keys["ed25519"].pem, model.Ptr("qualquer"), t0)
	mustOK(t, err)
	if cr.Passphrase != nil {
		t.Fatal("passphrase desnecessária guardada")
	}
}

func TestGenerateEd25519(t *testing.T) {
	c := model.EmptyContent()
	plain, err := GenerateEd25519(c, "nova", nil, t0)
	mustOK(t, err)
	if plain.KeyType != "ssh-ed25519" || plain.Origin != "generated" {
		t.Fatalf("%+v", plain)
	}
	if _, err := ssh.ParseRawPrivateKey([]byte(plain.PrivateKey)); err != nil {
		t.Fatal(err)
	}
	withPass, err := GenerateEd25519(c, "nova2", model.Ptr("frase-123"), t0)
	mustOK(t, err)
	if _, err := ssh.ParseRawPrivateKey([]byte(withPass.PrivateKey)); err == nil {
		t.Fatal("chave com passphrase abriu sem ela")
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase([]byte(withPass.PrivateKey), []byte("frase-123")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveCredentialInUse(t *testing.T) {
	c := model.EmptyContent()
	cr, _ := CreatePassword(c, "prod", "x", t0)
	c.Hosts = append(c.Hosts, model.Host{ID: "00000000-0000-4000-8000-0000000000c1", Label: "h",
		Address: "h", Port: 22, Username: "u", AuthMethod: model.AuthPassword, CredentialID: &cr.ID})
	wantCode(t, RemoveCredential(c, cr.ID), "INVALID_INPUT")
	if ToSummary(c, cr).UsedByHostCount != 1 {
		t.Fatal("contagem de uso")
	}
	c.Hosts = nil
	mustOK(t, RemoveCredential(c, cr.ID))
	if len(c.Credentials) != 0 {
		t.Fatal("não removeu")
	}
}

func TestRenameCredential(t *testing.T) {
	c := model.EmptyContent()
	cr, _ := CreatePassword(c, "a", "x", t0)
	r, err := RenameCredential(c, cr.ID, "  b ")
	mustOK(t, err)
	if r.Name != "b" {
		t.Fatal(r.Name)
	}
}

func TestSummaryHasNoSecrets(t *testing.T) {
	keys := genKeys(t)
	c := model.EmptyContent()
	pw, _ := CreatePassword(c, "pw", "senha-super-secreta", t0)
	k, _ := ImportKey(c, "k", keys["ed25519-passphrase"].pem, model.Ptr(testPassphrase), t0)
	for _, cr := range []model.Credential{pw, k} {
		b, _ := json.Marshal(ToSummary(c, cr))
		s := string(b)
		for _, bad := range []string{"senha-super-secreta", testPassphrase, "PRIVATE KEY", `"password":`, `"privateKey":`, `"passphrase":`} {
			if strings.Contains(s, bad) {
				t.Errorf("resumo contém %s: %s", bad, s)
			}
		}
	}
}
