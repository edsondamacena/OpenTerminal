//go:build windows

package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"openterminal/internal/model"
)

func TestWindowsAgentPipeAuthenticatesWithKeyring(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	pipe, agentDone := serveTestAgent(t, privateKey)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENTERMINAL_AGENT_PIPE", pipe)

	server := startServer(t, func(s *server) { s.authorized = signer.PublicKey() })
	options := opts(server, newKnown(), Auth{Method: model.AuthAgent})
	client, err := dial(t, options, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatalf("Dial() with Windows agent pipe: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-agentDone:
		if err != nil && err != io.EOF {
			t.Logf("agent server stopped with expected pipe close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent server did not stop after authentication")
	}
}

func TestMissingWindowsAgentPipeMapsToAgentUnavailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENTERMINAL_AGENT_PIPE", fmt.Sprintf(
		`\\.\pipe\openterminal-agent-missing-%d-%d`, os.Getpid(), time.Now().UnixNano(),
	))

	server := startServer(t, nil)
	options := opts(server, newKnown(), Auth{Method: model.AuthAgent})
	_, err := dial(t, options, &fakeIx{acceptKey: true})
	wantCode(t, err, "AGENT_UNAVAILABLE")
}

func TestSystemAgentIgnoresSSHAuthSock(t *testing.T) {
	t.Setenv("OPENTERMINAL_AGENT_PIPE", "")
	t.Setenv("SSH_AUTH_SOCK", `\\.\pipe\malicious-agent`)
	if got := systemAgentPipe(); got != windowsOpenSSHAgentPipe {
		t.Fatalf("pipe de produção = %q", got)
	}
}

func serveTestAgent(t *testing.T, privateKey any) (string, <-chan error) {
	t.Helper()
	pipe := fmt.Sprintf(`\\.\pipe\openterminal-agent-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(pipe, nil)
	if err != nil {
		t.Fatal(err)
	}

	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: privateKey, Comment: "keyuser"}); err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		done <- agent.ServeAgent(keyring, conn)
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return pipe, done
}
