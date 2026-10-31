package forwarding

import (
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

type fakeClient struct {
	closed chan struct{}
	once   sync.Once
}

func newFakeClient() *fakeClient { return &fakeClient{closed: make(chan struct{})} }

func (c *fakeClient) Dial(string, string) (net.Conn, error) {
	local, remote := net.Pipe()
	go func() {
		defer remote.Close()
		buffer := make([]byte, 4096)
		for {
			n, err := remote.Read(buffer)
			if n > 0 {
				_, _ = remote.Write(buffer[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	return local, nil
}
func (c *fakeClient) Wait() error  { <-c.closed; return nil }
func (c *fakeClient) Close() error { c.once.Do(func() { close(c.closed) }); return nil }

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func forwardRule(id string, port int) model.ForwardRule {
	return model.ForwardRule{ID: id, HostID: "00000000-0000-4000-8000-000000000001",
		BindAddress: "127.0.0.1", BindPort: port, DestHost: "echo", DestPort: 7000}
}

func TestManagerForwardsAndSharesHostConnection(t *testing.T) {
	client := newFakeClient()
	var connects atomic.Int32
	var events []StatusEvent
	var eventMu sync.Mutex
	manager := NewManager(func(string) (Client, error) {
		connects.Add(1)
		return client, nil
	}, func(event StatusEvent) {
		eventMu.Lock()
		events = append(events, event)
		eventMu.Unlock()
	})
	t.Cleanup(manager.CloseAll)

	first := forwardRule("00000000-0000-4000-8000-000000000011", freePort(t))
	second := forwardRule("00000000-0000-4000-8000-000000000012", freePort(t))
	if err := manager.Activate(first, "Host"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Activate(second, "Host"); err != nil {
		t.Fatal(err)
	}
	if connects.Load() != 1 || manager.ActiveCount() != 2 || len(manager.ListActive()) != 2 {
		t.Fatalf("compartilhamento: connects=%d active=%d list=%d", connects.Load(), manager.ActiveCount(), len(manager.ListActive()))
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(first.BindAddress, strconv.Itoa(first.BindPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(conn, buffer); err != nil || string(buffer) != "ping" {
		t.Fatalf("eco %q, %v", buffer, err)
	}
	manager.Deactivate(first.ID)
	if manager.ActiveCount() != 1 {
		t.Fatalf("regra não desativada: %d", manager.ActiveCount())
	}
	manager.Deactivate(second.ID)
	select {
	case <-client.closed:
	case <-time.After(time.Second):
		t.Fatal("conexão compartilhada não fechou após última regra")
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	if len(events) < 6 || events[0].Status.Status != StatusStarting {
		t.Fatalf("eventos: %+v", events)
	}
}

func TestManagerReportsPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client := newFakeClient()
	manager := NewManager(func(string) (Client, error) { return client, nil }, nil)
	rule := forwardRule("00000000-0000-4000-8000-000000000013", listener.Addr().(*net.TCPAddr).Port)
	err = manager.Activate(rule, "Host")
	if apperr.CodeOf(err) != apperr.PortInUse {
		t.Fatalf("erro %v, código %s", err, apperr.CodeOf(err))
	}
	status := manager.Status(rule.ID)
	if status.Status != StatusError || status.Reason == nil || *status.Reason != apperr.PortInUse {
		t.Fatalf("status: %+v", status)
	}
	manager.Deactivate(rule.ID)
}
