// Package session guarda as sessões de terminal abertas (research R11): uma goroutine por sessão
// lê a saída, os eventos saem na ordem e term:closed é sempre o último.
package session

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
)

type Kind string

const (
	KindSSH    Kind = "ssh"
	KindTelnet Kind = "telnet"
	KindLocal  Kind = "local"
	KindSFTP   Kind = "sftp"
)

type State string

const (
	StateConnecting      State = "connecting"
	StateAuthenticating  State = "authenticating"
	StateAwaitingHostKey State = "awaitingHostKey"
	StateAwaitingPrompt  State = "awaitingPrompt"
	StateConnected       State = "connected"
	StateDisconnected    State = "disconnected"
	StateError           State = "error"
	StateClosed          State = "closed"
)

const (
	EventData    = "term:data"
	EventState   = "term:state"
	EventHostKey = "term:hostkey"
	EventPrompt  = "term:prompt"
	EventClosed  = "term:closed"
)

const pendingInputLimit = 64 * 1024

// opening diz se a sessão está a caminho de conectar (entrada digitada fica guardada).
func (st State) opening() bool {
	switch st {
	case StateConnecting, StateAuthenticating, StateAwaitingHostKey, StateAwaitingPrompt:
		return true
	}
	return false
}

type DataEvent struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

type StateEvent struct {
	SessionID string  `json:"sessionId"`
	State     State   `json:"state"`
	Code      *string `json:"code"`
	// Mensagem pt-BR para disconnected/error.
	Reason *string `json:"reason"`
	// Código de saída do shell local, em closed.
	ExitCode *int `json:"exitCode"`
	// RTT do último keepalive SSH; nil antes da primeira medição e em sessões locais.
	LatencyMs *int `json:"latencyMs"`
}

type HostKeyQuestion struct {
	Kind                string  `json:"kind"`
	KeyType             string  `json:"keyType"`
	Fingerprint         string  `json:"fingerprint"`
	PreviousFingerprint *string `json:"previousFingerprint"`
}

type HostKeyEvent struct {
	SessionID string `json:"sessionId"`
	HostKeyQuestion
}

type PromptItem struct {
	Text string `json:"text"`
	Echo bool   `json:"echo"`
}

type PromptQuestion struct {
	Name         string       `json:"name"`
	Instructions string       `json:"instructions"`
	Prompts      []PromptItem `json:"prompts"`
}

type PromptEvent struct {
	SessionID string `json:"sessionId"`
	PromptQuestion
}

type ClosedEvent struct {
	SessionID string `json:"sessionId"`
}

// Emitter entrega eventos à interface (runtime.EventsEmit no app).
type Emitter func(event string, payload any)

// Interactor é o que o transporte usa durante a conexão: estado e perguntas ao usuário. As
// perguntas esperam a resposta ou o cancelamento da sessão.
type Interactor interface {
	SetState(State)
	AskHostKey(HostKeyQuestion) (bool, error)
	AskPrompt(PromptQuestion) ([]string, error)
}

// Conn é uma conexão já autenticada (SSH) ou um processo preparado (shell local).
type Conn interface {
	// Start pede o PTY com o primeiro tamanho válido; a saída vai para out.
	Start(cols, rows int, out io.Writer) error
	// Wait bloqueia até a saída terminar (fim do shell ou queda da conexão).
	Wait() error
	Write(p []byte) error
	Resize(cols, rows int) error
	// Close é idempotente e destrava a leitura.
	Close() error
}

// ExitCoder é implementado por shells locais.
type ExitCoder interface {
	ExitCode() *int
}

type latencyReporter interface {
	Latency() time.Duration
}

// Dialer conecta e autentica; deve desistir quando ctx é cancelado.
type Dialer func(ctx context.Context, ix Interactor) (Conn, error)

type size struct{ cols, rows int }

type Session struct {
	ID     string
	Kind   Kind
	HostID *string

	m      *Manager
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	out    *Batcher

	mu      sync.Mutex
	state   State
	conn    Conn
	started bool
	pending []byte
	size    size
	sized   chan struct{}
	latency *int

	reconnect chan Dialer
	hostKey   chan bool
	prompt    chan []string

	emitMu sync.Mutex
	closed bool
}

type Manager struct {
	emit Emitter

	mu           sync.Mutex
	sessions     map[string]*Session
	interactions map[string]*interaction
}

func NewManager(emit Emitter) *Manager {
	return &Manager{emit: emit, sessions: map[string]*Session{}, interactions: map[string]*interaction{}}
}

func newID() string { return domain.NewID() }

// Open cria a sessão e começa a conectar. O PTY só é pedido no primeiro Resize válido.
func (m *Manager) Open(kind Kind, hostID *string, dial Dialer) string {
	return m.OpenID(newID(), kind, hostID, dial)
}

