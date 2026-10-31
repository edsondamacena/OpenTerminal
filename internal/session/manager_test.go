package session

import (
	"context"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"openterminal/internal/apperr"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type event struct {
	name    string
	payload any
}

type recorder struct {
	mu     sync.Mutex
	events []event
}

func (r *recorder) emit(name string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event{name, payload})
}

func (r *recorder) snapshot() []event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event(nil), r.events...)
}

func (r *recorder) states(id string) []State {
	var out []State
	for _, e := range r.snapshot() {
		if ev, ok := e.payload.(StateEvent); ok && ev.SessionID == id {
			out = append(out, ev.State)
		}
	}
	return out
}

func (r *recorder) waitState(t *testing.T, id string, st State) StateEvent {
	t.Helper()
	var found StateEvent
	waitFor(t, func() bool {
		for _, e := range r.snapshot() {
			if ev, ok := e.payload.(StateEvent); ok && ev.SessionID == id && ev.State == st {
				found = ev
				return true
			}
		}
		return false
	})
	return found
}

// fakeConn simula o PTY: a saída vem de um pipe que o teste alimenta.
type fakeConn struct {
	mu       sync.Mutex
	starts   []size
	resizes  []size
	written  []byte
	pr       *io.PipeReader
	pw       *io.PipeWriter
	out      io.Writer
	closed   bool
	exitCode *int
	latency  atomic.Int64
}

func newFakeConn() *fakeConn {
	pr, pw := io.Pipe()
	return &fakeConn{pr: pr, pw: pw}
}

func (c *fakeConn) Start(cols, rows int, out io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts = append(c.starts, size{cols, rows})
	c.out = out
	return nil
}

func (c *fakeConn) Wait() error {
	c.mu.Lock()
	out := c.out
	c.mu.Unlock()
	_, err := io.Copy(out, c.pr)
	return err
}

func (c *fakeConn) Write(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, p...)
	return nil
}

func (c *fakeConn) Resize(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resizes = append(c.resizes, size{cols, rows})
	return nil
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	_ = c.pw.Close()
	return nil
}

func (c *fakeConn) ExitCode() *int { return c.exitCode }

func (c *fakeConn) Latency() time.Duration { return time.Duration(c.latency.Load()) }

func (c *fakeConn) snapshot() (starts, resizes []size) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]size(nil), c.starts...), append([]size(nil), c.resizes...)
}

func TestLatencyReemitsConnectedStateAfterSignificantChange(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	_ = m.Resize(id, 80, 24)
	initial := r.waitState(t, id, StateConnected)
	if initial.LatencyMs != nil {
		t.Fatalf("initial latency = %v, want nil", *initial.LatencyMs)
	}

	c.latency.Store(int64(10 * time.Millisecond))
	waitFor(t, func() bool {
		for _, event := range r.snapshot() {
			if state, ok := event.payload.(StateEvent); ok && state.SessionID == id &&
				state.LatencyMs != nil && *state.LatencyMs == 10 {
				return true
			}
		}
		return false
	})
	c.latency.Store(int64(13 * time.Millisecond))
	waitFor(t, func() bool {
		for _, event := range r.snapshot() {
			if state, ok := event.payload.(StateEvent); ok && state.SessionID == id &&
				state.LatencyMs != nil && *state.LatencyMs == 13 {
				return true
			}
		}
		return false
	})
	if err := m.Close(id); err != nil {
		t.Fatal(err)
	}
}

func dialTo(c *fakeConn) Dialer {
	return func(ctx context.Context, ix Interactor) (Conn, error) {
		ix.SetState(StateAuthenticating)
		return c, nil
	}
}

func TestOpenCloseHundredTimesNoLeak(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	before := runtime.NumGoroutine()
	for i := 0; i < 100; i++ {
		c := newFakeConn()
		id := m.Open(KindSSH, nil, dialTo(c))
		if err := m.Resize(id, 80, 24); err != nil {
			t.Fatal(err)
		}
		r.waitState(t, id, StateConnected)
		go func() { _, _ = c.pw.Write([]byte("prompt> ")) }()
		if err := m.Close(id); err != nil {
			t.Fatal(err)
		}
	}
	if m.Count() != 0 {
		t.Fatalf("sobraram %d sessões", m.Count())
	}
	goleak.VerifyNone(t)
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("goroutines: antes %d, depois %d", before, after)
	}
}

func TestNothingAfterClosed(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	_ = m.Resize(id, 80, 24)
	r.waitState(t, id, StateConnected)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			if _, err := c.pw.Write([]byte("x")); err != nil {
				return
			}
		}
	}()
	if err := m.Close(id); err != nil {
		t.Fatal(err)
	}
	<-done
	time.Sleep(3 * FlushDelay)
	ev := r.snapshot()
	last := ev[len(ev)-1]
	if last.name != EventClosed {
		t.Fatalf("último evento = %s", last.name)
	}
	for _, e := range ev[:len(ev)-1] {
		if e.name == EventClosed {
			t.Fatal("term:closed duplicado")
		}
	}
	if err := m.Write(id, "a"); apperr.CodeOf(err) != apperr.NotFound {
		t.Fatalf("Write depois de fechar: %v", err)
	}
}

