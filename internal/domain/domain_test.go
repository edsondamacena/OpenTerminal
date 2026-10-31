package domain

import (
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

const (
	pwCred  = "00000000-0000-4000-8000-0000000000a1"
	keyCred = "00000000-0000-4000-8000-0000000000a2"
	missing = "00000000-0000-4000-8000-0000000000ff"
	t0      = "2026-10-23T10:00:00.000Z"
	t1      = "2026-10-23T11:00:00.000Z"
)

func testContent() *model.Content {
	c := model.EmptyContent()
	c.Credentials = append(c.Credentials,
		model.Credential{Kind: model.CredentialPassword, ID: pwCred, Name: "senha", Password: "x", CreatedAt: t0},
		model.Credential{Kind: model.CredentialKey, ID: keyCred, Name: "chave", PrivateKey: "k",
			KeyType: "ssh-ed25519", PublicKey: "ssh-ed25519 AAAA", Fingerprint: "SHA256:abc",
			Origin: "imported", CreatedAt: t0},
	)
	return c
}

func baseInput() HostInput {
	return HostInput{Label: "Servidor", Address: "srv.example.com", Port: 22, Username: "deploy",
		AuthMethod: model.AuthPassword, CredentialID: model.Ptr(pwCred)}
}

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if apperr.CodeOf(err) != code {
		t.Fatalf("esperava %s, veio %v", code, err)
	}
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
