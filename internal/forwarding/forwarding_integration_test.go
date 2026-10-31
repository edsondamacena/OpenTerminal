//go:build integration

package forwarding

import (
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

type integrationHarness struct {
	manager     *Manager
	connections atomic.Int32
	eventsMu    sync.Mutex
	events      []StatusEvent
}

func newIntegrationHarness(t *testing.T) *integrationHarness {
	t.Helper()
	harness := &integrationHarness{}
	harness.manager = NewManager(func(string) (Client, error) {
		harness.connections.Add(1)
		return ssh.Dial("tcp", "127.0.0.1:2222", &ssh.ClientConfig{
			User:            "pwuser",
			Auth:            []ssh.AuthMethod{ssh.Password("pw-test-123")},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Servidor efêmero exclusivo da fixture.
			Timeout:         10 * time.Second,
		})
	}, func(event StatusEvent) {
		harness.eventsMu.Lock()
		harness.events = append(harness.events, event)
		harness.eventsMu.Unlock()
	})
	t.Cleanup(harness.manager.CloseAll)
	return harness
}

func (h *integrationHarness) snapshotEvents() []StatusEvent {
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	return append([]StatusEvent(nil), h.events...)
}

func integrationForwardRule(id string, bindPort int) model.ForwardRule {
	return model.ForwardRule{
		ID:               id,
		HostID:           "00000000-0000-4000-8000-000000000001",
		BindAddress:      "127.0.0.1",
		BindPort:         bindPort,
		DestHost:         "echo",
		DestPort:         7000,
		AllowNonLoopback: false,
	}
}

func echoThroughForward(t *testing.T, port int, text string) string {
	t.Helper()
	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	received := make([]byte, len(text))
	if _, err := io.ReadFull(connection, received); err != nil {
		t.Fatal(err)
	}
	return string(received)
}

func waitForwarding(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("tempo esgotado esperando estado do encaminhamento")
}

func assertPortAvailable(t *testing.T, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("porta %d não foi liberada: %v", port, err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
}

func runSSHDControl(t *testing.T, action string) {
	t.Helper()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(workingDir, "..", "..")
	command := exec.Command("node", filepath.Join(repoRoot, "scripts", "sshd.mjs"), action)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sshd %s: %v\n%s", action, err, output)
	}
}

func TestForwardingIntegrationEchoAndSharedConnection(t *testing.T) {
	harness := newIntegrationHarness(t)
	first := integrationForwardRule("00000000-0000-4000-8000-000000000011", freePort(t))
	second := integrationForwardRule("00000000-0000-4000-8000-000000000012", freePort(t))

	if err := harness.manager.Activate(first, "Teste"); err != nil {
		t.Fatal(err)
	}
	events := harness.snapshotEvents()
	if len(events) != 2 || events[0].Status.Status != StatusStarting || events[1].Status.Status != StatusActive {
		t.Fatalf("eventos da primeira regra: %+v", events)
	}
	if got := echoThroughForward(t, first.BindPort, "olá, eco!"); got != "olá, eco!" {
		t.Fatalf("eco = %q", got)
	}

	if err := harness.manager.Activate(second, "Teste"); err != nil {
		t.Fatal(err)
	}
	if got := echoThroughForward(t, second.BindPort, "segunda"); got != "segunda" {
		t.Fatalf("segundo eco = %q", got)
	}
	if got := harness.connections.Load(); got != 1 {
		t.Fatalf("conexões SSH = %d, esperado 1", got)
	}
	active := harness.manager.ListActive()
	ids := make([]string, len(active))
	for i, forward := range active {
		ids[i] = forward.RuleID
	}
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != first.ID || ids[1] != second.ID {
		t.Fatalf("regras ativas = %v", ids)
	}

	harness.manager.Deactivate(first.ID)
	if status := harness.manager.Status(first.ID); status.Status != StatusInactive || status.Reason != nil {
		t.Fatalf("status depois de desativar: %+v", status)
	}
	assertPortAvailable(t, first.BindPort)
	if got := echoThroughForward(t, second.BindPort, "ainda ativa"); got != "ainda ativa" {
		t.Fatalf("eco da regra restante = %q", got)
	}
}

func TestForwardingIntegrationPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	harness := newIntegrationHarness(t)
	rule := integrationForwardRule("00000000-0000-4000-8000-000000000013", port)

	err = harness.manager.Activate(rule, "Teste")
	if code := apperr.CodeOf(err); code != apperr.PortInUse {
		t.Fatalf("código = %q, esperado %q (erro %v)", code, apperr.PortInUse, err)
	}
	status := harness.manager.Status(rule.ID)
	if status.Status != StatusError || status.Reason == nil || *status.Reason != apperr.PortInUse {
		t.Fatalf("status = %+v", status)
	}
	active := harness.manager.ListActive()
	if len(active) != 1 || active[0].RuleID != rule.ID || active[0].Status != StatusError {
		t.Fatalf("lista ativa = %+v", active)
	}
	events := harness.snapshotEvents()
	if len(events) < 2 || events[len(events)-1].RuleID != rule.ID || events[len(events)-1].Status.Status != StatusError {
		t.Fatalf("eventos = %+v", events)
	}
}

func TestForwardingIntegrationConnectionDropReleasesPort(t *testing.T) {
	harness := newIntegrationHarness(t)
	rule := integrationForwardRule("00000000-0000-4000-8000-000000000014", freePort(t))
	if err := harness.manager.Activate(rule, "Teste"); err != nil {
		t.Fatal(err)
	}
	if got := echoThroughForward(t, rule.BindPort, "antes"); got != "antes" {
		t.Fatalf("eco antes da queda = %q", got)
	}

	restarted := false
	t.Cleanup(func() {
		if !restarted {
			runSSHDControl(t, "start")
		}
	})
	runSSHDControl(t, "stop")
	waitForwarding(t, 60*time.Second, func() bool {
		events := harness.snapshotEvents()
		if len(events) == 0 {
			return false
		}
		last := events[len(events)-1]
		return last.RuleID == rule.ID && last.Status.Status == StatusInactive
	})
	assertPortAvailable(t, rule.BindPort)
	events := harness.snapshotEvents()
	last := events[len(events)-1]
	if last.Status.Reason == nil || *last.Status.Reason != apperr.SessionClosed {
		t.Fatalf("evento final = %+v", last)
	}
	runSSHDControl(t, "start")
	restarted = true
}
