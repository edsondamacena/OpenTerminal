package apperr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
)

func TestEveryCodeHasMessage(t *testing.T) {
	for _, c := range Codes {
		if strings.TrimSpace(Messages[c]) == "" {
			t.Errorf("código %s sem mensagem", c)
		}
	}
	if len(Messages) != len(Codes) {
		t.Errorf("mensagens (%d) e códigos (%d) divergem", len(Messages), len(Codes))
	}
}

func TestNewCodesExist(t *testing.T) {
	for _, c := range []Code{NoCommonAlgorithm, AgentUnavailable, SchemeInvalid, AlreadyExists, InUse, VaultMigrationFailed} {
		if _, ok := Messages[c]; !ok {
			t.Errorf("código novo %s ausente", c)
		}
	}
}

func TestErrorIsJSONWithFixedMessage(t *testing.T) {
	err := New(AuthFailed)
	var payload struct {
		Code    Code   `json:"code"`
		Message string `json:"message"`
	}
	if e := json.Unmarshal([]byte(err.Error()), &payload); e != nil {
		t.Fatalf("Error() não é JSON: %v", e)
	}
	if payload.Code != AuthFailed || payload.Message != Messages[AuthFailed] {
		t.Fatalf("payload inesperado: %+v", payload)
	}
}

func TestSanitizeNeverLeaksOriginalText(t *testing.T) {
	secret := "senha-super-secreta-123"
	err := Sanitize(fmt.Errorf("falha com %s", secret))
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("mensagem vazou o erro original: %s", err.Error())
	}
	if CodeOf(err) != Internal {
		t.Fatalf("erro desconhecido deveria virar INTERNAL, veio %s", CodeOf(err))
	}
}

func TestSanitizeRedactsCommonCredentialShapes(t *testing.T) {
	cases := []string{
		`falhou: api_key="api-super-secreta"`,
		`curl --password senha-da-cli https://example.com`,
		`Authorization: Bearer token-do-cabecalho`,
		`ssh://usuario:senha-na-url@example.com/repositorio`,
		`resposta incluiu ghp_1234567890abcdefghijklmn`,
		"-----BEGIN OPENSSH PRIVATE KEY-----\nsegredo\n-----END OPENSSH PRIVATE KEY-----",
	}
	for _, input := range cases {
		got := redact(input)
		if !strings.Contains(got, "[REDACTED") {
			t.Errorf("não removeu segredo de %q: %q", input, got)
		}
		for _, secret := range []string{"api-super-secreta", "senha-da-cli", "token-do-cabecalho", "senha-na-url", "ghp_1234567890abcdefghijklmn", "\nsegredo\n"} {
			if strings.Contains(got, secret) {
				t.Errorf("segredo %q vazou em %q", secret, got)
			}
		}
	}
}

func TestSanitizeNil(t *testing.T) {
	if Sanitize(nil) != nil {
		t.Fatal("nil deve continuar nil")
	}
}

func TestCodeOfWrapped(t *testing.T) {
	err := fmt.Errorf("contexto: %w", New(VaultLocked))
	if CodeOf(err) != VaultLocked {
		t.Fatalf("esperava VAULT_LOCKED, veio %s", CodeOf(err))
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestCodeOfSystemErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Code
	}{
		{"cancelado", context.Canceled, Cancelled},
		{"prazo", context.DeadlineExceeded, ConnectTimeout},
		{"timeout de rede", &net.OpError{Op: "dial", Err: timeoutErr{}}, ConnectTimeout},
		{"dns", &net.DNSError{Err: "no such host", Name: "x.invalid", IsNotFound: true}, HostUnreachable},
		{"permissão", os.ErrPermission, PermissionDenied},
		{"inexistente", os.ErrNotExist, NotFound},
		{"autenticação ssh", errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"), AuthFailed},
		{"algoritmo ssh", errors.New("ssh: handshake failed: ssh: no common algorithm for key exchange; client offered: [curve25519-sha256], server offered: [diffie-hellman-group1-sha1]"), NoCommonAlgorithm},
	}
	for _, c := range cases {
		if got := CodeOf(c.err); got != c.want {
			t.Errorf("%s: esperava %s, veio %s", c.name, c.want, got)
		}
	}
}
