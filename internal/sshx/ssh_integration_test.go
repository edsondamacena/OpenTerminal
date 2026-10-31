//go:build integration

// Testes contra o OpenSSH de teste em contêiner. Antes: `node scripts/sshd.mjs up`.
// Rodar: go test -tags integration ./internal/sshx/
package sshx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

const (
	sshdPort       = 2222
	sshdLegacyPort = 2223
	testPassphrase = "kp-test-123"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	return filepath.Join(dir, "..", "..")
}

func composeExec(t *testing.T, script string) string {
	t.Helper()
	cmd := exec.Command("docker", "compose", "-f", filepath.Join(repoRoot(t), "tests", "fixtures", "sshd", "compose.yml"),
		"exec", "-T", "sshd", "sh", "-c", script)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func readKey(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "tests", "fixtures", "keys", "generated", name))
	if err != nil {
		t.Fatalf("chaves de teste ausentes (rode node scripts/sshd.mjs up): %v", err)
	}
	return string(b)
}

func target(port int, user string) Target {
	return Target{Address: "127.0.0.1", Port: port, Username: user}
}

func integrationOpts(port int, user string, known *fakeKnown, auth Auth) Options {
	return Options{Target: target(port, user), Auth: auth, ConnectTimeout: 10 * time.Second, KnownHosts: known}
}

func TestIntegrationAuthMethods(t *testing.T) {
	cases := []struct {
		name string
		user string
		auth Auth
		ix   *fakeIx
	}{
		{"senha", "pwuser", Auth{Method: model.AuthPassword, Password: "pw-test-123"}, &fakeIx{acceptKey: true}},
		{"chave", "keyuser", Auth{Method: model.AuthKey, PrivateKey: readKey(t, "ed25519")}, &fakeIx{acceptKey: true}},
		{"chave com passphrase", "keyuser", Auth{Method: model.AuthKey, PrivateKey: readKey(t, "ed25519-passphrase"),
			Passphrase: model.Ptr(testPassphrase)}, &fakeIx{acceptKey: true}},
		{"keyboard-interactive", "kbuser", Auth{Method: model.AuthAsk}, &fakeIx{acceptKey: true, answers: []string{"kb-test-123"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Dial(context.Background(), integrationOpts(sshdPort, tc.user, newKnown(), tc.auth), tc.ix)
			if err != nil {
				t.Fatal(err)
			}
			_ = c.Close()
		})
	}
}

func TestIntegrationHostKeyChangedBlocks(t *testing.T) {
	known := newKnown()
	auth := Auth{Method: model.AuthPassword, Password: "pw-test-123"}
	c, err := Dial(context.Background(), integrationOpts(sshdPort, "pwuser", known, auth), &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	composeExec(t, "rm -f /etc/ssh/ssh_host_ed25519_key* && ssh-keygen -q -t ed25519 -N '' -f /etc/ssh/ssh_host_ed25519_key && pkill -HUP -f sshd.pam")
	defer composeExec(t, "cp /etc/ssh/original/* /etc/ssh/ && pkill -HUP -f sshd.pam")
	time.Sleep(time.Second)
	ix := &fakeIx{acceptKey: false}
	_, err = Dial(context.Background(), integrationOpts(sshdPort, "pwuser", known, auth), ix)
	if apperr.CodeOf(err) != apperr.HostKeyRejected {
		t.Fatalf("erro %v", err)
	}
	if len(ix.hostKeys) != 1 || ix.hostKeys[0].Kind != "changed" {
		t.Fatalf("pergunta %+v", ix.hostKeys)
	}
}

func TestIntegrationLegacyServer(t *testing.T) {
	auth := Auth{Method: model.AuthPassword, Password: "legacy-test-123"}
	_, err := Dial(context.Background(), integrationOpts(sshdLegacyPort, "legacyuser", newKnown(), auth), &fakeIx{acceptKey: true})
	if apperr.CodeOf(err) != apperr.NoCommonAlgorithm {
		t.Fatalf("sem legados: %v", err)
	}
	o := integrationOpts(sshdLegacyPort, "legacyuser", newKnown(), auth)
	o.LegacyAlgorithms = true
	c, err := Dial(context.Background(), o, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func TestIntegrationConfiguredTerm(t *testing.T) {
	o := integrationOpts(sshdPort, "pwuser", newKnown(), Auth{Method: model.AuthPassword, Password: "pw-test-123"})
	o.Term = "vt100"
	client, err := Dial(context.Background(), o, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatal(err)
	}
	terminal := NewTerminal(client)
	var output lockedBuf
	if err := terminal.Start(80, 24, &output); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = terminal.Close() })
	if err := terminal.Write([]byte("printf '__TERM__%s\\n' \"$TERM\"\nexit\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), "__TERM__vt100") {
		if time.Now().After(deadline) {
			t.Fatalf("TERM escolhido não apareceu na saída: %q", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type collector struct {
	mu   sync.Mutex
	data strings.Builder
}

func (c *collector) emit(name string, payload any) {
	if ev, ok := payload.(session.DataEvent); ok {
		c.mu.Lock()
		c.data.WriteString(ev.Data)
		c.mu.Unlock()
	}
}

// Regressão do bug 2: o prompt aparece uma única vez nos primeiros 2 s.
func TestIntegrationSinglePrompt(t *testing.T) {
	known := newKnown()
	for i := 0; i < 20; i++ {
		var col collector
		m := session.NewManager(col.emit)
		id := m.Open(session.KindSSH, nil, func(ctx context.Context, ix session.Interactor) (session.Conn, error) {
			c, err := Dial(ctx, integrationOpts(sshdPort, "pwuser", known, Auth{Method: model.AuthPassword, Password: "pw-test-123"}), &autoIx{ix})
			if err != nil {
				return nil, err
			}
			return NewTerminal(c), nil
		})
		// O fit acontece algumas vezes seguidas na interface; só o primeiro tamanho vale.
		_ = m.Resize(id, 80, 24)
		_ = m.Resize(id, 80, 24)
		_ = m.Resize(id, 120, 30)
		time.Sleep(2 * time.Second)
		_ = m.Close(id)
		col.mu.Lock()
		out := col.data.String()
		col.mu.Unlock()
		lines := strings.Split(strings.TrimRight(strings.ReplaceAll(out, "\r", ""), "\n "), "\n")
		prompt := strings.TrimSpace(lines[len(lines)-1])
		if prompt == "" || strings.Count(out, prompt) != 1 {
			t.Fatalf("conexão %d: prompt %q aparece %d vezes em %q", i, prompt, strings.Count(out, prompt), out)
		}
	}
	time.Sleep(500 * time.Millisecond)
	if left := strings.TrimSpace(composeExec(t, "pgrep -u pwuser || true")); left != "" {
		t.Fatalf("processos de pwuser restantes no servidor: %s", left)
	}
}

// autoIx aceita a host key nova (a primeira conexão do teste).
type autoIx struct{ session.Interactor }

func (a *autoIx) AskHostKey(q session.HostKeyQuestion) (bool, error) { return q.Kind == "new", nil }
