package session

import (
	"context"
	"sync"

	"openterminal/internal/apperr"
)

// interaction leva perguntas de conexões sem aba (como encaminhamentos) à mesma fila de eventos
// usada pelas sessões. O id temporário permite que AnswerHostKey e AnswerPrompt entreguem a resposta.
type interaction struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	emit   Emitter

	mu      sync.Mutex
	state   State
	hostKey chan bool
	prompt  chan []string
}

// BeginInteraction registra um Interactor temporário para uma conexão que não cria aba.
// A função de encerramento sempre deve ser chamada quando a tentativa de conexão terminar.
func (m *Manager) BeginInteraction(ctx context.Context) (string, Interactor, func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	id := newInteractionID()
	ix := &interaction{id: id, ctx: ctx, cancel: cancel, emit: m.emit,
		state: StateConnecting, hostKey: make(chan bool, 1), prompt: make(chan []string, 1)}
	m.mu.Lock()
	m.interactions[id] = ix
	m.mu.Unlock()
	var once sync.Once
	done := func() {
		once.Do(func() {
			m.mu.Lock()
			if m.interactions[id] == ix {
				delete(m.interactions, id)
			}
			m.mu.Unlock()
			cancel()
		})
	}
	return id, ix, done
}

func newInteractionID() string { return newID() }

func (i *interaction) SetState(state State) {
	i.mu.Lock()
	i.state = state
	i.mu.Unlock()
}

func (i *interaction) AskHostKey(question HostKeyQuestion) (bool, error) {
	drain(i.hostKey)
	i.SetState(StateAwaitingHostKey)
	if i.emit != nil {
		i.emit(EventHostKey, HostKeyEvent{SessionID: i.id, HostKeyQuestion: question})
	}
	select {
	case accept := <-i.hostKey:
		return accept, nil
	case <-i.ctx.Done():
		return false, i.ctx.Err()
	}
}

func (i *interaction) AskPrompt(question PromptQuestion) ([]string, error) {
	drain(i.prompt)
	i.SetState(StateAwaitingPrompt)
	if i.emit != nil {
		i.emit(EventPrompt, PromptEvent{SessionID: i.id, PromptQuestion: question})
	}
	select {
	case answers := <-i.prompt:
		return answers, nil
	case <-i.ctx.Done():
		return nil, i.ctx.Err()
	}
}

func (i *interaction) answerHostKey(accept bool) error {
	i.mu.Lock()
	state := i.state
	i.mu.Unlock()
	if state != StateAwaitingHostKey {
		return apperr.New(apperr.InvalidInput)
	}
	select {
	case i.hostKey <- accept:
		return nil
	default:
		return apperr.New(apperr.InvalidInput)
	}
}

func (i *interaction) answerPrompt(answers []string) error {
	i.mu.Lock()
	state := i.state
	i.mu.Unlock()
	if state != StateAwaitingPrompt {
		return apperr.New(apperr.InvalidInput)
	}
	select {
	case i.prompt <- answers:
		return nil
	default:
		return apperr.New(apperr.InvalidInput)
	}
}