// OpenID abre uma sessão com id previamente reservado. Serviços que precisam devolver o id
// apenas depois de concluir uma conexão interativa (como SFTP) usam este caminho.
func (m *Manager) OpenID(id string, kind Kind, hostID *string, dial Dialer) string {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: id, Kind: kind, HostID: hostID, m: m, ctx: ctx, cancel: cancel,
		state: StateConnecting, sized: make(chan struct{}), reconnect: make(chan Dialer, 1),
		hostKey: make(chan bool, 1), prompt: make(chan []string, 1)}
	s.out = NewBatcher(func(data string) { s.emit(EventData, DataEvent{SessionID: s.ID, Data: data}) })
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	s.wg.Add(1)
	go s.run(dial)
	return s.ID
}

func (m *Manager) get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func (m *Manager) Get(id string) (*Session, error) { return m.get(id) }

func (m *Manager) Write(id, data string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if !s.started {
		// Só guarda enquanto a conexão está sendo aberta; com a aba caída, o que se digita
		// não pode ir para o shell numa reconexão horas depois.
		if !s.state.opening() {
			s.mu.Unlock()
			return nil
		}
		remaining := pendingInputLimit - len(s.pending)
		if remaining > 0 {
			input := []byte(data)
			if len(input) > remaining {
				input = input[:remaining]
			}
			s.pending = append(s.pending, input...)
		}
		s.mu.Unlock()
		return nil
	}
	conn := s.conn
	s.mu.Unlock()
	return conn.Write([]byte(data))
}

// Resize guarda o tamanho; o primeiro válido libera o pedido do PTY. Repetir o último tamanho
// não faz nada.
func (m *Manager) Resize(id string, cols, rows int) error {
	if cols < 1 || cols > 1000 || rows < 1 || rows > 1000 {
		return apperr.New(apperr.InvalidInput)
	}
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := size{cols, rows}
	if next == s.size {
		return nil
	}
	first := s.size == size{}
	s.size = next
	if first {
		close(s.sized)
	}
	if s.started {
		return s.conn.Resize(cols, rows)
	}
	return nil
}

// Reconnect refaz a conexão de uma sessão desconectada.
func (m *Manager) Reconnect(id string, dial Dialer) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	st := s.state
	s.mu.Unlock()
	if st != StateDisconnected && st != StateError {
		return apperr.New(apperr.InvalidInput)
	}
	select {
	case s.reconnect <- dial:
	default:
	}
	return nil
}

func (m *Manager) AnswerHostKey(id string, accept bool) error {
	m.mu.Lock()
	s := m.sessions[id]
	ix := m.interactions[id]
	m.mu.Unlock()
	if s != nil {
		return s.answer(StateAwaitingHostKey, func() bool {
			select {
			case s.hostKey <- accept:
				return true
			default:
				return false
			}
		})
	}
	if ix != nil {
		return ix.answerHostKey(accept)
	}
	return apperr.New(apperr.NotFound)
}

// AnswerPrompt entrega as respostas; nil cancela a autenticação.
func (m *Manager) AnswerPrompt(id string, answers []string) error {
	m.mu.Lock()
	s := m.sessions[id]
	ix := m.interactions[id]
	m.mu.Unlock()
	if s != nil {
		return s.answer(StateAwaitingPrompt, func() bool {
			select {
			case s.prompt <- answers:
				return true
			default:
				return false
			}
		})
	}
	if ix != nil {
		return ix.answerPrompt(answers)
	}
	return apperr.New(apperr.NotFound)
}

// Close cancela a sessão, espera a goroutine terminar e emite term:closed, o último evento.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if !ok {
		return apperr.New(apperr.NotFound)
	}
	s.shutdown()
	return nil
}

// CloseAll encerra todas as sessões (saída do app).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for id, s := range m.sessions {
		all = append(all, s)
		delete(m.sessions, id)
	}
	interactions := make([]*interaction, 0, len(m.interactions))
	for id, ix := range m.interactions {
		interactions = append(interactions, ix)
		delete(m.interactions, id)
	}
	m.mu.Unlock()
	for _, ix := range interactions {
		ix.cancel()
	}
	var wg sync.WaitGroup
	for _, s := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.shutdown()
		}()
	}
	wg.Wait()
}

// ActiveCount conta as sessões em andamento, para a confirmação ao fechar.
func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sessions {
		switch s.State() {
		case StateDisconnected, StateError, StateClosed:
		default:
			n++
		}
	}
	return n
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// SizeOf devolve o último tamanho enviado a uma sessão que já iniciou o PTY.
func (m *Manager) SizeOf(id string) (cols, rows int, ok bool) {
	s, err := m.get(id)
	if err != nil {
		return 0, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return 0, 0, false
	}
	return s.size.cols, s.size.rows, true
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) answer(want State, deliver func() bool) error {
	if s.State() != want || !deliver() {
		return apperr.New(apperr.InvalidInput)
	}
	return nil
}

func (s *Session) emit(event string, payload any) {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	if !s.closed {
		s.m.emit(event, payload)
	}
}

