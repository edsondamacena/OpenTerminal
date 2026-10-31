//go:build windows

package localpty

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type buf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *buf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *buf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func waitText(t *testing.T, b *buf, text string, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for strings.Count(b.String(), text) < n {
		if time.Now().After(deadline) {
			t.Fatalf("%q não apareceu %d vezes em %q", text, n, b.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func start(t *testing.T, s Shell) (*Process, *buf, chan struct{}) {
	t.Helper()
	p, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	var out buf
	if err := p.Start(100, 30, &out); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = p.Wait()
		close(done)
	}()
	return p, &out, done
}

func waitDone(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Wait não voltou")
	}
}

func TestShellPathsAreAbsoluteInSystemRoot(t *testing.T) {
	self, _ := os.Executable()
	for _, s := range []Shell{PowerShell, Cmd} {
		p, err := ShellPath(s)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(p) || !strings.HasPrefix(strings.ToLower(p), strings.ToLower(os.Getenv("SystemRoot"))) {
			t.Fatalf("%s: %s", s, p)
		}
		// Regressão do bug 1: nunca executar o próprio binário.
		if strings.EqualFold(p, self) {
			t.Fatal("shell aponta para o próprio executável")
		}
	}
	if _, err := ShellPath("bash"); err == nil {
		t.Fatal("shell desconhecido aceito")
	}
}

func TestCmdEchoAndExitCode(t *testing.T) {
	p, out, done := start(t, Cmd)
	_ = p.Write([]byte("echo marcador-cmd-1\r"))
	waitText(t, out, "marcador-cmd-1", 2)
	_ = p.Write([]byte("exit 7\r"))
	waitDone(t, done)
	if c := p.ExitCode(); c == nil || *c != 7 {
		t.Fatalf("código de saída %v", c)
	}
	_ = p.Close()
}

func TestPowerShellEchoAndExitCode(t *testing.T) {
	p, out, done := start(t, PowerShell)
	_ = p.Write([]byte("Write-Output ('marcador' + '-ps-2')\r"))
	waitText(t, out, "marcador-ps-2", 1)
	_ = p.Write([]byte("exit 3\r"))
	waitDone(t, done)
	if c := p.ExitCode(); c == nil || *c != 3 {
		t.Fatalf("código de saída %v", c)
	}
	_ = p.Close()
}

func TestCloseKillsShell(t *testing.T) {
	p, out, done := start(t, Cmd)
	_ = p.Write([]byte("echo pronto-3\r"))
	waitText(t, out, "pronto-3", 2)
	_ = p.Close()
	waitDone(t, done)
	if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() && p.cmd.ProcessState.ExitCode() == -1 {
		t.Fatalf("processo não terminou: %v", p.cmd.ProcessState)
	}
}
