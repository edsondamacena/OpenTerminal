package sshx

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

type fakeKeepaliveConn struct {
	mu       sync.Mutex
	requests []keepaliveRequest
	err      error
	delay    time.Duration
	closed   chan struct{}
	once     sync.Once
}

type keepaliveRequest struct {
	name      string
	wantReply bool
	at        time.Time
}

func newFakeKeepaliveConn(err error) *fakeKeepaliveConn {
	return &fakeKeepaliveConn{err: err, closed: make(chan struct{})}
}

func (c *fakeKeepaliveConn) SendRequest(name string, wantReply bool, _ []byte) (bool, []byte, error) {
	c.mu.Lock()
	c.requests = append(c.requests, keepaliveRequest{name: name, wantReply: wantReply, at: time.Now()})
	err := c.err
	delay := c.delay
	c.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return err == nil, nil, err
}

func (c *fakeKeepaliveConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *fakeKeepaliveConn) snapshot() []keepaliveRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]keepaliveRequest(nil), c.requests...)
}

func TestKeepaliveUsesVaultDefault(t *testing.T) {
	if got := model.DefaultVaultSettings().KeepaliveSeconds; got != 30 {
		t.Fatalf("keepaliveSeconds default = %d, want 30", got)
	}
}

func TestKeepaliveSendsOpenSSHRequestAtConfiguredInterval(t *testing.T) {
	stop := make(chan struct{})
	conn := newFakeKeepaliveConn(nil)
	done := make(chan error, 1)
	go func() { done <- runKeepalive(stop, conn, 10*time.Millisecond, nil) }()

	waitForKeepaliveRequests(t, conn, 3)
	close(stop)
	if err := waitKeepaliveDone(t, done); err != nil {
		t.Fatalf("runKeepalive() after stop = %v, want nil", err)
	}

	requests := conn.snapshot()
	for i, request := range requests {
		if request.name != "keepalive@openssh.com" || !request.wantReply {
			t.Fatalf("request[%d] = %#v, want keepalive@openssh.com with reply", i, request)
		}
	}
	// Média, não cada par: sob carga o ticker entrega um tique atrasado e o seguinte logo depois.
	if n := len(requests); n > 1 {
		mean := requests[n-1].at.Sub(requests[0].at) / time.Duration(n-1)
		if mean < 5*time.Millisecond {
			t.Fatalf("mean request interval = %v, want approximately 10ms", mean)
		}
	}
}

func TestKeepaliveClosesConnectionAfterThreeConsecutiveFailures(t *testing.T) {
	stop := make(chan struct{})
	conn := newFakeKeepaliveConn(errors.New("no reply"))
	done := make(chan error, 1)
	go func() { done <- runKeepalive(stop, conn, time.Millisecond, nil) }()

	err := waitKeepaliveDone(t, done)
	if got := apperr.CodeOf(err); got != apperr.SessionClosed {
		t.Fatalf("runKeepalive() code = %q, want %q (err=%v)", got, apperr.SessionClosed, err)
	}
	if got := len(conn.snapshot()); got != 3 {
		t.Fatalf("keepalive requests = %d, want 3", got)
	}
	select {
	case <-conn.closed:
	default:
		t.Fatal("connection was not closed after three failures")
	}
}

func TestKeepaliveReportsSuccessfulRoundTripTime(t *testing.T) {
	stop := make(chan struct{})
	conn := newFakeKeepaliveConn(nil)
	conn.delay = 5 * time.Millisecond
	done := make(chan error, 1)
	latencies := make(chan time.Duration, 1)
	go func() {
		done <- runKeepalive(stop, conn, 10*time.Millisecond, func(rtt time.Duration) {
			select {
			case latencies <- rtt:
			default:
			}
		})
	}()

	select {
	case rtt := <-latencies:
		if rtt < conn.delay {
			t.Fatalf("reported RTT = %v, want at least %v", rtt, conn.delay)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive did not report RTT")
	}
	close(stop)
	if err := waitKeepaliveDone(t, done); err != nil {
		t.Fatalf("runKeepalive() after stop = %v, want nil", err)
	}
}

func TestKeepaliveStopsWithSessionWithoutLeaking(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	stop := make(chan struct{})
	conn := newFakeKeepaliveConn(nil)
	done := make(chan error, 1)
	go func() { done <- runKeepalive(stop, conn, time.Millisecond, nil) }()
	waitForKeepaliveRequests(t, conn, 1)

	close(stop)
	if err := waitKeepaliveDone(t, done); err != nil {
		t.Fatalf("runKeepalive() after stop = %v, want nil", err)
	}
	before := len(conn.snapshot())
	time.Sleep(5 * time.Millisecond)
	if after := len(conn.snapshot()); after != before {
		t.Fatalf("requests continued after stop: before=%d after=%d", before, after)
	}
}

func waitForKeepaliveRequests(t *testing.T, conn *fakeKeepaliveConn, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for len(conn.snapshot()) < want {
		select {
		case <-deadline:
			t.Fatalf("keepalive requests = %d, want at least %d", len(conn.snapshot()), want)
		case <-time.After(time.Millisecond):
		}
	}
}

func waitKeepaliveDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("runKeepalive did not stop")
		return nil
	}
}