func (s *Session) shutdown() {
	s.cancel()
	s.wg.Wait()
	s.out.Close()
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	s.closed = true
	s.m.emit(EventClosed, ClosedEvent{SessionID: s.ID})
}

// SetState implementa Interactor. A saída pendente sai antes da mudança de estado.
func (s *Session) SetState(st State) { s.setState(st, nil, nil) }

func (s *Session) setState(st State, cause error, exitCode *int) {
	s.mu.Lock()
	s.state = st
	if !st.opening() && st != StateConnected {
		s.pending = nil
	}
	if st != StateConnected {
		s.latency = nil
	}
	latency := s.latency
	s.mu.Unlock()
	s.out.Flush()
	ev := StateEvent{SessionID: s.ID, State: st, ExitCode: exitCode, LatencyMs: latency}
	if cause != nil {
		e := apperr.New(apperr.CodeOf(cause))
		code := string(e.Code)
		ev.Code, ev.Reason = &code, &e.Message
	}
	s.emit(EventState, ev)
}

func (s *Session) monitorLatency(stop <-chan struct{}, conn Conn) {
	reporter, ok := conn.(latencyReporter)
	if !ok {
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	last, lastEmit := 0, time.Time{}
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			rtt := reporter.Latency()
			if rtt <= 0 {
				continue
			}
			ms := int((rtt + time.Millisecond - 1) / time.Millisecond)
			changed := last == 0 || absInt(ms-last)*100 > last*20
			if !changed && now.Sub(lastEmit) < 10*time.Second {
				continue
			}
			s.mu.Lock()
			if s.state != StateConnected {
				s.mu.Unlock()
				continue
			}
			value := ms
			s.latency = &value
			s.mu.Unlock()
			s.emit(EventState, StateEvent{SessionID: s.ID, State: StateConnected, LatencyMs: &value})
			last, lastEmit = ms, now
		}
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (s *Session) AskHostKey(q HostKeyQuestion) (bool, error) {
	drain(s.hostKey)
	s.SetState(StateAwaitingHostKey)
	s.emit(EventHostKey, HostKeyEvent{SessionID: s.ID, HostKeyQuestion: q})
	select {
	case ok := <-s.hostKey:
		return ok, nil
	case <-s.ctx.Done():
		return false, s.ctx.Err()
	}
}

func (s *Session) AskPrompt(q PromptQuestion) ([]string, error) {
	drain(s.prompt)
	s.SetState(StateAwaitingPrompt)
	s.emit(EventPrompt, PromptEvent{SessionID: s.ID, PromptQuestion: q})
	select {
	case answers := <-s.prompt:
		return answers, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func drain[T any](ch chan T) {
	select {
	case <-ch:
	default:
	}
}

func (s *Session) run(dial Dialer) {
	defer s.wg.Done()
	for {
		s.connect(dial)
		select {
		case <-s.ctx.Done():
			return
		case dial = <-s.reconnect:
		}
	}
}

// connect faz um ciclo: conectar, esperar o tamanho, ler até o fim.
func (s *Session) connect(dial Dialer) {
	s.SetState(StateConnecting)
	conn, err := dial(s.ctx, s)
	if s.ctx.Err() != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return
	}
	if err != nil {
		s.setState(StateError, err, nil)
		return
	}
	stop := context.AfterFunc(s.ctx, func() { _ = conn.Close() })
	defer func() {
		stop()
		_ = conn.Close()
		s.mu.Lock()
		s.conn, s.started = nil, false
		s.mu.Unlock()
	}()

	select {
	case <-s.sized:
	case <-s.ctx.Done():
		return
	}
	s.mu.Lock()
	first := s.size
	s.mu.Unlock()
	if err := conn.Start(first.cols, first.rows, s.out); err != nil {
		if s.ctx.Err() == nil {
			s.setState(StateError, err, nil)
		}
		return
	}
	s.mu.Lock()
	s.conn = conn
	// Um Resize pode ter chegado enquanto o PTY era pedido.
	if s.size != first {
		_ = conn.Resize(s.size.cols, s.size.rows)
	}
	if len(s.pending) > 0 {
		err = conn.Write(s.pending)
		s.pending = nil
		if err != nil {
			s.mu.Unlock()
			if s.ctx.Err() == nil {
				s.setState(StateError, err, nil)
			}
			return
		}
	}
	s.started = true
	s.mu.Unlock()
	s.SetState(StateConnected)

	latencyStop := make(chan struct{})
	latencyDone := make(chan struct{})
	go func() {
		defer close(latencyDone)
		s.monitorLatency(latencyStop, conn)
	}()
	err = conn.Wait()
	close(latencyStop)
	<-latencyDone
	if s.ctx.Err() != nil {
		return
	}
	if s.Kind == KindLocal {
		var code *int
		if ec, ok := conn.(ExitCoder); ok {
			code = ec.ExitCode()
		}
		s.setState(StateClosed, nil, code)
		return
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = apperr.New(apperr.SessionClosed)
	}
	s.setState(StateDisconnected, err, nil)
}