func TestPTYOnlyAfterFirstValidResize(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	defer m.Close(id)
	r.waitState(t, id, StateAuthenticating)
	time.Sleep(20 * time.Millisecond)
	if starts, _ := c.snapshot(); len(starts) != 0 {
		t.Fatal("PTY pedido antes do Resize")
	}
	for _, bad := range [][2]int{{0, 0}, {0, 24}, {80, 0}, {-1, 5}, {1001, 24}} {
		if err := m.Resize(id, bad[0], bad[1]); apperr.CodeOf(err) != apperr.InvalidInput {
			t.Fatalf("Resize(%v) = %v", bad, err)
		}
	}
	if starts, _ := c.snapshot(); len(starts) != 0 {
		t.Fatal("PTY pedido com tamanho inválido")
	}
	_ = m.Resize(id, 120, 30)
	r.waitState(t, id, StateConnected)
	starts, resizes := c.snapshot()
	if len(starts) != 1 || starts[0] != (size{120, 30}) || len(resizes) != 0 {
		t.Fatalf("starts=%v resizes=%v", starts, resizes)
	}
}

func TestWriteBeforeStartIsFlushedInOrder(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	defer m.Close(id)

	if err := m.Write(id, "primeiro "); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(id, "segundo"); err != nil {
		t.Fatal(err)
	}
	if err := m.Resize(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	r.waitState(t, id, StateConnected)
	c.mu.Lock()
	got := string(c.written)
	c.mu.Unlock()
	if got != "primeiro segundo" {
		t.Fatalf("entrada = %q", got)
	}
}

func TestPendingInputIsBounded(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	defer m.Close(id)

	input := string(make([]byte, pendingInputLimit+1024))
	if err := m.Write(id, input); err != nil {
		t.Fatal(err)
	}
	if err := m.Resize(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	r.waitState(t, id, StateConnected)
	c.mu.Lock()
	got := len(c.written)
	c.mu.Unlock()
	if got != pendingInputLimit {
		t.Fatalf("entrada pendente = %d bytes, esperado %d", got, pendingInputLimit)
	}
}

func TestInputWhileDisconnectedIsDropped(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c1 := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c1))
	defer m.Close(id)
	_ = m.Resize(id, 80, 24)
	r.waitState(t, id, StateConnected)
	_ = c1.pw.Close()
	r.waitState(t, id, StateDisconnected)

	if err := m.Write(id, "rm -rf ~"); err != nil {
		t.Fatal(err)
	}
	c2 := newFakeConn()
	if err := m.Reconnect(id, dialTo(c2)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		s := r.states(id)
		return s[len(s)-1] == StateConnected
	})
	c2.mu.Lock()
	got := string(c2.written)
	c2.mu.Unlock()
	if got != "" {
		t.Fatalf("entrada digitada com a aba caída chegou ao shell: %q", got)
	}
}

func TestSameResizeIgnored(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c))
	defer m.Close(id)
	_ = m.Resize(id, 80, 24)
	r.waitState(t, id, StateConnected)
	_ = m.Resize(id, 80, 24)
	_ = m.Resize(id, 100, 30)
	_ = m.Resize(id, 100, 30)
	if _, resizes := c.snapshot(); len(resizes) != 1 || resizes[0] != (size{100, 30}) {
		t.Fatalf("resizes=%v", resizes)
	}
}

func TestCloseDuringAuthDoesNotHang(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	id := m.Open(KindSSH, nil, func(ctx context.Context, ix Interactor) (Conn, error) {
		ix.SetState(StateAuthenticating)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	r.waitState(t, id, StateAuthenticating)
	done := make(chan struct{})
	go func() {
		_ = m.Close(id)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close pendurou durante a autenticação")
	}
	for _, st := range r.states(id) {
		if st == StateError {
			t.Fatal("cancelamento virou erro")
		}
	}
}

func TestCloseWhileAwaitingHostKey(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	id := m.Open(KindSSH, nil, func(ctx context.Context, ix Interactor) (Conn, error) {
		ok, err := ix.AskHostKey(HostKeyQuestion{Kind: "new", KeyType: "ssh-ed25519", Fingerprint: "SHA256:x"})
		if err != nil || !ok {
			return nil, apperr.New(apperr.HostKeyRejected)
		}
		return newFakeConn(), nil
	})
	r.waitState(t, id, StateAwaitingHostKey)
	if err := m.Close(id); err != nil {
		t.Fatal(err)
	}
}

func TestHostKeyAndPromptAnswers(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	got := make(chan []string, 1)
	id := m.Open(KindSSH, nil, func(ctx context.Context, ix Interactor) (Conn, error) {
		ok, err := ix.AskHostKey(HostKeyQuestion{Kind: "new", KeyType: "ssh-ed25519", Fingerprint: "SHA256:x"})
		if err != nil || !ok {
			return nil, apperr.New(apperr.HostKeyRejected)
		}
		answers, err := ix.AskPrompt(PromptQuestion{Prompts: []PromptItem{{Text: "Senha:", Echo: false}}})
		if err != nil {
			return nil, err
		}
		got <- answers
		return c, nil
	})
	defer m.Close(id)
	if err := m.AnswerPrompt(id, []string{"cedo"}); apperr.CodeOf(err) != apperr.InvalidInput {
		t.Fatalf("resposta fora de hora aceita: %v", err)
	}
	r.waitState(t, id, StateAwaitingHostKey)
	if err := m.AnswerHostKey(id, true); err != nil {
		t.Fatal(err)
	}
	r.waitState(t, id, StateAwaitingPrompt)
	if err := m.AnswerPrompt(id, []string{"segredo"}); err != nil {
		t.Fatal(err)
	}
	if a := <-got; len(a) != 1 || a[0] != "segredo" {
		t.Fatalf("respostas %v", a)
	}
	var sawHostKey, sawPrompt bool
	for _, e := range r.snapshot() {
		sawHostKey = sawHostKey || e.name == EventHostKey
		sawPrompt = sawPrompt || e.name == EventPrompt
	}
	if !sawHostKey || !sawPrompt {
		t.Fatal("perguntas não emitidas")
	}
}

func TestRejectedHostKeyIsError(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	id := m.Open(KindSSH, nil, func(ctx context.Context, ix Interactor) (Conn, error) {
		ok, _ := ix.AskHostKey(HostKeyQuestion{Kind: "changed"})
		if !ok {
			return nil, apperr.New(apperr.HostKeyRejected)
		}
		return newFakeConn(), nil
	})
	defer m.Close(id)
	r.waitState(t, id, StateAwaitingHostKey)
	_ = m.AnswerHostKey(id, false)
	ev := r.waitState(t, id, StateError)
	if ev.Code == nil || *ev.Code != string(apperr.HostKeyRejected) || ev.Reason == nil {
		t.Fatalf("evento %+v", ev)
	}
}

func TestOutputBeforeDisconnectAndReconnect(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c1 := newFakeConn()
	id := m.Open(KindSSH, nil, dialTo(c1))
	defer m.Close(id)
	_ = m.Resize(id, 80, 24)
	r.waitState(t, id, StateConnected)
	_, _ = c1.pw.Write([]byte("tchau"))
	_ = c1.pw.Close()
	ev := r.waitState(t, id, StateDisconnected)
	if ev.Code == nil || *ev.Code != string(apperr.SessionClosed) {
		t.Fatalf("evento %+v", ev)
	}
	var order []string
	for _, e := range r.snapshot() {
		switch p := e.payload.(type) {
		case DataEvent:
			order = append(order, "data:"+p.Data)
		case StateEvent:
			order = append(order, string(p.State))
		}
	}
	if n := len(order); n < 2 || order[n-2] != "data:tchau" || order[n-1] != "disconnected" {
		t.Fatalf("ordem %v", order)
	}
	if m.ActiveCount() != 0 {
		t.Fatal("sessão desconectada conta como ativa")
	}

	c2 := newFakeConn()
	if err := m.Reconnect(id, dialTo(c2)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		s := r.states(id)
		return s[len(s)-1] == StateConnected
	})
	// O PTY novo usa o último tamanho, sem esperar outro Resize.
	if starts, _ := c2.snapshot(); len(starts) != 1 || starts[0] != (size{80, 24}) {
		t.Fatalf("starts %v", starts)
	}
	if m.ActiveCount() != 1 {
		t.Fatal("sessão reconectada não conta como ativa")
	}
	_ = m.Write(id, "ls\r")
	c2.mu.Lock()
	w := string(c2.written)
	c2.mu.Unlock()
	if w != "ls\r" {
		t.Fatalf("escrito %q", w)
	}
}

func TestLocalExitReportsCode(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	c := newFakeConn()
	code := 3
	c.exitCode = &code
	id := m.Open(KindLocal, nil, dialTo(c))
	defer m.Close(id)
	_ = m.Resize(id, 80, 24)
	r.waitState(t, id, StateConnected)
	_ = c.pw.Close()
	ev := r.waitState(t, id, StateClosed)
	if ev.ExitCode == nil || *ev.ExitCode != 3 {
		t.Fatalf("evento %+v", ev)
	}
	if err := m.Reconnect(id, dialTo(newFakeConn())); apperr.CodeOf(err) != apperr.InvalidInput {
		t.Fatalf("reconectar shell local: %v", err)
	}
}

func TestCloseAll(t *testing.T) {
	var r recorder
	m := NewManager(r.emit)
	for i := 0; i < 5; i++ {
		id := m.Open(KindSSH, nil, dialTo(newFakeConn()))
		_ = m.Resize(id, 80, 24)
	}
	m.CloseAll()
	if m.Count() != 0 {
		t.Fatal("sessões restantes")
	}
	n := 0
	for _, e := range r.snapshot() {
		if e.name == EventClosed {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("term:closed = %d", n)
	}
}
